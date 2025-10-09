package utils

import (
	"PoolManagerVM/backend/models"
	"context"
	"os"

	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/v2/openstack/image/v2/images"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/networks"
	"github.com/gophercloud/utils/v2/openstack/clientconfig"
)

func GetAllImages(ctx context.Context) []images.Image {
	opts := &clientconfig.ClientOpts{
		Cloud: os.Getenv("OPTS_CLOUD"),
	}

	client, err := clientconfig.NewServiceClient(ctx, "image", opts)
	if err != nil {
		return nil
	}

	allPages, err := images.List(client, images.ListOpts{}).AllPages(ctx)
	if err != nil {
		return nil
	}

	allImages, err := images.ExtractImages(allPages)
	if err != nil {
		return nil
	}

	return allImages
}

func GetallFlavors(ctx context.Context) []flavors.Flavor {

	allPages, err := flavors.ListDetail(models.ComputeClient, flavors.ListOpts{}).AllPages(ctx)
	if err != nil {
		return nil
	}
	allFlavors, err := flavors.ExtractFlavors(allPages)
	if err != nil {
		return nil
	}

	return allFlavors
}

func GetAllNetworks(ctx context.Context) []networks.Network {
	opts := &clientconfig.ClientOpts{
		Cloud: os.Getenv("OPTS_CLOUD"),
	}

	client, err := clientconfig.NewServiceClient(ctx, "network", opts)
	if err != nil {
		return nil
	}

	allPages, err := networks.List(client, networks.ListOpts{}).AllPages(ctx)
	if err != nil {
		return nil
	}

	allNets, err := networks.ExtractNetworks(allPages)
	if err != nil {
		return nil
	}

	return allNets
}

func GetAllVolumes(ctx context.Context) []volumes.Volume {
	allPages, err := volumes.List(models.BlockstorageClient, volumes.ListOpts{}).AllPages(ctx)
	if err != nil {
		return nil
	}

	allVolumes, err := volumes.ExtractVolumes(allPages)
	if err != nil {
		return nil
	}

	return allVolumes
}
