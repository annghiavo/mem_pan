package gapi

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"mem_pan/services/study-service/internal/authclient"
	"mem_pan/services/study-service/internal/db"
	"mem_pan/services/study-service/internal/domain"
	"mem_pan/services/study-service/internal/service"
	"mem_pan/services/study-service/pb"
)

type Server struct {
	pb.UnimplementedStudyServiceServer
	studySvc   service.StudyService
	authClient authclient.Client
}

func NewServer(studySvc service.StudyService, authClient authclient.Client) *Server {
	return &Server{
		studySvc:   studySvc,
		authClient: authClient,
	}
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, domain.ErrSessionNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrSessionFinished):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrCardNotInSession):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrCardAlreadyReviewed):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidRating):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrDeckEmpty):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

func sessionResultToPb(r *service.SessionResult) *pb.StudySession {
	s := r.Session
	out := &pb.StudySession{
		SessionId:      s.SessionID.String(),
		UserId:         s.UserID.String(),
		DeckId:         s.DeckID.String(),
		Status:         s.Status,
		TotalCards:     s.TotalCards,
		CompletedCards: s.CompletedCards,
		StartedAt:      timestamppb.New(s.StartedAt),
	}
	if s.FinishedAt.Valid {
		out.FinishedAt = timestamppb.New(s.FinishedAt.Time)
	}
	cards := make([]*pb.SessionCardItem, len(r.Cards))
	for i, sc := range r.Cards {
		cards[i] = sessionCardToPb(sc)
	}
	out.Cards = cards
	return out
}

func sessionCardToPb(sc db.SessionCard) *pb.SessionCardItem {
	item := &pb.SessionCardItem{
		Position:   sc.Position,
		CardId:     sc.CardID.String(),
		UserCardId: sc.UserCardID.String(),
		Reviewed:   sc.ReviewedAt.Valid,
	}
	if sc.Rating.Valid {
		item.Rating = sc.Rating.Int32
	}
	return item
}

func userCardToPb(uc db.UserCard) *pb.UserCardState {
	out := &pb.UserCardState{
		UserCardId:    uc.UserCardID.String(),
		UserId:        uc.UserID.String(),
		CardId:        uc.CardID.String(),
		DeckId:        uc.DeckID.String(),
		State:         uc.State,
		Stability:     uc.Stability,
		Difficulty:    uc.Difficulty,
		Reps:          uc.Reps,
		Lapses:        uc.Lapses,
		ScheduledDays: uc.ScheduledDays,
		NextReviewDate: timestamppb.New(uc.NextReviewDate),
	}
	if uc.LastReviewDate.Valid {
		out.LastReviewDate = timestamppb.New(uc.LastReviewDate.Time)
	}
	return out
}
