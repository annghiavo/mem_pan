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

// Payload contains the verified identity extracted from the access token.
type Payload struct {
	UserID   uuid.UUID
	Username string
	Role     string
}

// UserProfile holds public profile fields fetched from auth-service.
type UserProfile struct {
	Username  string
	AvatarURL string
}

// Client verifies access tokens by calling auth-service over gRPC.
type Client interface {
	VerifyToken(ctx context.Context, accessToken string) (*Payload, error)
	GetUserByID(ctx context.Context, userID uuid.UUID) (*UserProfile, error)
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
	return &grpcClient{
		conn:    conn,
		authSvc: authpb.NewAuthServiceClient(conn),
	}, nil
}

func (c *grpcClient) VerifyToken(ctx context.Context, accessToken string) (*Payload, error) {
	resp, err := c.authSvc.VerifyToken(ctx, &authpb.VerifyTokenRequest{AccessToken: accessToken})
	if err != nil {
		// Translate auth-service Unauthenticated → Unauthenticated for the caller.
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unauthenticated {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired access token")
		}
		// TEMP DEBUG: surface underlying error so Cloud Run logs show the real cause.
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
	}, nil
}

func (c *grpcClient) GetUserByID(ctx context.Context, userID uuid.UUID) (*UserProfile, error) {
	resp, err := c.authSvc.GetUserByID(ctx, &authpb.GetUserByIDRequest{UserId: userID.String()})
	if err != nil {
		return nil, status.Error(codes.Internal, "auth service unavailable")
	}
	if resp.User == nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	return &UserProfile{
		Username:  resp.User.Username,
		AvatarURL: resp.User.AvatarUrl,
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
