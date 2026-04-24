package gapi

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/auth-service/pb"
)

func (s *Server) ResendVerification(ctx context.Context, req *pb.ResendVerificationRequest) (*pb.ResendVerificationResponse, error) {
	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	// errors are swallowed to avoid email enumeration
	_ = s.authSvc.ResendEmailVerification(ctx, req.Email)

	return &pb.ResendVerificationResponse{
		Message: "if the email exists, a verification link has been sent",
	}, nil
}
