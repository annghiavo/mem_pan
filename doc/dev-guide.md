# Developer Guide — Xây dựng Feature từ A đến Z

Hướng dẫn này mô tả cách viết query, xây dựng repository, service và endpoint trong dự án, lấy **deck-service** làm ví dụ minh họa.

---

## Tổng quan kiến trúc

Dự án sử dụng **Clean Architecture** với stack: Go + gRPC + PostgreSQL + sqlc.

Mỗi feature đi qua 5 lớp theo thứ tự:

```
SQL Query → Repository → Service → gRPC Handler → Proto/Route
```

Cấu trúc thư mục của một service:

```
services/deck-service/
├── cmd/server/main.go          # Entry point, dependency injection
├── config/                     # Load config từ environment
├── db/
│   ├── migration/              # SQL schema migrations
│   └── query/                  # SQL query definitions
├── internal/
│   ├── db/                     # Code được generate bởi sqlc (KHÔNG sửa tay)
│   ├── domain/                 # Domain errors, constants
│   ├── gapi/                   # gRPC handlers
│   ├── mock/                   # Mock được generate bởi mockgen
│   ├── repository/             # Repository interfaces + implementations
│   └── service/                # Business logic
├── pb/                         # Protobuf Go code (KHÔNG sửa tay)
└── proto/                      # Proto definitions
```

---

## Bước 1 — SQL Migration (Schema)

**File:** `db/migration/XXXXXX_name.up.sql`

```sql
CREATE TYPE content_status AS ENUM ('active', 'hidden', 'deleted');

CREATE TABLE decks (
    deck_id     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL,
    name        varchar NOT NULL,
    description text,
    is_public   boolean NOT NULL DEFAULT false,
    status      content_status NOT NULL DEFAULT 'active',
    settings    jsonb NOT NULL DEFAULT '{}',
    card_count  int NOT NULL DEFAULT 0,
    cloned_from uuid,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ON decks (user_id);
CREATE INDEX ON decks (is_public) WHERE status = 'active';
```

**Quy tắc:**
- Luôn dùng `uuid` làm primary key với `gen_random_uuid()`
- Soft delete bằng `status` enum — **không dùng** `DELETE` thật
- Thêm index cho cột thường xuất hiện trong `WHERE`
- Luôn có file `.down.sql` để rollback

---

## Bước 2 — SQL Queries

**File:** `db/query/deck.sql`

```sql
-- name: CreateDeck :one
INSERT INTO decks (user_id, name, description, is_public)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetDeckByID :one
SELECT * FROM decks
WHERE deck_id = $1 AND status != 'deleted'
LIMIT 1;

-- name: ListDecksByUser :many
SELECT * FROM decks
WHERE user_id = $1 AND status != 'deleted'
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountDecksByUser :one
SELECT COUNT(*) FROM decks
WHERE user_id = $1 AND status != 'deleted';

-- name: UpdateDeck :one
UPDATE decks
SET name        = $2,
    description = $3,
    updated_at  = now()
WHERE deck_id = $1
RETURNING *;

-- name: SoftDeleteDeck :exec
UPDATE decks
SET status     = 'deleted',
    updated_at = now()
WHERE deck_id = $1 AND user_id = $2;
```

**Quy tắc:**
- Comment `-- name: MethodName :one/:many/:exec` là bắt buộc cho sqlc
- `:one` → trả về 1 row | `:many` → trả về slice | `:exec` → không trả về row
- Sau khi viết xong chạy `make sqlc` hoặc `sqlc generate` để tạo Go code

---

## Bước 3 — Domain Errors

**File:** `internal/domain/errors.go`

```go
package domain

import "errors"

var (
    ErrDeckNotFound = errors.New("deck not found")
    ErrCardNotFound = errors.New("card not found")
    ErrForbidden    = errors.New("access denied")
)
```

Khai báo tất cả lỗi business tại đây. Repository và Service sẽ dùng lại các lỗi này.

---

## Bước 4 — Repository Layer

**File:** `internal/repository/deck_repo.go`

```go
package repository

import (
    "context"
    "database/sql"
    "errors"

    "github.com/google/uuid"
    db "github.com/mem_pan/deck-service/internal/db"
    "github.com/mem_pan/deck-service/internal/domain"
)

// Interface — bắt buộc để mock khi test
type DeckRepository interface {
    CreateDeck(ctx context.Context, arg db.CreateDeckParams) (db.Deck, error)
    GetDeckByID(ctx context.Context, id uuid.UUID) (db.Deck, error)
    ListDecksByUser(ctx context.Context, arg db.ListDecksByUserParams) ([]db.Deck, error)
    CountDecksByUser(ctx context.Context, userID uuid.UUID) (int64, error)
    UpdateDeck(ctx context.Context, arg db.UpdateDeckParams) (db.Deck, error)
    SoftDeleteDeck(ctx context.Context, arg db.SoftDeleteDeckParams) error
}

// Implementation
type deckRepository struct {
    q *db.Queries
}

func NewDeckRepository(database *sql.DB) DeckRepository {
    return &deckRepository{q: db.New(database)}
}

func (r *deckRepository) GetDeckByID(ctx context.Context, id uuid.UUID) (db.Deck, error) {
    d, err := r.q.GetDeckByID(ctx, id)
    if errors.Is(err, sql.ErrNoRows) {
        return db.Deck{}, domain.ErrDeckNotFound  // Chuyển lỗi DB → domain error
    }
    return d, err
}

func (r *deckRepository) CreateDeck(ctx context.Context, arg db.CreateDeckParams) (db.Deck, error) {
    return r.q.CreateDeck(ctx, arg)
}

func (r *deckRepository) SoftDeleteDeck(ctx context.Context, arg db.SoftDeleteDeckParams) error {
    return r.q.SoftDeleteDeck(ctx, arg)
}

// ... các method còn lại tương tự
```

**Quy tắc:**
- Luôn khai báo **interface** trước implementation
- Repository chỉ làm **một việc duy nhất**: chuyển `sql.ErrNoRows` → domain error
- **Không** chứa business logic ở đây

---

## Bước 5 — Service Layer (Business Logic)

**File:** `internal/service/deck_service.go`

```go
package service

import (
    "context"
    "database/sql"

    "github.com/google/uuid"
    db "github.com/mem_pan/deck-service/internal/db"
    "github.com/mem_pan/deck-service/internal/domain"
    "github.com/mem_pan/deck-service/internal/repository"
)

// Params riêng — không dùng trực tiếp db.XxxParams
type CreateDeckParams struct {
    UserID      uuid.UUID
    Name        string
    Description sql.NullString
    IsPublic    bool
}

type UpdateDeckParams struct {
    DeckID      uuid.UUID
    UserID      uuid.UUID
    Name        string
    Description sql.NullString
}

type ListDecksParams struct {
    UserID uuid.UUID
    Limit  int32
    Page   int32
}

type DecksPage struct {
    Decks []db.Deck
    Total int64
}

// Interface
type DeckService interface {
    CreateDeck(ctx context.Context, p CreateDeckParams) (db.Deck, error)
    GetDeck(ctx context.Context, deckID, userID uuid.UUID) (db.Deck, error)
    ListDecks(ctx context.Context, p ListDecksParams) (DecksPage, error)
    UpdateDeck(ctx context.Context, p UpdateDeckParams) (db.Deck, error)
    DeleteDeck(ctx context.Context, deckID, userID uuid.UUID) error
}

type deckService struct {
    deckRepo repository.DeckRepository
}

func NewDeckService(deckRepo repository.DeckRepository) DeckService {
    return &deckService{deckRepo: deckRepo}
}

func (s *deckService) CreateDeck(ctx context.Context, p CreateDeckParams) (db.Deck, error) {
    return s.deckRepo.CreateDeck(ctx, db.CreateDeckParams{
        UserID:      p.UserID,
        Name:        p.Name,
        Description: p.Description,
        IsPublic:    p.IsPublic,
    })
}

func (s *deckService) GetDeck(ctx context.Context, deckID, userID uuid.UUID) (db.Deck, error) {
    deck, err := s.deckRepo.GetDeckByID(ctx, deckID)
    if err != nil {
        return db.Deck{}, err
    }
    // Kiểm tra quyền truy cập
    if deck.UserID != userID && !deck.IsPublic {
        return db.Deck{}, domain.ErrForbidden
    }
    return deck, nil
}

func (s *deckService) UpdateDeck(ctx context.Context, p UpdateDeckParams) (db.Deck, error) {
    deck, err := s.deckRepo.GetDeckByID(ctx, p.DeckID)
    if err != nil {
        return db.Deck{}, err
    }
    // Chỉ owner mới được sửa
    if deck.UserID != p.UserID {
        return db.Deck{}, domain.ErrForbidden
    }
    return s.deckRepo.UpdateDeck(ctx, db.UpdateDeckParams{
        DeckID:      p.DeckID,
        Name:        p.Name,
        Description: p.Description,
    })
}

func (s *deckService) DeleteDeck(ctx context.Context, deckID, userID uuid.UUID) error {
    deck, err := s.deckRepo.GetDeckByID(ctx, deckID)
    if err != nil {
        return err
    }
    if deck.UserID != userID {
        return domain.ErrForbidden
    }
    return s.deckRepo.SoftDeleteDeck(ctx, db.SoftDeleteDeckParams{
        DeckID: deckID,
        UserID: userID,
    })
}

func (s *deckService) ListDecks(ctx context.Context, p ListDecksParams) (DecksPage, error) {
    offset := (p.Page - 1) * p.Limit

    decks, err := s.deckRepo.ListDecksByUser(ctx, db.ListDecksByUserParams{
        UserID: p.UserID,
        Limit:  p.Limit,
        Offset: offset,
    })
    if err != nil {
        return DecksPage{}, err
    }

    total, err := s.deckRepo.CountDecksByUser(ctx, p.UserID)
    if err != nil {
        return DecksPage{}, err
    }

    return DecksPage{Decks: decks, Total: total}, nil
}
```

**Quy tắc:**
- Service là nơi **duy nhất** chứa business logic và kiểm tra authorization (ownership)
- Dùng params riêng thay vì truyền thẳng `db.Params` để tách biệt lớp
- Không gọi trực tiếp DB, chỉ gọi qua Repository interface

---

## Bước 6 — Proto Definition (Routes + Contract)

**File:** `proto/deck_service.proto`

```protobuf
syntax = "proto3";

service DeckService {
    rpc ListDecks (ListDecksRequest) returns (ListDecksResponse) {
        option (google.api.http) = { get: "/v1/decks" };
    }
    rpc CreateDeck (CreateDeckRequest) returns (CreateDeckResponse) {
        option (google.api.http) = { post: "/v1/decks"; body: "*" };
    }
    rpc GetDeck (GetDeckRequest) returns (GetDeckResponse) {
        option (google.api.http) = { get: "/v1/decks/{deck_id}" };
    }
    rpc UpdateDeck (UpdateDeckRequest) returns (UpdateDeckResponse) {
        option (google.api.http) = { put: "/v1/decks/{deck_id}"; body: "*" };
    }
    rpc DeleteDeck (DeleteDeckRequest) returns (google.protobuf.Empty) {
        option (google.api.http) = { delete: "/v1/decks/{deck_id}" };
    }
}

message Deck {
    string deck_id     = 1;
    string user_id     = 2;
    string name        = 3;
    string description = 4;
    bool   is_public   = 5;
    int32  card_count  = 6;
    google.protobuf.Timestamp created_at = 7;
    google.protobuf.Timestamp updated_at = 8;
}

message CreateDeckRequest {
    string name        = 1;
    string description = 2;
    bool   is_public   = 3;
}

message CreateDeckResponse {
    Deck deck = 1;
}

message GetDeckRequest {
    string deck_id = 1;
}

message GetDeckResponse {
    Deck deck = 1;
}

message ListDecksRequest {
    int32 page       = 1;
    int32 page_size  = 2;
}

message ListDecksResponse {
    repeated Deck decks = 1;
    int64         total = 2;
}
```

Sau khi sửa proto, chạy `make proto` để generate lại Go code trong `pb/`.

---

## Bước 7 — gRPC Handler

**File:** `internal/gapi/rpc_deck.go`

```go
package gapi

import (
    "context"
    "errors"

    "github.com/google/uuid"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "google.golang.org/protobuf/types/known/timestamppb"

    db "github.com/mem_pan/deck-service/internal/db"
    "github.com/mem_pan/deck-service/internal/domain"
    "github.com/mem_pan/deck-service/internal/service"
    pb "github.com/mem_pan/deck-service/pb"
)

func (s *Server) CreateDeck(ctx context.Context, req *pb.CreateDeckRequest) (*pb.CreateDeckResponse, error) {
    // 1. Xác thực user từ JWT trong gRPC metadata
    payload, err := s.authorizeUser(ctx)
    if err != nil {
        return nil, err
    }

    // 2. Validate input
    if req.Name == "" {
        return nil, status.Error(codes.InvalidArgument, "name is required")
    }

    // 3. Gọi service
    deck, err := s.deckSvc.CreateDeck(ctx, service.CreateDeckParams{
        UserID:   payload.UserID,
        Name:     req.Name,
        IsPublic: req.IsPublic,
    })
    if err != nil {
        return nil, toGRPCError(err)
    }

    // 4. Trả về response
    return &pb.CreateDeckResponse{Deck: dbDeckToPb(deck)}, nil
}

func (s *Server) GetDeck(ctx context.Context, req *pb.GetDeckRequest) (*pb.GetDeckResponse, error) {
    payload, err := s.authorizeUser(ctx)
    if err != nil {
        return nil, err
    }

    deckID, err := uuid.Parse(req.DeckId)
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
    }

    deck, err := s.deckSvc.GetDeck(ctx, deckID, payload.UserID)
    if err != nil {
        return nil, toGRPCError(err)
    }

    return &pb.GetDeckResponse{Deck: dbDeckToPb(deck)}, nil
}

func (s *Server) ListDecks(ctx context.Context, req *pb.ListDecksRequest) (*pb.ListDecksResponse, error) {
    payload, err := s.authorizeUser(ctx)
    if err != nil {
        return nil, err
    }

    pageSize := req.PageSize
    if pageSize <= 0 {
        pageSize = 20
    }

    page, err := s.deckSvc.ListDecks(ctx, service.ListDecksParams{
        UserID: payload.UserID,
        Limit:  pageSize,
        Page:   req.Page,
    })
    if err != nil {
        return nil, toGRPCError(err)
    }

    pbDecks := make([]*pb.Deck, len(page.Decks))
    for i, d := range page.Decks {
        pbDecks[i] = dbDeckToPb(d)
    }

    return &pb.ListDecksResponse{Decks: pbDecks, Total: page.Total}, nil
}

func (s *Server) DeleteDeck(ctx context.Context, req *pb.DeleteDeckRequest) (*emptypb.Empty, error) {
    payload, err := s.authorizeUser(ctx)
    if err != nil {
        return nil, err
    }

    deckID, err := uuid.Parse(req.DeckId)
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid deck_id")
    }

    if err := s.deckSvc.DeleteDeck(ctx, deckID, payload.UserID); err != nil {
        return nil, toGRPCError(err)
    }

    return &emptypb.Empty{}, nil
}

// Chuyển domain errors → gRPC status codes
func toGRPCError(err error) error {
    switch {
    case errors.Is(err, domain.ErrDeckNotFound):
        return status.Error(codes.NotFound, err.Error())
    case errors.Is(err, domain.ErrForbidden):
        return status.Error(codes.PermissionDenied, err.Error())
    default:
        return status.Error(codes.Internal, "internal server error")
    }
}

// Chuyển DB model → Protobuf message
func dbDeckToPb(d db.Deck) *pb.Deck {
    r := &pb.Deck{
        DeckId:    d.DeckID.String(),
        UserId:    d.UserID.String(),
        Name:      d.Name,
        IsPublic:  d.IsPublic,
        CardCount: d.CardCount,
        CreatedAt: timestamppb.New(d.CreatedAt),
        UpdatedAt: timestamppb.New(d.UpdatedAt),
    }
    if d.Description.Valid {
        r.Description = d.Description.String
    }
    return r
}
```

**Quy tắc:**
- Handler chỉ làm 4 việc: **authorize → validate → gọi service → convert response**
- Không chứa business logic
- Luôn dùng `toGRPCError()` để map domain error → gRPC status code

---

## Bước 8 — Dependency Injection (main.go)

**File:** `cmd/server/main.go`

```go
func main() {
    // 1. Config
    cfg, err := config.Load()

    // 2. Database
    database, err := sql.Open("postgres", cfg.DBUrl)

    // 3. External clients
    authClient, err := authclient.NewGRPCClient(cfg.AuthServiceAddress)

    // 4. Repositories (inject database)
    deckRepo := repository.NewDeckRepository(database)
    cardRepo := repository.NewCardRepository(database)

    // 5. Services (inject repositories)
    deckSvc := service.NewDeckService(deckRepo)
    cardSvc := service.NewCardService(cardRepo, deckRepo)

    // 6. Server (inject services + clients)
    server := gapi.NewServer(deckSvc, cardSvc, authClient)

    // 7. Chạy gRPC + HTTP gateway
    go runGRPCServer(cfg, server)
    runHTTPGateway(cfg, server)
}
```

Thứ tự inject: `DB → Repository → Service → Server`. Không bao giờ inject ngược chiều.

---

## Luồng dữ liệu end-to-end

```
POST /v1/decks  (HTTP)
        │
        ▼
gRPC Handler (gapi/rpc_deck.go)
  ├── authorizeUser()     ← xác thực JWT từ metadata
  ├── validate input      ← kiểm tra required fields
  └── gọi service
        │
        ▼
Service (service/deck_service.go)
  ├── kiểm tra ownership  ← business rule
  └── gọi repository
        │
        ▼
Repository (repository/deck_repo.go)
  ├── gọi sqlc query
  └── chuyển sql.ErrNoRows → domain error
        │
        ▼
sqlc generated code (internal/db/)
        │
        ▼
PostgreSQL
        │
        ▲  (trả về data ngược lên)
        │
gRPC Handler
  └── dbDeckToPb()        ← convert DB model → Protobuf
        │
        ▼
JSON Response
```

---

## Checklist khi thêm feature mới

- [ ] Viết SQL migration nếu cần thay đổi schema
- [ ] Viết SQL query trong `db/query/` → chạy `sqlc generate`
- [ ] Thêm domain error trong `internal/domain/errors.go` nếu cần
- [ ] Thêm method vào Repository interface + implementation
- [ ] Thêm method vào Service interface + implementation
- [ ] Thêm RPC vào `.proto` → chạy `make proto`
- [ ] Viết handler trong `gapi/rpc_xxx.go`
- [ ] Cập nhật `toGRPCError()` nếu có domain error mới
- [ ] Đăng ký dependency trong `main.go` nếu có service/repo mới
- [ ] Viết unit test cho service với mock repository

---

## Testing Pattern

```go
func TestCreateDeck_Success(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    deckRepo := mock.NewMockDeckRepository(ctrl)

    expected := db.Deck{DeckID: uuid.New(), Name: "My Deck"}

    deckRepo.EXPECT().
        CreateDeck(gomock.Any(), gomock.Any()).
        Return(expected, nil)

    svc := service.NewDeckService(deckRepo)
    result, err := svc.CreateDeck(context.Background(), service.CreateDeckParams{
        UserID: uuid.New(),
        Name:   "My Deck",
    })

    assert.NoError(t, err)
    assert.Equal(t, expected.Name, result.Name)
}

func TestGetDeck_Forbidden(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    deckRepo := mock.NewMockDeckRepository(ctrl)

    ownerID := uuid.New()
    otherID := uuid.New()
    privateDeck := db.Deck{UserID: ownerID, IsPublic: false}

    deckRepo.EXPECT().
        GetDeckByID(gomock.Any(), gomock.Any()).
        Return(privateDeck, nil)

    svc := service.NewDeckService(deckRepo)
    _, err := svc.GetDeck(context.Background(), uuid.New(), otherID)

    assert.ErrorIs(t, err, domain.ErrForbidden)
}
```

Regenerate mocks sau khi thay đổi interface:

```bash
go generate ./internal/repository/...
go generate ./internal/service/...
```
