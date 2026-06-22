package grpc

import (
	"context"
	"net/http"
	"strings"

	"control_center/config"
	"control_center/models"

	"github.com/danielgtaylor/huma/v2"
)

// registerAnnouncementHuma : GET /api/announcement (public) + POST /api/admin/announcement (admin).
func registerAnnouncementHuma(api huma.API) {
	// GET /api/announcement — annonce courante (public : visible par tous, même non connecté).
	huma.Register(api, huma.Operation{
		OperationID: "get-announcement", Method: http.MethodGet, Path: "/api/announcement",
		Summary: "Annonce courante (bandeau)", Tags: []string{"announcement"},
	}, func(ctx context.Context, _ *struct{}) (*AnyOutput, error) {
		var a models.Announcement
		config.Database.Order("id ASC").First(&a)
		return &AnyOutput{Body: map[string]any{
			"message": a.Message, "active": a.Active, "updated_at": a.UpdatedAt,
		}}, nil
	})

	// POST /api/admin/announcement {message, active} — définit l'annonce (admin uniquement).
	huma.Register(api, huma.Operation{
		OperationID: "set-announcement", Method: http.MethodPost, Path: "/api/admin/announcement",
		Summary: "Définir l'annonce", Tags: []string{"announcement"},
	}, func(ctx context.Context, in *struct {
		Body struct {
			Message string `json:"message"`
			Active  bool   `json:"active"`
		}
	}) (*AnyOutput, error) {
		var a models.Announcement
		if err := config.Database.Order("id ASC").First(&a).Error; err != nil {
			a = models.Announcement{}
		}
		a.Message = strings.TrimSpace(in.Body.Message)
		a.Active = in.Body.Active && a.Message != ""
		if err := config.Database.Save(&a).Error; err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		return &AnyOutput{Body: map[string]any{"message": a.Message, "active": a.Active}}, nil
	})
}
