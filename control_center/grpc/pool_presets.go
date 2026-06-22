package grpc

import (
	"context"
	"net/http"
	"strings"

	"control_center/config"
	"control_center/models"

	"github.com/danielgtaylor/huma/v2"
)

// registerPoolPresetsHuma : /api/pool/presets — presets de création de pool (staff).
//
//	GET           → liste les presets de l'utilisateur
//	POST {…}      → enregistre un preset (upsert par nom)
//	DELETE ?id=N  → supprime un preset (le sien)
func registerPoolPresetsHuma(api huma.API) {
	// GET /api/pool/presets — liste les presets de l'utilisateur.
	huma.Register(api, huma.Operation{
		OperationID: "list-presets", Method: http.MethodGet, Path: "/api/pool/presets",
		Summary: "Lister les presets de pool", Tags: []string{"pool"},
	}, func(ctx context.Context, _ *struct{}) (*AnyOutput, error) {
		id, _ := identityFrom(ctx)
		var presets []models.PoolPreset
		config.Database.Where("owner_email = ?", id.Email).Order("name").Find(&presets)
		return &AnyOutput{Body: map[string]any{"presets": presets}}, nil
	})

	// POST /api/pool/presets — enregistre un preset (upsert par (owner, name)).
	huma.Register(api, huma.Operation{
		OperationID: "save-preset", Method: http.MethodPost, Path: "/api/pool/presets",
		Summary: "Enregistrer un preset de pool", Tags: []string{"pool"},
	}, func(ctx context.Context, in *struct{ Body models.PoolPreset }) (*AnyOutput, error) {
		id, _ := identityFrom(ctx)
		owner := id.Email
		p := in.Body
		if strings.TrimSpace(p.Name) == "" {
			return nil, huma.Error400BadRequest("nom du preset requis")
		}
		preset := models.PoolPreset{
			OwnerEmail:  owner,
			Name:        strings.TrimSpace(p.Name),
			Image:       p.Image,
			Flavor:      p.Flavor,
			Network:     p.Network,
			Config:      p.Config,
			MinVM:       p.MinVM,
			MaxVM:       p.MaxVM,
			AppPort:     p.AppPort,
			OffDays:     p.OffDays,
			ComputeMode: p.ComputeMode,
		}
		var existing models.PoolPreset
		if config.Database.Where("owner_email = ? AND name = ?", owner, preset.Name).First(&existing).Error == nil {
			preset.ID = existing.ID
			preset.CreatedAt = existing.CreatedAt
			config.Database.Save(&preset)
		} else {
			config.Database.Create(&preset)
		}
		return &AnyOutput{Body: map[string]any{"ok": true, "preset": preset}}, nil
	})

	// DELETE /api/pool/presets?id=N — supprime un preset (le sien).
	huma.Register(api, huma.Operation{
		OperationID: "delete-preset", Method: http.MethodDelete, Path: "/api/pool/presets",
		Summary: "Supprimer un preset de pool", Tags: []string{"pool"},
	}, func(ctx context.Context, in *struct {
		ID int `query:"id"`
	}) (*AnyOutput, error) {
		id, _ := identityFrom(ctx)
		if in.ID <= 0 {
			return nil, huma.Error400BadRequest("id requis")
		}
		config.Database.Where("id = ? AND owner_email = ?", in.ID, id.Email).Delete(&models.PoolPreset{})
		return &AnyOutput{Body: map[string]any{"ok": true}}, nil
	})
}
