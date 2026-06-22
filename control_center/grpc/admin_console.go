package grpc

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"control_center/config"
	"control_center/models"
	"control_center/pb"

	"github.com/danielgtaylor/huma/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// registerAdminConsoleHuma : console admin globale, kill-switch et purge des orphelins (admin only).
func registerAdminConsoleHuma(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "admin-console", Method: http.MethodGet, Path: "/api/admin/console",
		Summary: "Vue d'ensemble admin (pools, VMs, orphelins, alertes)", Tags: []string{"admin"},
	}, func(ctx context.Context, _ *struct{}) (*AnyOutput, error) {
		return &AnyOutput{Body: buildAdminConsole()}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "admin-kill-switch", Method: http.MethodPost, Path: "/api/admin/kill-switch",
		Summary: "Arrêt d'urgence de toutes les VMs ACTIVE", Tags: []string{"admin"},
	}, func(ctx context.Context, _ *struct{}) (*AnyOutput, error) {
		return adminKillSwitch()
	})
	huma.Register(api, huma.Operation{
		OperationID: "admin-cleanup-orphans", Method: http.MethodPost, Path: "/api/admin/cleanup-orphans",
		Summary: "Supprimer les serveurs orphelins", Tags: []string{"admin"},
	}, func(ctx context.Context, _ *struct{}) (*AnyOutput, error) {
		return adminCleanupOrphans()
	})
}

// Ports sensibles qui ne devraient JAMAIS être ouverts à l'extérieur (G1).
var sensitivePorts = map[int]string{
	3306: "MySQL", 5432: "PostgreSQL", 6379: "Redis", 27017: "MongoDB",
	9200: "Elasticsearch", 5984: "CouchDB", 11211: "Memcached", 2375: "Docker", 9000: "App/Portainer",
}

// buildAdminConsole — vue d'ensemble admin (K1) : pools, VMs, utilisateurs,
// VMs orphelines, et alertes de sécurité (ports sensibles exposés — G1).
func buildAdminConsole() map[string]any {
	inv, _ := buildInventory()

	// Utilisateurs.
	type userRow struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	var users []userRow
	config.Database.Model(&models.User{}).Select("email", "role").Order("role, email").Scan(&users)

	// VMs orphelines : serveurs dont le pool (serverpool_id,user_id) n'existe plus.
	validPool := map[string]bool{}
	var pools []models.Serverpool
	config.Database.Find(&pools)
	for _, p := range pools {
		validPool[p.ServerpoolID+":"+p.UserID] = true
	}
	type orphan struct {
		Name   string `json:"name"`
		ID     string `json:"id"`
		IP     string `json:"ip"`
		PoolID string `json:"pool_id"`
		UserID string `json:"user_id"`
	}
	var servers []models.Server
	config.Database.Find(&servers)
	var orphans []orphan
	totalVMs := 0
	for _, s := range servers {
		totalVMs++
		if !validPool[serverPoolID(s)+":"+serverUserID(s)] {
			orphans = append(orphans, orphan{Name: s.Name, ID: s.ID, IP: s.IP_Address, PoolID: serverPoolID(s), UserID: serverUserID(s)})
		}
	}

	// Scan de sécurité à la demande : ports sensibles ouverts sur les VMs joignables.
	type alert struct {
		VM      string `json:"vm"`
		IP      string `json:"ip"`
		Port    int    `json:"port"`
		Service string `json:"service"`
	}
	var alerts []alert
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 32) // borne la concurrence
	for _, s := range servers {
		ip := s.IP_Address
		if ip == "" {
			continue
		}
		for port, svc := range sensitivePorts {
			wg.Add(1)
			sem <- struct{}{}
			go func(vm, ip, svc string, port int) {
				defer wg.Done()
				defer func() { <-sem }()
				conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, itoa(port)), 1200*time.Millisecond)
				if err == nil {
					conn.Close()
					mu.Lock()
					alerts = append(alerts, alert{VM: vm, IP: ip, Port: port, Service: svc})
					mu.Unlock()
				}
			}(s.Name, ip, svc, port)
		}
	}
	wg.Wait()

	activeVMs := 0
	for _, p := range inv {
		for _, vm := range p.VMs {
			if vm.PowerState == "ACTIVE" {
				activeVMs++
			}
		}
	}

	return map[string]any{
		"stats": map[string]int{
			"pools": len(pools), "vms": totalVMs, "active": activeVMs,
			"users": len(users), "orphans": len(orphans), "alerts": len(alerts),
		},
		"pools":   inv,
		"users":   users,
		"orphans": orphans,
		"alerts":  alerts,
	}
}

// adminKillSwitch — arrêt d'urgence : stoppe toutes les VMs ACTIVE.
func adminKillSwitch() (*AnyOutput, error) {
	conn, err := grpc.NewClient("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, huma.Error502BadGateway("microservice injoignable")
	}
	defer conn.Close()
	client := pb.NewPoolManagerClient(conn)

	powerStates := fetchPowerStates()
	var servers []models.Server
	config.Database.Find(&servers)
	stopped := 0
	for _, s := range servers {
		if powerStates[s.ID] != "ACTIVE" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		_, err := client.SendRessources(ctx, &pb.RessourceRequest{
			User: s.UserID, Status: pb.Status_UPDATE, Type: pb.Type_SERVER,
			Data: map[string]string{"id": s.ID, "action": "stop"},
		})
		cancel()
		if err == nil {
			stopped++
		}
	}
	invalidatePowerStates()
	return &AnyOutput{Body: map[string]any{"ok": true, "stopped": stopped}}, nil
}

// adminCleanupOrphans — supprime les serveurs orphelins (pool inexistant).
func adminCleanupOrphans() (*AnyOutput, error) {
	validPool := map[string]bool{}
	var pools []models.Serverpool
	config.Database.Find(&pools)
	for _, p := range pools {
		validPool[p.ServerpoolID+":"+p.UserID] = true
	}
	conn, _ := grpc.NewClient("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	var client pb.PoolManagerClient
	if conn != nil {
		defer conn.Close()
		client = pb.NewPoolManagerClient(conn)
	}
	var servers []models.Server
	config.Database.Find(&servers)
	removed := 0
	for _, s := range servers {
		if validPool[serverPoolID(s)+":"+serverUserID(s)] {
			continue
		}
		// Détruire la VM côté OpenStack puis purger la ligne.
		if client != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			_, _ = client.SendRessources(ctx, &pb.RessourceRequest{
				User: s.UserID, Status: pb.Status_DELETE, Type: pb.Type_SERVER,
				Data: map[string]string{"name": s.Name, "server_id": s.ID},
			})
			cancel()
		}
		config.Database.Where("id = ?", s.ID).Delete(&models.Server{})
		removed++
	}
	invalidatePowerStates()
	return &AnyOutput{Body: map[string]any{"ok": true, "removed": removed}}, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [6]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
