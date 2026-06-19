package grpc

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"control_center/config"
	"control_center/models"
)

// /api/jobs — jobs batch (staff/chercheur). Préfixe protégé via adminHTTPPrefixes.
//
//	GET            → liste les jobs de l'utilisateur (récents d'abord)
//	POST {name, pool_id, script[, auto_stop]} → soumet un job (status queued)
//
// /api/jobs/cancel?id=N (POST) → annule un job en attente.
func handleBatchJobs(w http.ResponseWriter, r *http.Request) {
	id, _ := identityFrom(r.Context())
	owner := id.Email

	switch r.Method {
	case http.MethodGet:
		limit := 100
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
				limit = n
			}
		}
		var jobs []models.BatchJob
		q := config.Database.Order("id DESC").Limit(limit)
		if id.Role != RoleAdmin {
			q = q.Where("owner_email = ?", owner)
		}
		q.Find(&jobs)
		writeJSONMoodle(w, http.StatusOK, map[string]any{"jobs": jobs})

	case http.MethodPost:
		var req struct {
			Name     string `json:"name"`
			PoolID   string `json:"pool_id"`
			Script   string `json:"script"`
			AutoStop *bool  `json:"auto_stop"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONMoodle(w, http.StatusBadRequest, map[string]string{"error": "JSON invalide"})
			return
		}
		if strings.TrimSpace(req.PoolID) == "" || strings.TrimSpace(req.Script) == "" {
			writeJSONMoodle(w, http.StatusBadRequest, map[string]string{"error": "pool_id et script requis"})
			return
		}
		autoStop := true
		if req.AutoStop != nil {
			autoStop = *req.AutoStop
		}
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = "job"
		}
		job := models.BatchJob{
			OwnerEmail: owner, Name: name, PoolID: strings.TrimSpace(req.PoolID),
			Script: req.Script, Status: "queued", AutoStop: autoStop,
		}
		if err := config.Database.Create(&job).Error; err != nil {
			writeJSONMoodle(w, http.StatusInternalServerError, map[string]string{"error": "création du job échouée"})
			return
		}
		writeJSONMoodle(w, http.StatusOK, map[string]any{"ok": true, "job": job})

	default:
		writeJSONMoodle(w, http.StatusMethodNotAllowed, map[string]string{"error": "méthode non autorisée"})
	}
}

// POST /api/jobs/cancel?id=N — annule un job encore en file d'attente.
func handleBatchJobCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONMoodle(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST requis"})
		return
	}
	id, _ := identityFrom(r.Context())
	jid, _ := strconv.Atoi(r.URL.Query().Get("id"))
	if jid <= 0 {
		writeJSONMoodle(w, http.StatusBadRequest, map[string]string{"error": "id requis"})
		return
	}
	q := config.Database.Model(&models.BatchJob{}).Where("id = ? AND status = ?", jid, "queued")
	if id.Role != RoleAdmin {
		q = q.Where("owner_email = ?", id.Email)
	}
	res := q.Update("status", "canceled")
	if res.RowsAffected == 0 {
		writeJSONMoodle(w, http.StatusConflict, map[string]string{"error": "job introuvable ou déjà démarré"})
		return
	}
	writeJSONMoodle(w, http.StatusOK, map[string]any{"ok": true})
}
