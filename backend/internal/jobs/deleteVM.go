package jobs

import (
	"context"
	"fmt"
	"os"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/gophercloud/utils/v2/openstack/clientconfig"
)

// DeleteVM deletes an existing virtual machine (VM) from OpenStack.
//
// Workflow:
//  1. Reads the cloud configuration name from the OPTS_CLOUD environment variable.
//  2. Initializes an OpenStack compute client using the clouds.yaml configuration.
//  3. Sends a delete request for the VM with the given instance ID.
//  4. Returns an error if the environment variable is missing, the client cannot be created,
//     or the deletion request fails.
//
// Parameters:
//   - instanceID: The unique identifier of the VM to be deleted.
//
// Returns:
//   - error: An error if the client setup fails or the VM deletion request fails.
//     Returns nil if the VM is successfully deleted.
func DeleteVM(instanceID string) error {
	cloudName := os.Getenv("OPTS_CLOUD")
	if cloudName == "" {
		return fmt.Errorf("OPTS_CLOUD environment variable not set")
	}

	opts := &clientconfig.ClientOpts{
		Cloud: cloudName,
	}

	// Crée un provider client à partir du clouds.yaml
	provider, err := clientconfig.NewServiceClient(context.Background(), "compute", opts)
	if err != nil {
		return fmt.Errorf("failed to create provider client: %w", err)
	}

	// Supprime la VM
	err = servers.Delete(context.Background(), provider, instanceID).ExtractErr()
	if err != nil {
		return fmt.Errorf("failed to delete VM %s: %w", instanceID, err)
	}

	return nil
}
