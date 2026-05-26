# moderation-fsrs-service — Tài liệu chi tiết

> Microservice nội bộ duy nhất viết bằng **Python** trong hệ mem_pan. Đảm nhận 2 nghiệp vụ trục:
> 1. **Moderation** — kiểm duyệt nội dung deck (text + image) bằng 2 model ML (XLM-RoBERTa + ViT-base).
> 2. **FSRS Optimization** — tối ưu trọng số học tập cá nhân hoá cho user dựa trên review-log lịch sử.
>
> Tài liệu này mô tả: cấu trúc thư mục, tech stack & lý do chọn, các luồng logic nghiệp vụ chính, cơ chế triển khai và các quy tắc "bất khả xâm phạm".

---

## Mục lục

1. [Tổng quan kiến trúc](#1-tổng-quan-kiến-trúc)
2. [Tech stack & lý do lựa chọn](#2-tech-stack--lý-do-lựa-chọn)
3. [Cấu trúc thư mục](#3-cấu-trúc-thư-mục)
4. [Lifecycle khởi động (`app/main.py`)](#4-lifecycle-khởi-động-appmainpy)
5. [Hai luồng logic quan trọng](#5-hai-luồng-logic-quan-trọng)
   - [5.1. Luồng moderation theo Pub/Sub event (chính)](#51-luồng-moderation-theo-pubsub-event-chính)
   - [5.2. Luồng moderation theo gRPC trực tiếp](#52-luồng-moderation-theo-grpc-trực-tiếp)
   - [5.3. Luồng FSRS optimization](#53-luồng-fsrs-optimization)
6. [Inference internals](#6-inference-internals)
7. [Side-effects khi phát hiện vi phạm](#7-side-effects-khi-phát-hiện-vi-phạm)
8. [Cấu hình & threshold động](#8-cấu-hình--threshold-động)
9. [Observability](#9-observability)
10. [Đóng gói & triển khai Cloud Run](#10-đóng-gói--triển-khai-cloud-run)
11. [5 quy tắc "sống còn"](#11-5-quy-tắc-sống-còn)
12. [Bảng biến môi trường](#12-bảng-biến-môi-trường)

---

## 1. Tổng quan kiến trúc

```
┌────────────────────┐  Pub/Sub push (card.created, card.updated)  ┌──────────────────────────────┐
│  deck-service (Go) │ ──────────────────────────────────────────► │                              │
└────────────────────┘                                             │   moderation-fsrs-service    │
                                                                   │   (Python 3.11 · asyncio)    │
┌────────────────────┐  gRPC ModerateDeck (đồng bộ, batch)         │                              │
│  admin-service (Go)│ ──────────────────────────────────────────► │  ┌────────────────────────┐  │
└────────────────────┘                                             │  │ XLM-RoBERTa (text)     │  │
                                                                   │  │ ViT-base    (image)    │  │
┌────────────────────┐  gRPC OptimizeWeights                       │  │ fsrs-optimizer (proc.) │  │
│  study-service (Go)│ ──────────────────────────────────────────► │  └────────────────────────┘  │
└────────────────────┘                                             └──────┬───────────────────────┘
                                                                          │ Side-effects khi VIOLATION:
                                                                          ├─ gRPC ─► deck-service.AdminUpdateDeckStatus(deleted)
                                                                          └─ Pub/Sub `moderation.deck_deleted` ─►
                                                                                      ├─ notification-service (FCM + email)
                                                                                      └─ admin-service (audit log)
```

### Triết lý thiết kế

| Tiêu chí | Lựa chọn | Lý do |
|---|---|---|
| Chính sách | **Recall-first** (1 card vi phạm → xoá nguyên deck) | App học tập đa lứa tuổi: false-positive rẻ hơn false-negative. |
| Inter-service | gRPC (HTTP/2 + protobuf) + Pub/Sub | Schema chặt với Go, binary nhẹ, async fan-out. |
| Runtime | Python 3.11 + `grpcio.aio` + `asyncio` | Inference CPU-bound → offload thread-pool; event loop xử lý I/O Pub/Sub song song. |
| ML stack | `torch==2.7.1+cpu` + `transformers` | Cloud Run không có GPU; wheel CPU-only giảm image từ ~2.5GB → ~1.4GB. |
| Cold start | Models lazy-mount qua **GCS FUSE** + load 1 lần ở lifespan | 4Gi RAM giữ luôn cả 2 model trong memory. |

---

## 2. Tech stack & lý do lựa chọn

| Layer | Tech | Phiên bản | Lý do chọn |
|---|---|---|---|
| **Language** | Python | 3.11 | Bắt buộc khi dùng `transformers` + `fsrs-optimizer`; 3.11 cải thiện ~15% tốc độ so với 3.10. |
| **Server async** | `grpc.aio` (grpcio) | 1.68.0 | Cho phép gRPC server và HTTP server cùng chạy trên 1 event loop; backpressure khi inference chậm. |
| **HTTP server** | `aiohttp` | 3.10.11 | Nhẹ, async-native, đủ cho 3 endpoint (`/healthz`, `/metrics`, `/internal/pubsub`). Tránh kéo theo cả FastAPI/Starlette chỉ để serve 3 route. |
| **ML — text** | `transformers` + `xlm-roberta-base` | 4.46.3 | Multi-lingual (EN + VI), fine-tuned cho task toxic-classification. XLM thay vì BERT vì user mem_pan đa số viết tiếng Việt. |
| **ML — image** | `transformers` + `google/vit-base-patch16-224` | 4.46.3 | ViT-base nhỏ hơn ResNet-50 ở cùng accuracy, dễ deploy CPU-only, 224×224 phù hợp ảnh card. |
| **Torch backend** | `torch==2.7.1+cpu` | 2.7.1+cpu | CPU-only wheel ~200MB (so với CUDA ~2.5GB) → cold start ~40s thay vì >2 phút. |
| **FSRS** | `fsrs-optimizer` | 5.5.0 | Thư viện chính chủ của FSRS-5 (Free Spaced Repetition Scheduler) — output 19 trọng số. |
| **Async HTTP client** | `httpx` | 0.27.2 | Dùng cho cả 2 việc: tải ảnh từ URL khi moderate, và publish Pub/Sub qua REST API. |
| **Validation** | `pydantic` | 2.9.2 | Validate JSON envelope của Pub/Sub, đảm bảo wire-format khớp với struct GoLang. |
| **Metrics** | `prometheus-client` | 0.21.0 | Expose `/metrics` cho Cloud Run sidecar scrape. |
| **gRPC health** | `grpc-health-checking` | 1.68.0 | Cloud Run readiness probe gọi `grpc.health.v1.Health/Check`. |

### Tại sao tách riêng service Python?

- Hệ còn lại là Go monorepo. Nhưng `transformers` + `torch` chỉ ổn định trên Python. Việc dùng `cgo` để gọi libtorch từ Go là quá đau cho 1 đồ án.
- Cô lập dependency nặng (~1.4GB image) khỏi các service Go nhẹ (~30MB image).
- Re-train model offline bằng Python notebook trong `ml_model/` rồi push lên GCS — service runtime chỉ lo serving, không train.

---

## 3. Cấu trúc thư mục

```
services/moderation-fsrs-service/
├── proto/
│   ├── moderation_fsrs.proto       # contract chính (chia sẻ với Go)
│   └── deck_admin.proto            # subset của deck-service mà mình callback
├── pb/                             # stub Python sinh từ protoc (commit hay không tuỳ chính sách)
├── app/
│   ├── main.py                     # entrypoint: lifespan, start 2 server (gRPC + HTTP)
│   ├── config.py                   # load settings + threshold từ disk
│   ├── http_server.py              # aiohttp: /healthz /metrics /internal/pubsub
│   ├── models/
│   │   ├── registry.py             # gom 2 moderator vào 1 dataclass, build 1 lần
│   │   ├── text_moderator.py       # XLM-RoBERTa wrapper
│   │   └── image_moderator.py      # ViT-base wrapper + decode safety
│   ├── services/
│   │   ├── moderation_servicer.py  # gRPC: ModerateDeck (batch path)
│   │   └── fsrs_servicer.py        # gRPC: OptimizeWeights (process-pool offload)
│   ├── events/
│   │   ├── dispatcher.py           # nhận card.created/updated từ Pub/Sub, chạy moderation
│   │   ├── publisher.py            # publish moderation.deck_deleted lên Pub/Sub
│   │   └── types.py                # pydantic models — phải khớp với struct GoLang
│   ├── clients/
│   │   └── deck_admin_client.py    # async gRPC client gọi sang deck-service
│   ├── dataset/                    # schema JSONL versioned cho re-train
│   └── utils/
│       ├── logging.py
│       └── metrics.py              # khai báo Prometheus counter/histogram
├── tests/                          # unit + e2e (mock torch model)
├── Dockerfile                      # multi-stage, CPU-only torch
├── Makefile                        # venv / proto / run / docker
├── DEPLOY.md                       # manual deploy guide (không nằm trong deploy.yml)
└── README.md
```

### Layer mapping

```
        ┌─────────────────────────────────────────────────────┐
        │                   Transport layer                   │
        │   gRPC (services/)      │      HTTP (http_server)   │
        └──────────┬──────────────┴──────────┬────────────────┘
                   │                         │
                   ▼                         ▼
        ┌──────────────────────────────────────────────────┐
        │      Domain layer — events/dispatcher.py         │
        │  (luật nghiệp vụ: recall-first, side-effects)    │
        └──────────────────┬───────────────────────────────┘
                           ▼
        ┌──────────────────────────────────────────────────┐
        │     Inference layer — models/{text,image}        │
        │              (stateless, idempotent)             │
        └──────────────────────────────────────────────────┘
                           ▼
        ┌──────────────────────────────────────────────────┐
        │   Integration layer — clients/ + events/publisher │
        │       (gRPC out, Pub/Sub publish HTTP REST)      │
        └──────────────────────────────────────────────────┘
```

---

## 4. Lifecycle khởi động (`app/main.py`)

```
asyncio.run(serve())
  │
  ├─ setup_logging()
  ├─ settings = load_settings()                  # đọc env + threshold từ disk
  ├─ registry = build_registry(settings)         # TẢI model 1 LẦN — Rule 2
  │     ├─ TextModerator: tokenizer + XLM-RoBERTa + warm-up forward
  │     └─ ImageModerator: processor + ViT-base + warm-up forward
  ├─ deck_admin = DeckAdminClient(...)           # lazy connect
  ├─ moderation_publisher = PubsubPublisher(...) # lazy httpx client
  ├─ dispatcher = EventDispatcher(...)           # wire 3 dependency trên
  ├─ fsrs_pool = ProcessPoolExecutor(2)          # tách process để training nặng không block event loop
  │
  ├─ grpc_server = grpc.aio.server(...)
  │     ├─ add ModerationServicer
  │     ├─ add FsrsServicer
  │     └─ add HealthServicer (SERVING)
  │
  ├─ await grpc_server.start()
  ├─ await serve_http(dispatcher, ...)           # aiohttp lên port khác
  │
  ├─ register SIGTERM/SIGINT → stop_event
  ├─ await stop_event.wait()
  │
  └─ Graceful shutdown:
       ├─ grpc_server.stop(grace=10)
       ├─ runner.cleanup()
       ├─ fsrs_pool.shutdown(cancel_futures=True)
       ├─ deck_admin.close()
       └─ moderation_publisher.close()
```

Điểm quan trọng:

- **Warm-up forward pass** trong constructor (`TextModerator.__init__` và `ImageModerator.__init__`) bằng input dummy (`"warmup"` / `Image.new("RGB", (224,224))`). Mục đích: trigger graph fusion của PyTorch để **request thật đầu tiên có latency ổn định** (~30ms thay vì ~800ms).
- 2 listener (gRPC + HTTP) cùng chạy trên 1 process vì Cloud Run chỉ expose 1 container port. HTTP nhận Pub/Sub push, gRPC nhận RPC trực tiếp từ admin-service.
- **ProcessPoolExecutor** cho FSRS thay vì ThreadPool: `fsrs-optimizer` import `pandas` + nội bộ chạy gradient descent CPU-bound dài 10–60s → cần process riêng để không khoá GIL.

---

## 5. Hai luồng logic quan trọng

### 5.1. Luồng moderation theo Pub/Sub event (chính)

Đây là luồng **mặc định** trong production. Mỗi khi user tạo/sửa card, deck-service publish event vào Pub/Sub topic `deck-events`, Cloud Pub/Sub đẩy push HTTP đến `moderation-fsrs-service`.

```
deck-service (Go)
    │
    │ httpPublisher.Publish("card.created", inner_json)
    │       envelope = {event_type, data: b64(inner_json)}
    │       outer    = {messages: [{data: b64(envelope)}]}
    ▼
Cloud Pub/Sub topic: deck-events
    │
    │ Push subscription: moderation-deck-events-sub
    │ HTTPS POST  https://<run-url>/internal/pubsub?token=<secret>
    ▼
moderation-fsrs-service → HTTP /internal/pubsub
    │
    ├─ Check ?token=… ≡ PUBSUB_PUSH_SECRET                 (401 nếu mismatch)
    ├─ body = await request.json()
    ├─ b64decode lớp 1 → envelope {event_type, data}
    ├─ b64decode lớp 2 → inner JSON (CardCreated payload)
    └─ await dispatcher.dispatch(event_type, inner_bytes)
              │
              ▼
EventDispatcher._moderate_card(card_id, deck_id, user_id, front, back, image_url)
    │
    ├─ text_verdicts = registry.text.predict([content_front, content_back])
    ├─ image_verdicts = registry.image.predict([(card_id, None, image_url)])  (nếu có image)
    │
    ├─ any text violation OR any image violation?
    │     ├─ NO  → log "clean" + tăng counter; KẾT THÚC (204 cho Pub/Sub)
    │     └─ YES → reason = mixed | text_violation | image_violation
    │              │
    │              ├─ await deck_admin.update_deck_status(deck_id, "deleted")  ─── gRPC tới deck-service
    │              │     ← trả về deck_name để forward xuống notification
    │              │
    │              └─ await moderation_publisher.publish(
    │                       "moderation.deck_deleted",
    │                       ModerationDeckDeletedEvent{deck_id, user_id, deck_name,
    │                                                  reason, violated_card_ids, deleted_at}
    │                  )                                  ─── Pub/Sub REST publish
    │
    └─ HTTP 204 → Cloud Pub/Sub ack (không retry)
```

**Cơ chế envelope double-base64**: Pub/Sub yêu cầu field `data` là base64. GoLang `json.Marshal([]byte)` cũng tự base64. Khi deck-service publish 1 struct chứa field `Data []byte`, lớp ngoài (Pub/Sub) base64 lần nữa toàn bộ JSON. Service Python phải decode 2 lần.

**Lý do trả `204` thay vì `200` khi clean**: chuẩn Pub/Sub — bất kỳ 2xx nào cũng là ack. `204` rõ nghĩa là "không có body". Khi parse envelope lỗi cũng trả `200` để ack (tránh poison message vòng lặp vô hạn).

### 5.2. Luồng moderation theo gRPC trực tiếp

Dùng cho **admin re-scan** một deck đã tồn tại (vd: phát hiện model mới, rescan deck cũ). Khác với luồng event:

- Input là cả batch `texts[]` + `images[]` thay vì 1 card.
- Trả về response chi tiết từng item (frontend admin hiển thị mức confidence).

```
admin-service (Go) ──gRPC── ModerateDeck(request) ──► ModerationServicer.ModerateDeck
    │
    ├─ run text inference qua loop.run_in_executor(None, text.predict, texts)   ─── default ThreadPool
    ├─ run image inference qua loop.run_in_executor(None, image.predict, items)
    │
    ├─ aggregate: ANY violation → status = VIOLATION (recall-first)
    ├─ build items[] = [{card_id, kind, is_violation, confidence, reason}, ...]
    │
    ├─ if any_violation:
    │     asyncio.create_task(self._on_violation(...))      ─── fire-and-forget
    │           ↑ side-effects (deck_admin.update_deck_status + Pub/Sub publish)
    │             KHÔNG chờ; gRPC response trả ngay
    │
    └─ return ModerateDeckResponse{deck_id, status, items}
```

**Điểm quan trọng**: cả 2 luồng (event + gRPC trực tiếp) đều gọi cùng 2 dependency `deck_admin` + `moderation_publisher` → đảm bảo deck bị xoá theo cách nào cũng phát sinh **cùng 1 sự kiện** xuống notification + admin (event-driven idempotent).

### 5.3. Luồng FSRS optimization

```
study-service (Go) ──gRPC── OptimizeWeights({user_id, review_logs[]})
    │
    └─► FsrsServicer.OptimizeWeights
        │
        ├─ validate: review_logs không rỗng (else INVALID_ARGUMENT)
        ├─ map review_logs → list[dict] (review_time, rating, elapsed_days)
        │
        ├─ await loop.run_in_executor(self.pool, _run_optimizer, rows)
        │       ↑ PROCESS POOL — không phải ThreadPool
        │
        │   _run_optimizer chạy trong worker process:
        │     import pandas, fsrs_optimizer        (heavy import in worker)
        │     df = pd.DataFrame(rows)
        │     optimizer = Optimizer()
        │     weights, loss = optimizer.train(df)
        │     return ([float(w) for w in weights], loss, version)
        │
        ├─ try/except: any error → gRPC INTERNAL
        ├─ fsrs_duration.observe(elapsed)
        │
        └─ return OptimizeWeightsResponse{user_id, weights[19], num_reviews_used, loss, fsrs_version}
```

**Lý do ProcessPoolExecutor**: `fsrs-optimizer.train()` là CPU-bound thuần (gradient descent 10–60s tuỳ data size). Nếu chạy trong ThreadPool, GIL chặn event loop → gRPC server không nhận được request khác. Process pool trả tự do GIL cho main process.

---

## 6. Inference internals

### 6.1. Text moderator (`models/text_moderator.py`)

| Thuộc tính | Giá trị |
|---|---|
| Base model | `xlm-roberta-base` (fine-tuned cho 2-class toxic) |
| Loader | `AutoModelForSequenceClassification.from_pretrained(model_dir)` |
| Tokenizer | `AutoTokenizer.from_pretrained(model_dir)` |
| Max length | 160 tokens (truncation=True) |
| Batch size | 32 |
| Threshold | đọc từ `threshold.json` → `recall_threshold` (mặc định 0.35) |
| Device | CPU (`torch.device("cpu")`) |

Luồng `predict(texts)`:

1. Trip empty/whitespace ra trước → trả `TextVerdict(False, 0.0)` ngay (Rule 4).
2. Còn lại split thành batch 32.
3. Forward pass: `softmax(logits, dim=-1)[:, 1]` lấy xác suất class "toxic".
4. `is_violation = prob >= threshold`.
5. Mọi exception → log + trả `prob=0.0` (fail-open: thà bỏ sót còn hơn crash deck).

### 6.2. Image moderator (`models/image_moderator.py`)

| Thuộc tính | Giá trị |
|---|---|
| Base model | `google/vit-base-patch16-224` (fine-tuned 2-class) |
| Loader | `AutoModelForImageClassification.from_pretrained(model_dir)` |
| Processor | `AutoImageProcessor.from_pretrained(model_dir)` + fallback (Rule 5) |
| Input | RGB 224×224 (processor auto-resize) |
| Batch size | 8 (image inference RAM-heavy hơn text) |
| Threshold | đọc từ `threshold.txt` (mặc định 0.5) |
| HTTP timeout fetch URL | 5s |

Luồng `predict(items)`:

1. Mỗi item là `(card_id, raw_bytes|None, url|None)`.
2. Nếu `raw` rỗng và có `url`: fetch qua `httpx.Client(timeout=5)`.
3. `_decode(raw)`: PIL `Image.open` → `verify()` → re-open + `.convert("RGB")`. Nếu fail → trả `ImageVerdict(decode_error=True)` (Rule 4) và inc counter `mod_image_decode_error_total`.
4. Image hợp lệ → gom batch 8 → forward pass → softmax → so threshold.

### 6.3. Cả 2 model dùng chung pattern

- **Warm-up trong constructor**: forward pass với input dummy → triggers torch graph fusion. Loại bỏ outlier latency request đầu.
- **`@torch.inference_mode()`** trên `_infer_batch`: tắt autograd, giảm memory ~30%, tăng tốc ~10–20%.
- **Single-thread CPU** + `OMP_NUM_THREADS=2` (env trong Dockerfile): 2 CPU core là sweet spot cho ViT/XLM batch nhỏ — thêm core không cải thiện vì batch quá nhỏ để parallelize.

---

## 7. Side-effects khi phát hiện vi phạm

Sự kiện `moderation.deck_deleted` là **single source of truth** khi 1 deck bị xoá vì vi phạm. Cả 2 luồng (event + gRPC) đều phát sinh nó.

```
violation detected
    │
    ├─ Step 1: deck_admin.update_deck_status(deck_id, status="deleted")
    │       gRPC tới deck-service (port 9091 trên cluster nội bộ,
    │                              hoặc :443 + TLS khi gọi qua Cloud Run URL)
    │       deck-service flip cột content_status = 'deleted' trong DB.
    │       Trả về deck.name để forward xuống event sau.
    │
    └─ Step 2: pubsub_publisher.publish("moderation.deck_deleted", payload)
              Topic: PUBSUB_MODERATION_TOPIC (default "moderation-events")
              Payload (pydantic ModerationDeckDeletedEvent):
                  {deck_id, user_id, deck_name, reason,
                   violated_card_ids[], deleted_at, moderator_version}
              ──► notification-service: gửi FCM push + email cảnh báo user
              ──► admin-service: ghi audit log "moderation_action"
```

### Auth tới Pub/Sub khi publish

`PubsubPublisher._auth_header()`:

- **Trên Cloud Run**: fetch token từ `http://metadata.google.internal/.../default/token`, cache đến 5 phút trước khi expire, gắn `Authorization: Bearer ...`.
- **Local dev** (`PUBSUB_EMULATOR_HOST` set): không cần auth, gọi thẳng emulator.
- **Metadata server fail** (vd: chạy local mà không có emulator): log warning, gọi không auth → Pub/Sub trả 403 → caller log error nhưng không crash.

### gRPC tới deck-service: TLS vs insecure

`_is_cloud_run(addr)` kiểm tra `addr.endswith(":443") or ".run.app" in addr`:

- Match → `grpc.ssl_channel_credentials()` (TLS bắt buộc cho Cloud Run public URL).
- Không match → `grpc.aio.insecure_channel(addr)` (Docker Compose internal hostname `deck-service:9091`).

---

## 8. Cấu hình & threshold động

`config.py::load_settings()` đọc:

| Nguồn | Field | Mặc định | Ràng buộc |
|---|---|---|---|
| `threshold.json` của text model | `recall_threshold` (fallback `best_f1_threshold`) | (không có default) | 0.0 ≤ x ≤ 1.0 |
| `threshold.txt` của image model | toàn bộ file (1 float) | (không có default) | 0.0 ≤ x ≤ 1.0 |
| env `TEXT_MODEL_DIR` | path | `/models/flashcard_text_moderator` | phải tồn tại |
| env `IMAGE_MODEL_DIR` | path | `/models/flashcard_image_moderator` | phải tồn tại |
| env `GRPC_PORT` | int | 50051 | |
| env `HTTP_PORT` | int | 8087 | |
| env `DECK_SERVICE_ADDR` | host:port | `deck-service:9091` | |
| env `PUBSUB_PROJECT_ID` | string | `local-dev` | |
| env `PUBSUB_MODERATION_TOPIC` | string | `moderation-events` | |
| env `PUBSUB_PUSH_SECRET` | string (secret) | `dev-secret` | dùng so token query string |
| env `FSRS_POOL_WORKERS` | int | 2 | |

**Tại sao threshold đọc từ disk thay vì env?** Vì model + threshold đi cùng nhau như một artifact: re-train model → update threshold → push cả 2 lên GCS. Nếu để env, một sai sót deploy có thể dùng threshold cũ với model mới.

---

## 9. Observability

### Metrics (`/metrics` — Prometheus format)

| Metric | Type | Labels | Ý nghĩa |
|---|---|---|---|
| `mod_text_latency_seconds` | Histogram | — | Latency 1 lần text inference |
| `mod_image_latency_seconds` | Histogram | — | Latency 1 lần image inference |
| `fsrs_optimize_duration_seconds` | Histogram | — | Wall-clock FSRS train |
| `mod_violation_total` | Counter | `kind=text|image` | Số item vi phạm |
| `mod_clean_total` | Counter | `kind=text|image` | Số item sạch |
| `mod_image_decode_error_total` | Counter | — | Ảnh decode lỗi (URL chết / file hỏng) |
| `mod_cpu_pct`, `mod_rss_bytes` | Gauge | — | Process metrics |

### Logging

- Cấu trúc: stdlib `logging`, format chuẩn module-level.
- Cloud Run tự thu stdout → Cloud Logging.
- Key events log:
  - `startup: models loaded` (sau khi `build_registry` xong)
  - `gRPC listening on :50051`
  - `HTTP server listening on :8087`
  - `moderate deck=... user=... status=... violated=N items=N` (gRPC path)
  - `VIOLATION origin=card.created card=... deck=... reason=...` (event path)
  - `pubsub published type=moderation.deck_deleted topic=...`
  - `deck status updated deck=... status=deleted name=...`

### Health check

- `grpc.health.v1.Health/Check` → trả SERVING khi đã `await grpc_server.start()`.
- `GET /healthz` → `200 ok` ngay khi aiohttp lên.
- Cloud Run readiness probe có thể dùng 1 trong 2.

---

## 10. Đóng gói & triển khai Cloud Run

### Dockerfile (multi-stage)

```
Stage 1 — builder (python:3.11-slim)
    apt: build-essential, git
    venv: /opt/venv
    pip install -r requirements.txt
        --index-url https://download.pytorch.org/whl/cpu       ← Rule 3
        --extra-index-url https://pypi.org/simple
    shrink venv: rm tests/, __pycache__/, *.pyc
    codegen proto → /build/pb

Stage 2 — runtime (python:3.11-slim)
    apt: libgomp1, ca-certificates
    useradd app (uid 1000)
    COPY /opt/venv from builder
    COPY /build/pb from builder
    COPY app/ ./app/
    ENV OMP_NUM_THREADS=2, TOKENIZERS_PARALLELISM=false
    USER app
    EXPOSE 50051 8087
    ENTRYPOINT ["python", "-m", "app.main"]
```

Image cuối cùng **~1.4GB**. Phần lớn là torch CPU wheel (~200MB) + transformers + tokenizers + protobuf stubs.

### Cloud Run config (xem `DEPLOY.md` để biết command đầy đủ)

| Flag | Giá trị | Lý do |
|---|---|---|
| `--execution-environment=gen2` | **bắt buộc** | Cần GCS FUSE driver |
| `--add-volume type=cloud-storage bucket=mempan-cac51-models` | mount `/models` | Lazy-read 1.5GB model từ GCS |
| `--cpu-boost` | bật | Giảm cold start từ ~70s xuống ~40s |
| `--memory=4Gi` | tối thiểu | XLM (~1.1GB) + ViT (~343MB) + tensor working set |
| `--cpu=2` | 2 vCPU | Khớp `OMP_NUM_THREADS=2`; thêm CPU không tăng throughput vì batch nhỏ |
| `--min-instances=0` | scale-to-zero | Tiết kiệm vì traffic thưa |
| `--max-instances=2` | cap | Cap chi phí; mỗi instance ~$0.10/h khi reserve CPU |
| `--timeout=300` | 5 phút | FSRS optimize có thể ~60s; cho thừa biên |
| `--use-http2` | bật | Cần cho gRPC qua Cloud Run public URL |
| `--port=8080` | HTTP_PORT | Cloud Run hiện chỉ expose 1 port; HTTP đảm nhận Pub/Sub push |

**Lưu ý quan trọng**: Service này **không** nằm trong `deploy.yml` matrix (GitHub Actions) vì cấu hình đặc biệt (gen2, FUSE, 4Gi). Deploy thủ công theo `DEPLOY.md`.

---

## 11. 5 quy tắc "sống còn"

| # | Quy tắc | File / điểm enforce |
|---|---|---|
| 1 | **Threshold đọc từ disk**, không hardcode hằng số 0.35 / 0.5 ở đâu khác | `app/config.py::_load_text_threshold`, `_load_image_threshold` |
| 2 | **Model load đúng 1 lần** ở startup, không lazy load per-request | `app/main.py` gọi `build_registry()` TRƯỚC `grpc_server.start()` |
| 3 | **Torch CPU-only wheel** | `Dockerfile`: `--index-url https://download.pytorch.org/whl/cpu` |
| 4 | **Input rỗng / corrupt → CLEAN**, không bao giờ raise lên gRPC handler | `text_moderator.py::predict` (skip whitespace), `image_moderator.py::_decode` (catch UnidentifiedImageError, OSError, ValueError) |
| 5 | **Preprocessor fallback** về `google/vit-base-patch16-224` nếu `preprocessor_config.json` rỗng | `image_moderator.py::_load_processor` |

Vi phạm bất kỳ quy tắc nào trong số này đã từng gây ra incident → giữ nguyên, không refactor.

---

## 12. Bảng biến môi trường

| Biến | Default | Mô tả |
|---|---|---|
| `TEXT_MODEL_DIR` | `/models/flashcard_text_moderator` | Đường dẫn dir chứa XLM-RoBERTa weights + tokenizer + `threshold.json` |
| `IMAGE_MODEL_DIR` | `/models/flashcard_image_moderator` | Đường dẫn dir chứa ViT-base weights + processor + `threshold.txt` |
| `GRPC_PORT` | `50051` | Port gRPC server (cluster nội bộ) |
| `HTTP_PORT` | `8087` | Port HTTP (Pub/Sub push + health + metrics). Trên Cloud Run set thành `8080`. |
| `GRPC_MAX_WORKERS` | `8` | Thread pool offload inference đồng bộ |
| `FSRS_POOL_WORKERS` | `2` | Số worker process cho FSRS optimize |
| `DECK_SERVICE_ADDR` | `deck-service:9091` | Target gRPC callback. Trên prod: `deck-service-...-as.a.run.app:443`. |
| `NOTIFICATION_SERVICE_ADDR` | `notification-service:9095` | (Reserved — hiện tại notification subscribe Pub/Sub thay vì nhận gRPC) |
| `PUBSUB_PROJECT_ID` | `local-dev` | GCP project ID (prod: `mempan-cac51`) |
| `PUBSUB_MODERATION_TOPIC` | `moderation-events` | Tên topic phát sự kiện `moderation.deck_deleted` |
| `PUBSUB_PUSH_SECRET` | `dev-secret` | Shared secret so với query `?token=...` của Pub/Sub push |
| `PUBSUB_EMULATOR_HOST` | (unset) | Local dev: set để PubsubPublisher đi qua emulator không cần auth |
| `OMP_NUM_THREADS` | `2` (set trong Dockerfile) | Cap OpenMP cho torch — 2 là sweet spot với batch nhỏ |
| `TOKENIZERS_PARALLELISM` | `false` (set trong Dockerfile) | Tắt fork-parallelism của tokenizers; tránh deadlock khi fork sau khi khởi tạo |

---

## Phụ lục — Tham khảo chéo

- Spec gốc: `doc/MODERATION_SERVICE_SPEC.md`
- Event catalog: `doc/event-catalog.md`
- Kiến trúc tổng: `doc/architecture.md`
- Deploy script chi tiết: `services/moderation-fsrs-service/DEPLOY.md`
- Proto contract: `services/moderation-fsrs-service/proto/moderation_fsrs.proto`
