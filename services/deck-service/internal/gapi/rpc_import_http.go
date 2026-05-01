package gapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"mem_pan/services/deck-service/pb"
)

// ServeParseImportFile handles POST /v1/import/parse as multipart/form-data,
// allowing clients to upload the file directly without base64 encoding.
// Form fields:
//   - file      — the CSV, TSV, or PDF file (required)
//   - file_type — "csv", "tsv", or "pdf" (required)
func (s *Server) ServeParseImportFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeHTTPError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	fileType := strings.ToLower(strings.TrimSpace(r.FormValue("file_type")))

	f, _, err := r.FormFile("file")
	if err != nil {
		writeHTTPError(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	ctx := r.Context()
	if auth := r.Header.Get("Authorization"); auth != "" {
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", auth))
	}

	resp, err := s.ParseImportFile(ctx, &pb.ParseImportFileRequest{
		FileContent: content,
		FileType:    fileType,
	})
	if err != nil {
		st, _ := status.FromError(err)
		writeHTTPError(w, grpcCodeToHTTP(st.Code()), st.Message())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeHTTPError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"message": msg})
}

func grpcCodeToHTTP(code codes.Code) int {
	switch code {
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Unimplemented:
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}
