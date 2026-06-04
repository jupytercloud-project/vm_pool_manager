package jobs

import (
	"PoolManagerVM/backend/models"
	"context"
	"fmt"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
)

// StopVM powers off a VM (off-days) without deleting it — the disk/state is kept.
func StopVM(instanceID string) error {
	if err := servers.Stop(context.Background(), models.ComputeClient, instanceID).ExtractErr(); err != nil {
		return fmt.Errorf("failed to stop VM %s: %w", instanceID, err)
	}
	return nil
}

// StartVM powers a previously stopped VM back on.
func StartVM(instanceID string) error {
	if err := servers.Start(context.Background(), models.ComputeClient, instanceID).ExtractErr(); err != nil {
		return fmt.Errorf("failed to start VM %s: %w", instanceID, err)
	}
	return nil
}
