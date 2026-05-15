package gapi

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/auth-service/internal/publisher"
	"mem_pan/services/auth-service/pb"
)

var allowedReportCategories = map[string]struct{}{
	"inappropriate_content": {},
	"copyright_violation":   {},
	"spam":                  {},
	"harassment":            {},
	"misinformation":        {},
	"other":                 {},
}

func (s *Server) ReportUser(ctx context.Context, req *pb.ReportUserRequest) (*pb.ReportUserResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	targetID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}

	if targetID == payload.UserID {
		return nil, status.Error(codes.InvalidArgument, "cannot report yourself")
	}

	if _, ok := allowedReportCategories[req.GetReasonCategory()]; !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid reason_category")
	}

	if _, err := s.userSvc.GetProfile(ctx, targetID); err != nil {
		return nil, toGRPCError(err)
	}

	event := publisher.ReportSubmittedEvent{
		ReporterID:     payload.UserID.String(),
		TargetType:     "user",
		TargetID:       targetID.String(),
		ReasonCategory: req.GetReasonCategory(),
		Description:    req.GetDescription(),
		SubmittedAt:    time.Now().UTC(),
	}
	if err := s.pub.PublishReportSubmitted(ctx, event); err != nil {
		log.Printf("[auth-service] publish report.submitted failed: %v", err)
		return nil, status.Error(codes.Internal, "failed to submit report")
	}

	return &pb.ReportUserResponse{Message: "report submitted"}, nil
}
