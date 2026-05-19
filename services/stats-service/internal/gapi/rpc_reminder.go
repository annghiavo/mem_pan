package gapi

import (
	"context"

	pb "mem_pan/services/stats-service/pb"
)

// ListReminderState is an internal-only RPC consumed by notification-service
// from its reminder cron handlers. It bypasses the user-auth interceptor —
// rely on private VPC / shared-secret at the deployment layer.
func (s *Server) ListReminderState(ctx context.Context, req *pb.ListReminderStateRequest) (*pb.ListReminderStateResponse, error) {
	rows, err := s.statsSvc.ListReminderState(ctx, req.OnlyActiveStreak)
	if err != nil {
		return nil, toGRPCError(err)
	}

	out := make([]*pb.UserReminderState, 0, len(rows))
	for _, r := range rows {
		u := &pb.UserReminderState{
			UserId:             r.UserID.String(),
			CurrentStreak:      r.CurrentStreak,
			OptimalHourWeekday: -1,
			OptimalHourWeekend: -1,
			ReminderLocalTime:  r.ReminderLocalTime,
		}
		if r.LastStudiedDate != nil {
			u.LastStudiedDate = r.LastStudiedDate.Format("2006-01-02")
		}
		if r.OptimalHourWeekday != nil {
			u.OptimalHourWeekday = int32(*r.OptimalHourWeekday)
		}
		if r.OptimalHourWeekend != nil {
			u.OptimalHourWeekend = int32(*r.OptimalHourWeekend)
		}
		out = append(out, u)
	}
	return &pb.ListReminderStateResponse{Users: out}, nil
}
