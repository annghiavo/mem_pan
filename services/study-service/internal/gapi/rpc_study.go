package gapi

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/study-service/internal/service"
	"mem_pan/services/study-service/pb"
)

func (s *Server) StartSession(ctx context.Context, req *pb.StartSessionRequest) (*pb.StartSessionResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	token, err := extractBearerToken(ctx)
	if err != nil {
		return nil, err
	}

	if req.DeckId == "" {
		return nil, status.Error(codes.InvalidArgument, "deck_id is required")
	}
	deckID, err := uuid.Parse(req.DeckId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}

	result, err := s.studySvc.StartSession(ctx, service.StartSessionParams{
		UserID:        payload.UserID,
		DeckID:        deckID,
		NewCardsLimit: req.NewCardsLimit,
		ReviewLimit:   req.ReviewLimit,
		AccessToken:   token,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.StartSessionResponse{Session: sessionResultToPb(result)}, nil
}

func (s *Server) GetSession(ctx context.Context, req *pb.GetSessionRequest) (*pb.GetSessionResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	sessionID, err := uuid.Parse(req.SessionId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid session_id")
	}

	result, err := s.studySvc.GetSession(ctx, sessionID, payload.UserID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.GetSessionResponse{Session: sessionResultToPb(result)}, nil
}

func (s *Server) ReviewCard(ctx context.Context, req *pb.ReviewCardRequest) (*pb.ReviewCardResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	sessionID, err := uuid.Parse(req.SessionId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid session_id")
	}
	cardID, err := uuid.Parse(req.CardId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid card_id")
	}
	if req.Rating < 1 || req.Rating > 4 {
		return nil, status.Error(codes.InvalidArgument, "rating must be between 1 and 4")
	}

	updatedUC, err := s.studySvc.ReviewCard(ctx, service.ReviewCardParams{
		SessionID:  sessionID,
		UserID:     payload.UserID,
		CardID:     cardID,
		Rating:     req.Rating,
		DurationMS: req.DurationMs,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.ReviewCardResponse{UserCard: userCardToPb(updatedUC)}, nil
}

func (s *Server) FinishSession(ctx context.Context, req *pb.FinishSessionRequest) (*pb.FinishSessionResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	sessionID, err := uuid.Parse(req.SessionId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid session_id")
	}

	result, err := s.studySvc.FinishSession(ctx, sessionID, payload.UserID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.FinishSessionResponse{Session: sessionResultToPb(result)}, nil
}

func (s *Server) GetDueCards(ctx context.Context, req *pb.GetDueCardsRequest) (*pb.GetDueCardsResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	var deckID *uuid.UUID
	if req.DeckId != "" {
		id, err := uuid.Parse(req.DeckId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
		}
		deckID = &id
	}

	cards, err := s.studySvc.GetDueCards(ctx, payload.UserID, deckID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	out := make([]*pb.DueCard, len(cards))
	for i, uc := range cards {
		out[i] = &pb.DueCard{
			CardId:          uc.CardID.String(),
			DeckId:          uc.DeckID.String(),
			UserCardId:      uc.UserCardID.String(),
			State:           uc.State,
			NextReviewDate:  userCardToPb(uc).NextReviewDate,
		}
	}
	return &pb.GetDueCardsResponse{Cards: out, Total: int32(len(out))}, nil
}
