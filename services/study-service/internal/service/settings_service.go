package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"mem_pan/services/study-service/internal/db"
	"mem_pan/services/study-service/internal/domain"
	"mem_pan/services/study-service/internal/grading"
	"mem_pan/services/study-service/internal/repository"
)

// defaultDeckStudySettings mirrors the column-level DEFAULTs in
// db/migration/000003_deck_study_settings.up.sql. Returned by GetDeckSettings
// when no row exists yet so the client can render the settings UI on a
// freshly-opened deck instead of hitting a 404.
func defaultDeckStudySettings(userID, deckID uuid.UUID) db.DeckStudySetting {
	return db.DeckStudySetting{
		UserID:                       userID,
		DeckID:                       deckID,
		ShuffleTerms:                 false,
		TextToSpeech:                 false,
		AnswerWithTerm:               true,
		AnswerWithDefinition:         true,
		QuestionTypeFlashcards:       false,
		QuestionTypeMultipleChoice:   true,
		QuestionTypeWritten:          true,
		StrictnessLevel:              grading.StrictnessFlexible,
		RequireRetypingCorrectAnswer: false,
	}
}

type UpsertSettingsParams struct {
	UserID                       uuid.UUID
	DeckID                       uuid.UUID
	ShuffleTerms                 bool
	TextToSpeech                 bool
	AnswerWithTerm               bool
	AnswerWithDefinition         bool
	QuestionTypeFlashcards       bool
	QuestionTypeMultipleChoice   bool
	QuestionTypeWritten          bool
	StrictnessLevel              string
	RequireRetypingCorrectAnswer bool
}

type CheckAnswerParams struct {
	UserID        uuid.UUID
	DeckID        uuid.UUID
	UserAnswer    string
	CorrectAnswer string
}

type CheckAnswerResult struct {
	IsCorrect bool
	Score     float32 // 1.0 = correct, 0.0 = incorrect
}

type SettingsService interface {
	GetDeckSettings(ctx context.Context, userID, deckID uuid.UUID) (db.DeckStudySetting, error)
	UpsertDeckSettings(ctx context.Context, p UpsertSettingsParams) (db.DeckStudySetting, error)
	CheckAnswer(ctx context.Context, p CheckAnswerParams) (CheckAnswerResult, error)
}

type settingsService struct {
	settingsRepo repository.DeckSettingsRepository
}

func NewSettingsService(settingsRepo repository.DeckSettingsRepository) SettingsService {
	return &settingsService{settingsRepo: settingsRepo}
}

func (s *settingsService) GetDeckSettings(ctx context.Context, userID, deckID uuid.UUID) (db.DeckStudySetting, error) {
	settings, err := s.settingsRepo.GetDeckSettings(ctx, userID, deckID)
	if errors.Is(err, domain.ErrSettingsNotFound) {
		return defaultDeckStudySettings(userID, deckID), nil
	}
	return settings, err
}

func (s *settingsService) UpsertDeckSettings(ctx context.Context, p UpsertSettingsParams) (db.DeckStudySetting, error) {
	if p.StrictnessLevel != grading.StrictnessFlexible && p.StrictnessLevel != grading.StrictnessStrict {
		return db.DeckStudySetting{}, domain.ErrInvalidStrictness
	}
	return s.settingsRepo.UpsertDeckSettings(ctx, db.UpsertDeckStudySettingsParams{
		UserID:                       p.UserID,
		DeckID:                       p.DeckID,
		ShuffleTerms:                 p.ShuffleTerms,
		TextToSpeech:                 p.TextToSpeech,
		AnswerWithTerm:               p.AnswerWithTerm,
		AnswerWithDefinition:         p.AnswerWithDefinition,
		QuestionTypeFlashcards:       p.QuestionTypeFlashcards,
		QuestionTypeMultipleChoice:   p.QuestionTypeMultipleChoice,
		QuestionTypeWritten:          p.QuestionTypeWritten,
		StrictnessLevel:              p.StrictnessLevel,
		RequireRetypingCorrectAnswer: p.RequireRetypingCorrectAnswer,
	})
}

func (s *settingsService) CheckAnswer(ctx context.Context, p CheckAnswerParams) (CheckAnswerResult, error) {
	settings, err := s.settingsRepo.GetDeckSettings(ctx, p.UserID, p.DeckID)
	if errors.Is(err, domain.ErrSettingsNotFound) {
		settings = defaultDeckStudySettings(p.UserID, p.DeckID)
	} else if err != nil {
		return CheckAnswerResult{}, err
	}

	ok := grading.CheckAnswer(p.UserAnswer, p.CorrectAnswer, settings.StrictnessLevel)
	score := float32(0)
	if ok {
		score = 1.0
	}
	return CheckAnswerResult{IsCorrect: ok, Score: score}, nil
}
