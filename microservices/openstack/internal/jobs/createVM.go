package jobs

import (
	"PoolManagerVM/backend/config"
	"PoolManagerVM/backend/models"
	"PoolManagerVM/backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/keypairs"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
)

func CreateVM(workerID int, job models.Job) error {

	metadata := map[string]string{}
	if metaStr, ok := job.Data["Metadata"]; ok && metaStr != "" {
		if err := json.Unmarshal([]byte(metaStr), &metadata); err != nil {
			log.Println("Error unmarshall metadata: ", err)
		}
	}
	metadata["user_id"] = job.Data["user_id"]
	metadata["serverpool_id"] = job.Data["serverpool_id"]
	metadata["min_vm"] = job.Data["min_vm"]
	metadata["max_vm"] = job.Data["max_vm"]
	metadata["host"] = "OpenStack"
	metadata["network_uuid"] = job.Data["networks"]

	var networks models.JSONStringSlice
	if err := networks.Scan(job.Data["networks"]); err != nil {
		log.Println("Failed to parse networks:", err)
		networks = models.JSONStringSlice{}
	}

	paramID := utils.ParseInt(job.Data["ID"])
	fmt.Println("Worker ", workerID, " takes the job of creating a VM")
	log.Printf("job.data[config_id]:%s", job.Data["config_id"])
	serv := models.Server{
		FlavorRef:    job.Data["flavor_ref"],
		ImageRef:     job.Data["image_ref"],
		UserID:       job.Data["user_id"],
		ServerpoolID: job.Data["serverpool_id"],
		Metadata:     metadata,
		Networks:     networks,
		ConfigID:     job.Data["config_id"],
	}

	var conf_file models.ConfigPool
	conferr := config.Database.Model(&models.ConfigPool{}).Where("id = ?", job.Data["config_id"]).First(&conf_file).Error
	if conferr != nil {
		log.Println("Error fetching config file:", conferr)
		conf_file = models.ConfigPool{
			Data: "#!/bin/bash\n",
		}
	} else {
		log.Printf("Found config file : \n%s\n", conf_file.Data)
	}
	createOpts := servers.CreateOpts{
		Name:      fmt.Sprintf(`%s-%s`, serv.ServerpoolID, uuid.New().String()),
		FlavorRef: serv.FlavorRef,
		ImageRef:  serv.ImageRef,
		Metadata:  serv.Metadata,
		Networks:  serv.Networks.ToNetworks(),
		UserData:  []byte(conf_file.Data),
	}

	createOptsExt := keypairs.CreateOptsExt{
		CreateOptsBuilder: createOpts,
		KeyName:           os.Getenv("API_KEYNAME"),
	}

	server, err := servers.Create(context.Background(),
		models.ComputeClient, createOptsExt, nil).Extract()
	if err != nil {
		log.Println("failed to create VM:", err)
		DecrementPending(uint(paramID))
		return fmt.Errorf("failed to create VM: %w", err)
	}

	for {
		current, err := servers.Get(context.Background(),
			models.ComputeClient, server.ID).Extract()
		if err != nil {
			DecrementPending(uint(paramID))
			return fmt.Errorf("failed to get server status: %w", err)
		}

		if current.Status == "ACTIVE" {
			log.Printf("[VM] Server %s is ACTIVE\n", current.ID)
			break
		}

		if current.Status == "ERROR" {
			DecrementPending(uint(paramID))
			log.Println("Server entered ERROR state:", current.ID)
			return fmt.Errorf("server %s failed to boot (ERROR state)",
				current.ID)
		}

		log.Printf("[VM] Waiting for server %s (status=%s)\n", current.ID,
			current.Status)
		time.Sleep(3 * time.Second)
	}

	DecrementPending(uint(paramID))
	fmt.Println("Worker ", workerID, " finished its job")

	return nil
}
