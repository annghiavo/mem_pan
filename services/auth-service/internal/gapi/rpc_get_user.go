package gapi

import (
	"context"

	"mem_pan/services/auth-service/pb"
)

func (s *Server) GetUser(ctx context.Context, _ *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	user, err := s.userSvc.GetProfile(ctx, payload.UserID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.GetUserResponse{User: dbUserToPb(user)}, nil
}
