package grpc

import (
	"control_center/config"
	"control_center/models"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// vmActivityCache holds recent SSH activity reported via POST /api/vm-activity
// (used when vm-registrar is not running or vm_instances row is missing).
var vmActivityCache sync.Map // hostname -> activity status string

// InventoryPool groups VMs by serverpool for the admin dashboard.
type InventoryPool struct {
	PoolID string              `json:"pool_id"`
	UserID string              `json:"user_id"`
	VMs    []models.VMInstance `json:"vms"`
}

const registrarStaleAfter = 30 * time.Minute

func buildInventory() ([]InventoryPool, error) {
	var activePools []models.Serverpool
	if err := config.Database.Find(&activePools).Error; err != nil {
		return nil, err
	}
	validPools := make(map[string]bool, len(activePools))
	for _, p := range activePools {
		validPools[p.ServerpoolID+":"+p.UserID] = true
	}

	registrarByName := map[string]models.VMInstance{}
	var registrarRows []models.VMInstance
	if err := config.Database.Order("name ASC").Find(&registrarRows).Error; err == nil {
		for _, vm := range registrarRows {
			if time.Since(vm.LastSeen) > registrarStaleAfter {
				continue
			}
			registrarByName[vm.Name] = vm
		}
	}

	var servers []models.Server
	if err := config.Database.Order("name ASC").Find(&servers).Error; err != nil {
		return nil, err
	}

	pools := make(map[string]*InventoryPool)
	seen := make(map[string]bool)

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
				VMs:    []models.VMInstance{},
			}
		}
		pools[key].VMs = append(pools[key].VMs, vm)
		seen[vm.Name] = true
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
				VMs:    []models.VMInstance{},
			}
		}
		pools[key].VMs = append(pools[key].VMs, vm)
	}

	result := make([]InventoryPool, 0, len(pools))
	for _, p := range pools {
		result = append(result, *p)
	}
	return result, nil
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
	}

	if cached, ok := vmActivityCache.Load(srv.Name); ok {
		if status, _ := cached.(string); status != "" && status != "idle" {
			vm.ActivityStatus = status
		}
	}

	return vm
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
