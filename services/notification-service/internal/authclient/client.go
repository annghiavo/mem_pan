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

type Payload struct {
	UserID   uuid.UUID
	Username string
	Role     string
}

type User struct {
	UserID   uuid.UUID
	Username string
	Email    string
}

type Client interface {
	VerifyToken(ctx context.Context, accessToken string) (*Payload, error)
	GetUserByID(ctx context.Context, userID uuid.UUID) (*User, error)
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
	return &grpcClient{conn: conn, authSvc: authpb.NewAuthServiceClient(conn)}, nil
}

func (c *grpcClient) VerifyToken(ctx context.Context, accessToken string) (*Payload, error) {
	resp, err := c.authSvc.VerifyToken(ctx, &authpb.VerifyTokenRequest{AccessToken: accessToken})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unauthenticated {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired access token")
		}
		return nil, status.Error(codes.Internal, "auth service unavailable")
	}

	userID, err := uuid.Parse(resp.UserId)
	if err != nil {
		return nil, status.Error(codes.Internal, "invalid user_id in token response")
	}

	return &Payload{UserID: userID, Username: resp.Username, Role: resp.Role}, nil
}

func (c *grpcClient) GetUserByID(ctx context.Context, userID uuid.UUID) (*User, error) {
	resp, err := c.authSvc.GetUserByID(ctx, &authpb.GetUserByIDRequest{UserId: userID.String()})
	if err != nil {
		return nil, err
	}
	if resp.User == nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	return &User{
		UserID:   userID,
		Username: resp.User.Username,
		Email:    resp.User.Email,
	}, nil
}

func (c *grpcClient) Close() error {
	return c.conn.Close()
}
