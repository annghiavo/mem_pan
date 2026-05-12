package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"mem_pan/services/stats-service/internal/db"
	"mem_pan/services/stats-service/internal/repository"
)

type StatsService interface {
	GetUserStats(ctx context.Context, userID uuid.UUID) (db.UserStat, error)
	GetHeatmap(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]db.DailyStat, error)
	GetDeckStats(ctx context.Context, deckID uuid.UUID) (db.DeckStat, error)
	ListDeckStatsByUser(ctx context.Context, userID uuid.UUID) ([]db.DeckStat, error)
	GetDeckProgress(ctx context.Context, deckID uuid.UUID, from, to time.Time) ([]db.DeckProgressSnapshot, error)
}

type statsService struct {
	repo repository.StatsRepository
}

func New(repo repository.StatsRepository) StatsService {
	return &statsService{repo: repo}
}

func (s *statsService) GetUserStats(ctx context.Context, userID uuid.UUID) (db.UserStat, error) {
	return s.repo.GetUserStats(ctx, userID)
}

func (s *statsService) GetHeatmap(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]db.DailyStat, error) {
	return s.repo.ListDailyStats(ctx, userID, from, to)
}

func (s *statsService) GetDeckStats(ctx context.Context, deckID uuid.UUID) (db.DeckStat, error) {
	return s.repo.GetDeckStats(ctx, deckID)
}

func (s *statsService) ListDeckStatsByUser(ctx context.Context, userID uuid.UUID) ([]db.DeckStat, error) {
	return s.repo.ListDeckStatsByUser(ctx, userID)
}

func (s *statsService) GetDeckProgress(ctx context.Context, deckID uuid.UUID, from, to time.Time) ([]db.DeckProgressSnapshot, error) {
	return s.repo.ListDeckProgressSnapshots(ctx, deckID, from, to)
}
