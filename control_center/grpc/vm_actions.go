package grpc

import (
	"context"
	"net/http"
	"strings"
	"time"

	"control_center/config"
	"control_center/models"
	"control_center/pb"

	"github.com/danielgtaylor/huma/v2"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var validVMActions = map[string]bool{
	"start": true, "stop": true, "suspend": true, "resume": true, "reboot": true,
}

// registerVMHuma enregistre /api/vm/* (cycle de vie des VMs, staff uniquement).
func registerVMHuma(api huma.API) {
	// POST /api/vm/action {server_id, action} — start/stop/suspend/resume/reboot.
	huma.Register(api, huma.Operation{
		OperationID: "vm-action", Method: http.MethodPost, Path: "/api/vm/action",
		Summary: "Piloter le cycle de vie d'une VM", Tags: []string{"vm"},
	}, func(ctx context.Context, in *struct {
		Body struct {
			ServerID string `json:"server_id"`
			Action   string `json:"action"`
		}
	}) (*AnyOutput, error) {
		action := strings.ToLower(strings.TrimSpace(in.Body.Action))
		if strings.TrimSpace(in.Body.ServerID) == "" || !validVMActions[action] {
			return nil, huma.Error400BadRequest("server_id et action valides requis (start|stop|suspend|resume|reboot)")
		}
		id, _ := identityFrom(ctx)

		conn, err := grpc.NewClient("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
		if err != nil {
			return nil, huma.Error502BadGateway("microservice injoignable")
		}
		defer conn.Close()
		client := pb.NewPoolManagerClient(conn)

		rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		resp, err := client.SendRessources(rctx, &pb.RessourceRequest{
			User:   id.Email,
			Status: pb.Status_UPDATE,
			Type:   pb.Type_SERVER,
			Data:   map[string]string{"id": in.Body.ServerID, "action": action},
		})
		if err != nil || (resp != nil && !resp.GetSuccess()) {
			msg := "échec de l'action"
			if err != nil {
				msg = err.Error()
			}
			return nil, huma.Error502BadGateway(msg)
		}
		invalidatePowerStates() // refléter le nouvel état au prochain inventaire

		// Réarmer le compteur d'activité à la reprise pour éviter une re-suspension immédiate.
		if action == "resume" || action == "start" {
			var srv models.Server
			if config.Database.Where("id = ?", in.Body.ServerID).First(&srv).Error == nil {
				config.Database.Model(&models.VMInstance{}).
					Where("name = ?", srv.Name).
					Updates(map[string]any{
						"activity_status": "connected",
						"last_active":     time.Now().UTC(),
					})
			}
		}

		return &AnyOutput{Body: map[string]any{
			"server_id": in.Body.ServerID, "action": action, "ok": true}}, nil
	})

	// POST /api/vm/rebuild {server_id} — réinitialise (rebuild) une VM sur son image d'origine.
	huma.Register(api, huma.Operation{
		OperationID: "vm-rebuild", Method: http.MethodPost, Path: "/api/vm/rebuild",
		Summary: "Réinitialiser (rebuild) une VM", Tags: []string{"vm"},
	}, func(ctx context.Context, in *struct {
		Body struct {
			ServerID string `json:"server_id"`
		}
	}) (*AnyOutput, error) {
		if strings.TrimSpace(in.Body.ServerID) == "" {
			return nil, huma.Error400BadRequest("server_id requis")
		}
		var server models.Server
		if err := config.Database.Where("id = ?", in.Body.ServerID).First(&server).Error; err != nil {
			return nil, huma.Error404NotFound("VM introuvable")
		}

		conn, err := grpc.NewClient("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
		if err != nil {
			return nil, huma.Error502BadGateway("microservice injoignable")
		}
		defer conn.Close()
		client := pb.NewPoolManagerClient(conn)

		rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		resp, err := client.SendRessources(rctx, &pb.RessourceRequest{
			User:   server.UserID,
			Status: pb.Status_UPDATE,
			Type:   pb.Type_SERVER,
			Data:   server.ToMap(), // image_ref + name + id → le microservice fait un rebuild
		})
		if err != nil || (resp != nil && !resp.GetSuccess()) {
			msg := "échec de la réinitialisation"
			if err != nil {
				msg = err.Error()
			}
			return nil, huma.Error502BadGateway(msg)
		}
		invalidatePowerStates()
		return &AnyOutput{Body: map[string]any{"server_id": in.Body.ServerID, "ok": true}}, nil
	})

	// POST /api/vm/resize {server_id, flavor_ref} — change le flavor (gabarit) d'une VM.
	huma.Register(api, huma.Operation{
		OperationID: "vm-resize", Method: http.MethodPost, Path: "/api/vm/resize",
		Summary: "Redimensionner une VM (flavor)", Tags: []string{"vm"},
	}, func(ctx context.Context, in *struct {
		Body struct {
			ServerID  string `json:"server_id"`
			FlavorRef string `json:"flavor_ref"`
		}
	}) (*AnyOutput, error) {
		if strings.TrimSpace(in.Body.ServerID) == "" || strings.TrimSpace(in.Body.FlavorRef) == "" {
			return nil, huma.Error400BadRequest("server_id et flavor_ref requis")
		}
		var server models.Server
		if err := config.Database.Where("id = ?", in.Body.ServerID).First(&server).Error; err != nil {
			return nil, huma.Error404NotFound("VM introuvable")
		}

		conn, err := grpc.NewClient("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
		if err != nil {
			return nil, huma.Error502BadGateway("microservice injoignable")
		}
		defer conn.Close()
		client := pb.NewPoolManagerClient(conn)

		rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		resp, err := client.SendRessources(rctx, &pb.RessourceRequest{
			User:   server.UserID,
			Status: pb.Status_UPDATE,
			Type:   pb.Type_SERVER,
			Data:   map[string]string{"id": in.Body.ServerID, "action": "resize", "flavor_ref": in.Body.FlavorRef},
		})
		if err != nil || (resp != nil && !resp.GetSuccess()) {
			msg := "échec du redimensionnement"
			if err != nil {
				msg = err.Error()
			}
			return nil, huma.Error502BadGateway(msg)
		}

		// Reflet optimiste : l'inventaire affichera le nouveau gabarit (le resize réel suit en job).
		config.Database.Model(&models.Server{}).Where("id = ?", in.Body.ServerID).Update("flavor_ref", in.Body.FlavorRef)
		invalidatePowerStates()
		return &AnyOutput{Body: map[string]any{"server_id": in.Body.ServerID, "flavor_ref": in.Body.FlavorRef, "ok": true}}, nil
	})
}
