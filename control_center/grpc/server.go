package grpc

import (
	"context"
	"control_center/config"
	"control_center/models"
	"control_center/pb"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
)

type ControlCenterServer struct {
	pb.UnimplementedPoolManagerServer
	DB *gorm.DB
}

func Start_grpc(ctx context.Context) {
	log.Println("Démarrage du serveur gRPC...")

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Erreur lors de l'écoute du port : %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPoolManagerServer(grpcServer, &ControlCenterServer{DB: config.Database})

	// Lance le serveur dans une goroutine
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Erreur serveur gRPC: %v", err)
		}
	}()

	log.Println("Serveur gRPC lancé sur le port 50051")

	// Attend que le contexte soit annulé
	<-ctx.Done()

	log.Println("Arrêt du serveur gRPC demandé...")

	// Arrêt propre du serveur
	grpcServer.GracefulStop()
	log.Println("Serveur gRPC arrêté proprement ✅")
}

func (s *ControlCenterServer) GetAllImages(req *emptypb.Empty, stream grpc.ServerStreamingServer[pb.Image]) error {
	rows, err := s.DB.Model(&models.Image{}).Rows()
	if err != nil {
		log.Println("Error retrieving servers")
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var img models.Image
		if err := s.DB.ScanRows(rows, &img); err != nil {
			log.Println("Error rows server")
			return err
		}
		if err := stream.Send(img.ToPb()); err != nil {
			log.Println("error sending server")
			return err
		}
	}
	return nil
}

func (s *ControlCenterServer) GetAllFlavors(req *emptypb.Empty, stream grpc.ServerStreamingServer[pb.Flavor]) error {
	rows, err := s.DB.Model(&models.Flavor{}).Rows()
	if err != nil {
		log.Println("Error retrieving servers")
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var f models.Flavor
		if err := s.DB.ScanRows(rows, &f); err != nil {
			log.Println("Error rows server")
			return err
		}
		if err := stream.Send(f.ToPb()); err != nil {
			log.Println("error sending server")
			return err
		}
	}
	return nil
}

func (s *ControlCenterServer) GetAllNetworks(req *emptypb.Empty, stream grpc.ServerStreamingServer[pb.Network]) error {
	rows, err := s.DB.Model(&models.Network{}).Rows()
	if err != nil {
		log.Println("Error retrieving servers")
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var n models.Network
		if err := s.DB.ScanRows(rows, &n); err != nil {
			log.Println("Error rows server")
			return err
		}
		if err := stream.Send(n.ToPb()); err != nil {
			log.Println("error sending server")
			return err
		}
	}
	return nil
}
