package utils

import (
	"PoolManagerVM/backend/models"
	"context"
	"log"

	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/v2/openstack/image/v2/images"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/networks"
)

// GetAllImages lists images from the INFRA project
func GetAllImages(ctx context.Context) []images.Image {
	allPages, err := images.List(models.InfraImageClient, images.ListOpts{}).AllPages(ctx)
	if err != nil {
		return nil
	}

	allImages, err := images.ExtractImages(allPages)
	if err != nil {
		return nil
	}

	return allImages
}

// GetallFlavors lists flavors from the INFRA project
func GetallFlavors(ctx context.Context) []flavors.Flavor {
	allPages, err := flavors.ListDetail(models.InfraComputeClient,
		flavors.ListOpts{}).AllPages(ctx)
	if err != nil {
		return nil
	}

	allFlavors, err := flavors.ExtractFlavors(allPages)
	if err != nil {
		return nil
	}

	return allFlavors
}

// GetAllNetworks lists networks from the INFRA project
func GetAllNetworks(ctx context.Context) []networks.Network {
	allPages, err := networks.List(models.InfraNetworkClient, networks.ListOpts{}).AllPages(ctx)
	if err != nil {
		return nil
	}

	allNets, err := networks.ExtractNetworks(allPages)
	if err != nil {
		return nil
	}

	return allNets
}

// GetAllVolumes lists volumes from the STUDENT project (volumes belong to student VMs)
// Renvoie une erreur explicite : une liste vide (0 volume) n'est PAS une panne,
// il faut pouvoir la distinguer d'un échec de connexion côté appelant.
func GetAllVolumes(ctx context.Context) ([]volumes.Volume, error) {
	allPages, err := volumes.List(models.BlockstorageClient,
		volumes.ListOpts{}).AllPages(ctx)
	if err != nil {
		log.Printf("GetAllVolumes List error: %v", err)
		return nil, err
	}

	allVolumes, err := volumes.ExtractVolumes(allPages)
	if err != nil {
		log.Printf("GetAllVolumes Extract error: %v", err)
		return nil, err
	}

	return allVolumes, nil
}
