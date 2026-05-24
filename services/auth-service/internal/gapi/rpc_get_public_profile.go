package gapi

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/auth-service/pb"
)

func (s *Server) GetPublicProfile(ctx context.Context, req *pb.GetPublicProfileRequest) (*pb.GetPublicProfileResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}

	user, err := s.userSvc.GetProfile(ctx, userID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.GetPublicProfileResponse{User: dbUserToPublicPb(user)}, nil
}
