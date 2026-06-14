package gapi

import (
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/billing-service/internal/domain"
	"mem_pan/services/billing-service/internal/service"
	pb "mem_pan/services/billing-service/pb"
)

type Server struct {
	pb.UnimplementedBillingServiceServer
	billingSvc service.BillingService
}

func NewServer(billingSvc service.BillingService) *Server {
	return &Server{billingSvc: billingSvc}
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidPlan), errors.Is(err, domain.ErrInvalidWebhook), errors.Is(err, domain.ErrAmountMismatch), errors.Is(err, domain.ErrInvalidPayout), errors.Is(err, domain.ErrPayoutAmountTooSmall), errors.Is(err, domain.ErrInsufficientBalance):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrSubscriptionNotFound), errors.Is(err, domain.ErrPaymentNotFound), errors.Is(err, domain.ErrEarningNotFound), errors.Is(err, domain.ErrPayoutAccountNotFound), errors.Is(err, domain.ErrWithdrawalNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		slog.Error("billing internal error", "err", err.Error())
		return status.Error(codes.Internal, "internal server error")
	}
}
