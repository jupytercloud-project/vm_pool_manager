package config

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"PoolManagerVM/backend/models"
	"PoolManagerVM/backend/utils"
)

var Database *gorm.DB

func Sync_DB() {
	var err error
	Database, err = gorm.Open(sqlite.Open("PoolManagerVM.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	Database.AutoMigrate(&models.User{})
	Database.AutoMigrate(&models.Server{})
	Database.AutoMigrate(&models.ServerPool{})

	allserv, err := utils.GetAllServers()
	if err != nil {
		log.Fatalf("error connexion to openstack")
	}

	for _, s := range allserv {
		pool := models.ServerPool{}

		poolID, hasPool := s.Metadata["serverpool-id"]
		userID, hasUser := s.Metadata["userID"]

		if hasPool && hasUser {
			pool = models.ServerPool{
				ServerpoolID: poolID,
				UserID:       userID,
				MinVM:        utils.ParseInt(s.Metadata["minVM"]),
				MaxVM:        utils.ParseInt(s.Metadata["maxVM"]),
				PendingJobs:  0,
			}

			Database.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "serverpool_id"}, {Name: "user_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"min_vm", "max_vm", "pending_jobs"}),
			}).FirstOrCreate(&pool)
		}

		server := models.Server{
			ID:        s.ID,
			Name:      s.Name,
			Status:    s.Status,
			FlavorRef: fmt.Sprintf("%v", s.Flavor["id"]),
			ImageRef:  fmt.Sprintf("%v", s.Image["id"]),
		}

		if pool.ID != 0 {
			server.PoolID = &pool.ID
		}
		Database.Save(&server)
	}
}

func Resync_DB(ctx context.Context) {
	for {
		log.Println("Resync DB")
		syncServers()
		syncPools()
		select {
		case <-ctx.Done():
			log.Println("Resync stopped")
			return
		case <-time.After(30 * time.Second):
			//next cycle
		}
	}
}

func syncServers() {
	allServ, err := utils.GetAllServers()
	if err != nil {
		log.Println("Error fetching servers from OpenStack:", err)
		return
	}

	existingServerIDs := make(map[string]struct{})

	for _, s := range allServ {
		server := models.Server{
			ID:        s.ID,
			Name:      s.Name,
			Status:    s.Status,
			FlavorRef: s.Flavor["id"].(string),
			ImageRef:  s.Image["id"].(string),
		}

		var linkedPool models.ServerPool
		if err := Database.Where("serverpool_id = ? AND user_id = ?", strings.TrimSpace(s.Metadata["serverpool-id"]), strings.TrimSpace(s.Metadata["userID"])).First(&linkedPool).Error; err != nil {
			server.PoolID = &linkedPool.ID
		}

		if err := Database.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "status", "flavor_id", "image_id", "pool_id"}),
		}).Create(&server).Error; err != nil {
			log.Println("Error create/update server:", err)
			continue
		}
		existingServerIDs[s.ID] = struct{}{}

	}
	var dbServers []models.Server
	if err := Database.Find(&dbServers).Error; err != nil {
		log.Println("Error fetching server DB:", err)
		return
	}

	for _, s := range dbServers {
		if _, ok := existingServerIDs[s.ID]; !ok {
			log.Println("Server ", s.ID, " not in Openstack, delete")
			Database.Delete(&s)
		}
	}
}

func syncPools() {
	allPool, err := utils.GetAllServerPool()
	if err != nil {
		log.Println("Error fetching pools from OpenStack:", err)
		return
	}

	existingPoolKeys := make(map[utils.PoolKey]struct{})

	for _, p := range allPool {
		key := utils.PoolKey{
			UserID: strings.TrimSpace(p.UserID),
			PoolID: strings.TrimSpace(p.ServerpoolID),
		}

		if err := Database.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "serverpool_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"min_vm", "max_vm", "pending_jobs"}),
		}).Create(&p).Error; err != nil {
			log.Println("Error create/update pool:", err)
			continue
		}

		existingPoolKeys[key] = struct{}{}
	}

	var dbPools []models.ServerPool
	if err := Database.Find(&dbPools).Error; err != nil {
		log.Println("Error fetching pools from DB:", err)
		return
	}

	for _, p := range dbPools {
		key := utils.PoolKey{
			UserID: strings.TrimSpace(p.UserID),
			PoolID: strings.TrimSpace(p.ServerpoolID),
		}
		if _, ok := existingPoolKeys[key]; !ok {
			log.Println("Pool", key, " not in OpenStack, delete")
			if err := Database.Delete(&p).Error; err != nil {
				log.Println("Error deleting pool:", err)
			}
		}
	}
}
