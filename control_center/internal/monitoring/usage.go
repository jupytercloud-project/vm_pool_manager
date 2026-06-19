package monitoring

import (
	"context"
	"log"
	"strings"
	"time"

	"control_center/config"
	"control_center/models"
)

// Comptabilisation de consommation (Phase 6, F1) : échantillonne périodiquement les
// VMs actives et cumule leur temps d'activité par mois, pondéré par le flavor
// (vCPU / RAM), pour la page coût et les rapports d'usage.

const usageSampleInterval = 1 * time.Minute

// StartUsageAccounting lance la boucle d'échantillonnage de consommation.
func StartUsageAccounting(ctx context.Context) {
	ticker := time.NewTicker(usageSampleInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sampleUsage()
		}
	}
}

func currentYearMonth() string {
	return time.Now().UTC().Format("2006-01")
}

func sampleUsage() {
	// Flavors : id -> flavor, pour pondérer l'usage par vCPU / RAM.
	var flavors []models.Flavor
	config.Database.Find(&flavors)
	flavorByID := map[string]models.Flavor{}
	for _, f := range flavors {
		flavorByID[f.ID] = f
	}

	var servers []models.Server
	if err := config.Database.Where("status = ?", "ACTIVE").Find(&servers).Error; err != nil {
		return
	}

	now := time.Now().UTC()
	ym := currentYearMonth()
	for _, srv := range servers {
		f := flavorByID[srv.FlavorRef]

		var row models.VMUsage
		err := config.Database.Where("year_month = ? AND vm_id = ?", ym, srv.ID).First(&row).Error
		if err != nil {
			// Première observation ce mois-ci : on initialise sans incrément (on ne sait
			// pas depuis quand elle tourne — on compte à partir de maintenant).
			config.Database.Create(&models.VMUsage{
				YearMonth: ym, VMID: srv.ID, PoolID: serverPool(srv), UserID: srv.UserID,
				Flavor: srv.FlavorRef, VCPUs: f.VCPUs, RAMMB: f.RAM,
				Seconds: 0, LastSampled: now,
			})
			continue
		}

		// Incrément borné à 2× l'intervalle : évite de compter un long arrêt du service.
		elapsed := int64(now.Sub(row.LastSampled).Seconds())
		if maxStep := int64(2 * usageSampleInterval.Seconds()); elapsed > maxStep {
			elapsed = maxStep
		}
		if elapsed < 0 {
			elapsed = 0
		}
		// Update piloté par le struct (mapping champ→colonne géré par GORM) — évite
		// les noms de colonnes en dur qui ne correspondaient pas au schéma.
		row.Seconds += elapsed
		row.LastSampled = now
		row.VCPUs = f.VCPUs
		row.RAMMB = f.RAM
		row.PoolID = serverPool(srv)
		row.UserID = srv.UserID
		row.Flavor = srv.FlavorRef
		if err := config.Database.Save(&row).Error; err != nil {
			log.Printf("[usage] maj %s: %v", srv.ID, err)
		}
	}
}

// serverPool renvoie l'identifiant de pool d'un serveur (colonne ou metadata).
func serverPool(srv models.Server) string {
	if srv.ServerpoolID != "" {
		return srv.ServerpoolID
	}
	if srv.Metadata != nil {
		return strings.TrimSpace(srv.Metadata["serverpool_id"])
	}
	return ""
}
