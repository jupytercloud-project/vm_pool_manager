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
	// Choisir une VM joignable du pool cible.
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

	signer, err := sshinject.LoadPrivateKey(os.Getenv("SSH_PRIVATE_KEY_PATH"))
	if err != nil {
		finishJob(job.ID, "failed", -1, "Clé SSH indisponible: "+err.Error(), target.Name)
		return
	}

	out, code, runErr := runScriptOnVM(target.IP_Address, signer, job.Script)
	if len(out) > batchMaxLog {
		out = out[len(out)-batchMaxLog:] // garder la fin (la plus utile)
	}
	status := "succeeded"
	if runErr != nil || code != 0 {
		status = "failed"
		if runErr != nil && out == "" {
			out = runErr.Error()
		}
	}
	finishJob(job.ID, status, code, out, target.Name)

	// B4 : auto-arrêt — suspendre la VM en fin de job.
	if job.AutoStop && client != nil {
		_ = suspendServer(client, *target)
	}
}

func finishJob(jobID uint, status string, code int, log, vmName string) {
	now := time.Now().UTC()
	config.Database.Model(&models.BatchJob{}).Where("id = ?", jobID).Updates(map[string]any{
		"status": status, "exit_code": code, "log": log, "vm_name": vmName, "finished_at": now,
	})
	// Notification de fin (B5) : trace dans le journal d'audit (visible dans la cloche).
	var j models.BatchJob
	if config.Database.Select("owner_email", "name").Where("id = ?", jobID).First(&j).Error == nil {
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
