package grpc

import (
	"context"
	"net"
	"quantlo/internal/model"
	"quantlo/internal/proto"
	"quantlo/internal/service"

	"google.golang.org/grpc"
)

type Server struct {
	proto.UnimplementedLedgerServiceServer
	svc  service.LedgerService
	srv  *grpc.Server
	addr string
}

func NewServer(addr string, svc service.LedgerService) *Server {
	s := &Server{svc: svc, addr: addr, srv: grpc.NewServer()}
	proto.RegisterLedgerServiceServer(s.srv, s)
	return s
}

func (s *Server) Start(ctx context.Context) error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	return s.srv.Serve(lis)
}

func (s *Server) Stop(ctx context.Context) error {
	s.srv.GracefulStop()
	return nil
}

func (s *Server) Spend(ctx context.Context, req *proto.SpendRequest) (*proto.SpendResponse, error) {
	res, err := s.svc.Spend(ctx, model.SpendRequest{
		AccountID:      req.AccountId,
		ResourceType:   req.ResourceType,
		Amount:         req.Amount,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return &proto.SpendResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	return &proto.SpendResponse{
		Success:    true,
		NewBalance: res.NewBalance,
		Status:     res.Status,
	}, nil
}

func (s *Server) Recharge(ctx context.Context, req *proto.RechargeRequest) (*proto.RechargeResponse, error) {
	err := s.svc.Recharge(ctx, model.RechargeRequest{
		AccountID:    req.AccountId,
		ResourceType: req.ResourceType,
		Amount:       req.Amount,
	})
	if err != nil {
		return &proto.RechargeResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	return &proto.RechargeResponse{Success: true, Status: "SUCCESS"}, nil
}
