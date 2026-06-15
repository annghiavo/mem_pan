package gapi

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"mem_pan/services/billing-service/internal/db"
	"mem_pan/services/billing-service/internal/service"
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

func (s *Server) SyncRevenuePool(ctx context.Context, req *pb.SyncRevenuePoolRequest) (*pb.SyncRevenuePoolResponse, error) {
	if req.GetPool() == nil {
		return nil, status.Error(codes.InvalidArgument, "pool is required")
	}
	poolMonth, err := time.Parse("2006-01-02", req.Pool.PoolMonth)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid pool_month")
	}
	pool := db.UpsertMonthlyRevenuePoolParams{
		PoolMonth:            poolMonth,
		GrossAmountVnd:       req.Pool.GrossAmountVnd,
		CreatorPoolAmountVnd: req.Pool.CreatorPoolAmountVnd,
		PlatformAmountVnd:    req.Pool.PlatformAmountVnd,
		Status:               req.Pool.Status,
	}
	if ts := req.Pool.FinalizedAt; ts != nil {
		pool.FinalizedAt = sql.NullTime{Time: ts.AsTime().UTC(), Valid: true}
	}

	earnings := make([]db.UpsertCreatorEarningParams, 0, len(req.Earnings))
	for _, item := range req.Earnings {
		creatorID, err := uuid.Parse(item.CreatorId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid creator_id")
		}
		itemPoolMonth, err := time.Parse("2006-01-02", item.PoolMonth)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid earning pool_month")
		}
		earnings = append(earnings, db.UpsertCreatorEarningParams{
			PoolMonth:        itemPoolMonth,
			CreatorID:        creatorID,
			EligibleLearners: item.EligibleLearners,
			WeightedScore:    item.WeightedScore,
			AmountVnd:        item.AmountVnd,
			Status:           item.Status,
		})
	}

	if err := s.billingSvc.SyncRevenuePool(ctx, service.RevenuePoolSyncInput{
		Pool:     pool,
		Earnings: earnings,
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.SyncRevenuePoolResponse{
		Ok:             true,
		SyncedEarnings: int32(len(earnings)),
	}, nil
}

func (s *Server) GetAllocatedRevenue(ctx context.Context, req *pb.GetAllocatedRevenueRequest) (*pb.GetAllocatedRevenueResponse, error) {
	amount, err := s.billingSvc.GetAllocatedRevenue(ctx, req.GetPoolMonth())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.GetAllocatedRevenueResponse{
		AllocatedGrossAmountVnd: amount,
	}, nil
}
