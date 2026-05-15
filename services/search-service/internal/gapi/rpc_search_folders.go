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

func (s *Server) SearchFolders(ctx context.Context, req *pb.SearchFoldersRequest) (*pb.SearchFoldersResponse, error) {
	scope := service.FolderScopePublic
	switch req.GetScope() {
	case pb.FolderSearchScope_FOLDER_SCOPE_MINE:
		scope = service.FolderScopeMine
	case pb.FolderSearchScope_FOLDER_SCOPE_ALL:
		scope = service.FolderScopeAll
	}

	callerID := ""
	if scope == service.FolderScopeMine || scope == service.FolderScopeAll {
		payload, err := s.authorizeUser(ctx)
		if err != nil {
			return nil, err
		}
		callerID = payload.UserID.String()
	} else {
		if payload, _ := s.optionalUser(ctx); payload != nil {
			callerID = payload.UserID.String()
		}
	}

	result, err := s.svc.SearchFolders(ctx, service.FolderSearchParams{
		Query:    req.GetQuery(),
		Scope:    scope,
		CallerID: callerID,
		Page:     req.GetPage(),
		PageSize: req.GetPageSize(),
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "search failed")
	}

	folders := make([]*pb.Folder, 0, len(result.Hits))
	for _, h := range result.Hits {
		var doc es.FolderDoc
		if err := json.Unmarshal(h.Source, &doc); err != nil {
			continue
		}
		folders = append(folders, &pb.Folder{
			FolderId:    doc.FolderID,
			UserId:      doc.UserID,
			Name:        doc.Name,
			Description: doc.Description,
			IsPublic:    doc.IsPublic,
			CreatedAt:   timestamppb.New(doc.CreatedAt),
			UpdatedAt:   timestamppb.New(doc.UpdatedAt),
			Score:       h.Score,
		})
	}
	return &pb.SearchFoldersResponse{Folders: folders, Total: result.Total}, nil
}
