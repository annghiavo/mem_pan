package gapi

import (
	"context"

	pb "mem_pan/services/study-service/pb"
)

// TopDecksByLearners is an internal RPC consumed by deck-service to build the
// "top public decks" leaderboard. Like the other internal RPCs it does not
// require the user-auth interceptor — it is private service-to-service traffic
// and returns no per-user data.
func (s *Server) TopDecksByLearners(ctx context.Context, req *pb.TopDecksByLearnersRequest) (*pb.TopDecksByLearnersResponse, error) {
	rows, err := s.studySvc.TopDecksByLearners(ctx, req.WindowDays, req.Limit)
	if err != nil {
		return nil, toGRPCError(err)
	}
	decks := make([]*pb.DeckLearners, len(rows))
	for i, r := range rows {
		decks[i] = &pb.DeckLearners{
			DeckId:   r.DeckID.String(),
			Learners: r.Learners,
		}
	}
	return &pb.TopDecksByLearnersResponse{Decks: decks}, nil
}
