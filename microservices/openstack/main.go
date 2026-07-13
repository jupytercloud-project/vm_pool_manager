package main

import (
	"PoolManagerVM/backend/config"
	ss "PoolManagerVM/backend/grpc"
	"PoolManagerVM/backend/internal"
	"PoolManagerVM/backend/internal/metrics"
	"PoolManagerVM/backend/internal/telemetry"
	"PoolManagerVM/backend/internal/worker"
	"PoolManagerVM/backend/models"
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {

	// loading .env
	config.LoadEnvConfig()

	// OpenTelemetry (traces + métriques + logs OTLP). No-op si non configuré.
	otelShutdown, err := telemetry.Setup(context.Background(), "openstack-microservice")
	if err != nil {
		log.Printf("[otel] init: %v", err)
	}

	if err := models.CreateParams(); err != nil {
		log.Fatalf("Failed to initialize OpenStack clients: %v", err)
	}

	// creating context to stop cleanly
	ctx, cancel := context.WithCancel(context.Background())

	// Endpoint Prometheus /metrics (provisioning, erreurs OpenStack). No-op si port pris.
	metrics.Serve()

	//starting database
	config.Start_DB()
	go config.Sync_DB(ctx)

	//preparing workers
	var wg sync.WaitGroup
	worker.LaunchWorkers(5, &wg, ctx)

	// 	//starting goroutines
	go internal.Monitor(ctx)

	go ss.Start_grpc()

	// bloc instruction to shutdown cleanly
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown signal received")
	cancel()
	wg.Wait()

	if otelShutdown != nil {
		shCtx, shCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := otelShutdown(shCtx); err != nil {
			log.Printf("[otel] shutdown: %v", err)
		}
		shCancel()
	}

	log.Println("Program exited cleanly")
}
