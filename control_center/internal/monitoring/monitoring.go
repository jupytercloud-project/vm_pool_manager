package monitoring

import (
	"bytes"
	"context"
	"control_center/config"
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
	"gorm.io/gorm"
)

func Start_Monitoring(
	ctx context.Context,
	clientMicroservice pb.PoolManagerClient,
) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	go StartSSHActivityChecker(ctx)

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

// ReapStaleVMs periodically marks VMs without recent heartbeats as dead.
func ReapStaleVMs(ctx context.Context, db *gorm.DB, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			result := db.Exec(`
				UPDATE vm_instances
				SET status = 'dead', healthy = false
				WHERE last_seen < NOW() - INTERVAL '90 seconds'
				AND status != 'dead'
			`)
			if result.RowsAffected > 0 {
				log.Printf("[Reaper] Marked %d stale VMs as dead", result.RowsAffected)
			}
		}
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
	result := config.Database.Model(&models.VMInstance{}).
		Where("name = ?", srv.Name).
		Updates(map[string]any{
			"activity_status": status,
			"last_seen":       now,
		})
	if result.RowsAffected == 0 {
		config.Database.Create(&models.VMInstance{
			ID:             srv.Name,
			Name:           srv.Name,
			IP:             srv.IP_Address,
			Status:         "ready",
			Healthy:        true,
			ActivityStatus: status,
			LastSeen:       now,
			RegisteredAt:   now,
			RawMeta:        json.RawMessage("{}"),
		})
	}
}
