package config

import (
	"context"
	"encoding/json"
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

	// Migration
	Database.AutoMigrate(&models.User{}, &models.Serverpool{}, &models.Param{}, &models.Server{})

	allServ, err := utils.GetAllServers()
	if err != nil {
		log.Fatalf("error connexion to OpenStack: %v", err)
	}

	for _, s := range allServ {
		poolID, hasPool := s.Metadata["serverpool-id"]
		userID, hasUser := s.Metadata["userID"]
		if !hasPool || !hasUser {
			continue
		}

		pool := models.Serverpool{
			ServerpoolID: poolID,
			UserID:       userID,
		}

		Database.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "serverpool_id"}, {Name: "user_id"}},
			DoNothing: true,
		}).FirstOrCreate(&pool)

		param := models.Param{
			ServerpoolID: poolID,
			UserID:       userID,
			MinVM:        utils.ParseInt(s.Metadata["minVM"]),
			MaxVM:        utils.ParseInt(s.Metadata["maxVM"]),
			PendingJobs:  0,
			ImageRef:     s.Image["id"].(string),
			FlavorRef:    s.Flavor["id"].(string),
		}

		// Vérifier si un param identique existe déjà
		var existingParams []models.Param
		Database.Where("serverpool_id = ? AND user_id = ?", poolID, userID).Find(&existingParams)

		found := false
		for _, p := range existingParams {
			if p.MinVM == param.MinVM &&
				p.MaxVM == param.MaxVM &&
				p.ImageRef == param.ImageRef &&
				p.FlavorRef == param.FlavorRef {
				found = true
				break
			}
		}

		if !found {
			Database.Create(&param)
		}

		server := models.Server{
			ID:           s.ID,
			Name:         s.Name,
			Status:       s.Status,
			FlavorRef:    fmt.Sprintf("%v", s.Flavor["id"]),
			ImageRef:     fmt.Sprintf("%v", s.Image["id"]),
			ServerpoolID: poolID,
			UserID:       userID,
			Metadata:     s.Metadata,
		}

		// Extraire les IPs depuis Addresses
		networks := []string{}
		for _, addrList := range s.Addresses {
			for _, addr := range addrList.([]interface{}) {
				m := addr.(map[string]interface{})
				if ip, ok := m["addr"].(string); ok {
					networks = append(networks, ip)
				}
			}
		}
		server.Networks = models.JSONStringSlice(networks)

		Database.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "status", "flavor_ref", "image_ref", "serverpool_id", "user_id", "networks", "metadata"}),
		}).Create(&server)

		metadataJSON, err := json.Marshal(server.Metadata)
		if err != nil {
			log.Println("Failed to marshal metadata:", err)
		} else {
			Database.Model(&server).Update("metadata", metadataJSON)
		}

		networksJSON, err := json.Marshal(server.Networks)
		if err != nil {
			log.Println("Failed to marshal networks:", err)
		} else {
			Database.Model(&server).Update("networks", networksJSON)
		}
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
			ID:           s.ID,
			Name:         s.Name,
			Status:       s.Status,
			FlavorRef:    s.Flavor["id"].(string),
			ImageRef:     s.Image["id"].(string),
			ServerpoolID: strings.TrimSpace(s.Metadata["serverpool-id"]),
			UserID:       strings.TrimSpace(s.Metadata["userID"]),
			Metadata:     models.JSONStringMap(s.Metadata),
		}

		// Initialiser slice pour stocker les réseaux
		networks := []string{}

		// Parcourir les réseaux dans s.Addresses
		for _, addrList := range s.Addresses {
			for _, addr := range addrList.([]interface{}) {
				m := addr.(map[string]interface{})
				if ip, ok := m["addr"].(string); ok {
					networks = append(networks, ip)
				}
			}
		}

		networksJSON, _ := json.Marshal(networks)
		json.Unmarshal(networksJSON, &server.Networks)

		if err := Database.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "status", "flavor_ref", "image_ref", "serverpool_id", "user_id", "networks", "metadata"}),
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

	var dbPools []models.Serverpool
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
