package monitoring

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"control_center/config"
	"control_center/models"
	"control_center/pb"
)

// Auto-suspend des VMs inactives (Phase 2 — cycle de vie & ressources).
//
// Principe : une VM assignée à un étudiant (locked=true) dont aucune session
// utilisateur n'est ouverte depuis plus que le seuil configuré est suspendue
// pour libérer les ressources. La suspension passe par le même chemin gRPC que
// l'action manuelle (qui pose ManualOff=true côté microservice → le crawler ne
// la relance pas). Une reprise (resume/start) réarme le compteur d'activité.
//
// Désactivé par défaut : il faut AUTOSUSPEND_ENABLED=true pour l'armer.

func autosuspendEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("AUTOSUSPEND_ENABLED")), "true")
}

// idleMinutes : seuil d'inactivité avant suspension (défaut 60 min).
func idleMinutes() int {
	v, err := strconv.Atoi(strings.TrimSpace(os.Getenv("AUTOSUSPEND_IDLE_MINUTES")))
	if err != nil || v <= 0 {
		return 60
	}
	return v
}

// StartAutoSuspend lance la boucle de suspension automatique (no-op si désactivé).
func StartAutoSuspend(ctx context.Context, client pb.PoolManagerClient) {
	if !autosuspendEnabled() {
		log.Println("[autosuspend] désactivé (AUTOSUSPEND_ENABLED != true)")
		return
	}
	log.Printf("[autosuspend] activé — suspension après %d min d'inactivité", idleMinutes())

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sweepIdleVMs(client)
		}
	}
}

// sweepIdleVMs suspend les VMs assignées restées inactives au-delà du seuil.
func sweepIdleVMs(client pb.PoolManagerClient) {
	cutoff := time.Now().UTC().Add(-time.Duration(idleMinutes()) * time.Minute)

	var servers []models.Server
	// On ne cible que les VMs assignées (locked) et joignables : les VMs « prêtes »
	// non attribuées doivent rester disponibles pour une attribution rapide.
	if err := config.Database.Where("locked = true AND ip_address <> ''").Find(&servers).Error; err != nil {
		log.Printf("[autosuspend] lecture des serveurs: %v", err)
		return
	}

	for _, srv := range servers {
		var vm models.VMInstance
		if err := config.Database.Where("name = ?", srv.Name).First(&vm).Error; err != nil {
			continue // pas de signal d'activité connu
		}
		if vm.ActivityStatus != "idle" {
			continue // connectée, déjà suspendue, ou état inconnu
		}
		if vm.LastActive.IsZero() || vm.LastActive.After(cutoff) {
			continue // jamais marquée active, ou active récemment
		}

		if err := suspendServer(client, srv); err != nil {
			log.Printf("[autosuspend] échec suspend %s: %v", srv.Name, err)
			continue
		}

		// Marque l'instance comme suspendue pour ne pas la re-suspendre au prochain tour.
		config.Database.Model(&models.VMInstance{}).
			Where("name = ?", srv.Name).
			Update("activity_status", "suspended")

		// Trace dans le journal d'audit.
		config.Database.Create(&models.AuditLog{
			Actor:  "système (auto-suspend)",
			Role:   "system",
			Method: "AUTO",
			Path:   "/vm/suspend/" + srv.ID,
			IP:     "-",
		})

		log.Printf("[autosuspend] %s suspendue (inactive > %d min)", srv.Name, idleMinutes())
	}
}

// suspendServer demande au microservice de suspendre la VM (même chemin que l'action manuelle).
func suspendServer(client pb.PoolManagerClient, srv models.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.SendRessources(ctx, &pb.RessourceRequest{
		User:   srv.UserID,
		Status: pb.Status_UPDATE,
		Type:   pb.Type_SERVER,
		Data:   map[string]string{"id": srv.ID, "action": "suspend"},
	})
	if err != nil {
		return err
	}
	if resp != nil && !resp.GetSuccess() {
		return fmt.Errorf("le microservice a refusé la suspension")
	}
	return nil
}
