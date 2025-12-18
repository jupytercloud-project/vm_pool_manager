package monitoring

import (
	"context"
	"control_center/config"
	"control_center/frontcontrolpb"
	"control_center/internal/pool"
	"control_center/models"
	"log"
	"strconv"
	"time"
)

func Start_Monitoring(
	ctx context.Context,
	poolService *pool.Service,
) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Monitoring stopped")
			return
		case <-ticker.C:
			log.Println("Monitoring tick...")
			checkallpools(poolService)
		}
	}
}

func checkallpools(poolService *pool.Service) {
	var pools []models.Serverpool
	err := config.Database.Find(&pools).Error
	if err != nil {
		log.Println("Error fetching server pools:", err)
		return
	}
	for _, pool := range pools {
		checkpool(&pool, poolService)
	}
}

func checkpool(pool *models.Serverpool, poolService *pool.Service) {
	if pool.Status != "scheduled" {
		return
	}
	now := time.Now().UTC()
	if !shouldStartPool(pool, now) {
		return
	}

	log.Printf("Starting pool ID %s as per schedule", pool.ServerpoolID)
	err := config.Database.Model(pool).
		Where("status = ?", "scheduled").
		Update("status", "creating").Error
	if err != nil {
		log.Println("Failed to change pool status:", err)
		return
	}
	go launchCreatePool(pool, poolService)
}

func shouldStartPool(pool *models.Serverpool, now time.Time) bool {
	if pool.TimeStart == nil || pool.Timewindow == nil {
		return false
	}

	startWindow := pool.TimeStart.Add(-30 * time.Minute)
	return now.After(startWindow) && now.Before(*pool.TimeStart)
}

func launchCreatePool(p *models.Serverpool, poolService *pool.Service) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req := &frontcontrolpb.CreatePoolRequest{
		User:    p.UserID,
		Name:    p.ServerpoolID,
		Image:   p.ImageRef,
		Flavor:  p.FlavorRef,
		Network: p.Networks[0],
		MinVm:   strconv.Itoa(p.MinVM),
		MaxVm:   strconv.Itoa(p.MaxVM),
		Config:  p.ConfigID,
	}
	resp, err := poolService.CreatePool(ctx, req)
	if err != nil || !resp.GetSuccess() {
		log.Printf("Failed to create pool ID %s: %v", p.ServerpoolID, err)
		err := config.Database.Model(p).
			Where("status = ?", "creating").
			Update("status", "scheduled").Error
		if err != nil {
			log.Println("Failed to update pool status to error:", err)
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
