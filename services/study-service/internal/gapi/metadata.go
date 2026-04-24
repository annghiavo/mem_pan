package gapi

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"mem_pan/services/study-service/internal/authclient"
)

func (s *Server) authorizeUser(ctx context.Context) (*authclient.Payload, error) {
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

	return s.authClient.VerifyToken(ctx, fields[1])
}

func extractBearerToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}
	values := md.Get("authorization")
	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization header")
	}
	fields := strings.Fields(values[0])
	if len(fields) != 2 {
		return "", status.Error(codes.Unauthenticated, "invalid authorization header format")
	}
	return fields[1], nil
}
