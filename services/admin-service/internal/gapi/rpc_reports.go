package gapi

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/admin-service/internal/db"
	"mem_pan/services/admin-service/internal/service"
	pb "mem_pan/services/admin-service/pb/proto"
)

func (s *Server) ListReports(ctx context.Context, req *pb.ListReportsRequest) (*pb.ListReportsResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if !isModerator(payload.Role) {
		return nil, status.Error(codes.PermissionDenied, "moderator access required")
	}

	limit := req.GetPageSize()
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var statusFilter db.NullReportStatus
	if req.GetStatusFilter() != "" {
		statusFilter = db.NullReportStatus{
			ReportStatus: db.ReportStatus(req.GetStatusFilter()),
			Valid:        true,
		}
	}

	page, err := s.reportSvc.ListReports(ctx, service.ListReportsParams{
		Limit:        limit,
		Offset:       0,
		StatusFilter: statusFilter,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	pbReports := make([]*pb.Report, 0, len(page.Reports))
	for _, r := range page.Reports {
		pbReports = append(pbReports, dbReportToPb(r))
	}

	return &pb.ListReportsResponse{Reports: pbReports}, nil
}

var allowedReportActions = map[string]service.ProcessAction{
	"ban_user":    service.ActionBanUser,
	"hide_deck":   service.ActionHideDeck,
	"delete_deck": service.ActionDeleteDeck,
	"dismiss":     service.ActionDismiss,
}

func (s *Server) ProcessReport(ctx context.Context, req *pb.ProcessReportRequest) (*pb.ProcessReportResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if !isModerator(payload.Role) {
		return nil, status.Error(codes.PermissionDenied, "moderator access required")
	}

	reportID, err := uuid.Parse(req.GetReportId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid report_id")
	}

	action, ok := allowedReportActions[req.GetAction()]
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "action must be ban_user, hide_deck, delete_deck, or dismiss")
	}

	var adminNote *string
	if v := req.GetAdminNote(); v != "" {
		adminNote = &v
	}

	result, err := s.reportSvc.ProcessReport(ctx, service.ProcessReportParams{
		ReportID:  reportID,
		AdminID:   payload.UserID,
		Action:    action,
		AdminNote: adminNote,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.ProcessReportResponse{
		Report:            dbReportToPb(result.Report),
		AffectedReports:   result.AffectedReports,
		NotifiedReporters: result.NotifiedReporters,
	}, nil
}

func dbReportToPb(r db.Report) *pb.Report {
	var resolvedBy, resolvedAt string
	if r.ResolvedBy.Valid {
		resolvedBy = r.ResolvedBy.UUID.String()
	}
	if r.ResolvedAt.Valid {
		resolvedAt = r.ResolvedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	return &pb.Report{
		ReportId:       r.ReportID.String(),
		ReporterId:     r.ReporterID.String(),
		TargetType:     string(r.TargetType),
		TargetId:       r.TargetID.String(),
		ReasonCategory: string(r.ReasonCategory),
		Description:    r.Description.String,
		Status:         string(r.Status),
		AdminNote:      r.AdminNote.String,
		Resolution:     r.Resolution.String,
		ResolvedBy:     resolvedBy,
		ResolvedAt:     resolvedAt,
		CreatedAt:      r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      r.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
