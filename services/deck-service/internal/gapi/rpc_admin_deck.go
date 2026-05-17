package gapi

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/deck-service/internal/db"
	"mem_pan/services/deck-service/pb"
)

var allowedDeckStatuses = map[string]bool{
	"active":  true,
	"hidden":  true,
	"deleted": true,
}

func (s *Server) AdminUpdateDeckStatus(ctx context.Context, req *pb.AdminUpdateDeckStatusRequest) (*pb.AdminUpdateDeckStatusResponse, error) {
	deckID, err := uuid.Parse(req.GetDeckId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}
	if !allowedDeckStatuses[req.GetStatus()] {
		return nil, status.Error(codes.InvalidArgument, "status must be active, hidden, or deleted")
	}

	deck, err := s.deckSvc.AdminUpdateDeckStatus(ctx, deckID, req.GetStatus())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.AdminUpdateDeckStatusResponse{
		DeckId: deck.DeckID.String(),
		Status: deck.Status,
	}, nil
}

func (s *Server) AdminListDecks(ctx context.Context, req *pb.AdminListDecksRequest) (*pb.AdminListDecksResponse, error) {
	limit := req.GetPageSize()
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := req.GetOffset()
	if offset < 0 {
		offset = 0
	}
	if f := req.GetStatusFilter(); f != "" && !allowedDeckStatuses[f] {
		return nil, status.Error(codes.InvalidArgument, "status_filter must be active, hidden, or deleted")
	}

	page, err := s.deckSvc.AdminListDecks(ctx, limit, offset, req.GetStatusFilter())
	if err != nil {
		return nil, toGRPCError(err)
	}

	decks := make([]*pb.AdminDeckRecord, 0, len(page.Decks))
	for _, d := range page.Decks {
		decks = append(decks, dbDeckToAdminPb(d))
	}
	return &pb.AdminListDecksResponse{Decks: decks, Total: page.Total}, nil
}

func dbDeckToAdminPb(d db.Deck) *pb.AdminDeckRecord {
	return &pb.AdminDeckRecord{
		DeckId:      d.DeckID.String(),
		UserId:      d.UserID.String(),
		Name:        d.Name,
		Description: d.Description.String,
		IsPublic:    d.IsPublic,
		Status:      d.Status,
		CardCount:   d.CardCount,
		CreatedAt:   d.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   d.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
