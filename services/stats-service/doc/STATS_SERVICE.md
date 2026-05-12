# Stats Service

Stats service là một **projection service** — không phải source of truth. Tất cả dữ liệu được build từ events qua Google Cloud Pub/Sub và phục vụ dashboard + heatmap với eventual consistency.

---

## Architecture

```
auth-service   ──┐                         POST /internal/pubsub
deck-service   ──┼──► Pub/Sub topics ──► (Google calls stats-service) ──► PostgreSQL projections
study-service  ──┘        push ↑                                                    │
                     subscriptions                                                   │
                                                                  gRPC + HTTP API ──► client
```

Dùng **Pub/Sub push subscription** — stats-service không cần poll. Khi có message mới, Google Pub/Sub chủ động gọi `POST /internal/pubsub` trên stats-service. Stats-service trả về `204` để ACK, hoặc `5xx` để NACK (Pub/Sub sẽ retry với exponential back-off).

- **Write path:** Pub/Sub gọi push endpoint → handler cập nhật projections bất đồng bộ
- **Read path:** gRPC handlers query thẳng từ projection tables, không join cross-service
- **Rebuild:** Seek subscription về thời điểm cũ → Pub/Sub replay lại toàn bộ messages

---

## Ports

| Protocol | Default |
|----------|---------|
| gRPC     | `:9094` |
| HTTP     | `:8084` |

---

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | ✅ | — | PostgreSQL connection string |
| `AUTH_SERVICE_ADDRESS` | | `localhost:9090` | Auth service gRPC address |
| `GRPC_SERVER_ADDRESS` | | `:9094` | gRPC listen address |
| `HTTP_SERVER_ADDRESS` | | `:8084` | HTTP gateway listen address |
| `PUBSUB_PUSH_SECRET` | | _(empty)_ | Shared secret appended as `?token=` on push endpoint URL — nên set trong production |

> Không cần `PUBSUB_PROJECT_ID` hay subscription names nữa. Stats-service chỉ là HTTP server thụ động — Pub/Sub tự biết endpoint từ config subscription trên GCP.

---

## Database Schema

Tất cả tables là **projections** — không phải source of truth.

### `user_stats`
Stats tổng per user. Aggregates từ tất cả events liên quan đến user.

| Column | Type | Description |
|--------|------|-------------|
| `user_id` | UUID PK | Reference auth_db.users |
| `total_cards` | INTEGER | Tổng số cards đã tạo |
| `total_reviews` | INTEGER | Tổng số lần review |
| `total_study_time_ms` | BIGINT | Tổng thời gian học (ms) |
| `current_streak` | INTEGER | Streak hiện tại (ngày) |
| `longest_streak` | INTEGER | Streak dài nhất từ trước đến nay |
| `last_studied_date` | DATE | Ngày học gần nhất |
| `total_correct` | INTEGER | Số lần rating ≥ 3 |
| `total_incorrect` | INTEGER | Số lần rating < 3 |
| `username` | VARCHAR | Denormalized từ auth-service |
| `avatar_url` | TEXT | Denormalized từ auth-service |

### `daily_stats`
Dữ liệu từng ngày cho heatmap. Primary key là `(user_id, study_date)`.

| Column | Type | Description |
|--------|------|-------------|
| `user_id` | UUID | |
| `study_date` | DATE | |
| `reviews_count` | INTEGER | Số reviews trong ngày |
| `new_cards_count` | INTEGER | Số cards mới học trong ngày |
| `study_time_ms` | BIGINT | Thời gian học trong ngày (ms) |
| `correct_count` | INTEGER | Số đúng trong ngày |

### `deck_stats`
Stats per deck. Denormalized `deck_name` để tránh join cross-service khi hiển thị UI.

| Column | Type | Description |
|--------|------|-------------|
| `deck_id` | UUID PK | Reference deck_db.decks |
| `user_id` | UUID | Owner |
| `total_cards` | INTEGER | Tổng số cards trong deck |
| `new_cards` | INTEGER | Cards chưa học lần nào |
| `learning_cards` | INTEGER | Cards đang trong giai đoạn learning/relearning |
| `review_cards` | INTEGER | Cards đang trong giai đoạn review |
| `mastered_cards` | INTEGER | Cards có stability > 21 ngày |
| `due_today` | INTEGER | Cards cần review hôm nay |
| `deck_name` | VARCHAR | Denormalized từ deck-service |

### `deck_progress_snapshots`
Snapshot hàng ngày của card state distribution per deck, dùng cho biểu đồ progress over time. Primary key là `(deck_id, user_id, snapshot_date)`.

| Column | Type | Description |
|--------|------|-------------|
| `deck_id` | UUID | |
| `user_id` | UUID | |
| `snapshot_date` | DATE | |
| `new_count` | INTEGER | |
| `learning_count` | INTEGER | |
| `review_count` | INTEGER | |
| `mastered_count` | INTEGER | |

---

## API Endpoints

### `GET /v1/stats/me`
Lấy tổng stats của current user (dashboard).

**Response:**
```json
{
  "stats": {
    "user_id": "...",
    "total_cards": 120,
    "total_reviews": 850,
    "total_study_time_ms": 3600000,
    "current_streak": 7,
    "longest_streak": 14,
    "last_studied_date": "2026-05-12",
    "total_correct": 700,
    "total_incorrect": 150,
    "username": "anvo",
    "avatar_url": "https://...",
    "updated_at": "2026-05-12T10:00:00Z"
  }
}
```

---

### `GET /v1/stats/me/heatmap?from_date=2026-01-01&to_date=2026-05-12`
Dữ liệu heatmap theo ngày trong khoảng thời gian.

**Query params:**
- `from_date` — YYYY-MM-DD (default: 1 năm trước)
- `to_date` — YYYY-MM-DD (default: hôm nay)

**Response:**
```json
{
  "entries": [
    {
      "study_date": "2026-05-11",
      "reviews_count": 30,
      "new_cards_count": 5,
      "study_time_ms": 900000,
      "correct_count": 25
    }
  ]
}
```

---

### `GET /v1/stats/me/decks`
List stats của tất cả decks thuộc current user.

**Response:**
```json
{
  "decks": [
    {
      "deck_id": "...",
      "user_id": "...",
      "total_cards": 50,
      "new_cards": 20,
      "learning_cards": 10,
      "review_cards": 15,
      "mastered_cards": 5,
      "due_today": 8,
      "deck_name": "JLPT N3",
      "updated_at": "2026-05-12T10:00:00Z"
    }
  ]
}
```

---

### `GET /v1/stats/decks/{deck_id}`
Stats của một deck cụ thể. Chỉ owner mới có quyền truy cập.

---

### `GET /v1/stats/decks/{deck_id}/progress?from_date=2026-01-01&to_date=2026-05-12`
Progress timeline của deck theo ngày, dùng cho biểu đồ stacked bar/area.

**Response:**
```json
{
  "entries": [
    {
      "snapshot_date": "2026-05-01",
      "new_count": 30,
      "learning_count": 8,
      "review_count": 10,
      "mastered_count": 2
    }
  ]
}
```

---

## Event Contracts

Stats service **subscribe** các events sau. Các services khác phải publish đúng format này.

### Envelope format
Tất cả messages gửi lên Pub/Sub phải wrap trong envelope:
```json
{
  "event_type": "card.reviewed",
  "data": { ... }
}
```

### `user.registered` — từ auth-service
```json
{
  "user_id": "uuid",
  "username": "anvo",
  "email": "annghiavo@gmail.com",
  "avatar_url": "https://...",
  "created_at": "2026-05-12T10:00:00Z"
}
```
→ Tạo row trong `user_stats`.

---

### `deck.created` — từ deck-service
```json
{
  "deck_id": "uuid",
  "user_id": "uuid",
  "deck_name": "JLPT N3",
  "created_at": "2026-05-12T10:00:00Z"
}
```
→ Tạo row trong `deck_stats`.

---

### `deck.updated` — từ deck-service
```json
{
  "deck_id": "uuid",
  "user_id": "uuid",
  "deck_name": "JLPT N3 (updated)"
}
```
→ Cập nhật `deck_name` trong `deck_stats`.

---

### `card.created` — từ deck-service
```json
{
  "card_id": "uuid",
  "deck_id": "uuid",
  "user_id": "uuid",
  "created_at": "2026-05-12T10:00:00Z"
}
```
→ Tăng `total_cards` + `new_cards` trong `deck_stats`, tăng `total_cards` trong `user_stats`.

---

### `card.reviewed` — từ study-service
Event quan trọng nhất. Trigger cập nhật tất cả projection tables.

```json
{
  "user_id": "uuid",
  "card_id": "uuid",
  "deck_id": "uuid",
  "rating": 3,
  "duration_ms": 5000,
  "state_before": "new",
  "state_after": "learning",
  "stability_after": 1.5,
  "is_new_card": true,
  "review_time": "2026-05-12T10:00:00Z"
}
```

**Xử lý:**
1. Tăng `total_reviews`, `total_study_time_ms`, `total_correct`/`total_incorrect` trong `user_stats`
2. Tính và cập nhật streak
3. Upsert `daily_stats` cho ngày hôm đó
4. Dịch chuyển card state counts trong `deck_stats` theo `state_before → state_after`
5. Upsert `deck_progress_snapshots` cho ngày hôm đó

---

## Streak Logic

```
last_studied_date == hôm nay  →  streak không đổi (đã tính rồi)
last_studied_date == hôm qua  →  streak + 1
last_studied_date < hôm qua   →  streak = 1 (reset)
chưa học lần nào              →  streak = 1
```

---

## Card State Transition Logic

Dùng FSRS states: `new`, `learning`, `relearning`, `review`.

Khi nhận `card.reviewed`:
- Decrement bucket của `state_before`
- Increment bucket của `state_after`
- `relearning` được gộp vào `learning_cards`
- Card được tính là **mastered** khi `state_after == "review"` và `stability_after ≥ 21` (ngày)

Các giá trị âm được bảo vệ bằng `GREATEST(0, ...)` ở tầng SQL.

---

## Google Cloud Pub/Sub — Push Model

Thay vì stats-service chủ động poll Pub/Sub, **Google Pub/Sub chủ động gọi vào** stats-service mỗi khi có message mới. Stats-service chỉ cần expose một HTTP endpoint và không cần Pub/Sub SDK.

```
Pub/Sub topic có message mới
        │
        ▼
Google gọi POST https://stats-service/internal/pubsub?token=SECRET
        │
        ▼
PushHandler decode message → Dispatch → DB
        │
        ▼
Trả 204 No Content  →  Google xác nhận đã nhận (ACK)
Trả 5xx             →  Google retry với exponential back-off (NACK)
```

### Concepts

| Term | Ý nghĩa |
|------|---------|
| **Topic** | Kênh phát sự kiện. Publisher gửi message vào đây. |
| **Push Subscription** | Pub/Sub gọi một URL cụ thể mỗi khi có message mới. Khác với pull subscription — consumer không cần poll. |
| **ACK** | HTTP 2xx — message được xử lý thành công, Pub/Sub xóa khỏi queue. |
| **NACK** | HTTP 5xx hoặc timeout — message bị redelivery sau một khoảng thời gian (retry). |
| **Retention** | Topic giữ message trong N ngày kể cả sau khi acked — dùng để seek (replay) lại lịch sử. |

---

### 1. GCP Setup (Production)

#### Cài gcloud CLI

```bash
# macOS
brew install --cask google-cloud-sdk

gcloud auth login
gcloud config set project YOUR_PROJECT_ID
```

#### Tạo topics

```bash
gcloud pubsub topics create user-events
gcloud pubsub topics create deck-events
gcloud pubsub topics create study-events
```

Thêm retention để có thể replay events (khuyến nghị 7 ngày):

```bash
gcloud pubsub topics update user-events  --message-retention-duration=7d
gcloud pubsub topics update deck-events  --message-retention-duration=7d
gcloud pubsub topics update study-events --message-retention-duration=7d
```

#### Tạo push subscriptions trỏ vào stats-service

Mỗi subscription dùng cùng một endpoint. `event_type` trong body phân biệt loại event.

```bash
PUSH_ENDPOINT="https://stats-service.example.com/internal/pubsub?token=YOUR_SECRET"

gcloud pubsub subscriptions create stats-user-events-sub \
  --topic=user-events \
  --push-endpoint="$PUSH_ENDPOINT" \
  --ack-deadline=60 \
  --message-retention-duration=7d

gcloud pubsub subscriptions create stats-deck-events-sub \
  --topic=deck-events \
  --push-endpoint="$PUSH_ENDPOINT" \
  --ack-deadline=60 \
  --message-retention-duration=7d

gcloud pubsub subscriptions create stats-study-events-sub \
  --topic=study-events \
  --push-endpoint="$PUSH_ENDPOINT" \
  --ack-deadline=60 \
  --message-retention-duration=7d
```

> **`--ack-deadline=60`**: Nếu stats-service không trả về response trong 60 giây, Pub/Sub coi là NACK và retry. Tăng lên nếu DB thường chậm.

> **`?token=YOUR_SECRET`**: Set giá trị này trùng với `PUBSUB_PUSH_SECRET` trong env của stats-service.

#### IAM — cho phép Pub/Sub gọi stats-service (Cloud Run)

Nếu stats-service deploy trên Cloud Run với authentication bật, cần cấp quyền cho Pub/Sub service account:

```bash
# Lấy Pub/Sub service account của project
PROJECT_NUMBER=$(gcloud projects describe YOUR_PROJECT_ID --format='value(projectNumber)')
PUBSUB_SA="service-${PROJECT_NUMBER}@gcp-sa-pubsub.iam.gserviceaccount.com"

# Cấp quyền invoke Cloud Run
gcloud run services add-iam-policy-binding stats-service \
  --region=YOUR_REGION \
  --member="serviceAccount:$PUBSUB_SA" \
  --role="roles/run.invoker"
```

#### IAM — publisher services

```bash
# auth-service, deck-service, study-service cần publish vào topics
gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
  --member="serviceAccount:auth-service@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/pubsub.publisher"

gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
  --member="serviceAccount:deck-service@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/pubsub.publisher"

gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
  --member="serviceAccount:study-service@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/pubsub.publisher"
```

---

### 2. Push Message Format

Khi Pub/Sub gọi endpoint, body có dạng:

```json
{
  "message": {
    "data": "<base64-encoded bytes>",
    "messageId": "1234567890",
    "publishTime": "2026-05-12T10:00:00Z",
    "attributes": {}
  },
  "subscription": "projects/my-project/subscriptions/stats-study-events-sub"
}
```

`message.data` là base64 của JSON `events.Envelope`:

```json
{
  "event_type": "card.reviewed",
  "data": { "user_id": "...", "rating": 4, ... }
}
```

Stats-service tự decode base64 → unmarshal envelope → dispatch theo `event_type`.

---

### 3. Local Development

Pub/Sub push yêu cầu URL công khai (HTTPS). Có hai lựa chọn khi dev local:

#### Lựa chọn A — ngrok (đơn giản nhất)

```bash
# Cài ngrok: https://ngrok.com/download
ngrok http 8084
# ngrok sẽ in ra: Forwarding https://abc123.ngrok.io -> localhost:8084
```

Dùng URL ngrok để tạo push subscription trên GCP:

```bash
gcloud pubsub subscriptions modify-push-config stats-study-events-sub \
  --push-endpoint="https://abc123.ngrok.io/internal/pubsub?token=dev-secret"
```

#### Lựa chọn B — Pub/Sub Emulator (không cần internet)

Emulator hỗ trợ push. Chạy emulator và tạo subscription với `pushEndpoint` trỏ vào localhost:

```bash
# Terminal 1: chạy emulator
docker run --rm -p 8085:8085 google/cloud-sdk \
  gcloud beta emulators pubsub start --host-port=0.0.0.0:8085

# Terminal 2: tạo topics và push subscriptions
PROJECT=local-project
PUSH_URL="http://host.docker.internal:8084/internal/pubsub"

# Tạo topics
curl -s -X PUT "http://localhost:8085/v1/projects/$PROJECT/topics/user-events"
curl -s -X PUT "http://localhost:8085/v1/projects/$PROJECT/topics/deck-events"
curl -s -X PUT "http://localhost:8085/v1/projects/$PROJECT/topics/study-events"

# Tạo push subscriptions
curl -s -X PUT "http://localhost:8085/v1/projects/$PROJECT/subscriptions/stats-user-events-sub" \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"projects/$PROJECT/topics/user-events\",\"pushConfig\":{\"pushEndpoint\":\"$PUSH_URL\"},\"ackDeadlineSeconds\":60}"

curl -s -X PUT "http://localhost:8085/v1/projects/$PROJECT/subscriptions/stats-deck-events-sub" \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"projects/$PROJECT/topics/deck-events\",\"pushConfig\":{\"pushEndpoint\":\"$PUSH_URL\"},\"ackDeadlineSeconds\":60}"

curl -s -X PUT "http://localhost:8085/v1/projects/$PROJECT/subscriptions/stats-study-events-sub" \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"projects/$PROJECT/topics/study-events\",\"pushConfig\":{\"pushEndpoint\":\"$PUSH_URL\"},\"ackDeadlineSeconds\":60}"
```

#### Test thủ công — gọi push endpoint trực tiếp

Không cần Pub/Sub emulator, có thể giả lập push bằng `curl`:

```bash
# Tạo payload: base64-encode event envelope
PAYLOAD=$(echo -n '{"event_type":"card.reviewed","data":{"user_id":"00000000-0000-0000-0000-000000000001","card_id":"00000000-0000-0000-0000-000000000002","deck_id":"00000000-0000-0000-0000-000000000003","rating":4,"duration_ms":3000,"state_before":"new","state_after":"learning","stability_after":1.5,"is_new_card":true,"review_time":"2026-05-12T10:00:00Z"}}' | base64 -w 0)

curl -s -X POST "http://localhost:8084/internal/pubsub?token=dev-secret" \
  -H "Content-Type: application/json" \
  -d "{
    \"message\": {
      \"data\": \"$PAYLOAD\",
      \"messageId\": \"test-001\",
      \"publishTime\": \"2026-05-12T10:00:00Z\"
    },
    \"subscription\": \"projects/local/subscriptions/stats-study-events-sub\"
  }"

# Kết quả mong đợi: HTTP 204 No Content
```

---

### 4. Implement Publisher trong các services khác

Các services (auth, deck, study) cần publish events. Stats-service không cần Pub/Sub SDK — chỉ publishers cần.

#### Publisher helper

```go
// internal/publisher/pubsub.go (trong auth-service, deck-service, study-service)
package publisher

import (
    "context"
    "encoding/json"

    "cloud.google.com/go/pubsub"
)

type Envelope struct {
    EventType string          `json:"event_type"`
    Data      json.RawMessage `json:"data"`
}

type PubSubPublisher struct {
    topic *pubsub.Topic
}

func NewPubSubPublisher(client *pubsub.Client, topicID string) *PubSubPublisher {
    return &PubSubPublisher{topic: client.Topic(topicID)}
}

func (p *PubSubPublisher) Publish(ctx context.Context, eventType string, payload any) error {
    data, err := json.Marshal(payload)
    if err != nil {
        return err
    }
    env := Envelope{EventType: eventType, Data: data}
    body, err := json.Marshal(env)
    if err != nil {
        return err
    }
    result := p.topic.Publish(ctx, &pubsub.Message{Data: body})
    _, err = result.Get(ctx)
    return err
}
```

#### Sử dụng trong study-service

```go
err = publisher.Publish(ctx, "card.reviewed", events.CardReviewed{
    UserID:         userID.String(),
    CardID:         cardID.String(),
    DeckID:         deckID.String(),
    Rating:         int32(rating),
    DurationMs:     durationMs,
    StateBefore:    string(stateBefore),
    StateAfter:     string(stateAfter),
    StabilityAfter: stabilityAfter,
    IsNewCard:      isNewCard,
    ReviewTime:     time.Now().UTC(),
})
if err != nil {
    // Không fail request — stats là best-effort, eventual consistency
    log.Printf("[publisher] card.reviewed: %v", err)
}
```

---

### 5. Monitoring & Troubleshooting

#### Xem subscription backlog

```bash
gcloud pubsub subscriptions describe stats-study-events-sub
# Nhìn vào numUndeliveredMessages
```

Cloud Console: **Pub/Sub → Subscriptions → Metrics tab**

#### Dead-letter topic (capture messages bị nack liên tục)

```bash
gcloud pubsub topics create stats-dead-letter

gcloud pubsub subscriptions update stats-study-events-sub \
  --dead-letter-topic=stats-dead-letter \
  --max-delivery-attempts=5
```

#### Seek — replay events từ một thời điểm

Dùng khi cần rebuild projections từ đầu:

```bash
# Replay lại 24 giờ qua
gcloud pubsub subscriptions seek stats-study-events-sub \
  --time="2026-05-11T00:00:00Z"
```

Pub/Sub sẽ push lại tất cả messages từ thời điểm đó. Vì queries dùng `ON CONFLICT DO UPDATE`, replay là idempotent với hầu hết events.

---

## Running Locally

```bash
# 1. Tạo database và chạy migrations
make migrateup

# 2. Copy và điền env vars
cp app.env.example app.env

# 3. Chạy service
make run
```

**`app.env` cần có:**
```
DATABASE_URL=postgres://user:pass@localhost:5432/stats_db?sslmode=disable
PUBSUB_PROJECT_ID=your-gcp-project
AUTH_SERVICE_ADDRESS=localhost:9090
```

**Swagger UI:** http://localhost:8084/swagger/

---

## Makefile Commands

| Command | Description |
|---------|-------------|
| `make proto` | Tái sinh pb/ từ proto/stats_service.proto |
| `make sqlc` | Tái sinh internal/db/ từ db/query/*.sql |
| `make migrateup` | Chạy tất cả migrations |
| `make migratedown` | Rollback tất cả migrations |
| `make run` | Chạy service |
| `make evans` | Kết nối gRPC reflection qua Evans |
