// Package metrics expose des métriques d'ÉVÉNEMENT Prometheus (compteurs/histogrammes)
// partagées par tous les packages du control center. Elles s'enregistrent sur le registre
// par défaut (promauto) et sont donc servies par le même endpoint /metrics que le collecteur
// d'état (grpc.registerMetrics). Importable partout sans cycle : ce package ne dépend d'aucun
// autre package interne.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Attribution de VM aux étudiants (AttribVMinPool). result: success|no_available|error.
	attributions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cpm_vm_attribution_total",
		Help: "Attributions de VM par résultat (success|no_available|error).",
	}, []string{"result"})

	// Ouverture de sessions de proxy applicatif. kind: jupyter|vscode.
	proxySessions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cpm_proxy_sessions_total",
		Help: "Sessions de proxy ouvertes, par type d'application.",
	}, []string{"kind"})

	// Jobs batch traités. result: succeeded|failed|canceled.
	batchJobs = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cpm_batch_jobs_processed_total",
		Help: "Jobs batch terminés, par résultat.",
	}, []string{"result"})

	// Durée d'exécution d'un job batch (secondes).
	batchJobDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "cpm_batch_job_duration_seconds",
		Help:    "Durée d'exécution des jobs batch.",
		Buckets: []float64{5, 15, 30, 60, 120, 300, 600, 1800, 3600, 7200},
	})

	// Actions de cycle de vie VM (start/stop/…). result: success|error.
	vmActions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cpm_vm_action_total",
		Help: "Actions de cycle de vie VM (start|stop|suspend|resume|reboot), par résultat.",
	}, []string{"action", "result"})
)

// RecordAttribution incrémente le compteur d'attributions de VM.
func RecordAttribution(result string) { attributions.WithLabelValues(result).Inc() }

// RecordProxySession incrémente le compteur d'ouvertures de session de proxy.
func RecordProxySession(kind string) { proxySessions.WithLabelValues(kind).Inc() }

// RecordBatchJob incrémente le compteur de jobs batch terminés.
func RecordBatchJob(result string) { batchJobs.WithLabelValues(result).Inc() }

// ObserveBatchJobDuration enregistre la durée d'un job batch (secondes).
func ObserveBatchJobDuration(seconds float64) { batchJobDuration.Observe(seconds) }

// RecordVMAction incrémente le compteur d'actions de cycle de vie VM.
func RecordVMAction(action, result string) { vmActions.WithLabelValues(action, result).Inc() }
