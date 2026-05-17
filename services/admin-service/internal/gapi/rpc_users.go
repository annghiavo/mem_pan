package gapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/admin-service/internal/authclient"
	"mem_pan/services/admin-service/internal/db"
	pb "mem_pan/services/admin-service/pb/proto"
)

func (s *Server) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if !isModerator(payload.Role) {
		return nil, status.Error(codes.PermissionDenied, "moderator access required")
	}

	pageSize := req.GetPageSize()
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	offset := int32(0)
	if token := req.GetPageToken(); token != "" {
		v, err := strconv.Atoi(token)
		if err != nil || v < 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid page_token")
		}
		offset = int32(v)
	}

	result, err := s.authClient.ListUsers(ctx, pageSize, offset, req.GetFilterBanned())
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list users")
	}

	users := make([]*pb.User, 0, len(result.Users))
	for _, u := range result.Users {
		users = append(users, adminUserToPb(u))
	}

	nextToken := ""
	if int64(offset)+int64(len(result.Users)) < result.Total {
		nextToken = strconv.Itoa(int(offset) + len(result.Users))
	}

	return &pb.ListUsersResponse{Users: users, NextPageToken: nextToken}, nil
}

func (s *Server) BanUser(ctx context.Context, req *pb.BanUserRequest) (*pb.BanUserResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if !isModerator(payload.Role) {
		return nil, status.Error(codes.PermissionDenied, "moderator access required")
	}

	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}

	banResult, err := s.authClient.BanUser(ctx, userID, req.GetBan(), req.GetReason())
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to update ban status")
	}

	action := "ban_user"
	if !req.GetBan() {
		action = "unban_user"
	}
	logMeta, _ := json.Marshal(map[string]any{"target_username": banResult.Username})
	_, _ = s.reportRepo.CreateModerationLog(ctx, db.CreateModerationLogParams{
		AdminID:    payload.UserID,
		Action:     action,
		TargetType: "user",
		TargetID:   userID,
		Reason:     sql.NullString{String: req.GetReason(), Valid: req.GetReason() != ""},
		Metadata:   pqtype.NullRawMessage{RawMessage: logMeta, Valid: true},
	})

	return &pb.BanUserResponse{User: &pb.User{
		Id:        banResult.UserID.String(),
		Username:  banResult.Username,
		Email:     banResult.Email,
		IsBanned:  banResult.IsBanned,
		CreatedAt: "",
	}}, nil
}

func (s *Server) PromoteModerator(ctx context.Context, req *pb.PromoteModeratorRequest) (*pb.PromoteModeratorResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if payload.Role != "admin" {
		return nil, status.Error(codes.PermissionDenied, "admin access required")
	}
	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	promoted, err := s.authClient.SetUserRole(ctx, req.Email, "moderator")
	if err != nil {
		return nil, err
	}

	return &pb.PromoteModeratorResponse{
		UserId:   promoted.UserID.String(),
		Email:    promoted.Email,
		Username: promoted.Username,
	}, nil
}

func adminUserToPb(u authclient.AdminUser) *pb.User {
	return &pb.User{
		Id:        u.UserID.String(),
		Username:  u.Username,
		Email:     u.Email,
		IsBanned:  u.IsBanned,
		CreatedAt: u.CreatedAt,
	}
}

func isModerator(role string) bool {
	return role == "admin" || role == "moderator"
}
