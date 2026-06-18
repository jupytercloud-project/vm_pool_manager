package grpc

import (
	"control_center/config"
	"control_center/internal/guacamole"
	"control_center/models"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// vmActivityCache holds recent SSH activity reported via POST /api/vm-activity
// (used when vm-registrar is not running or vm_instances row is missing).
var vmActivityCache sync.Map // hostname -> activity status string

// guacClient is initialized in Start_grpc and used to build terminal URLs.
var guacClient *guacamole.Client

// InventoryVM wraps VMInstance with a derived Guacamole terminal URL.
type InventoryVM struct {
	models.VMInstance
	PowerState   string `json:"power_state,omitempty"` // état Nova live : ACTIVE | SHUTOFF | SUSPENDED…
	GuacURL      string `json:"guac_url,omitempty"`
	Student      string `json:"student,omitempty"`       // étudiant attribué (par IP), si VM étudiante
	IsInstructor bool   `json:"is_instructor,omitempty"` // VM de l'enseignant (la plus ancienne du pool)
}

// InventoryPool groups VMs by serverpool for the admin dashboard.
type InventoryPool struct {
	LinkedCourse string        `json:"linked_course,omitempty"` // cours lié (Moodle / X), si renseigné
	Label        string        `json:"label,omitempty"`         // nom d'affichage facultatif
	Tags         string        `json:"tags,omitempty"`          // étiquettes (CSV)
	PoolID       string        `json:"pool_id"`
	UserID       string        `json:"user_id"`
	VMs          []InventoryVM `json:"vms"`
}

const registrarStaleAfter = 30 * time.Minute

func buildInventory() ([]InventoryPool, error) {
	var activePools []models.Serverpool
	if err := config.Database.Find(&activePools).Error; err != nil {
		return nil, err
	}
	validPools := make(map[string]bool, len(activePools))
	linkedByKey := make(map[string]string, len(activePools))
	labelByKey := make(map[string]string, len(activePools))
	tagsByKey := make(map[string]string, len(activePools))
	for _, p := range activePools {
		k := p.ServerpoolID + ":" + p.UserID
		validPools[k] = true
		labelByKey[k] = p.Label
		tagsByKey[k] = p.Tags
		if p.XCourseCode != "" {
			linkedByKey[k] = "X · " + p.XCourseCode
		} else if p.MoodleCourseID != 0 {
			linkedByKey[k] = fmt.Sprintf("Moodle #%d", p.MoodleCourseID)
		}
	}

	registrarByName := map[string]models.VMInstance{}
	var registrarRows []models.VMInstance
	if err := config.Database.Order("name ASC").Find(&registrarRows).Error; err == nil {
		for _, vm := range registrarRows {
			// Garder les lignes stale si elles ont une connexion Guacamole enregistrée
			if time.Since(vm.LastSeen) > registrarStaleAfter && vm.GuacConnectionID == "" {
				continue
			}
			registrarByName[vm.Name] = vm
		}
	}

	var servers []models.Server
	if err := config.Database.Order("name ASC").Find(&servers).Error; err != nil {
		return nil, err
	}

	// IP -> nom de l'étudiant attribué (pour afficher qui est connecté sur quelle VM).
	studentByIP := map[string]string{}
	{
		type srow struct{ IP, Name string }
		var srows []srow
		config.Database.Model(&models.Student{}).Where("ip <> ''").Select("ip", "name").Scan(&srows)
		for _, s := range srows {
			if s.IP != "" {
				studentByIP[s.IP] = s.Name
			}
		}
	}
	// VM instructeur de chaque pool = la plus ancienne (created_at), cohérent avec attribvm/nbgrader.
	instructorID := map[string]string{}
	oldest := map[string]time.Time{}
	for _, srv := range servers {
		k := serverPoolID(srv) + ":" + serverUserID(srv)
		if k == ":" {
			continue
		}
		if t, ok := oldest[k]; !ok || srv.CreatedAt.Before(t) {
			oldest[k] = srv.CreatedAt
			instructorID[k] = srv.ID
		}
	}

	pools := make(map[string]*InventoryPool)
	seen := make(map[string]bool)

	// Collect all merged VMs first, then probe app ports in parallel.
	type pendingProbe struct {
		vm  models.VMInstance
		key string
		srv models.Server
	}
	var pending []pendingProbe

	for _, srv := range servers {
		poolID := serverPoolID(srv)
		userID := serverUserID(srv)
		if poolID == "" || userID == "" {
			continue
		}
		key := poolID + ":" + userID
		if !validPools[key] {
			continue
		}

		vm := mergeInventoryVM(srv, registrarByName[srv.Name])
		if _, ok := pools[key]; !ok {
			pools[key] = &InventoryPool{
				PoolID: poolID,
				UserID: userID,
				VMs:    []InventoryVM{},
			}
		}
		pending = append(pending, pendingProbe{vm: vm, key: key, srv: srv})
		seen[vm.Name] = true
	}

	// Probe app ports in parallel (bounded to 500ms timeout).
	var wg sync.WaitGroup
	for i := range pending {
		wg.Add(1)
		go func(p *pendingProbe) {
			defer wg.Done()
			probeAppPort(&p.vm)
		}(&pending[i])
	}
	wg.Wait()

	powerStates := fetchPowerStates()

	for _, p := range pending {
		ivm := toInventoryVM(p.vm)
		ivm.PowerState = powerStates[ivm.ID]
		if name, ok := studentByIP[p.srv.IP_Address]; ok {
			ivm.Student = name
		}
		if instructorID[p.key] == p.srv.ID {
			ivm.IsInstructor = true
		}
		pools[p.key].VMs = append(pools[p.key].VMs, ivm)
	}

	// Registrar-only rows (VM created before servers sync).
	for name, vm := range registrarByName {
		if seen[name] {
			continue
		}
		var meta map[string]string
		_ = json.Unmarshal(vm.RawMeta, &meta)
		poolID := meta["serverpool_id"]
		userID := meta["user_id"]
		if poolID == "" {
			continue
		}
		key := poolID + ":" + userID
		if !validPools[key] {
			continue
		}
		if _, ok := pools[key]; !ok {
			pools[key] = &InventoryPool{
				PoolID: poolID,
				UserID: userID,
				VMs:    []InventoryVM{},
			}
		}
		ivm := toInventoryVM(vm)
		ivm.PowerState = powerStates[ivm.ID]
		if name, ok := studentByIP[vm.IP]; ok {
			ivm.Student = name
		}
		pools[key].VMs = append(pools[key].VMs, ivm)
	}

	result := make([]InventoryPool, 0, len(pools))
	for _, p := range pools {
		k := p.PoolID + ":" + p.UserID
		p.LinkedCourse = linkedByKey[k]
		p.Label = labelByKey[k]
		p.Tags = tagsByKey[k]
		result = append(result, *p)
	}
	return result, nil
}

func toInventoryVM(vm models.VMInstance) InventoryVM {
	ivm := InventoryVM{VMInstance: vm}
	if guacClient != nil && vm.GuacConnectionID != "" {
		ivm.GuacURL = guacClient.BuildClientURL(vm.GuacConnectionID)
	}
	return ivm
}

func mergeInventoryVM(srv models.Server, reg models.VMInstance) models.VMInstance {
	meta := map[string]string{
		"serverpool_id": serverPoolID(srv),
		"user_id":       serverUserID(srv),
	}
	rawMeta, _ := json.Marshal(meta)

	vm := models.VMInstance{
		ID:             srv.ID,
		Name:           srv.Name,
		IP:             serverPrimaryIP(srv),
		Status:         mapServerStatus(srv.Status),
		Healthy:        isServerHealthy(srv.Status),
		ActivityStatus: "idle",
		LastSeen:       time.Now().UTC(),
		RawMeta:        rawMeta,
	}

	if reg.Name != "" {
		vm.Healthy = reg.Healthy
		vm.Status = reg.Status
		vm.LastSeen = reg.LastSeen
		vm.RegisteredAt = reg.RegisteredAt
		if reg.IP != "" {
			vm.IP = reg.IP
		}
		if reg.PublicIP != "" {
			vm.PublicIP = reg.PublicIP
		}
		if reg.ID != "" {
			vm.ID = reg.ID
		}
		if reg.ActivityStatus != "" && reg.ActivityStatus != "idle" {
			vm.ActivityStatus = reg.ActivityStatus
		}
		if reg.GuacConnectionID != "" {
			vm.GuacConnectionID = reg.GuacConnectionID
		}
		if reg.AppPort > 0 {
			vm.AppPort = reg.AppPort
		}
	}

	if cached, ok := vmActivityCache.Load(srv.Name); ok {
		if status, _ := cached.(string); status != "" && status != "idle" {
			vm.ActivityStatus = status
		}
	}

	return vm
}

// probeAppPort checks if the app port is reachable and marks the VM as active.
// Must be called outside of the main inventory loop (use in a goroutine or after merging).
func probeAppPort(vm *models.VMInstance) {
	if vm.IP == "" {
		return
	}
	port := vm.AppPort
	if port <= 0 {
		port = 8888 // Jupyter default
	}
	// Ask Jupyter Server how many live connections/kernels it has: >0 means
	// someone currently has the notebook open (or a kernel running) = "active".
	// Only upgrades to active (never downgrades an SSH-active VM).
	client := http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://%s:%d/api/status", vm.IP, port))
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var st struct {
		Connections int `json:"connections"`
		Kernels     int `json:"kernels"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&st); err == nil {
		if st.Connections > 0 || st.Kernels > 0 {
			vm.ActivityStatus = "active"
		}
	}
}

func serverPoolID(srv models.Server) string {
	if srv.ServerpoolID != "" {
		return srv.ServerpoolID
	}
	if srv.Metadata != nil {
		return srv.Metadata["serverpool_id"]
	}
	return ""
}

func serverUserID(srv models.Server) string {
	if srv.UserID != "" {
		return srv.UserID
	}
	if srv.Metadata != nil {
		return srv.Metadata["user_id"]
	}
	return ""
}

// handleGuacURL returns the Guacamole client URL for a VM given its IP.
func handleGuacURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ip := r.URL.Query().Get("ip")
	if ip == "" {
		http.Error(w, "missing ip parameter", http.StatusBadRequest)
		return
	}
	if guacClient == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"url": ""})
		return
	}
	var vm models.VMInstance
	if err := config.Database.Where("ip = ? AND guac_connection_id <> ''", ip).First(&vm).Error; err != nil {
		// Try matching by server IP
		var srv models.Server
		if err2 := config.Database.Where("ip_address = ?", ip).First(&srv).Error; err2 != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"url": ""})
			return
		}
		if err2 := config.Database.Where("name = ? AND guac_connection_id <> ''", srv.Name).First(&vm).Error; err2 != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"url": ""})
			return
		}
	}
	url := guacClient.BuildClientURL(vm.GuacConnectionID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": url})
}

// handleAppStatus checks if a TCP port is open on a VM (used to poll app readiness).
func handleAppStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ip := r.URL.Query().Get("ip")
	port := r.URL.Query().Get("port")
	if ip == "" || port == "" {
		http.Error(w, "missing ip or port", http.StatusBadRequest)
		return
	}
	conn, err := net.DialTimeout("tcp", ip+":"+port, 2*time.Second)
	ready := err == nil
	if ready {
		conn.Close()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ready": ready})
}

func RecordVMActivity(hostname, status string) {
	if hostname == "" || status == "" {
		return
	}
	vmActivityCache.Store(hostname, status)
	now := time.Now().UTC()
	result := config.Database.Model(&models.VMInstance{}).
		Where("name = ?", hostname).
		Updates(map[string]any{
			"activity_status": status,
			"last_seen":       now,
		})
	if result.RowsAffected == 0 {
		config.Database.Create(&models.VMInstance{
			ID:             hostname,
			Name:           hostname,
			Status:         "ready",
			Healthy:        true,
			ActivityStatus: status,
			LastSeen:       now,
			RegisteredAt:   now,
			RawMeta:        json.RawMessage("{}"),
		})
	}
}

func serverPrimaryIP(srv models.Server) string {
	if srv.IP_Address != "" {
		return srv.IP_Address
	}
	for _, net := range srv.Networks {
		if idx := strings.LastIndex(net, ":"); idx >= 0 && idx < len(net)-1 {
			return net[idx+1:]
		}
	}
	return ""
}

func mapServerStatus(openstackStatus string) string {
	switch strings.ToUpper(openstackStatus) {
	case "ACTIVE":
		return "ready"
	case "BUILD", "BUILDING":
		return "starting"
	default:
		return "starting"
	}
}

func isServerHealthy(status string) bool {
	return strings.EqualFold(status, "ACTIVE")
}
