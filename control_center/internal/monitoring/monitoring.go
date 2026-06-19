package monitoring

import (
	"bytes"
	"context"
	"control_center/config"
	"control_center/internal/guacamole"
	"control_center/internal/sshinject"
	"control_center/models"
	"control_center/pb"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

func Start_Monitoring(
	ctx context.Context,
	clientMicroservice pb.PoolManagerClient,
	guacClient *guacamole.Client,
) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	go StartSSHActivityChecker(ctx)
	go StartAutoSuspend(ctx, clientMicroservice)
	go StartUsageAccounting(ctx)
	go StartBatchRunner(ctx, clientMicroservice)
	if guacClient != nil {
		go guacamoleSyncLoop(ctx, guacClient)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Monitoring stopped")
			return
		case <-ticker.C:
			checkallpool(clientMicroservice)
		}
	}
}

func checkallpool(client pb.PoolManagerClient) {
	var pools []models.Serverpool
	err := config.Database.Find(&pools).Error
	if err != nil {
		log.Println("Error fetching server pools:", err)
		return
	}
	for _, pool := range pools {
		checkpool(&pool, client)
	}
}

func hasActiveSchedule(pool *models.Serverpool) bool {
	return pool.TimeStart != nil &&
		pool.Timewindow != nil &&
		*pool.Timewindow > 0 &&
		!pool.TimeStart.IsZero()
}

func checkpool(pool *models.Serverpool, client pb.PoolManagerClient) {
	now := time.Now().UTC()

	// Pools créés sans planning peuvent avoir des valeurs résiduelles en base.
	if !hasActiveSchedule(pool) && (pool.TimeStart != nil || pool.Timewindow != nil) {
		if err := config.Database.Model(pool).Updates(map[string]any{
			"time_start": nil,
			"timewindow": nil,
		}).Error; err != nil {
			log.Printf("Failed to clear stale schedule for pool %s: %v", pool.ServerpoolID, err)
		} else {
			pool.TimeStart = nil
			pool.Timewindow = nil
		}
	}

	switch pool.Status {
	case "scheduled":
		if !hasActiveSchedule(pool) {
			// Planning résiduel supprimé : repasser en running pour que le crawler provisionne les VMs.
			if err := config.Database.Model(pool).
				Where("status = ?", "scheduled").
				Update("status", "running").Error; err != nil {
				log.Printf("Failed to recover pool %s to running: %v", pool.ServerpoolID, err)
			}
			return
		}
		if shouldStartPool(pool, now) {
			startPool(pool, client)
		}
	case "idle":
		if hasActiveSchedule(pool) && shouldStartPool(pool, now) {
			startPool(pool, client)
		}
	case "running":
		if hasActiveSchedule(pool) && shouldDeletePool(pool, now) {
			deletePool(pool, client)
		}
	}
}

func startPool(pool *models.Serverpool, client pb.PoolManagerClient) {
	log.Printf("Starting pool ID %s as per schedule", pool.ServerpoolID)
	err := config.Database.Model(pool).
		Where("status = ?", "scheduled").
		Update("status", "creating").Error
	if err != nil {
		log.Println("Failed to change pool status:", err)
		return
	}
	go launchCreatePool(pool, client)
}

func shouldDeletePool(pool *models.Serverpool, now time.Time) bool {
	if !hasActiveSchedule(pool) {
		return false
	}

	endTime := pool.TimeStart.Add(*pool.Timewindow)
	return now.After(endTime)
}

func deletePool(pool *models.Serverpool, client pb.PoolManagerClient) {
	log.Printf("Deleting pool ID %s as per schedule", pool.ServerpoolID)
	err := config.Database.Model(pool).
		Where("status = ?", "running").
		Update("status", "deleting").Error
	if err != nil {
		log.Println("Failed to change pool status:", err)
		return
	}

	go launchDeletePool(pool, client)
}

func shouldStartPool(pool *models.Serverpool, now time.Time) bool {
	if pool.TimeStart == nil || pool.Timewindow == nil {
		return false
	}

	startWindow := pool.TimeStart.Add(-30 * time.Minute)
	return now.After(startWindow) && now.Before(*pool.TimeStart)
}

func launchCreatePool(p *models.Serverpool, client pb.PoolManagerClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	rep, err := client.SendRessources(
		ctx,
		&pb.RessourceRequest{
			User:   p.UserID,
			Data:   p.ToMap(),
			Status: pb.Status_CREATE,
			Type:   pb.Type_SERVERPOOL,
		},
	)
	if err != nil || rep.GetSuccess() == false {
		log.Println("Error on creating pool as planned")
		err := config.Database.Model(p).
			Where("status = ?", "creating").
			Update("status", "schedlued").Error
		if err != nil {
			log.Println("Failed to update pool status:", err)
		}
		return
	}
	log.Printf("Pool ID %s created successfully", p.ServerpoolID)
	err = config.Database.Model(p).
		Where("status = ?", "creating").
		Update("status", "running").Error
	if err != nil {
		log.Println("Failed to update pool status to running:", err)
	}
}

func launchDeletePool(p *models.Serverpool, client pb.PoolManagerClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	rep, err := client.SendRessources(
		ctx,
		&pb.RessourceRequest{
			User:   p.UserID,
			Data:   p.ToMap(),
			Status: pb.Status_DELETE,
			Type:   pb.Type_SERVERPOOL,
		},
	)
	if err != nil || rep.GetSuccess() == false {
		log.Println("Error on deleting pool as planned")
		err := config.Database.Model(p).
			Where("status = ?", "deleting").
			Update("status", "running").Error
		if err != nil {
			log.Println("Failed to update pool status:", err)
		}
		return
	}
	log.Printf("Pool ID %s deleted successfully", p.ServerpoolID)

	updates := map[string]any{"status": "idle"}
	if hasActiveSchedule(p) {
		var nextTimeStart *time.Time
		if p.TimeStart != nil {
			t := p.TimeStart.AddDate(0, 0, 7)
			nextTimeStart = &t
		}
		updates["status"] = "scheduled"
		updates["time_start"] = nextTimeStart
	}

	err = config.Database.Model(p).
		Where("status = ?", "deleting").
		Updates(updates).Error
	if err != nil {
		log.Println("Failed to update pool status:", err)
	}
}

func guacamoleSyncLoop(ctx context.Context, client *guacamole.Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			syncGuacamoleRegistrations(client)
			cleanupDeadGuacamoleConnections(client)
		}
	}
}

func syncGuacamoleRegistrations(client *guacamole.Client) {
	// Enregistrer les VMs présentes dans vm_instances mais pas encore dans Guacamole
	var vms []models.VMInstance
	config.Database.Where("guac_connection_id = '' AND ip <> '' AND status = 'ready'").Find(&vms)
	for _, vm := range vms {
		connID, err := client.CreateSSHConnection(vm.Name, vm.IP)
		if err != nil {
			log.Printf("[guac] register vm_instance %s: %v", vm.Name, err)
			continue
		}
		config.Database.Model(&vm).Update("guac_connection_id", connID)
		log.Printf("[guac] registered %s -> conn %s", vm.Name, connID)
	}

	// Enregistrer les VMs ACTIVE dans servers qui n'ont pas de ligne vm_instances
	var servers []models.Server
	config.Database.Where("status = 'ACTIVE' AND ip_address <> ''").Find(&servers)
	for _, srv := range servers {
		// Vérifier si une ligne vm_instances existe déjà avec un guac_connection_id
		var existing models.VMInstance
		if err := config.Database.Where("name = ?", srv.Name).First(&existing).Error; err == nil {
			if existing.GuacConnectionID != "" {
				continue // déjà enregistré
			}
			// Ligne existe mais pas de connexion guac
			connID, err := client.CreateSSHConnection(srv.Name, srv.IP_Address)
			if err != nil {
				log.Printf("[guac] register server %s: %v", srv.Name, err)
				continue
			}
			config.Database.Model(&existing).Updates(map[string]any{
				"guac_connection_id": connID,
				"last_seen":          time.Now().UTC(),
			})
			log.Printf("[guac] registered server %s -> conn %s", srv.Name, connID)
		} else {
			// Pas de ligne vm_instances — créer une entrée minimale avec le guac_connection_id
			connID, err := client.CreateSSHConnection(srv.Name, srv.IP_Address)
			if err != nil {
				log.Printf("[guac] register new server %s: %v", srv.Name, err)
				continue
			}
			meta, _ := json.Marshal(map[string]string{
				"serverpool_id": srv.ServerpoolID,
				"user_id":       srv.UserID,
			})
			now := time.Now().UTC()
			config.Database.Create(&models.VMInstance{
				ID:               srv.ID,
				Name:             srv.Name,
				IP:               srv.IP_Address,
				Status:           "ready",
				Healthy:          true,
				ActivityStatus:   "idle",
				RegisteredAt:     now,
				LastSeen:         now,
				LastActive:       now,
				RawMeta:          meta,
				GuacConnectionID: connID,
			})
			log.Printf("[guac] created vm_instance + registered %s -> conn %s", srv.Name, connID)
		}
	}
}

func cleanupDeadGuacamoleConnections(client *guacamole.Client) {
	var vms []models.VMInstance
	config.Database.Where("status = 'dead' AND guac_connection_id <> ''").Find(&vms)
	for _, vm := range vms {
		if err := client.DeleteConnection(vm.GuacConnectionID); err != nil {
			log.Printf("[guac] delete conn %s: %v", vm.GuacConnectionID, err)
		}
		config.Database.Model(&vm).Update("guac_connection_id", "")
	}
}

func StartSSHActivityChecker(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkSSHActivity()
		}
	}
}

func checkSSHActivity() {
	keyPath := os.Getenv("SSH_PRIVATE_KEY_PATH")
	if keyPath == "" {
		return
	}
	signer, err := sshinject.LoadPrivateKey(keyPath)
	if err != nil {
		return
	}

	var servers []models.Server
	if err := config.Database.Where("ip_address <> '' AND locked = true").Find(&servers).Error; err != nil {
		return
	}

	for _, srv := range servers {
		go checkOneVM(srv, signer)
	}
}

func checkOneVM(srv models.Server, signer ssh.Signer) {
	cfg := &ssh.ClientConfig{
		User:            "vmuser",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	addr := fmt.Sprintf("%s:22", srv.IP_Address)
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return
	}
	defer session.Close()

	var stdout bytes.Buffer
	session.Stdout = &stdout
	if err := session.Run("who | wc -l"); err != nil {
		return
	}

	count := strings.TrimSpace(stdout.String())
	status := "idle"
	if count != "0" && count != "" {
		status = "connected"
	}

	now := time.Now().UTC()
	updates := map[string]any{
		"activity_status": status,
		"last_seen":       now,
	}
	// last_active ne bouge QUE sur activité réelle : c'est ce qui mesure l'inactivité.
	if status == "connected" {
		updates["last_active"] = now
	}
	result := config.Database.Model(&models.VMInstance{}).
		Where("name = ?", srv.Name).
		Updates(updates)
	if result.RowsAffected == 0 {
		config.Database.Create(&models.VMInstance{
			ID:             srv.Name,
			Name:           srv.Name,
			IP:             srv.IP_Address,
			Status:         "ready",
			Healthy:        true,
			ActivityStatus: status,
			LastSeen:       now,
			LastActive:     now,
			RegisteredAt:   now,
			RawMeta:        json.RawMessage("{}"),
		})
	}
}
