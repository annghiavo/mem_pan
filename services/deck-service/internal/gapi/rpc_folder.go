package gapi

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/deck-service/internal/service"
	"mem_pan/services/deck-service/pb"
)

func (s *Server) ListFolders(ctx context.Context, _ *pb.ListFoldersRequest) (*pb.ListFoldersResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	folders, err := s.folderSvc.ListFolders(ctx, payload.UserID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	out := make([]*pb.Folder, len(folders))
	for i, f := range folders {
		out[i] = dbFolderToPb(f)
	}
	return &pb.ListFoldersResponse{Folders: out}, nil
}

func (s *Server) CreateFolder(ctx context.Context, req *pb.CreateFolderRequest) (*pb.CreateFolderResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	folder, err := s.folderSvc.CreateFolder(ctx, service.CreateFolderParams{
		UserID:      payload.UserID,
		Name:        req.Name,
		Description: nullStrFromProto(req.Description),
		IsPublic:    req.IsPublic,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.CreateFolderResponse{Folder: dbFolderToPb(folder)}, nil
}

func (s *Server) GetFolder(ctx context.Context, req *pb.GetFolderRequest) (*pb.GetFolderResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	folderID, err := uuid.Parse(req.FolderId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid folder_id")
	}

	result, err := s.folderSvc.GetFolder(ctx, folderID, payload.UserID, true)
	if err != nil {
		return nil, toGRPCError(err)
	}

	decks := make([]*pb.Deck, len(result.Decks))
	for i, d := range result.Decks {
		decks[i] = dbDeckToPb(d)
	}
	return &pb.GetFolderResponse{
		Data: &pb.FolderWithDecks{
			Folder: dbFolderToPb(result.Folder),
			Decks:  decks,
		},
	}, nil
}

func (s *Server) UpdateFolder(ctx context.Context, req *pb.UpdateFolderRequest) (*pb.UpdateFolderResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	folderID, err := uuid.Parse(req.FolderId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid folder_id")
	}

	folder, err := s.folderSvc.UpdateFolder(ctx, service.UpdateFolderParams{
		FolderID:    folderID,
		UserID:      payload.UserID,
		Name:        nullStrFromProto(req.Name),
		Description: nullStrFromProto(req.Description),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.UpdateFolderResponse{Folder: dbFolderToPb(folder)}, nil
}

func (s *Server) DeleteFolder(ctx context.Context, req *pb.DeleteFolderRequest) (*pb.DeleteFolderResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	folderID, err := uuid.Parse(req.FolderId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid folder_id")
	}

	if err := s.folderSvc.DeleteFolder(ctx, folderID, payload.UserID); err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.DeleteFolderResponse{Success: true}, nil
}

func (s *Server) AddDeckToFolder(ctx context.Context, req *pb.AddDeckToFolderRequest) (*pb.AddDeckToFolderResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	folderID, err := uuid.Parse(req.FolderId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid folder_id")
	}
	deckID, err := uuid.Parse(req.DeckId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}

	if err := s.folderSvc.AddDeckToFolder(ctx, folderID, deckID, payload.UserID); err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.AddDeckToFolderResponse{Success: true}, nil
}

func (s *Server) ListPublicFolders(ctx context.Context, req *pb.ListPublicFoldersRequest) (*pb.ListPublicFoldersResponse, error) {
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (req.Page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	page, err := s.folderSvc.ListPublicFolders(ctx, service.ListPublicFoldersParams{
		Limit:  pageSize,
		Offset: offset,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	folders := make([]*pb.Folder, len(page.Folders))
	for i, f := range page.Folders {
		folders[i] = dbFolderToPb(f)
	}
	return &pb.ListPublicFoldersResponse{Folders: folders, Total: page.Total}, nil
}

func (s *Server) UpdateFolderVisibility(ctx context.Context, req *pb.UpdateFolderVisibilityRequest) (*pb.UpdateFolderVisibilityResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	folderID, err := uuid.Parse(req.FolderId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid folder_id")
	}

	folder, err := s.folderSvc.UpdateFolderVisibility(ctx, folderID, payload.UserID, req.IsPublic)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.UpdateFolderVisibilityResponse{Folder: dbFolderToPb(folder)}, nil
}

func (s *Server) RemoveDeckFromFolder(ctx context.Context, req *pb.RemoveDeckFromFolderRequest) (*pb.RemoveDeckFromFolderResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	folderID, err := uuid.Parse(req.FolderId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid folder_id")
	}
	deckID, err := uuid.Parse(req.DeckId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
	}

	if err := s.folderSvc.RemoveDeckFromFolder(ctx, folderID, deckID, payload.UserID); err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.RemoveDeckFromFolderResponse{Success: true}, nil
}
