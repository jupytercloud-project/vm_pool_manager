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

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/keypairs"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
)

func CreateNFSVM(workerID int, job models.Job) error {

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
	fmt.Println("Worker ", workerID, " takes the job of creating a VM NFS")
	log.Printf("job.data[config_id]:%s", job.Data["config_id"])
	serv := models.Server{
		Metadata: metadata,
		Networks: networks,
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

	adminsshkey, err := readSSHPublicKey()
	if err != nil {
		log.Println("Failed to fetch admin's key")
	}
	userData, err := buildUserData(baseUserConfig(adminsshkey), nfsCloudConfig(job.Data["user_id"], job.Data["serverpool_id"]), conf_file.Data)
	if err != nil {
		log.Println("Failed to build user-data:", err)
		userData = "#!/bin/bash\n"
	}

	createOpts := servers.CreateOpts{
		Name:      fmt.Sprintf(`%s-%s-NFS`, job.Data["user_id"], job.Data["serverpool_id"]),
		FlavorRef: os.Getenv("SERVER_FLAVOR_REF"),
		ImageRef:  os.Getenv("SERVER_IMAGE_REF"),
		Metadata:  serv.Metadata,
		Networks:  serv.Networks.ToNetworks(),
		UserData:  []byte(userData),
	}

	createOptsExt := keypairs.CreateOptsExt{
		CreateOptsBuilder: createOpts,
		KeyName:           os.Getenv("API_KEYNAME"),
	}

	server, err := servers.Create(context.Background(),
		models.ComputeClient, createOptsExt, nil).Extract()
	if err != nil {
		log.Println("failed to create VM:", err)
		ChangePendingNFS(uint(paramID))
		return fmt.Errorf("failed to create VM: %w", err)
	}

	for {
		current, err := servers.Get(context.Background(),
			models.ComputeClient, server.ID).Extract()
		if err != nil {
			ChangePendingNFS(uint(paramID))
			return fmt.Errorf("failed to get server status: %w", err)
		}

		if current.Status == "ACTIVE" {
			log.Printf("[VM] Server %s is ACTIVE\n", current.ID)
			break
		}

		if current.Status == "ERROR" {
			ChangePendingNFS(uint(paramID))
			log.Println("Server entered ERROR state:", current.ID)
			return fmt.Errorf("server %s failed to boot (ERROR state)",
				current.ID)
		}

		log.Printf("[VM] Waiting for server %s (status=%s)\n", current.ID,
			current.Status)
		time.Sleep(3 * time.Second)
	}
	fmt.Println("Worker ", workerID, " finished its job")
	res := config.Database.Model(models.Serverpool{}).
		Where("serverpool_id = ? AND user_id = ?", job.Data["serverpool_id"], job.Data["user_id"]).
		UpdateColumn("ip_address_nfs", server.AccessIPv4)
	if res != nil {
		log.Println("Error adding ip_address_nfs")
	}

	return nil
}
