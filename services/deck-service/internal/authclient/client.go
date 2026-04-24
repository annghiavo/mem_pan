package authclient

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	authpb "mem_pan/services/auth-service/pb"
)

// Payload contains the verified identity extracted from the access token.
type Payload struct {
	UserID   uuid.UUID
	Username string
	Role     string
}

// Client verifies access tokens by calling auth-service over gRPC.
type Client interface {
	VerifyToken(ctx context.Context, accessToken string) (*Payload, error)
	Close() error
}

type grpcClient struct {
	conn    *grpc.ClientConn
	authSvc authpb.AuthServiceClient
}

func NewGRPCClient(addr string) (Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
		return nil, status.Error(codes.Internal, "auth service unavailable")
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

func (c *grpcClient) Close() error {
	return c.conn.Close()
}
