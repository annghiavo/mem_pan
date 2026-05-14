package gapi

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"mem_pan/services/search-service/internal/authclient"
)

// authorizeUser verifies the Bearer token in the gRPC metadata and returns the payload.
// Returns codes.Unauthenticated if the header is missing/malformed.
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

// optionalUser returns the caller's payload if present, or (nil, nil) if no auth header was sent.
func (s *Server) optionalUser(ctx context.Context) (*authclient.Payload, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, nil
	}
	values := md.Get("authorization")
	if len(values) == 0 {
		return nil, nil
	}
	fields := strings.Fields(values[0])
	if len(fields) != 2 || !strings.EqualFold(fields[0], "bearer") {
		return nil, nil
	}
	payload, err := s.authClient.VerifyToken(ctx, fields[1])
	if err != nil {
		return nil, nil
	}
	return payload, nil
}
