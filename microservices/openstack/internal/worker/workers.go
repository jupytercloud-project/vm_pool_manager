package worker

import (
	"PoolManagerVM/backend/internal/jobs"
	"PoolManagerVM/backend/models"
	"context"

	"log"
	"runtime/debug"
	"sync"
)

var (
	HighPriorityJobs   chan models.Job
	NormalPriorityJobs chan models.Job
)

func LaunchWorkers(numWorkers int, wg *sync.WaitGroup, ctx context.Context) {
	HighPriorityJobs = make(chan models.Job, 50)
	NormalPriorityJobs = make(chan models.Job, 100)

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

func processJob(workerID int, job models.Job) {
	// Garde-fou : un job qui panique est journalisé sans tuer le worker (ni le process).
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[recover] panic dans le job %v (worker %d): %v\n%s", job.Type, workerID, r, debug.Stack())
		}
	}()
	switch job.Type {
	case models.CreateVM:
		jobs.CreateVM(workerID, job)

	case models.AttribVM:
		jobs.AttribVM(workerID, job)

	case models.DeleteVM:
		instanceID := job.Data["instance_id"]
		err := jobs.DeleteVM(instanceID)
		if err != nil {
			log.Println("Failed to delete VM:", err)
		} else {
			log.Println("VM deleted successfully:", instanceID)
		}

	case models.StopVM:
		if err := jobs.StopVM(job.Data["instance_id"]); err != nil {
			log.Println("Failed to stop VM:", err)
		} else {
			log.Println("VM stopped (off-day):", job.Data["instance_id"])
		}

	case models.StartVM:
		if err := jobs.StartVM(job.Data["instance_id"]); err != nil {
			log.Println("Failed to start VM:", err)
		} else {
			log.Println("VM started:", job.Data["instance_id"])
		}

	case models.ResizeVM:
		if err := jobs.ResizeVM(job.Data["instance_id"], job.Data["flavor_ref"]); err != nil {
			log.Println("Failed to resize VM:", err)
		} else {
			log.Println("VM resized:", job.Data["instance_id"])
		}
	}
}

func CreateJob(JobType models.JobType, data map[string]string) *models.Job {
	return &models.Job{
		Type: JobType,
		Data: data,
	}
}

func AddJob(job models.Job, highPriority bool) {
	log.Println("Adding job to queue")
	if highPriority {
		HighPriorityJobs <- job
	} else {
		NormalPriorityJobs <- job
	}
}
