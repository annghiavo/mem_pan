package gapi

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/auth-service/pb"
)

func (s *Server) GetUserByID(ctx context.Context, req *pb.GetUserByIDRequest) (*pb.GetUserByIDResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}

	user, err := s.userSvc.GetProfile(ctx, userID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.GetUserByIDResponse{User: dbUserToPb(user)}, nil
}
