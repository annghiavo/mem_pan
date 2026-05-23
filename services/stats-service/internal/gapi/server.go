package gapi

import (
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/stats-service/internal/authclient"
	"mem_pan/services/stats-service/internal/domain"
	"mem_pan/services/stats-service/internal/service"
	pb "mem_pan/services/stats-service/pb"
)

type Server struct {
	pb.UnimplementedStatsServiceServer
	statsSvc   service.StatsService
	authClient authclient.Client
}

func NewServer(statsSvc service.StatsService, authClient authclient.Client) *Server {
	return &Server{
		statsSvc:   statsSvc,
		authClient: authClient,
	}
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, domain.ErrUserStatsNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrDeckStatsNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		slog.Error("stats internal error", "err", err.Error())
		return status.Error(codes.Internal, "internal server error")
	}
}
