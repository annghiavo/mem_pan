# Stats Service

Stats service là một **projection service** — không phải source of truth. Tất cả dữ liệu được build từ events qua Google Cloud Pub/Sub và phục vụ dashboard + heatmap với eventual consistency.

---

## Architecture

```
auth-service   ──┐
deck-service   ──┼──► Pub/Sub topics ──► stats-service subscriber ──► PostgreSQL projections
study-service  ──┘                              │
                                                └──► gRPC + HTTP API ──► client
```

- **Write path:** Event consumers cập nhật projection tables bất đồng bộ (delay vài giây là OK)
- **Read path:** gRPC handlers query thẳng từ projection tables, không join cross-service
- **Rebuild:** Có thể replay toàn bộ events từ Pub/Sub topic retention để build lại từ đầu

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
| `PUBSUB_PROJECT_ID` | ✅ | — | GCP project ID |
| `AUTH_SERVICE_ADDRESS` | | `localhost:9090` | Auth service gRPC address |
| `GRPC_SERVER_ADDRESS` | | `:9094` | gRPC listen address |
| `HTTP_SERVER_ADDRESS` | | `:8084` | HTTP gateway listen address |
| `PUBSUB_USER_EVENTS_SUB` | | `stats-user-events-sub` | Subscription cho user events |
| `PUBSUB_DECK_EVENTS_SUB` | | `stats-deck-events-sub` | Subscription cho deck events |
| `PUBSUB_STUDY_EVENTS_SUB` | | `stats-study-events-sub` | Subscription cho study events |

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

## Google Cloud Pub/Sub Setup

### Concepts

| Term | Ý nghĩa |
|------|---------|
| **Topic** | Kênh phát sự kiện. Publisher gửi message vào topic. |
| **Subscription** | Một consumer "đăng ký" vào topic. Mỗi subscription nhận độc lập bản sao của message. |
| **Message** | Payload dạng bytes + attributes. Stats service dùng JSON. |
| **Ack / Nack** | Ack = xử lý thành công, xóa khỏi queue. Nack = thất bại, redelivery sau. |
| **Retention** | Topic giữ message trong N ngày kể cả sau khi acked — dùng để replay lại toàn bộ lịch sử. |

### Pub/Sub flow trong project này

```
Publisher (auth/deck/study-service)          Consumer (stats-service)
─────────────────────────────────            ─────────────────────────────
topic: user-events          ──────────────►  subscription: stats-user-events-sub
topic: deck-events          ──────────────►  subscription: stats-deck-events-sub
topic: study-events         ──────────────►  subscription: stats-study-events-sub
```

Mỗi topic có thể có nhiều subscriptions — ví dụ `study-events` có thể đồng thời được subscribe bởi stats-service, worker-service, notification-service mà không ảnh hưởng lẫn nhau.

---

### 1. GCP Setup (Production)

#### Cài gcloud CLI

```bash
# macOS
brew install --cask google-cloud-sdk

# Hoặc download từ https://cloud.google.com/sdk/docs/install

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

#### Tạo subscriptions cho stats-service

```bash
# User events
gcloud pubsub subscriptions create stats-user-events-sub \
  --topic=user-events \
  --ack-deadline=60 \
  --message-retention-duration=7d

# Deck events
gcloud pubsub subscriptions create stats-deck-events-sub \
  --topic=deck-events \
  --ack-deadline=60 \
  --message-retention-duration=7d

# Study events (volume cao nhất)
gcloud pubsub subscriptions create stats-study-events-sub \
  --topic=study-events \
  --ack-deadline=60 \
  --message-retention-duration=7d
```

> **`--ack-deadline=60`**: Service có 60 giây để xử lý mỗi message trước khi Pub/Sub redeliver. Tăng lên nếu DB thường bị slow.

#### IAM permissions

Stats-service cần role `roles/pubsub.subscriber` trên project:

```bash
# Nếu chạy trên Cloud Run với service account riêng
gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
  --member="serviceAccount:stats-service@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/pubsub.subscriber"

# Publisher services (auth/deck/study) cần role publisher
gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
  --member="serviceAccount:auth-service@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/pubsub.publisher"
```

---

### 2. Local Development — Pub/Sub Emulator

Không cần kết nối GCP thật khi dev local. Dùng Pub/Sub emulator:

#### Cài đặt

```bash
gcloud components install pubsub-emulator
```

Hoặc dùng Docker:

```bash
docker run --rm -p 8085:8085 \
  google/cloud-sdk:latest \
  gcloud beta emulators pubsub start --host-port=0.0.0.0:8085
```

#### Chạy emulator và set env

```bash
# Terminal 1: chạy emulator
gcloud beta emulators pubsub start --host-port=localhost:8085

# Terminal 2: set env vars để Go SDK tự redirect sang emulator
export PUBSUB_EMULATOR_HOST=localhost:8085
export PUBSUB_PROJECT_ID=local-project
```

Khi `PUBSUB_EMULATOR_HOST` được set, `pubsub.NewClient()` tự động kết nối emulator — không cần thay code.

#### Tạo topics và subscriptions trên emulator

Emulator không giữ state sau khi restart, nên cần tạo lại mỗi lần. Tạo một script setup:

```bash
#!/bin/bash
# scripts/setup_pubsub_local.sh

export PUBSUB_EMULATOR_HOST=localhost:8085
PROJECT=local-project

# Tạo topics
curl -s -X PUT "http://localhost:8085/v1/projects/$PROJECT/topics/user-events"
curl -s -X PUT "http://localhost:8085/v1/projects/$PROJECT/topics/deck-events"
curl -s -X PUT "http://localhost:8085/v1/projects/$PROJECT/topics/study-events"

# Tạo subscriptions
curl -s -X PUT "http://localhost:8085/v1/projects/$PROJECT/subscriptions/stats-user-events-sub" \
  -H "Content-Type: application/json" \
  -d '{"topic":"projects/'"$PROJECT"'/topics/user-events","ackDeadlineSeconds":60}'

curl -s -X PUT "http://localhost:8085/v1/projects/$PROJECT/subscriptions/stats-deck-events-sub" \
  -H "Content-Type: application/json" \
  -d '{"topic":"projects/'"$PROJECT"'/topics/deck-events","ackDeadlineSeconds":60}'

curl -s -X PUT "http://localhost:8085/v1/projects/$PROJECT/subscriptions/stats-study-events-sub" \
  -H "Content-Type: application/json" \
  -d '{"topic":"projects/'"$PROJECT"'/topics/study-events","ackDeadlineSeconds":60}'

echo "Done. Topics and subscriptions created on emulator."
```

```bash
chmod +x scripts/setup_pubsub_local.sh
./scripts/setup_pubsub_local.sh
```

#### Publish test message thủ công

```bash
# Publish một card.reviewed event để test stats-service
curl -s -X POST \
  "http://localhost:8085/v1/projects/local-project/topics/study-events:publish" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [{
      "data": "'"$(echo -n '{
        "event_type": "card.reviewed",
        "data": {
          "user_id": "00000000-0000-0000-0000-000000000001",
          "card_id": "00000000-0000-0000-0000-000000000002",
          "deck_id": "00000000-0000-0000-0000-000000000003",
          "rating": 4,
          "duration_ms": 3000,
          "state_before": "new",
          "state_after": "learning",
          "stability_after": 1.5,
          "is_new_card": true,
          "review_time": "2026-05-12T10:00:00Z"
        }
      }' | base64 -w 0)"'"
    }]
  }'
```

---

### 3. Implement Publisher trong các services khác

Các services (auth, deck, study) cần publish events theo đúng format. Pattern chuẩn để thêm vào:

#### Tạo publisher helper

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
    _, err = result.Get(ctx) // blocks until ack from server
    return err
}
```

#### Sử dụng trong study-service sau khi user review card

```go
// Sau khi lưu revlog thành công, publish event
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
    // Log nhưng không fail request — stats là best-effort
    log.Printf("[publisher] card.reviewed: %v", err)
}
```

> **Quan trọng:** Lỗi publish **không nên** fail request của user. Stats là eventually consistent — nếu publish thất bại, event đó bị mất cho đến khi rebuild. Nack chỉ dùng phía consumer để retry.

---

### 4. Authentication với GCP

#### Trên Cloud Run (Production)

Cloud Run tự động inject service account credentials. Không cần config thêm gì — `pubsub.NewClient()` dùng [Application Default Credentials](https://cloud.google.com/docs/authentication/application-default-credentials) tự động.

#### Chạy local với GCP thật (không dùng emulator)

```bash
# Authenticate với user account (dùng khi dev)
gcloud auth application-default login

# Hoặc dùng service account key file
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account-key.json"
```

#### Dùng service account key (CI/CD)

```bash
# Tạo service account
gcloud iam service-accounts create stats-service \
  --display-name="Stats Service"

# Tạo key file
gcloud iam service-accounts keys create ./stats-service-key.json \
  --iam-account=stats-service@YOUR_PROJECT_ID.iam.gserviceaccount.com

# Grant permissions
gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
  --member="serviceAccount:stats-service@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/pubsub.subscriber"
```

---

### 5. Monitoring & Troubleshooting

#### Xem subscription backlog (số messages chưa xử lý)

```bash
gcloud pubsub subscriptions describe stats-study-events-sub
# Nhìn vào: numUndeliveredMessages
```

Hoặc qua Cloud Console: **Pub/Sub → Subscriptions → stats-study-events-sub → Metrics**

#### Xem dead-letter messages

Nếu muốn capture messages bị nack quá nhiều lần, tạo dead-letter topic:

```bash
gcloud pubsub topics create stats-dead-letter

gcloud pubsub subscriptions modify-push-config stats-study-events-sub \
  --dead-letter-topic=stats-dead-letter \
  --max-delivery-attempts=5
```

#### Seek (replay events từ một thời điểm)

Dùng khi cần rebuild projections từ một mốc thời gian cụ thể:

```bash
# Seek subscription về 24 giờ trước (replay lại tất cả events trong 24h)
gcloud pubsub subscriptions seek stats-study-events-sub \
  --time="2026-05-11T00:00:00Z"
```

Sau khi seek, stats-service sẽ nhận lại tất cả messages từ thời điểm đó. Vì queries dùng `ON CONFLICT DO UPDATE`, việc replay sẽ idempotent với hầu hết events.

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
