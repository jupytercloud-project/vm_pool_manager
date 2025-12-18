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
	"strconv"
	"time"

	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/volumes"
	"gorm.io/gorm"
)

func Monitor(c context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Done():
			log.Println("Monitoring stopped")
			return

		case <-ticker.C:
			CheckAndCreate()
			attachVolume()
			volnotattached()
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
	res_servs := config.Database.Find(&servs)
	if res_servs.Error != nil {
		log.Println(res_servs.Error)
		config.DBmu.Unlock()
		return
	}
	res_pools := config.Database.Find(&pools)
	if res_pools.Error != nil {
		log.Println(res_pools.Error)
		config.DBmu.Unlock()
		return
	}

	countadmin := 0
	for _, p := range pools {
		count := 0
		for _, s := range servs {
			if serverisinpool(p, s) {
				count++
			}
			if s.UserID == "admin" {
				if !servadminmap[s.ID] {
					servadminmap[s.ID] = true
					countadmin++
				}
			}
		}
		missing := p.MinVM - (count + p.PendingJobs)
		for i := 0; i < missing; i++ {
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
	config.DBmu.Unlock()
}

func serverisinpool(p models.Serverpool, s models.Server) bool {
	if s.ServerpoolID == p.ServerpoolID &&
		s.UserID == p.UserID &&
		s.FlavorRef == p.FlavorRef &&
		s.ImageRef == p.ImageRef {
		return true
	} else {
		return false
	}
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

func attachVolume() {
	allServ, err := utils.GetAllServers()
	if err != nil {
		log.Println("Failed to get all servers:", err)
		return
	}
	config.DBmu.Lock()
	for _, serv := range allServ {
		var server models.Server
		if err := config.Database.Select("vol_pending").
			Where("id = ?", serv.ID).First(&server).Error; err != nil {
			log.Println("Error fetching updated vol_pending:", err)
			config.DBmu.Unlock()
			return
		}
		if utils.NoVolAttached(serv) &&
			utils.NoVolAttachedDB(models.FromGopherServer(serv),
				config.Database) &&
			serv.Status == "ACTIVE" &&
			!server.VolPending {
			jobs.ChangePendingVol(serv.ID)
			worker.AddJob(*worker.CreateJob(models.CreateVolumeAndAttach,
				map[string]string{
					"size":        os.Getenv("VOLUME_SIZE"),
					"description": os.Getenv("VOLUME_DESCRIPTION"),
					"name":        os.Getenv("VOLUME_NAME"),
					"volume_type": os.Getenv("VOLUME_TYPE"),
					"server_id":   serv.ID,
				}), false)
		}
	}
	config.DBmu.Unlock()
}

func volnotattached() {
	allVol := utils.GetAllVolumes(context.Background())
	if allVol == nil {
		log.Println("Failed to get all volumes")
		return
	}
	for _, vol := range allVol {
		if len(vol.Attachments) == 0 && vol.Status == "available" &&
			!servstillinuse(vol) {
			worker.AddJob(*worker.CreateJob(models.DeleteVolume,
				map[string]string{
					"instance_id": vol.ID,
				}), false)
		}
	}
}

func servstillinuse(v volumes.Volume) bool {
	allserv, err := utils.GetAllServers()
	if err != nil {
		log.Println("Failed to get all servers:", err)
		return true
	}
	for _, serv := range allserv {
		if v.Metadata["instance_id"] == serv.ID {
			return true
		}
	}
	return false
}
