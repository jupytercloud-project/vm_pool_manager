package utils

import (
	"PoolManagerVM/backend/models"
	"fmt"
)

type PoolKey struct {
	UserID string
	PoolID string
}

// GetAllServerPool aggregates servers into server pools based on user and pool IDs.
//
// Workflow:
//  1. Calls GetAllServers() to fetch all servers from the infrastructure.
//  2. Converts each raw server into a models.Server using FromGopherServer.
//  3. Groups servers into Serverpool structs using a map keyed by user ID and serverpool ID.
//     - If a pool does not exist yet in the map, it creates a new Serverpool with metadata and networks.
//     - Otherwise, it appends the server to the existing pool's ListServ slice.
//  4. Converts the map of server pools into a slice and returns it.
//
// Returns:
//   - []models.Serverpool: A slice of all server pools, each containing its servers.
//   - error: If fetching servers fails, returns a wrapped error.
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
