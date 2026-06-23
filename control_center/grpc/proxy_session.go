package grpc

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"control_center/config"
	"control_center/models"
)

// Durée de vie d'une session de proxy. Assez longue pour une séance de travail, mais
// bornée : un cookie volé n'ouvre l'accès que temporairement.
const proxySessionTTL = 8 * time.Hour

// randomToken génère un identifiant opaque non devinable (valeur de cookie).
func randomToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// proxyCookieName : un cookie distinct par type, scopé au préfixe de chemin du proxy,
// pour que le navigateur le renvoie automatiquement sur l'iframe, les liens et les WS.
func proxyCookieName(kind string) string { return kind + "_proxy_session" }
func proxyCookiePath(kind string) string { return "/api/" + kind + "-proxy/" }

// mintProxySession crée une ProxySession pour une cible déjà résolue + autorisée, pose le
// cookie HttpOnly correspondant, et renvoie l'URL de base du proxy à ouvrir côté front.
func mintProxySession(w http.ResponseWriter, email, kind, poolID, ownerID string, tgt proxyTarget) string {
	sess := models.ProxySession{
		ID:         randomToken(),
		Email:      email,
		Kind:       kind,
		PoolID:     poolID,
		OwnerID:    ownerID,
		Target:     tgt.Target,
		VMID:       tgt.VMID,
		TargetIP:   tgt.IP,
		TargetPort: tgt.Port,
		Mode:       tgt.Mode,
		ExpiresAt:  time.Now().Add(proxySessionTTL),
	}
	config.Database.Create(&sess)

	http.SetCookie(w, &http.Cookie{
		Name:     proxyCookieName(kind),
		Value:    sess.ID,
		Path:     proxyCookiePath(kind),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(proxySessionTTL.Seconds()),
	})

	// Le chemin du proxy = l'UUID de la VM (URL-safe). C'est aussi le base_url calé côté
	// VM au boot (Jupyter --ServerApp.base_url=/api/jupyter-proxy/{uuid}/), ce qui évite
	// d'avoir l'email du propriétaire (avec @) dans l'URL — que Caddy rejette.
	return fmt.Sprintf("/api/%s-proxy/%s/", kind, tgt.VMID)
}

// lookupProxySession lit et valide la session référencée par le cookie pour une requête
// de proxy donnée. Vérifie : présence, expiration, type, et correspondance de l'UUID de
// VM du chemin (le cookie n'autorise que la VM figée à l'émission).
func lookupProxySession(r *http.Request, kind, vmID string) (*models.ProxySession, error) {
	c, err := r.Cookie(proxyCookieName(kind))
	if err != nil || c.Value == "" {
		return nil, fmt.Errorf("aucune session de proxy")
	}
	var sess models.ProxySession
	if err := config.Database.Where("id = ?", c.Value).First(&sess).Error; err != nil {
		return nil, fmt.Errorf("session inconnue")
	}
	if time.Now().After(sess.ExpiresAt) {
		config.Database.Delete(&sess)
		return nil, fmt.Errorf("session expirée")
	}
	if sess.Kind != kind || !strings.EqualFold(sess.VMID, vmID) {
		return nil, fmt.Errorf("session ne correspond pas à ce proxy")
	}
	if sess.TargetIP == "" {
		return nil, fmt.Errorf("session sans cible")
	}
	return &sess, nil
}

// serveAppProxy effectue le reverse-proxy (HTTP + WebSocket) vers la VM résolue dans la
// session. Gère les cibles HTTP (Jupyter) et HTTPS auto-signées (code-server --cert).
func serveAppProxy(w http.ResponseWriter, r *http.Request, sess *models.ProxySession) {
	scheme := "http"
	if sess.Kind == "vscode" {
		scheme = "https" // code-server tourne en --cert (TLS auto-signé)
	}
	targetBase, _ := url.Parse(fmt.Sprintf("%s://%s:%d", scheme, sess.TargetIP, sess.TargetPort))

	proxy := httputil.NewSingleHostReverseProxy(targetBase)
	if scheme == "https" {
		proxy.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // cert auto-signé interne
		}
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		// Permettre l'embarquement en iframe (même origine HTTPS via Caddy).
		resp.Header.Del("X-Frame-Options")
		resp.Header.Del("Content-Security-Policy")
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		log.Printf("[%s-proxy] %s/%s → %s erreur: %v", sess.Kind, sess.PoolID, sess.Target, targetBase.Host, err)
		http.Error(w, "application injoignable: "+err.Error(), http.StatusBadGateway)
	}

	r2 := r.Clone(r.Context())
	r2.URL.Scheme = targetBase.Scheme
	r2.URL.Host = targetBase.Host
	r2.Host = targetBase.Host
	// Jupyter vérifie l'Origin ; on l'aligne sur la cible pour qu'il accepte.
	r2.Header.Set("Origin", targetBase.String())
	r2.Header.Set("X-Forwarded-Proto", "https")
	// Caddy ajoute X-Forwarded-Host/Forwarded (= domaine public). code-server compare l'Origin
	// du WebSocket à cet hôte → mismatch avec l'Origin posé ci-dessus (la VM) → 403 sur le WS
	// (workbench "WebSocket close 1006"). On retire ces en-têtes : code-server retombe sur le
	// Host (la VM), cohérent avec l'Origin → handshake 101.
	r2.Header.Del("X-Forwarded-Host")
	r2.Header.Del("X-Forwarded-Port")
	r2.Header.Del("Forwarded")
	// Ne jamais transmettre nos cookies de session de proxy à la VM.
	r2.Header.Del("Cookie")

	// code-server est servi à la RACINE (pas de base-path comme Jupyter) : on retire le
	// préfixe /api/vscode-proxy/{uuid} avant de transmettre. Ses assets et redirections sont
	// relatifs (./_static, ./?folder=…) → ils se résolvent correctement sous le préfixe côté
	// navigateur. (Jupyter, lui, a un base_url calé sur le préfixe → on ne touche pas.)
	if sess.Kind == "vscode" {
		prefix := "/api/vscode-proxy/" + sess.VMID
		p := strings.TrimPrefix(r2.URL.Path, prefix)
		if p == "" || p[0] != '/' {
			p = "/" + p
		}
		r2.URL.Path = p
		r2.URL.RawPath = ""
	}

	proxy.ServeHTTP(w, r2)
}

// registerProxySessionRoutes branche les endpoints d'émission de session (handlers bruts
// car ils posent des cookies). Authentifiés via httpAuthMiddleware (préfixe /api/).
func registerProxySessionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/proxy-session", handleMintProxySession)
	mux.HandleFunc("/api/vscode-grant", handleVscodeGrant)     // POST créer / GET lister / DELETE révoquer
	mux.HandleFunc("/api/vscode-grant/join", handleVscodeJoin) // POST rejoindre via (cible + mot de passe)
}

// handleMintProxySession : POST /api/proxy-session
// Body: {kind, pool_id, owner_id, target?, mode?}. Résout + autorise la VM côté serveur,
// pose le cookie, et renvoie {url} (base du proxy à ouvrir en iframe / nouvel onglet).
func handleMintProxySession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}
	id, ok := requireProxyIdentity(w, r)
	if !ok {
		return
	}
	var body struct {
		Kind    string `json:"kind"`
		PoolID  string `json:"pool_id"`
		OwnerID string `json:"owner_id"`
		Target  string `json:"target"`
		Mode    string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "JSON invalide", http.StatusBadRequest)
		return
	}
	if body.Kind != "jupyter" && body.Kind != "vscode" {
		http.Error(w, "kind invalide (jupyter|vscode)", http.StatusBadRequest)
		return
	}
	if body.PoolID == "" || body.OwnerID == "" {
		http.Error(w, "pool_id et owner_id requis", http.StatusBadRequest)
		return
	}

	tgt, code, err := resolveProxyTarget(id, body.Kind, body.PoolID, body.OwnerID, body.Target, body.Mode)
	if err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	proxyURL := mintProxySession(w, id.Email, body.Kind, body.PoolID, body.OwnerID, tgt)
	writeJSON(w, map[string]any{"url": proxyURL, "mode": tgt.Mode, "target": tgt.Target})
}

// writeJSON encode une réponse JSON 200.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
