package gapi

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/notification-service/internal/authclient"
	"mem_pan/services/notification-service/internal/domain"
	"mem_pan/services/notification-service/internal/service"
	pb "mem_pan/services/notification-service/pb"
)

type Server struct {
	pb.UnimplementedNotificationServiceServer
	svc        service.NotificationService
	authClient authclient.Client
}

func NewServer(svc service.NotificationService, authClient authclient.Client) *Server {
	return &Server{svc: svc, authClient: authClient}
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, domain.ErrTokenNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
