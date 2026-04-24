package gapi

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/auth-service/pb"
)

func (s *Server) ForgotPassword(ctx context.Context, req *pb.ForgotPasswordRequest) (*pb.ForgotPasswordResponse, error) {
	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	// errors are swallowed to avoid email enumeration
	_ = s.authSvc.ForgotPassword(ctx, req.Email)

	return &pb.ForgotPasswordResponse{
		Message: "if the email exists, a reset link has been sent",
	}, nil
}
