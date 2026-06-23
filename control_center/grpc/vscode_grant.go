package grpc

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"control_center/config"
	"control_center/models"

	"golang.org/x/crypto/bcrypt"
)

// Partage de VS Code entre élèves (Phase C).
//
// Un élève partage SA PROPRE VM (la cible est forcée à son identité authentifiée — il ne
// peut pas créer un partage pour la machine d'un autre). Il choisit le mode (lecture ou
// lecture+écriture), un mot de passe et une expiration. Un binôme présente (cible + mot
// de passe) via /join : si un grant valide existe, une session de proxy est ouverte vers
// la VM de la cible dans le mode autorisé. Le prof n'a pas besoin de grant (rôle staff).

const defaultGrantTTL = 24 * time.Hour

// handleVscodeGrant : POST créer / GET lister / DELETE révoquer un partage (le sien).
func handleVscodeGrant(w http.ResponseWriter, r *http.Request) {
	id, ok := requireProxyIdentity(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodPost:
		createVscodeGrant(w, r, id)
	case http.MethodGet:
		listVscodeGrants(w, r, id)
	case http.MethodDelete:
		revokeVscodeGrant(w, r, id)
	default:
		http.Error(w, "méthode non autorisée", http.StatusMethodNotAllowed)
	}
}

func createVscodeGrant(w http.ResponseWriter, r *http.Request, id httpIdentity) {
	var body struct {
		PoolID   string `json:"pool_id"`
		OwnerID  string `json:"owner_id"`
		Mode     string `json:"mode"`
		Password string `json:"password"`
		TTLHours int    `json:"ttl_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "JSON invalide", http.StatusBadRequest)
		return
	}
	if body.PoolID == "" || body.OwnerID == "" {
		http.Error(w, "pool_id et owner_id requis", http.StatusBadRequest)
		return
	}
	if len(body.Password) < 4 {
		http.Error(w, "mot de passe trop court (min 4 caractères)", http.StatusBadRequest)
		return
	}
	mode := "read"
	if body.Mode == "write" {
		mode = "write"
	}
	// La cible est TOUJOURS l'identité authentifiée : on ne partage que sa propre VM.
	// On vérifie au passage que cet élève a bien une VM dans ce pool.
	if _, _, err := resolveStudentVM(body.PoolID, body.OwnerID, id.Email); err != nil {
		http.Error(w, "vous n'avez pas de VM dans ce pool: "+err.Error(), http.StatusForbidden)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "erreur de hachage", http.StatusInternalServerError)
		return
	}
	ttl := defaultGrantTTL
	if body.TTLHours > 0 && body.TTLHours <= 24*7 {
		ttl = time.Duration(body.TTLHours) * time.Hour
	}

	// Un seul grant actif par (pool, cible) : on remplace l'éventuel précédent.
	config.Database.Where("pool_id = ? AND owner_id = ? AND target = ?", body.PoolID, body.OwnerID, id.Email).
		Delete(&models.VscodeGrant{})

	grant := models.VscodeGrant{
		PoolID:       body.PoolID,
		OwnerID:      body.OwnerID,
		Target:       id.Email,
		PasswordHash: string(hash),
		Mode:         mode,
		ExpiresAt:    time.Now().Add(ttl),
	}
	config.Database.Create(&grant)
	writeJSON(w, map[string]any{
		"ok":         true,
		"target":     grant.Target,
		"mode":       grant.Mode,
		"expires_at": grant.ExpiresAt,
	})
}

func listVscodeGrants(w http.ResponseWriter, r *http.Request, id httpIdentity) {
	poolID := r.URL.Query().Get("pool_id")
	ownerID := r.URL.Query().Get("owner_id")
	var grants []models.VscodeGrant
	q := config.Database.Where("target = ?", id.Email)
	if poolID != "" {
		q = q.Where("pool_id = ?", poolID)
	}
	if ownerID != "" {
		q = q.Where("owner_id = ?", ownerID)
	}
	q.Order("created_at DESC").Find(&grants)
	out := make([]map[string]any, 0, len(grants))
	for _, g := range grants {
		out = append(out, map[string]any{
			"id": g.ID, "pool_id": g.PoolID, "owner_id": g.OwnerID,
			"mode": g.Mode, "expires_at": g.ExpiresAt,
			"expired": time.Now().After(g.ExpiresAt),
		})
	}
	writeJSON(w, map[string]any{"grants": out})
}

func revokeVscodeGrant(w http.ResponseWriter, r *http.Request, id httpIdentity) {
	idStr := r.URL.Query().Get("id")
	gid, err := strconv.Atoi(idStr)
	if err != nil || gid <= 0 {
		http.Error(w, "id requis", http.StatusBadRequest)
		return
	}
	// On ne peut révoquer que son propre grant.
	config.Database.Where("id = ? AND target = ?", gid, id.Email).Delete(&models.VscodeGrant{})
	writeJSON(w, map[string]any{"ok": true})
}

// handleVscodeJoin : POST /api/vscode-grant/join {pool_id, owner_id, target, password}
// Vérifie un grant valide pour (pool, cible) + mot de passe, puis ouvre une session de
// proxy vscode vers la VM de la cible dans le mode du grant.
func handleVscodeJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}
	id, ok := requireProxyIdentity(w, r)
	if !ok {
		return
	}
	var body struct {
		PoolID   string `json:"pool_id"`
		OwnerID  string `json:"owner_id"`
		Target   string `json:"target"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "JSON invalide", http.StatusBadRequest)
		return
	}
	body.Target = strings.TrimSpace(body.Target)
	if body.PoolID == "" || body.OwnerID == "" || body.Target == "" || body.Password == "" {
		http.Error(w, "pool_id, owner_id, target et password requis", http.StatusBadRequest)
		return
	}
	// Le rejoignant doit appartenir au pool (élève avec VM, ou staff/propriétaire) — un
	// inconnu ne peut pas tenter des mots de passe sur des partages au hasard.
	if !isStaff(id.Role) && !strings.EqualFold(id.Email, body.OwnerID) {
		if _, _, err := resolveStudentVM(body.PoolID, body.OwnerID, id.Email); err != nil {
			http.Error(w, "réservé aux membres du pool", http.StatusForbidden)
			return
		}
	}

	var grant models.VscodeGrant
	err := config.Database.
		Where("pool_id = ? AND owner_id = ? AND target = ?", body.PoolID, body.OwnerID, body.Target).
		First(&grant).Error
	if err != nil {
		http.Error(w, "aucun partage pour cet élève", http.StatusNotFound)
		return
	}
	if time.Now().After(grant.ExpiresAt) {
		config.Database.Delete(&grant)
		http.Error(w, "partage expiré", http.StatusForbidden)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(grant.PasswordHash), []byte(body.Password)) != nil {
		http.Error(w, "mot de passe incorrect", http.StatusForbidden)
		return
	}

	vmID, ip, err := resolveStudentVM(body.PoolID, body.OwnerID, body.Target)
	if err != nil {
		http.Error(w, "VM de la cible introuvable: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	tgt := proxyTarget{
		Target: body.Target,
		VMID:   vmID,
		IP:     ip,
		Port:   kindPort("vscode", grant.Mode),
		Mode:   grant.Mode,
	}
	proxyURL := mintProxySession(w, id.Email, "vscode", body.PoolID, body.OwnerID, tgt)
	writeJSON(w, map[string]any{"url": proxyURL, "mode": grant.Mode, "target": grant.Target})
}
