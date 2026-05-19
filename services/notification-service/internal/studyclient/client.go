// Package studyclient wraps the study-service gRPC client, exposing only the
// internal RPCs used by reminder cron jobs.
package studyclient

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	studypb "mem_pan/services/study-service/pb"
)

type Client interface {
	CountDueForUser(ctx context.Context, userID, timezone string) (int32, error)
	Close() error
}

type grpcClient struct {
	conn *grpc.ClientConn
	svc  studypb.StudyServiceClient
}

func NewGRPCClient(addr string) (Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &grpcClient{conn: conn, svc: studypb.NewStudyServiceClient(conn)}, nil
}

func (c *grpcClient) CountDueForUser(ctx context.Context, userID, timezone string) (int32, error) {
	resp, err := c.svc.CountDueForUser(ctx, &studypb.CountDueForUserRequest{
		UserId:   userID,
		Timezone: timezone,
	})
	if err != nil {
		return 0, err
	}
	return resp.DueToday, nil
}

func (c *grpcClient) Close() error { return c.conn.Close() }
