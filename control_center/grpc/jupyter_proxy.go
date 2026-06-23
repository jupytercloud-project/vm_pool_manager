package grpc

import (
	"net/http"
	"net/url"
	"strings"
)

func decodePathSegment(s string) string {
	if d, err := url.PathUnescape(s); err == nil {
		return d
	}
	return s
}

// appProxyHandler renvoie le handler de reverse-proxy applicatif pour un type donné
// ("jupyter" → JupyterLab 8888, "vscode" → code-server 8443/8444).
//
// URL : /api/{kind}-proxy/{vm_uuid}/{...rest}
//
// Le segment de chemin est l'UUID de la VM (URL-safe) — c'est aussi le base_url calé
// côté VM au boot. L'accès n'est PAS ouvert par le simple fait d'être authentifié : il
// exige une ProxySession valide (cookie HttpOnly posé par POST /api/proxy-session après
// contrôle d'accès + résolution de la VM côté serveur). C'est ce qui permet à l'iframe,
// aux liens et aux WebSockets de passer sans porter le Bearer token JS, tout en gardant
// les VMs non exposées et l'accès maîtrisé (rôle ou grant).
func appProxyHandler(kind string) http.HandlerFunc {
	prefix := "/api/" + kind + "-proxy/"
	return func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, prefix)
		vmID := decodePathSegment(rest)
		if i := strings.IndexByte(vmID, '/'); i >= 0 {
			vmID = vmID[:i]
		}
		if vmID == "" {
			http.Error(w, "usage: "+prefix+"{vm_uuid}/...", http.StatusBadRequest)
			return
		}

		sess, err := lookupProxySession(r, kind, vmID)
		if err != nil {
			// 401 → le front sait qu'il doit (re)demander une session via /api/proxy-session.
			http.Error(w, "session de proxy requise: "+err.Error(), http.StatusUnauthorized)
			return
		}
		serveAppProxy(w, r, sess)
	}
}
