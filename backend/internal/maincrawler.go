package internal

import (
	"PoolManagerVM/backend/utils"
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
)

func Monitor(c context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Done():
			log.Println("Monitoring stopped")
			return

		case <-ticker.C:
			log.Println("Checking serverpools...")
			allServers, err := utils.GetAllServers()
			if err != nil {
				log.Fatalf("Error : %v", err)
				return
			}
			CheckAndCreate(allServers)
		}
	}
}

func CheckAndCreate(allServers []servers.Server) {
	serverPools := map[string][]servers.Server{}
	minVM := map[string]int{}
	maxVM := map[string]int{}

	for _, s := range allServers {
		poolID := s.Metadata["serverpool"]
		serverPools[poolID] = append(serverPools[poolID], s)
		min, err := strconv.Atoi(s.Metadata["minVM"])
		if err != nil {
			//stuff
		}
		minVM[poolID] = min
		max, err := strconv.Atoi(s.Metadata["maxVM"])
		if err != nil {
			//stuff
		}
		maxVM[poolID] = max
	}

	for poolID, serversInPool := range serverPools {
		active := len(serversInPool)
		missing := minVM[poolID] - active
		if missing > 0 {
			fmt.Printf("Serverpool %s: missing %d VM(s)\n", poolID, missing)
			for range missing {
				// go createVM(poolID, active, missing)
			}
		}
	}
}
