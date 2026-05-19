// Package statsclient wraps the stats-service gRPC client, exposing only the
// internal RPC used by reminder cron jobs.
package statsclient

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	statspb "mem_pan/services/stats-service/pb"
)

type ReminderState struct {
	UserID             string
	CurrentStreak      int32
	LastStudiedDate    string // "" if never studied
	OptimalHourWeekday int32  // -1 if not yet computed
	OptimalHourWeekend int32  // -1 if not yet computed
	ReminderLocalTime  string // "HH:MM", default "21:00"
}

type Client interface {
	ListReminderState(ctx context.Context, onlyActiveStreak bool) ([]ReminderState, error)
	Close() error
}

type grpcClient struct {
	conn *grpc.ClientConn
	svc  statspb.StatsServiceClient
}

func NewGRPCClient(addr string) (Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &grpcClient{conn: conn, svc: statspb.NewStatsServiceClient(conn)}, nil
}

func (c *grpcClient) ListReminderState(ctx context.Context, onlyActiveStreak bool) ([]ReminderState, error) {
	resp, err := c.svc.ListReminderState(ctx, &statspb.ListReminderStateRequest{
		OnlyActiveStreak: onlyActiveStreak,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ReminderState, 0, len(resp.Users))
	for _, u := range resp.Users {
		out = append(out, ReminderState{
			UserID:             u.UserId,
			CurrentStreak:      u.CurrentStreak,
			LastStudiedDate:    u.LastStudiedDate,
			OptimalHourWeekday: u.OptimalHourWeekday,
			OptimalHourWeekend: u.OptimalHourWeekend,
			ReminderLocalTime:  u.ReminderLocalTime,
		})
	}
	return out, nil
}

func (c *grpcClient) Close() error { return c.conn.Close() }
