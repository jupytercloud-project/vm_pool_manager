package worker

import (
	"PoolManagerVM/backend/internal/jobs"
	"PoolManagerVM/backend/models"
	"PoolManagerVM/backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/google/uuid"
)

type JobType int

const (
	CreateVM JobType = iota
	CreateVMAdmin
	DeleteVM
	AttribVM
)

type Job struct {
	ID   uint64
	Name string
	Type JobType
	Data map[string]string
	// retryCount int
}

var (
	HighPriorityJobs   chan Job
	NormalPriorityJobs chan Job
	jobCount           uint64
)

func LaunchWorkers(numWorkers int, wg *sync.WaitGroup, ctx context.Context) {
	HighPriorityJobs = make(chan Job, 50)
	NormalPriorityJobs = make(chan Job, 100)

	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go worker(i, wg, ctx)
	}
}

func worker(id int, wg *sync.WaitGroup, ctx context.Context) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d stopping\n", id)
			return
		case job, ok := <-HighPriorityJobs:
			if !ok {
				HighPriorityJobs = nil
				continue
			}
			processJob(id, job)
		case job, ok := <-NormalPriorityJobs:
			if !ok {
				NormalPriorityJobs = nil
				continue
			}
			processJob(id, job)
		}
	}
}

func processJob(workerID int, job Job) {
	switch job.Type {
	case CreateVM:
		metadata := map[string]string{}
		if metaStr, ok := job.Data["Metadata"]; ok && metaStr != "" {
			if err := json.Unmarshal([]byte(metaStr), &metadata); err != nil {
				log.Println("Error unmarshall metadata: ", err)
			}
		}
		metadata["user_id"] = job.Data["user_id"]
		metadata["serverpool_id"] = job.Data["serverpool_id"]
		metadata["min_vm"] = job.Data["min_vm"]
		metadata["max_vm"] = job.Data["max_vm"]

		var networks models.JSONStringSlice
		if err := networks.Scan(job.Data["networks"]); err != nil {
			log.Println("Failed to parse networks:", err)
			networks = models.JSONStringSlice{} // fallback
		}

		paramID := utils.ParseInt(job.Data["paramID"])
		fmt.Println("Worker ", workerID, " takes the job of creating a VM")
		jobs.CreateVM(models.Server{
			Name:         fmt.Sprintf(`%s-%s`, job.Data["name"], uuid.New().String()),
			FlavorRef:    job.Data["flavor_ref"],
			ImageRef:     job.Data["image_ref"],
			UserID:       job.Data["user_id"],
			ServerpoolID: job.Data["serverpool_id"],
			Metadata:     metadata,
			Networks:     networks,
		}, uint(paramID))
		fmt.Println("Worker ", workerID, " finished its job")
	}
}

func CreateJob(name string, JobType JobType, data map[string]string) *Job {
	jobCount++
	return &Job{
		ID:   jobCount,
		Name: name,
		Type: JobType,
		Data: data,
	}
}

func AddJob(job Job, highPriority bool) {
	fmt.Printf("Adding job %d to queue\n", job.ID)
	if highPriority {
		HighPriorityJobs <- job
	} else {
		NormalPriorityJobs <- job
	}
}
