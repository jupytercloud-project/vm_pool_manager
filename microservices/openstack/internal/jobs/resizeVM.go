package jobs

import (
	"PoolManagerVM/backend/config"
	"PoolManagerVM/backend/models"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
)

// ResizeVM change le flavor (gabarit) d'une VM. OpenStack fait un resize en deux
// temps : RESIZE → VERIFY_RESIZE, puis il faut confirmer (ConfirmResize) pour
// finaliser. On enchaîne automatiquement la confirmation une fois l'état stabilisé.
// Exécuté en job asynchrone car l'opération peut durer plusieurs minutes.
func ResizeVM(instanceID, flavorRef string) error {
	if instanceID == "" || flavorRef == "" {
		return fmt.Errorf("resize: instance_id et flavor_ref requis")
	}
	ctx := context.Background()

	if err := servers.Resize(ctx, models.ComputeClient, instanceID,
		servers.ResizeOpts{FlavorRef: flavorRef}).ExtractErr(); err != nil {
		return fmt.Errorf("resize de %s échoué: %w", instanceID, err)
	}

	// Le resize peut impliquer une migration entre hôtes → potentiellement long.
	// On attend VERIFY_RESIZE puis on confirme. Certains clouds confirment seuls
	// (RESIZE → ACTIVE sans VERIFY_RESIZE) : on ne considère ACTIVE comme terminal
	// qu'APRÈS avoir vu le resize démarrer (sinon on sortirait avant qu'il commence).
	deadline := time.Now().Add(20 * time.Minute)
	seenResizing := false
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("resize de %s: délai dépassé (état non stabilisé)", instanceID)
		}
		srv, err := servers.Get(ctx, models.ComputeClient, instanceID).Extract()
		if err != nil {
			return fmt.Errorf("resize de %s: lecture d'état échouée: %w", instanceID, err)
		}
		switch strings.ToUpper(srv.Status) {
		case "VERIFY_RESIZE":
			if err := servers.ConfirmResize(ctx, models.ComputeClient, instanceID).ExtractErr(); err != nil {
				return fmt.Errorf("confirmation du resize de %s échouée: %w", instanceID, err)
			}
			updateFlavorInDB(instanceID, flavorRef)
			log.Printf("[resize] %s redimensionné vers le flavor %s", instanceID, flavorRef)
			return nil
		case "RESIZE", "RESIZED", "MIGRATING":
			seenResizing = true
		case "ACTIVE":
			if seenResizing {
				// Le cloud a confirmé automatiquement.
				updateFlavorInDB(instanceID, flavorRef)
				log.Printf("[resize] %s redimensionné (auto-confirmé) vers %s", instanceID, flavorRef)
				return nil
			}
			// Sinon : le resize n'a pas encore démarré, on continue d'attendre.
		case "ERROR":
			return fmt.Errorf("resize de %s: la VM est passée en ERROR", instanceID)
		}
		time.Sleep(5 * time.Second)
	}
}

func updateFlavorInDB(instanceID, flavorRef string) {
	if err := config.Database.Model(&models.Server{}).
		Where("id = ?", instanceID).
		Update("flavor_ref", flavorRef).Error; err != nil {
		log.Printf("[resize] màj flavor en base pour %s: %v", instanceID, err)
	}
}
