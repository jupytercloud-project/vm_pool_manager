package utils

import (
	"PoolManagerVM/backend/models"
	"fmt"
)

type PoolKey struct {
	UserID string
	PoolID string
}

func GetAllServerPool() ([]models.Serverpool, error) {
	allServers, err := GetAllServers()
	if err != nil {
		return nil, fmt.Errorf("failed to list all servers: %w", err)
	}

	poolMap := make(map[PoolKey]models.Serverpool)

	for _, s := range allServers {
		// models.PrintServer(models.FromGopherServer(s))
		userID, ok1 := s.Metadata["user_id"]
		poolID, ok2 := s.Metadata["serverpool_id"]
		if !ok1 || !ok2 {
			continue
		}

		key := PoolKey{UserID: userID, PoolID: poolID}

		// Récupérer ImageRef et FlavorRef
		var imageID, flavorID string
		if s.Image != nil {
			if id, ok := s.Image["id"].(string); ok {
				imageID = id
			}
		}
		if s.Flavor != nil {
			if id, ok := s.Flavor["id"].(string); ok {
				flavorID = id
			}
		}

		// Construire le param
		param := models.Param{
			ServerpoolID: poolID,
			UserID:       userID,
			MinVM:        ParseInt(s.Metadata["min_vm"]),
			MaxVM:        ParseInt(s.Metadata["max_vm"]),
			PendingJobs:  0,
			ImageRef:     imageID,
			FlavorRef:    flavorID,
			Networks:     models.JSONStringSlice{},
		}

		// Récupérer les IPs
		networks := []string{}
		for _, addrList := range s.Addresses {
			list, ok := addrList.([]interface{})
			if !ok {
				continue
			}
			for _, addr := range list {
				m, ok := addr.(map[string]interface{})
				if !ok {
					continue
				}
				if ip, ok := m["addr"].(string); ok {
					networks = append(networks, ip)
				}
			}
		}
		param.Networks = models.JSONStringSlice(networks)

		// Créer le modèle Server
		serverModel := models.Server{
			ID:           s.ID,
			Name:         s.Name,
			Status:       s.Status,
			FlavorRef:    flavorID,
			ImageRef:     imageID,
			Networks:     models.JSONStringSlice(networks),
			Metadata:     s.Metadata,
			ServerpoolID: poolID,
			UserID:       userID,
		}

		if pool, exists := poolMap[key]; exists {
			// Ajouter param si nécessaire
			found := false
			for _, p := range pool.Params {
				if p.MinVM == param.MinVM &&
					p.MaxVM == param.MaxVM &&
					p.ImageRef == param.ImageRef &&
					p.FlavorRef == param.FlavorRef {
					found = true
					break
				}
			}
			if !found {
				pool.Params = append(pool.Params, param)
			}

			// Ajouter le serveur
			pool.ListServ = append(pool.ListServ, serverModel)
			poolMap[key] = pool
		} else {
			// Premier param et serveur pour ce pool
			poolMap[key] = models.Serverpool{
				ServerpoolID: poolID,
				UserID:       userID,
				Params:       []models.Param{param},
				ListServ:     []models.Server{serverModel},
			}
		}
		// models.PrintServerpool(poolMap[key])
	}

	// Transformer la map en slice
	pools := make([]models.Serverpool, 0, len(poolMap))
	for _, pool := range poolMap {
		pools = append(pools, pool)
	}

	return pools, nil
}
