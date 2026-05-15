package gapi

import (
	"context"
	"encoding/json"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/notification-service/internal/authclient"
	"mem_pan/services/notification-service/internal/db"
	"mem_pan/services/notification-service/internal/domain"
	"mem_pan/services/notification-service/internal/service"
	pb "mem_pan/services/notification-service/pb"
)

func (s *Server) ListEmailTemplates(ctx context.Context, _ *pb.ListEmailTemplatesRequest) (*pb.ListEmailTemplatesResponse, error) {
	if _, err := s.authorizeAdmin(ctx); err != nil {
		return nil, err
	}
	rows, err := s.svc.ListEmailTemplates(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}
	out := make([]*pb.EmailTemplate, 0, len(rows))
	for _, r := range rows {
		out = append(out, dbTemplateToPb(r))
	}
	return &pb.ListEmailTemplatesResponse{Templates: out}, nil
}

func (s *Server) GetEmailTemplate(ctx context.Context, req *pb.GetEmailTemplateRequest) (*pb.EmailTemplate, error) {
	if _, err := s.authorizeAdmin(ctx); err != nil {
		return nil, err
	}
	if req.GetTemplateKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "template_key is required")
	}
	row, err := s.svc.GetEmailTemplate(ctx, req.GetTemplateKey(), req.GetLocale())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return dbTemplateToPb(row), nil
}

func (s *Server) UpdateEmailTemplate(ctx context.Context, req *pb.UpdateEmailTemplateRequest) (*pb.EmailTemplate, error) {
	payload, err := s.authorizeAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetTemplateKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "template_key is required")
	}
	if req.GetSubject() == "" || req.GetHtmlBody() == "" || req.GetTextBody() == "" {
		return nil, status.Error(codes.InvalidArgument, "subject, html_body, and text_body are required")
	}
	updated, err := s.svc.UpdateEmailTemplate(ctx, service.UpdateTemplateParams{
		Key:       req.GetTemplateKey(),
		Locale:    req.GetLocale(),
		Subject:   req.GetSubject(),
		HTMLBody:  req.GetHtmlBody(),
		TextBody:  req.GetTextBody(),
		UpdatedBy: payload.UserID,
	})
	if err != nil {
		if isValidationErr(err) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, toGRPCError(err)
	}
	return dbTemplateToPb(updated), nil
}

func (s *Server) PreviewEmailTemplate(ctx context.Context, req *pb.PreviewEmailTemplateRequest) (*pb.PreviewEmailTemplateResponse, error) {
	if _, err := s.authorizeAdmin(ctx); err != nil {
		return nil, err
	}
	if req.GetTemplateKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "template_key is required")
	}
	rendered, err := s.svc.PreviewEmailTemplate(ctx, req.GetTemplateKey(), req.GetLocale(), req.GetData())
	if err != nil {
		if isValidationErr(err) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, toGRPCError(err)
	}
	return &pb.PreviewEmailTemplateResponse{
		Subject:  rendered.Subject,
		HtmlBody: rendered.HTMLBody,
		TextBody: rendered.TextBody,
	}, nil
}

func (s *Server) SendTestEmail(ctx context.Context, req *pb.SendTestEmailRequest) (*pb.SendTestEmailResponse, error) {
	if _, err := s.authorizeAdmin(ctx); err != nil {
		return nil, err
	}
	if req.GetTemplateKey() == "" || req.GetTo() == "" {
		return nil, status.Error(codes.InvalidArgument, "template_key and to are required")
	}
	if err := s.svc.SendTestEmail(ctx, req.GetTemplateKey(), req.GetLocale(), req.GetTo(), req.GetData()); err != nil {
		if isValidationErr(err) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, toGRPCError(err)
	}
	return &pb.SendTestEmailResponse{Message: "test email sent"}, nil
}

func (s *Server) authorizeAdmin(ctx context.Context) (*authclient.Payload, error) {
	p, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if p.Role != "admin" {
		return nil, status.Error(codes.PermissionDenied, domain.ErrAdminRequired.Error())
	}
	return p, nil
}

func dbTemplateToPb(t db.EmailTemplate) *pb.EmailTemplate {
	var vars []string
	if len(t.Variables) > 0 {
		_ = json.Unmarshal(t.Variables, &vars)
	}
	var updatedBy string
	if t.UpdatedBy != nil {
		updatedBy = t.UpdatedBy.String()
	}
	return &pb.EmailTemplate{
		Id:          t.ID.String(),
		TemplateKey: t.TemplateKey,
		Locale:      t.Locale,
		Subject:     t.Subject,
		HtmlBody:    t.HtmlBody,
		TextBody:    t.TextBody,
		Variables:   vars,
		IsActive:    t.IsActive,
		Version:     t.Version,
		UpdatedBy:   updatedBy,
		CreatedAt:   t.CreatedAt.String(),
		UpdatedAt:   t.UpdatedAt.String(),
	}
}

func isValidationErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "invalid template") ||
		strings.Contains(msg, "subject parse") ||
		strings.Contains(msg, "html parse") ||
		strings.Contains(msg, "text parse") ||
		strings.Contains(msg, "render ")
}
