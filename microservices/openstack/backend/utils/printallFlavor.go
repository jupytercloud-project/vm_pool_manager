package utils

import (
	"log"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
)

func PrintAllFlavor(flavorsList []flavors.Flavor) {
	for _, flavor := range flavorsList {
		log.Printf("Flavor ID: %s, Name: %s, RAM: %dMB, Disk: %dGB, VCPUs: %d, RXTX Factor: %f",
			flavor.ID, flavor.Name, flavor.RAM, flavor.Disk, flavor.VCPUs, flavor.RxTxFactor)
	}
}
