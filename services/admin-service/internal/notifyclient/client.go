package notifyclient

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	notifypb "mem_pan/services/notification-service/pb"

	"crypto/tls"
	"google.golang.org/grpc/credentials"
	"strings"
)

// Client is the subset of notification-service RPCs the admin-service forwards.
type Client interface {
	ListEmailTemplates(ctx context.Context) (*notifypb.ListEmailTemplatesResponse, error)
	GetEmailTemplate(ctx context.Context, key, locale string) (*notifypb.EmailTemplate, error)
	UpdateEmailTemplate(ctx context.Context, req *notifypb.UpdateEmailTemplateRequest) (*notifypb.EmailTemplate, error)
	PreviewEmailTemplate(ctx context.Context, req *notifypb.PreviewEmailTemplateRequest) (*notifypb.PreviewEmailTemplateResponse, error)
	SendTestEmail(ctx context.Context, req *notifypb.SendTestEmailRequest) (*notifypb.SendTestEmailResponse, error)
	Close() error
}

type grpcClient struct {
	conn      *grpc.ClientConn
	notifySvc notifypb.NotificationServiceClient
}

func NewGRPCClient(addr string) (Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(pickCreds(addr)))
	if err != nil {
		return nil, err
	}
	return &grpcClient{conn: conn, notifySvc: notifypb.NewNotificationServiceClient(conn)}, nil
}

// forwardAuth propagates the inbound bearer token to the downstream call so
// notification-service can re-verify the admin's role.
func forwardAuth(ctx context.Context) context.Context {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("authorization"); len(vals) > 0 {
			return metadata.AppendToOutgoingContext(ctx, "authorization", vals[0])
		}
	}
	return ctx
}

func (c *grpcClient) ListEmailTemplates(ctx context.Context) (*notifypb.ListEmailTemplatesResponse, error) {
	return c.notifySvc.ListEmailTemplates(forwardAuth(ctx), &notifypb.ListEmailTemplatesRequest{})
}

func (c *grpcClient) GetEmailTemplate(ctx context.Context, key, locale string) (*notifypb.EmailTemplate, error) {
	return c.notifySvc.GetEmailTemplate(forwardAuth(ctx), &notifypb.GetEmailTemplateRequest{TemplateKey: key, Locale: locale})
}

func (c *grpcClient) UpdateEmailTemplate(ctx context.Context, req *notifypb.UpdateEmailTemplateRequest) (*notifypb.EmailTemplate, error) {
	return c.notifySvc.UpdateEmailTemplate(forwardAuth(ctx), req)
}

func (c *grpcClient) PreviewEmailTemplate(ctx context.Context, req *notifypb.PreviewEmailTemplateRequest) (*notifypb.PreviewEmailTemplateResponse, error) {
	return c.notifySvc.PreviewEmailTemplate(forwardAuth(ctx), req)
}

func (c *grpcClient) SendTestEmail(ctx context.Context, req *notifypb.SendTestEmailRequest) (*notifypb.SendTestEmailResponse, error) {
	return c.notifySvc.SendTestEmail(forwardAuth(ctx), req)
}

func (c *grpcClient) Close() error { return c.conn.Close() }

// pickCreds returns TLS credentials when the target appears to be a
// Cloud Run / managed endpoint (port :443 or *.run.app), otherwise an
// insecure transport for local docker-compose or in-cluster gRPC.
func pickCreds(addr string) credentials.TransportCredentials {
	if strings.HasSuffix(addr, ":443") || strings.Contains(addr, ".run.app") {
		return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	}
	return insecure.NewCredentials()
}
