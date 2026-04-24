package gapi

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/auth-service/internal/service"
	"mem_pan/services/auth-service/pb"
)

func (s *Server) RegisterUser(ctx context.Context, req *pb.RegisterUserRequest) (*pb.RegisterUserResponse, error) {
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "username, email, and password are required")
	}
	if len(req.Password) < 8 {
		return nil, status.Error(codes.InvalidArgument, "password must be at least 8 characters")
	}

	var fullName *string
	if req.FullName != "" {
		fullName = &req.FullName
	}

	user, err := s.authSvc.Register(ctx, service.RegisterParams{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
		FullName: fullName,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.RegisterUserResponse{User: dbUserToPb(user)}, nil
}
