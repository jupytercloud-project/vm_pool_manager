package grpc

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"control_center/config"
	"control_center/internal/oidc"
	"control_center/models"
)

// httpIdentity = identité authentifiée d'une requête REST, dérivée côté serveur
// (jamais d'un paramètre fourni par le client).
type httpIdentity struct {
	Email   string
	IsAdmin bool
	Via     string // "oidc" | "moodle" | "github"
}

type httpIdentityKeyT struct{}

var httpIdentityKey = httpIdentityKeyT{}

func identityFrom(ctx context.Context) (httpIdentity, bool) {
	id, ok := ctx.Value(httpIdentityKey).(httpIdentity)
	return id, ok
}

// effectiveEmail renvoie l'email à utiliser pour une action liée à un élève. Pour un
// non-admin, on FORCE l'email de l'identité authentifiée (un email fourni par le client
// est ignoré → pas d'IDOR). Un admin peut agir pour le compte d'un autre email.
func effectiveEmail(r *http.Request, requested string) string {
	id, ok := identityFrom(r.Context())
	if !ok {
		return ""
	}
	if id.IsAdmin && requested != "" {
		return requested
	}
	return id.Email
}

// publicHTTPPaths : routes accessibles SANS authentification — uniquement le login,
// les callbacks OAuth, le statut Moodle, le binaire d'enrôlement et les métriques.
var publicHTTPPaths = map[string]bool{
	"/api/moodle/status":    true,
	"/api/moodle/login":     true,
	"/api/github/login":     true,
	"/api/github/session":   true, // bootstrap : résout un session_id GitHub (id non devinable, infos publiques)
	"/auth/github/callback": true,
	"/vm-registrar":         true,
	"/metrics":              true,
}

// adminHTTPPrefixes : routes réservées aux enseignants/admin (lecture/écriture de notes,
// distribution, correction, gestion Moodle, données de tous les élèves).
var adminHTTPPrefixes = []string{
	"/api/nbgrader/release",
	"/api/nbgrader/collect",
	"/api/nbgrader/autograde",
	"/api/nbgrader/grades",
	"/api/nbgrader/export-csv",
	"/api/nbgrader/assignments",
	"/api/nbgrader/submission-url",
	"/api/nbgrader/jupyter-url",
	"/api/moodle/import",
	"/api/moodle/push-grades",
	"/api/moodle/link-pool",
	"/api/moodle/courses",
	"/api/moodle/enrolments",
	"/api/github/students",
	"/api/image-proposals",
}

func isAdminPath(p string) bool {
	for _, pre := range adminHTTPPrefixes {
		if strings.HasPrefix(p, pre) {
			return true
		}
	}
	return false
}

// resolveIdentity tente d'authentifier la requête, dans l'ordre :
//  1. Bearer JWT OIDC (profs via Dex) → email + groupe admins.
//  2. session_id Moodle/GitHub (élèves) présenté en Bearer ou X-Session-Id.
func resolveIdentity(r *http.Request) (httpIdentity, bool) {
	tok := strings.TrimSpace(r.Header.Get("Authorization"))
	tok = strings.TrimPrefix(tok, "Bearer ")
	tok = strings.TrimSpace(tok)
	if tok == "" {
		tok = strings.TrimSpace(r.Header.Get("X-Session-Id"))
	}
	if tok == "" {
		return httpIdentity{}, false
	}

	// 1. JWT OIDC.
	if claims, err := oidc.ParseToken(tok); err == nil {
		email, _ := claims["email"].(string)
		admin := false
		if raw, ok := claims["groups"].([]interface{}); ok {
			for _, g := range raw {
				if s, _ := g.(string); s == "admins" {
					admin = true
				}
			}
		}
		return httpIdentity{Email: email, IsAdmin: admin, Via: "oidc"}, email != ""
	}

	// 2. Session Moodle (élève ou admin site).
	var ms models.MoodleSession
	if err := config.Database.Where("id = ?", tok).First(&ms).Error; err == nil {
		if time.Since(ms.CreatedAt) <= 24*time.Hour {
			return httpIdentity{Email: ms.Email, IsAdmin: ms.Role == "admin", Via: "moodle"}, ms.Email != ""
		}
	}

	// 3. Session GitHub (élève, identité = login GitHub).
	var gs models.GitHubSession
	if err := config.Database.Where("id = ?", tok).First(&gs).Error; err == nil {
		if time.Since(gs.CreatedAt) <= time.Hour {
			return httpIdentity{Email: gs.Login, IsAdmin: false, Via: "github"}, gs.Login != ""
		}
	}

	return httpIdentity{}, false
}

// httpAuthMiddleware protège toutes les routes REST : public en liste blanche,
// authentification requise sinon, et rôle admin exigé sur les routes sensibles.
func httpAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if publicHTTPPaths[path] {
			next.ServeHTTP(w, r)
			return
		}

		id, ok := resolveIdentity(r)
		if !ok {
			httpJSONError(w, http.StatusUnauthorized, "authentification requise")
			return
		}
		if isAdminPath(path) && !id.IsAdmin {
			httpJSONError(w, http.StatusForbidden, "réservé aux enseignants")
			return
		}

		ctx := context.WithValue(r.Context(), httpIdentityKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func httpJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
