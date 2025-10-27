package grpc

import (
	"PoolManagerVM/backend/pb"
	"log"
	"net"

	"google.golang.org/grpc"
)

type GRPCServer struct {
	pb.UnimplementedControlCenterServer
}

func NewGRPCServer() *GRPCServer {
	return &GRPCServer{}
}

func StartGRPCServer(listaddr string) error {
	lis, err := net.Listen("tcp", listaddr)
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	pb.RegisterControlCenterServer(grpcServer, NewGRPCServer())
	log.Println("gRPC server listening on ", listaddr)
	return grpcServer.Serve(lis)
}
