package gapi

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"mem_pan/services/auth-service/internal/token"
	"mem_pan/services/auth-service/pb"
)

func (s *Server) VerifyToken(ctx context.Context, req *pb.VerifyTokenRequest) (*pb.VerifyTokenResponse, error) {
	if req.AccessToken == "" {
		return nil, status.Error(codes.InvalidArgument, "access_token is required")
	}

	payload, err := s.tokenMaker.VerifyToken(req.AccessToken, token.TokenTypeAccess)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
	}

	return &pb.VerifyTokenResponse{
		UserId:    payload.UserID.String(),
		Username:  payload.Username,
		Role:      payload.Role,
		TokenId:   payload.TokenID.String(),
		ExpiredAt: timestamppb.New(payload.ExpiredAt),
	}, nil
}
