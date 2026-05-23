package gapi

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/notification-service/internal/service"
	pb "mem_pan/services/notification-service/pb"
)

// validTestNotificationTypes mirrors the switch in service.SendTestNotification.
// Validating up-front lets us return InvalidArgument with a clear message
// instead of leaking the service-layer error as an Internal status.
var validTestNotificationTypes = map[string]bool{
	"":               true, // empty → defaults to study_reminder
	"study_reminder": true,
	"streak_warning": true,
}

func (s *Server) SendTestNotification(ctx context.Context, req *pb.SendTestNotificationRequest) (*pb.SendTestNotificationResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	if !validTestNotificationTypes[req.NotificationType] {
		return nil, status.Errorf(codes.InvalidArgument,
			"notification_type must be 'study_reminder' or 'streak_warning' (got %q)", req.NotificationType)
	}

	res, err := s.svc.SendTestNotification(ctx, payload.UserID, service.TestNotificationParams{
		Type:     req.NotificationType,
		Token:    req.Token,
		DueCount: req.DueCount,
		Streak:   req.Streak,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.SendTestNotificationResponse{
		DeviceCount: res.DeviceCount,
		Title:       res.Title,
		Body:        res.Body,
	}, nil
}
