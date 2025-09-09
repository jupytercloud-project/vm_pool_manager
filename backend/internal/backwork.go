package internal

import (
	"PoolManagerVM/backend/internal/worker"
	"PoolManagerVM/backend/utils"
	"context"
	"log"
	"strconv"
	"time"

	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
)

// Backwork is a background loop that monitors admin servers and ensures a minimum number of VMs are running.
// It fetches all servers, filters those owned by the admin, and compares the current count to a configured minimum.
// If there are too few, it adds jobs to create additional VMs. The loop repeats every 20 seconds.

func Backwork(ctx context.Context) {

	for {
		allServers, err := utils.GetAllServers()
		if err != nil {
			log.Printf("Error : %v", err)
			return
		}
		var myPool []servers.Server
		for _, s := range allServers {
			if s.Metadata["userID"] == "admin" {
				myPool = append(myPool, s)
			}
		}
		if len(myPool) == 0 {
			cfg, err := utils.LoadConfig("config.toml")
			if err != nil {
				log.Printf("Error")
				return
			}
			numVM, err := strconv.Atoi(cfg.Metadata["minVM"])
			if err != nil {
				log.Printf("Error : %v", err)
			}
			utils.PendingMu.Lock()
			if utils.PendingJobs < numVM {
				for range numVM {
					worker.AddJob(*worker.CreateJob("base", worker.CreateVMAdmin, nil), false)
					utils.PendingJobs++
				}
			}
			utils.PendingMu.Unlock()
		} else {
			numVM, err := strconv.Atoi(myPool[0].Metadata["minVM"])
			if err != nil {
				log.Printf("Error : %v", err)
			}
			utils.PendingMu.Lock()
			if len(myPool)+utils.PendingJobs < numVM {
				numToCreate := numVM - (len(myPool) + utils.PendingJobs)
				for range numToCreate {
					worker.AddJob(*worker.CreateJob("base", worker.CreateVMAdmin, nil), false)
					utils.PendingJobs++
				}
			}
			utils.PendingMu.Unlock()
		}
		select {
		case <-ctx.Done():
			log.Println("Backwork stopped")
			return
		case <-time.After(10 * time.Second):
			// next cycle
		}
	}
}
