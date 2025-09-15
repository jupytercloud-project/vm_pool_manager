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

	poolMap := make(map[PoolKey]*models.Serverpool)

	for _, ops := range allServers {
		s := models.FromGopherServer(ops)

		key := PoolKey{UserID: s.UserID, PoolID: s.ServerpoolID}
		if _, exists := poolMap[key]; !exists {
			poolMap[key] = &models.Serverpool{
				UserID:       s.UserID,
				ServerpoolID: s.ServerpoolID,
				ImageRef:     s.ImageRef,
				FlavorRef:    s.FlavorRef,
				MinVM:        ParseInt(s.Metadata["min_vm"]),
				MaxVM:        ParseInt(s.Metadata["max_vm"]),
				Networks:     s.Networks,
				ListServ:     []models.Server{s},
			}
		} else {
			poolMap[key].ListServ = append(poolMap[key].ListServ, s)
		}
	}

	pools := make([]models.Serverpool, 0, len(poolMap))
	for _, p := range poolMap {
		pools = append(pools, *p)
	}
	return pools, nil
}
