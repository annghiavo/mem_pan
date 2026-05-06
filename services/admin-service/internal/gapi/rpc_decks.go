package gapi

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "mem_pan/services/admin-service/pb/proto"
)

func (s *Server) UpdateDeckStatus(ctx context.Context, req *pb.UpdateDeckStatusRequest) (*pb.UpdateDeckStatusResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if !isModerator(payload.Role) {
		return nil, status.Error(codes.PermissionDenied, "moderator access required")
	}
	return nil, status.Error(codes.Unimplemented, "method UpdateDeckStatus not implemented")
}
