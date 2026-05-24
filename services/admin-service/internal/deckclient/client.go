package deckclient

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	deckpb "mem_pan/services/deck-service/pb"

	"crypto/tls"
	"google.golang.org/grpc/credentials"
	"strings"
)

type AdminDeck struct {
	DeckID      string
	UserID      string
	Name        string
	Description string
	IsPublic    bool
	Status      string
	CardCount   int32
	CreatedAt   string
	UpdatedAt   string
}

type ListDecksResult struct {
	Decks []AdminDeck
	Total int64
}

type Client interface {
	UpdateDeckStatus(ctx context.Context, deckID, status string) (string, string, error)
	ListDecks(ctx context.Context, pageSize, offset int32, statusFilter string) (*ListDecksResult, error)
	Close() error
}

type grpcClient struct {
	conn    *grpc.ClientConn
	deckSvc deckpb.DeckServiceClient
}

func NewGRPCClient(addr string) (Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(pickCreds(addr)))
	if err != nil {
		return nil, err
	}
	return &grpcClient{conn: conn, deckSvc: deckpb.NewDeckServiceClient(conn)}, nil
}

func (c *grpcClient) UpdateDeckStatus(ctx context.Context, deckID, status string) (string, string, error) {
	resp, err := c.deckSvc.AdminUpdateDeckStatus(ctx, &deckpb.AdminUpdateDeckStatusRequest{
		DeckId: deckID,
		Status: status,
	})
	if err != nil {
		return "", "", err
	}
	return resp.Status, resp.UserId, nil
}

func (c *grpcClient) ListDecks(ctx context.Context, pageSize, offset int32, statusFilter string) (*ListDecksResult, error) {
	resp, err := c.deckSvc.AdminListDecks(ctx, &deckpb.AdminListDecksRequest{
		PageSize:     pageSize,
		Offset:       offset,
		StatusFilter: statusFilter,
	})
	if err != nil {
		return nil, err
	}
	decks := make([]AdminDeck, 0, len(resp.Decks))
	for _, d := range resp.Decks {
		decks = append(decks, AdminDeck{
			DeckID:      d.DeckId,
			UserID:      d.UserId,
			Name:        d.Name,
			Description: d.Description,
			IsPublic:    d.IsPublic,
			Status:      d.Status,
			CardCount:   d.CardCount,
			CreatedAt:   d.CreatedAt,
			UpdatedAt:   d.UpdatedAt,
		})
	}
	return &ListDecksResult{Decks: decks, Total: resp.Total}, nil
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
