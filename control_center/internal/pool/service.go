package pool

import (
	"context"
	"control_center/frontcontrolpb"
	"control_center/models"
	"control_center/pb"
	"strconv"

	"gorm.io/gorm"
)

type Service struct {
	frontcontrolpb.UnimplementedPoolServiceServer
	DB *gorm.DB
	pm pb.PoolManagerClient
}

func New() *Service {
	return &Service{}
}

func (s *Service) CreatePool(ctx context.Context, req *frontcontrolpb.CreatePoolRequest) (*frontcontrolpb.CreatePoolResponse, error) {
	minVM, _ := strconv.Atoi(req.GetMinVm())
	maxVM, _ := strconv.Atoi(req.GetMaxVm())

	pool := models.Serverpool{
		UserID:       req.GetUser(),
		ServerpoolID: req.GetName(),
		ImageRef:     req.GetImage(),
		FlavorRef:    req.GetFlavor(),
		MinVM:        minVM,
		MaxVM:        maxVM,
		Networks:     models.JSONStringSlice{req.GetNetwork()},
		ConfigID:     req.GetConfig(),
	}

	rep, err := s.pm.SendRessources(context.Background(), &pb.RessourceRequest{
		User:   req.GetUser(),
		Data:   pool.ToMap(),
		Status: pb.Status_CREATE,
		Type:   pb.Type_SERVERPOOL,
	})
	if rep.GetSuccess() == false || err != nil {
		return &frontcontrolpb.CreatePoolResponse{Success: false}, err
	}
	return &frontcontrolpb.CreatePoolResponse{Success: true}, nil
}
