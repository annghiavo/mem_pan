// Package moderationclient is study-service's gRPC client to
// moderation-fsrs-service's FsrsOptimizationService (Python). It is used by the
// daily FSRS-weight optimization cron to re-tune each eligible user's 21 weights.
package moderationclient

import (
	"context"
	"crypto/tls"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	pb "mem_pan/services/study-service/pb"
)

// ReviewLog is one training sample fed to the optimizer.
type ReviewLog struct {
	CardID      string
	ReviewDate  int64 // unix seconds
	Rating      int32 // 1..4
	ElapsedDays int32
}

// OptimizeResult is the optimizer's output for one user.
type OptimizeResult struct {
	Weights     []float64
	NumReviews  int32
	Loss        float64
	FsrsVersion string
}

type Client interface {
	OptimizeWeights(ctx context.Context, userID string, logs []ReviewLog) (*OptimizeResult, error)
	Close() error
}

type grpcClient struct {
	conn *grpc.ClientConn
	fsrs pb.FsrsOptimizationServiceClient
}

func NewGRPCClient(addr string) (Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(pickCreds(addr)))
	if err != nil {
		return nil, err
	}
	return &grpcClient{
		conn: conn,
		fsrs: pb.NewFsrsOptimizationServiceClient(conn),
	}, nil
}

func (c *grpcClient) OptimizeWeights(ctx context.Context, userID string, logs []ReviewLog) (*OptimizeResult, error) {
	req := &pb.OptimizeWeightsRequest{
		UserId:     userID,
		ReviewLogs: make([]*pb.ReviewLog, 0, len(logs)),
	}
	for _, l := range logs {
		req.ReviewLogs = append(req.ReviewLogs, &pb.ReviewLog{
			CardId:      l.CardID,
			ReviewDate:  l.ReviewDate,
			Rating:      l.Rating,
			ElapsedDays: l.ElapsedDays,
		})
	}

	resp, err := c.fsrs.OptimizeWeights(ctx, req)
	if err != nil {
		return nil, err
	}

	weights := make([]float64, len(resp.Weights))
	for i, w := range resp.Weights {
		weights[i] = float64(w)
	}
	return &OptimizeResult{
		Weights:     weights,
		NumReviews:  resp.NumReviewsUsed,
		Loss:        float64(resp.Loss),
		FsrsVersion: resp.FsrsVersion,
	}, nil
}

func (c *grpcClient) Close() error {
	return c.conn.Close()
}

// pickCreds mirrors authclient/deckclient: TLS for Cloud Run (*.run.app or :443),
// insecure for local docker-compose / in-cluster gRPC.
func pickCreds(addr string) credentials.TransportCredentials {
	if strings.HasSuffix(addr, ":443") || strings.Contains(addr, ".run.app") {
		return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	}
	return insecure.NewCredentials()
}
