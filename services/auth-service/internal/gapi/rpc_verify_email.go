package gapi

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/auth-service/pb"
)

func (s *Server) VerifyEmail(ctx context.Context, req *pb.VerifyEmailRequest) (*pb.VerifyEmailResponse, error) {
	if req.Token == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}

	if err := s.authSvc.VerifyEmail(ctx, req.Token); err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.VerifyEmailResponse{Message: "email verified successfully"}, nil
}
