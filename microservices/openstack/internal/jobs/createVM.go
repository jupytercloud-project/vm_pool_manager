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
	"strings"
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
		ServerpoolID: job.Data["serverpool_id"],
		Metadata:     metadata,
		Networks:     networks,
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

	sshkey, err := readSSHPublicKey()
	if err != nil {
		log.Println("Failed to fetch admin's sshKey")
	}
	var pool models.Serverpool
	if err := config.Database.Where("serverpool_id = ? AND user_id = ?", job.Data["serverpool_id"], job.Data["user_id"]).
		First(&pool).Error; err != nil {
		log.Println("Unable to fetch serverpool:", err)
	}
	var userData string
	if pool.IPAddressNFS != "" {
		userData, err = buildUserData(
			baseUserConfig(sshkey),
			// computeNFSCloudConfig(pool.IPAddressNFS),
			conf_file.Data)
	} else {
		userData, err = buildUserData(
			baseUserConfig(sshkey),
			conf_file.Data,
		)
	}

	createOpts := servers.CreateOpts{
		Name:      fmt.Sprintf(`%s-%s`, serv.ServerpoolID, uuid.New().String()),
		FlavorRef: serv.FlavorRef,
		ImageRef:  serv.ImageRef,
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
func buildUserData(configs ...string) (string, error) {
	boundary := "==BOUNDARY=="
	var parts []string
	for _, cfg := range configs {
		if strings.TrimSpace(cfg) == "" {
			continue
		}
		confType := detectContentType(cfg)
		part := fmt.Sprintf(
			`--%s
Content-Type: %s
			
%s
`, boundary, confType, cfg)

		parts = append(parts, part)
	}
	footer := fmt.Sprintf(`--%s--`, boundary)
	return fmt.Sprintf(
		`MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="%s"

%s
%s
`, boundary, strings.Join(parts, ""), footer), nil
}

func detectContentType(data string) string {
	if strings.HasPrefix(strings.TrimSpace(data), "#cloud-config") {
		return "text/cloud-config"
	}
	return "text/x-shellscript"
}

func readSSHPublicKey() (string, error) {
	path := os.Getenv("SSH_PUBLIC_KEY_PATH")
	if path == "" {
		return "", fmt.Errorf("SSH_PUBLIC_KEY_PATH not set")
	}

	key, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(key)), nil
}
