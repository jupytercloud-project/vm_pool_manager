package config

import (
	"PoolManagerVM/backend/models"
	"PoolManagerVM/backend/utils"
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"gorm.io/gorm/clause"
)

func Resync_DB(ctx context.Context) {
	for {
		// log.Println("Resync DB")
		syncServers()
		syncPools()
		select {
		case <-ctx.Done():
			log.Println("Resync stopped")
			return
		case <-time.After(3 * time.Second):
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
