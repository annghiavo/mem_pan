package gapi

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/deck-service/internal/service"
	"mem_pan/services/deck-service/pb"
)

func (s *Server) UpsertCreatorProfile(ctx context.Context, req *pb.UpsertCreatorProfileRequest) (*pb.UpsertCreatorProfileResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	profile, err := s.deckSvc.UpsertCreatorProfile(ctx, service.CreatorProfileParams{
		UserID:            payload.UserID,
		DisplayName:       nullStrFromProto(req.DisplayName),
		Bio:               nullStrFromProto(req.Bio),
		BankName:          nullStrFromProto(req.BankName),
		BankAccountNumber: nullStrFromProto(req.BankAccountNumber),
		BankAccountName:   nullStrFromProto(req.BankAccountName),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.UpsertCreatorProfileResponse{Profile: dbCreatorProfileToPb(profile)}, nil
}

func (s *Server) GetCreatorProfile(ctx context.Context, req *pb.GetCreatorProfileRequest) (*pb.GetCreatorProfileResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}
	profile, err := s.deckSvc.GetCreatorProfile(ctx, userID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.GetCreatorProfileResponse{Profile: dbCreatorProfileToPb(profile)}, nil
}

func (s *Server) FollowCreator(ctx context.Context, req *pb.FollowCreatorRequest) (*pb.FollowCreatorResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	creatorID, err := uuid.Parse(req.CreatorId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid creator_id")
	}
	if err := s.deckSvc.FollowCreator(ctx, creatorID, payload.UserID); err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.FollowCreatorResponse{Success: true}, nil
}

func (s *Server) UpsertDeckReview(ctx context.Context, req *pb.UpsertDeckReviewRequest) (*pb.UpsertDeckReviewResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	deckID, err := uuid.Parse(req.DeckId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}
	review, deck, err := s.deckSvc.UpsertDeckReview(ctx, deckID, payload.UserID, req.Rating, payload.IsPlus)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.UpsertDeckReviewResponse{Review: dbDeckReviewToPb(review), Deck: dbDeckToPb(deck)}, nil
}

func (s *Server) ListDeckReviews(ctx context.Context, req *pb.ListDeckReviewsRequest) (*pb.ListDeckReviewsResponse, error) {
	deckID, err := uuid.Parse(req.DeckId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (req.Page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	reviews, err := s.deckSvc.ListDeckReviews(ctx, deckID, pageSize, offset)
	if err != nil {
		return nil, toGRPCError(err)
	}
	out := make([]*pb.DeckReview, len(reviews))
	for i, rw := range reviews {
		out[i] = dbDeckReviewToPb(rw)
	}
	return &pb.ListDeckReviewsResponse{Reviews: out}, nil
}
