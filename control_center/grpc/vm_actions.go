package grpc

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"control_center/config"
	"control_center/models"
	"control_center/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var validVMActions = map[string]bool{
	"start": true, "stop": true, "suspend": true, "resume": true, "reboot": true,
}

// POST /api/vm/action {server_id, action} — pilote le cycle de vie d'une VM
// (start/stop/suspend/resume/reboot) via le microservice. Réservé à l'équipe pédagogique.
func handleVMAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONMoodle(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST requis"})
		return
	}
	var req struct {
		ServerID string `json:"server_id"`
		Action   string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONMoodle(w, http.StatusBadRequest, map[string]string{"error": "JSON invalide"})
		return
	}
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	if strings.TrimSpace(req.ServerID) == "" || !validVMActions[req.Action] {
		writeJSONMoodle(w, http.StatusBadRequest, map[string]string{
			"error": "server_id et action valides requis (start|stop|suspend|resume|reboot)"})
		return
	}

	id, _ := identityFrom(r.Context())

	conn, err := grpc.NewClient("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		writeJSONMoodle(w, http.StatusBadGateway, map[string]string{"error": "microservice injoignable"})
		return
	}
	defer conn.Close()
	client := pb.NewPoolManagerClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := client.SendRessources(ctx, &pb.RessourceRequest{
		User:   id.Email,
		Status: pb.Status_UPDATE,
		Type:   pb.Type_SERVER,
		Data:   map[string]string{"id": req.ServerID, "action": req.Action},
	})
	if err != nil || (resp != nil && !resp.GetSuccess()) {
		msg := "échec de l'action"
		if err != nil {
			msg = err.Error()
		}
		writeJSONMoodle(w, http.StatusBadGateway, map[string]string{"error": msg})
		return
	}
	invalidatePowerStates() // refléter le nouvel état au prochain inventaire
	writeJSONMoodle(w, http.StatusOK, map[string]any{
		"server_id": req.ServerID, "action": req.Action, "ok": true})
}

// POST /api/vm/rebuild {server_id} — réinitialise (rebuild) une VM sur son image d'origine.
// DESTRUCTIF : réinstalle le système (les données de la VM sont perdues). Staff uniquement.
// Réplique la logique de PoolService.RebuildServer (réutilise l'image stockée du serveur).
func handleVMRebuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONMoodle(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST requis"})
		return
	}
	var req struct {
		ServerID string `json:"server_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.ServerID) == "" {
		writeJSONMoodle(w, http.StatusBadRequest, map[string]string{"error": "server_id requis"})
		return
	}

	var server models.Server
	if err := config.Database.Where("id = ?", req.ServerID).First(&server).Error; err != nil {
		writeJSONMoodle(w, http.StatusNotFound, map[string]string{"error": "VM introuvable"})
		return
	}

	conn, err := grpc.NewClient("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		writeJSONMoodle(w, http.StatusBadGateway, map[string]string{"error": "microservice injoignable"})
		return
	}
	defer conn.Close()
	client := pb.NewPoolManagerClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := client.SendRessources(ctx, &pb.RessourceRequest{
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
		writeJSONMoodle(w, http.StatusBadGateway, map[string]string{"error": msg})
		return
	}
	invalidatePowerStates()
	writeJSONMoodle(w, http.StatusOK, map[string]any{"server_id": req.ServerID, "ok": true})
}
