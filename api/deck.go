package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	db "mem_pan/db/sqlc"
)

type createDeckRequest struct {
	Name        string            `json:"name" binding:"required"`
	Description *string           `json:"description"`
	IsPublic    *bool             `json:"is_public"`
	Status      *db.ContentStatus `json:"status"`
	Settings    jsonRaw           `json:"settings"`
}

type getDeckRequest struct {
	ID string `uri:"id" binding:"required,uuid"`
}

type listDecksRequest struct {
	PageID   int32 `form:"page_id" binding:"required,min=1"`
	PageSize int32 `form:"page_size" binding:"required,min=1,max=100"`
}

type searchDecksRequest struct {
	Q        string `form:"q"`
	PageID   int32  `form:"page_id" binding:"required,min=1"`
	PageSize int32  `form:"page_size" binding:"required,min=1,max=100"`
}

type updateDeckRequest struct {
	Name        *string           `json:"name"`
	Description *string           `json:"description"`
	IsPublic    *bool             `json:"is_public"`
	Status      *db.ContentStatus `json:"status"`
	Settings    jsonRaw           `json:"settings"`
}

type deleteDeckRequest struct {
	ID string `uri:"id" binding:"required,uuid"`
}

type cloneDeckRequest struct {
	TargetUserID *string `json:"target_user_id"`
	NewName      *string `json:"new_name"`
}

type jsonRaw json.RawMessage

func (j jsonRaw) toNull() pqtype.NullRawMessage {
	if len(j) == 0 {
		return pqtype.NullRawMessage{}
	}
	return pqtype.NullRawMessage{RawMessage: json.RawMessage(j), Valid: true}
}

func (server *Server) createDeck(ctx *gin.Context) {
	var req createDeckRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*AuthPayload)
	arg := db.CreateDeckParams{
		UserID:      authPayload.UserID,
		Name:        strings.TrimSpace(req.Name),
		Description: toNullString(req.Description),
		IsPublic:    toNullBool(req.IsPublic),
		Status:      toNullStatus(req.Status),
		Settings:    req.Settings.toNull(),
	}

	deck, err := server.store.CreateDeck(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	ctx.JSON(http.StatusOK, deck)
}

func (server *Server) getDeck(ctx *gin.Context) {
	var req getDeckRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	deckID, _ := uuid.Parse(req.ID)
	deck, err := server.store.GetDeck(ctx, deckID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*AuthPayload)
	if !canAccessDeck(authPayload.UserID, deck) {
		ctx.JSON(http.StatusUnauthorized, errorResponse(errors.New("deck doesn't belong to the authenticated user")))
		return
	}

	ctx.JSON(http.StatusOK, deck)
}

func (server *Server) listDecks(ctx *gin.Context) {
	var req listDecksRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*AuthPayload)
	arg := db.ListUserDecksParams{
		UserID: authPayload.UserID,
		Limit:  req.PageSize,
		Offset: (req.PageID - 1) * req.PageSize,
	}

	decks, err := server.store.ListUserDecks(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}
	ctx.JSON(http.StatusOK, decks)
}

func (server *Server) searchPublicDecks(ctx *gin.Context) {
	var req searchDecksRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	arg := db.SearchPublicDecksParams{
		Column1: sql.NullString{String: strings.TrimSpace(req.Q), Valid: true},
		Limit:   req.PageSize,
		Offset:  (req.PageID - 1) * req.PageSize,
	}

	decks, err := server.store.SearchPublicDecks(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}
	ctx.JSON(http.StatusOK, decks)
}

func (server *Server) updateDeck(ctx *gin.Context) {
	var uriReq deleteDeckRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req updateDeckRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	deckID, _ := uuid.Parse(uriReq.ID)
	existing, err := server.store.GetDeck(ctx, deckID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*AuthPayload)
	if existing.UserID != authPayload.UserID {
		ctx.JSON(http.StatusUnauthorized, errorResponse(errors.New("deck doesn't belong to the authenticated user")))
		return
	}

	arg := db.UpdateDeckParams{
		DeckID:      deckID,
		Name:        toNullString(req.Name),
		Description: toNullString(req.Description),
		IsPublic:    toNullBool(req.IsPublic),
		Status:      toNullStatus(req.Status),
		Settings:    req.Settings.toNull(),
	}

	updated, err := server.store.UpdateDeck(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}
	ctx.JSON(http.StatusOK, updated)
}

func (server *Server) deleteDeck(ctx *gin.Context) {
	var req deleteDeckRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	deckID, _ := uuid.Parse(req.ID)
	existing, err := server.store.GetDeck(ctx, deckID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*AuthPayload)
	if existing.UserID != authPayload.UserID {
		ctx.JSON(http.StatusUnauthorized, errorResponse(errors.New("deck doesn't belong to the authenticated user")))
		return
	}

	deleted, err := server.store.DeleteDeck(ctx, deckID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}
	ctx.JSON(http.StatusOK, deleted)
}

func (server *Server) cloneDeck(ctx *gin.Context) {
	var uriReq deleteDeckRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req cloneDeckRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	sourceDeckID, _ := uuid.Parse(uriReq.ID)
	authPayload := ctx.MustGet(authorizationPayloadKey).(*AuthPayload)
	targetUserID := authPayload.UserID
	if req.TargetUserID != nil {
		parsed, err := uuid.Parse(*req.TargetUserID)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		targetUserID = parsed
	}

	cloned, err := server.cloneDeckTx(ctx, sourceDeckID, authPayload.UserID, targetUserID, req.NewName)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			ctx.JSON(http.StatusUnauthorized, errorResponse(err))
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	ctx.JSON(http.StatusOK, cloned)
}

func (server *Server) cloneDeckTx(ctx *gin.Context, sourceDeckID, requesterID, targetUserID uuid.UUID, newName *string) (db.Deck, error) {
	if server.db == nil {
		return db.Deck{}, errors.New("database is not configured")
	}

	tx, err := server.db.BeginTx(ctx, nil)
	if err != nil {
		return db.Deck{}, err
	}
	defer tx.Rollback()

	clonedDeck, err := cloneDeckWithStore(ctx, db.New(tx), sourceDeckID, requesterID, targetUserID, newName)
	if err != nil {
		return db.Deck{}, err
	}

	if err := tx.Commit(); err != nil {
		return db.Deck{}, err
	}
	return clonedDeck, nil
}

type cloneStore interface {
	GetDeck(ctx context.Context, deckID uuid.UUID) (db.Deck, error)
	CreateDeck(ctx context.Context, arg db.CreateDeckParams) (db.Deck, error)
	ListCardsByDeck(ctx context.Context, deckID uuid.UUID) ([]db.Card, error)
	GetNote(ctx context.Context, noteID uuid.UUID) (db.Note, error)
	CreateNote(ctx context.Context, arg db.CreateNoteParams) (db.Note, error)
	CreateCard(ctx context.Context, arg db.CreateCardParams) (db.Card, error)
}

func cloneDeckWithStore(ctx context.Context, qtx cloneStore, sourceDeckID, requesterID, targetUserID uuid.UUID, newName *string) (db.Deck, error) {
	sourceDeck, err := qtx.GetDeck(ctx, sourceDeckID)
	if err != nil {
		return db.Deck{}, err
	}
	if !canAccessDeck(requesterID, sourceDeck) {
		return db.Deck{}, ErrForbidden
	}

	name := sourceDeck.Name + " (Copy)"
	if newName != nil && strings.TrimSpace(*newName) != "" {
		name = strings.TrimSpace(*newName)
	}

	clonedDeck, err := qtx.CreateDeck(ctx, db.CreateDeckParams{
		UserID:      targetUserID,
		Name:        name,
		Description: sourceDeck.Description,
		IsPublic:    sql.NullBool{Bool: false, Valid: true},
		Status:      sourceDeck.Status,
		Settings:    sourceDeck.Settings,
	})
	if err != nil {
		return db.Deck{}, err
	}

	cards, err := qtx.ListCardsByDeck(ctx, sourceDeckID)
	if err != nil {
		return db.Deck{}, err
	}

	noteMap := make(map[uuid.UUID]uuid.UUID)
	for _, card := range cards {
		newNoteID, ok := noteMap[card.NoteID]
		if !ok {
			note, err := qtx.GetNote(ctx, card.NoteID)
			if err != nil {
				return db.Deck{}, err
			}
			newNote, err := qtx.CreateNote(ctx, db.CreateNoteParams{
				UserID:       targetUserID,
				ContentFront: note.ContentFront,
				ContentBack:  note.ContentBack,
				ImageUrl:     note.ImageUrl,
			})
			if err != nil {
				return db.Deck{}, err
			}
			newNoteID = newNote.NoteID
			noteMap[card.NoteID] = newNoteID
		}

		nextReview := card.NextReviewDate
		if !nextReview.Valid {
			nextReview = sql.NullTime{Time: time.Now(), Valid: true}
		}
		_, err = qtx.CreateCard(ctx, db.CreateCardParams{
			UserID:         targetUserID,
			NoteID:         newNoteID,
			DeckID:         clonedDeck.DeckID,
			State:          card.State,
			NextReviewDate: nextReview,
		})
		if err != nil {
			return db.Deck{}, err
		}
	}

	return clonedDeck, nil
}
