// Package statsclient wraps the stats-service gRPC client, exposing only the
// internal RPC used by reminder cron jobs.
package statsclient

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	statspb "mem_pan/services/stats-service/pb"

	"crypto/tls"
	"google.golang.org/grpc/credentials"
	"strings"
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
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(pickCreds(addr)))
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

// pickCreds returns TLS credentials when the target appears to be a
// Cloud Run / managed endpoint (port :443 or *.run.app), otherwise an
// insecure transport for local docker-compose or in-cluster gRPC.
func pickCreds(addr string) credentials.TransportCredentials {
	if strings.HasSuffix(addr, ":443") || strings.Contains(addr, ".run.app") {
		return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	}
	return insecure.NewCredentials()
}
