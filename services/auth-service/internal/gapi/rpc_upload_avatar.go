package gapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	cldapi "github.com/cloudinary/cloudinary-go/v2/api"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mem_pan/services/auth-service/internal/service"
	"mem_pan/services/auth-service/internal/token"
	"mem_pan/services/auth-service/pb"
)

const maxAvatarBytes = 5 << 20 // 5 MB

var allowedImageMIME = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

// UploadAvatar satisfies the gRPC interface for gRPC clients.
func (s *Server) UploadAvatar(ctx context.Context, req *pb.UploadAvatarRequest) (*pb.UploadAvatarResponse, error) {
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, err
	}

	data := req.Avatar
	if len(data) == 0 {
		return nil, status.Error(codes.InvalidArgument, "avatar bytes are required")
	}
	if len(data) > maxAvatarBytes {
		return nil, status.Error(codes.InvalidArgument, "avatar exceeds 5 MB limit")
	}

	ct := http.DetectContentType(data)
	if !allowedImageMIME[ct] {
		return nil, status.Error(codes.InvalidArgument, "only jpeg, png, gif, and webp images are allowed")
	}

	result, err := s.cld.Upload.Upload(ctx, bytes.NewReader(data), uploader.UploadParams{
		PublicID:     "avatars/" + payload.UserID.String(),
		Overwrite:    cldapi.Bool(true),
		ResourceType: "image",
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to upload image")
	}

	avatarURL := result.SecureURL
	user, err := s.userSvc.UpdateProfile(ctx, payload.UserID, service.UpdateProfileParams{
		AvatarURL: &avatarURL,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.UploadAvatarResponse{
		AvatarUrl: user.AvatarUrl.String,
		UserId:    user.UserID.String(),
		Username:  user.Username,
	}, nil
}

// UploadAvatarHTTP handles POST /v1/users/me/avatar as multipart/form-data.
// The client sends the file in a form field named "avatar".
func (s *Server) UploadAvatarHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "missing authorization header", http.StatusUnauthorized)
		return
	}
	fields := strings.Fields(authHeader)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "bearer") {
		http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
		return
	}
	payload, err := s.tokenMaker.VerifyToken(fields[1], token.TokenTypeAccess)
	if err != nil {
		http.Error(w, "invalid or expired access token", http.StatusUnauthorized)
		return
	}

	if err := r.ParseMultipartForm(maxAvatarBytes); err != nil {
		http.Error(w, "request too large or not multipart/form-data", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("avatar")
	if err != nil {
		http.Error(w, `form field "avatar" is required`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, int64(maxAvatarBytes)+1))
	if err != nil {
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}
	if len(data) > maxAvatarBytes {
		http.Error(w, "avatar exceeds 5 MB limit", http.StatusBadRequest)
		return
	}
	if len(data) == 0 {
		http.Error(w, "avatar file is empty", http.StatusBadRequest)
		return
	}

	ct := http.DetectContentType(data)
	if !allowedImageMIME[ct] {
		http.Error(w, "only jpeg, png, gif, and webp images are allowed", http.StatusBadRequest)
		return
	}

	result, err := s.cld.Upload.Upload(r.Context(), bytes.NewReader(data), uploader.UploadParams{
		PublicID:     "avatars/" + payload.UserID.String(),
		Overwrite:    cldapi.Bool(true),
		ResourceType: "image",
	})
	if err != nil {
		http.Error(w, "failed to upload image", http.StatusInternalServerError)
		return
	}

	avatarURL := result.SecureURL
	user, err := s.userSvc.UpdateProfile(r.Context(), payload.UserID, service.UpdateProfileParams{
		AvatarURL: &avatarURL,
	})
	if err != nil {
		http.Error(w, "failed to update profile", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"avatar_url": user.AvatarUrl.String,
		"user_id":    user.UserID.String(),
		"username":   user.Username,
	})
}
