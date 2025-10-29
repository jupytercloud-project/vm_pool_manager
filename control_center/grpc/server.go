package grpc

import (
	"control_center/pb"

	"gorm.io/gorm"
)

type ControlCenterServer struct {
	pb.UnimplementedPoolManagerServer
	DB *gorm.DB
}
