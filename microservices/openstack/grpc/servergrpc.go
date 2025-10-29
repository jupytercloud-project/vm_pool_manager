package grpc

import (
	"PoolManagerVM/backend/config"
	"PoolManagerVM/backend/models"
	"PoolManagerVM/backend/notifier"
	"PoolManagerVM/backend/pb"
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

type ServerMicroOpenstack struct {
	pb.UnimplementedPoolManagerServer
	DB *gorm.DB
}

func Start_grpc() {
	log.Println("gRPC started")
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("Error listening: %v", err)
	}

	grpcServer := grpc.NewServer()

	pb.RegisterPoolManagerServer(grpcServer, &ServerMicroOpenstack{DB: config.Database})
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Erreur serveur gRPC: %v", err)
	}
}

func (s *ServerMicroOpenstack) SendRessources(ctx context.Context, req *pb.RessourceRequest) (*emptypb.Empty, error) {
	log.Printf("[SendRessources] User=%s Data=%v", req.GetUser(), req.GetData())
	//create ressources here
	return &emptypb.Empty{}, nil
}

func sendAllServer(s *ServerMicroOpenstack, stream pb.PoolManager_GetStreamRessourcesServer) error {
	rows, err := s.DB.Model(&models.Server{}).Rows()
	if err != nil {
		log.Println("Error retrieving servers")
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var serv models.Server
		if err := s.DB.ScanRows(rows, &serv); err != nil {
			log.Println("Error rows server")
			return err
		}
		ret := &pb.StreamRessourceResponse{
			User:   serv.UserID,
			Status: pb.Status_STATUS_UNKNOWN,
			Type:   pb.Type_SERVER,
			Data:   serv.ToMap(),
		}

		if err := stream.Send(ret); err != nil {
			log.Println("error sending server")
			return err
		}
	}
	return nil
}

func sendAllPool(s *ServerMicroOpenstack, stream pb.PoolManager_GetStreamRessourcesServer) error {
	rows, err := s.DB.Model(&models.Serverpool{}).Rows()
	if err != nil {
		log.Println("Error retrieving servers")
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var pool models.Serverpool
		if err := s.DB.ScanRows(rows, &pool); err != nil {
			log.Println("Error rows server")
			return err
		}
		ret := &pb.StreamRessourceResponse{
			User:   pool.UserID,
			Status: pb.Status_STATUS_UNKNOWN,
			Type:   pb.Type_SERVER,
			Data:   pool.ToMap(),
		}

		if err := stream.Send(ret); err != nil {
			log.Println("error sending server")
			return err
		}
	}
	return nil
}

func sendAllConfig(s *ServerMicroOpenstack, stream pb.PoolManager_GetStreamRessourcesServer) error {
	rows, err := s.DB.Model(&models.ConfigPool{}).Rows()
	if err != nil {
		log.Println("Error retrieving servers")
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var pool models.ConfigPool
		if err := s.DB.ScanRows(rows, &pool); err != nil {
			log.Println("Error rows server")
			return err
		}
		ret := &pb.StreamRessourceResponse{
			User:   pool.UserID,
			Status: pb.Status_STATUS_UNKNOWN,
			Type:   pb.Type_SERVER,
			Data:   pool.ToMap(),
		}

		if err := stream.Send(ret); err != nil {
			log.Println("error sending server")
			return err
		}
	}
	return nil
}

func (s *ServerMicroOpenstack) GetStreamRessources(req *emptypb.Empty, stream pb.PoolManager_GetStreamRessourcesServer) error {
	log.Println("[GetStreamRessources] Stream global started")

	// Send all ressources at first connection to ensure synchronize
	if err := sendAllServer(s, stream); err != nil {
		log.Printf("Error Server: %v", err)
		return err
	}
	if err := sendAllPool(s, stream); err != nil {
		log.Printf("Error Serverpool: %v", err)
		return err
	}
	if err := sendAllConfig(s, stream); err != nil {
		log.Printf("Error Serverpool: %v", err)
		return err
	}

	// Infinite loop to send all modification on the database
	for {
		select {
		case evt := <-notifier.GlobalChan:
			switch evt.Type {
			case pb.Type_SERVER:
				server, ok := evt.Ressource.(models.Server)
				if !ok {
					continue
				}
				var status pb.Status
				switch evt.Action {
				case "created":
					status = pb.Status_CREATE
				case "updated":
					status = pb.Status_UPDATE
				case "deleted":
					status = pb.Status_DELETE
				default:
					status = pb.Status_STATUS_UNKNOWN
				}
				err := stream.Send(&pb.StreamRessourceResponse{
					User:   server.UserID,
					Type:   pb.Type_SERVER,
					Status: status,
					Data:   server.ToMap(),
				})
				if err != nil {
					log.Printf("Stream closed for client: %v", err)
					return err
				}

			case pb.Type_SERVERPOOL:
				pool, ok := evt.Ressource.(models.Serverpool)
				if !ok {
					continue
				}
				var status pb.Status
				switch evt.Action {
				case "created":
					status = pb.Status_CREATE
				case "updated":
					status = pb.Status_UPDATE
				case "deleted":
					status = pb.Status_DELETE
				default:
					status = pb.Status_STATUS_UNKNOWN
				}
				err := stream.Send(&pb.StreamRessourceResponse{
					User:   pool.UserID,
					Type:   pb.Type_SERVER,
					Status: status,
					Data:   pool.ToMap(),
				})
				if err != nil {
					log.Printf("Stream closed for client: %v", err)
					return err
				}

			case pb.Type_CONFIG:
				config, ok := evt.Ressource.(models.ConfigPool)
				if !ok {
					continue
				}
				var status pb.Status
				switch evt.Action {
				case "created":
					status = pb.Status_CREATE
				case "updated":
					status = pb.Status_UPDATE
				case "deleted":
					status = pb.Status_DELETE
				default:
					status = pb.Status_STATUS_UNKNOWN
				}
				err := stream.Send(&pb.StreamRessourceResponse{
					User:   config.UserID,
					Type:   pb.Type_SERVER,
					Status: status,
					Data:   config.ToMap(),
				})
				if err != nil {
					log.Printf("Stream closed for client: %v", err)
					return err
				}
			}

		case <-stream.Context().Done():
			log.Println("[GetStreamRessources] Client disconnected, end of stream")
			return nil
		}
	}
}

func (s *ServerMicroOpenstack) GetStreamRessourcesUser(req *pb.UserRequest, stream grpc.ServerStreamingServer[pb.StreamRessourceResponse]) error {
	log.Println("[GetStreamRessourcesUser] Stream User started")
	// stream user-specific ressources
	return nil
}

func (s *ServerMicroOpenstack) GetAllImages(req *emptypb.Empty, stream grpc.ServerStreamingServer[pb.Image]) error {
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
		ret := &pb.Image{
			Id:               img.ID,
			Name:             img.Name,
			Status:           img.Status,
			Tags:             img.Tags,
			ContainerFormat:  img.ContainerFormat,
			DiskFormat:       img.DiskFormat,
			MinDiskGigabytes: int32(img.MinDiskGigabytes),
			MinRamMegabytes:  int32(img.MinRAMMegabytes),
			Owner:            img.Owner,
			Protected:        img.Protected,
			Visibility:       img.Visibility,
			Hidden:           img.Hidden,
			Checksum:         img.Checksum,
			SizeBytes:        img.SizeBytes,
			CreatedAt:        timestamppb.New(img.CreatedAt),
			UpdatedAt:        timestamppb.New(img.UpdatedAt),
			File:             img.File,
			Schema:           img.Schema,
			VirtualSize:      img.VirtualSize,
			ImportMethods:    img.ImportMethods,
			StoreIds:         img.StoreIDs,
		}

		if err := stream.Send(ret); err != nil {
			log.Println("error sending server")
			return err
		}
	}
	return nil
}

func (s *ServerMicroOpenstack) GetAllFlavors(req *emptypb.Empty, stream grpc.ServerStreamingServer[pb.Flavor]) error {
	rows, err := s.DB.Model(&models.Image{}).Rows()
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
		ret := &pb.Flavor{
			Id:          f.ID,
			Name:        f.Name,
			Disk:        int32(f.Disk),
			Ram:         int32(f.RAM),
			Vcpus:       int32(f.VCPUs),
			RxtxFactor:  f.RxTxFactor,
			Swap:        int32(f.Swap),
			Ephemeral:   int32(f.Ephemeral),
			IsPublic:    f.IsPublic,
			Description: f.Description,
			ExtraSpecs:  f.ExtraSpecs,
		}

		if err := stream.Send(ret); err != nil {
			log.Println("error sending server")
			return err
		}
	}
	return nil
}

func (s *ServerMicroOpenstack) GetAllNetwork(req *emptypb.Empty, stream grpc.ServerStreamingServer[pb.Network]) error {
	rows, err := s.DB.Model(&models.Image{}).Rows()
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
		ret := &pb.Network{
			Id:                    n.ID,
			Name:                  n.Name,
			Description:           n.Description,
			AdminStateUp:          n.AdminStateUp,
			Status:                n.Status,
			TenantId:              n.TenantID,
			ProjectId:             n.ProjectID,
			Shared:                n.Shared,
			RevisionNumber:        int32(n.RevisionNumber),
			Subnets:               n.Subnets,
			AvailabilityZoneHints: n.AvailabilityZoneHints,
			Tags:                  n.Tags,
		}

		if err := stream.Send(ret); err != nil {
			log.Println("error sending server")
			return err
		}
	}
	return nil
}
