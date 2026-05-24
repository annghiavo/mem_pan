package gapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/admin-service/internal/db"
	pb "mem_pan/services/admin-service/pb/proto"
	notifypb "mem_pan/services/notification-service/pb"
)

func (s *Server) ListEmailTemplates(ctx context.Context, _ *pb.ListEmailTemplatesRequest) (*pb.ListEmailTemplatesResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if payload.Role != "admin" {
		return nil, status.Error(codes.PermissionDenied, "admin access required")
	}

	resp, err := s.notifyClient.ListEmailTemplates(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*pb.EmailTemplate, 0, len(resp.GetTemplates()))
	for _, t := range resp.GetTemplates() {
		out = append(out, notifyTemplateToAdmin(t))
	}
	return &pb.ListEmailTemplatesResponse{Templates: out}, nil
}

func (s *Server) GetEmailTemplate(ctx context.Context, req *pb.GetEmailTemplateRequest) (*pb.EmailTemplate, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if payload.Role != "admin" {
		return nil, status.Error(codes.PermissionDenied, "admin access required")
	}
	if req.GetTemplateKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "template_key is required")
	}

	t, err := s.notifyClient.GetEmailTemplate(ctx, req.GetTemplateKey(), req.GetLocale())
	if err != nil {
		return nil, err
	}
	return notifyTemplateToAdmin(t), nil
}

func (s *Server) UpdateEmailTemplate(ctx context.Context, req *pb.UpdateEmailTemplateRequest) (*pb.EmailTemplate, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if payload.Role != "admin" {
		return nil, status.Error(codes.PermissionDenied, "admin access required")
	}
	if req.GetTemplateKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "template_key is required")
	}

	updated, err := s.notifyClient.UpdateEmailTemplate(ctx, &notifypb.UpdateEmailTemplateRequest{
		TemplateKey: req.GetTemplateKey(),
		Locale:      req.GetLocale(),
		Subject:     req.GetSubject(),
		HtmlBody:    req.GetHtmlBody(),
		TextBody:    req.GetTextBody(),
	})
	if err != nil {
		return nil, err
	}

	// Audit the change. Use the template UUID as target_id so the log links
	// back to the row that changed; metadata records the human-readable key.
	templateID, parseErr := uuid.Parse(updated.GetId())
	if parseErr == nil {
		meta, _ := json.Marshal(map[string]any{
			"template_key": updated.GetTemplateKey(),
			"locale":       updated.GetLocale(),
			"version":      updated.GetVersion(),
		})
		if _, logErr := s.reportRepo.CreateModerationLog(ctx, db.CreateModerationLogParams{
			AdminID:    payload.UserID,
			Action:     "update_email_template",
			TargetType: "email_template",
			TargetID:   templateID,
			Reason:     sql.NullString{String: "admin edit", Valid: true},
			Metadata:   sql.NullString{String: string(meta), Valid: true},
		}); logErr != nil {
			log.Printf("[admin] failed to write moderation log for template update: %v", logErr)
		}
	}

	return notifyTemplateToAdmin(updated), nil
}

func (s *Server) PreviewEmailTemplate(ctx context.Context, req *pb.PreviewEmailTemplateRequest) (*pb.PreviewEmailTemplateResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if payload.Role != "admin" {
		return nil, status.Error(codes.PermissionDenied, "admin access required")
	}
	if req.GetTemplateKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "template_key is required")
	}

	resp, err := s.notifyClient.PreviewEmailTemplate(ctx, &notifypb.PreviewEmailTemplateRequest{
		TemplateKey: req.GetTemplateKey(),
		Locale:      req.GetLocale(),
		Data:        req.GetData(),
	})
	if err != nil {
		return nil, err
	}
	return &pb.PreviewEmailTemplateResponse{
		Subject:  resp.GetSubject(),
		HtmlBody: resp.GetHtmlBody(),
		TextBody: resp.GetTextBody(),
	}, nil
}

func (s *Server) SendTestEmail(ctx context.Context, req *pb.SendTestEmailRequest) (*pb.SendTestEmailResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if payload.Role != "admin" {
		return nil, status.Error(codes.PermissionDenied, "admin access required")
	}
	if req.GetTemplateKey() == "" || req.GetTo() == "" {
		return nil, status.Error(codes.InvalidArgument, "template_key and to are required")
	}

	resp, err := s.notifyClient.SendTestEmail(ctx, &notifypb.SendTestEmailRequest{
		TemplateKey: req.GetTemplateKey(),
		Locale:      req.GetLocale(),
		To:          req.GetTo(),
		Data:        req.GetData(),
	})
	if err != nil {
		return nil, err
	}
	return &pb.SendTestEmailResponse{Message: resp.GetMessage()}, nil
}

func notifyTemplateToAdmin(t *notifypb.EmailTemplate) *pb.EmailTemplate {
	if t == nil {
		return nil
	}
	return &pb.EmailTemplate{
		Id:          t.GetId(),
		TemplateKey: t.GetTemplateKey(),
		Locale:      t.GetLocale(),
		Subject:     t.GetSubject(),
		HtmlBody:    t.GetHtmlBody(),
		TextBody:    t.GetTextBody(),
		Variables:   t.GetVariables(),
		IsActive:    t.GetIsActive(),
		Version:     t.GetVersion(),
		UpdatedBy:   t.GetUpdatedBy(),
		CreatedAt:   t.GetCreatedAt(),
		UpdatedAt:   t.GetUpdatedAt(),
	}
}
