package monitoring

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"control_center/config"
	"control_center/internal/metrics"
	"control_center/internal/sshinject"
	"control_center/models"
	"control_center/pb"

	"golang.org/x/crypto/ssh"
)

const (
	batchPollInterval = 5 * time.Second
	batchMaxLog       = 64 * 1024 // 64 Kio de log conservés
	batchMaxRuntime   = 30 * time.Minute
)

// batchConcurrency : nombre max de jobs exécutés en parallèle (env BATCH_CONCURRENCY, défaut 2).
func batchConcurrency() int {
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv("BATCH_CONCURRENCY"))); err == nil && v >= 1 {
		return v
	}
	return 2
}

// StartBatchRunner traite la file de jobs batch : jusqu'à BATCH_CONCURRENCY jobs en
// parallèle, par ordre de PRIORITÉ (puis FIFO). Chaque job exécute son script sur une
// VM du pool cible via SSH, collecte la sortie, et suspend la VM en fin (B4).
// Phase 4 — B1+B4+B6 (file d'attente & priorités).
func StartBatchRunner(ctx context.Context, client pb.PoolManagerClient) {
	ticker := time.NewTicker(batchPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dispatchBatchJobs(client)
		}
	}
}

// dispatchBatchJobs (exécuté séquentiellement par le ticker → claim sans course)
// lance des jobs tant que la concurrence n'est pas atteinte, par priorité.
func dispatchBatchJobs(client pb.PoolManagerClient) {
	conc := batchConcurrency()
	for {
		var running int64
		config.Database.Model(&models.BatchJob{}).Where("status = ?", "running").Count(&running)
		if int(running) >= conc {
			return
		}
		var job models.BatchJob
		if err := config.Database.Where("status = ?", "queued").
			Order("priority DESC, id ASC").First(&job).Error; err != nil {
			return // file vide
		}
		// Claim : passe en running (le ticker est seul à claimer → pas de double-prise).
		config.Database.Model(&models.BatchJob{}).Where("id = ?", job.ID).
			Updates(map[string]any{"status": "running", "started_at": time.Now().UTC()})
		go processBatchJob(client, job)
	}
}

func processBatchJob(client pb.PoolManagerClient, job models.BatchJob) {
	signer, err := sshinject.LoadPrivateKey(os.Getenv("SSH_PRIVATE_KEY_PATH"))
	if err != nil {
		finishJob(job.ID, "failed", -1, "Clé SSH indisponible: "+err.Error(), "")
		return
	}
	if job.Ephemeral {
		processEphemeralJob(client, job, signer)
		return
	}

	// Mode réutilisation : exécute sur une VM existante du pool cible.
	var servers []models.Server
	config.Database.Where("serverpool_id = ? AND ip_address <> ''", job.PoolID).Find(&servers)
	var target *models.Server
	for i := range servers {
		if sshReachable(servers[i].IP_Address) {
			target = &servers[i]
			break
		}
	}
	if target == nil {
		finishJob(job.ID, "failed", -1, "Aucune VM joignable dans le pool « "+job.PoolID+" ».", "")
		return
	}

	out, code, runErr := runScriptOnVM(target.IP_Address, signer, job.Script)
	status, out := jobOutcome(out, code, runErr)
	finishJob(job.ID, status, code, out, target.Name)

	if job.AutoStop && client != nil {
		_ = suspendServer(client, *target) // B4 : suspend la VM en fin de job
	}
}

// jobOutcome dérive le statut + tronque la sortie.
func jobOutcome(out string, code int, runErr error) (string, string) {
	if len(out) > batchMaxLog {
		out = out[len(out)-batchMaxLog:]
	}
	if runErr != nil || code != 0 {
		if runErr != nil && out == "" {
			out = runErr.Error()
		}
		return "failed", out
	}
	return "succeeded", out
}

// processEphemeralJob provisionne un pool transitoire (1 VM, ou N pour un cluster),
// exécute le script sur le nœud « head », puis DÉTRUIT le pool (et donc les VMs).
// ⚠️ Orchestration OpenStack — calquée sur le cycle de vie de pool existant.
func processEphemeralJob(client pb.PoolManagerClient, job models.BatchJob, signer ssh.Signer) {
	n := job.Nodes
	if n < 1 {
		n = 1
	}
	poolID := fmt.Sprintf("jobvm-%d", job.ID)

	if err := provisionTransientPool(client, job, poolID, n); err != nil {
		finishJob(job.ID, "failed", -1, "Provisionnement échoué : "+err.Error(), "")
		return
	}
	// Toujours détruire le pool transitoire (et ses VMs) en sortie.
	defer teardownTransientPool(client, job.OwnerEmail, poolID)

	vms, err := waitForPoolVMs(poolID, n, 12*time.Minute)
	if err != nil {
		finishJob(job.ID, "failed", -1, "VMs non prêtes : "+err.Error(), "")
		return
	}
	head := vms[0]

	// Cluster : écrire un hostfile (IPs des nœuds) sur chaque VM pour MPI/Dask.
	if n > 1 {
		hostfile := ""
		for _, vm := range vms {
			hostfile += vm.IP_Address + "\n"
		}
		for _, vm := range vms {
			_ = writeFileOnVM(vm.IP_Address, signer, "/home/"+sshVMUser()+"/hostfile", hostfile)
		}
	}

	script := job.Script
	if n > 1 {
		script = "export CLUSTER_NODES=" + fmt.Sprint(n) + "\nexport HOSTFILE=/home/" + sshVMUser() + "/hostfile\n" + script
	}
	out, code, runErr := runScriptOnVM(head.IP_Address, signer, script)
	status, out := jobOutcome(out, code, runErr)
	vmLabel := head.Name
	if n > 1 {
		vmLabel = fmt.Sprintf("%s (+%d nœuds)", head.Name, n-1)
	}
	finishJob(job.ID, status, code, out, vmLabel)
}

// provisionTransientPool crée un pool transitoire (copie la config du pool source)
// et déclenche le provisionnement de n VMs via le microservice.
func provisionTransientPool(client pb.PoolManagerClient, job models.BatchJob, poolID string, n int) error {
	var src models.Serverpool
	if err := config.Database.Where("serverpool_id = ?", job.PoolID).First(&src).Error; err != nil {
		return fmt.Errorf("pool source « %s » introuvable", job.PoolID)
	}
	tp := models.Serverpool{
		ServerpoolID: poolID, UserID: job.OwnerEmail,
		ImageRef: src.ImageRef, FlavorRef: src.FlavorRef, Networks: src.Networks,
		ConfigID: src.ConfigID, MinVM: n, MaxVM: n, Status: "running",
	}
	if err := config.Database.Create(&tp).Error; err != nil {
		return err
	}
	data := tp.ToMap()
	if tp.ConfigID != "" {
		var cfg models.ConfigPool
		if config.Database.Where("name = ?", tp.ConfigID).
			Order("CASE WHEN user_id = 'system' THEN 0 ELSE 1 END, id").First(&cfg).Error == nil {
			data["config_data"] = cfg.Data
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := client.SendRessources(ctx, &pb.RessourceRequest{
		User: job.OwnerEmail, Data: data, Status: pb.Status_CREATE, Type: pb.Type_SERVERPOOL,
	})
	if err != nil {
		return err
	}
	if resp != nil && !resp.GetSuccess() {
		return fmt.Errorf("le microservice a refusé la création du pool")
	}
	return nil
}

// waitForPoolVMs attend que n VMs du pool soient ACTIVE, avec IP, et joignables en SSH.
func waitForPoolVMs(poolID string, n int, timeout time.Duration) ([]models.Server, error) {
	deadline := time.Now().Add(timeout)
	for {
		var servers []models.Server
		config.Database.Where("serverpool_id = ? AND ip_address <> '' AND status = ?", poolID, "ACTIVE").
			Order("created_at ASC").Find(&servers)
		ready := servers[:0]
		for _, s := range servers {
			if sshReachable(s.IP_Address) {
				ready = append(ready, s)
			}
		}
		if len(ready) >= n {
			return ready[:n], nil
		}
		if time.Now().After(deadline) {
			if len(ready) > 0 {
				return ready, nil // dégradé : on lance sur ce qui est prêt
			}
			return nil, fmt.Errorf("délai dépassé (aucune VM prête)")
		}
		time.Sleep(10 * time.Second)
	}
}

// teardownTransientPool détruit le pool transitoire et ses VMs (même séquence que la
// suppression de pool : purge les serveurs en base puis DELETE SERVERPOOL côté microservice).
func teardownTransientPool(client pb.PoolManagerClient, owner, poolID string) {
	config.Database.Where("serverpool_id = ?", poolID).Delete(&models.Server{})
	config.Database.Where("serverpool_id = ? AND user_id = ?", poolID, owner).Delete(&models.Serverpool{})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _ = client.SendRessources(ctx, &pb.RessourceRequest{
		User: owner, Data: map[string]string{"serverpool_id": poolID}, Status: pb.Status_DELETE, Type: pb.Type_SERVERPOOL,
	})
}

// writeFileOnVM écrit un contenu dans un fichier sur la VM via SSH.
func writeFileOnVM(ip string, signer ssh.Signer, dest, content string) error {
	cfg := sshinject.SshConfig(sshVMUser(), signer)
	cfg.Timeout = 10 * time.Second
	cl, err := ssh.Dial("tcp", ip+":22", cfg)
	if err != nil {
		return err
	}
	defer cl.Close()
	sess, err := cl.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	sess.Stdin = strings.NewReader(content)
	return sess.Run("cat > '" + dest + "'")
}

func finishJob(jobID uint, status string, code int, log, vmName string) {
	now := time.Now().UTC()
	config.Database.Model(&models.BatchJob{}).Where("id = ?", jobID).Updates(map[string]any{
		"status": status, "exit_code": code, "log": log, "vm_name": vmName, "finished_at": now,
	})
	// Métriques : compteur par résultat + durée d'exécution (started_at → maintenant).
	metrics.RecordBatchJob(status)
	// Notification de fin (B5) : trace dans le journal d'audit (visible dans la cloche).
	var j models.BatchJob
	if config.Database.Select("owner_email", "name", "started_at").Where("id = ?", jobID).First(&j).Error == nil {
		if j.StartedAt != nil && !j.StartedAt.IsZero() {
			metrics.ObserveBatchJobDuration(now.Sub(*j.StartedAt).Seconds())
		}
		config.Database.Create(&models.AuditLog{
			Actor: j.OwnerEmail, Role: "system", Method: "JOB",
			Path: "/jobs/" + status + "/" + j.Name, IP: "-",
		})
	}
}

// sshReachable teste rapidement l'ouverture du port SSH.
func sshReachable(ip string) bool {
	conn, err := net.DialTimeout("tcp", ip+":22", 3*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// runScriptOnVM exécute le script (bash) sur la VM via SSH et renvoie la sortie
// combinée (stdout+stderr) et le code de sortie.
func runScriptOnVM(ip string, signer ssh.Signer, script string) (string, int, error) {
	cfg := sshinject.SshConfig(sshVMUser(), signer)
	cfg.Timeout = 15 * time.Second

	client, err := ssh.Dial("tcp", ip+":22", cfg)
	if err != nil {
		return "", -1, fmt.Errorf("connexion SSH: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", -1, fmt.Errorf("session SSH: %w", err)
	}
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf
	session.Stderr = &buf
	session.Stdin = strings.NewReader(script)

	// Garde-fou d'exécution : tue le job après batchMaxRuntime.
	done := make(chan error, 1)
	go func() { done <- session.Run("timeout " + fmt.Sprint(int(batchMaxRuntime.Seconds())) + " bash -s") }()
	select {
	case err = <-done:
	case <-time.After(batchMaxRuntime + time.Minute):
		_ = session.Signal(ssh.SIGKILL)
		return buf.String(), -1, fmt.Errorf("délai d'exécution dépassé")
	}

	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			return buf.String(), exitErr.ExitStatus(), nil
		}
		return buf.String(), -1, err
	}
	return buf.String(), 0, nil
}

// sshVMUser : utilisateur Linux des VMs (même logique que pour la diffusion de fichiers).
func sshVMUser() string {
	if u := strings.TrimSpace(os.Getenv("GUACAMOLE_SSH_USER")); u != "" {
		return u
	}
	return "vmuser"
}
