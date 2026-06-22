package utils

import (
	"PoolManagerVM/backend/models"
	"context"
	"log"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/v2/openstack/image/v2/images"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/networks"
)

// GetAllImages liste les images des DEUX projets : infra (publiques/partagées) ET étudiant
// (qui contient les snapshots privés jupyter-snapshot-* / jupyterhub utilisés pour les pools).
// Fusion dédupliquée par ID (une image partagée peut apparaître dans les deux projets).
func GetAllImages(ctx context.Context) []images.Image {
	seen := map[string]bool{}
	var merged []images.Image

	collect := func(client *gophercloud.ServiceClient) {
		if client == nil {
			return
		}
		pages, err := images.List(client, images.ListOpts{}).AllPages(ctx)
		if err != nil {
			log.Printf("[images] liste échouée: %v", err)
			return
		}
		imgs, err := images.ExtractImages(pages)
		if err != nil {
			log.Printf("[images] extraction échouée: %v", err)
			return
		}
		for _, img := range imgs {
			if seen[img.ID] {
				continue
			}
			seen[img.ID] = true
			merged = append(merged, img)
		}
	}

	collect(models.InfraImageClient) // images publiques/partagées (projet infra)
	collect(models.ImageClient)      // snapshots privés (projet étudiant)
	return merged
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
