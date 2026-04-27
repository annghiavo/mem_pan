package gapi

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/deck-service/internal/service"
	"mem_pan/services/deck-service/pb"
)

func (s *Server) ListDecks(ctx context.Context, req *pb.ListDecksRequest) (*pb.ListDecksResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (req.Page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	page, err := s.deckSvc.ListDecks(ctx, service.ListDecksParams{
		UserID: payload.UserID,
		Limit:  pageSize,
		Offset: offset,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	decks := make([]*pb.Deck, len(page.Decks))
	for i, d := range page.Decks {
		decks[i] = dbDeckToPb(d)
	}
	return &pb.ListDecksResponse{Decks: decks, Total: page.Total}, nil
}

func (s *Server) CreateDeck(ctx context.Context, req *pb.CreateDeckRequest) (*pb.CreateDeckResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	deck, err := s.deckSvc.CreateDeck(ctx, service.CreateDeckParams{
		UserID:      payload.UserID,
		Name:        req.Name,
		Description: nullStrFromProto(req.Description),
		IsPublic:    req.IsPublic,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.CreateDeckResponse{Deck: dbDeckToPb(deck)}, nil
}

func (s *Server) ListPublicDecks(ctx context.Context, req *pb.ListPublicDecksRequest) (*pb.ListPublicDecksResponse, error) {
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (req.Page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	page, err := s.deckSvc.ListPublicDecks(ctx, service.ListPublicDecksParams{
		Limit:  pageSize,
		Offset: offset,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	decks := make([]*pb.Deck, len(page.Decks))
	for i, d := range page.Decks {
		decks[i] = dbDeckToPb(d)
	}
	return &pb.ListPublicDecksResponse{Decks: decks, Total: page.Total}, nil
}

func (s *Server) GetDeck(ctx context.Context, req *pb.GetDeckRequest) (*pb.GetDeckResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	deckID, err := uuid.Parse(req.DeckId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}

	deck, err := s.deckSvc.GetDeck(ctx, deckID, payload.UserID, true)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.GetDeckResponse{Deck: dbDeckToPb(deck)}, nil
}

func (s *Server) UpdateDeck(ctx context.Context, req *pb.UpdateDeckRequest) (*pb.UpdateDeckResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	deckID, err := uuid.Parse(req.DeckId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}

	deck, err := s.deckSvc.UpdateDeck(ctx, service.UpdateDeckParams{
		DeckID:      deckID,
		UserID:      payload.UserID,
		Name:        nullStrFromProto(req.Name),
		Description: nullStrFromProto(req.Description),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.UpdateDeckResponse{Deck: dbDeckToPb(deck)}, nil
}

func (s *Server) DeleteDeck(ctx context.Context, req *pb.DeleteDeckRequest) (*pb.DeleteDeckResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	deckID, err := uuid.Parse(req.DeckId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}

	if err := s.deckSvc.DeleteDeck(ctx, deckID, payload.UserID); err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.DeleteDeckResponse{Success: true}, nil
}

func (s *Server) UpdateDeckSettings(ctx context.Context, req *pb.UpdateDeckSettingsRequest) (*pb.UpdateDeckSettingsResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	deckID, err := uuid.Parse(req.DeckId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}
	if req.Settings == nil {
		return nil, status.Error(codes.InvalidArgument, "settings is required")
	}

	settings := service.DeckSettings{
		QuizType:       req.Settings.QuizType,
		AnswerSide:     req.Settings.AnswerSide,
		StrictTyping:   req.Settings.StrictTyping,
		PartialCorrect: req.Settings.PartialCorrect,
		NewCardsPerDay: req.Settings.NewCardsPerDay,
		ReviewsPerDay:  req.Settings.ReviewsPerDay,
	}

	deck, err := s.deckSvc.UpdateSettings(ctx, deckID, payload.UserID, settings)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.UpdateDeckSettingsResponse{Deck: dbDeckToPb(deck)}, nil
}

func (s *Server) UpdateDeckVisibility(ctx context.Context, req *pb.UpdateDeckVisibilityRequest) (*pb.UpdateDeckVisibilityResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	deckID, err := uuid.Parse(req.DeckId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}

	deck, err := s.deckSvc.UpdateVisibility(ctx, deckID, payload.UserID, req.IsPublic)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.UpdateDeckVisibilityResponse{Deck: dbDeckToPb(deck)}, nil
}

func (s *Server) CloneDeck(ctx context.Context, req *pb.CloneDeckRequest) (*pb.CloneDeckResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	deckID, err := uuid.Parse(req.DeckId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}

	deck, err := s.deckSvc.CloneDeck(ctx, deckID, payload.UserID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.CloneDeckResponse{Deck: dbDeckToPb(deck)}, nil
}

func (s *Server) GetDeckStats(ctx context.Context, req *pb.GetDeckStatsRequest) (*pb.GetDeckStatsResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	deckID, err := uuid.Parse(req.DeckId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}

	stats, err := s.deckSvc.GetStats(ctx, deckID, payload.UserID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.GetDeckStatsResponse{
		Stats: &pb.DeckStats{
			DeckId:     stats.DeckID.String(),
			TotalCards: int32(stats.TotalCards),
		},
	}, nil
}

func (s *Server) ListDeckCards(ctx context.Context, req *pb.ListDeckCardsRequest) (*pb.ListDeckCardsResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	deckID, err := uuid.Parse(req.DeckId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}

	cards, err := s.cardSvc.ListCardsByDeck(ctx, deckID, payload.UserID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	out := make([]*pb.Card, len(cards))
	for i, c := range cards {
		out[i] = dbListCardRowToPb(c)
	}
	return &pb.ListDeckCardsResponse{Cards: out}, nil
}

func (s *Server) CreateCard(ctx context.Context, req *pb.CreateCardRequest) (*pb.CreateCardResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	deckID, err := uuid.Parse(req.DeckId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}
	if req.ContentFront == "" || req.ContentBack == "" {
		return nil, status.Error(codes.InvalidArgument, "content_front and content_back are required")
	}

	card, err := s.cardSvc.CreateCard(ctx, service.CreateCardParams{
		UserID:       payload.UserID,
		DeckID:       deckID,
		ContentFront: req.ContentFront,
		ContentBack:  req.ContentBack,
		ImageURL:     nullStrFromProto(req.ImageUrl),
		Position:     req.Position,
		LangFront:    req.LangFront,
		LangBack:     req.LangBack,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.CreateCardResponse{Card: dbCardRowToPb(card)}, nil
}

func (s *Server) BulkCreateCards(ctx context.Context, req *pb.BulkCreateCardsRequest) (*pb.BulkCreateCardsResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	deckID, err := uuid.Parse(req.DeckId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}
	if len(req.Cards) == 0 {
		return nil, status.Error(codes.InvalidArgument, "cards list is empty")
	}

	items := make([]service.CreateCardParams, len(req.Cards))
	for i, c := range req.Cards {
		if c.ContentFront == "" || c.ContentBack == "" {
			return nil, status.Errorf(codes.InvalidArgument, "card[%d]: content_front and content_back are required", i)
		}
		items[i] = service.CreateCardParams{
			ContentFront: c.ContentFront,
			ContentBack:  c.ContentBack,
			ImageURL:     nullStrFromProto(c.ImageUrl),
			Position:     int32(i),
			LangFront:    c.LangFront,
			LangBack:     c.LangBack,
		}
	}

	created, err := s.cardSvc.BulkCreateCards(ctx, payload.UserID, deckID, items)
	if err != nil {
		return nil, toGRPCError(err)
	}

	out := make([]*pb.Card, len(created))
	for i, c := range created {
		out[i] = dbCardRowToPb(c)
	}
	return &pb.BulkCreateCardsResponse{Cards: out, Created: int32(len(out))}, nil
}
