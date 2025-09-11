package utils

import (
	"fmt"
	"os"

	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/utils/openstack/clientconfig"
)

// GetAllServers retrieves the full list of servers from OpenStack.
//
// This function performs the following steps:
// 1. Creates an OpenStack client for the "compute" service using the cloud
//    configuration defined in clientconfig.ClientOpts.
// 2. Lists all servers available via the `servers.List` API.
// 3. Extracts all servers from the paginated response returned by OpenStack.
//
// Errors encountered during client creation, server listing, or extraction are
// returned instead of terminating the program, making this function safe to use
// in long-running applications or web servers.
//
// Returns:
// - A slice of `servers.Server` containing all retrieved servers.
// - An error if any step fails.

func GetAllServers() ([]servers.Server, error) {
	opts := &clientconfig.ClientOpts{
		Cloud: os.Getenv("OPTS_CLOUD"),
	}

	client, err := clientconfig.NewServiceClient("compute", opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute client: %w", err)
	}

	pages, err := servers.List(client, servers.ListOpts{}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}

	allServers, err := servers.ExtractServers(pages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract servers: %w", err)
	}

	return allServers, nil
}
