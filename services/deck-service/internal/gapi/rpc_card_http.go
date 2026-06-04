package gapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"mem_pan/services/deck-service/internal/service"
)

// ServeCreateCard handles POST /v1/decks/{deck_id}/cards as multipart/form-data.
// Form fields:
//   - content_front  string (required)
//   - content_back   string (required)
//   - image          file   (optional — uploaded to Cloudinary)
//   - image_url      string (optional — used when image file is absent)
//   - position       int    (optional)
//   - lang_front     string (optional, default "en")
//   - lang_back      string (optional, default "en")
func (s *Server) ServeCreateCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if auth := r.Header.Get("Authorization"); auth != "" {
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", auth))
	}
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		writeHTTPError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	deckID, err := uuid.Parse(r.PathValue("deck_id"))
	if err != nil {
		writeHTTPError(w, http.StatusBadRequest, "invalid deck_id")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeHTTPError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	contentFront := r.FormValue("content_front")
	contentBack := r.FormValue("content_back")
	if contentFront == "" || contentBack == "" {
		writeHTTPError(w, http.StatusBadRequest, "content_front and content_back are required")
		return
	}

	imageURL, err := s.resolveImageURL(ctx, r)
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, "failed to upload image")
		return
	}

	var position int32
	if p := r.FormValue("position"); p != "" {
		n, _ := strconv.Atoi(p)
		position = int32(n)
	}

	card, err := s.cardSvc.CreateCard(ctx, service.CreateCardParams{
		UserID:       payload.UserID,
		DeckID:       deckID,
		ContentFront: contentFront,
		ContentBack:  contentBack,
		ImageURL:     imageURL,
		Position:     position,
		LangFront:    r.FormValue("lang_front"),
		LangBack:     r.FormValue("lang_back"),
	})
	if err != nil {
		st, _ := status.FromError(toGRPCError(err))
		writeHTTPError(w, grpcCodeToHTTP(st.Code()), st.Message())
		return
	}

	cardJSON, _ := protojson.Marshal(dbCardRowToPb(card))
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"card":%s}`, cardJSON)
}

// ServeUpdateCard handles PUT /v1/cards/{card_id} as multipart/form-data.
// Form fields (all optional — only provided fields are updated):
//   - content_front  string
//   - content_back   string
//   - image          file   (uploaded to Cloudinary, replaces existing image)
//   - image_url      string (used when image file is absent)
//   - lang_front     string
//   - lang_back      string
func (s *Server) ServeUpdateCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if auth := r.Header.Get("Authorization"); auth != "" {
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", auth))
	}
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		writeHTTPError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	cardID, err := uuid.Parse(r.PathValue("card_id"))
	if err != nil {
		writeHTTPError(w, http.StatusBadRequest, "invalid card_id")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeHTTPError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	imageURL, err := s.resolveImageURL(ctx, r)
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, "failed to upload image")
		return
	}

	card, err := s.cardSvc.UpdateCard(ctx, service.UpdateCardParams{
		CardID:       cardID,
		UserID:       payload.UserID,
		ContentFront: nullStrFromProto(r.FormValue("content_front")),
		ContentBack:  nullStrFromProto(r.FormValue("content_back")),
		ImageURL:     imageURL,
		LangFront:    nullStrFromProto(r.FormValue("lang_front")),
		LangBack:     nullStrFromProto(r.FormValue("lang_back")),
	})
	if err != nil {
		st, _ := status.FromError(toGRPCError(err))
		writeHTTPError(w, grpcCodeToHTTP(st.Code()), st.Message())
		return
	}

	cardJSON, _ := protojson.Marshal(dbCardRowToPb(card))
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"card":%s}`, cardJSON)
}

// resolveImageURL uploads the "image" file to Cloudinary when present,
// otherwise falls back to the "image_url" text field. Returns nil when neither is set.
//
// If the form contains an "image" part we always attempt to upload it: silently
// treating a zero-byte upload as "no image" produced a confusing bug where the
// frontend reported success but the card kept its old image URL.
func (s *Server) resolveImageURL(ctx context.Context, r *http.Request) (*string, error) {
	file, fh, err := r.FormFile("image")
	if err == nil {
		defer file.Close()
		if fh.Size == 0 {
			return nil, fmt.Errorf("uploaded image is empty")
		}
		if s.uploader == nil {
			return nil, fmt.Errorf("uploader not configured")
		}
		return s.uploadFile(ctx, file)
	}
	if !errors.Is(err, http.ErrMissingFile) {
		return nil, err
	}
	return nullStrFromProto(r.FormValue("image_url")), nil
}

func (s *Server) uploadFile(ctx context.Context, file multipart.File) (*string, error) {
	url, err := s.uploader.Upload(ctx, file, "mem_pan/cards")
	if err != nil {
		return nil, err
	}
	return &url, nil
}

// ServeReorderCards handles PUT /v1/decks/{deck_id}/cards/reorder.
// Body (JSON): { "card_ids": ["uuid1", "uuid2", ...] }
// The provided order becomes the new position sequence (0-indexed).
func (s *Server) ServeReorderCards(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if auth := r.Header.Get("Authorization"); auth != "" {
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", auth))
	}
	payload, err := s.authorizeUser(ctx)
	if err != nil {
		writeHTTPError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	deckID, err := uuid.Parse(r.PathValue("deck_id"))
	if err != nil {
		writeHTTPError(w, http.StatusBadRequest, "invalid deck_id")
		return
	}

	var body struct {
		CardIDs []string `json:"card_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.CardIDs) == 0 {
		writeHTTPError(w, http.StatusBadRequest, "card_ids is required")
		return
	}

	cardUUIDs := make([]uuid.UUID, 0, len(body.CardIDs))
	for _, id := range body.CardIDs {
		uid, err := uuid.Parse(id)
		if err != nil {
			writeHTTPError(w, http.StatusBadRequest, "invalid card_id: "+id)
			return
		}
		cardUUIDs = append(cardUUIDs, uid)
	}

	if err := s.cardSvc.ReorderCards(ctx, deckID, payload.UserID, cardUUIDs); err != nil {
		st, _ := status.FromError(toGRPCError(err))
		writeHTTPError(w, grpcCodeToHTTP(st.Code()), st.Message())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"success":true}`)
}
