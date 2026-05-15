package gapi

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"mem_pan/services/search-service/internal/es"
	"mem_pan/services/search-service/internal/service"
	pb "mem_pan/services/search-service/pb"
)

func (s *Server) SearchDecks(ctx context.Context, req *pb.SearchDecksRequest) (*pb.SearchDecksResponse, error) {
	scope := service.DeckScopePublic
	switch req.GetScope() {
	case pb.DeckSearchScope_DECK_SCOPE_MINE:
		scope = service.DeckScopeMine
	case pb.DeckSearchScope_DECK_SCOPE_ALL:
		scope = service.DeckScopeAll
	}

	callerID := ""
	if scope == service.DeckScopeMine || scope == service.DeckScopeAll {
		payload, err := s.authorizeUser(ctx)
		if err != nil {
			return nil, err
		}
		callerID = payload.UserID.String()
	} else {
		if payload, _ := s.optionalUser(ctx); payload != nil {
			callerID = payload.UserID.String()
		}
	}

	result, err := s.svc.SearchDecks(ctx, service.DeckSearchParams{
		Query:    req.GetQuery(),
		Scope:    scope,
		CallerID: callerID,
		Page:     req.GetPage(),
		PageSize: req.GetPageSize(),
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "search failed")
	}

	decks := make([]*pb.Deck, 0, len(result.Hits))
	for _, h := range result.Hits {
		var doc es.DeckDoc
		if err := json.Unmarshal(h.Source, &doc); err != nil {
			continue
		}
		decks = append(decks, &pb.Deck{
			DeckId:      doc.DeckID,
			UserId:      doc.UserID,
			Name:        doc.Name,
			Description: doc.Description,
			IsPublic:    doc.IsPublic,
			Status:      doc.Status,
			CardCount:   doc.CardCount,
			ClonedFrom:  doc.ClonedFrom,
			CreatedAt:   timestamppb.New(doc.CreatedAt),
			UpdatedAt:   timestamppb.New(doc.UpdatedAt),
			Score:       h.Score,
		})
	}
	return &pb.SearchDecksResponse{Decks: decks, Total: result.Total}, nil
}
