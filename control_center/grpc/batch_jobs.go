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
			Name      string `json:"name"`
			PoolID    string `json:"pool_id"`
			Script    string `json:"script"`
			Priority  int    `json:"priority"`
			Ephemeral bool   `json:"ephemeral"`
			Nodes     int    `json:"nodes"`
			AutoStop  *bool  `json:"auto_stop"`
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
		prio := req.Priority
		if prio < -1 {
			prio = -1
		} else if prio > 1 {
			prio = 1
		}
		nodes := req.Nodes
		if nodes < 1 {
			nodes = 1
		} else if nodes > 16 {
			nodes = 16
		}
		ephemeral := req.Ephemeral || nodes > 1 // un cluster (>1 nœud) est forcément éphémère
		job := models.BatchJob{
			OwnerEmail: owner, Name: name, PoolID: strings.TrimSpace(req.PoolID),
			Script: req.Script, Priority: prio, Ephemeral: ephemeral, Nodes: nodes,
			Status: "queued", AutoStop: autoStop,
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

// POST /api/jobs/sweep — balayage de paramètres (B3) : crée un job par valeur,
// en injectant la valeur comme variable d'environnement en tête de script.
// {name, pool_id, script, param_name, values[], priority, auto_stop}
func handleBatchJobSweep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONMoodle(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST requis"})
		return
	}
	id, _ := identityFrom(r.Context())
	var req struct {
		Name      string   `json:"name"`
		PoolID    string   `json:"pool_id"`
		Script    string   `json:"script"`
		ParamName string   `json:"param_name"`
		Values    []string `json:"values"`
		Priority  int      `json:"priority"`
		AutoStop  *bool    `json:"auto_stop"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONMoodle(w, http.StatusBadRequest, map[string]string{"error": "JSON invalide"})
		return
	}
	param := strings.TrimSpace(req.ParamName)
	if param == "" || !safeSegment.MatchString(param) {
		writeJSONMoodle(w, http.StatusBadRequest, map[string]string{"error": "nom de paramètre invalide (lettres/chiffres/_-.)"})
		return
	}
	if strings.TrimSpace(req.PoolID) == "" || strings.TrimSpace(req.Script) == "" || len(req.Values) == 0 {
		writeJSONMoodle(w, http.StatusBadRequest, map[string]string{"error": "pool_id, script et au moins une valeur requis"})
		return
	}
	if len(req.Values) > 100 {
		writeJSONMoodle(w, http.StatusBadRequest, map[string]string{"error": "100 valeurs maximum"})
		return
	}
	autoStop := true
	if req.AutoStop != nil {
		autoStop = *req.AutoStop
	}
	prio := req.Priority
	if prio < -1 {
		prio = -1
	} else if prio > 1 {
		prio = 1
	}
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
		// Injecte le paramètre en tête de script (échappé via single-quote bash).
		script := param + "='" + strings.ReplaceAll(v, "'", `'\''`) + "'\nexport " + param + "\n" + req.Script
		job := models.BatchJob{
			OwnerEmail: id.Email, Name: base + " [" + param + "=" + v + "]", PoolID: strings.TrimSpace(req.PoolID),
			Script: script, Priority: prio, Status: "queued", AutoStop: autoStop,
		}
		if config.Database.Create(&job).Error == nil {
			created++
		}
	}
	writeJSONMoodle(w, http.StatusOK, map[string]any{"ok": true, "created": created})
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

// POST /api/jobs/rerun?id=N — relance un job terminé (reprise) : recrée un job
// en file avec les mêmes paramètres (script, pool, priorité, auto-stop).
func handleBatchJobRerun(w http.ResponseWriter, r *http.Request) {
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
	var src models.BatchJob
	q := config.Database.Where("id = ?", jid)
	if id.Role != RoleAdmin {
		q = q.Where("owner_email = ?", id.Email)
	}
	if err := q.First(&src).Error; err != nil {
		writeJSONMoodle(w, http.StatusNotFound, map[string]string{"error": "job introuvable"})
		return
	}
	job := models.BatchJob{
		OwnerEmail: src.OwnerEmail, Name: src.Name, PoolID: src.PoolID,
		Script: src.Script, Priority: src.Priority, AutoStop: src.AutoStop, Status: "queued",
	}
	if err := config.Database.Create(&job).Error; err != nil {
		writeJSONMoodle(w, http.StatusInternalServerError, map[string]string{"error": "relance échouée"})
		return
	}
	writeJSONMoodle(w, http.StatusOK, map[string]any{"ok": true, "job": job})
}
