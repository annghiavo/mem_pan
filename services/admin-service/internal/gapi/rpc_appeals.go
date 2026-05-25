package gapi

import (
	"context"
	"strconv"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/admin-service/internal/db"
	"mem_pan/services/admin-service/internal/service"
	pb "mem_pan/services/admin-service/pb/proto"
)

var allowedAppealStatuses = map[string]db.AppealStatus{
	"pending":   db.AppealStatusPending,
	"submitted": db.AppealStatusSubmitted,
	"approved":  db.AppealStatusApproved,
	"rejected":  db.AppealStatusRejected,
}

// GetAppealByToken is the PUBLIC endpoint the deck owner hits from their email
// link. No auth — possession of the unguessable token authorizes the call.
func (s *Server) GetAppealByToken(ctx context.Context, req *pb.GetAppealByTokenRequest) (*pb.Appeal, error) {
	if req.GetToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}
	appeal, err := s.appealSvc.GetAppealByToken(ctx, req.GetToken())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return appealToPb(appeal), nil
}

// SubmitAppeal is the PUBLIC endpoint the deck owner uses to file their case.
// Idempotency: once submitted/approved/rejected, further submits return
// FailedPrecondition (the UI just shows the current status).
func (s *Server) SubmitAppeal(ctx context.Context, req *pb.SubmitAppealRequest) (*pb.Appeal, error) {
	if req.GetToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}
	appeal, err := s.appealSvc.SubmitAppeal(ctx, service.SubmitAppealParams{
		Token:   req.GetToken(),
		Message: req.GetMessage(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return appealToPb(appeal), nil
}

// ListAppeals (admin/moderator) returns deck appeals paginated, optionally
// filtered by status.
func (s *Server) ListAppeals(ctx context.Context, req *pb.ListAppealsRequest) (*pb.ListAppealsResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if !isModerator(payload.Role) {
		return nil, status.Error(codes.PermissionDenied, "moderator access required")
	}

	pageSize := req.GetPageSize()
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	offset := int32(0)
	if token := req.GetPageToken(); token != "" {
		v, err := strconv.Atoi(token)
		if err != nil || v < 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid page_token")
		}
		offset = int32(v)
	}

	var statusFilter db.NullAppealStatus
	if f := req.GetStatusFilter(); f != "" {
		s, ok := allowedAppealStatuses[f]
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "status_filter must be pending, submitted, approved, or rejected")
		}
		statusFilter = db.NullAppealStatus{AppealStatus: s, Valid: true}
	}

	page, err := s.appealSvc.ListAppeals(ctx, service.ListAppealsParams{
		Limit:        pageSize,
		Offset:       offset,
		StatusFilter: statusFilter,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	pbAppeals := make([]*pb.Appeal, 0, len(page.Appeals))
	for _, a := range page.Appeals {
		pbAppeals = append(pbAppeals, appealToPb(a))
	}

	nextToken := ""
	if int64(offset)+int64(len(page.Appeals)) < page.Total {
		nextToken = strconv.Itoa(int(offset) + len(page.Appeals))
	}

	return &pb.ListAppealsResponse{
		Appeals:       pbAppeals,
		NextPageToken: nextToken,
		Total:         page.Total,
	}, nil
}

// DecideAppeal (admin/moderator) approves or rejects a submitted appeal.
// On approve, the deck is restored to "active" before the appeal row is updated.
func (s *Server) DecideAppeal(ctx context.Context, req *pb.DecideAppealRequest) (*pb.Appeal, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if !isModerator(payload.Role) {
		return nil, status.Error(codes.PermissionDenied, "moderator access required")
	}

	appealID, err := uuid.Parse(req.GetAppealId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid appeal_id")
	}

	var decision service.AppealDecision
	switch req.GetDecision() {
	case "approve":
		decision = service.AppealDecisionApprove
	case "reject":
		decision = service.AppealDecisionReject
	default:
		return nil, status.Error(codes.InvalidArgument, "decision must be approve or reject")
	}

	appeal, err := s.appealSvc.DecideAppeal(ctx, service.DecideAppealParams{
		AppealID: appealID,
		AdminID:  payload.UserID,
		Decision: decision,
		Note:     req.GetNote(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return appealToPb(appeal), nil
}

func appealToPb(a db.DeckAppeal) *pb.Appeal {
	var (
		userMessage  string
		submittedAt  string
		decidedBy    string
		decisionNote string
		decidedAt    string
	)
	if a.UserMessage.Valid {
		userMessage = a.UserMessage.String
	}
	if a.SubmittedAt.Valid {
		submittedAt = a.SubmittedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	if a.DecidedBy.Valid {
		decidedBy = a.DecidedBy.UUID.String()
	}
	if a.DecisionNote.Valid {
		decisionNote = a.DecisionNote.String
	}
	if a.DecidedAt.Valid {
		decidedAt = a.DecidedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	return &pb.Appeal{
		AppealId:         a.AppealID.String(),
		DeckId:           a.DeckID.String(),
		UserId:           a.UserID.String(),
		DeckName:         a.DeckName,
		ModerationReason: a.ModerationReason,
		Status:           string(a.Status),
		UserMessage:      userMessage,
		SubmittedAt:      submittedAt,
		DecidedBy:        decidedBy,
		DecisionNote:     decisionNote,
		DecidedAt:        decidedAt,
		CreatedAt:        a.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:        a.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
