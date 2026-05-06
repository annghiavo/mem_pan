package gapi

import (
	"encoding/json"
	"net/http"

	"google.golang.org/grpc/metadata"
)

// ServeUploadCardImage handles POST /v1/cards/upload-image as multipart/form-data.
// Form fields:
//   - image — the image file (required, max 10 MB)
//
// Returns: { "image_url": "https://res.cloudinary.com/..." }
func (s *Server) ServeUploadCardImage(w http.ResponseWriter, r *http.Request) {
	if s.uploader == nil {
		writeHTTPError(w, http.StatusServiceUnavailable, "image upload is not configured")
		return
	}

	ctx := r.Context()
	if auth := r.Header.Get("Authorization"); auth != "" {
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", auth))
	}
	if _, err := s.authorizeUser(ctx); err != nil {
		writeHTTPError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeHTTPError(w, http.StatusBadRequest, "request too large or invalid form")
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		writeHTTPError(w, http.StatusBadRequest, "image field is required")
		return
	}
	defer file.Close()

	imageURL, err := s.uploader.Upload(ctx, file, "mem_pan/cards")
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, "failed to upload image")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"image_url": imageURL})
}
