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
		userID, ok1 := s.Metadata["userID"]
		poolID, ok2 := s.Metadata["serverpool-id"]
		if !ok1 || !ok2 {
			continue
		}

		key := PoolKey{UserID: userID, PoolID: poolID}

		// Créer le param pour ce serveur
		param := models.Param{
			ServerpoolID: s.Metadata["serverpool-id"],
			UserID:       s.Metadata["userID"],
			MinVM:        ParseInt(s.Metadata["minVM"]),
			MaxVM:        ParseInt(s.Metadata["maxVM"]),
			PendingJobs:  0,
			ImageRef:     s.Image["id"].(string),
			FlavorRef:    s.Flavor["id"].(string),
			Networks:     models.JSONStringSlice{},
		}

		networks := []string{}

		for _, addrList := range s.Addresses {
			for _, addr := range addrList.([]interface{}) {
				m := addr.(map[string]interface{})
				if ip, ok := m["addr"].(string); ok {
					networks = append(networks, ip)
				}
			}
		}

		// Affecter à param.Networks
		param.Networks = models.JSONStringSlice(networks)

		if pool, exists := poolMap[key]; exists {
			// Vérifier si le param existe déjà
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
				// Ajouter le nouveau param
				pool.Params = append(pool.Params, param)
				poolMap[key] = pool
			}

		} else {
			// Premier param pour ce pool
			poolMap[key] = models.Serverpool{
				ServerpoolID: poolID,
				UserID:       userID,
				Params:       []models.Param{param},
			}
		}
	}

	// Transformer la map en slice
	pools := make([]models.Serverpool, 0, len(poolMap))
	for _, pool := range poolMap {
		pools = append(pools, pool)
	}

	return pools, nil
}
