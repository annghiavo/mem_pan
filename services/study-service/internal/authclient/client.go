package authclient

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	authpb "mem_pan/services/auth-service/pb"

	"crypto/tls"
	"google.golang.org/grpc/credentials"
	"strings"
)

type Payload struct {
	UserID   uuid.UUID
	Username string
	Role     string
	IsPlus   bool
}

type Client interface {
	VerifyToken(ctx context.Context, accessToken string) (*Payload, error)
	Close() error
}

type grpcClient struct {
	conn    *grpc.ClientConn
	authSvc authpb.AuthServiceClient
}

func NewGRPCClient(addr string) (Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(pickCreds(addr)))
	if err != nil {
		return nil, err
	}
	return &grpcClient{conn: conn, authSvc: authpb.NewAuthServiceClient(conn)}, nil
}

func (c *grpcClient) VerifyToken(ctx context.Context, accessToken string) (*Payload, error) {
	resp, err := c.authSvc.VerifyToken(ctx, &authpb.VerifyTokenRequest{AccessToken: accessToken})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unauthenticated:
				return nil, status.Error(codes.Unauthenticated, "invalid or expired access token")
			case codes.PermissionDenied:
				return nil, err
			}
		}
		return nil, status.Errorf(codes.Internal, "auth service unavailable: %v", err)
	}
	userID, err := uuid.Parse(resp.UserId)
	if err != nil {
		return nil, status.Error(codes.Internal, "invalid user_id in token response")
	}

	return &Payload{
		UserID:   userID,
		Username: resp.Username,
		Role:     resp.Role,
		IsPlus:   resp.IsPlus,
	}, nil
}

func (c *grpcClient) Close() error {
	return c.conn.Close()
}

// pickCreds returns TLS credentials when the target appears to be a
// Cloud Run / managed endpoint (port :443 or *.run.app), otherwise an
// insecure transport for local docker-compose or in-cluster gRPC.
func pickCreds(addr string) credentials.TransportCredentials {
	if strings.HasSuffix(addr, ":443") || strings.Contains(addr, ".run.app") {
		return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	}
	return insecure.NewCredentials()
}
