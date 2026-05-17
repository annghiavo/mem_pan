package gapi

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/auth-service/pb"
)

func (s *Server) BanUser(ctx context.Context, req *pb.BanUserRequest) (*pb.BanUserResponse, error) {
	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}

	var user = struct {
		ID       uuid.UUID
		Username string
		Email    string
		Banned   bool
	}{}

	if req.GetBan() {
		u, err := s.userSvc.BanUser(ctx, userID, req.GetReason())
		if err != nil {
			return nil, toGRPCError(err)
		}
		user.ID, user.Username, user.Email, user.Banned = u.UserID, u.Username, u.Email, u.IsBanned
	} else {
		u, err := s.userSvc.UnbanUser(ctx, userID)
		if err != nil {
			return nil, toGRPCError(err)
		}
		user.ID, user.Username, user.Email, user.Banned = u.UserID, u.Username, u.Email, u.IsBanned
	}

	return &pb.BanUserResponse{
		UserId:   user.ID.String(),
		Username: user.Username,
		Email:    user.Email,
		IsBanned: user.Banned,
	}, nil
}
