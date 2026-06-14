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

	user, err := s.userSvc.GetProfile(ctx, payload.UserID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	if user.IsBanned {
		return nil, status.Error(codes.PermissionDenied, "user is banned")
	}
	if !user.EmailVerified {
		return nil, status.Error(codes.PermissionDenied, "email not verified")
	}

	return &pb.VerifyTokenResponse{
		UserId:    user.UserID.String(),
		Username:  user.Username,
		Role:      user.Role,
		TokenId:   payload.TokenID.String(),
		ExpiredAt: timestamppb.New(payload.ExpiredAt),
		IsPlus:    user.IsPlus,
	}, nil
}
