package gapi

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/auth-service/pb"
)

func (s *Server) UpdateUserPlusStatus(ctx context.Context, req *pb.UpdateUserPlusStatusRequest) (*pb.UpdateUserPlusStatusResponse, error) {
	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user id: %s", err)
	}

	err = s.userSvc.UpdateUserPlusStatus(ctx, userID, req.GetIsPlus())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update user plus status: %s", err)
	}

	return &pb.UpdateUserPlusStatusResponse{
		Success: true,
	}, nil
}
