# Session Summary — 2026-05-24

> Tổng kết một session làm việc: từ "viết blueprint test cho 8 microservices" → "deploy toàn bộ hệ thống lên Cloud Run với API Gateway, CI/CD, cron jobs".

## Mục lục

1. [Test infrastructure (Go)](#1-test-infrastructure-go)
2. [GCP deployment from scratch](#2-gcp-deployment-from-scratch)
3. [API Gateway](#3-api-gateway)
4. [GitHub Actions CI/CD](#4-github-actions-cicd)
5. [ML models + moderation-fsrs-service](#5-ml-models--moderation-fsrs-service)
6. [FCM via Application Default Credentials](#6-fcm-via-application-default-credentials)
7. [Cloud Scheduler cron jobs](#7-cloud-scheduler-cron-jobs)
8. [Bug fixes nảy sinh trong quá trình deploy](#8-bug-fixes-nảy-sinh-trong-quá-trình-deploy)
9. [Memory entries cho session sau](#9-memory-entries-cho-session-sau)
10. [Files thay đổi](#10-files-thay-đổi)
11. [Resource inventory trên GCP](#11-resource-inventory-trên-gcp)
12. [Việc còn lại](#12-việc-còn-lại)

---

## 1. Test infrastructure (Go)

### 1.1. Test strategy doc (Blueprint)

Viết tiếng Việt, đăng trong chat. Tổng kết:

- **Service layer**: `go.uber.org/mock` mock cho repository + EventPublisher; table-driven; 3 kịch bản bắt buộc (Success / Bad Input / Internal Error).
- **Repository layer**: `testcontainers-go/modules/postgres` spin Postgres 16 thật + `golang-migrate` apply migrations; build tag `//go:build integration`.
- **Gapi layer**: `google.golang.org/grpc/test/bufconn` cho in-memory gRPC.
- File test cùng thư mục code; testify/require + assert.

### 1.2. Mock generation (manual mockgen-style output)

Vì không có `mockgen` available local, viết tay mock files theo đúng format mockgen sinh ra (đối chiếu với mock có sẵn để khớp pattern).

| Service | New mock file | Interface |
|---|---|---|
| deck-service | `internal/mock/event_publisher.go` | `publisher.EventPublisher` |
| stats-service | `internal/mock/stats_repo.go` | `repository.StatsRepository` |
| notification-service | `internal/mock/notification_repo.go` | `repository.NotificationRepository` |
| notification-service | `internal/mock/fcm_sender.go` | `fcm.Sender` |
| notification-service | `internal/mock/mailer.go` | `mailer.Mailer` |

### 1.3. Service-layer table-driven tests

| File | Method tested | Kịch bản |
|---|---|---|
| `deck-service/internal/service/clone_deck_test.go` | `DeckService.CloneDeck` | Success/Owner clones own/SourceDeleted→NotFound/PrivateDeck→Forbidden/DBError/TxFails |
| `stats-service/internal/service/stats_service_test.go` | `StatsService.RecomputeOptimalHours` | Argmax + minSamples threshold + invalid day_type + DB errors |
| `notification-service/internal/service/notification_service_test.go` | `NotificationService.SendDeckCloneReadyPush` | InvalidUUID/DBError/NoTokens/HappyPath với payload assertion/FCMFail |
| `study-service/internal/service/study_service_count_due_test.go` | `StudyService.CountDueByEndOfDay` | UTC fallback/Asia\_Ho\_Chi\_Minh local EOD/DB error |
| `search-service/internal/es/search_test.go` | `PageOffset`, `MultiMatchOrMatchAll`, `BoolQuery` | Pure unit, không cần mock |

Study-service service coverage **81.1% → 84.6%**.

### 1.4. Repository integration tests (testcontainers)

| File | Coverage |
|---|---|
| `deck-service/internal/repository/testmain_integration_test.go` + `deck_repo_integration_test.go` | CreateDeck+GetDeckByID round-trip, ErrDeckNotFound mapping, SoftDelete |
| `stats-service/internal/repository/testmain_integration_test.go` + `stats_repo_integration_test.go` | UserStats lifecycle, GetUserStats→ErrUserStatsNotFound, BumpActivityBucket accumulation |
| `notification-service/internal/repository/testmain_integration_test.go` + `notification_repo_integration_test.go` | FCM token upsert+delete, CountRecentNotifications window |
| `search-service/internal/service/search_service_integration_test.go` | Real Elasticsearch 8.13.4 container; SearchDecks scope filtering (Public/Mine/All + anonymous) |

Lệnh chạy: `go test -tags=integration ./internal/...`

### 1.5. Per-service GitHub Actions workflows

| Workflow | Trạng thái trước session | Trạng thái sau |
|---|---|---|
| `auth-service.yml` | Đã có | Patched: strip Neon `-pooler.` từ DB URL |
| `deck-service.yml` | **Trống** (0 line) | Filled — build + race tests + integration |
| `stats-service.yml` | **Trống** | Filled — tương tự |
| `search-service.yml` | Không tồn tại | Created — build + ES integration (timeout 25 phút) |
| `admin-service.yml` | **Trống** | Filled — conditional integration step |
| `notification-service.yml` | Đã có | Updated — add integration step, fix `setup-go@v4` → `@v5` |
| `study-service.yml` | Đã có | Patched: strip pooler |

Pattern chung: trigger theo path filter, `concurrency` cancel-in-progress, `--use-http2` ready, `TESTCONTAINERS_RYUK_DISABLED=true`, merge coverage profiles + filter codegen dirs.

---

## 2. GCP deployment from scratch

Project tồn tại sẵn (`mempan-cac51`, number `272885252422`) nhưng chưa setup gì.

### Phase 0 — Prerequisites
- Verify billing enabled
- `gcloud config set project mempan-cac51 / run/region asia-southeast1`

### Phase 1 — Enable APIs
9 APIs: Cloud Run, Artifact Registry, Cloud Build, Pub/Sub, Secret Manager, IAM, IAM Credentials, STS, Cloud Scheduler. Sau đó thêm: FCM, Firebase, API Gateway, Service Control, Service Management.

### Phase 2 — Artifact Registry
1 repository `mempan-services` ở `asia-southeast1`. `gcloud auth configure-docker` để Docker push được.

### Phase 3 — IAM (Service Accounts + Workload Identity Federation)

| SA | Roles |
|---|---|
| `mempan-runtime@` | `secretmanager.secretAccessor`, `pubsub.publisher`, `pubsub.subscriber`, `firebasecloudmessaging.admin`, bucket-level `storage.objectViewer` cho ml models |
| `github-deployer@` | `run.admin`, `artifactregistry.writer`, + `iam.serviceAccountUser` trên runtime SA |

WIF Pool `github-pool` + Provider `github-provider`:
- Issuer: `https://token.actions.githubusercontent.com`
- Attribute condition: `assertion.repository_owner == 'annghiavo'`
- Bound principalSet: `attribute.repository_owner/annghiavo`

→ GitHub Actions không cần JSON key file, dùng OIDC token exchange.

### Phase 4 — Secrets (13 secrets)

Bulk-import từ `services/*/app.env`:

| Category | Secrets |
|---|---|
| DB URLs | `auth-db-url`, `deck-db-url`, `study-db-url`, `stats-db-url`, `admin-db-url`, `notif-db-url` |
| Token | `paseto-symmetric-key` (shared) |
| Storage | `auth-cloudinary-url`, `deck-cloudinary-url` |
| Mail/FCM | `smtp-password` |
| Search | `es-url`, `es-api-key` |
| Pub/Sub | `pubsub-push-token` (random 32-byte hex) |

### Phase 5 — Pub/Sub topics (5)

`user-events`, `deck-events`, `card-events`, `study-events`, `cron-study-reminder`.

### Phase 6 — Build + push 7 Docker images
~17-22MB mỗi image. Push lên `asia-southeast1-docker.pkg.dev/mempan-cac51/mempan-services/<svc>:v0.2.x`.

### Phase 7 — Deploy 7 Cloud Run services

Mỗi service:
- `--platform=managed --region=asia-southeast1`
- `--service-account=mempan-runtime@...`
- `--set-secrets=...` + `--set-env-vars=...`
- `--use-http2 --allow-unauthenticated --port=8080`
- `--memory=512Mi --cpu=1 --max-instances=3 --timeout=300`

Service-to-service gRPC: `AUTH_SERVICE_ADDRESS=auth-service-wzed7v5hbq-as.a.run.app:443` (HTTPS endpoint).

### Phase 8 — Pub/Sub subscriptions (10 push)

| Topic | Subscription | Receiver |
|---|---|---|
| user-events | stats-user-events-sub, search-user-events-sub, notif-user-events-sub | 3 services |
| deck-events | stats-deck-events-sub, search-deck-events-sub, admin-deck-events-sub | 3 services |
| card-events | stats-card-events-sub, search-card-events-sub | 2 services |
| study-events | stats-study-events-sub | 1 service |
| cron-study-reminder | notif-cron-sub | 1 service |

Push endpoint: `<service-url>/internal/pubsub?token=<pubsub-push-token>`.

---

## 3. API Gateway

Provider: **Google Cloud API Gateway** (region `us-central1` — `asia-southeast1` chưa available cho project này).

**URL**: `https://mempan-gateway-3hd0u0cm.uc.gateway.dev`

**Spec**: `deploy/api-gateway/openapi.yaml` (OpenAPI v2 với extension `x-google-backend`).

| Prefix | Backend |
|---|---|
| `/v1/auth/*`, `/v1/users/*` | auth-service |
| `/v1/decks/*`, `/v1/folders/*`, `/v1/cards/*`, `/v1/import/*` | deck-service |
| `/v1/study/*` | study-service |
| `/v1/stats/*` | stats-service |
| `/v1/admin/*` | admin-service |
| `/v1/notifications/*` | notification-service |
| `/v1/search/*` | search-service |

**Key config bits**:
- `path_translation: APPEND_PATH_TO_ADDRESS` — gateway append nguyên path vào backend host
- `disable_auth: true` — gateway KHÔNG inject Google IAM token, forward user's `Authorization: Bearer <PASETO>` nguyên xi
- Path template `{path=**}` để catchall multi-segment
- `options:` method cho mỗi route → CORS preflight forward đến backend `withCORS` middleware

Hiện chạy config version `mempan-config-v3`.

---

## 4. GitHub Actions CI/CD

### 4.1. Deploy workflow (mới)

File: `.github/workflows/deploy.yml`

- Trigger: `push: main` với path filter `services/**`, `pkg/**`, `proto/**`, `go.work*`, hoặc manual `workflow_dispatch`
- 3 jobs:
  1. `detect-changes` — git diff phân tích service nào đổi, output JSON array
  2. `build-and-deploy` (matrix, max-parallel 7, fail-fast false) — auth WIF → build/push Docker → deploy Cloud Run với `--use-http2`
  3. `summary` — emit GH Actions summary table

Cải tiến từ tuần tự ~25 phút → song song ~3-5 phút cho 7 services.

### 4.2. GitHub secrets cần config (Settings → Secrets and variables → Actions)

```
GCP_WIF_PROVIDER        = projects/272885252422/locations/global/workloadIdentityPools/github-pool/providers/github-provider
GCP_WIF_SERVICE_ACCOUNT = github-deployer@mempan-cac51.iam.gserviceaccount.com
GCP_PROJECT_ID          = mempan-cac51
GCP_REGION              = asia-southeast1
```

### 4.3. Per-service test workflows
Xem bảng Phần 1.5 — 7 workflows trigger độc lập theo service folder.

### 4.4. Removed/dropped

- `.github/workflows/ci.yml` (root) — bị xóa hoàn toàn vì trùng chức năng + có bug `GOWORK=off` làm cross-module import fail
- `.golangci.yml` — bị xóa vì không workflow nào còn reference

---

## 5. ML models + moderation-fsrs-service

### 5.1. Models lưu trên GCS

Bucket: `gs://mempan-cac51-models/` (asia-southeast1, uniform access).

| Path | Size |
|---|---|
| `flashcard_image_moderator/model.safetensors` | 343 MB (ViT-base) |
| `flashcard_text_moderator/model.safetensors` | 1.1 GB (XLM-RoBERTa) |

Upload ~50s (30 MiB/s).

### 5.2. moderation-fsrs-service deployment (Python)

- Stack: Python 3.11 + PyTorch CPU-only + transformers + safetensors
- Image: `asia-southeast1-docker.pkg.dev/mempan-cac51/mempan-services/moderation-fsrs-service:v0.1.0`
- Cloud Run **gen2** (cần thiết cho GCS FUSE volume mount)
- `--add-volume=name=models,type=cloud-storage,bucket=mempan-cac51-models --add-volume-mount=volume=models,mount-path=/models`
- env: `TEXT_MODEL_DIR=/models/flashcard_text_moderator`, `IMAGE_MODEL_DIR=/models/flashcard_image_moderator`
- Resources: `--memory=4Gi --cpu=2`
- Cold start ~40s (lazy-read 1.5GB từ GCS)

Service URL: `https://moderation-fsrs-service-272885252422.asia-southeast1.run.app`

---

## 6. FCM via Application Default Credentials

Code FCM sender (`internal/fcm/sender.go`) **đã hỗ trợ ADC sẵn**: nếu `credentialsFile == ""` thì Firebase Admin SDK tự dùng metadata-server token.

Việc cần làm:
1. `gcloud services enable fcm.googleapis.com firebase.googleapis.com`
2. Grant `roles/firebasecloudmessaging.admin` cho `mempan-runtime@` SA
3. **KHÔNG** set env `FCM_CREDENTIALS_FILE` trên Cloud Run

Verify: gửi test với token giả → log "[fcm] token fake-test-to… failed: The registration token is not a valid FCM registration token" → ADC chain xác thực thành công, FCM API reject đúng vì token sai (đúng behavior).

---

## 7. Cloud Scheduler cron jobs

| Job | Schedule | Topic | Time zone |
|---|---|---|---|
| `cron-study-reminder-tick` | `*/15 * * * *` | `cron-study-reminder` | `Asia/Ho_Chi_Minh` |

Message body publish:
```json
{"event_type":"cron.study_reminder","data":""}
```

Pipeline xác nhận end-to-end:
1. Scheduler fire → Pub/Sub topic
2. Push delivery → `notification-service /internal/pubsub` (HTTP 204)
3. Subscriber decode envelope → `handler.handleCronStudyReminder` → `scheduler.HandleStudyReminderTick`
4. gRPC `stats-service.ListReminderState` (code=OK)
5. For each eligible user: study-service `CountDueForUser` + FCM send

---

## 8. Bug fixes nảy sinh trong quá trình deploy

### 8.1. Service-to-service gRPC qua TLS

**Triệu chứng**: deck-service trả "auth service unavailable: rpc error code = Unavailable: upstream connect error or disconnect/reset before headers".

**Nguyên nhân**: 11 client files dùng `grpc.WithTransportCredentials(insecure.NewCredentials())`. Cloud Run terminate TLS ở port 443.

**Fix**: Patch tất cả 11 client file thêm helper `pickCreds(addr)`:
```go
func pickCreds(addr string) credentials.TransportCredentials {
    if strings.HasSuffix(addr, ":443") || strings.Contains(addr, ".run.app") {
        return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
    }
    return insecure.NewCredentials()
}
```

### 8.2. HTTP/gRPC single-port mux

**Triệu chứng**: TLS connect OK nhưng "protocol error" — auth-service nghe HTTP trên port 8080 + gRPC trên 9090, Cloud Run chỉ expose 1 port.

**Fix**: Mỗi service main.go:
- Tách `runGRPCServer` thành `buildGRPCServer` (chỉ tạo `*grpc.Server`) + `runStandaloneGRPC` (chỉ chạy cho local dev nếu khác HTTP port)
- `runHTTPGateway` thay `pb.RegisterXxxHandlerFromEndpoint(dial localhost)` bằng `pb.RegisterXxxHandlerServer(in-process)`
- Mux HTTP/gRPC trên cùng port via `h2c`:
```go
mixed := http.HandlerFunc(func(w, r) {
    if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
        grpcServer.ServeHTTP(w, r); return
    }
    wrapped.ServeHTTP(w, r)
})
srv := &http.Server{Handler: h2c.NewHandler(mixed, &http2.Server{})}
```
- Deploy với `--use-http2`

Áp dụng cho 7 main.go (auth, deck, study, stats, admin, notification, search).

### 8.3. Pub/Sub publisher OAuth

**Triệu chứng**: register user → stats-service không có user_stats row. Publisher gửi POST tới `pubsub.googleapis.com` không kèm OAuth → Pub/Sub silently reject.

**Fix**: 4 publisher.go (auth/admin/deck/study) — thay `&http.Client{}` bằng `google.DefaultClient(ctx, "https://www.googleapis.com/auth/pubsub")`. Cloud Run metadata-server cung cấp token tự động.

### 8.4. API Gateway CORS preflight

**Triệu chứng**: Vercel frontend báo `Access to fetch ... has been blocked by CORS policy: No 'Access-Control-Allow-Origin' header`.

**Nguyên nhân**: OpenAPI spec chỉ khai báo `get/post/put/patch/delete` cho mỗi path. Gateway trả 404 cho preflight `OPTIONS`, không bao giờ tới backend.

**Fix**: Auto-add `options:` method vào 13 paths trỏ cùng backend. Backend `withCORS` middleware xử lý preflight đúng (echo Origin + return 204).

### 8.5. CI matrix secrets bash array

**Triệu chứng**: CI deploy fail với `DATABASE_URL is required`.

**Nguyên nhân**: `.github/workflows/deploy.yml` dùng `declare -A SECRETS` (associative array) rồi assign bằng indexed syntax `SECRETS=( "a" "b" "c" )`. Bash 4 trên Ubuntu runner xử lý khác bash 3 (Mac) → chỉ giữ 1 phần tử, `--set-secrets` thiếu 2/3 entries.

**Fix**: Bỏ `declare -A`, dùng plain string `SECRET_STR="K1=v1,K2=v2,K3=v3"`.

### 8.6. golang-migrate trên Neon pooler

**Triệu chứng**: `bind message supplies 1 parameters, but prepared statement requires 6`.

**Nguyên nhân**: Neon URL có `ep-xxx-pooler.region.aws.neon.tech` = transaction-mode pooler, không cho prepared statements.

**Fix**: Trong workflow `Resolve DB URL` step thêm `DB_URL="${DB_URL/-pooler./.}"` để dùng direct endpoint cho migration. Runtime trên Cloud Run vẫn dùng pooler (high-concurrency).

### 8.7. Migration dirty flag

**Triệu chứng**: `error: Dirty database version 1. Fix and force version.`

**Fix**: Migration cũ fail giữa chừng để dirty=true. Force clean:
```bash
migrate -path services/<svc>/db/migration -database "$DIRECT_URL" force <target_version>
```
3 DB cần force: deck (force 3), notification (force 4), admin (force 2).

### 8.8. Cloud Run env vars clobber secrets

**Triệu chứng**: Sau `gcloud run services update --update-env-vars`, container fail `DATABASE_URL is required`.

**Nguyên nhân**: `--update-env-vars` không drop secrets nhưng `--set-secrets` đi kèm với deployment update có thể clear secrets nếu không pass đủ. Cần re-apply full `--set-secrets` mỗi lần.

---

## 9. Memory entries cho session sau

Lưu vào `/Users/annghiavo/.claude/projects/-Users-annghiavo-Documents-mem-pan/memory/`:

| File | Type | Nội dung |
|---|---|---|
| `gcp-project.md` | reference | Project ID `mempan-cac51`, number `272885252422`, region `asia-southeast1` |
| `api-gateway.md` | reference | URL `https://mempan-gateway-3hd0u0cm.uc.gateway.dev`, region us-central1, spec path |
| `cloud-run-services.md` | reference | 8 services + image tags + runtime SA + cách deploy mới |
| `migration-pitfalls.md` | feedback | Dirty flag + Neon pooler issue + fix pattern |
| `cron-reminders.md` | reference | Cloud Scheduler job mỗi 15p, pipeline flow |

`MEMORY.md` index update tất cả.

---

## 10. Files thay đổi

### Code (Go)
- 7 `cmd/server/main.go` — refactored to h2c mux + HandlerServer in-process registration
- 4 `internal/publisher/*.go` — OAuth via `google.DefaultClient`
- 11 client files (`*/internal/{authclient,deckclient,statsclient,studyclient,notifyclient}/client.go`) — TLS detection `pickCreds()`

### Tests (mới)
- 5 service test files (clone_deck, stats_service, notification_service, study_count_due, es search)
- 4 testcontainers integration test setups (deck, stats, notif, search)
- 5 mock files (manually generated)

### Infra
- `.github/workflows/deploy.yml` — new, matrix parallel
- `.github/workflows/{admin,deck,stats,search}-service.yml` — created
- `.github/workflows/{auth,study,notification}-service.yml` — patched
- `.github/workflows/ci.yml` — created then **deleted** (duplicate + buggy)
- `.golangci.yml` — created then **deleted**
- `deploy/api-gateway/openapi.yaml` — new

### Docs
- `README.md` — appended ~250 dòng playbook GCP + CI + ML + FCM + troubleshooting
- `doc/session-summary-2026-05-24.md` — file này

### Git history
```
c226edc  ci: drop root ci.yml + .golangci.yml
0e78b55  ci: scope golangci-lint to 3 linters, mark Lint+codegen non-blocking
c53082f  fix: strip Neon -pooler from migration DB URL in CI
97e60bc  fix: CI secrets array + API Gateway CORS preflight
f67dc32  feat: API Gateway + CI matrix deploy + Cloud Run HTTP/2 mux
```

---

## 11. Resource inventory trên GCP

```
GCP project:     mempan-cac51 (272885252422)
Region (primary): asia-southeast1
Region (gateway): us-central1

Cloud Run services:           8
  - auth-service
  - deck-service
  - study-service
  - stats-service
  - admin-service
  - notification-service
  - search-service
  - moderation-fsrs-service

Artifact Registry:            1 repo (mempan-services)
  Docker images:              8 (v0.2.x + 0.1.x)

API Gateway:                  1 gateway (mempan-gateway)
  API:                        mempan-api
  Configs:                    v1, v2, v3 (v3 active)
  Routes:                     13 path prefixes × 4-6 methods + OPTIONS

GCS buckets:                  1 (mempan-cac51-models, 1.48GB)

Pub/Sub topics:               5
Pub/Sub subscriptions:        10

Secret Manager secrets:       13

Cloud Scheduler jobs:         1 (cron-study-reminder-tick, every 15min)

Service accounts:             2
  - mempan-runtime
  - github-deployer

IAM:
  Workload Identity Pool:     1 (github-pool)
  WIF Provider:               1 (github-provider, OIDC)

Firebase:                     enabled (FCM)
APIs enabled:                 12
```

---

## 12. Việc còn lại

| # | Việc | Mức độ |
|---|---|---|
| 1 | **SMTP App Password mới cho Gmail** — `smtp-password` secret hiện sai (`535 5.7.8 Username and Password not accepted`). Tạo App Password mới ở myaccount.google.com → Security, rồi `echo -n "NEW_PW" \| gcloud secrets versions add smtp-password --data-file=-`. | Khẩn cấp khi muốn gửi welcome/reset email |
| 2 | **Mobile app config** — đổi `API_BASE_URL` trong `mem_pan_mb/services/api.ts` thành `https://mempan-gateway-3hd0u0cm.uc.gateway.dev` | Khi test mobile thật |
| 3 | **Frontend Vercel** — hard refresh sau khi CORS fix; verify login flow | Verify |
| 4 | **cron-streak-warning** — chưa tạo (chỉ mới có cron-study-reminder). Tương tự pattern: tạo topic + sub + cron job với event_type `cron.streak_warning` | Optional, nếu muốn cảnh báo user sắp mất streak |
| 5 | **CORS_ALLOWED_ORIGINS** — backend hiện accept any origin (default permissive). Restrict sau bằng `gcloud run services update --update-env-vars="CORS_ALLOWED_ORIGINS=https://mem-pan-xxx.vercel.app"` | Security hardening |
| 6 | **Lint nghiêm túc hơn** — đã bỏ Lint job. Tương lai nếu muốn enforce: cài golangci-lint local, fix warnings, viết `.golangci.yml` đầy đủ, thêm lại workflow | Polish |
| 7 | **Custom domain** cho API Gateway thay vì `*.gateway.dev` | Branding |
| 8 | **Migrate API Gateway sang asia-southeast1** khi GCP enable cho project (hiện us-central1 → ~150ms cross-region latency) | Latency optimization |

---

## Phụ lục: Lệnh hay dùng

```bash
# Deploy 1 service mới qua CLI (giống deploy.yml CI)
gcloud run deploy <svc> --image=<img> --region=asia-southeast1 \
  --service-account=mempan-runtime@mempan-cac51.iam.gserviceaccount.com \
  --use-http2 --allow-unauthenticated --port=8080 \
  --set-secrets="..." --set-env-vars="..."

# Trigger cron manual
gcloud scheduler jobs run cron-study-reminder-tick --location=asia-southeast1

# Force-clean dirty migration
DIRECT_URL=$(gcloud secrets versions access latest --secret=<svc>-db-url | sed 's/-pooler\./\./')
migrate -path services/<svc>/db/migration -database "$DIRECT_URL" force <target>

# Update gateway sau khi edit openapi.yaml
gcloud api-gateway api-configs create mempan-config-v4 \
  --api=mempan-api --openapi-spec=deploy/api-gateway/openapi.yaml \
  --backend-auth-service-account=mempan-runtime@mempan-cac51.iam.gserviceaccount.com
gcloud api-gateway gateways update mempan-gateway \
  --location=us-central1 --api=mempan-api --api-config=mempan-config-v4

# Smoke test toàn bộ qua gateway
TOKEN=$(curl -s -X POST https://mempan-gateway-3hd0u0cm.uc.gateway.dev/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"smoke@test.com","password":"password123"}' \
  | python3 -c "import sys,json;print(json.load(sys.stdin)['accessToken'])")
for p in /v1/users/me /v1/decks /v1/study/decks/recent /v1/stats/me; do
  curl -s -o /dev/null -w "$p -> HTTP %{http_code}\n" \
    -H "Authorization: Bearer $TOKEN" \
    "https://mempan-gateway-3hd0u0cm.uc.gateway.dev$p"
done
```
