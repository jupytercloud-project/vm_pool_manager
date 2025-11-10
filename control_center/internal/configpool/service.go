package configpool

import (
	"context"
	"control_center/frontcontrolpb"
	"control_center/models"
	"control_center/pb"

	"gorm.io/gorm"
)

type Service struct {
	frontcontrolpb.UnimplementedConfigServiceServer
	pm pb.PoolManagerClient
	DB *gorm.DB
}

func New() *Service {
	return &Service{}
}

func (s *Service) GetConfig(ctx context.Context, req *frontcontrolpb.GetConfigRequest) (*frontcontrolpb.GetConfigResponse, error) {
	var conf models.ConfigPool
	if err := s.DB.Where(" userid = ? && name = ? ", req.GetUser(), req.GetKey()).First(&conf).Error; err != nil {
		return nil, err
	}
	return &frontcontrolpb.GetConfigResponse{
		Value: conf.Data,
		Key:   conf.Name,
	}, nil
}

func (s *Service) CreateConfig(ctx context.Context, req *frontcontrolpb.CreateConfigRequest) (*frontcontrolpb.CreateConfigResponse, error) {
	conf := models.ConfigPool{
		UserID: req.GetUser(),
		Name:   req.GetKey(),
		Data:   req.GetValue(),
	}

	ress, err := s.pm.SendRessources(context.Background(), &pb.RessourceRequest{
		User:   req.GetUser(),
		Data:   conf.ToMap(),
		Status: pb.Status_CREATE,
		Type:   pb.Type_CONFIG,
	})
	if ress.GetSuccess() == false || err != nil {
		return &frontcontrolpb.CreateConfigResponse{
			Success: false,
		}, err
	}
	return &frontcontrolpb.CreateConfigResponse{
		Success: true,
	}, nil
}

func (s *Service) UpdateConfig(ctx context.Context, req *frontcontrolpb.UpdateConfigRequest) (*frontcontrolpb.UpdateConfigResponse, error) {
	var conf models.ConfigPool
	if err := s.DB.Where(" userid = ? && name = ? ", req.GetUser(), req.GetKey()).First(&conf).Error; err != nil {
		return &frontcontrolpb.UpdateConfigResponse{
			Success: false,
		}, err
	}
	conf.Data = req.GetValue()
	ress, err := s.pm.SendRessources(context.Background(), &pb.RessourceRequest{
		User:   req.GetUser(),
		Data:   conf.ToMap(),
		Status: pb.Status_UPDATE,
		Type:   pb.Type_CONFIG,
	})
	if ress.GetSuccess() == false || err != nil {
		return &frontcontrolpb.UpdateConfigResponse{
			Success: false,
		}, err
	}
	return &frontcontrolpb.UpdateConfigResponse{
		Success: true,
	}, nil
}

func (s *Service) DeleteConfig(ctx context.Context, req *frontcontrolpb.DeleteConfigRequest) (*frontcontrolpb.DeleteConfigResponse, error) {
	var conf models.ConfigPool
	if err := s.DB.Where(" userid = ? && name = ? ", req.GetUser(), req.GetKey()).First(&conf).Error; err != nil {
		return &frontcontrolpb.DeleteConfigResponse{
			Success: false,
		}, err
	}
	ress, err := s.pm.SendRessources(context.Background(), &pb.RessourceRequest{
		User:   req.GetUser(),
		Data:   conf.ToMap(),
		Status: pb.Status_DELETE,
		Type:   pb.Type_CONFIG,
	})
	if ress.GetSuccess() == false || err != nil {
		return &frontcontrolpb.DeleteConfigResponse{
			Success: false,
		}, err
	}
	return &frontcontrolpb.DeleteConfigResponse{
		Success: true,
	}, nil
}

func (s *Service) GetAllConfigs(req *frontcontrolpb.GetConfigRequest, stream frontcontrolpb.ConfigService_GetAllConfigsServer) error {
	rows, err := s.DB.Model(&models.ConfigPool{}).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cp models.ConfigPool
		if err := s.DB.ScanRows(rows, &cp); err != nil {
			return err
		}
		if err := stream.Send(&frontcontrolpb.GetConfigResponse{
			Key:   cp.Name,
			Value: cp.Data,
		}); err != nil {
			return err
		}
	}
	return nil
}
