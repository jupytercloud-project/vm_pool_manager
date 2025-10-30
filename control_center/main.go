package main

import (
	"context"
	"control_center/config"
	cc "control_center/grpc"
	"control_center/pb"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/joho/godotenv"

	"google.golang.org/grpc"
)

func main() {
	if err := godotenv.Load(); err != nil {
		panic("Error on loading .env")
	}

	// Starting database
	config.Start_DB()

	// Context pour contrôler l'arrêt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Sync DB en goroutine
	go config.Sync_DB(ctx)

	grpcServer := grpc.NewServer()
	controlCenter := &cc.ControlCenterServer{DB: config.Database}
	pb.RegisterPoolManagerServer(grpcServer, controlCenter)

	port := os.Getenv("CONTROL_CENTER_PORT")
	if port == "" {
		port = "50051"
	}
	list, err := net.Listen("tcp", ":"+port)
	if err != nil {
		panic("Impossible d'écouter sur le port " + port)
	}

	// Remplissage DB initial
	cc.PopulateDBImageMicroOpen()
	cc.PopulateDBFlavorMicroOpen()
	cc.PopulateDBNetworkMicroOpen()

	// Goroutine de streaming
	go cc.ConnectToMicroOpen(ctx)

	// Capture SIGINT
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		log.Println("SIGINT reçu, arrêt du streaming et du serveur…")
		cancel()                  // stop ConnectToMicroOpen
		grpcServer.GracefulStop() // stop serveur gRPC proprement
	}()

	log.Println("Server lancé sur ", port)
	if err := grpcServer.Serve(list); err != nil {
		log.Fatalf("Erreur server gRPC : %v", err)
	}
}
