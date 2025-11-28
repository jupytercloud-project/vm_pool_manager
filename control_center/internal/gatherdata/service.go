package gatherdata

import (
	"control_center/frontcontrolpb"
	"control_center/models"
	"control_center/pb"
	"log"

	"gorm.io/gorm"
)

type Service struct {
	frontcontrolpb.UnimplementedGatherDataServiceServer
	DB *gorm.DB
	pm pb.PoolManagerClient
}

func New(pm pb.PoolManagerClient, db *gorm.DB) *Service {
	return &Service{
		pm: pm,
		DB: db,
	}
}

func (s *Service) GetAllImages(req *frontcontrolpb.UserRequest, stream frontcontrolpb.GatherDataService_GetAllImagesServer) error {
	log.Println("Recieving message")
	rows, err := s.DB.Model(&models.Image{}).Rows()
	if err != nil {
		log.Println("Error retrieving images: ", err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var img models.Image
		if err := s.DB.ScanRows(rows, &img); err != nil {
			log.Println("Error scanning image row: ", err)
			return err
		}
		if err := stream.Send(img.ToFrontControlPb()); err != nil {
			log.Println("Error sending image: ", err)
			return err
		}
	}
	return nil
}

func (s *Service) GetAllFlavors(req *frontcontrolpb.UserRequest, stream frontcontrolpb.GatherDataService_GetAllFlavorsServer) error {
	log.Println("Recieving message")
	rows, err := s.DB.Model(&models.Flavor{}).Rows()
	if err != nil {
		log.Println("Error retrieving flavors: ", err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var f models.Flavor
		if err := s.DB.ScanRows(rows, &f); err != nil {
			log.Println("Error scanning flavor row: ", err)
			return err
		}
		if err := stream.Send(f.ToFrontControlPb()); err != nil {
			log.Println("error sending flavor: ", err)
			return err
		}
	}
	return nil
}

func (s *Service) GetAllNetworks(req *frontcontrolpb.UserRequest, stream frontcontrolpb.GatherDataService_GetAllNetworksServer) error {
	log.Println("Recieving message")
	rows, err := s.DB.Model(&models.Network{}).Rows()
	if err != nil {
		log.Println("Error retrieving networks: ", err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var n models.Network
		if err := s.DB.ScanRows(rows, &n); err != nil {
			log.Println("Error scanning network row: ", err)
			return err
		}
		if err := stream.Send(n.ToFrontControlPb()); err != nil {
			log.Println("Error sending network: ", err)
			return err
		}
	}
	return nil
}

func (s *Service) GetAllServers(req *frontcontrolpb.UserRequest, stream frontcontrolpb.GatherDataService_GetAllServersServer) error {
	log.Println("Recieving message")
	rows, err := s.DB.Model(&models.Server{}).Rows()
	if err != nil {
		log.Println("Error retrieving servers: ", err)
		return err
	}
	defer rows.Close()
	sent := false
	for rows.Next() {
		var n models.Server
		if err := s.DB.ScanRows(rows, &n); err != nil {
			log.Println("Error scanning server row: ", err)
			return err
		}
		if n.UserID == req.GetUser() {
			if err := stream.Send(n.ToFrontControlPb()); err != nil {
				log.Println("Error sending server: ", err)
			}
			sent = true
		}
	}
	if !sent {
		empty := frontcontrolpb.Server{}
		return stream.Send(&empty)
	}
	return nil
}

func (s *Service) GetAllServerPools(req *frontcontrolpb.UserRequest, stream frontcontrolpb.GatherDataService_GetAllServerPoolsServer) error {
	log.Println("Recieving message")
	rows, err := s.DB.Model(&models.Serverpool{}).Rows()
	if err != nil {
		log.Println("Error retrieving servers: ", err)
		return err
	}
	defer rows.Close()
	sent := false
	for rows.Next() {
		var n models.Serverpool
		if err := s.DB.ScanRows(rows, &n); err != nil {
			log.Println("Error scanning server row: ", err)
			return err
		}
		if n.UserID == req.GetUser() {
			if err := stream.Send(n.ToFrontControlPb()); err != nil {
				log.Println("Error sending server: ", err)
			}
			sent = true
		}
	}

	if !sent {
		empty := &frontcontrolpb.ServerPool{}
		return stream.Send(empty)
	}
	return nil
}

func (s *Service) GetAllConfigs(req *frontcontrolpb.UserRequest, stream frontcontrolpb.GatherDataService_GetAllConfigsServer) error {
	log.Println("Recieving message")
	rows, err := s.DB.Model(&models.ConfigPool{}).Rows()
	if err != nil {
		log.Println("Error retrieving servers: ", err)
		return err
	}
	defer rows.Close()
	sent := false
	for rows.Next() {
		var n models.ConfigPool
		if err := s.DB.ScanRows(rows, &n); err != nil {
			log.Println("Error scanning server row: ", err)
			return err
		}
		if n.UserID == req.GetUser() {
			if err := stream.Send(n.ToFrontControlPb()); err != nil {
				log.Println("Error sending server: ", err)
			}
			sent = true
		}
	}

	if !sent {
		empty := &frontcontrolpb.Config{}
		return stream.Send(empty)
	}
	return nil
}
