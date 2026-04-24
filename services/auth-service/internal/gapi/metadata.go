package gapi

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"mem_pan/services/auth-service/internal/token"
)

func (s *Server) authorizeUser(ctx context.Context) (*token.Payload, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization header")
	}

	fields := strings.Fields(values[0])
	if len(fields) != 2 || !strings.EqualFold(fields[0], "bearer") {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization header format")
	}

	payload, err := s.tokenMaker.VerifyToken(fields[1], token.TokenTypeAccess)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid or expired access token")
	}

	return payload, nil
}
