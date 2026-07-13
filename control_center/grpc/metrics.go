package grpc

import (
	"time"

	"control_center/config"
	"control_center/models"

	"github.com/prometheus/client_golang/prometheus"
)

// cpmCollector expose des métriques d'ÉTAT en interrogeant PostgreSQL à chaque scrape.
// Prometheus en fait des séries temporelles (heures de pointe, occupation, coût, quotas…).
// Les métriques d'ÉVÉNEMENT (compteurs/histogrammes : provisioning, attribution, jobs…) sont
// dans appmetrics.go (promauto sur le registre par défaut, même endpoint /metrics).
type cpmCollector struct {
	pools          *prometheus.Desc
	servers        *prometheus.Desc
	vmsActive      *prometheus.Desc
	students       *prometheus.Desc
	ghSessions     *prometheus.Desc
	poolUsage      *prometheus.Desc
	jobs           *prometheus.Desc
	vmInstances    *prometheus.Desc
	registrarStale *prometheus.Desc
	monthCost      *prometheus.Desc
	monthVMHours   *prometheus.Desc
	poolCost       *prometheus.Desc
	storageGB      *prometheus.Desc
	storageQuota   *prometheus.Desc
}

func newCPMCollector() *cpmCollector {
	return &cpmCollector{
		pools:          prometheus.NewDesc("cpm_pools_total", "Nombre de serverpools.", nil, nil),
		servers:        prometheus.NewDesc("cpm_servers", "Nombre de VMs par statut.", []string{"status"}, nil),
		vmsActive:      prometheus.NewDesc("cpm_vms_active", "VMs avec activité récente (utilisateur connecté).", nil, nil),
		students:       prometheus.NewDesc("cpm_students_total", "Nombre d'étudiants enregistrés.", nil, nil),
		ghSessions:     prometheus.NewDesc("cpm_github_sessions_24h", "Connexions GitHub sur les dernières 24 h.", nil, nil),
		poolUsage:      prometheus.NewDesc("cpm_pool_students", "Étudiants rattachés, par pool.", []string{"pool", "owner"}, nil),
		jobs:           prometheus.NewDesc("cpm_batch_jobs", "Jobs batch par statut (queued/running/succeeded/failed/canceled).", []string{"status"}, nil),
		vmInstances:    prometheus.NewDesc("cpm_vm_instances_total", "VMs enregistrées auprès du registrar.", nil, nil),
		registrarStale: prometheus.NewDesc("cpm_vm_registrar_stale", "VMs sans heartbeat registrar depuis > 30 min.", nil, nil),
		monthCost:      prometheus.NewDesc("cpm_month_cost", "Coût estimé cumulé du mois courant (devise configurée).", nil, nil),
		monthVMHours:   prometheus.NewDesc("cpm_month_vm_hours", "Heures-VM cumulées du mois courant.", nil, nil),
		poolCost:       prometheus.NewDesc("cpm_pool_month_cost", "Coût estimé du mois courant, par pool.", []string{"pool", "owner"}, nil),
		storageGB:      prometheus.NewDesc("cpm_storage_allocated_gb", "Stockage alloué (somme disque des flavors × VMs).", nil, nil),
		storageQuota:   prometheus.NewDesc("cpm_storage_quota_gb", "Quota de stockage configuré (0 = illimité).", nil, nil),
	}
}

func (c *cpmCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.pools
	ch <- c.servers
	ch <- c.vmsActive
	ch <- c.students
	ch <- c.ghSessions
	ch <- c.poolUsage
	ch <- c.jobs
	ch <- c.vmInstances
	ch <- c.registrarStale
	ch <- c.monthCost
	ch <- c.monthVMHours
	ch <- c.poolCost
	ch <- c.storageGB
	ch <- c.storageQuota
}

func (c *cpmCollector) Collect(ch chan<- prometheus.Metric) {
	db := config.Database
	if db == nil {
		return
	}

	var n int64
	db.Model(&models.Serverpool{}).Count(&n)
	ch <- prometheus.MustNewConstMetric(c.pools, prometheus.GaugeValue, float64(n))

	type kv struct {
		K string
		C int64
	}
	var rows []kv
	db.Model(&models.Server{}).
		Select("COALESCE(NULLIF(status,''),'unknown') as k, count(*) as c").
		Group("status").Scan(&rows)
	for _, r := range rows {
		ch <- prometheus.MustNewConstMetric(c.servers, prometheus.GaugeValue, float64(r.C), r.K)
	}

	var active int64
	db.Model(&models.VMInstance{}).
		Where("activity_status <> 'idle' AND last_seen > now() - interval '10 minutes'").
		Count(&active)
	ch <- prometheus.MustNewConstMetric(c.vmsActive, prometheus.GaugeValue, float64(active))

	db.Model(&models.Student{}).Count(&n)
	ch <- prometheus.MustNewConstMetric(c.students, prometheus.GaugeValue, float64(n))

	var gh int64
	db.Model(&models.GitHubSession{}).Where("created_at > now() - interval '24 hours'").Count(&gh)
	ch <- prometheus.MustNewConstMetric(c.ghSessions, prometheus.GaugeValue, float64(gh))

	type poolCount struct {
		Pool  string
		Owner string
		C     int64
	}
	var pcs []poolCount
	db.Raw(`SELECT sp.serverpool_id AS pool, sp.user_id AS owner, count(st.id) AS c
	        FROM serverpools sp
	        LEFT JOIN list_students ls ON ls.pool_id = sp.id
	        LEFT JOIN students st ON st.list_id = ls.id
	        WHERE sp.serverpool_id <> ''
	        GROUP BY sp.serverpool_id, sp.user_id`).Scan(&pcs)
	for _, p := range pcs {
		ch <- prometheus.MustNewConstMetric(c.poolUsage, prometheus.GaugeValue, float64(p.C), p.Pool, p.Owner)
	}

	// Jobs batch par statut.
	var jobRows []kv
	db.Model(&models.BatchJob{}).
		Select("COALESCE(NULLIF(status,''),'unknown') as k, count(*) as c").
		Group("status").Scan(&jobRows)
	for _, r := range jobRows {
		ch <- prometheus.MustNewConstMetric(c.jobs, prometheus.GaugeValue, float64(r.C), r.K)
	}

	// Inventaire registrar : total + VMs « périmées ». On ne compte comme périmée qu'une VM
	// qui DEVRAIT être vivante (encore présente dans `servers`) et dont le heartbeat date de
	// > 30 min — pas les entrées orphelines de VMs déjà supprimées (qui gonfleraient à tort).
	var vmTotal, vmStale int64
	db.Model(&models.VMInstance{}).Count(&vmTotal)
	db.Model(&models.VMInstance{}).
		Where("last_seen < now() - interval '30 minutes'").
		Where("(name IN (SELECT name FROM servers) OR ip IN (SELECT ip_address FROM servers))").
		Count(&vmStale)
	ch <- prometheus.MustNewConstMetric(c.vmInstances, prometheus.GaugeValue, float64(vmTotal))
	ch <- prometheus.MustNewConstMetric(c.registrarStale, prometheus.GaugeValue, float64(vmStale))

	// Comptabilité (accounting) du mois courant, dérivée de VMUsage (heures-VM pondérées vCPU/RAM).
	// Réutilise le même calcul de coût que GET /api/usage (priceVCPUHour/priceGBHour).
	c.collectCost(ch)

	// Stockage alloué + quota (même calcul que GET /api/storage : disque du flavor × VMs).
	var flavors []models.Flavor
	db.Find(&flavors)
	diskByFlavor := map[string]int{}
	for _, f := range flavors {
		diskByFlavor[f.ID] = f.Disk
	}
	var servers []models.Server
	db.Find(&servers)
	allocGB := 0
	for _, s := range servers {
		allocGB += diskByFlavor[s.FlavorRef]
	}
	ch <- prometheus.MustNewConstMetric(c.storageGB, prometheus.GaugeValue, float64(allocGB))
	ch <- prometheus.MustNewConstMetric(c.storageQuota, prometheus.GaugeValue, float64(storageQuotaGB()))
}

// collectCost agrège VMUsage pour le mois courant : coût & heures-VM totaux + coût par pool.
func (c *cpmCollector) collectCost(ch chan<- prometheus.Metric) {
	month := time.Now().UTC().Format("2006-01")
	var usages []models.VMUsage
	if err := config.Database.Where("year_month = ?", month).Find(&usages).Error; err != nil {
		return
	}
	var totalCost, totalHours float64
	type agg struct{ cost float64 }
	byPool := map[[2]string]*agg{}
	for _, u := range usages {
		vmHours, _, _, cost := vmCost(u.Seconds, u.VCPUs, u.RAMMB)
		totalCost += cost
		totalHours += vmHours
		key := [2]string{u.PoolID, u.UserID}
		if byPool[key] == nil {
			byPool[key] = &agg{}
		}
		byPool[key].cost += cost
	}
	ch <- prometheus.MustNewConstMetric(c.monthCost, prometheus.GaugeValue, totalCost)
	ch <- prometheus.MustNewConstMetric(c.monthVMHours, prometheus.GaugeValue, totalHours)
	for k, a := range byPool {
		pool := k[0]
		if pool == "" {
			pool = "unknown"
		}
		ch <- prometheus.MustNewConstMetric(c.poolCost, prometheus.GaugeValue, a.cost, pool, k[1])
	}
}

// registerMetrics enregistre le collecteur (idempotent en cas de double appel).
func registerMetrics() {
	_ = prometheus.Register(newCPMCollector())
}
