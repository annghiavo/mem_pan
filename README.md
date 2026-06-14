# mem_pan

export PATH=$PATH:/home/anvo/go/bin && make migrateup

CSV/TSV — two-column rows, auto-detected separator, blank rows skipped, BOM-safe           
  - PDF — Quizlet-style numbered two-column tables, text-selectable only                       
  - A quick decision table at the bottom so users can pick without reading the whole thing     
                                                                                               
  The note about the header row becoming a card is worth calling out — it's a common surprise  
  with CSV imports.

  check docker disk usage:

  docker system df
  docker system prune -a -f
  
  open -a OrbStack

  ./scripts/reset-data.sh

  docker compose restart deck-service



  # Study reminder push
  curl -sS -X POST 'http://192.168.1.44:8000/v1/notifications/devices:test' \
    -H "Authorization: Bearer v2.local.xrO_WOjmIiGwST24rrwdCp_Vxmm9HdjG5Du6VT9pFlJdidzB-Kom4-PgTIHi_y13NYxENMOxXxQScXO9REoBrhjGGcyOmWbN-f1fEVfxd932rBPOl2VkR1yFkZp-DMiPZGqYOcBMtF06FavIZoC-KSo_PqRTWkT7yUs2l8YyQmD1Yg7oR-hTR29yFCtlDiUTS8X01w99Q_1jO9D3ddKMzCyb8B04tThL7DboPDPW9tjyr8eVcX-XhxqqNveBCOWmRlSwHk3beyna1IGGV4v1kjV88sc4Srr1blSJhx4zFRePPn9YOlCam__wnetEGPE0HqT_5pD2URH-V56zL4Ay4KwsBhG7gv_QvhTYKowea2NbT32iqw.bnVsbA" \
    -H "Content-Type: application/json" \
    -d '{"notification_type":"study_reminder","due_count":7,"streak":4}'

  # Streak warning push
  curl -sS -X POST 'http://192.168.1.44:8000/v1/notifications/devices:test' \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"notification_type":"streak_warning","due_count":3,"streak":5}'

---

# GCP Deployment Playbook

> Project: **mempan-cac51** (number `272885252422`) · Region: **asia-southeast1** · API Gateway: **us-central1**
> Chạy lần lượt theo từng phase. Mỗi phase đều có lệnh verify ở cuối — chỉ đi phase sau khi verify hiện đúng.

## Live endpoints

```
API Gateway (single client URL):
  https://mempan-gateway-3hd0u0cm.uc.gateway.dev

moderation-fsrs-service (Python, GCS-mounted ML models):
  https://moderation-fsrs-service-272885252422.asia-southeast1.run.app

GCS bucket (ML models, 1.5GB):
  gs://mempan-cac51-models/
  └── flashcard_image_moderator/  (343MB — ViT-base)
  └── flashcard_text_moderator/   (1.1GB — XLM-RoBERTa)

Direct service URLs (cho debug):
  auth          → https://auth-service-wzed7v5hbq-as.a.run.app
  deck          → https://deck-service-wzed7v5hbq-as.a.run.app
  study         → https://study-service-wzed7v5hbq-as.a.run.app
  stats         → https://stats-service-wzed7v5hbq-as.a.run.app
  admin         → https://admin-service-wzed7v5hbq-as.a.run.app
  notification  → https://notification-service-wzed7v5hbq-as.a.run.app
  search        → https://search-service-wzed7v5hbq-as.a.run.app
```

Billing-service deployment and PayOS payout status:
`doc/billing-service-deployment.md`

## CI Deploy (GitHub Actions, matrix parallel)

File: `.github/workflows/deploy.yml`. Trigger:
- **Push lên `main`** với thay đổi trong `services/**`, `pkg/**`, `proto/**`, `go.work*` → auto detect service nào đổi → build/push/deploy song song (max 7 jobs).
- **Manual** qua "Run workflow" → chọn 1 service hoặc `all`.

Yêu cầu GitHub secrets (Settings → Secrets and variables → Actions):
- `GCP_WIF_PROVIDER` = `projects/272885252422/locations/global/workloadIdentityPools/github-pool/providers/github-provider`
- `GCP_WIF_SERVICE_ACCOUNT` = `github-deployer@mempan-cac51.iam.gserviceaccount.com`

Mỗi job:
1. Auth qua Workload Identity Federation (không cần JSON key file).
2. Build Docker image, tag `:<sha>` + `:latest`, dùng `cache-from registry`.
3. Push lên Artifact Registry.
4. `gcloud run deploy` với secrets từ Secret Manager + env vars + `--use-http2`.
5. Smoke test swagger endpoint.

Toàn bộ pipeline ~3-5 phút cho 7 service chạy song song (so với ~25 phút nếu tuần tự).

## ML models (Cloud Storage FUSE mount)

`moderation-fsrs-service` (Python) cần 2 model `.safetensors` (~1.5GB) — quá lớn để đẩy lên Docker image. Models sống trong GCS bucket `mempan-cac51-models` và được mount vào container qua Cloud Storage FUSE volume:

```bash
gcloud run deploy moderation-fsrs-service \
  --execution-environment=gen2 \
  --add-volume=name=models,type=cloud-storage,bucket=mempan-cac51-models \
  --add-volume-mount=volume=models,mount-path=/models \
  --set-env-vars="TEXT_MODEL_DIR=/models/flashcard_text_moderator,IMAGE_MODEL_DIR=/models/flashcard_image_moderator,..." \
  --memory=4Gi --cpu=2
```

Cold start ~40s vì service phải lazy-read 1.5GB từ GCS. Sau khi load xong, inference <100ms.

Update models:
```bash
gcloud storage cp -r ml_model/results/flashcard_text_moderator gs://mempan-cac51-models/
# Restart Cloud Run service để pick up
gcloud run services update moderation-fsrs-service --region=asia-southeast1 --clear-base-image
```

## FCM (Firebase Cloud Messaging) — Application Default Credentials

`notification-service` **không cần** file JSON service account key. Cloud Run runtime SA `mempan-runtime@` đã có role `roles/firebasecloudmessaging.admin`, Firebase Admin SDK tự xài metadata server token. Cấu hình:

```yaml
# Cloud Run env vars
FCM_PROJECT_ID: mempan-cac51
# (KHÔNG set FCM_CREDENTIALS_FILE — code sẽ fallback sang ADC)
```

Test:
```bash
curl -X POST $GATE/v1/notifications/devices:test \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"notification_type":"study_reminder","token":"REAL_FCM_TOKEN","due_count":5}'
```

> Lưu ý: SMTP password trong secret `smtp-password` hiện không xác thực được với Gmail. Tạo lại Gmail App Password tại myaccount.google.com → Security → 2-Step → App passwords, rồi:
> ```bash
> echo -n "NEW_APP_PASSWORD" | gcloud secrets versions add smtp-password --data-file=-
> gcloud run services update notification-service --region=asia-southeast1 --quiet  # pick up new version
> ```

## Phase 0 — Prerequisites

```bash
# 1. gcloud CLI version (>= 470)
gcloud --version

# 2. Login (mở browser)
gcloud auth login

# 3. Chọn project + region mặc định
gcloud config set project mempan-cac51
gcloud config set run/region asia-southeast1
gcloud config set artifacts/location asia-southeast1

# 4. Billing phải bật. Nếu billingEnabled: false → vào Console link billing account.
gcloud beta billing projects describe mempan-cac51
```

## Phase 1 — Enable APIs (~2 phút)

```bash
gcloud services enable \
  run.googleapis.com \
  artifactregistry.googleapis.com \
  cloudbuild.googleapis.com \
  pubsub.googleapis.com \
  secretmanager.googleapis.com \
  iam.googleapis.com \
  iamcredentials.googleapis.com \
  sts.googleapis.com \
  cloudscheduler.googleapis.com
```

Verify:

```bash
gcloud services list --enabled \
  --filter="NAME:(run.googleapis.com OR artifactregistry.googleapis.com OR pubsub.googleapis.com OR secretmanager.googleapis.com)" \
  --format="value(NAME)"
# Mong đợi: 4 dòng (artifactregistry, pubsub, run, secretmanager)
```

## Phase 2 — Artifact Registry

```bash
gcloud artifacts repositories create mempan-services \
  --repository-format=docker \
  --location=asia-southeast1 \
  --description="mem_pan microservice images"

gcloud auth configure-docker asia-southeast1-docker.pkg.dev
```

Verify:

```bash
gcloud artifacts repositories list --location=asia-southeast1
# Phải thấy: mempan-services  DOCKER  asia-southeast1
```

## Phase 3 — Service Accounts + Workload Identity Federation (cho GitHub Actions)

### 3.1 — Runtime SA (Cloud Run sẽ chạy dưới SA này)

```bash
gcloud iam service-accounts create mempan-runtime \
  --display-name="mem_pan Cloud Run runtime"

RUNTIME_SA="mempan-runtime@mempan-cac51.iam.gserviceaccount.com"

gcloud projects add-iam-policy-binding mempan-cac51 \
  --member="serviceAccount:$RUNTIME_SA" \
  --role="roles/secretmanager.secretAccessor"

gcloud projects add-iam-policy-binding mempan-cac51 \
  --member="serviceAccount:$RUNTIME_SA" \
  --role="roles/pubsub.publisher"

gcloud projects add-iam-policy-binding mempan-cac51 \
  --member="serviceAccount:$RUNTIME_SA" \
  --role="roles/pubsub.subscriber"
```

### 3.2 — Deployer SA (GitHub Actions impersonates SA này)

```bash
gcloud iam service-accounts create github-deployer \
  --display-name="GitHub Actions deployer"

DEPLOY_SA="github-deployer@mempan-cac51.iam.gserviceaccount.com"
RUNTIME_SA="mempan-runtime@mempan-cac51.iam.gserviceaccount.com"

gcloud projects add-iam-policy-binding mempan-cac51 \
  --member="serviceAccount:$DEPLOY_SA" \
  --role="roles/run.admin"

gcloud projects add-iam-policy-binding mempan-cac51 \
  --member="serviceAccount:$DEPLOY_SA" \
  --role="roles/artifactregistry.writer"

gcloud iam service-accounts add-iam-policy-binding "$RUNTIME_SA" \
  --member="serviceAccount:$DEPLOY_SA" \
  --role="roles/iam.serviceAccountUser"
```

### 3.3 — Workload Identity Pool + Provider

> **Lưu ý**: chuỗi `attribute-mapping` rất dài, terminal hay tự wrap dòng làm gãy chuỗi `attribute.repository_owner`. Cách an toàn: bọc trong heredoc và `bash /tmp/wif.sh`.

```bash
gcloud iam workload-identity-pools create github-pool \
  --location=global \
  --display-name="GitHub Actions Pool"

cat > /tmp/wif.sh <<'EOF'
gcloud iam workload-identity-pools providers create-oidc github-provider \
  --location=global \
  --workload-identity-pool=github-pool \
  --display-name="GitHub OIDC" \
  --attribute-mapping="google.subject=assertion.sub,attribute.repository=assertion.repository,attribute.repository_owner=assertion.repository_owner" \
  --attribute-condition="assertion.repository_owner == 'annghiavo'" \
  --issuer-uri="https://token.actions.githubusercontent.com"
EOF
bash /tmp/wif.sh

DEPLOY_SA="github-deployer@mempan-cac51.iam.gserviceaccount.com"

gcloud iam service-accounts add-iam-policy-binding "$DEPLOY_SA" \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/projects/272885252422/locations/global/workloadIdentityPools/github-pool/attribute.repository_owner/annghiavo"
```

Verify + lưu config:

```bash
echo "Runtime SA:  $(gcloud iam service-accounts list --filter='email:mempan-runtime@' --format='value(email)')"
echo "Deployer SA: $(gcloud iam service-accounts list --filter='email:github-deployer@' --format='value(email)')"
echo "WIF Pool:    $(gcloud iam workload-identity-pools list --location=global --format='value(name)')"

cat <<'CFG'

=== Lưu 2 dòng dưới làm GitHub repo secrets (Settings → Secrets and variables → Actions) ===
WIF_PROVIDER=projects/272885252422/locations/global/workloadIdentityPools/github-pool/providers/github-provider
WIF_SERVICE_ACCOUNT=github-deployer@mempan-cac51.iam.gserviceaccount.com
CFG
```

## Phase 4 — Secrets (Secret Manager)

Mỗi service có 1 DB URL Neon riêng + PASETO key. Tạo 1 secret cho mỗi cái:

```bash
# 4.1 — DATABASE_URL cho từng service (lặp lại cho deck/study/stats/...)
echo -n "postgresql://<user>:<password>@<host>.neon.tech/<db>?sslmode=require" | \
  gcloud secrets create auth-db-url --replication-policy=automatic --data-file=-

# 4.2 — PASETO symmetric key (32 bytes hex)
openssl rand -hex 32 | gcloud secrets create paseto-symmetric-key \
  --replication-policy=automatic --data-file=-
```

Lặp lại 4.1 cho 8 service:

| Secret name        | Service               |
| ------------------ | --------------------- |
| `auth-db-url`      | auth-service          |
| `deck-db-url`      | deck-service          |
| `study-db-url`     | study-service         |
| `stats-db-url`     | stats-service         |
| `admin-db-url`     | admin-service         |
| `notif-db-url`     | notification-service  |
| `worker-db-url`    | worker-service        |
| `moderation-db-url`| moderation-fsrs-service |

Search-service không có DB riêng (dùng Elasticsearch). Lưu URL ES + API key:

```bash
echo -n "https://<es-host>" | gcloud secrets create es-url \
  --replication-policy=automatic --data-file=-
echo -n "<es-api-key>"      | gcloud secrets create es-api-key \
  --replication-policy=automatic --data-file=-
```

## Phase 5 — Pub/Sub topics

```bash
for t in user-events deck-events card-events study-events cron-study-reminder; do
  gcloud pubsub topics create "$t" 2>/dev/null || echo "topic $t already exists"
done

gcloud pubsub topics list --format="value(name)"
# Phải thấy 5 topic
```

Subscription tạo sau khi deploy service (cần URL `/internal/pubsub` thật) — xem Phase 8.

## Phase 6 — Build + push image đầu tiên (auth-service)

```bash
cd /Users/annghiavo/Documents/mem_pan

IMG="asia-southeast1-docker.pkg.dev/mempan-cac51/mempan-services/auth-service:v0.1.0"

docker build -f services/auth-service/Dockerfile -t "$IMG" .
docker push "$IMG"
```

## Phase 7 — Deploy Cloud Run service đầu tiên

```bash
gcloud run deploy auth-service \
  --image "asia-southeast1-docker.pkg.dev/mempan-cac51/mempan-services/auth-service:v0.1.0" \
  --region asia-southeast1 \
  --platform managed \
  --service-account "mempan-runtime@mempan-cac51.iam.gserviceaccount.com" \
  --set-secrets "DATABASE_URL=auth-db-url:latest,PASETO_SYMMETRIC_KEY=paseto-symmetric-key:latest" \
  --set-env-vars "GIN_MODE=release,PUBSUB_PROJECT_ID=mempan-cac51" \
  --allow-unauthenticated \
  --port 8080 \
  --memory 512Mi

# Lưu URL hiện ra
gcloud run services describe auth-service --region asia-southeast1 --format='value(status.url)'
```

Lặp lại Phase 6+7 cho 7 service còn lại (đổi `auth-service` → tên service tương ứng và secret tương ứng).

## Phase 8 — Pub/Sub subscriptions (push)

Sau khi mỗi service đã deploy và có URL Cloud Run, tạo subscription push tới `<URL>/internal/pubsub`:

```bash
# Token bảo vệ endpoint (lưu vào secret)
openssl rand -hex 32 | gcloud secrets create pubsub-push-token \
  --replication-policy=automatic --data-file=-

STATS_URL=$(gcloud run services describe stats-service --region asia-southeast1 --format='value(status.url)')
PUSH_TOKEN=$(gcloud secrets versions access latest --secret=pubsub-push-token)

# Ví dụ: stats-service subscribe user-events
gcloud pubsub subscriptions create stats-user-events-sub \
  --topic=user-events \
  --push-endpoint="${STATS_URL}/internal/pubsub?token=${PUSH_TOKEN}" \
  --ack-deadline=60
```

Bảng đăng ký subscription đầy đủ:

| Topic              | Subscription                  | Service receiver       |
|--------------------|-------------------------------|------------------------|
| user-events        | stats-user-events-sub         | stats-service          |
| user-events        | search-user-events-sub        | search-service         |
| user-events        | notif-user-events-sub         | notification-service   |
| deck-events        | stats-deck-events-sub         | stats-service          |
| deck-events        | search-deck-events-sub        | search-service         |
| deck-events        | admin-deck-events-sub         | admin-service          |
| card-events        | stats-card-events-sub         | stats-service          |
| card-events        | search-card-events-sub        | search-service         |
| study-events       | stats-study-events-sub        | stats-service          |
| cron-study-reminder| notif-cron-sub                | notification-service   |

## Phase 9 — GitHub Actions secrets

Vào **Settings → Secrets and variables → Actions** của repo `annghiavo/mem_pan`:

| Tên                       | Giá trị                                                                                  |
| ------------------------- | ---------------------------------------------------------------------------------------- |
| `GCP_WIF_PROVIDER`        | `projects/272885252422/locations/global/workloadIdentityPools/github-pool/providers/github-provider` |
| `GCP_WIF_SERVICE_ACCOUNT` | `github-deployer@mempan-cac51.iam.gserviceaccount.com`                                   |
| `GCP_PROJECT_ID`          | `mempan-cac51`                                                                           |
| `GCP_REGION`              | `asia-southeast1`                                                                        |

Sau đó update `.github/workflows/*.yml` thêm step `google-github-actions/auth@v2` + `setup-gcloud@v2` + deploy (làm sau khi Phase 6+7 verify được).

---

## Troubleshooting

| Triệu chứng | Fix |
|---|---|
| `ERROR: required property [project] is not currently set` | `gcloud config set project mempan-cac51` (gcloud update thường reset config) |
| `INVALID_ARGUMENT: Invalid mapped attribute key: attribute.repository_ow ner` | Terminal wrap dòng giữa chuỗi. Dùng `cat > /tmp/wif.sh <<'EOF' ... EOF; bash /tmp/wif.sh` |
| `billingEnabled: false` | Link billing ở Console: https://console.cloud.google.com/billing/linkedaccount?project=mempan-cac51 |
| `PERMISSION_DENIED: ... iam.serviceAccountUser` | SA `github-deployer` chưa được bind quyền act-as `mempan-runtime`. Chạy lại lệnh cuối ở 3.2 |
| Docker push fail `denied: ... insufficient_scope` | Chạy lại `gcloud auth configure-docker asia-southeast1-docker.pkg.dev` |
