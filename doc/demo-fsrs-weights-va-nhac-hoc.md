# Demo: Tối ưu 21 trọng số FSRS & Thông báo nhắc học

Tài liệu này giải thích **khi nào** hai chức năng dưới đây chạy, **luồng code cụ thể**,
và **cách demo** chúng khi bảo vệ đồ án. Mọi đường dẫn file đều tính từ gốc repo `mem_pan/`.

> **TL;DR cho người vội**
> - **Nhắc học**: hoàn chỉnh, chạy thật. Cloud Scheduler tick mỗi 15 phút → notification-service
>   gửi FCM cho user nào (a) có thẻ tới hạn, (b) đang đúng khung giờ, (c) chưa được nhắc hôm nay.
> - **Tối ưu 21 trọng số**: optimizer (Python) chạy thật, và nay **đã có cron 24h tự động gọi nó**
>   (xem [Phần 1.7](#17-cron-tối-ưu-24h-đã-triển-khai)). study-service mỗi ngày quét user có
>   ≥ `FSRS_OPTIMIZE_MIN_REVIEWS` review → re-tune → lưu version mới active.

---

## Phần 1 — 21 trọng số FSRS được tối ưu khi nào?

### 1.1 Tóm tắt nhanh

| Câu hỏi | Trả lời |
|---|---|
| 21 trọng số lưu ở đâu? | Bảng `user_fsrs_weights`, cột `weights DOUBLE PRECISION[21]` |
| Khi review thẻ có dùng trọng số riêng của user không? | **Có** — `ReviewCard` đọc bộ active để tính lịch ôn |
| Optimizer (train lại trọng số) có chạy thật không? | **Có** — `moderation-fsrs-service` (Python), RPC `OptimizeWeights` |
| Có gì TỰ ĐỘNG gọi optimize không? | **KHÔNG** — không cron / subscriber / endpoint / threshold |
| Hệ quả thực tế? | User dùng mãi bộ trọng số mặc định, chưa bao giờ được re-tune |

### 1.2 Nơi lưu trữ

**Bảng** — `services/study-service/db/migration/000001_init.up.sql:94`

```sql
CREATE TABLE user_fsrs_weights (
    user_id            UUID NOT NULL,
    version            INTEGER NOT NULL DEFAULT 1,
    weights            DOUBLE PRECISION[] NOT NULL DEFAULT
        '{0.212,1.2931,2.3065,8.2956,6.4133,0.8334,3.0194,0.001,1.8722,
          0.1666,0.796,1.4835,0.0614,0.2629,1.6483,0.6014,1.8729,0.5425,
          0.0912,0.0658,0.1542}'::double precision[],   -- 21 số = FSRS-6 default
    is_active          BOOLEAN NOT NULL DEFAULT TRUE,
    trained_on_reviews INTEGER,            -- số review đã dùng để train
    training_loss      DOUBLE PRECISION,   -- loss sau khi train
    created_at         TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, version)
);
```

- Mỗi user một (hoặc nhiều) bản ghi, `version` tăng dần mỗi lần train; chỉ một bản `is_active = TRUE`.
- `trained_on_reviews` + `training_loss` là metadata của lần train sinh ra bộ trọng số đó.

### 1.3 Trọng số được DÙNG khi nào (phần này đã chạy)

Mỗi lần user review một thẻ, `ReviewCard` nạp bộ trọng số active để FSRS tính ngày ôn kế tiếp.

`services/study-service/internal/service/study_service.go:294`

```go
params := fsrs.DefaultParams()
weights, err := s.weightsRepo.GetActiveWeights(ctx, p.UserID)
if err == nil && len(weights.Weights) == 21 {
    params = fsrs.ParamsFromWeights([]float64(weights.Weights))   // dùng trọng số riêng
}
```

Hàm chuyển 21 số → tham số FSRS: `services/study-service/internal/fsrs/weights.go:13` (chỉ nhận khi `len == 21`).

### 1.4 Trọng số được TỐI ƯU như thế nào (optimizer chạy thật)

**RPC** — `services/moderation-fsrs-service/proto/moderation_fsrs.proto:60`

```proto
service FsrsOptimizationService {
  rpc OptimizeWeights(OptimizeWeightsRequest) returns (OptimizeWeightsResponse);
}
message ReviewLog {
  string card_id     = 1;
  int64  review_date = 2;   // unix seconds
  int32  rating      = 3;   // 1..4
  int32  elapsed_days = 4;
}
message OptimizeWeightsRequest { string user_id = 1; repeated ReviewLog review_logs = 2; }
message OptimizeWeightsResponse {
  string user_id = 1;
  repeated float weights = 2;       // 17/19/21 số tuỳ version thư viện
  int32  num_reviews_used = 3;
  float  loss = 4;
  string fsrs_version = 5;
}
```

**Implementation** — `services/moderation-fsrs-service/app/services/fsrs_servicer.py:46`

1. Validate `review_logs` không rỗng.
2. Map review logs → DataFrame.
3. Chạy `fsrs_optimizer.Optimizer().train(df)` trong **ProcessPoolExecutor** (không block event loop), ~10–60s.
4. Trả về `weights + loss + fsrs_version`.

### 1.5 Lịch sử: trước đây thiếu trigger

Ở phiên bản đầu, optimizer chạy được nhưng **không service nào gọi `OptimizeWeights`** — không cron,
không subscriber, không endpoint, không ngưỡng N. `ReviewCard` chỉ publish `card.reviewed`. Hệ quả:
trọng số user không bao giờ được cập nhật.

Mắt xích này **nay đã nối** bằng cron 24h trong study-service — xem
[Phần 1.7](#17-cron-tối-ưu-24h-đã-triển-khai). Kết quả train được lưu qua
`fsrs_weights_repo.go`: `DeactivateWeights(user_id)` → `InsertWeights({version+1, weights, trained_on_reviews, training_loss})`.

### 1.6 Sơ đồ luồng (sau khi nối cron)

```
Cloud Scheduler (mỗi 24h, HTTP)
   └─ POST /internal/fsrs/optimize  (X-Cron-Secret + OIDC)
        └─ study-service: fsrsopt.RunOnce
             ├─ ListUsersWithMinReviews(N)            ── user nào đủ N review
             └─ foreach user:
                  ├─ ListReviewLogsForOptimize
                  ├─ moderationClient.OptimizeWeights ──► moderation-fsrs (Python) train()
                  │                                   ◄── weights(21) + loss + version
                  └─ DeactivateWeights + InsertWeights(active, trained_on_reviews, loss)
                       └─ lần ReviewCard kế tiếp dùng ngay trọng số mới  ✅
```

### 1.7 Cron tối ưu 24h (ĐÃ triển khai)

| Thành phần | Vị trí |
|---|---|
| Proto client (mirror) | `services/study-service/proto/fsrs_optimization.proto` → `pb/fsrs_optimization*.pb.go` |
| gRPC client | `services/study-service/internal/moderationclient/client.go` |
| Orchestrator | `services/study-service/internal/fsrsopt/optimizer.go` (`RunOnce`) |
| Queries | `db/query/revlog.sql`: `ListUsersWithMinReviews`, `ListReviewLogsForOptimize` |
| HTTP trigger | `cmd/server/main.go` `fsrsOptimizeHandler` → `POST /internal/fsrs/optimize` |
| Cloud Scheduler | `deploy/pubsub-setup/cloud-scheduler.sh` job `cron-fsrs-optimize` (mặc định 18:00 UTC) |

**Vì sao là HTTP job (không phải Pub/Sub như nhắc học)?** study-service không có push subscriber;
nó đã sẵn HTTP server. Cloud Run scale-to-zero nên không thể dùng ticker nội bộ — phải có trigger
ngoài đánh thức service. Cloud Scheduler HTTP + OIDC (`run.invoker`) là chuẩn GCP, ít thành phần nhất.

**Cấu hình (env của study-service):**

| Env | Mặc định | Ý nghĩa |
|---|---|---|
| `MODERATION_SERVICE_ADDRESS` | "" | gRPC moderation-fsrs; rỗng → cron tắt, endpoint trả 503 |
| `FSRS_OPTIMIZE_MIN_REVIEWS` | `1000` | Ngưỡng N (xem dưới) |
| `FSRS_OPTIMIZE_MAX_USERS` | `200` | Trần số user mỗi lần chạy (0 = không trần) |
| `CRON_SECRET` | "" | Bí mật bắt buộc ở header `X-Cron-Secret` |

#### Vì sao N = 1000?

| Mức N | Đánh giá |
|---|---|
| **< ~400** | Optimizer **overfit** — bộ trọng số học ra có thể tệ hơn mặc định. Tránh. |
| **~1000** | **Khuyến nghị** của FSRS để re-tune đáng tin cậy (cũng là ngưỡng Anki dùng). Mặc định của code. |
| **demo (50–100)** | Quá ít để chính xác, nhưng đủ để cron **có việc làm** khi demo. Đặt qua env. |

→ Code để mặc định **1000** (đúng về mặt học thuật), nhưng **N là env-configurable**. Khi bảo vệ,
hạ `FSRS_OPTIMIZE_MIN_REVIEWS=50` để cron thực sự tối ưu cho user test.

> **Lưu ý độ dài trọng số:** orchestrator chỉ ghi đè khi optimizer trả về đúng **21** số
> (khớp `go-fsrs/v4`). `fsrs-optimizer==5.5.0` cho FSRS-6 (21) nên khớp; nếu version khác trả 19,
> bản ghi bị **skip** (log `skipped`) thay vì lưu trọng số mà `ReviewCard` sẽ bỏ qua.

---

## Phần 2 — Thông báo nhắc học được gửi khi nào?

### 2.1 Tóm tắt nhanh

Chức năng **hoàn chỉnh và chạy thật**. Cloud Scheduler `cron-study-reminder` tick mỗi 15 phút (UTC)
→ publish topic `cron-study-reminder` → notification-service xử lý, fan-out tối đa 32 user song song.

Một user **nhận push** chỉ khi đủ **cả 4** điều kiện:

| # | Điều kiện | Kiểm tra ở đâu |
|---|---|---|
| 1 | Có ≥ 1 thẻ tới hạn (`due_count > 0`) | gọi study-service `CountDueForUser` |
| 2 | Giờ local đang trong cửa sổ `[sendTime−15p, sendTime]` | `inWindow`, theo timezone user |
| 3 | Tính được `sendTime` (từ optimal_hour hoặc fallback) | `pickOptimalHour` / `reminder_local_time` |
| 4 | Chưa gửi `study_reminder` nào hôm nay (dedup) | đếm trong `notification_log` |

### 2.2 Luồng code

| Bước | File:dòng | Vai trò |
|---|---|---|
| Nhận push Pub/Sub | `notification-service/internal/subscriber/subscriber.go:33` | decode envelope, dispatch theo `event_type` |
| Lấy thời điểm tick | `.../subscriber/handler.go:166` `decodeTickTime` | đọc trường `now` trong payload (mặc định = `time.Now()` nếu rỗng) |
| Entry tick | `.../scheduler/scheduler.go` `HandleStudyReminderTick` | lấy danh sách user (`ListReminderState`), fan-out |
| Quyết định / user | `.../scheduler/scheduler.go` `runStudyReminderForUser` | chạy 4 điều kiện ở trên |
| Gửi FCM | `.../scheduler/scheduler.go` `dispatch` → `internal/fcm/sender.go:56` | multicast tới mọi device token của user |

### 2.3 Cách tính `sendTime` (quan trọng để demo)

`const TickWindow = 15 * time.Minute` (`scheduler.go:40`)

```
optimalHour = pickOptimalHour(user_stats, localNow)       // tách weekday / weekend
            = (reminder_hour - 1)   nếu optimal_hour chưa được tính (= -1)   ← fallback
sendTime    = today @ optimalHour:00  (local)  − 15 phút
            = (reminder_hour − 2) : 45    (khi dùng fallback)
window      = [sendTime − 15p, sendTime]      (inWindow: cả 2 đầu inclusive)
```

Ví dụ: `reminder_local_time = "21:00"`, tz `Asia/Bangkok` → `optimalHour = 20` → `sendTime = 19:45`,
cửa sổ gửi là **[19:30, 19:45] giờ Bangkok**.

> Nếu `optimal_hour_weekday`/`optimal_hour_weekend` trong `user_stats` đã có giá trị (≠ NULL),
> nó sẽ **đè lên** fallback và `sendTime` sẽ khác. Khi demo nên set hai cột này về NULL cho user test.

### 2.4 Nội dung & nơi lấy device token

- Token: bảng `fcm_tokens` (notification-service DB), `ListFCMTokensByUser`.
- Gửi: Firebase `SendEachForMulticast`, priority `high`.
- Payload: title `"Time to study"`, body `"You have N card(s) to review"` (+ streak nếu ≥ 3),
  data `{type: "study_reminder", due_count, streak}`.
- Ghi `notification_log` (status `sent`/`failed`) — vừa để audit vừa để dedup.

### 2.5 Sơ đồ luồng

```
Cloud Scheduler (mỗi 15p, UTC)
   └─ publish topic cron-study-reminder  { event_type, data: base64({"now": ...}) }
        └─ notification-service /push  (subscriber.go)
             └─ handleCronStudyReminder → HandleStudyReminderTick(now)
                  └─ ListReminderState() → foreach user (≤32 song song):
                       1) timezone (auth-service)
                       2) sendTime = optimalHour:00 − 15p
                       3) inWindow(localNow, sendTime)?            ── không → skip
                       4) CountDueForUser > 0?                     ── không → skip
                       5) đã gửi hôm nay? (notification_log)       ── rồi   → skip
                       6) dispatch → FCM multicast → thiết bị user
```

---

## Phần 3 — Cách demo khi bảo vệ

Hai script nằm trong `scripts/demo/`.

### 3.1 Demo nhắc học — `scripts/demo/demo-study-reminder.sh`

Thay phần "tick mỗi 15p" bằng một message tự publish. **Mẹo**: handler đọc thời điểm từ trường `now`
trong payload, nên script **giả `now` = đúng `sendTime`** của user test → cửa sổ luôn khớp bất kể giờ thật.

**Chuẩn bị user test (chạy 1 lần trên DB):**

```sql
-- stats-service DB: dùng fallback giờ ổn định
UPDATE user_stats
   SET optimal_hour_weekday = NULL,
       optimal_hour_weekend = NULL,
       reminder_local_time  = '21:00'
 WHERE user_id = '<USER_ID>';
```

- Đảm bảo user test có **≥ 1 thẻ tới hạn** (vào học vài thẻ để tạo due).
- Đảm bảo đã **đăng ký FCM token** (mở app trên device/emulator).

**Chạy:**

```bash
PROJECT=mempan-cac51 USER_TZ=Asia/Bangkok REMINDER_HHMM=21:00 \
  ./scripts/demo/demo-study-reminder.sh
```

**Nếu push không bắn**, kiểm tra theo thứ tự (script cũng in sẵn):
- thiếu thẻ due → vào học tạo due;
- `optimal_hour_*` chưa NULL → set NULL như trên;
- đã gửi hôm nay (dedup) → xóa log:
  ```sql
  DELETE FROM notification_log
   WHERE user_id = '<USER_ID>'
     AND notification_type = 'study_reminder'
     AND created_at >= date_trunc('day', now());
  ```
- xem log: dòng `[cron] study_reminder: sent to <uid> (N devices)`.

### 3.2a Demo cron tối ưu (luồng đầy đủ) — gọi endpoint thật

Cách demo "đúng sản phẩm": hạ ngưỡng N rồi gọi chính endpoint mà Cloud Scheduler gọi.

```bash
# 1) study-service chạy với cron bật + ngưỡng demo (docker-compose local đã set sẵn:
#    MODERATION_SERVICE_ADDRESS, FSRS_OPTIMIZE_MIN_REVIEWS=50, CRON_SECRET=local-dev-cron-secret)

# 2) Gọi endpoint (giả lập Cloud Scheduler):
curl -s -X POST http://localhost:8082/internal/fsrs/optimize \
     -H "X-Cron-Secret: local-dev-cron-secret" | jq
# → {"eligible":N,"optimized":N,"skipped":0,"failed":0,"min_reviews":50,"duration_ms":...}

# 3) Cho hội đồng xem version mới active trong DB:
#    SELECT user_id, version, is_active, trained_on_reviews, training_loss, created_at
#      FROM user_fsrs_weights ORDER BY created_at DESC LIMIT 5;
```

Trên prod, job `cron-fsrs-optimize` chạy việc này mỗi ngày (xem `cloud-scheduler.sh`); có thể chạy tay:
`gcloud scheduler jobs run cron-fsrs-optimize --location=asia-southeast1`.

### 3.2b Demo tối ưu trọng số (chỉ phần lõi) — `scripts/demo/demo-optimize-weights.py`

Nếu không muốn dựng cả study-service: đóng vai study-service gọi thẳng RPC `OptimizeWeights` để
hội đồng thấy review logs vào → 21 trọng số mới + loss ra. **Không cần sửa code service.**

**Yêu cầu:** `moderation-fsrs-service` đang chạy gRPC ở `:50051`
(docker-compose: service `moderation-service`, `GRPC_PORT=50051`).

**Chạy bằng venv của service** (đã có sẵn `grpc` + pb stubs):

```bash
cd services/moderation-fsrs-service
.venv/bin/python ../../scripts/demo/demo-optimize-weights.py
# hoặc tuỳ chỉnh:
FSRS_ADDR=localhost:50051 USER_ID=demo-user N_REVIEWS=600 \
  .venv/bin/python ../../scripts/demo/demo-optimize-weights.py
```

Kết quả in ra `fsrs_version`, `num_reviews_used`, `loss`, và toàn bộ trọng số `w[0..n]`.

### 3.3 Gợi ý trình bày trước hội đồng

- **Nhắc học**: trình bày là tính năng hoàn chỉnh; nhấn 4 điều kiện lọc và việc tách giờ tối ưu
  weekday/weekend theo timezone từng user.
- **Tối ưu trọng số**: trình bày luồng tự động — Cloud Scheduler chạy mỗi 24h, study-service quét user
  có ≥ N review (mặc định N=1000, FSRS khuyến nghị), re-tune qua moderation-fsrs, lưu version mới active.
- Nếu hội đồng hỏi "tự động khi nào / N bao nhiêu?": **mỗi 24h**, N mặc định **1000** (ngưỡng FSRS để
  re-tune đáng tin cậy; dưới ~400 thì overfit) và **env-configurable** — demo hạ xuống 50.

---

## Phụ lục — Tham chiếu file nhanh

| Chủ đề | File |
|---|---|
| Bảng trọng số | `services/study-service/db/migration/000001_init.up.sql:94` |
| Dùng trọng số khi review | `services/study-service/internal/service/study_service.go:294` |
| Chuyển 21 số → params | `services/study-service/internal/fsrs/weights.go:13` |
| Lưu/đổi trọng số (repo) | `services/study-service/internal/repository/fsrs_weights_repo.go` |
| RPC optimize (proto) | `services/moderation-fsrs-service/proto/moderation_fsrs.proto:60` |
| Optimize (impl) | `services/moderation-fsrs-service/app/services/fsrs_servicer.py:46` |
| Cron orchestrator | `services/study-service/internal/fsrsopt/optimizer.go` |
| gRPC client → moderation | `services/study-service/internal/moderationclient/client.go` |
| HTTP trigger endpoint | `services/study-service/cmd/server/main.go` (`fsrsOptimizeHandler`) |
| Cron job (24h) | `deploy/pubsub-setup/cloud-scheduler.sh` (`cron-fsrs-optimize`) |
| Nhận push tick | `services/notification-service/internal/subscriber/handler.go:166` |
| Logic nhắc học | `services/notification-service/internal/scheduler/scheduler.go` |
| Gửi FCM | `services/notification-service/internal/fcm/sender.go:56` |
| Cloud Scheduler jobs | `deploy/pubsub-setup/cloud-scheduler.sh` |
| Script demo nhắc học | `scripts/demo/demo-study-reminder.sh` |
| Script demo optimize | `scripts/demo/demo-optimize-weights.py` |
