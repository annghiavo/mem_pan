package gapi

import (
	"encoding/json"
	"errors"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"mem_pan/services/deck-service/internal/authclient"
	"mem_pan/services/deck-service/internal/db"
	"mem_pan/services/deck-service/internal/domain"
	"mem_pan/services/deck-service/internal/publisher"
	"mem_pan/services/deck-service/internal/service"
	"mem_pan/services/deck-service/internal/studyclient"
	"mem_pan/services/deck-service/internal/uploader"
	"mem_pan/services/deck-service/pb"
)

type Server struct {
	pb.UnimplementedDeckServiceServer
	folderSvc   service.FolderService
	deckSvc     service.DeckService
	cardSvc     service.CardService
	authClient  authclient.Client
	studyClient studyclient.Client
	uploader    uploader.ImageUploader
	pub         publisher.EventPublisher
}

func NewServer(
	folderSvc service.FolderService,
	deckSvc service.DeckService,
	cardSvc service.CardService,
	authClient authclient.Client,
	studyClient studyclient.Client,
	imageUploader uploader.ImageUploader,
	pub publisher.EventPublisher,
) *Server {
	return &Server{
		folderSvc:   folderSvc,
		deckSvc:     deckSvc,
		cardSvc:     cardSvc,
		authClient:  authClient,
		studyClient: studyClient,
		uploader:    imageUploader,
		pub:         pub,
	}
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, domain.ErrFolderNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrDeckNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrNoteNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrCardNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrPlusRequired):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrInvalidAccessLevel):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrInvalidPlusStatus):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrReviewNotAllowed):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrCreatorProfileNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrDeckAlreadyInFolder):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrDeckNotInFolder):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrDeckDeleted):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

func dbFolderToPb(f db.Folder) *pb.Folder {
	r := &pb.Folder{
		FolderId:  f.FolderID.String(),
		UserId:    f.UserID.String(),
		Name:      f.Name,
		IsPublic:  f.IsPublic,
		CreatedAt: timestamppb.New(f.CreatedAt),
		UpdatedAt: timestamppb.New(f.UpdatedAt),
	}
	if f.Description.Valid {
		r.Description = f.Description.String
	}
	return r
}

func dbDeckToPb(d db.Deck) *pb.Deck {
	r := &pb.Deck{
		DeckId:       d.DeckID.String(),
		UserId:       d.UserID.String(),
		Name:         d.Name,
		IsPublic:     d.IsPublic,
		Status:       d.Status,
		CardCount:    d.CardCount,
		CreatedAt:    timestamppb.New(d.CreatedAt),
		UpdatedAt:    timestamppb.New(d.UpdatedAt),
		AccessLevel:  string(d.AccessLevel),
		PlusStatus:   string(d.PlusStatus),
		TotalReviews: d.TotalReviews,
	}
	if avg, err := strconv.ParseFloat(d.AvgRating, 64); err == nil {
		r.AvgRating = avg
	}
	if d.Description.Valid {
		r.Description = d.Description.String
	}
	if d.ClonedFrom.Valid {
		r.ClonedFrom = d.ClonedFrom.UUID.String()
	}
	if len(d.Settings) > 0 {
		r.Settings = rawSettingsToPb(d.Settings)
	}
	if d.PlusSubmittedAt.Valid {
		r.PlusSubmittedAt = timestamppb.New(d.PlusSubmittedAt.Time)
	}
	if d.PlusApprovedAt.Valid {
		r.PlusApprovedAt = timestamppb.New(d.PlusApprovedAt.Time)
	}
	return r
}

func dbCreatorProfileToPb(p db.CreatorProfile) *pb.CreatorProfile {
	r := &pb.CreatorProfile{
		UserId:        p.UserID.String(),
		Tier:          string(p.Tier),
		FollowerCount: p.FollowerCount,
		CreatedAt:     timestamppb.New(p.CreatedAt),
		UpdatedAt:     timestamppb.New(p.UpdatedAt),
	}
	if p.DisplayName.Valid {
		r.DisplayName = p.DisplayName.String
	}
	if p.Bio.Valid {
		r.Bio = p.Bio.String
	}
	if p.BankName.Valid {
		r.BankName = p.BankName.String
	}
	if p.BankAccountNumber.Valid {
		r.BankAccountNumber = p.BankAccountNumber.String
	}
	if p.BankAccountName.Valid {
		r.BankAccountName = p.BankAccountName.String
	}
	if p.BankVerifiedAt.Valid {
		r.BankVerifiedAt = timestamppb.New(p.BankVerifiedAt.Time)
	}
	return r
}

func dbDeckReviewToPb(rw db.DeckReview) *pb.DeckReview {
	r := &pb.DeckReview{
		ReviewId:  rw.ReviewID.String(),
		DeckId:    rw.DeckID.String(),
		UserId:    rw.UserID.String(),
		Rating:    int32(rw.Rating),
		Status:    rw.Status,
		CreatedAt: timestamppb.New(rw.CreatedAt),
		UpdatedAt: timestamppb.New(rw.UpdatedAt),
	}
	return r
}

func rawSettingsToPb(raw json.RawMessage) *pb.DeckSettings {
	var s service.DeckSettings
	if err := json.Unmarshal(raw, &s); err != nil {
		return &pb.DeckSettings{}
	}
	return &pb.DeckSettings{
		QuizType:       s.QuizType,
		AnswerSide:     s.AnswerSide,
		StrictTyping:   s.StrictTyping,
		PartialCorrect: s.PartialCorrect,
		NewCardsPerDay: s.NewCardsPerDay,
		ReviewsPerDay:  s.ReviewsPerDay,
	}
}

func dbCardRowToPb(c db.GetCardByIDRow) *pb.Card {
	r := &pb.Card{
		CardId:       c.CardID.String(),
		UserId:       c.UserID.String(),
		DeckId:       c.DeckID.String(),
		NoteId:       c.NoteID.String(),
		Position:     c.Position,
		ContentFront: c.ContentFront,
		ContentBack:  c.ContentBack,
		LangFront:    c.LangFront,
		LangBack:     c.LangBack,
		CreatedAt:    timestamppb.New(c.CreatedAt),
	}
	if c.ImageUrl.Valid {
		r.ImageUrl = c.ImageUrl.String
	}
	return r
}

func dbListCardRowToPb(c db.ListCardsByDeckRow) *pb.Card {
	r := &pb.Card{
		CardId:       c.CardID.String(),
		UserId:       c.UserID.String(),
		DeckId:       c.DeckID.String(),
		NoteId:       c.NoteID.String(),
		Position:     c.Position,
		ContentFront: c.ContentFront,
		ContentBack:  c.ContentBack,
		LangFront:    c.LangFront,
		LangBack:     c.LangBack,
		CreatedAt:    timestamppb.New(c.CreatedAt),
	}
	if c.ImageUrl.Valid {
		r.ImageUrl = c.ImageUrl.String
	}
	return r
}

func nullStrFromProto(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
