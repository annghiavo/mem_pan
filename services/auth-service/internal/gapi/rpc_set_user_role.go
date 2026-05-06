package gapi

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/auth-service/internal/domain"
	"mem_pan/services/auth-service/pb"
)

var allowedRoles = map[string]bool{
	"user":      true,
	"moderator": true,
	"admin":     true,
}

func (s *Server) SetUserRole(ctx context.Context, req *pb.SetUserRoleRequest) (*pb.SetUserRoleResponse, error) {
	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}
	if !allowedRoles[req.Role] {
		return nil, status.Error(codes.InvalidArgument, "role must be one of: user, moderator, admin")
	}

	user, err := s.userSvc.SetUserRole(ctx, req.Email, req.Role)
	if err != nil {
		if err == domain.ErrUserNotFound {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}

	return &pb.SetUserRoleResponse{
		UserId:   user.UserID.String(),
		Email:    user.Email,
		Username: user.Username,
		Role:     user.Role,
	}, nil
}
