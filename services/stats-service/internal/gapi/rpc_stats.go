package gapi

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/stats-service/internal/db"
	pb "mem_pan/services/stats-service/pb"
)

func (s *Server) GetMyStats(ctx context.Context, req *pb.GetMyStatsRequest) (*pb.GetMyStatsResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	row, err := s.statsSvc.GetUserStats(ctx, payload.UserID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	// Just-In-Time Streak Evaluation
	if row.LastStudiedDate.Valid {
		tz := req.GetTimezone()
		loc, err := time.LoadLocation(tz)
		if err != nil || tz == "" {
			loc = time.UTC
		}

		now := time.Now().In(loc)
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		ly, lm, ld := row.LastStudiedDate.Time.UTC().Date()
		ty, tm, td := today.Date()
		isToday := (ly == ty && lm == tm && ld == td)

		yy, ym, yd := today.AddDate(0, 0, -1).Date()
		isYesterday := (ly == yy && lm == ym && ld == yd)

		if !isToday && !isYesterday {
			row.CurrentStreak = 0 // Streak has expired
		}
	} else {
		row.CurrentStreak = 0
	}

	return &pb.GetMyStatsResponse{Stats: userStatToPb(row)}, nil
}

func (s *Server) GetHeatmap(ctx context.Context, req *pb.GetHeatmapRequest) (*pb.GetHeatmapResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	from, to, err := parseDateRange(req.GetFromDate(), req.GetToDate())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	rows, err := s.statsSvc.GetHeatmap(ctx, payload.UserID, from, to)
	if err != nil {
		return nil, toGRPCError(err)
	}

	entries := make([]*pb.DailyStatEntry, 0, len(rows))
	for _, r := range rows {
		entries = append(entries, dailyStatToPb(r))
	}

	return &pb.GetHeatmapResponse{Entries: entries}, nil
}

func (s *Server) GetDeckStats(ctx context.Context, req *pb.GetDeckStatsRequest) (*pb.GetDeckStatsResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	deckID, err := uuid.Parse(req.GetDeckId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}

	row, err := s.statsSvc.GetDeckStats(ctx, deckID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	if row.UserID != payload.UserID {
		return nil, status.Error(codes.PermissionDenied, "deck not owned by caller")
	}

	return &pb.GetDeckStatsResponse{Stats: deckStatToPb(row)}, nil
}

func (s *Server) ListMyDeckStats(ctx context.Context, _ *pb.ListMyDeckStatsRequest) (*pb.ListMyDeckStatsResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := s.statsSvc.ListDeckStatsByUser(ctx, payload.UserID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	decks := make([]*pb.DeckStat, 0, len(rows))
	for _, r := range rows {
		decks = append(decks, deckStatToPb(r))
	}

	return &pb.ListMyDeckStatsResponse{Decks: decks}, nil
}

func (s *Server) GetDeckProgress(ctx context.Context, req *pb.GetDeckProgressRequest) (*pb.GetDeckProgressResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	deckID, err := uuid.Parse(req.GetDeckId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}

	// Verify ownership via deck_stats
	ds, err := s.statsSvc.GetDeckStats(ctx, deckID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	if ds.UserID != payload.UserID {
		return nil, status.Error(codes.PermissionDenied, "deck not owned by caller")
	}

	from, to, err := parseDateRange(req.GetFromDate(), req.GetToDate())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	rows, err := s.statsSvc.GetDeckProgress(ctx, deckID, from, to)
	if err != nil {
		return nil, toGRPCError(err)
	}

	entries := make([]*pb.DeckProgressEntry, 0, len(rows))
	for _, r := range rows {
		entries = append(entries, deckProgressToPb(r))
	}

	return &pb.GetDeckProgressResponse{Entries: entries}, nil
}

// ── Converters ────────────────────────────────────────────────────────────────

func userStatToPb(r db.UserStat) *pb.UserStats {
	out := &pb.UserStats{
		UserId:           r.UserID.String(),
		TotalCards:       r.TotalCards,
		TotalReviews:     r.TotalReviews,
		TotalStudyTimeMs: r.TotalStudyTimeMs,
		CurrentStreak:    r.CurrentStreak,
		LongestStreak:    r.LongestStreak,
		TotalCorrect:     r.TotalCorrect,
		TotalIncorrect:   r.TotalIncorrect,
		UpdatedAt:        r.UpdatedAt.Format(time.RFC3339),
	}
	if r.LastStudiedDate.Valid {
		out.LastStudiedDate = r.LastStudiedDate.Time.Format("2006-01-02")
	}
	if r.Username.Valid {
		out.Username = r.Username.String
	}
	if r.AvatarUrl.Valid {
		out.AvatarUrl = r.AvatarUrl.String
	}
	return out
}

func dailyStatToPb(r db.DailyStat) *pb.DailyStatEntry {
	return &pb.DailyStatEntry{
		StudyDate:     r.StudyDate.Format("2006-01-02"),
		ReviewsCount:  r.ReviewsCount,
		NewCardsCount: r.NewCardsCount,
		StudyTimeMs:   r.StudyTimeMs,
		CorrectCount:  r.CorrectCount,
	}
}

func deckStatToPb(r db.DeckStat) *pb.DeckStat {
	out := &pb.DeckStat{
		DeckId:        r.DeckID.String(),
		UserId:        r.UserID.String(),
		TotalCards:    r.TotalCards,
		NewCards:      r.NewCards,
		LearningCards: r.LearningCards,
		ReviewCards:   r.ReviewCards,
		MasteredCards: r.MasteredCards,
		DueToday:      r.DueToday,
		UpdatedAt:     r.UpdatedAt.Format(time.RFC3339),
	}
	if r.DeckName.Valid {
		out.DeckName = r.DeckName.String
	}
	return out
}

func deckProgressToPb(r db.DeckProgressSnapshot) *pb.DeckProgressEntry {
	return &pb.DeckProgressEntry{
		SnapshotDate:  r.SnapshotDate.Format("2006-01-02"),
		NewCount:      r.NewCount,
		LearningCount: r.LearningCount,
		ReviewCount:   r.ReviewCount,
		MasteredCount: r.MasteredCount,
	}
}

func parseDateRange(fromStr, toStr string) (from, to time.Time, err error) {
	const layout = "2006-01-02"

	if fromStr == "" {
		from = time.Now().UTC().AddDate(-1, 0, 0).Truncate(24 * time.Hour)
	} else {
		from, err = time.Parse(layout, fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}

	if toStr == "" {
		to = time.Now().UTC().Truncate(24 * time.Hour)
	} else {
		to, err = time.Parse(layout, toStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}

	return from, to, nil
}
