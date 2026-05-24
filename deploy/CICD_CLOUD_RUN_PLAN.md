# Kế hoạch CI/CD lên Google Cloud Run

> **Flow tổng quát:** GitHub push → Cloud Build (build & test) → Artifact Registry (lưu image) → Cloud Run (deploy)
>
> Tài liệu này viết cho người **chưa từng dùng Cloud Run**. Bạn chỉ cần làm tuần tự từ Phần 0 → Phần 9. Mỗi lệnh đều có giải thích trước khi chạy.

---

## Mục lục

- [Phần 0. Khái niệm cần biết trước](#phần-0-khái-niệm-cần-biết-trước)
- [Phần 1. Chuẩn bị máy local](#phần-1-chuẩn-bị-máy-local)
- [Phần 2. Tạo & cấu hình GCP Project](#phần-2-tạo--cấu-hình-gcp-project)
- [Phần 3. Tạo Artifact Registry](#phần-3-tạo-artifact-registry)
- [Phần 4. Chuẩn bị Service Account & quyền IAM](#phần-4-chuẩn-bị-service-account--quyền-iam)
- [Phần 5. Quản lý secret bằng Secret Manager](#phần-5-quản-lý-secret-bằng-secret-manager)
- [Phần 6. Viết cloudbuild.yaml cho từng service](#phần-6-viết-cloudbuildyaml-cho-từng-service)
- [Phần 7. Triển khai thử bằng tay (smoke test)](#phần-7-triển-khai-thử-bằng-tay-smoke-test)
- [Phần 8. Kết nối CI/CD tự động với GitHub](#phần-8-kết-nối-cicd-tự-động-với-github)
- [Phần 9. Vận hành: logs, rollback, cleanup](#phần-9-vận-hành-logs-rollback-cleanup)
- [Phụ lục A. Bảng cấu hình port của 9 service](#phụ-lục-a-bảng-cấu-hình-port-của-9-service)
- [Phụ lục B. Sự cố thường gặp](#phụ-lục-b-sự-cố-thường-gặp)

---

## Phần 0. Khái niệm cần biết trước

| Thuật ngữ | Giải thích ngắn |
|---|---|
| **GCP Project** | Một "hộp" chứa toàn bộ resource (Cloud Run, DB, secret, …). Tính tiền theo project. |
| **Cloud Build** | Dịch vụ chạy job CI/CD của Google. Mỗi job đọc file `cloudbuild.yaml`, chạy từng bước trong container. |
| **Artifact Registry** | Kho chứa Docker image (tương tự Docker Hub nhưng nằm trong GCP, private). |
| **Cloud Run** | Nơi chạy container. Bạn đưa image → Cloud Run tự scale 0 ↔ N theo lưu lượng. Trả tiền theo CPU/RAM-second. |
| **Service Account (SA)** | "Tài khoản máy" để các dịch vụ tự gọi nhau (Cloud Build push image, Cloud Run đọc secret, …). |
| **Secret Manager** | Lưu mật khẩu, API key. Cloud Run mount secret vào container như biến môi trường. |

**Lưu ý quan trọng về Cloud Run:**

1. Container **phải listen `$PORT`** mà Cloud Run cấp (mặc định `8080`). Nếu code của bạn đang nghe `:8080` cố định là OK; nhưng tốt hơn là đọc từ biến môi trường `PORT`.
2. Cloud Run **stateless**: file viết trong container sẽ mất khi instance bị scale xuống.
3. Cloud Run **chỉ expose 1 port HTTP**. Service của bạn đang mở cả HTTP `:8080` và gRPC `:9090` — sẽ xử lý ở [Phần 6.4](#64-xử-lý-service-có-cả-http--grpc).
4. Cloud Run hỗ trợ cả **HTTP/2 + gRPC** nếu bạn bật "Use HTTP/2 end-to-end".

---

## Phần 1. Chuẩn bị máy local

### 1.1. Cài Google Cloud SDK (`gcloud`)

macOS (bạn đang dùng):

```bash
brew install --cask google-cloud-sdk
```

Kiểm tra:

```bash
gcloud --version
```

### 1.2. Đăng nhập

```bash
gcloud auth login          # mở trình duyệt, login bằng tài khoản annghiavo@gmail.com
gcloud auth application-default login   # cấp credential cho SDK local
```

### 1.3. Cài Docker (để build/test image local trước khi đẩy lên)

Đã có Docker Desktop là OK. Kiểm tra:

```bash
docker --version
```

---

## Phần 2. Tạo & cấu hình GCP Project

### 2.1. Tạo project

Bạn có thể tạo qua [console.cloud.google.com](https://console.cloud.google.com) hoặc bằng CLI:

```bash
# Chọn tên project ID — phải là duy nhất toàn cầu. Ví dụ: mempan-dev
export PROJECT_ID="mempan-dev"
export REGION="asia-southeast1"          # Singapore — gần Việt Nam nhất
export AR_REPO="mempan-services"         # tên repo Artifact Registry

gcloud projects create "$PROJECT_ID" --name="Mem Pan Dev"
gcloud config set project "$PROJECT_ID"
gcloud config set run/region "$REGION"
gcloud config set artifacts/location "$REGION"
```

> **Lưu ý billing:** Project mới chưa có billing → không tạo được resource. Vào [console.cloud.google.com/billing](https://console.cloud.google.com/billing) link một billing account vào project trước khi qua bước tiếp theo.

### 2.2. Bật các API cần thiết

Chạy 1 lệnh duy nhất:

```bash
gcloud services enable \
  run.googleapis.com \
  cloudbuild.googleapis.com \
  artifactregistry.googleapis.com \
  secretmanager.googleapis.com \
  iam.googleapis.com \
  pubsub.googleapis.com \
  logging.googleapis.com
```

Lần đầu mất ~1 phút. Kiểm tra:

```bash
gcloud services list --enabled
```

---

## Phần 3. Tạo Artifact Registry

Đây là kho chứa Docker image của bạn. Tạo 1 repo Docker duy nhất, mỗi service là 1 image riêng bên trong.

```bash
gcloud artifacts repositories create "$AR_REPO" \
  --repository-format=docker \
  --location="$REGION" \
  --description="Docker images for Mem Pan microservices"
```

Cho Docker local biết cách authenticate với Artifact Registry:

```bash
gcloud auth configure-docker "${REGION}-docker.pkg.dev"
```

URL image cuối cùng sẽ có dạng:

```
asia-southeast1-docker.pkg.dev/mempan-dev/mempan-services/auth-service:abc123
                                ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
                                 PROJECT     REPO         IMAGE   :TAG
```

---

## Phần 4. Chuẩn bị Service Account & quyền IAM

Ta sẽ dùng **2 service account riêng** (best practice — không dùng default):

| SA | Việc của nó |
|---|---|
| `cicd-deployer@…` | Cloud Build dùng để build image, push lên AR, deploy lên Cloud Run |
| `runtime-<service>@…` | Mỗi service Cloud Run chạy dưới SA này — chỉ có quyền tối thiểu (đọc secret, publish Pub/Sub) |

### 4.1. SA cho CI/CD

```bash
gcloud iam service-accounts create cicd-deployer \
  --display-name="CI/CD Cloud Build deployer"

CICD_SA="cicd-deployer@${PROJECT_ID}.iam.gserviceaccount.com"

# Quyền: push lên Artifact Registry
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:$CICD_SA" \
  --role="roles/artifactregistry.writer"

# Quyền: deploy lên Cloud Run
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:$CICD_SA" \
  --role="roles/run.admin"

# Quyền: Cloud Build cần actAs lên runtime SA khi deploy
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:$CICD_SA" \
  --role="roles/iam.serviceAccountUser"

# Quyền: ghi log
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:$CICD_SA" \
  --role="roles/logging.logWriter"
```

### 4.2. SA runtime cho mỗi service

Làm chung 1 lần cho cả 9 service:

```bash
SERVICES=(auth deck admin study stats notification search moderation-fsrs worker)

for s in "${SERVICES[@]}"; do
  gcloud iam service-accounts create "runtime-$s" \
    --display-name="Runtime SA for $s service"

  SA="runtime-$s@${PROJECT_ID}.iam.gserviceaccount.com"

  # Đọc secret
  gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:$SA" \
    --role="roles/secretmanager.secretAccessor"

  # Publish Pub/Sub (nếu service nào không publish thì gỡ bỏ — ta giữ chung cho gọn)
  gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:$SA" \
    --role="roles/pubsub.publisher"

  # Subscribe Pub/Sub (cho consumer services)
  gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:$SA" \
    --role="roles/pubsub.subscriber"
done
```

> **Nguyên tắc least-privilege:** Sau khi service chạy ổn, hãy review và **bỏ bớt** quyền không cần với từng service.

---

## Phần 5. Quản lý secret bằng Secret Manager

`app.env` của bạn đang chứa `DATABASE_URL`, `PASETO_SYMMETRIC_KEY`, `CLOUDINARY_URL`… — **không bao giờ commit lên git** và **không hardcode vào image**.

### 5.1. Đẩy secret lên Secret Manager

Ví dụ cho `auth-service`:

```bash
# Tạo secret rỗng
gcloud secrets create auth-database-url --replication-policy="automatic"

# Đẩy giá trị (đọc từ file để không lộ trong shell history)
printf "postgresql://neondb_owner:...@...neon.tech/neondb?sslmode=require" \
  | gcloud secrets versions add auth-database-url --data-file=-

# Tương tự
gcloud secrets create auth-paseto-key --replication-policy="automatic"
printf "ab3fd6e56535b31a468ae68b34319a62" \
  | gcloud secrets versions add auth-paseto-key --data-file=-

gcloud secrets create auth-cloudinary-url --replication-policy="automatic"
printf "cloudinary://..." \
  | gcloud secrets versions add auth-cloudinary-url --data-file=-
```

### 5.2. Đặt convention tên

| Service | Secret name pattern |
|---|---|
| auth-service | `auth-database-url`, `auth-paseto-key`, `auth-cloudinary-url` |
| deck-service | `deck-database-url`, `deck-cloudinary-url` |
| admin-service | `admin-database-url`, … |
| … | `<service>-<var-name-kebab>` |

Làm chung cho cả 9 service. Bạn sẽ map secret → biến môi trường ở [Phần 6.3](#63-cloudbuildyaml-mẫu).

---

## Phần 6. Viết `cloudbuild.yaml` cho từng service

### 6.1. Cấu trúc thư mục mục tiêu

Thêm vào mỗi service 1 file `cloudbuild.yaml`:

```
services/auth-service/
  Dockerfile
  cloudbuild.yaml      ← thêm mới
  cmd/
  ...
```

### 6.2. File `cloudbuild.yaml` mẫu

**Tạo file `services/auth-service/cloudbuild.yaml`:**

```yaml
# Cloud Build sẽ chạy file này. Biến _* là tham số truyền vào.
substitutions:
  _REGION: asia-southeast1
  _REPO: mempan-services
  _SERVICE: auth-service
  _RUNTIME_SA: runtime-auth@${PROJECT_ID}.iam.gserviceaccount.com

# Cloud Build cần Docker BuildKit cho cache layer
options:
  logging: CLOUD_LOGGING_ONLY
  machineType: E2_HIGHCPU_8        # build nhanh hơn, ~1-2 phút/service

steps:
  # 1) Build image — context là ROOT repo vì Dockerfile dùng go workspace
  - id: build
    name: gcr.io/cloud-builders/docker
    args:
      - build
      - --tag=${_REGION}-docker.pkg.dev/${PROJECT_ID}/${_REPO}/${_SERVICE}:${SHORT_SHA}
      - --tag=${_REGION}-docker.pkg.dev/${PROJECT_ID}/${_REPO}/${_SERVICE}:latest
      - --file=services/${_SERVICE}/Dockerfile
      - .

  # 2) Push image lên Artifact Registry
  - id: push
    name: gcr.io/cloud-builders/docker
    args:
      - push
      - --all-tags
      - ${_REGION}-docker.pkg.dev/${PROJECT_ID}/${_REPO}/${_SERVICE}

  # 3) Deploy lên Cloud Run
  - id: deploy
    name: gcr.io/google.com/cloudsdktool/cloud-sdk:slim
    entrypoint: gcloud
    args:
      - run
      - deploy
      - ${_SERVICE}
      - --image=${_REGION}-docker.pkg.dev/${PROJECT_ID}/${_REPO}/${_SERVICE}:${SHORT_SHA}
      - --region=${_REGION}
      - --platform=managed
      - --service-account=${_RUNTIME_SA}
      - --allow-unauthenticated       # public HTTP; với internal-only bỏ flag này
      - --port=8080
      - --cpu=1
      - --memory=512Mi
      - --min-instances=0
      - --max-instances=5
      - --concurrency=80
      - --timeout=60s
      # Map secret → env var
      - --set-secrets=DATABASE_URL=auth-database-url:latest,PASETO_SYMMETRIC_KEY=auth-paseto-key:latest,CLOUDINARY_URL=auth-cloudinary-url:latest
      # Env thường (không nhạy cảm)
      - --set-env-vars=PUBSUB_PROJECT_ID=${PROJECT_ID},PUBSUB_TOPIC=user-events,ACCESS_TOKEN_DURATION=15m,REFRESH_TOKEN_DURATION=168h

# Image kết quả — Cloud Build tự ghi metadata
images:
  - ${_REGION}-docker.pkg.dev/${PROJECT_ID}/${_REPO}/${_SERVICE}:${SHORT_SHA}
  - ${_REGION}-docker.pkg.dev/${PROJECT_ID}/${_REPO}/${_SERVICE}:latest

# Cloud Build dùng SA này
serviceAccount: projects/${PROJECT_ID}/serviceAccounts/cicd-deployer@${PROJECT_ID}.iam.gserviceaccount.com
```

### 6.3. Sao chép cho 8 service còn lại

Copy file trên qua 8 service kia, mỗi service chỉ cần thay:
- `_SERVICE`: tên thư mục (vd `deck-service`)
- `_RUNTIME_SA`: vd `runtime-deck@…`
- `--set-secrets`: map đúng secret của service đó
- `--set-env-vars`: copy từ `app.env` của service đó (bỏ các biến nhạy cảm vì đã ở Secret Manager)
- `--port`: tham chiếu [Phụ lục A](#phụ-lục-a-bảng-cấu-hình-port-của-9-service)

### 6.4. Xử lý service có cả HTTP & gRPC

Cloud Run chỉ expose **1 port HTTP**. Có 2 lựa chọn:

**Option A — Khuyến nghị: chia thành 2 Cloud Run service.**
- `auth-service-http` (port 8080)
- `auth-service-grpc` (port 9090, dùng `--use-http2`)

Tạo 2 file `cloudbuild.yaml` riêng (hoặc 2 step deploy trong cùng file), chạy cùng 1 image — chỉ khác port + flag `--use-http2` cho bản gRPC.

```yaml
# Step deploy bản gRPC
- id: deploy-grpc
  name: gcr.io/google.com/cloudsdktool/cloud-sdk:slim
  entrypoint: gcloud
  args:
    - run
    - deploy
    - ${_SERVICE}-grpc
    - --image=${_REGION}-docker.pkg.dev/${PROJECT_ID}/${_REPO}/${_SERVICE}:${SHORT_SHA}
    - --region=${_REGION}
    - --port=9090
    - --use-http2
    - --service-account=${_RUNTIME_SA}
    - --no-allow-unauthenticated     # gRPC nội bộ, chặn public
    # ... các flag khác giống bản HTTP
```

**Option B:** Dùng gRPC-Gateway gộp HTTP+gRPC cùng 1 port. Repo bạn đang dùng gRPC-Gateway rồi (`pb/auth_service.pb.gw.go`) — kiểm tra xem `main.go` có mount cả 2 trên cùng port không. Nếu có, bỏ qua Option A.

> **Action item:** Sau khi đọc xong tài liệu, hãy quyết định Option A hay B trước khi viết `cloudbuild.yaml` cho các service có gRPC (`auth`, `deck`, `admin`, `study`).

### 6.5. Code cần sửa nhỏ: đọc `$PORT`

Cloud Run inject biến `PORT`. Trong code, sửa nơi đang nghe cứng:

```go
// trước
HTTP_SERVER_ADDRESS := ":8080"

// sau
port := os.Getenv("PORT")
if port == "" {
    port = "8080"
}
HTTP_SERVER_ADDRESS := ":" + port
```

Làm tương tự cho tất cả service.

---

## Phần 7. Triển khai thử bằng tay (smoke test)

Trước khi nối CI/CD GitHub, **deploy thử 1 service bằng tay** để chắc cú.

### 7.1. Submit build local lên Cloud Build

Từ root repo (`/Users/annghiavo/Documents/mem_pan`):

```bash
gcloud builds submit \
  --config=services/auth-service/cloudbuild.yaml \
  --substitutions=SHORT_SHA=$(git rev-parse --short HEAD) \
  .
```

Nó sẽ:
1. Upload toàn bộ repo lên Cloud Build (qua GCS staging).
2. Chạy 3 step: build → push → deploy.
3. Trả về URL Cloud Run.

Theo dõi log live ngay trên terminal. Nếu lỗi → đọc step nào fail → fix → chạy lại.

### 7.2. Test endpoint

```bash
URL=$(gcloud run services describe auth-service --region=$REGION --format='value(status.url)')
curl -i "$URL/v1/auth/health"   # hoặc endpoint healthcheck thực tế của service
```

### 7.3. Xem log

```bash
gcloud run services logs read auth-service --region=$REGION --limit=50
```

Hoặc vào console: **Cloud Run → auth-service → LOGS**.

---

## Phần 8. Kết nối CI/CD tự động với GitHub

Khi smoke test OK, chuyển sang tự động hoá. Có **2 cách**, chọn 1:

### Cách 1 (KHUYẾN NGHỊ): Cloud Build Trigger trực tiếp từ GitHub

Ưu điểm: ít step nhất, không cần GitHub Actions cho phần deploy. CI test có thể giữ ở GitHub Actions hiện tại (workflows `services/auth-service/**`).

**8.1. Connect GitHub repo:**

Vào console: **Cloud Build → Triggers → Connect Repository → GitHub (Cloud Build GitHub App)**.
Cấp quyền, chọn repo `mem_pan`.

**8.2. Tạo trigger cho mỗi service:**

```bash
gcloud builds triggers create github \
  --name="auth-service-deploy" \
  --repo-name="mem_pan" \
  --repo-owner="annghiavo" \
  --branch-pattern="^main$" \
  --included-files="services/auth-service/**,go.work,go.work.sum" \
  --build-config="services/auth-service/cloudbuild.yaml" \
  --service-account="projects/${PROJECT_ID}/serviceAccounts/cicd-deployer@${PROJECT_ID}.iam.gserviceaccount.com"
```

Lặp 9 lần cho 9 service. `--included-files` đảm bảo chỉ deploy service nào đụng tới file — tiết kiệm thời gian.

Bây giờ mỗi `git push origin main`:
- File `services/auth-service/**` thay đổi → trigger `auth-service-deploy` chạy.
- File `services/deck-service/**` thay đổi → trigger `deck-service-deploy` chạy.
- Hai trigger có thể chạy **song song**.

### Cách 2: Giữ GitHub Actions, gọi gcloud từ workflow

Nếu muốn quản lý CI/CD trong 1 chỗ (`.github/workflows`):

**8.3. Tạo Workload Identity Federation** (tránh dùng JSON key dài hạn):

```bash
# Tạo pool
gcloud iam workload-identity-pools create "github-pool" \
  --location="global" --display-name="GitHub Actions Pool"

POOL_ID=$(gcloud iam workload-identity-pools describe github-pool \
  --location=global --format='value(name)')

# Tạo provider cho GitHub OIDC
gcloud iam workload-identity-pools providers create-oidc "github-provider" \
  --location="global" \
  --workload-identity-pool="github-pool" \
  --display-name="GitHub provider" \
  --attribute-mapping="google.subject=assertion.sub,attribute.repository=assertion.repository" \
  --issuer-uri="https://token.actions.githubusercontent.com"

# Cho phép repo annghiavo/mem_pan impersonate cicd-deployer
gcloud iam service-accounts add-iam-policy-binding \
  "cicd-deployer@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/${POOL_ID}/attribute.repository/annghiavo/mem_pan"
```

**8.4. Thêm job deploy vào `.github/workflows/auth-service.yml`:**

```yaml
  deploy:
    needs: test
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    permissions:
      contents: read
      id-token: write       # cần cho Workload Identity Federation
    steps:
      - uses: actions/checkout@v4

      - uses: google-github-actions/auth@v2
        with:
          workload_identity_provider: projects/<PROJECT_NUMBER>/locations/global/workloadIdentityPools/github-pool/providers/github-provider
          service_account: cicd-deployer@mempan-dev.iam.gserviceaccount.com

      - uses: google-github-actions/setup-gcloud@v2

      - name: Submit Cloud Build
        run: |
          gcloud builds submit \
            --config=services/auth-service/cloudbuild.yaml \
            --substitutions=SHORT_SHA=${GITHUB_SHA::7} \
            .
```

> **Khuyến nghị:** Chọn **Cách 1** cho mới bắt đầu — ít file phải maintain hơn.

---

## Phần 9. Vận hành: logs, rollback, cleanup

### 9.1. Xem log

```bash
# Realtime tail
gcloud beta run services logs tail auth-service --region=$REGION

# Hoặc lọc 100 dòng cuối
gcloud run services logs read auth-service --region=$REGION --limit=100
```

Hoặc **Console → Cloud Run → service → LOGS** (có filter, search).

### 9.2. Rollback về revision cũ

Mỗi lần deploy tạo 1 revision. List:

```bash
gcloud run revisions list --service=auth-service --region=$REGION
```

Chuyển 100% traffic về revision cũ:

```bash
gcloud run services update-traffic auth-service \
  --region=$REGION \
  --to-revisions=auth-service-00007-abc=100
```

### 9.3. Canary (10% traffic về bản mới)

```bash
gcloud run services update-traffic auth-service \
  --region=$REGION \
  --to-revisions=auth-service-00008-def=10,auth-service-00007-abc=90
```

### 9.4. Dọn image cũ trong Artifact Registry

Tạo cleanup policy (giữ 10 tag mới nhất):

```bash
gcloud artifacts repositories set-cleanup-policies "$AR_REPO" \
  --location="$REGION" \
  --policy=cleanup-policy.json
```

Với `cleanup-policy.json`:

```json
[{
  "name": "keep-recent",
  "action": {"type": "Keep"},
  "mostRecentVersions": {"keepCount": 10}
},{
  "name": "delete-old",
  "action": {"type": "Delete"},
  "condition": {"olderThan": "30d"}
}]
```

### 9.5. Theo dõi chi phí

Vào **Billing → Reports**, filter theo service `Cloud Run`, `Artifact Registry`, `Cloud Build`. Set **budget alert** ngay từ đầu để khỏi giật mình:

```bash
# Ví dụ: cảnh báo khi vượt 50% của $50/tháng
# Tạo qua console: Billing → Budgets & alerts → CREATE BUDGET
```

---

## Phụ lục A. Bảng cấu hình port của 9 service

Trích từ `docker-compose.yml` / Dockerfile hiện tại:

| Service | HTTP port | gRPC port | Cần `--use-http2`? |
|---|---|---|---|
| auth-service | 8080 | 9090 | Có (nếu deploy gRPC riêng) |
| deck-service | 8081 | 9091 | Có |
| admin-service | 8083 | 9093 | Có |
| study-service | 8082 | 9092 | Có |
| stats-service | 8084 | — | Không |
| notification-service | 8085 | — | Không |
| search-service | 8086 | — | Không |
| moderation-fsrs-service | 8087 | — | Không |
| worker-service | 8088 | — | Không (background worker — xem ghi chú dưới) |

**Worker-service** không cần HTTP listener — Cloud Run yêu cầu phải listen $PORT. Có 2 cách:
- (a) Thêm 1 endpoint `/healthz` đơn giản trả 200 OK trên `$PORT` để Cloud Run hài lòng.
- (b) Chuyển sang **Cloud Run Jobs** (chạy on-demand, không cần HTTP) — phù hợp hơn cho worker pull từ Pub/Sub.

---

## Phụ lục B. Sự cố thường gặp

| Triệu chứng | Nguyên nhân | Cách fix |
|---|---|---|
| Cloud Build báo `permission denied` khi push | SA chưa có `artifactregistry.writer` | Quay lại [4.1](#41-sa-cho-cicd) |
| Cloud Run revision báo `Container failed to start. Failed to start and listen on the port…` | Code nghe cứng `:8080` thay vì `$PORT`, hoặc service crash khi boot | Đọc [6.5](#65-code-cần-sửa-nhỏ-đọc-port), xem log container |
| `Permission 'iam.serviceAccounts.actAs' denied` | CICD SA chưa có `iam.serviceAccountUser` | Quay lại [4.1](#41-sa-cho-cicd) |
| Secret không vào container | Sai tên secret hoặc runtime SA chưa có `secretAccessor` | Check `gcloud secrets versions list <name>` và [4.2](#42-sa-runtime-cho-mỗi-service) |
| gRPC client từ Cloud Run gọi sang Cloud Run khác bị `unavailable` | Chưa bật HTTP/2 ở target | Thêm `--use-http2` khi deploy service đích |
| Pub/Sub không nhận event | Topic chưa tồn tại trên GCP (chỉ có ở emulator local) | `gcloud pubsub topics create user-events deck-events …` |
| Build timeout sau 10 phút | Build lâu (go workspace lớn) | Thêm `timeout: 1200s` ở `cloudbuild.yaml` và/hoặc cache layer Docker |

---

## Checklist tổng kết

Cứ tick từng cái khi xong:

- [ ] Phần 1: `gcloud` + Docker hoạt động
- [ ] Phần 2: Project tạo xong, API bật xong, billing link
- [ ] Phần 3: Artifact Registry `mempan-services` tồn tại
- [ ] Phần 4: 1 SA cicd-deployer + 9 SA runtime-*
- [ ] Phần 5: Toàn bộ secret từ `app.env` đã đẩy lên Secret Manager
- [ ] Phần 6: Mỗi service có `cloudbuild.yaml`; code đã đọc `$PORT`
- [ ] Phần 7: Smoke test deploy thủ công `auth-service` thành công
- [ ] Phần 8: Trigger GitHub hoạt động — push 1 commit nhỏ thấy build tự chạy
- [ ] Phần 9: Đã set budget alert + cleanup policy

Khi tất cả tick xong → bạn có CI/CD đầy đủ từ `git push` → production trên Cloud Run.
