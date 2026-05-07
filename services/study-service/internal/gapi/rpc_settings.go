package gapi

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/study-service/internal/domain"
	"mem_pan/services/study-service/internal/service"
	"mem_pan/services/study-service/pb"
)

func (s *Server) GetDeckSettings(ctx context.Context, req *pb.GetStudyDeckSettingsRequest) (*pb.GetStudyDeckSettingsResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	deckID, err := uuid.Parse(req.DeckId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}

	settings, err := s.settingsSvc.GetDeckSettings(ctx, payload.UserID, deckID)
	if err != nil {
		if errors.Is(err, domain.ErrSettingsNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, toGRPCError(err)
	}
	return &pb.GetStudyDeckSettingsResponse{Settings: dbSettingsToPb(settings)}, nil
}

func (s *Server) UpdateDeckSettings(ctx context.Context, req *pb.UpdateStudyDeckSettingsRequest) (*pb.UpdateStudyDeckSettingsResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	deckID, err := uuid.Parse(req.DeckId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}
	if req.Settings == nil {
		return nil, status.Error(codes.InvalidArgument, "settings is required")
	}

	updated, err := s.settingsSvc.UpsertDeckSettings(ctx, service.UpsertSettingsParams{
		UserID:                       payload.UserID,
		DeckID:                       deckID,
		ShuffleTerms:                 req.Settings.ShuffleTerms,
		TextToSpeech:                 req.Settings.TextToSpeech,
		AnswerWithTerm:               req.Settings.AnswerWithTerm,
		AnswerWithDefinition:         req.Settings.AnswerWithDefinition,
		QuestionTypeFlashcards:       req.Settings.QuestionTypeFlashcards,
		QuestionTypeMultipleChoice:   req.Settings.QuestionTypeMultipleChoice,
		QuestionTypeWritten:          req.Settings.QuestionTypeWritten,
		StrictnessLevel:              req.Settings.StrictnessLevel,
		RequireRetypingCorrectAnswer: req.Settings.RequireRetypingCorrectAnswer,
	})
	if err != nil {
		if errors.Is(err, domain.ErrInvalidStrictness) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, toGRPCError(err)
	}
	return &pb.UpdateStudyDeckSettingsResponse{Settings: dbSettingsToPb(updated)}, nil
}

func (s *Server) CheckAnswer(ctx context.Context, req *pb.CheckAnswerRequest) (*pb.CheckAnswerResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	deckID, err := uuid.Parse(req.DeckId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}

	result, err := s.settingsSvc.CheckAnswer(ctx, service.CheckAnswerParams{
		UserID:        payload.UserID,
		DeckID:        deckID,
		UserAnswer:    req.UserAnswer,
		CorrectAnswer: req.CorrectAnswer,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.CheckAnswerResponse{
		IsCorrect: result.IsCorrect,
		Score:     result.Score,
	}, nil
}
