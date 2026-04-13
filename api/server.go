package api

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	db "mem_pan/db/sqlc"
)

const authorizationPayloadKey = "authorization_payload"

type AuthPayload struct {
	UserID uuid.UUID
}

type Server struct {
	db     *sql.DB
	store  Store
	router *gin.Engine
}

type Store interface {
	CreateDeck(ctx context.Context, arg db.CreateDeckParams) (db.Deck, error)
	GetDeck(ctx context.Context, deckID uuid.UUID) (db.Deck, error)
	ListUserDecks(ctx context.Context, arg db.ListUserDecksParams) ([]db.Deck, error)
	SearchPublicDecks(ctx context.Context, arg db.SearchPublicDecksParams) ([]db.Deck, error)
	UpdateDeck(ctx context.Context, arg db.UpdateDeckParams) (db.Deck, error)
	DeleteDeck(ctx context.Context, deckID uuid.UUID) (db.Deck, error)
	CreateFolder(ctx context.Context, arg db.CreateFolderParams) (db.Folder, error)
	ListFoldersByUser(ctx context.Context, arg db.ListFoldersByUserParams) ([]db.Folder, error)
	ListDecksInFolder(ctx context.Context, folderID uuid.UUID) ([]db.Deck, error)
	AddDeckToFolder(ctx context.Context, arg db.AddDeckToFolderParams) (db.FolderDeck, error)
	RemoveDeckFromFolder(ctx context.Context, arg db.RemoveDeckFromFolderParams) error
	GetFolder(ctx context.Context, folderID uuid.UUID) (db.Folder, error)
	ListCardsByDeck(ctx context.Context, deckID uuid.UUID) ([]db.Card, error)
	GetNote(ctx context.Context, noteID uuid.UUID) (db.Note, error)
	CreateNote(ctx context.Context, arg db.CreateNoteParams) (db.Note, error)
	CreateCard(ctx context.Context, arg db.CreateCardParams) (db.Card, error)
}

func NewServer(database *sql.DB) *Server {
	return NewServerWithStore(database, db.New(database))
}

func NewServerWithStore(database *sql.DB, store Store) *Server {
	server := &Server{
		db:    database,
		store: store,
	}
	server.setupRouter()
	return server
}

func (server *Server) setupRouter() {
	router := gin.Default()
	authRoutes := router.Group("/")
	authRoutes.Use(authMiddleware())
	{
		authRoutes.POST("/decks", server.createDeck)
		authRoutes.GET("/decks/:id", server.getDeck)
		authRoutes.GET("/decks", server.listDecks)
		authRoutes.GET("/decks/public/search", server.searchPublicDecks)
		authRoutes.PATCH("/decks/:id", server.updateDeck)
		authRoutes.DELETE("/decks/:id", server.deleteDeck)
		authRoutes.POST("/decks/:id/clone", server.cloneDeck)

		authRoutes.POST("/folders", server.createFolder)
		authRoutes.GET("/folders", server.listFolders)
		authRoutes.POST("/folders/:folder_id/decks/:deck_id", server.addDeckToFolder)
		authRoutes.DELETE("/folders/:folder_id/decks/:deck_id", server.removeDeckFromFolder)
	}

	server.router = router
}

func (server *Server) Start(address string) error {
	return server.router.Run(address)
}

func (server *Server) Router() http.Handler {
	return server.router
}

func errorResponse(err error) gin.H {
	return gin.H{"error": err.Error()}
}
