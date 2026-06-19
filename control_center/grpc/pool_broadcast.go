package grpc

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"control_center/config"
	"control_center/internal/sshinject"
	"control_center/models"

	"golang.org/x/crypto/ssh"
)

// Taille max d'un fichier diffusé (garde-fou).
const maxBroadcastBytes = 25 << 20 // 25 Mio

var safeSegment = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// sshVMUser : utilisateur Linux des VMs étudiantes (même que le monitoring/Guacamole).
func sshVMUser() string {
	if u := strings.TrimSpace(os.Getenv("GUACAMOLE_SSH_USER")); u != "" {
		return u
	}
	return "vmuser"
}

// sanitizeFilename garde un nom de fichier simple et sûr (pas de chemin, pas de '..').
func sanitizeFilename(name string) (string, bool) {
	name = path.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == ".." || !safeSegment.MatchString(name) {
		return "", false
	}
	return name, true
}

// sanitizeSubdir valide un sous-dossier relatif (segments sûrs, pas de '..' ni de '/').
func sanitizeSubdir(sub string) (string, bool) {
	sub = strings.Trim(strings.TrimSpace(sub), "/")
	if sub == "" {
		return "", true
	}
	for _, seg := range strings.Split(sub, "/") {
		if seg == "" || seg == ".." || !safeSegment.MatchString(seg) {
			return "", false
		}
	}
	return sub, true
}

// POST /api/pool/broadcast-file — pousse un fichier (sujet, jeu de données…) dans le
// home de chaque VM d'un pool, en une fois. Staff uniquement (préfixe /api/pool/).
// Corps JSON : {pool_id, user_id, filename, subdir?, content_b64}.
func handlePoolBroadcastFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONMoodle(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST requis"})
		return
	}
	var req struct {
		PoolID     string `json:"pool_id"`
		UserID     string `json:"user_id"`
		Filename   string `json:"filename"`
		Subdir     string `json:"subdir"`
		ContentB64 string `json:"content_b64"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONMoodle(w, http.StatusBadRequest, map[string]string{"error": "JSON invalide"})
		return
	}

	filename, ok := sanitizeFilename(req.Filename)
	if !ok {
		writeJSONMoodle(w, http.StatusBadRequest, map[string]string{"error": "nom de fichier invalide"})
		return
	}
	subdir, ok := sanitizeSubdir(req.Subdir)
	if !ok {
		writeJSONMoodle(w, http.StatusBadRequest, map[string]string{"error": "sous-dossier invalide"})
		return
	}
	if strings.TrimSpace(req.PoolID) == "" {
		writeJSONMoodle(w, http.StatusBadRequest, map[string]string{"error": "pool_id requis"})
		return
	}

	content, err := base64.StdEncoding.DecodeString(req.ContentB64)
	if err != nil {
		writeJSONMoodle(w, http.StatusBadRequest, map[string]string{"error": "contenu (base64) invalide"})
		return
	}
	if len(content) == 0 {
		writeJSONMoodle(w, http.StatusBadRequest, map[string]string{"error": "fichier vide"})
		return
	}
	if len(content) > maxBroadcastBytes {
		writeJSONMoodle(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "fichier trop volumineux (max 25 Mio)"})
		return
	}

	// Le pool appartient à un enseignant : un non-admin ne peut diffuser que sur SES pools.
	poolUserID := effectiveEmail(r, req.UserID)

	var servers []models.Server
	if err := config.Database.
		Where("serverpool_id = ? AND user_id = ? AND ip_address <> ''", req.PoolID, poolUserID).
		Find(&servers).Error; err != nil {
		writeJSONMoodle(w, http.StatusInternalServerError, map[string]string{"error": "lecture des VMs échouée"})
		return
	}
	if len(servers) == 0 {
		writeJSONMoodle(w, http.StatusNotFound, map[string]string{"error": "aucune VM joignable dans ce pool"})
		return
	}

	signer, err := sshinject.LoadPrivateKey(os.Getenv("SSH_PRIVATE_KEY_PATH"))
	if err != nil {
		writeJSONMoodle(w, http.StatusInternalServerError, map[string]string{"error": "clé SSH indisponible"})
		return
	}

	destDir := "/home/" + sshVMUser()
	if subdir != "" {
		destDir += "/" + subdir
	}

	type result struct {
		name string
		err  error
	}
	results := make([]result, len(servers))
	var wg sync.WaitGroup
	for i, srv := range servers {
		wg.Add(1)
		go func(i int, srv models.Server) {
			defer wg.Done()
			results[i] = result{name: srv.Name, err: pushFileToVM(srv.IP_Address, signer, destDir, filename, content)}
		}(i, srv)
	}
	wg.Wait()

	succeeded := 0
	var failures []map[string]string
	for _, res := range results {
		if res.err == nil {
			succeeded++
		} else {
			failures = append(failures, map[string]string{"vm": res.name, "error": res.err.Error()})
		}
	}

	writeJSONMoodle(w, http.StatusOK, map[string]any{
		"ok":        true,
		"total":     len(servers),
		"succeeded": succeeded,
		"failed":    len(servers) - succeeded,
		"path":      destDir + "/" + filename,
		"failures":  failures,
	})
}

// pushFileToVM écrit le contenu dans destDir/filename sur la VM via SSH (stdin → cat).
// destDir et filename sont pré-validés (caractères sûrs) → quote simple sans risque d'injection.
func pushFileToVM(ip string, signer ssh.Signer, destDir, filename string, content []byte) error {
	if strings.TrimSpace(ip) == "" {
		return fmt.Errorf("pas d'IP")
	}
	cfg := sshinject.SshConfig(sshVMUser(), signer)
	cfg.Timeout = 10 * time.Second

	client, err := ssh.Dial("tcp", ip+":22", cfg)
	if err != nil {
		return fmt.Errorf("connexion: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("session: %w", err)
	}
	defer session.Close()

	session.Stdin = bytes.NewReader(content)
	cmd := fmt.Sprintf("mkdir -p '%s' && cat > '%s/%s'", destDir, destDir, filename)
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("écriture: %w", err)
	}
	return nil
}
