package grpc

import (
	"context"
	"net"
	"net/http"
	"strings"

	"control_center/config"
	"control_center/models"

	"github.com/danielgtaylor/huma/v2"
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

// registerAuditHuma : GET /api/admin/audit?limit=N — journal d'audit (admin uniquement).
func registerAuditHuma(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-audit", Method: http.MethodGet, Path: "/api/admin/audit",
		Summary: "Journal d'audit", Tags: []string{"admin"},
	}, func(ctx context.Context, in *struct {
		Limit int `query:"limit"`
	}) (*AnyOutput, error) {
		limit := 500
		if in.Limit > 0 && in.Limit <= 5000 {
			limit = in.Limit
		}
		var logs []models.AuditLog
		config.Database.Order("created_at DESC").Limit(limit).Find(&logs)
		return &AnyOutput{Body: map[string]any{"logs": logs}}, nil
	})
}
