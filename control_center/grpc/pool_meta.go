package grpc

import (
	"encoding/json"
	"net/http"
	"strings"

	"control_center/config"
	"control_center/models"
)

// POST /api/pool/meta {pool_id, user_id, label, tags} — définit le nom d'affichage
// et les étiquettes d'un pool. Réservé à l'équipe pédagogique.
func handlePoolMeta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONMoodle(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST requis"})
		return
	}
	var req struct {
		PoolID string `json:"pool_id"`
		UserID string `json:"user_id"`
		Label  string `json:"label"`
		Tags   string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONMoodle(w, http.StatusBadRequest, map[string]string{"error": "JSON invalide"})
		return
	}
	if strings.TrimSpace(req.PoolID) == "" || strings.TrimSpace(req.UserID) == "" {
		writeJSONMoodle(w, http.StatusBadRequest, map[string]string{"error": "pool_id et user_id requis"})
		return
	}
	res := config.Database.Model(&models.Serverpool{}).
		Where("serverpool_id = ? AND user_id = ?", req.PoolID, req.UserID).
		Updates(map[string]any{
			"label": strings.TrimSpace(req.Label),
			"tags":  strings.TrimSpace(req.Tags),
		})
	if res.Error != nil {
		writeJSONMoodle(w, http.StatusInternalServerError, map[string]string{"error": res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		writeJSONMoodle(w, http.StatusNotFound, map[string]string{"error": "pool introuvable"})
		return
	}
	writeJSONMoodle(w, http.StatusOK, map[string]any{"label": strings.TrimSpace(req.Label), "tags": strings.TrimSpace(req.Tags)})
}
