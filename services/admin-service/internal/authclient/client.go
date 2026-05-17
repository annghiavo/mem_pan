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

type AdminUser struct {
	UserID    uuid.UUID
	Username  string
	Email     string
	Role      string
	IsBanned  bool
	CreatedAt string
}

type ListUsersResult struct {
	Users []AdminUser
	Total int64
}

type BanResult struct {
	UserID   uuid.UUID
	Username string
	Email    string
	IsBanned bool
}

type Client interface {
	VerifyToken(ctx context.Context, accessToken string) (*Payload, error)
	GetUserByID(ctx context.Context, userID uuid.UUID) (*UserProfile, error)
	SetUserRole(ctx context.Context, email, role string) (*PromotedUser, error)
	ListUsers(ctx context.Context, pageSize, offset int32, filterBanned bool) (*ListUsersResult, error)
	BanUser(ctx context.Context, userID uuid.UUID, ban bool, reason string) (*BanResult, error)
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

func (c *grpcClient) ListUsers(ctx context.Context, pageSize, offset int32, filterBanned bool) (*ListUsersResult, error) {
	resp, err := c.authSvc.ListUsers(ctx, &authpb.ListUsersRequest{
		PageSize:     pageSize,
		Offset:       offset,
		FilterBanned: filterBanned,
	})
	if err != nil {
		return nil, err
	}
	users := make([]AdminUser, 0, len(resp.Users))
	for _, u := range resp.Users {
		id, parseErr := uuid.Parse(u.UserId)
		if parseErr != nil {
			return nil, status.Error(codes.Internal, "invalid user_id in list response")
		}
		users = append(users, AdminUser{
			UserID:    id,
			Username:  u.Username,
			Email:     u.Email,
			Role:      u.Role,
			IsBanned:  u.IsBanned,
			CreatedAt: u.CreatedAt,
		})
	}
	return &ListUsersResult{Users: users, Total: resp.Total}, nil
}

func (c *grpcClient) BanUser(ctx context.Context, userID uuid.UUID, ban bool, reason string) (*BanResult, error) {
	resp, err := c.authSvc.BanUser(ctx, &authpb.BanUserRequest{
		UserId: userID.String(),
		Ban:    ban,
		Reason: reason,
	})
	if err != nil {
		return nil, err
	}
	id, parseErr := uuid.Parse(resp.UserId)
	if parseErr != nil {
		return nil, status.Error(codes.Internal, "invalid user_id in ban response")
	}
	return &BanResult{
		UserID:   id,
		Username: resp.Username,
		Email:    resp.Email,
		IsBanned: resp.IsBanned,
	}, nil
}

func (c *grpcClient) Close() error {
	return c.conn.Close()
}
