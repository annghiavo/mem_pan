package gapi

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "mem_pan/services/billing-service/pb"
)

func (s *Server) CheckPlusAccess(ctx context.Context, req *pb.CheckPlusAccessRequest) (*pb.CheckPlusAccessResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}
	st, err := s.billingSvc.CheckPlusAccess(ctx, userID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	resp := &pb.CheckPlusAccessResponse{
		Active:   st.Active,
		PlanCode: st.PlanCode,
	}
	if !st.CurrentPeriodEnd.IsZero() {
		resp.CurrentPeriodEnd = timestamppb.New(st.CurrentPeriodEnd)
	}
	return resp, nil
}

func (s *Server) ExpireSubscriptions(ctx context.Context, _ *pb.ExpireSubscriptionsRequest) (*pb.ExpireSubscriptionsResponse, error) {
	if err := s.billingSvc.ExpireSubscriptions(ctx); err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.ExpireSubscriptionsResponse{Ok: true}, nil
}
