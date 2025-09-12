package internal

import (
	"PoolManagerVM/backend/internal/jobs"
	"PoolManagerVM/backend/internal/worker"
	"PoolManagerVM/backend/models"
	"PoolManagerVM/backend/utils"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

func Monitor(c context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Done():
			log.Println("Monitoring stopped")
			return

		case <-ticker.C:
			log.Println("Checking serverpools...")
			CheckAndCreate()
		}
	}
}

// works only if one type of server in pool
func CheckAndCreate() {
	serverpools, err := utils.GetAllServerPool()
	if err != nil {
		log.Println("Error, fetching serverpool from OpenStack:", err)
	}
	for _, sp := range serverpools {
		mapToCreate := utils.MissingServersByParam(sp)
		for i, missing := range mapToCreate {
			for j := 0; j < missing; j++ {
				worker.AddJob(*worker.CreateJob(sp.ServerpoolID, worker.CreateVM, NewCreateServerJob(sp, sp.Params[i])), false)
				jobs.IncrementPending(sp.ServerpoolID, sp.UserID, i)
			}
		}
	}

	pool, found := FindServerpool(serverpools, "admin", "PoolVms")
	if found {
		fmt.Println("base found : ", pool)
	} else {
		log.Println("Base serverpool not found, creating")
		createBaseServerJob()
	}
}

func createBaseServerJob() {
	pool, param, err := CreateServerpoolFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	worker.AddJob(*worker.CreateJob(os.Getenv("SERVER_NAME"), worker.CreateVM, NewCreateServerJob(pool, param)), false)
	jobs.IncrementPending(pool.ServerpoolID, pool.UserID, 1)
}

func CreateServerpoolFromEnv() (models.Serverpool, models.Param, error) {
	// Lire les variables d'environnement
	imageRef := os.Getenv("SERVER_IMAGE_REF")
	flavorRef := os.Getenv("SERVER_FLAVOR_REF")
	poolID := os.Getenv("METADATA_SERVERPOOL_ID")
	userID := os.Getenv("METADATA_USER_ID")

	minVMStr := os.Getenv("METADATA_MIN_VM")
	maxVMStr := os.Getenv("METADATA_MAX_VM")

	// Convertir MinVM et MaxVM en int
	minVM, err := strconv.Atoi(minVMStr)
	if err != nil {
		return models.Serverpool{}, models.Param{}, err
	}
	maxVM, err := strconv.Atoi(maxVMStr)
	if err != nil {
		return models.Serverpool{}, models.Param{}, err
	}

	// Construire le param
	param := models.Param{
		ServerpoolID: poolID,
		UserID:       userID,
		ImageRef:     imageRef,
		FlavorRef:    flavorRef,
		Networks:     models.JSONStringSlice{os.Getenv("NETWORK_ID")},
		MinVM:        minVM,
		MaxVM:        maxVM,
		PendingJobs:  0,
	}

	// Construire le serverpool
	pool := models.Serverpool{
		ServerpoolID: poolID,
		UserID:       userID,
		Params:       []models.Param{param},
		ListServ:     []models.Server{}, // vide au départ
	}

	return pool, param, nil
}

// Exemple : création d’un Job pour un serveur à partir d’un Param
func NewCreateServerJob(pool models.Serverpool, param models.Param) map[string]string {
	networkValue, err := param.Networks.Value()
	if err != nil {
		networkValue = []byte("[]") // fallback
	}

	// convertir []byte en string
	networkBytes, ok := networkValue.([]byte)
	if !ok {
		networkBytes = []byte("[]")
	}
	networkJSON := string(networkBytes)

	return utils.BuildDataMap(
		"name", pool.ServerpoolID,
		"serverpool_id", pool.ServerpoolID,
		"user_id", pool.UserID,
		"image_ref", param.ImageRef,
		"flavor_ref", param.FlavorRef,
		"networks", networkJSON,
		"paramID", strconv.FormatUint(uint64(param.ID), 10),
		"min_vm", strconv.Itoa(param.MinVM),
		"max_vm", strconv.Itoa(param.MaxVM),
	)
}

// FindServerpool cherche un Serverpool dans un slice à partir de userID et serverpoolID
func FindServerpool(pools []models.Serverpool, userID, poolID string) (*models.Serverpool, bool) {
	for i := range pools {
		if pools[i].UserID == userID && pools[i].ServerpoolID == poolID {
			return &pools[i], true
		}
	}
	return nil, false // pas trouvé
}
