package jobs

import (
	"fmt"
	"os"

	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/utils/openstack/clientconfig"
)

func DeleteVM(instanceID string) error {
	cloudName := os.Getenv("OPTS_CLOUD")
	if cloudName == "" {
		return fmt.Errorf("OPTS_CLOUD environment variable not set")
	}

	opts := &clientconfig.ClientOpts{
		Cloud: cloudName,
	}

	// Crée un provider client à partir du clouds.yaml
	provider, err := clientconfig.NewServiceClient("compute", opts)
	if err != nil {
		return fmt.Errorf("failed to create provider client: %w", err)
	}

	// Supprime la VM
	err = servers.Delete(provider, instanceID).ExtractErr()
	if err != nil {
		return fmt.Errorf("failed to delete VM %s: %w", instanceID, err)
	}

	return nil
}
