package main

import (
	"context"
	"control_center/config"
	cc "control_center/grpc"
	"control_center/pb"
	"log"
	"net"
	"os"

	"github.com/joho/godotenv"

	"google.golang.org/grpc"
)

func main() {

	if err := godotenv.Load(); err != nil {
		panic("Error on loading .env")
	}

	//starting database
	config.Start_DB()
	go config.Sync_DB(context.Background())

	grpcServer := grpc.NewServer()
	controlCenter := &cc.ControlCenterServer{DB: config.Database}
	pb.RegisterPoolManagerServer(grpcServer, controlCenter)

	port := os.Getenv("CONTROL_CENTER_PORT")
	if port == "" {
		port = "50051"
	}
	list, err := net.Listen("tcp", ":"+port)
	if err != nil {
		panic("Impossible d'écouter sur le port" + port)
	}

	log.Println("Server lancé sur ", port)
	if err := grpcServer.Serve(list); err != nil {
		log.Fatalf("Erreur server gRPC : %v", err)
	}
}
