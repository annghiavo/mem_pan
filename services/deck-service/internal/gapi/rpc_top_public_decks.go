package gapi

import (
	"context"

	"github.com/google/uuid"

	"mem_pan/services/deck-service/pb"
)

const (
	defaultTopPublicDecksLimit int32 = 10
	maxTopPublicDecksLimit     int32 = 50
	// Over-fetch factor: study-service ranks decks regardless of visibility, so
	// we ask for more candidates than requested and then drop the non-public
	// ones before trimming to limit.
	topPublicOverFetch int32 = 3
)

// ListTopPublicDecks is a public (no-auth) endpoint that returns public decks
// ranked by distinct learners within the trending window. study-service owns
// the ranking; deck-service filters it down to public, active decks and
// hydrates the metadata. If study-service is unreachable it returns an empty
// list rather than an error so the page degrades gracefully.
func (s *Server) ListTopPublicDecks(ctx context.Context, req *pb.ListTopPublicDecksRequest) (*pb.ListTopPublicDecksResponse, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = defaultTopPublicDecksLimit
	}
	if limit > maxTopPublicDecksLimit {
		limit = maxTopPublicDecksLimit
	}

	if s.studyClient == nil {
		return &pb.ListTopPublicDecksResponse{}, nil
	}

	ranked, err := s.studyClient.TopDecksByLearners(ctx, req.WindowDays, limit*topPublicOverFetch)
	if err != nil {
		// Best-effort: leaderboard is non-critical, don't fail the request.
		return &pb.ListTopPublicDecksResponse{}, nil
	}
	if len(ranked) == 0 {
		return &pb.ListTopPublicDecksResponse{}, nil
	}

	ids := make([]uuid.UUID, len(ranked))
	learnersByID := make(map[uuid.UUID]int64, len(ranked))
	for i, r := range ranked {
		ids[i] = r.DeckID
		learnersByID[r.DeckID] = r.Learners
	}

	publicDecks, err := s.deckSvc.ListPublicDecksByIDs(ctx, ids)
	if err != nil {
		return nil, toGRPCError(err)
	}

	// ListPublicDecksByIDs preserves the input (ranking) order. Trim to limit.
	resp := &pb.ListTopPublicDecksResponse{Decks: make([]*pb.RankedPublicDeck, 0, limit)}
	for _, deck := range publicDecks {
		resp.Decks = append(resp.Decks, &pb.RankedPublicDeck{
			Deck:         dbDeckToPb(deck),
			LearnerCount: learnersByID[deck.DeckID],
		})
		if int32(len(resp.Decks)) >= limit {
			break
		}
	}
	return resp, nil
}
