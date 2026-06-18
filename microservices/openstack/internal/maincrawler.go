package internal

import (
	"PoolManagerVM/backend/config"
	"PoolManagerVM/backend/internal/jobs"
	"PoolManagerVM/backend/internal/worker"
	"PoolManagerVM/backend/models"
	"PoolManagerVM/backend/utils"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// deleteInFlight tracks VMs we have already queued for scale-down deletion, so
// the 2s reconcile loop doesn't re-queue the same deletions every tick while
// OpenStack tears them down and the DB sync catches up (a delete-runaway,
// symmetric to the create bug). Entries self-expire.
var (
	deleteInFlight   = map[string]time.Time{}
	deleteInFlightMu sync.Mutex
)

func markDeleting(id string) {
	deleteInFlightMu.Lock()
	deleteInFlight[id] = time.Now()
	deleteInFlightMu.Unlock()
}

func isDeleting(id string) bool {
	deleteInFlightMu.Lock()
	defer deleteInFlightMu.Unlock()
	t, ok := deleteInFlight[id]
	if !ok {
		return false
	}
	if time.Since(t) > 90*time.Second {
		delete(deleteInFlight, id)
		return false
	}
	return true
}

// isWarmPool reports whether p is the shared pre-warmed pool that feeds student
// attribution — never scaled down here, or attribution would starve.
func isWarmPool(p models.Serverpool) bool {
	return p.ServerpoolID == "pool_vms" && p.UserID == "admin"
}

// powerInFlight throttles stop/start jobs so the 2s loop doesn't re-enqueue them
// every tick while OpenStack transitions the VM (ACTIVE <-> SHUTOFF).
var (
	powerInFlight   = map[string]time.Time{}
	powerInFlightMu sync.Mutex
)

func markPower(id string) {
	powerInFlightMu.Lock()
	powerInFlight[id] = time.Now()
	powerInFlightMu.Unlock()
}

func powerThrottled(id string) bool {
	powerInFlightMu.Lock()
	defer powerInFlightMu.Unlock()
	t, ok := powerInFlight[id]
	if !ok {
		return false
	}
	if time.Since(t) > 60*time.Second {
		delete(powerInFlight, id)
		return false
	}
	return true
}

// isOffDay reports whether today is one of the pool's off-days. On those days the
// pool's VMs are powered off (stopped, not deleted) to free resources, and powered
// back on the next day. OffDays is a CSV of lowercase English weekdays.
func isOffDay(p models.Serverpool) bool {
	if p.OffDays == "" {
		return false
	}
	today := strings.ToLower(time.Now().Weekday().String())
	for _, d := range strings.Split(p.OffDays, ",") {
		if strings.ToLower(strings.TrimSpace(d)) == today {
			return true
		}
	}
	return false
}

func Monitor(c context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Done():
			log.Println("Monitoring stopped")
			return

		case <-ticker.C:
			// Garde-fou : un panic dans une itération est journalisé sans arrêter le crawler.
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[recover] panic dans le crawler: %v\n%s", r, debug.Stack())
					}
				}()
				CheckAndCreate()
			}()
		}
	}
}

func CheckAndCreate() {

	var (
		servs        []models.Server
		pools        []models.Serverpool
		servadminmap = make(map[string]bool)
	)

	config.DBmu.Lock()
	defer config.DBmu.Unlock() // libéré même en cas de panic (évite tout deadlock)
	res_servs := config.Database.Find(&servs)
	if res_servs.Error != nil {
		log.Println(res_servs.Error)
		return
	}
	res_pools := config.Database.Find(&pools)
	if res_pools.Error != nil {
		log.Println(res_pools.Error)
		return
	}

	countadmin := 0
	for _, p := range pools {
		count := 0
		var poolServers []models.Server
		for _, s := range servs {
			if serverisinpool(p, s) {
				count++
				poolServers = append(poolServers, s)
			}
			if s.UserID == "admin" {
				if !servadminmap[s.ID] {
					servadminmap[s.ID] = true
					countadmin++
				}
			}
		}

		// Off-days: the pool's machines still exist (we keep provisioning up to
		// MinVM below) but are powered OFF (kept, not deleted) on closed days to
		// free compute, and powered back ON otherwise.
		if !isWarmPool(p) {
			if isOffDay(p) {
				for _, s := range poolServers {
					if strings.EqualFold(s.Status, "ACTIVE") && !powerThrottled(s.ID) {
						markPower(s.ID)
						worker.AddJob(*worker.CreateJob(models.StopVM, map[string]string{"instance_id": s.ID}), false)
						log.Printf("[off-day] pool %s/%s: stopping %s", p.ServerpoolID, p.UserID, s.ID)
					}
				}
			} else {
				for _, s := range poolServers {
					// Ne pas relancer une VM arrêtée VOLONTAIREMENT (ManualOff) par un admin.
					if strings.EqualFold(s.Status, "SHUTOFF") && !s.ManualOff && !powerThrottled(s.ID) {
						markPower(s.ID)
						worker.AddJob(*worker.CreateJob(models.StartVM, map[string]string{"instance_id": s.ID}), false)
						log.Printf("[off-day] pool %s/%s: starting %s", p.ServerpoolID, p.UserID, s.ID)
					}
				}
			}
		}

		missing := p.MinVM - (count + p.PendingJobs)
		if !shouldStartPool(p.TimeStart) {
			continue
		}
		for range missing {
			if p.ImageRef == os.Getenv("SERVER_IMAGE_REF") &&
				p.FlavorRef == os.Getenv("SERVER_FLAVOR_REF") &&
				len(p.Networks) == 1 &&
				p.Networks[0] == os.Getenv("NETWORK_ID") &&
				countadmin > 0 && p.UserID != "admin" &&
				p.PendingJobs < missing {
				jobs.IncrementPending(p.ID)
				worker.AddJob((*worker.CreateJob(models.AttribVM,
					map[string]string{
						"ID":            fmt.Sprint(p.ID),
						"serverpool_id": p.ServerpoolID,
						"user_id":       p.UserID,
						"min_vm":        fmt.Sprint(p.MinVM),
						"max_vm":        fmt.Sprint(p.MaxVM),
						"config_id":     fmt.Sprint(p.ConfigID),
					})), true)
				countadmin--
			} else {
				jobs.IncrementPending(p.ID)
				worker.AddJob(*worker.CreateJob(models.CreateVM,
					utils.BuildDataMap(utils.FlatstringSP(p))), false)
			}
		}

		// Scale-down: never keep more than MaxVM VMs in a pool. Only delete
		// unattributed (Reattrib==false), ACTIVE VMs, and never the warm pool —
		// so student VMs and in-flight builds are left untouched.
		if p.MaxVM > 0 && count > p.MaxVM && !isWarmPool(p) {
			excess := count - p.MaxVM
			for _, s := range servs {
				if excess <= 0 {
					break
				}
				if !serverisinpool(p, s) || s.Reattrib {
					continue
				}
				if !strings.EqualFold(s.Status, "ACTIVE") || isDeleting(s.ID) {
					continue
				}
				markDeleting(s.ID)
				worker.AddJob(*worker.CreateJob(models.DeleteVM,
					map[string]string{"instance_id": s.ID}), true)
				log.Printf("[scale-down] pool %s/%s over max (%d>%d): deleting %s",
					p.ServerpoolID, p.UserID, count, p.MaxVM, s.ID)
				excess--
			}
		}
	}

	found := false
	for _, sp := range pools {
		if sp.ServerpoolID == "pool_vms" && sp.UserID == "admin" {
			found = true
			break
		}
	}
	if !found {
		base_p, err := CreateServerpoolFromEnv()
		if err != nil {
			log.Println("Error: can't create param from env: ", err)
		}
		if err := config.Database.First(&base_p,
			"serverpool_id = ? AND user_id = ?",
			base_p.ServerpoolID, base_p.UserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				config.Database.Create(&base_p)
			} else {
				log.Println("Error Database: ", err)
			}
		}
		for i := 0; i < base_p.MinVM; i++ {
			worker.AddJob(*worker.CreateJob(models.CreateVM,
				utils.BuildDataMap(utils.FlatstringSP(base_p))), false)
			jobs.IncrementPending(base_p.ID)
		}
	}
}

// serverisinpool tests pool membership by the canonical (serverpool_id, user_id)
// key — the same key used by the inventory. Flavor/image are intentionally NOT
// compared: image UUIDs are resolved at create time (and change when a snapshot
// is rebuilt), so a freshly created VM legitimately carries a different image
// ref than the pool's stored one. Comparing them made the reconciler unable to
// count existing VMs, so it kept re-creating them forever.
func serverisinpool(p models.Serverpool, s models.Server) bool {
	return s.ServerpoolID == p.ServerpoolID && s.UserID == p.UserID
}

func CreateServerpoolFromEnv() (models.Serverpool, error) {
	imageRef := os.Getenv("SERVER_IMAGE_REF")
	flavorRef := os.Getenv("SERVER_FLAVOR_REF")
	poolID := os.Getenv("METADATA_SERVERPOOL_ID")
	userID := os.Getenv("METADATA_USER_ID")
	minVMStr := os.Getenv("METADATA_MIN_VM")
	maxVMStr := os.Getenv("METADATA_MAX_VM")

	minVM, err := strconv.Atoi(minVMStr)
	if err != nil {
		return models.Serverpool{}, err
	}
	maxVM, err := strconv.Atoi(maxVMStr)
	if err != nil {
		return models.Serverpool{}, err
	}

	pool := models.Serverpool{
		ServerpoolID: poolID,
		UserID:       userID,
		ImageRef:     imageRef,
		FlavorRef:    flavorRef,
		Networks:     models.JSONStringSlice{os.Getenv("NETWORK_ID")},
		MinVM:        minVM,
		MaxVM:        maxVM,
		PendingJobs:  0,
		NetworkUuid:  os.Getenv("NETWORK_ID"),
	}

	return pool, nil
}

func shouldStartPool(_ string) bool {
	return true
}
