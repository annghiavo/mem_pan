package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"mem_pan/services/study-service/internal/db"
	"mem_pan/services/study-service/internal/domain"
	"mem_pan/services/study-service/internal/grading"
	"mem_pan/services/study-service/internal/mock"
)

func makeSettings(userID, deckID uuid.UUID, strictness string) db.DeckStudySetting {
	return db.DeckStudySetting{
		UserID:          userID,
		DeckID:          deckID,
		StrictnessLevel: strictness,
	}
}

// ---------------------------------------------------------------------------
// GetDeckSettings
// ---------------------------------------------------------------------------

func TestGetDeckSettings_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	settingsRepo := mock.NewMockDeckSettingsRepository(ctrl)
	ctx := context.Background()
	userID := uuid.New()
	deckID := uuid.New()
	expected := makeSettings(userID, deckID, grading.StrictnessFlexible)

	settingsRepo.EXPECT().GetDeckSettings(ctx, userID, deckID).Return(expected, nil)

	svc := NewSettingsService(settingsRepo)
	result, err := svc.GetDeckSettings(ctx, userID, deckID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.UserID != userID || result.DeckID != deckID {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestGetDeckSettings_NotFound_ReturnsDefaults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	settingsRepo := mock.NewMockDeckSettingsRepository(ctrl)
	ctx := context.Background()
	userID := uuid.New()
	deckID := uuid.New()

	settingsRepo.EXPECT().GetDeckSettings(ctx, userID, deckID).Return(db.DeckStudySetting{}, domain.ErrSettingsNotFound)

	svc := NewSettingsService(settingsRepo)
	got, err := svc.GetDeckSettings(ctx, userID, deckID)

	if err != nil {
		t.Fatalf("expected no error (defaults should be returned), got %v", err)
	}
	if got.UserID != userID || got.DeckID != deckID {
		t.Errorf("expected defaults scoped to caller, got %+v", got)
	}
	if got.StrictnessLevel != grading.StrictnessFlexible {
		t.Errorf("expected default StrictnessLevel=%q, got %q", grading.StrictnessFlexible, got.StrictnessLevel)
	}
	if !got.AnswerWithTerm || !got.AnswerWithDefinition {
		t.Errorf("expected default AnswerWithTerm/Definition=true, got %+v", got)
	}
	if !got.QuestionTypeMultipleChoice || !got.QuestionTypeWritten {
		t.Errorf("expected default QuestionTypeMultipleChoice/Written=true, got %+v", got)
	}
}

func TestGetDeckSettings_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	settingsRepo := mock.NewMockDeckSettingsRepository(ctrl)
	ctx := context.Background()
	userID := uuid.New()
	deckID := uuid.New()
	boom := errors.New("db down")

	settingsRepo.EXPECT().GetDeckSettings(ctx, userID, deckID).Return(db.DeckStudySetting{}, boom)

	svc := NewSettingsService(settingsRepo)
	_, err := svc.GetDeckSettings(ctx, userID, deckID)

	if !errors.Is(err, boom) {
		t.Errorf("expected underlying repo error to surface, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// UpsertDeckSettings
// ---------------------------------------------------------------------------

func TestUpsertDeckSettings_Success_Flexible(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	settingsRepo := mock.NewMockDeckSettingsRepository(ctrl)
	ctx := context.Background()
	userID := uuid.New()
	deckID := uuid.New()
	saved := makeSettings(userID, deckID, grading.StrictnessFlexible)

	settingsRepo.EXPECT().UpsertDeckSettings(ctx, db.UpsertDeckStudySettingsParams{
		UserID:          userID,
		DeckID:          deckID,
		StrictnessLevel: grading.StrictnessFlexible,
	}).Return(saved, nil)

	svc := NewSettingsService(settingsRepo)
	result, err := svc.UpsertDeckSettings(ctx, UpsertSettingsParams{
		UserID:          userID,
		DeckID:          deckID,
		StrictnessLevel: grading.StrictnessFlexible,
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.StrictnessLevel != grading.StrictnessFlexible {
		t.Errorf("expected flexible, got %s", result.StrictnessLevel)
	}
}

func TestUpsertDeckSettings_Success_Strict(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	settingsRepo := mock.NewMockDeckSettingsRepository(ctrl)
	ctx := context.Background()
	userID := uuid.New()
	deckID := uuid.New()
	saved := makeSettings(userID, deckID, grading.StrictnessStrict)

	settingsRepo.EXPECT().UpsertDeckSettings(ctx, gomock.Any()).Return(saved, nil)

	svc := NewSettingsService(settingsRepo)
	result, err := svc.UpsertDeckSettings(ctx, UpsertSettingsParams{
		UserID:          userID,
		DeckID:          deckID,
		StrictnessLevel: grading.StrictnessStrict,
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.StrictnessLevel != grading.StrictnessStrict {
		t.Errorf("expected strict, got %s", result.StrictnessLevel)
	}
}

func TestUpsertDeckSettings_InvalidStrictness(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	settingsRepo := mock.NewMockDeckSettingsRepository(ctrl)
	ctx := context.Background()

	svc := NewSettingsService(settingsRepo)
	_, err := svc.UpsertDeckSettings(ctx, UpsertSettingsParams{
		UserID:          uuid.New(),
		DeckID:          uuid.New(),
		StrictnessLevel: "medium", // invalid value
	})

	if !errors.Is(err, domain.ErrInvalidStrictness) {
		t.Errorf("expected ErrInvalidStrictness, got %v", err)
	}
}

func TestUpsertDeckSettings_EmptyStrictness(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	settingsRepo := mock.NewMockDeckSettingsRepository(ctrl)
	ctx := context.Background()

	svc := NewSettingsService(settingsRepo)
	_, err := svc.UpsertDeckSettings(ctx, UpsertSettingsParams{
		UserID:          uuid.New(),
		DeckID:          uuid.New(),
		StrictnessLevel: "",
	})

	if !errors.Is(err, domain.ErrInvalidStrictness) {
		t.Errorf("expected ErrInvalidStrictness for empty string, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// CheckAnswer
// ---------------------------------------------------------------------------

func TestCheckAnswer_SettingsNotFound_FallsBackToFlexible(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	settingsRepo := mock.NewMockDeckSettingsRepository(ctrl)
	ctx := context.Background()
	userID := uuid.New()
	deckID := uuid.New()

	// Repository returns not-found; service must fall back to flexible grading.
	settingsRepo.EXPECT().GetDeckSettings(ctx, userID, deckID).Return(db.DeckStudySetting{}, domain.ErrSettingsNotFound)

	svc := NewSettingsService(settingsRepo)
	// "Hello!" flexible-matches "hello" (punctuation ignored in flexible).
	result, err := svc.CheckAnswer(ctx, CheckAnswerParams{
		UserID:        userID,
		DeckID:        deckID,
		UserAnswer:    "Hello!",
		CorrectAnswer: "hello",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.IsCorrect {
		t.Error("expected flexible fallback to accept 'Hello!' for 'hello'")
	}
	if result.Score != 1.0 {
		t.Errorf("expected score 1.0, got %f", result.Score)
	}
}

func TestCheckAnswer_StrictSettings_Correct(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	settingsRepo := mock.NewMockDeckSettingsRepository(ctrl)
	ctx := context.Background()
	userID := uuid.New()
	deckID := uuid.New()
	settings := makeSettings(userID, deckID, grading.StrictnessStrict)

	settingsRepo.EXPECT().GetDeckSettings(ctx, userID, deckID).Return(settings, nil)

	svc := NewSettingsService(settingsRepo)
	result, err := svc.CheckAnswer(ctx, CheckAnswerParams{
		UserID:        userID,
		DeckID:        deckID,
		UserAnswer:    "Tokyo",
		CorrectAnswer: "Tokyo",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.IsCorrect {
		t.Error("expected exact match to pass in strict mode")
	}
}

func TestCheckAnswer_StrictSettings_Incorrect(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	settingsRepo := mock.NewMockDeckSettingsRepository(ctrl)
	ctx := context.Background()
	userID := uuid.New()
	deckID := uuid.New()
	settings := makeSettings(userID, deckID, grading.StrictnessStrict)

	settingsRepo.EXPECT().GetDeckSettings(ctx, userID, deckID).Return(settings, nil)

	svc := NewSettingsService(settingsRepo)
	// Strict mode does not ignore punctuation.
	result, err := svc.CheckAnswer(ctx, CheckAnswerParams{
		UserID:        userID,
		DeckID:        deckID,
		UserAnswer:    "Tokyo!",
		CorrectAnswer: "Tokyo",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.IsCorrect {
		t.Error("expected punctuation mismatch to fail in strict mode")
	}
	if result.Score != 0.0 {
		t.Errorf("expected score 0.0 for incorrect answer, got %f", result.Score)
	}
}

func TestCheckAnswer_FlexibleSettings_Typo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	settingsRepo := mock.NewMockDeckSettingsRepository(ctrl)
	ctx := context.Background()
	userID := uuid.New()
	deckID := uuid.New()
	settings := makeSettings(userID, deckID, grading.StrictnessFlexible)

	settingsRepo.EXPECT().GetDeckSettings(ctx, userID, deckID).Return(settings, nil)

	svc := NewSettingsService(settingsRepo)
	// "Tokyp" is one edit away from "Tokyo" (len 5 → threshold 1).
	result, err := svc.CheckAnswer(ctx, CheckAnswerParams{
		UserID:        userID,
		DeckID:        deckID,
		UserAnswer:    "Tokyp",
		CorrectAnswer: "Tokyo",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.IsCorrect {
		t.Error("expected one-edit typo to pass in flexible mode for 5-char word")
	}
}
