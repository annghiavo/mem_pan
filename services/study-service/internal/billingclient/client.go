package billingclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	billingpb "mem_pan/services/billing-service/pb"
	studb "mem_pan/services/study-service/internal/db"
)

type Client interface {
	SyncRevenuePool(ctx context.Context, pool studb.MonthlyRevenuePool, earnings []studb.CreatorEarning) error
	GetAllocatedRevenue(ctx context.Context, poolMonth string) (int64, error)
	Close() error
}

type grpcClient struct {
	conn       *grpc.ClientConn
	billingSvc billingpb.BillingServiceClient
}

func NewGRPCClient(addr string) (Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(pickCreds(addr)))
	if err != nil {
		return nil, err
	}
	return &grpcClient{
		conn:       conn,
		billingSvc: billingpb.NewBillingServiceClient(conn),
	}, nil
}

func (c *grpcClient) SyncRevenuePool(ctx context.Context, pool studb.MonthlyRevenuePool, earnings []studb.CreatorEarning) error {
	req := &billingpb.SyncRevenuePoolRequest{
		Pool: &billingpb.RevenuePool{
			PoolMonth:            pool.PoolMonth.Format("2006-01-02"),
			GrossAmountVnd:       pool.GrossAmountVnd,
			CreatorPoolAmountVnd: pool.CreatorPoolAmountVnd,
			PlatformAmountVnd:    pool.PlatformAmountVnd,
			Status:               pool.Status,
		},
		Earnings: make([]*billingpb.CreatorEarning, 0, len(earnings)),
	}
	if pool.FinalizedAt.Valid {
		req.Pool.FinalizedAt = timestamppb.New(pool.FinalizedAt.Time)
	}
	for _, earning := range earnings {
		req.Earnings = append(req.Earnings, &billingpb.CreatorEarning{
			PoolMonth:        earning.PoolMonth.Format("2006-01-02"),
			CreatorId:        earning.CreatorID.String(),
			EligibleLearners: earning.EligibleLearners,
			WeightedScore:    earning.WeightedScore,
			AmountVnd:        earning.AmountVnd,
			Status:           firstNonEmpty(earning.Status, "pending"),
		})
	}
	syncCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if _, err := c.billingSvc.SyncRevenuePool(syncCtx, req); err != nil {
		return fmt.Errorf("billing-service SyncRevenuePool: %w", err)
	}
	return nil
}

func (c *grpcClient) GetAllocatedRevenue(ctx context.Context, poolMonth string) (int64, error) {
	resp, err := c.billingSvc.GetAllocatedRevenue(ctx, &billingpb.GetAllocatedRevenueRequest{
		PoolMonth: poolMonth,
	})
	if err != nil {
		return 0, err
	}
	return resp.GetAllocatedGrossAmountVnd(), nil
}

func (c *grpcClient) Close() error {
	return c.conn.Close()
}

func pickCreds(addr string) credentials.TransportCredentials {
	if strings.HasSuffix(addr, ":443") || strings.Contains(addr, ".run.app") {
		return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	}
	return insecure.NewCredentials()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
