package gapi

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/deck-service/internal/publisher"
	"mem_pan/services/deck-service/pb"
)

var allowedReportCategories = map[string]struct{}{
	"inappropriate_content": {},
	"copyright_violation":   {},
	"spam":                  {},
	"harassment":            {},
	"misinformation":        {},
	"other":                 {},
}

func (s *Server) ReportDeck(ctx context.Context, req *pb.ReportDeckRequest) (*pb.ReportDeckResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	deckID, err := uuid.Parse(req.GetDeckId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}

	if _, ok := allowedReportCategories[req.GetReasonCategory()]; !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid reason_category")
	}

	// Confirm the deck exists. publicOK=true so any reader (not just the owner) can report it.
	if _, err := s.deckSvc.GetDeck(ctx, deckID, payload.UserID, true); err != nil {
		return nil, toGRPCError(err)
	}

	event := publisher.ReportSubmittedEvent{
		ReporterID:     payload.UserID.String(),
		TargetType:     "deck",
		TargetID:       deckID.String(),
		ReasonCategory: req.GetReasonCategory(),
		Description:    req.GetDescription(),
		SubmittedAt:    time.Now().UTC(),
	}
	if err := s.pub.PublishReportSubmitted(ctx, event); err != nil {
		log.Printf("[deck-service] publish report.submitted failed: %v", err)
		return nil, status.Error(codes.Internal, "failed to submit report")
	}

	return &pb.ReportDeckResponse{Message: "report submitted"}, nil
}
