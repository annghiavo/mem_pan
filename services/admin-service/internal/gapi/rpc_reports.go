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

	reportStatus, err := actionToStatus(req.GetAction())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	var adminNote, resolution *string
	if v := req.GetAdminNote(); v != "" {
		adminNote = &v
	}
	if v := req.GetResolution(); v != "" {
		resolution = &v
	}

	report, err := s.reportSvc.ProcessReport(ctx, service.ProcessReportParams{
		ReportID:   reportID,
		AdminID:    payload.UserID,
		Status:     reportStatus,
		AdminNote:  adminNote,
		Resolution: resolution,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.ProcessReportResponse{Report: dbReportToPb(report)}, nil
}

func actionToStatus(action string) (db.ReportStatus, error) {
	switch action {
	case "resolve":
		return db.ReportStatusResolved, nil
	case "dismiss":
		return db.ReportStatusDismissed, nil
	case "review":
		return db.ReportStatusReviewing, nil
	default:
		return "", status.Errorf(codes.InvalidArgument, "invalid action %q: must be resolve, dismiss, or review", action)
	}
}

func dbReportToPb(r db.Report) *pb.Report {
	var assignedTo, resolvedBy, resolvedAt string
	if r.AssignedTo.Valid {
		assignedTo = r.AssignedTo.UUID.String()
	}
	if r.ResolvedBy.Valid {
		resolvedBy = r.ResolvedBy.UUID.String()
	}
	if r.ResolvedAt.Valid {
		resolvedAt = r.ResolvedAt.Time.String()
	}
	return &pb.Report{
		ReportId:       r.ReportID.String(),
		ReporterId:     r.ReporterID.String(),
		TargetType:     string(r.TargetType),
		TargetId:       r.TargetID.String(),
		ReasonCategory: string(r.ReasonCategory),
		Description:    r.Description.String,
		Status:         string(r.Status),
		AssignedTo:     assignedTo,
		AdminNote:      r.AdminNote.String,
		Resolution:     r.Resolution.String,
		ResolvedBy:     resolvedBy,
		ResolvedAt:     resolvedAt,
		CreatedAt:      r.CreatedAt.String(),
		UpdatedAt:      r.UpdatedAt.String(),
	}
}
