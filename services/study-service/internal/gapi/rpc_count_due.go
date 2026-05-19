package gapi

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "mem_pan/services/study-service/pb"
)

// CountDueForUser is an internal RPC consumed by notification-service from its
// reminder cron handlers. It does not require the user-auth interceptor —
// authentication is enforced at the network layer (shared secret / private VPC).
func (s *Server) CountDueForUser(ctx context.Context, req *pb.CountDueForUserRequest) (*pb.CountDueForUserResponse, error) {
	uid, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}
	due, err := s.studySvc.CountDueByEndOfDay(ctx, uid, req.Timezone)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.CountDueForUserResponse{DueToday: due}, nil
}
