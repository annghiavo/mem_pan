package gapi

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/auth-service/internal/db"
	"mem_pan/services/auth-service/internal/service"
	"mem_pan/services/auth-service/pb"
)

func (s *Server) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	limit := req.GetPageSize()
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := req.GetOffset()
	if offset < 0 {
		offset = 0
	}

	result, err := s.userSvc.ListUsers(ctx, service.ListUsersParams{
		Limit:        limit,
		Offset:       offset,
		FilterBanned: req.GetFilterBanned(),
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list users")
	}

	users := make([]*pb.AdminUser, 0, len(result.Users))
	for _, u := range result.Users {
		users = append(users, dbUserToAdminPb(u))
	}
	return &pb.ListUsersResponse{Users: users, Total: result.Total}, nil
}

func dbUserToAdminPb(u db.User) *pb.AdminUser {
	return &pb.AdminUser{
		UserId:    u.UserID.String(),
		Username:  u.Username,
		Email:     u.Email,
		Role:      u.Role,
		IsBanned:  u.IsBanned,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
