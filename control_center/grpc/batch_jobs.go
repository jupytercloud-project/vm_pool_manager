package grpc

import (
	"context"
	"net/http"
	"strings"

	"control_center/config"
	"control_center/models"

	"github.com/danielgtaylor/huma/v2"
)

// registerJobsHuma enregistre les endpoints /api/jobs* (jobs batch, staff/chercheur).
func registerJobsHuma(api huma.API) {
	// GET /api/jobs — liste les jobs (les siens, ou tous pour un admin), récents d'abord.
	huma.Register(api, huma.Operation{
		OperationID: "list-jobs", Method: http.MethodGet, Path: "/api/jobs",
		Summary: "Lister les jobs batch", Tags: []string{"jobs"},
	}, func(ctx context.Context, in *struct {
		Limit int `query:"limit"`
	}) (*AnyOutput, error) {
		id, _ := identityFrom(ctx)
		limit := 100
		if in.Limit > 0 && in.Limit <= 500 {
			limit = in.Limit
		}
		var jobs []models.BatchJob
		q := config.Database.Order("id DESC").Limit(limit)
		if id.Role != RoleAdmin {
			q = q.Where("owner_email = ?", id.Email)
		}
		q.Find(&jobs)
		return &AnyOutput{Body: map[string]any{"jobs": jobs}}, nil
	})

	// POST /api/jobs — soumet un job.
	huma.Register(api, huma.Operation{
		OperationID: "create-job", Method: http.MethodPost, Path: "/api/jobs",
		Summary: "Soumettre un job batch", Tags: []string{"jobs"},
	}, func(ctx context.Context, in *struct {
		Body struct {
			Name      string `json:"name"`
			PoolID    string `json:"pool_id"`
			Script    string `json:"script"`
			Priority  int    `json:"priority"`
			Ephemeral bool   `json:"ephemeral"`
			Nodes     int    `json:"nodes"`
			AutoStop  *bool  `json:"auto_stop"`
		}
	}) (*AnyOutput, error) {
		id, _ := identityFrom(ctx)
		req := in.Body
		if strings.TrimSpace(req.PoolID) == "" || strings.TrimSpace(req.Script) == "" {
			return nil, huma.Error400BadRequest("pool_id et script requis")
		}
		// Anti-IDOR : on ne soumet un job (exécution de script sur une VM du pool) que sur un
		// pool dont on est propriétaire (staff : n'importe lequel).
		if !poolOwnedByCallerOrStaff(ctx, req.PoolID) {
			return nil, huma.Error403Forbidden("ce pool ne vous appartient pas")
		}
		autoStop := true
		if req.AutoStop != nil {
			autoStop = *req.AutoStop
		}
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = "job"
		}
		prio := clampPriority(req.Priority)
		nodes := req.Nodes
		if nodes < 1 {
			nodes = 1
		} else if nodes > 16 {
			nodes = 16
		}
		ephemeral := req.Ephemeral || nodes > 1 // un cluster (>1 nœud) est forcément éphémère
		job := models.BatchJob{
			OwnerEmail: id.Email, Name: name, PoolID: strings.TrimSpace(req.PoolID),
			Script: req.Script, Priority: prio, Ephemeral: ephemeral, Nodes: nodes,
			Status: "queued", AutoStop: autoStop,
		}
		if err := config.Database.Create(&job).Error; err != nil {
			return nil, huma.Error500InternalServerError("création du job échouée")
		}
		return &AnyOutput{Body: map[string]any{"ok": true, "job": job}}, nil
	})

	// POST /api/jobs/sweep — balayage de paramètres (un job par valeur).
	huma.Register(api, huma.Operation{
		OperationID: "sweep-jobs", Method: http.MethodPost, Path: "/api/jobs/sweep",
		Summary: "Balayage de paramètres", Tags: []string{"jobs"},
	}, func(ctx context.Context, in *struct {
		Body struct {
			Name      string   `json:"name"`
			PoolID    string   `json:"pool_id"`
			Script    string   `json:"script"`
			ParamName string   `json:"param_name"`
			Values    []string `json:"values"`
			Priority  int      `json:"priority"`
			AutoStop  *bool    `json:"auto_stop"`
		}
	}) (*AnyOutput, error) {
		id, _ := identityFrom(ctx)
		req := in.Body
		param := strings.TrimSpace(req.ParamName)
		if param == "" || !safeSegment.MatchString(param) {
			return nil, huma.Error400BadRequest("nom de paramètre invalide (lettres/chiffres/_-.)")
		}
		if strings.TrimSpace(req.PoolID) == "" || strings.TrimSpace(req.Script) == "" || len(req.Values) == 0 {
			return nil, huma.Error400BadRequest("pool_id, script et au moins une valeur requis")
		}
		if len(req.Values) > 100 {
			return nil, huma.Error400BadRequest("100 valeurs maximum")
		}
		// Anti-IDOR : balayage uniquement sur un pool dont on est propriétaire (staff : tous).
		if !poolOwnedByCallerOrStaff(ctx, req.PoolID) {
			return nil, huma.Error403Forbidden("ce pool ne vous appartient pas")
		}
		autoStop := true
		if req.AutoStop != nil {
			autoStop = *req.AutoStop
		}
		prio := clampPriority(req.Priority)
		base := strings.TrimSpace(req.Name)
		if base == "" {
			base = "sweep"
		}
		created := 0
		for _, v := range req.Values {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			script := param + "='" + strings.ReplaceAll(v, "'", `'\''`) + "'\nexport " + param + "\n" + req.Script
			job := models.BatchJob{
				OwnerEmail: id.Email, Name: base + " [" + param + "=" + v + "]", PoolID: strings.TrimSpace(req.PoolID),
				Script: script, Priority: prio, Status: "queued", AutoStop: autoStop,
			}
			if config.Database.Create(&job).Error == nil {
				created++
			}
		}
		return &AnyOutput{Body: map[string]any{"ok": true, "created": created}}, nil
	})

	// POST /api/jobs/cancel?id=N — annule un job en file d'attente.
	huma.Register(api, huma.Operation{
		OperationID: "cancel-job", Method: http.MethodPost, Path: "/api/jobs/cancel",
		Summary: "Annuler un job en file", Tags: []string{"jobs"},
	}, func(ctx context.Context, in *struct {
		ID int `query:"id"`
	}) (*AnyOutput, error) {
		id, _ := identityFrom(ctx)
		if in.ID <= 0 {
			return nil, huma.Error400BadRequest("id requis")
		}
		q := config.Database.Model(&models.BatchJob{}).Where("id = ? AND status = ?", in.ID, "queued")
		if id.Role != RoleAdmin {
			q = q.Where("owner_email = ?", id.Email)
		}
		if res := q.Update("status", "canceled"); res.RowsAffected == 0 {
			return nil, huma.Error409Conflict("job introuvable ou déjà démarré")
		}
		return &AnyOutput{Body: map[string]any{"ok": true}}, nil
	})

	// POST /api/jobs/rerun?id=N — relance un job terminé.
	huma.Register(api, huma.Operation{
		OperationID: "rerun-job", Method: http.MethodPost, Path: "/api/jobs/rerun",
		Summary: "Relancer un job", Tags: []string{"jobs"},
	}, func(ctx context.Context, in *struct {
		ID int `query:"id"`
	}) (*AnyOutput, error) {
		id, _ := identityFrom(ctx)
		if in.ID <= 0 {
			return nil, huma.Error400BadRequest("id requis")
		}
		var src models.BatchJob
		q := config.Database.Where("id = ?", in.ID)
		if id.Role != RoleAdmin {
			q = q.Where("owner_email = ?", id.Email)
		}
		if err := q.First(&src).Error; err != nil {
			return nil, huma.Error404NotFound("job introuvable")
		}
		job := models.BatchJob{
			OwnerEmail: src.OwnerEmail, Name: src.Name, PoolID: src.PoolID,
			Script: src.Script, Priority: src.Priority, AutoStop: src.AutoStop, Status: "queued",
		}
		if err := config.Database.Create(&job).Error; err != nil {
			return nil, huma.Error500InternalServerError("relance échouée")
		}
		return &AnyOutput{Body: map[string]any{"ok": true, "job": job}}, nil
	})
}

func clampPriority(p int) int {
	if p < -1 {
		return -1
	}
	if p > 1 {
		return 1
	}
	return p
}
