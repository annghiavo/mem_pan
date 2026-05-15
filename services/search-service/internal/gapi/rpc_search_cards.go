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

func (s *Server) SearchCards(ctx context.Context, req *pb.SearchCardsRequest) (*pb.SearchCardsResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.svc.SearchCards(ctx, service.CardSearchParams{
		Query:    req.GetQuery(),
		UserID:   payload.UserID.String(),
		DeckID:   req.GetDeckId(),
		Page:     req.GetPage(),
		PageSize: req.GetPageSize(),
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "search failed")
	}

	cards := make([]*pb.Card, 0, len(result.Hits))
	for _, h := range result.Hits {
		var doc es.CardDoc
		if err := json.Unmarshal(h.Source, &doc); err != nil {
			continue
		}
		cards = append(cards, &pb.Card{
			CardId:       doc.CardID,
			UserId:       doc.UserID,
			DeckId:       doc.DeckID,
			NoteId:       doc.NoteID,
			ContentFront: doc.ContentFront,
			ContentBack:  doc.ContentBack,
			CreatedAt:    timestamppb.New(doc.CreatedAt),
			Score:        h.Score,
		})
	}
	return &pb.SearchCardsResponse{Cards: cards, Total: result.Total}, nil
}
