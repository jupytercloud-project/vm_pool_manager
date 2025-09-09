package main

import (
	"PoolManagerVM/backend/config"
	"PoolManagerVM/backend/internal"
	"PoolManagerVM/backend/internal/worker"
	"PoolManagerVM/backend/routes"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {

	//starting database
	config.Sync_DB()

	//configuring gin server
	r := gin.Default()
	routes.UserRoutes(r)
	routes.ServerpoolRoutes(r)

	//preparing workers
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	worker.LaunchWorkers(5, &wg, ctx)

	//starting goroutines
	go internal.Backwork(ctx)
	go internal.Monitor(ctx)

	//starting server gin in go routine
	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()
	log.Println("Server started on port 8080")

	// bloc instruction to shutdown cleanly
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown signal received")
	cancel()
	wg.Wait()
	ctxTimeout, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTimeout()
	if err := srv.Shutdown(ctxTimeout); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Program exited cleanly")
}
