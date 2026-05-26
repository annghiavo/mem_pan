# Ghi chú học kiến trúc mem_pan

> Tổng hợp các câu hỏi & giải đáp về cấu trúc tổng thể của dự án mem_pan.
> Mục tiêu: giúp người mới đọc code (kể cả khi code do agent viết) hiểu được luồng dữ liệu, các tầng, và quy trình làm việc.

---

## Mục lục

1. [Context (`ctx`) — được cấu hình ở đâu](#1-context-ctx--được-cấu-hình-ở-đâu)
2. [Context — ý nghĩa và tại sao cần](#2-context--ý-nghĩa-và-tại-sao-cần)
3. [Kiến trúc tầng: SQL → Repository → Service → Handler](#3-kiến-trúc-tầng-sql--repository--service--handler)
4. [Gọi qua interface hay trực tiếp — số bước từ handler đến client](#4-gọi-qua-interface-hay-trực-tiếp--số-bước-từ-handler-đến-client)
5. [Thư mục `pb/` được sinh từ proto](#5-thư-mục-pb-được-sinh-từ-proto)
6. [`pb.go` được gọi ở đâu và tác dụng gì](#6-pbgo-được-gọi-ở-đâu-và-tác-dụng-gì)
7. [Quy trình thêm 1 chức năng mới](#7-quy-trình-thêm-1-chức-năng-mới)

---

## 1. Context (`ctx`) — được cấu hình ở đâu

### Không có file cấu hình ctx tập trung

Dự án **không có** gói `pkg/context`, không có custom `ctxKey`, không có `FromContext`/`ToContext` helper. Lý do: tất cả service đều dùng **gRPC + grpc-gateway**, framework này đã quản trị ctx tự động (mỗi RPC nhận `ctx context.Context` từ runtime).

### Nơi `ctx` được khởi tạo gốc — `cmd/server/main.go`

Mỗi service Go có 1 root context tại `main()` để điều phối lifecycle (graceful shutdown):

```go
// services/deck-service/cmd/server/main.go:139
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
```

Pattern này lặp lại y hệt ở 7 service Go:

| Service | File | Dòng |
|---|---|---|
| deck-service | `cmd/server/main.go` | 139 |
| auth-service | `cmd/server/main.go` | 129 |
| study-service | `cmd/server/main.go` | 132 |
| admin-service | `cmd/server/main.go` | 131 |
| stats-service | `cmd/server/main.go` | 107 |
| notification-service | `cmd/server/main.go` | 164 |
| search-service | `cmd/server/main.go` | 46 (timeout bootstrap), 97 (cancel chính) |

Chỉ `search-service` có thêm `context.WithTimeout(..., 30s)` cho bootstrap (tạo index Algolia).

### Nơi `ctx` được làm giàu metadata — `internal/gapi/metadata.go`

Mỗi service có file giống nhau để rút token từ ctx:

```go
// services/deck-service/internal/gapi/metadata.go:14
func (s *Server) authorizeUser(ctx context.Context) (*authclient.Payload, error) {
    md, ok := metadata.FromIncomingContext(ctx)
    ...
    values := md.Get("authorization")
    return s.authClient.VerifyToken(ctx, fields[1])
}
```

→ ctx mang **gRPC metadata** (auth header) chứ không phải custom value qua `context.WithValue`.

### Nơi `ctx` được forward sang downstream gRPC — client wrapper

```go
// services/study-service/internal/deckclient/client.go:46
md := metadata.Pairs("authorization", "Bearer "+accessToken)
ctx = metadata.NewOutgoingContext(ctx, md)
resp, err := c.deckSvc.ListDeckCards(ctx, ...)
```

### Interceptor — `pkg/middleware/logging.go`

```go
func UnaryServerLogger(logger *slog.Logger) grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
        ...
        logger.LogAttrs(ctx, level, "grpc_call", attrs...)
        return resp, err
    }
}
```

Attach ở `buildGRPCServer`:
```go
grpcServer := grpc.NewServer(grpc.UnaryInterceptor(middleware.UnaryServerLogger(logger)))
```

### Sơ đồ ctx flow

```
incoming HTTP (grpc-gateway)
    │  r.Context()  ← stdlib net/http auto-cancel khi client disconnect
    ▼
grpc-gateway runtime ─── tự convert ─►  gRPC ctx (mang HTTP headers → metadata)
    │
    ▼
UnaryServerLogger interceptor (pkg/middleware/logging.go)
    │  ctx pass through
    ▼
gapi handler (internal/gapi/*.go)
    │  authorizeUser(ctx) → metadata.FromIncomingContext(ctx)
    ▼
service layer → repository (pgx) → ctx truyền xuống DB driver
    │
    └─ outbound gRPC call → metadata.NewOutgoingContext(ctx, md) → downstream service
```

### Tóm tắt

Hai chỗ duy nhất "configure" ctx:

1. **`cmd/server/main.go`** của mỗi service — tạo root ctx với `WithCancel` cho graceful shutdown.
2. **`internal/gapi/metadata.go`** — đọc gRPC metadata (auth header) từ ctx.

Còn lại, ctx được **truyền nguyên si** từ handler xuống service → repository → DB / outbound RPC.

---

## 2. Context — ý nghĩa và tại sao cần

### `ctx` mang theo 3 thứ qua mọi tầng

| Thông tin | Ai set | Ai đọc |
|---|---|---|
| **Tín hiệu huỷ** (`Done()`, `Err()`) | `main()` khi nhận SIGTERM, hoặc client disconnect | DB driver `pgx`, gRPC stub, HTTP server |
| **Deadline / timeout** | `context.WithTimeout` (vd: bootstrap 30s ở search-service) | Mọi I/O call (DB query, RPC) tự bỏ cuộc khi quá hạn |
| **gRPC metadata** (auth header, trace ID) | grpc-gateway tự inject từ HTTP request | `authorizeUser(ctx)` đọc `authorization: Bearer ...` |

### Tại sao mem_pan cần ctx

#### 1. Graceful shutdown trên Cloud Run

Cloud Run gửi `SIGTERM` rồi cho 10s để service đóng:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
// ...
<-quit              // bắt SIGTERM
cancel()            // huỷ ctx → mọi goroutine đang dùng nó dừng
```

Khi `cancel()` chạy, DB query đang `pgx.QueryContext(ctx, ...)` thấy `ctx.Done()` đóng → trả lỗi ngay thay vì giữ connection.

#### 2. Cascade cancel khi client bỏ đi

User đóng tab → grpc-gateway thấy HTTP connection đóng → cancel ctx → toàn bộ chain bên dưới (DB query, Pub/Sub publish, RPC sang service khác) huỷ luôn. Không cần code thủ công.

```go
// study-service/internal/deckclient/client.go:49
resp, err := c.deckSvc.ListDeckCards(ctx, ...)  // ctx của user request gốc
```

#### 3. Mang auth qua biên service (gRPC metadata)

ctx là **phương tiện duy nhất** để xác thực đi qua tầng:

```go
// internal/gapi/metadata.go
md, ok := metadata.FromIncomingContext(ctx)
values := md.Get("authorization")
```

Khi gọi sang service khác:
```go
ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token))
```

#### 4. Logging có ngữ cảnh (slog)

```go
// pkg/middleware/logging.go:41
logger.LogAttrs(ctx, level, "grpc_call", attrs...)
```

#### 5. Timeout cho I/O bên ngoài

Tránh service treo vô hạn:
- `search-service` bootstrap Algolia: `context.WithTimeout(ctx, 30*time.Second)`
- `authClient.VerifyToken(ctx, ...)` gọi auth-service: cùng ctx của request

### Vì sao không cần wrapper/custom ctx riêng

Codebase **không có** `context.WithValue` để nhét `userID` hay `traceID` thủ công. Lý do:

- Auth lấy từ **metadata** (chuẩn gRPC), không phải custom value → tránh "magic key" stringly-typed.
- `user_id` được truyền **explicit qua tham số function** sau khi `authorizeUser(ctx)` trả về `*Payload`. Type-safe hơn và dễ test.
- Trace tự động qua interceptor + `slog`.

**Ý nghĩa gói gọn**: "đưa tín hiệu huỷ + auth metadata xuyên qua mọi tầng I/O".

---

## 3. Kiến trúc tầng: SQL → Repository → Service → Handler

### Sơ đồ luồng đầy đủ (deck-service làm ví dụ)

```
SQL thuần           Code Go sinh tự động      Code mình viết tay

┌─────────────────┐
│ db/query/       │  ─── sqlc generate ───►   ┌─────────────────────┐
│   deck.sql      │                            │ internal/db/        │
│   card.sql      │                            │   deck.sql.go       │  ← TUYỆT
│   ...           │                            │   card.sql.go       │   ĐỐI KHÔNG
│                 │                            │   models.go         │   SỬA TAY
│ -- name: ...    │                            │                     │
│ SELECT ...      │                            │ Queries struct +    │
└─────────────────┘                            │ func per query      │
                                               └──────────┬──────────┘
                                                          │ wrap
                                                          ▼
                                               ┌─────────────────────┐
                                               │ internal/repository │  ← VIẾT TAY
                                               │   deck_repo.go      │   nhưng mỏng
                                               │                     │   (chủ yếu
                                               │ interface           │   forward)
                                               │   + struct impl     │
                                               └──────────┬──────────┘
                                                          │ injected vào
                                                          ▼
                                               ┌─────────────────────┐
                                               │ internal/service    │  ← VIẾT TAY
                                               │   deck_service.go   │   ★ logic
                                               │                     │   nghiệp vụ
                                               │ - validate input    │   ở đây
                                               │ - check ownership   │
                                               │ - gọi repo nhiều    │
                                               │   lần / transaction │
                                               │ - publish Pub/Sub   │
                                               └──────────┬──────────┘
                                                          │ injected vào
                                                          ▼
                                               ┌─────────────────────┐
                                               │ internal/gapi       │  ← VIẾT TAY
                                               │   rpc_deck.go       │   (handler)
                                               │                     │
                                               │ - authorizeUser(ctx)│
                                               │ - parse proto req   │
                                               │ - gọi service       │
                                               │ - map → proto resp  │
                                               └─────────────────────┘
                                                          ▲
                                                          │ gRPC request
                                          (grpc-gateway tự dịch HTTP → gRPC)
                                                          ▲
                                                       Client
```

### `db/` ≠ `repository/`

Hai layer trông giống nhau nhưng vai trò khác:

| Layer | Sinh ra từ | Vai trò |
|---|---|---|
| `internal/db/*.sql.go` | sqlc generate từ `db/query/*.sql` | "Driver" — 1 hàm Go ↔ 1 câu SQL. Không có logic. |
| `internal/repository/*_repo.go` | Viết tay | Wrapper mỏng quanh `db.Queries`. Định nghĩa **interface** → service mock được khi test. |

Tại sao cần `repository`? Vì `db.Queries` là struct cụ thể, không phải interface → không mock được.

### `handler` trong dự án này tên là `gapi`

`internal/gapi/` chính là handler. "gapi" = "gRPC API". Mỗi file `rpc_*.go` xử lý 1 nhóm RPC.

Pattern chuẩn:

```go
func (s *Server) CreateDeck(ctx context.Context, req *pb.CreateDeckRequest) (*pb.CreateDeckResponse, error) {
    payload, err := s.authorizeUser(ctx)           // 1. lấy user_id từ ctx
    if err != nil { return nil, err }

    if err := validateCreateDeck(req); err != nil { // 2. validate
        return nil, status.Error(codes.InvalidArgument, err.Error())
    }

    deck, err := s.deckService.Create(ctx, ...)    // 3. gọi service
    if err != nil { return nil, mapError(err) }

    return &pb.CreateDeckResponse{                  // 4. map response
        Deck: convertDeckToProto(deck),
    }, nil
}
```

### Sơ đồ ngắn để nhớ

```
gapi  → service  → repository → db (sqlc) → SQL
 ↑        ↑          ↑           ↑
auth   business    mockable     1 hàm
+      logic +     wrapper      = 1 SQL
proto  tx +        quanh
       events      sqlc
```

### Vì sao chia tầng kiểu này

| Tầng | Đổi cái gì thì sửa? |
|---|---|
| Đổi câu SQL | Chỉ sửa `db/query/*.sql` + chạy lại `sqlc generate` |
| Đổi logic nghiệp vụ | Chỉ sửa `internal/service/*.go` |
| Đổi protocol | Chỉ sửa `*.proto` + `internal/gapi/*.go` |
| Đổi cách test | Mock `repository` interface, không đụng DB thật |

→ Mỗi loại thay đổi gói gọn trong 1 tầng, không lan ra.

### Đọc 1 endpoint end-to-end

Lời khuyên: chọn 1 endpoint đơn giản, đọc theo thứ tự:

1. `db/query/deck.sql` — câu SQL `CreateDeck`
2. `internal/db/deck.sql.go` — hàm sinh ra (chỉ liếc qua)
3. `internal/repository/deck_repo.go` — phương thức `CreateDeck` (thường 1 dòng forward)
4. `internal/service/deck_service.go` — `Create()` — **chỗ thú vị nhất**
5. `internal/gapi/rpc_deck.go` — `CreateDeck` RPC — auth + map proto

---

## 4. Gọi qua interface hay trực tiếp — số bước từ handler đến client

### Tất cả các tầng gọi nhau qua INTERFACE

**Tầng repository** định nghĩa interface:
```go
// internal/repository/deck_repo.go:16
type DeckRepository interface {     // ← INTERFACE
    CreateDeck(ctx, arg) (db.Deck, error)
    GetDeckByID(ctx, id) (db.Deck, error)
    ...
}

type deckRepository struct {        // ← struct cụ thể (chữ thường, private)
    db *sql.DB
    q  *db.Queries
}

func NewDeckRepository(database *sql.DB) DeckRepository {  // ← TRẢ VỀ INTERFACE
    return &deckRepository{db: database, q: db.New(database)}
}
```

**Tầng service** nhận interface:
```go
// internal/service/deck_service.go:78
type deckService struct {
    deckRepo repository.DeckRepository  // ← KIỂU INTERFACE
    cardRepo repository.CardRepository
    pub      publisher.EventPublisher
}
```

**Tầng handler (gapi)** cũng nhận interface:
```go
// internal/gapi/server.go:20
type Server struct {
    folderSvc  service.FolderService  // ← INTERFACE
    deckSvc    service.DeckService
    cardSvc    service.CardService
    authClient authclient.Client
    ...
}
```

**Chỗ duy nhất biết struct cụ thể**: `cmd/server/main.go` (composition root):

```go
deckRepo := repository.NewDeckRepository(database)
deckSvc := service.NewDeckService(deckRepo, ...)
server := gapi.NewServer(folderSvc, deckSvc, ...)
```

### Vì sao bắt buộc interface

1. **Test**: mock `repository.DeckRepository` → test service không cần DB thật.
2. **Đổi implementation**: hôm nay `pgx`, mai `mongo` — chỉ viết struct mới impl interface.
3. **`publisher.EventPublisher`** có 2 impl — `NewNoopPublisher()` (dev) và `NewPubSubPublisher()` (prod).

### Bảng tóm tắt

| Caller | Callee | Kiểu khai báo |
|---|---|---|
| `handler (gapi.Server)` | service | `service.DeckService` (interface) |
| `service (deckService)` | repository | `repository.DeckRepository` (interface) |
| `repository (deckRepository)` | sqlc Queries | `*db.Queries` (struct cụ thể) |
| `sqlc Queries` | DB | `*sql.DB` |

Repo → sqlc không qua interface vì sqlc là code sinh, không có lý do mock nó.

### Số bước từ handler đến client (HTTP REST)

```
Client (browser / mobile app)
    │  HTTPS  GET /v1/decks
    ▼
API Gateway (Google Cloud API Gateway)                ← 1
    │  forward dựa trên path prefix /v1/decks/*
    │  endpoint: mempan-gateway-3hd0u0cm.uc.gateway.dev
    ▼
Cloud Run (deck-service)                              ← 2
    │  Container nhận HTTPS
    ▼
h2c.NewHandler + http2.Server                         ← 3 (HTTP/2 cleartext)
    │  upgrade HTTP/1.1 → HTTP/2
    ▼
mixed handler (cmd/server/main.go:165)                ← 4 (router)
    │  if Content-Type == "application/grpc" → grpcServer.ServeHTTP
    │  else → wrapped (HTTP)
    ▼
middleware.HTTPLogger + withCORS                       ← 5 (middleware)
    ▼
httpMux                                                ← 6 (chọn route)
    │  /v1/decks JSON → grpcMux (grpc-gateway)
    ▼
grpc-gateway runtime                                   ← 7 (dịch HTTP → gRPC)
    │  parse path/body theo annotation trong .proto
    │  build *pb.CreateDeckRequest từ JSON body
    │  copy HTTP headers → gRPC metadata
    ▼
gRPC server runtime                                    ← 8
    │  apply UnaryServerLogger interceptor
    ▼
gapi.Server.CreateDeck(ctx, req)                       ← 9 ★ HANDLER
    │  authorizeUser(ctx) → metadata.FromIncomingContext → authClient.VerifyToken
    │  parse req → service params
    ▼
deckSvc.CreateDeck(ctx, params)                        ← 10 SERVICE
    ▼
deckRepo.CreateDeck(ctx, arg)                          ← 11 REPOSITORY
    ▼
db.Queries.CreateDeck(ctx, arg)                        ← 12 SQLC
    ▼
PostgreSQL (Neon)                                      ← 13 DB
```

### Tóm tắt

- **9 bước hạ tầng/middleware** giữa handler và client.
- Code viết tay chỉ 4 tầng (handler → service → repo → db).
- 5 bước còn lại là **framework/infrastructure** lo (Cloud Run, gateway, grpc-gateway, gRPC runtime, h2c).
- **Quy ước**: gọi sang tầng khác trong cùng service luôn qua **interface**.
- **`cmd/server/main.go` là composition root** — chỗ duy nhất biết struct cụ thể.

---

## 5. Thư mục `pb/` được sinh từ proto

### Mapping file

| Proto (viết tay) | Sinh ra trong `pb/` | Vai trò |
|---|---|---|
| `rpc_deck.proto` | `rpc_deck.pb.go` | **Message structs**: `CreateDeckRequest`, `CreateDeckResponse`... + marshal/unmarshal protobuf |
| `deck_service.proto` (có `service ...`) | `deck_service_grpc.pb.go` | **gRPC stub**: `DeckServiceClient`, `DeckServiceServer`, `RegisterDeckServiceServer` |
| `deck_service.proto` (có annotation `google.api.http`) | `deck_service.pb.gw.go` | **grpc-gateway**: dịch HTTP REST → gRPC call |

Trên dòng đầu mọi file:
```go
// Code generated by protoc-gen-go. DO NOT EDIT.
```

### 3 thứ phải có để sinh code

1. **`protoc`** (compiler protobuf, viết bằng C++)
2. **`protoc-gen-go`** + **`protoc-gen-go-grpc`** (plugin sinh code Go)
3. **`protoc-gen-grpc-gateway`** + **`protoc-gen-openapiv2`** (plugin sinh gateway + Swagger)

### Lệnh sinh code — `services/deck-service/Makefile:17`

```makefile
proto:
    mkdir -p $(PB_OUT) $(SWAGGER_OUT)
    protoc \
        --proto_path=$(PROTO_DIR) \
        --proto_path=$(THIRD_PARTY) \
        --go_out=$(PB_OUT) --go_opt=paths=source_relative \
        --go-grpc_out=$(PB_OUT) --go-grpc_opt=paths=source_relative \
        --grpc-gateway_out=$(PB_OUT) --grpc-gateway_opt=paths=source_relative \
        --openapiv2_out=$(SWAGGER_OUT) \
        --openapiv2_opt=allow_merge=true,merge_file_name=deck_service \
        $(PROTO_DIR)/*.proto
```

### Sơ đồ tổng hợp

```
proto/                                pb/
├── deck_service.proto    ─┐       ┌─► deck_service.pb.go        (messages)
├── rpc_deck.proto         │       ├─► deck_service_grpc.pb.go   (gRPC client/server stub)
├── rpc_card.proto         │ make  ├─► deck_service.pb.gw.go     (HTTP gateway)
├── rpc_folder.proto       ├─proto─┼─► rpc_deck.pb.go            (messages)
├── models.proto           │       ├─► rpc_card.pb.go            (messages)
└── ...                   ─┘       └─► ...

                                    doc/swagger/
                                    └─► deck_service.swagger.json   (API docs cho FE)
```

### So sánh với sqlc

| | sqlc | protoc |
|---|---|---|
| Input | `db/query/*.sql` | `proto/*.proto` |
| Tool | `sqlc generate` | `protoc` + plugins |
| Output | `internal/db/*.sql.go` | `pb/*.pb.go` + `pb/*_grpc.pb.go` + `pb/*.pb.gw.go` |
| Sửa được? | **Không**, `DO NOT EDIT` | **Không**, `DO NOT EDIT` |
| Muốn đổi? | Sửa SQL → `make sqlc` | Sửa proto → `make proto` |

### Proto là hợp đồng chung giữa các service khác ngôn ngữ

- `deck-service/pb/` (Go server) — sinh từ `deck-service/proto/`.
- `study-service` cần gọi `deck-service` → cũng có thư mục `pb/` riêng sinh từ **cùng** `.proto`.
- `moderation-fsrs-service` (Python) → sinh stub Python từ `moderation_fsrs.proto` bằng `python -m grpc_tools.protoc`.

---

## 6. `pb.go` được gọi ở đâu và tác dụng gì

### Tác dụng — cung cấp 4 thứ

| Loại | Code sinh | Ví dụ tên |
|---|---|---|
| **Message struct** | `*.pb.go` | `pb.CreateDeckRequest`, `pb.Deck` |
| **Server interface** | `*_grpc.pb.go` | `pb.DeckServiceServer` + `pb.RegisterDeckServiceServer(...)` |
| **Client stub** | `*_grpc.pb.go` | `pb.DeckServiceClient` + `pb.NewDeckServiceClient(conn)` |
| **HTTP gateway** | `*.pb.gw.go` | `pb.RegisterDeckServiceHandlerServer(...)` |

### Nơi pb được gọi — 3 chỗ thực tế

#### 1. Handler (server side) — dùng message struct + implement interface

`internal/gapi/rpc_deck.go:45`:
```go
func (s *Server) CreateDeck(
    ctx context.Context,
    req *pb.CreateDeckRequest,                  // ← message struct sinh từ proto
) (*pb.CreateDeckResponse, error) {              // ← message struct sinh từ proto
    ...
    return &pb.CreateDeckResponse{
        Deck: dbDeckToPb(deck),
    }, nil
}
```

**Tác dụng**:
- Định nghĩa kiểu input/output cho mỗi RPC.
- Method signature phải khớp interface `pb.DeckServiceServer` — nếu thiếu method, Go compile fail.

#### 2. main.go — gọi `Register...` function

`cmd/server/main.go:112`:
```go
pb.RegisterDeckServiceServer(grpcServer, server)
```

`cmd/server/main.go:143`:
```go
pb.RegisterDeckServiceHandlerServer(ctx, grpcMux, srv)
```

**Tác dụng**:
- `RegisterDeckServiceServer` — gắn handler vào gRPC server runtime. Sau dòng này, khi có request gRPC đến `/DeckService/CreateDeck`, runtime biết gọi `server.CreateDeck()`.
- `RegisterDeckServiceHandlerServer` — tự sinh HTTP route dựa trên annotation trong `.proto`. Không cần tự viết route.

#### 3. Client (gọi service khác) — dùng client stub

`study-service/internal/deckclient/client.go:41`:
```go
import deckpb "mem_pan/services/deck-service/pb"   // import pb của service KHÁC

deckSvc: deckpb.NewDeckServiceClient(conn)         // tạo client stub

resp, err := c.deckSvc.ListDeckCards(ctx,
    &deckpb.ListDeckCardsRequest{
        DeckId: deckID.String(),
    })
```

**Tác dụng**:
- Gọi RPC sang service khác **như gọi hàm local**.
- Stub tự marshal request → protobuf bytes → gửi qua HTTP/2 → unmarshal response → trả struct Go.
- Type-safe — sai field là compile error.

### Sơ đồ trực quan

```
                         file proto (rpc_deck.proto)
                              │
                              │  protoc + plugins
                              ▼
        ┌─────────────────────┴──────────────────────┐
        │                                            │
        ▼                                            ▼
  rpc_deck.pb.go                          deck_service_grpc.pb.go
  (messages)                              (interfaces + stubs)
        │                                            │
        │ struct CreateDeckRequest                   │ interface DeckServiceServer  ◄─ handler implement
        │ struct CreateDeckResponse                  │ interface DeckServiceClient  ◄─ client gọi
        │                                            │ func RegisterDeckServiceServer
        │                                            │ func NewDeckServiceClient
        └────────────────┬───────────────────────────┘
                         │ được import ở 3 chỗ:
        ┌────────────────┼─────────────────────┐
        ▼                ▼                     ▼
┌────────────────┐ ┌──────────────────┐ ┌──────────────────────┐
│ HANDLER        │ │ MAIN (đăng ký)   │ │ CLIENT (service khác)│
│ gapi/rpc_deck  │ │ cmd/server/main  │ │ study-service/       │
│                │ │                  │ │ deckclient/client.go │
│ - input/output │ │ - Register...    │ │ - NewClient(conn)    │
│   là pb struct │ │   Server         │ │ - gọi RPC qua stub   │
│ - implement    │ │ - RegisterHandler│ │                      │
│   interface    │ │   (gateway)      │ │                      │
└────────────────┘ └──────────────────┘ └──────────────────────┘
```

### Tưởng tượng không có pb.go

Phải tự viết:
1. Struct cho mỗi request/response + tự viết code marshal/unmarshal protobuf binary.
2. Routing gRPC: tự parse path `/DeckService/CreateDeck` → dispatch đúng method.
3. HTTP gateway: tự parse JSON body, validate, map sang struct.
4. Client stub: tự mở HTTP/2 stream, gửi binary đúng format.

→ pb.go làm **toàn bộ công việc này**.

### Tóm tắt 1 dòng

> **`pb.go` = code "keo dán" tự động giữa file `.proto` và Go runtime**. Cung cấp struct (dữ liệu), interface (server implement), stub (client gọi), và hàm Register (gắn vào gRPC/HTTP runtime). Bạn chỉ viết logic; pb lo phần protocol.

---

## 7. Quy trình thêm 1 chức năng mới

### Câu trả lời ngắn

**`.proto` → `.sql` → repo → service → handler**.

Lý do: `.proto` định nghĩa "API contract" với client — quyết định cái này trước thì các tầng dưới mới có mục tiêu rõ ràng.

### Quy trình 10 bước

Ví dụ thêm RPC mới `ReportDeck` (user báo cáo 1 deck là spam):

#### Bước 1: Suy nghĩ — KHÔNG code

Hỏi 3 câu:
- **Client cần gì?** (input/output) → quyết định proto
- **Dữ liệu lưu ở đâu?** (table mới hay cũ?) → quyết định migration
- **Có publish event không?** (notify ai, log audit?) → quyết định publisher

#### Bước 2: `proto/rpc_report.proto` — define API trước

```protobuf
message ReportDeckRequest {
  string deck_id = 1;
  string reason  = 2;
}

message ReportDeckResponse {
  string report_id = 1;
}

service DeckService {
  rpc ReportDeck(ReportDeckRequest) returns (ReportDeckResponse) {
    option (google.api.http) = {
      post: "/v1/decks/{deck_id}/report"
      body: "*"
    };
  }
}
```

#### Bước 3: `make proto`

Sinh ra:
- `pb/rpc_report.pb.go` (struct request/response)
- `pb/deck_service_grpc.pb.go` (interface có thêm method `ReportDeck`)
- `pb/deck_service.pb.gw.go` (route `/v1/decks/{deck_id}/report`)
- `doc/swagger/deck_service.swagger.json` (cập nhật tự động)

**Lúc này Go compile fail** vì `gapi.Server` thiếu method `ReportDeck`. Đây là điều **tốt**.

#### Bước 4: `db/migration/00000X_add_reports.up.sql`

```sql
CREATE TABLE deck_reports (
  report_id  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  deck_id    UUID NOT NULL REFERENCES decks(deck_id),
  user_id    UUID NOT NULL,
  reason     TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Kèm file `.down.sql` để rollback.

#### Bước 5: `db/query/report.sql`

```sql
-- name: CreateReport :one
INSERT INTO deck_reports (deck_id, user_id, reason)
VALUES ($1, $2, $3)
RETURNING *;
```

#### Bước 6: `make sqlc`

Sinh ra:
- `internal/db/report.sql.go` với hàm `CreateReport(ctx, params)`
- `internal/db/models.go` cập nhật struct `DeckReport`

#### Bước 7: `internal/repository/report_repo.go` — viết tay

```go
type ReportRepository interface {
    CreateReport(ctx, arg) (db.DeckReport, error)
}

type reportRepository struct {
    q *db.Queries
}

func NewReportRepository(database *sql.DB) ReportRepository {
    return &reportRepository{q: db.New(database)}
}

func (r *reportRepository) CreateReport(ctx, arg) (db.DeckReport, error) {
    return r.q.CreateReport(ctx, arg)
}
```

#### Bước 8: `internal/service/report_service.go` — logic nghiệp vụ

```go
func (s *reportService) Report(ctx, userID, deckID, reason) (db.DeckReport, error) {
    // 1. validate reason không rỗng
    // 2. check deck tồn tại + chưa bị deleted
    // 3. check user không tự report deck của chính mình
    // 4. gọi repo.CreateReport
    // 5. publish event ReportSubmitted → admin-service log
    return ...
}
```

#### Bước 9: `internal/gapi/rpc_report.go` — handler

```go
func (s *Server) ReportDeck(
    ctx context.Context,
    req *pb.ReportDeckRequest,
) (*pb.ReportDeckResponse, error) {
    payload, err := s.authorizeUser(ctx)
    if err != nil { return nil, err }

    deckID, err := uuid.Parse(req.DeckId)
    if err != nil { return nil, status.Error(codes.InvalidArgument, "bad deck_id") }

    report, err := s.reportSvc.Report(ctx, payload.UserID, deckID, req.Reason)
    if err != nil { return nil, toGRPCError(err) }

    return &pb.ReportDeckResponse{ReportId: report.ReportID.String()}, nil
}
```

→ **Compile success.**

#### Bước 10: `cmd/server/main.go` — wire-up

```go
reportRepo := repository.NewReportRepository(database)
reportSvc := service.NewReportService(reportRepo, deckRepo, pub)
server := gapi.NewServer(..., reportSvc, ...)   // thêm vào constructor
```

Cũng phải thêm `reportSvc` vào struct `gapi.Server` (server.go).

#### Bước 11: Test

- Unit test cho service (mock `ReportRepository`).
- `make migrateup` để apply migration lên DB dev.
- Smoke test bằng curl: `POST /v1/decks/{id}/report`.

### Sơ đồ workflow

```
Suy nghĩ (input/output, data, events)
        │
        ▼
   .proto  ──── make proto ────►  pb/*.pb.go
        │                              │
        │                              │ compile fail → handler thiếu method
        │                              ▼
   .sql migration                  (sẽ fix ở bước cuối)
        │                              │
        ▼                              │
   .sql query  ──── make sqlc ────► internal/db/*.sql.go
        │                              │
        ▼                              │
   repository                          │
        │                              │
        ▼                              │
   service (logic, validate, events)   │
        │                              │
        ▼                              │
   handler (gapi)  ◄────────────────────┘
        │
        ▼
   wire-up trong main.go
        │
        ▼
   test + migrate + smoke test
```

### Một số ngoại lệ thực tế

#### Khi nào KHÔNG bắt đầu từ proto?

- **Thêm field vào RPC đã có**: vẫn sửa proto trước, nhưng workflow ngắn hơn.
- **Background job / cron** (vd FSRS optimizer định kỳ): không có RPC → bắt đầu từ SQL hoặc từ logic luôn.
- **Pub/Sub event consumer**: đi từ `events/types.go` (định nghĩa event struct) → handler subscribe → service → repo.
- **Endpoint multipart upload** (vd `POST /v1/cards/upload-image`): xem `rpc_card_http.go` — không qua proto vì grpc-gateway không xử lý multipart tốt; viết HTTP handler tay.

#### Khi nào bắt đầu từ SQL trước?

Chỉ khi đã chắc về data model và muốn explore: viết SQL → chạy thử trên DB → ổn → mới design API. Tuy nhiên production code đi theo hướng **API-first** (proto trước).

### Quy tắc vàng

> **Đi từ ngoài vào trong**: client thấy gì → API contract (`.proto`) → logic nghiệp vụ (service) → dữ liệu (SQL).
>
> Đừng đi từ trong ra ngoài (SQL trước rồi mới design API), vì dễ dẫn đến API "lộ schema DB" thay vì phản ánh nhu cầu thật của client.

### Checklist cho 1 chức năng mới

- [ ] `.proto` updated + `make proto`
- [ ] `.sql migration` (up + down)
- [ ] `.sql query` + `make sqlc`
- [ ] `repository` interface + impl
- [ ] `service` logic + validate + events (nếu có)
- [ ] `handler (gapi)` + map error
- [ ] Wire-up trong `cmd/server/main.go`
- [ ] Unit test service
- [ ] Migration apply: `make migrateup`
- [ ] Smoke test qua curl/Swagger
- [ ] Commit + PR

---

## Phụ lục — Tham khảo chéo

- Doc chi tiết moderation service: [`doc/moderation-fsrs-service.md`](./moderation-fsrs-service.md)
- Architecture tổng: [`doc/architecture.md`](./architecture.md)
- Event catalog: [`doc/event-catalog.md`](./event-catalog.md)
- Tech stack report: [`doc/tech-stack-report.md`](./tech-stack-report.md)
- ER diagrams: [`doc/er-diagrams.md`](./er-diagrams.md)
