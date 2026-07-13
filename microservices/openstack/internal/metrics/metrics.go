// Package metrics expose les métriques Prometheus du microservice OpenStack (provisioning,
// erreurs API OpenStack) et sert un endpoint /metrics dédié. Prometheus le scrape en plus du
// control center. Ne dépend d'aucun autre package interne (importable partout sans cycle).
package metrics

import (
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Provisioning de VM (CreateVM). result: success|failed.
	provisions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cpm_vm_provision_total",
		Help: "Provisionnements de VM par résultat (success|failed).",
	}, []string{"result"})

	// Durée du provisioning (création → statut ACTIVE), en secondes.
	provisionDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "cpm_vm_provision_duration_seconds",
		Help:    "Durée de provisioning d'une VM (création → ACTIVE).",
		Buckets: []float64{10, 20, 30, 45, 60, 90, 120, 180, 300, 600},
	})

	// Erreurs d'appels à l'API OpenStack, par opération (create|get|delete|power|resize…).
	openstackErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cpm_openstack_errors_total",
		Help: "Erreurs d'appels à l'API OpenStack, par opération.",
	}, []string{"operation"})
)

// RecordProvision enregistre l'issue et la durée d'un provisioning de VM.
func RecordProvision(result string, seconds float64) {
	provisions.WithLabelValues(result).Inc()
	if result == "success" {
		provisionDuration.Observe(seconds)
	}
}

// RecordOpenStackError incrémente le compteur d'erreurs API OpenStack pour une opération.
func RecordOpenStackError(operation string) { openstackErrors.WithLabelValues(operation).Inc() }

// Serve démarre un serveur HTTP /metrics dédié (port METRICS_PORT, défaut :50053) dans une
// goroutine. No-op silencieux si le port est déjà pris (n'empêche jamais le démarrage).
func Serve() {
	addr := os.Getenv("METRICS_PORT")
	if addr == "" {
		addr = ":50053"
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Printf("[metrics] endpoint Prometheus sur %s/metrics", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Printf("[metrics] serveur arrêté: %v", err)
		}
	}()
}
