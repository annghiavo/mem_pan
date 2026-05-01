package gapi

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/deck-service/internal/parser"
	"mem_pan/services/deck-service/pb"
)

// ParseImportFile parses a CSV/TSV/PDF file sent by the client and returns
// the extracted card pairs for the user to review before importing.
//
// HTTP: POST /v1/import/parse   (Bearer auth required)
// Body JSON: {"file_content": "<base64>", "file_type": "csv"|"tsv"|"pdf"}
//
// The client then edits the preview and calls BulkCreateCards to commit.
func (s *Server) ParseImportFile(ctx context.Context, req *pb.ParseImportFileRequest) (*pb.ParseImportFileResponse, error) {
	if _, err := s.authorizeUser(ctx); err != nil {
		return nil, err
	}

	if len(req.FileContent) == 0 {
		return nil, status.Error(codes.InvalidArgument, "file_content is required")
	}

	fileType := strings.ToLower(strings.TrimSpace(req.FileType))
	if fileType == "" {
		return nil, status.Error(codes.InvalidArgument, "file_type is required (csv, tsv, or pdf)")
	}

	var (
		parsed []parser.ParsedCard
		err    error
	)

	switch fileType {
	case "csv", "tsv":
		parsed, err = parser.ParseCSV(req.FileContent)
	case "pdf":
		parsed, err = parser.ParsePDF(req.FileContent)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unsupported file_type %q; use csv, tsv, or pdf", fileType)
	}

	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "parse file: %v", err)
	}

	cards := make([]*pb.ParsedCard, len(parsed))
	for i, c := range parsed {
		cards[i] = &pb.ParsedCard{
			Front: c.Front,
			Back:  c.Back,
		}
	}

	return &pb.ParseImportFileResponse{
		Cards: cards,
		Total: int32(len(cards)),
	}, nil
}
