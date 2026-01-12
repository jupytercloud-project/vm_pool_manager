package attribvm

import (
	"context"
	"control_center/frontcontrolpb"
	"control_center/models"
	"errors"

	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	frontcontrolpb.UnimplementedAttribVMServiceServer
	DB *gorm.DB
}

func New(db *gorm.DB) *Service {
	return &Service{
		DB: db,
	}
}

func (s *Service) ReturnPoolWithKey(
	req *frontcontrolpb.PoolWithKeyRequest,
	stream frontcontrolpb.AttribVMService_ReturnPoolWithKeyServer,
) error {

	pubKey := req.GetPubkey()
	if pubKey == "" {
		return status.Error(codes.InvalidArgument, "pubKey is empty")
	}

	_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKey))
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid public key: %v", err)
	}

	var pools []models.Serverpool

	if err := s.DB.
		Where("keypublist @> ARRAY[?]::text[]", pubKey).
		Find(&pools).Error; err != nil {
		return status.Errorf(codes.Internal, "database error: %v", err)
	}

	for _, pool := range pools {
		resp := &frontcontrolpb.PoolWithKeyResponse{
			PoolId: pool.ServerpoolID,
			UserId: pool.UserID,
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) AttribVMinPool(
	ctx context.Context,
	req *frontcontrolpb.AttribVMinPoolRequest,
) (*frontcontrolpb.AttribVMinPoolResponse, error) {

	if req.GetServerpoolId() == "" || req.GetUserId() == "" || req.GetPubkey() == "" {
		return &frontcontrolpb.AttribVMinPoolResponse{
			Success:     false,
			AddressedIp: "",
		}, status.Error(codes.InvalidArgument, "missing required fields")
	}
	var server models.Server
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("serverpool_id = ? AND user_id = ? AND locked = false",
				req.GetServerpoolId(), req.GetUserId()).
			Where("status = ?", "READY").
			Order("id").
			First(&server).Error; err != nil {

			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.ResourceExhausted, "no available server")
			}
			return err
		}
		if err := tx.Model(&server).
			Update("locked", true).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return &frontcontrolpb.AttribVMinPoolResponse{
			Success:     false,
			AddressedIp: "",
		}, err
	}
	if err := s.installSSHKey(&server, req.GetPubkey()); err != nil {
		_ = s.DB.Model(&server).Update("locked", false)
		return &frontcontrolpb.AttribVMinPoolResponse{
			Success:     false,
			AddressedIp: "",
		}, status.Errorf(codes.Internal, "ssh setup failed: %v", err)
	}
	return &frontcontrolpb.AttribVMinPoolResponse{
		Success:     true,
		AddressedIp: server.IP_Address,
	}, nil
}

func (s *Service) installSSHKey(server *models.Server, pubKey string) error {
	// TODO:
	// - ssh connect
	// - mkdir ~/.ssh
	// - append to authorized_keys
	return nil
}
