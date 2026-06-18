package grpc

import (
	"net"
	"net/http"
	"strconv"
	"strings"

	"control_center/config"
	"control_center/models"
)

// clientIP extrait l'IP source réelle (derrière le reverse-proxy Caddy via X-Forwarded-For).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// writeAudit enregistre une action mutante (POST/PUT/DELETE/PATCH) dans le journal.
func writeAudit(id httpIdentity, r *http.Request) {
	config.Database.Create(&models.AuditLog{
		Actor:  id.Email,
		Role:   id.Role,
		Method: r.Method,
		Path:   r.URL.Path,
		IP:     clientIP(r),
	})
}

// isMutating indique une méthode qui modifie l'état (donc à tracer).
func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	}
	return false
}

// GET /api/admin/audit?limit=N — journal d'audit (admin uniquement).
func handleAdminAudit(w http.ResponseWriter, r *http.Request) {
	limit := 500
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 5000 {
			limit = n
		}
	}
	var logs []models.AuditLog
	config.Database.Order("created_at DESC").Limit(limit).Find(&logs)
	writeJSONMoodle(w, http.StatusOK, map[string]any{"logs": logs})
}
