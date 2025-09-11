package utils

import (
	"PoolManagerVM/backend/models"
	"fmt"
)

type PoolKey struct {
	UserID string
	PoolID string
}

func GetAllServerPool() ([]models.ServerPool, error) {
	allServers, err := GetAllServers()
	if err != nil {
		return nil, fmt.Errorf("failed to list all servers: %w", err)
	}

	poolMap := make(map[PoolKey]models.ServerPool)

	for _, s := range allServers {
		userID, ok1 := s.Metadata["userID"]
		poolID, ok2 := s.Metadata["serverpool-id"]

		if !ok1 || !ok2 {
			continue
		}

		key := PoolKey{UserID: userID, PoolID: poolID}
		if _, exists := poolMap[key]; !exists {
			poolMap[key] = models.ServerPool{
				UserID:       userID,
				ServerpoolID: poolID,
				PendingJobs:  0,
				MinVM:        ParseInt(s.Metadata["minVM"]),
				MaxVM:        ParseInt(s.Metadata["maxVM"]),
			}
		}
	}

	var pools []models.ServerPool
	for _, pool := range poolMap {
		pools = append(pools, pool)
	}

	return pools, nil
}
