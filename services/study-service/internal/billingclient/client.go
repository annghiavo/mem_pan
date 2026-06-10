package billingclient

import (
	"context"
	"crypto/tls"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	billingpb "mem_pan/services/billing-service/pb"
)

type Client interface {
	CheckPlusAccess(ctx context.Context, userID uuid.UUID) (bool, error)
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

func (c *grpcClient) CheckPlusAccess(ctx context.Context, userID uuid.UUID) (bool, error) {
	resp, err := c.billingSvc.CheckPlusAccess(ctx, &billingpb.CheckPlusAccessRequest{UserId: userID.String()})
	if err != nil {
		return false, err
	}
	return resp.Active, nil
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
