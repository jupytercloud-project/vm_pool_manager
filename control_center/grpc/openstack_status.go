package grpc

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/gophercloud/utils/v2/openstack/clientconfig"
)

// Cache court pour ne pas marteler OpenStack à chaque rafraîchissement d'inventaire.
var (
	psMu        sync.Mutex
	psCache     map[string]string
	psFetchedAt time.Time
)

// invalidatePowerStates force un rafraîchissement de l'état Nova au prochain appel
// (après une action VM, pour refléter vite le changement).
func invalidatePowerStates() {
	psMu.Lock()
	psFetchedAt = time.Time{}
	psMu.Unlock()
}

// fetchPowerStates renvoie {server_id: statut Nova} (ACTIVE/SHUTOFF/SUSPENDED/…)
// pour le projet étudiant. Lecture seule. En cas d'erreur, renvoie le dernier cache connu.
func fetchPowerStates() map[string]string {
	psMu.Lock()
	defer psMu.Unlock()
	if psCache != nil && time.Since(psFetchedAt) < 8*time.Second {
		return psCache
	}

	cloud := os.Getenv("OS_CLOUD")
	if cloud == "" {
		cloud = os.Getenv("OPTS_CLOUD")
	}
	client, err := clientconfig.NewServiceClient(context.Background(), "compute",
		&clientconfig.ClientOpts{Cloud: cloud})
	if err != nil {
		return psCache
	}
	pages, err := servers.List(client, servers.ListOpts{}).AllPages(context.Background())
	if err != nil {
		return psCache
	}
	list, err := servers.ExtractServers(pages)
	if err != nil {
		return psCache
	}
	out := make(map[string]string, len(list))
	for _, s := range list {
		out[s.ID] = s.Status
	}
	psCache = out
	psFetchedAt = time.Now()
	return out
}
