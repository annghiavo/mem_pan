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

type UserProfile struct {
	Username  string
	AvatarURL string
}

type PromotedUser struct {
	UserID   uuid.UUID
	Email    string
	Username string
}

type Client interface {
	VerifyToken(ctx context.Context, accessToken string) (*Payload, error)
	GetUserByID(ctx context.Context, userID uuid.UUID) (*UserProfile, error)
	SetUserRole(ctx context.Context, email, role string) (*PromotedUser, error)
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

func (c *grpcClient) SetUserRole(ctx context.Context, email, role string) (*PromotedUser, error) {
	resp, err := c.authSvc.SetUserRole(ctx, &authpb.SetUserRoleRequest{Email: email, Role: role})
	if err != nil {
		return nil, err
	}
	userID, err := uuid.Parse(resp.UserId)
	if err != nil {
		return nil, status.Error(codes.Internal, "invalid user_id in set-role response")
	}
	return &PromotedUser{
		UserID:   userID,
		Email:    resp.Email,
		Username: resp.Username,
	}, nil
}

func (c *grpcClient) Close() error {
	return c.conn.Close()
}
