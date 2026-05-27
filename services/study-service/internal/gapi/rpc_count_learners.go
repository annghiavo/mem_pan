package gapi

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "mem_pan/services/study-service/pb"
)

// CountDeckLearners is an internal RPC consumed by deck-service to show a
// learner count on the deck detail page. Like CountDueForUser it does not
// require the user-auth interceptor — authentication is enforced at the
// network layer (private service-to-service traffic).
func (s *Server) CountDeckLearners(ctx context.Context, req *pb.CountDeckLearnersRequest) (*pb.CountDeckLearnersResponse, error) {
	deckID, err := uuid.Parse(req.DeckId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}
	count, err := s.studySvc.CountDeckLearners(ctx, deckID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.CountDeckLearnersResponse{Count: count}, nil
}
