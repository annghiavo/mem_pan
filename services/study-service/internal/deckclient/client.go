package deckclient

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	deckpb "mem_pan/services/deck-service/pb"

	"crypto/tls"
	"google.golang.org/grpc/credentials"
	"strings"
)

type CardInfo struct {
	CardID uuid.UUID
	DeckID uuid.UUID
}

type DeckInfo struct {
	DeckID      uuid.UUID
	UserID      uuid.UUID
	AccessLevel string
	PlusStatus  string
	IsPublic    bool
}

type Client interface {
	GetDeck(ctx context.Context, deckID uuid.UUID, accessToken string) (DeckInfo, error)
	ListDeckCards(ctx context.Context, deckID uuid.UUID, accessToken string) ([]CardInfo, error)
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
	return &grpcClient{
		conn:    conn,
		deckSvc: deckpb.NewDeckServiceClient(conn),
	}, nil
}

func (c *grpcClient) GetDeck(ctx context.Context, deckID uuid.UUID, accessToken string) (DeckInfo, error) {
	md := metadata.Pairs("authorization", "Bearer "+accessToken)
	ctx = metadata.NewOutgoingContext(ctx, md)

	resp, err := c.deckSvc.GetDeck(ctx, &deckpb.GetDeckRequest{DeckId: deckID.String()})
	if err != nil {
		return DeckInfo{}, fmt.Errorf("deck-service GetDeck: %w", err)
	}
	if resp.Deck == nil {
		return DeckInfo{}, fmt.Errorf("deck-service GetDeck: missing deck")
	}
	userID, err := uuid.Parse(resp.Deck.UserId)
	if err != nil {
		return DeckInfo{}, fmt.Errorf("deck-service GetDeck: invalid user_id")
	}
	dID, err := uuid.Parse(resp.Deck.DeckId)
	if err != nil {
		return DeckInfo{}, fmt.Errorf("deck-service GetDeck: invalid deck_id")
	}
	return DeckInfo{
		DeckID:      dID,
		UserID:      userID,
		AccessLevel: resp.Deck.AccessLevel,
		PlusStatus:  resp.Deck.PlusStatus,
		IsPublic:    resp.Deck.IsPublic,
	}, nil
}

func (c *grpcClient) ListDeckCards(ctx context.Context, deckID uuid.UUID, accessToken string) ([]CardInfo, error) {
	md := metadata.Pairs("authorization", "Bearer "+accessToken)
	ctx = metadata.NewOutgoingContext(ctx, md)

	resp, err := c.deckSvc.ListDeckCards(ctx, &deckpb.ListDeckCardsRequest{
		DeckId: deckID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("deck-service ListDeckCards: %w", err)
	}

	cards := make([]CardInfo, 0, len(resp.Cards))
	for _, c := range resp.Cards {
		cardID, err := uuid.Parse(c.CardId)
		if err != nil {
			continue
		}
		dID, err := uuid.Parse(c.DeckId)
		if err != nil {
			continue
		}
		cards = append(cards, CardInfo{CardID: cardID, DeckID: dID})
	}
	return cards, nil
}

func (c *grpcClient) Close() error {
	return c.conn.Close()
}

// pickCreds returns TLS credentials when the target appears to be a
// Cloud Run / managed endpoint (port :443 or *.run.app), otherwise an
// insecure transport for local docker-compose or in-cluster gRPC.
func pickCreds(addr string) credentials.TransportCredentials {
	if strings.HasSuffix(addr, ":443") || strings.Contains(addr, ".run.app") {
		return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	}
	return insecure.NewCredentials()
}
