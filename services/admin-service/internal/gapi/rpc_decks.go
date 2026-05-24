package gapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/admin-service/internal/db"
	"mem_pan/services/admin-service/internal/deckclient"
	pb "mem_pan/services/admin-service/pb/proto"
)

var allowedDeckStatuses = map[string]bool{
	"active":  true,
	"hidden":  true,
	"deleted": true,
}

func (s *Server) ListDecks(ctx context.Context, req *pb.ListDecksRequest) (*pb.ListDecksResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if !isModerator(payload.Role) {
		return nil, status.Error(codes.PermissionDenied, "moderator access required")
	}

	pageSize := req.GetPageSize()
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	offset := int32(0)
	if token := req.GetPageToken(); token != "" {
		v, err := strconv.Atoi(token)
		if err != nil || v < 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid page_token")
		}
		offset = int32(v)
	}

	if f := req.GetStatusFilter(); f != "" && !allowedDeckStatuses[f] {
		return nil, status.Error(codes.InvalidArgument, "status_filter must be active, hidden, or deleted")
	}

	result, err := s.deckClient.ListDecks(ctx, pageSize, offset, req.GetStatusFilter())
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list decks")
	}

	decks := make([]*pb.AdminDeck, 0, len(result.Decks))
	for _, d := range result.Decks {
		decks = append(decks, adminDeckToPb(d))
	}

	nextToken := ""
	if int64(offset)+int64(len(result.Decks)) < result.Total {
		nextToken = strconv.Itoa(int(offset) + len(result.Decks))
	}

	return &pb.ListDecksResponse{
		Decks:         decks,
		NextPageToken: nextToken,
		Total:         result.Total,
	}, nil
}

func adminDeckToPb(d deckclient.AdminDeck) *pb.AdminDeck {
	return &pb.AdminDeck{
		DeckId:      d.DeckID,
		UserId:      d.UserID,
		Name:        d.Name,
		Description: d.Description,
		IsPublic:    d.IsPublic,
		Status:      d.Status,
		CardCount:   d.CardCount,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

func (s *Server) UpdateDeckStatus(ctx context.Context, req *pb.UpdateDeckStatusRequest) (*pb.UpdateDeckStatusResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if !isModerator(payload.Role) {
		return nil, status.Error(codes.PermissionDenied, "moderator access required")
	}

	deckID, err := uuid.Parse(req.GetDeckId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}
	if !allowedDeckStatuses[req.GetStatus()] {
		return nil, status.Error(codes.InvalidArgument, "status must be active, hidden, or deleted")
	}

	newStatus, _, err := s.deckClient.UpdateDeckStatus(ctx, deckID.String(), req.GetStatus())
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to update deck status")
	}

	logMeta, _ := json.Marshal(map[string]any{"new_status": newStatus})
	_, _ = s.reportRepo.CreateModerationLog(ctx, db.CreateModerationLogParams{
		AdminID:    payload.UserID,
		Action:     "update_deck_status",
		TargetType: "deck",
		TargetID:   deckID,
		Reason:     sql.NullString{String: req.GetReason(), Valid: req.GetReason() != ""},
		Metadata:   sql.NullString{String: string(logMeta), Valid: true},
	})

	return &pb.UpdateDeckStatusResponse{
		DeckId: deckID.String(),
		Status: newStatus,
	}, nil
}
