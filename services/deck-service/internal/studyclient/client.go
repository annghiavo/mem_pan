package studyclient

import (
	"context"
	"crypto/tls"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	studypb "mem_pan/services/study-service/pb"
)

// DeckLearners is one ranked deck from the trending leaderboard.
type DeckLearners struct {
	DeckID   uuid.UUID
	Learners int64
}

// Client fetches study-side data that deck-service enriches its responses with.
type Client interface {
	// CountDeckLearners returns the number of distinct users who have ever
	// started a study session on the deck (deck owner included).
	CountDeckLearners(ctx context.Context, deckID uuid.UUID) (int64, error)
	// TopDecksByLearners ranks decks (regardless of visibility) by distinct
	// learners active within the last windowDays days. deck-service filters the
	// result down to public decks.
	TopDecksByLearners(ctx context.Context, windowDays, limit int32) ([]DeckLearners, error)
	Close() error
}

type grpcClient struct {
	conn     *grpc.ClientConn
	studySvc studypb.StudyServiceClient
}

func NewGRPCClient(addr string) (Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(pickCreds(addr)))
	if err != nil {
		return nil, err
	}
	return &grpcClient{
		conn:     conn,
		studySvc: studypb.NewStudyServiceClient(conn),
	}, nil
}

func (c *grpcClient) CountDeckLearners(ctx context.Context, deckID uuid.UUID) (int64, error) {
	resp, err := c.studySvc.CountDeckLearners(ctx, &studypb.CountDeckLearnersRequest{DeckId: deckID.String()})
	if err != nil {
		return 0, err
	}
	return resp.Count, nil
}

func (c *grpcClient) TopDecksByLearners(ctx context.Context, windowDays, limit int32) ([]DeckLearners, error) {
	resp, err := c.studySvc.TopDecksByLearners(ctx, &studypb.TopDecksByLearnersRequest{
		WindowDays: windowDays,
		Limit:      limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]DeckLearners, 0, len(resp.Decks))
	for _, d := range resp.Decks {
		id, err := uuid.Parse(d.DeckId)
		if err != nil {
			continue // skip malformed IDs rather than failing the whole list
		}
		out = append(out, DeckLearners{DeckID: id, Learners: d.Learners})
	}
	return out, nil
}

func (c *grpcClient) Close() error {
	return c.conn.Close()
}

// pickCreds mirrors authclient: TLS for Cloud Run / managed endpoints (:443 or
// *.run.app), insecure for local docker-compose or in-cluster gRPC.
func pickCreds(addr string) credentials.TransportCredentials {
	if strings.HasSuffix(addr, ":443") || strings.Contains(addr, ".run.app") {
		return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	}
	return insecure.NewCredentials()
}
