package gapi

import (
	"errors"

	"github.com/cloudinary/cloudinary-go/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"mem_pan/services/auth-service/internal/db"
	"mem_pan/services/auth-service/internal/domain"
	"mem_pan/services/auth-service/internal/publisher"
	"mem_pan/services/auth-service/internal/service"
	"mem_pan/services/auth-service/internal/token"
	"mem_pan/services/auth-service/pb"
)

type Server struct {
	pb.UnimplementedAuthServiceServer
	authSvc    service.AuthService
	userSvc    service.UserService
	tokenMaker token.Maker
	cld        *cloudinary.Cloudinary
	pub        publisher.EventPublisher
}

func NewServer(authSvc service.AuthService, userSvc service.UserService, tokenMaker token.Maker, cld *cloudinary.Cloudinary, pub publisher.EventPublisher) *Server {
	return &Server{
		authSvc:    authSvc,
		userSvc:    userSvc,
		tokenMaker: tokenMaker,
		cld:        cld,
		pub:        pub,
	}
}

func dbUserToPb(u db.User) *pb.User {
	r := &pb.User{
		UserId:        u.UserID.String(),
		Username:      u.Username,
		Email:         u.Email,
		Role:          u.Role,
		EmailVerified: u.EmailVerified,
		CreatedAt:     timestamppb.New(u.CreatedAt),
	}
	if u.FullName.Valid {
		r.FullName = u.FullName.String
	}
	if u.AvatarUrl.Valid {
		r.AvatarUrl = u.AvatarUrl.String
	}
	r.Timezone = u.Timezone
	return r
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrEmailAlreadyExists),
		errors.Is(err, domain.ErrUsernameAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrUserBanned):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrTokenNotFound),
		errors.Is(err, domain.ErrTokenExpired),
		errors.Is(err, domain.ErrTokenAlreadyUsed),
		errors.Is(err, domain.ErrTokenRevoked):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
