package gapi

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/deck-service/internal/service"
	"mem_pan/services/deck-service/pb"
)

func (s *Server) GetCard(ctx context.Context, req *pb.GetCardRequest) (*pb.GetCardResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	cardID, err := uuid.Parse(req.CardId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid card_id")
	}

	card, err := s.cardSvc.GetCard(ctx, cardID, payload.UserID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.GetCardResponse{Card: dbCardRowToPb(card)}, nil
}

func (s *Server) UpdateCard(ctx context.Context, req *pb.UpdateCardRequest) (*pb.UpdateCardResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	cardID, err := uuid.Parse(req.CardId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid card_id")
	}

	card, err := s.cardSvc.UpdateCard(ctx, service.UpdateCardParams{
		CardID:       cardID,
		UserID:       payload.UserID,
		ContentFront: nullStrFromProto(req.ContentFront),
		ContentBack:  nullStrFromProto(req.ContentBack),
		ImageURL:     nullStrFromProto(req.ImageUrl),
		LangFront:    nullStrFromProto(req.LangFront),
		LangBack:     nullStrFromProto(req.LangBack),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.UpdateCardResponse{Card: dbCardRowToPb(card)}, nil
}

func (s *Server) DeleteCard(ctx context.Context, req *pb.DeleteCardRequest) (*pb.DeleteCardResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	cardID, err := uuid.Parse(req.CardId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid card_id")
	}

	if err := s.cardSvc.DeleteCard(ctx, cardID, payload.UserID); err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.DeleteCardResponse{Success: true}, nil
}
