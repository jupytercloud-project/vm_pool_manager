package grpc

import (
	"context"
	"control_center/config"
	"control_center/models"
	"control_center/pb"
	"io"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm/clause"
)

func ConnectToMicroOpen(ctx context.Context) {
	conn, err := grpc.NewClient("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Erreur de connexion: %v", err)
	}
	defer conn.Close()

	client := pb.NewPoolManagerClient(conn)

	stream, err := client.GetStreamRessources(ctx, &emptypb.Empty{})
	if err != nil {
		log.Fatalf("Erreur stream: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Arrêt du streaming ConnectToMicroOpen")
			return
		default:
			resp, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Fatalf("Error listening stream: %v", err)
			}

			switch resp.Type {
			case pb.Type_SERVER:
				var serv models.Server
				serv.FromPb(resp)
				config.Database.Clauses(clause.OnConflict{UpdateAll: true}).Create(&serv)
			case pb.Type_SERVERPOOL:
				var pool models.Serverpool
				pool.FromPb(resp)
				config.Database.Clauses(clause.OnConflict{UpdateAll: true}).Create(&pool)
			case pb.Type_CONFIG:
				var conf models.ConfigPool
				conf.FromPb(resp)
				config.Database.Clauses(clause.OnConflict{UpdateAll: true}).Create(&conf)
			}
		}
	}
}

func PopulateDBImageMicroOpen() {
	conn, err := grpc.NewClient("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Erreur de connexion: %v", err)
	}
	defer conn.Close()

	client := pb.NewPoolManagerClient(conn)

	stream, err := client.GetAllImages(context.Background(), &emptypb.Empty{})
	if err != nil {
		log.Fatalf("Erreur stream: %v", err)
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Error listening stream: %v", err)
		}

		var img models.Image
		img.FromPb(resp, "Openstack")
		config.Database.Clauses(clause.OnConflict{UpdateAll: true}).Create(&img)
	}
}

func PopulateDBFlavorMicroOpen() {
	conn, err := grpc.NewClient("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Erreur de connexion: %v", err)
	}
	defer conn.Close()

	client := pb.NewPoolManagerClient(conn)

	stream, err := client.GetAllFlavors(context.Background(), &emptypb.Empty{})
	if err != nil {
		log.Fatalf("Erreur stream: %v", err)
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Error listening stream: %v", err)
		}

		var flavor models.Flavor
		flavor.FromPb(resp, "Openstack")
		config.Database.Clauses(clause.OnConflict{UpdateAll: true}).Create(&flavor)
	}
}

func PopulateDBNetworkMicroOpen() {
	conn, err := grpc.NewClient("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Erreur de connexion: %v", err)
	}
	defer conn.Close()

	client := pb.NewPoolManagerClient(conn)

	stream, err := client.GetAllNetworks(context.Background(), &emptypb.Empty{})
	if err != nil {
		log.Fatalf("Erreur stream: %v", err)
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Error listening stream: %v", err)
		}

		var network models.Network
		network.FromPb(resp, "Openstack")
		config.Database.Clauses(clause.OnConflict{UpdateAll: true}).Create(&network)
	}
}
