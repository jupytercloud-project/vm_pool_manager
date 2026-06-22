package grpc

import (
	"context"
	"net/http"
	"strings"

	"control_center/config"
	"control_center/models"

	"github.com/danielgtaylor/huma/v2"
)

// registerPoolMetaHuma : POST /api/pool/meta {pool_id, user_id, label, tags} — définit le nom
// d'affichage et les étiquettes d'un pool. Réservé à l'équipe pédagogique.
func registerPoolMetaHuma(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "pool-meta", Method: http.MethodPost, Path: "/api/pool/meta",
		Summary: "Définir libellé et étiquettes d'un pool", Tags: []string{"pool"},
	}, func(ctx context.Context, in *struct {
		Body struct {
			PoolID string `json:"pool_id"`
			UserID string `json:"user_id"`
			Label  string `json:"label"`
			Tags   string `json:"tags"`
		}
	}) (*AnyOutput, error) {
		req := in.Body
		if strings.TrimSpace(req.PoolID) == "" || strings.TrimSpace(req.UserID) == "" {
			return nil, huma.Error400BadRequest("pool_id et user_id requis")
		}
		res := config.Database.Model(&models.Serverpool{}).
			Where("serverpool_id = ? AND user_id = ?", req.PoolID, req.UserID).
			Updates(map[string]any{
				"label": strings.TrimSpace(req.Label),
				"tags":  strings.TrimSpace(req.Tags),
			})
		if res.Error != nil {
			return nil, huma.Error500InternalServerError(res.Error.Error())
		}
		if res.RowsAffected == 0 {
			return nil, huma.Error404NotFound("pool introuvable")
		}
		return &AnyOutput{Body: map[string]any{"label": strings.TrimSpace(req.Label), "tags": strings.TrimSpace(req.Tags)}}, nil
	})
}
