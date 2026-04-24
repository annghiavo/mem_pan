package gapi

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"mem_pan/services/auth-service/internal/service"
	"mem_pan/services/auth-service/pb"
)

func (s *Server) LoginUser(ctx context.Context, req *pb.LoginUserRequest) (*pb.LoginUserResponse, error) {
	if req.Email == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	userAgent := ""
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("user-agent"); len(vals) > 0 {
			userAgent = vals[0]
		}
	}

	clientIP := ""
	if p, ok := peer.FromContext(ctx); ok {
		clientIP = p.Addr.String()
	}

	resp, err := s.authSvc.Login(ctx, service.LoginParams{
		Email:     req.Email,
		Password:  req.Password,
		UserAgent: userAgent,
		ClientIP:  clientIP,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.LoginUserResponse{
		TokenId:               resp.Tokens.TokenID.String(),
		AccessToken:           resp.Tokens.AccessToken,
		AccessTokenExpiresAt:  timestamppb.New(resp.Tokens.AccessTokenExpiresAt),
		RefreshToken:          resp.Tokens.RefreshToken,
		RefreshTokenExpiresAt: timestamppb.New(resp.Tokens.RefreshTokenExpiresAt),
		User:                  dbUserToPb(resp.User),
	}, nil
}
