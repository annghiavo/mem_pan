package gapi

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"mem_pan/services/search-service/internal/es"
	"mem_pan/services/search-service/internal/service"
	pb "mem_pan/services/search-service/pb"
)

func (s *Server) SearchUsers(ctx context.Context, req *pb.SearchUsersRequest) (*pb.SearchUsersResponse, error) {
	result, err := s.svc.SearchUsers(ctx, service.UserSearchParams{
		Query:    req.GetQuery(),
		Page:     req.GetPage(),
		PageSize: req.GetPageSize(),
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "search failed")
	}

	users := make([]*pb.User, 0, len(result.Hits))
	for _, h := range result.Hits {
		var doc es.UserDoc
		if err := json.Unmarshal(h.Source, &doc); err != nil {
			continue
		}
		users = append(users, &pb.User{
			UserId:    doc.UserID,
			Username:  doc.Username,
			FullName:  doc.FullName,
			AvatarUrl: doc.AvatarURL,
			CreatedAt: timestamppb.New(doc.CreatedAt),
			Score:     h.Score,
		})
	}
	return &pb.SearchUsersResponse{Users: users, Total: result.Total}, nil
}
