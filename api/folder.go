package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	db "mem_pan/db/sqlc"
)

type createFolderRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
}

type listFoldersRequest struct {
	PageID   int32 `form:"page_id" binding:"required,min=1"`
	PageSize int32 `form:"page_size" binding:"required,min=1,max=100"`
}

type folderDeckURIRequest struct {
	FolderID string `uri:"folder_id" binding:"required,uuid"`
	DeckID   string `uri:"deck_id" binding:"required,uuid"`
}

type folderWithDecks struct {
	Folder db.Folder `json:"folder"`
	Decks  []db.Deck `json:"decks"`
}

func (server *Server) createFolder(ctx *gin.Context) {
	var req createFolderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*AuthPayload)
	folder, err := server.store.CreateFolder(ctx, db.CreateFolderParams{
		UserID:      authPayload.UserID,
		Name:        strings.TrimSpace(req.Name),
		Description: toNullString(req.Description),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}
	ctx.JSON(http.StatusOK, folder)
}

func (server *Server) listFolders(ctx *gin.Context) {
	var req listFoldersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*AuthPayload)
	folders, err := server.store.ListFoldersByUser(ctx, db.ListFoldersByUserParams{
		UserID: authPayload.UserID,
		Limit:  req.PageSize,
		Offset: (req.PageID - 1) * req.PageSize,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	result := make([]folderWithDecks, 0, len(folders))
	for _, folder := range folders {
		decks, err := server.store.ListDecksInFolder(ctx, folder.FolderID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(err))
			return
		}
		result = append(result, folderWithDecks{Folder: folder, Decks: decks})
	}

	ctx.JSON(http.StatusOK, result)
}

func (server *Server) addDeckToFolder(ctx *gin.Context) {
	var req folderDeckURIRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	folderID, _ := uuid.Parse(req.FolderID)
	deckID, _ := uuid.Parse(req.DeckID)
	authPayload := ctx.MustGet(authorizationPayloadKey).(*AuthPayload)

	folder, err := server.store.GetFolder(ctx, folderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}
	if folder.UserID != authPayload.UserID {
		ctx.JSON(http.StatusUnauthorized, errorResponse(errors.New("folder doesn't belong to the authenticated user")))
		return
	}

	deck, err := server.store.GetDeck(ctx, deckID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}
	if deck.UserID != authPayload.UserID {
		ctx.JSON(http.StatusUnauthorized, errorResponse(errors.New("deck doesn't belong to the authenticated user")))
		return
	}

	added, err := server.store.AddDeckToFolder(ctx, db.AddDeckToFolderParams{FolderID: folderID, DeckID: deckID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusOK, gin.H{"folder_id": folderID, "deck_id": deckID, "message": "already exists"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	ctx.JSON(http.StatusOK, added)
}

func (server *Server) removeDeckFromFolder(ctx *gin.Context) {
	var req folderDeckURIRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	folderID, _ := uuid.Parse(req.FolderID)
	deckID, _ := uuid.Parse(req.DeckID)
	authPayload := ctx.MustGet(authorizationPayloadKey).(*AuthPayload)

	folder, err := server.store.GetFolder(ctx, folderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}
	if folder.UserID != authPayload.UserID {
		ctx.JSON(http.StatusUnauthorized, errorResponse(errors.New("folder doesn't belong to the authenticated user")))
		return
	}

	err = server.store.RemoveDeckFromFolder(ctx, db.RemoveDeckFromFolderParams{FolderID: folderID, DeckID: deckID})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}
	ctx.Status(http.StatusNoContent)
}
