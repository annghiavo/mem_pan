package gapi

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/admin-service/internal/authclient"
	"mem_pan/services/admin-service/internal/domain"
	"mem_pan/services/admin-service/internal/notifyclient"
	"mem_pan/services/admin-service/internal/repository"
	"mem_pan/services/admin-service/internal/service"
	pb "mem_pan/services/admin-service/pb/proto"
)

type Server struct {
	pb.UnimplementedAdminServiceServer
	reportSvc    service.ReportService
	reportRepo   repository.ReportRepository
	authClient   authclient.Client
	notifyClient notifyclient.Client
}

func NewServer(
	reportSvc service.ReportService,
	reportRepo repository.ReportRepository,
	authClient authclient.Client,
	notifyClient notifyclient.Client,
) *Server {
	return &Server{
		reportSvc:    reportSvc,
		reportRepo:   reportRepo,
		authClient:   authClient,
		notifyClient: notifyClient,
	}
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, domain.ErrReportNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrAdminRequired):
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
