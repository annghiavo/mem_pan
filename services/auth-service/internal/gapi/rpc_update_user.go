package gapi

import (
	"context"

	"mem_pan/services/auth-service/internal/service"
	"mem_pan/services/auth-service/pb"
)

func (s *Server) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	params := service.UpdateProfileParams{}
	if req.FullName != nil {
		params.FullName = req.FullName
	}
	if req.AvatarUrl != nil {
		params.AvatarURL = req.AvatarUrl
	}

	user, err := s.userSvc.UpdateProfile(ctx, payload.UserID, params)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.UpdateUserResponse{User: dbUserToPb(user)}, nil
}
