package gapi

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "mem_pan/services/admin-service/pb/proto"
)

func (s *Server) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if !isModerator(payload.Role) {
		return nil, status.Error(codes.PermissionDenied, "moderator access required")
	}
	return nil, status.Error(codes.Unimplemented, "method ListUsers not implemented")
}

func (s *Server) BanUser(ctx context.Context, req *pb.BanUserRequest) (*pb.BanUserResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if !isModerator(payload.Role) {
		return nil, status.Error(codes.PermissionDenied, "moderator access required")
	}
	return nil, status.Error(codes.Unimplemented, "method BanUser not implemented")
}

func (s *Server) PromoteModerator(ctx context.Context, req *pb.PromoteModeratorRequest) (*pb.PromoteModeratorResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if payload.Role != "admin" {
		return nil, status.Error(codes.PermissionDenied, "admin access required")
	}
	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	promoted, err := s.authClient.SetUserRole(ctx, req.Email, "moderator")
	if err != nil {
		return nil, err
	}

	return &pb.PromoteModeratorResponse{
		UserId:   promoted.UserID.String(),
		Email:    promoted.Email,
		Username: promoted.Username,
	}, nil
}

func isModerator(role string) bool {
	return role == "admin" || role == "moderator"
}
