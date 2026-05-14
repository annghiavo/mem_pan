package gapi

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "mem_pan/services/notification-service/pb"
)

func (s *Server) RegisterDeviceToken(ctx context.Context, req *pb.RegisterDeviceTokenRequest) (*pb.RegisterDeviceTokenResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	if req.Token == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}

	if err := s.svc.RegisterDeviceToken(ctx, payload.UserID, req.Token, req.DeviceName); err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.RegisterDeviceTokenResponse{Message: "device token registered"}, nil
}

func (s *Server) UnregisterDeviceToken(ctx context.Context, req *pb.UnregisterDeviceTokenRequest) (*pb.UnregisterDeviceTokenResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	if req.Token == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}

	if err := s.svc.UnregisterDeviceToken(ctx, payload.UserID, req.Token); err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.UnregisterDeviceTokenResponse{}, nil
}
