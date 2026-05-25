# Manual Deploy — moderation-fsrs-service

> Python service không nằm trong `deploy.yml` matrix vì cần config riêng:
> gen2 exec env, GCS-FUSE volume mount, 4Gi/2cpu, build PyTorch chậm.
> Deploy thủ công theo các bước dưới.

---

## Prerequisites (làm 1 lần)

```bash
gcloud config set project mempan-cac51
gcloud config set run/region asia-southeast1
gcloud auth configure-docker asia-southeast1-docker.pkg.dev
# Docker desktop / OrbStack đang chạy:
docker info >/dev/null
```

Yêu cầu:
- `mempan-runtime@mempan-cac51.iam.gserviceaccount.com` đã có role
  `roles/secretmanager.secretAccessor` và `roles/storage.objectViewer` trên
  bucket `mempan-cac51-models`.
- Bucket `gs://mempan-cac51-models/` đã có sẵn 2 model:
  - `flashcard_text_moderator/` (XLM-RoBERTa, ~1.1GB)
  - `flashcard_image_moderator/` (ViT-base, ~343MB)
- Secret `pubsub-push-token` tồn tại (dùng chung với các Go service).

---

## Build + push image

```bash
cd services/moderation-fsrs-service

# Tag theo SHA short để dễ rollback. Push thêm `latest` để cache cho lần sau.
SHA=$(git rev-parse --short HEAD)
IMG_BASE="asia-southeast1-docker.pkg.dev/mempan-cac51/mempan-services/moderation-fsrs-service"

docker buildx build \
  --platform=linux/amd64 \
  --push \
  -t "$IMG_BASE:$SHA" \
  -t "$IMG_BASE:latest" \
  --cache-from=type=registry,ref=$IMG_BASE:latest \
  --cache-to=type=inline \
  .
```

Build lần đầu mất ~10-15 phút (tải PyTorch CPU wheels ~700MB). Lần sau
chỉ thay layer code → ~2 phút nếu giữ `--cache-from`.

> **Quên `--platform=linux/amd64`** = Cloud Run sẽ reject vì image arm64
> không chạy được. Đặc biệt khi build từ Mac M-series.

Kiểm tra image đã push:

```bash
gcloud artifacts docker images list \
  asia-southeast1-docker.pkg.dev/mempan-cac51/mempan-services/moderation-fsrs-service \
  --include-tags --limit=3
```

---

## Deploy lên Cloud Run

```bash
SHA=$(git rev-parse --short HEAD)
IMG="asia-southeast1-docker.pkg.dev/mempan-cac51/mempan-services/moderation-fsrs-service:$SHA"

gcloud run deploy moderation-fsrs-service \
  --image="$IMG" \
  --region=asia-southeast1 \
  --platform=managed \
  --service-account=mempan-runtime@mempan-cac51.iam.gserviceaccount.com \
  --execution-environment=gen2 \
  --use-http2 \
  --allow-unauthenticated \
  --port=8080 \
  --memory=4Gi \
  --cpu=2 \
  --cpu-boost \
  --min-instances=0 \
  --max-instances=2 \
  --timeout=300 \
  --add-volume="name=models,type=cloud-storage,bucket=mempan-cac51-models" \
  --add-volume-mount="volume=models,mount-path=/models" \
  --set-secrets="PUBSUB_PUSH_SECRET=pubsub-push-token:latest" \
  --set-env-vars="HTTP_PORT=8080,GRPC_PORT=9090,PUBSUB_PROJECT_ID=mempan-cac51,DECK_SERVICE_ADDR=deck-service-wzed7v5hbq-as.a.run.app:443,TEXT_MODEL_DIR=/models/flashcard_text_moderator,IMAGE_MODEL_DIR=/models/flashcard_image_moderator" \
  --quiet
```

Cờ phải hiểu:
- `--execution-environment=gen2`: **bắt buộc** để có GCS FUSE driver.
- `--add-volume + --add-volume-mount`: mount bucket models vào `/models`
  read-only. Container lazy-read khi cần.
- `--cpu-boost`: cấp CPU cao trong startup phase → giảm cold start từ
  ~70s xuống ~40s.
- `--memory=4Gi`: ViT + XLM-RoBERTa load đồng thời ~3GB. Dưới 4Gi sẽ OOM.
- `--max-instances=2`: cap chi phí. Mỗi instance ~$0.10/giờ khi idle vì
  reserve CPU. Mỗi event card.created mất ~2-5s inference.

---

## Tạo Pub/Sub subscription (1 lần)

Moderation listen trên topic `deck-events` (deck-service publish mọi
card/deck event vào đây). Subscription đẩy push tới `/internal/pubsub`.

```bash
MOD_URL=$(gcloud run services describe moderation-fsrs-service \
  --region=asia-southeast1 --format='value(status.url)')
TOKEN=$(gcloud secrets versions access latest --secret=pubsub-push-token)

gcloud pubsub subscriptions create moderation-deck-events-sub \
  --topic=deck-events \
  --push-endpoint="${MOD_URL}/internal/pubsub?token=${TOKEN}" \
  --ack-deadline=60 \
  --message-retention-duration=1d
```

Kiểm tra:

```bash
gcloud pubsub subscriptions list --filter='name~moderation' \
  --format='value(name,topic,pushConfig.pushEndpoint)'
```

> Sub này dùng `ack-deadline=60s`. Inference + DB call mất ~3-5s nên thừa
> sức ack trong thời hạn. Nếu cold start dài hơn 60s (lần đầu sau khi idle),
> Pub/Sub sẽ retry — handler là idempotent (gọi AdminUpdateDeckStatus
> nhiều lần cũng cho cùng kết quả).

---

## Verify end-to-end

```bash
# 1. Service alive
curl -sS -w "\nHTTP %{http_code}\n" \
  "$(gcloud run services describe moderation-fsrs-service \
       --region=asia-southeast1 --format='value(status.url)')/swagger" | head

# 2. Logs khi container start xong
gcloud logging read \
  'resource.type="cloud_run_revision"
   resource.labels.service_name="moderation-fsrs-service"
   textPayload=~"models loaded"' \
  --limit=3 --freshness=10m \
  --format='value(timestamp,textPayload)'
```

Smoke test trigger thật:

```bash
# Login → token → tạo deck → add 1 card toxic → wait → check deck status
TOKEN=$(curl -sS -X POST \
  https://mempan-gateway-3hd0u0cm.uc.gateway.dev/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"<admin-email>","password":"<pass>"}' | jq -r .accessToken)

DECK=$(curl -sS -X POST \
  https://mempan-gateway-3hd0u0cm.uc.gateway.dev/v1/decks \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"smoke","is_public":false}' | jq -r .deck.deckId)

curl -sS -X POST \
  "https://mempan-gateway-3hd0u0cm.uc.gateway.dev/v1/decks/$DECK/cards" \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"content_front":"fuck you bitch","content_back":"kill yourself"}'

# Đợi ~10 giây, deck phải flip về status=deleted
sleep 15
gcloud logging read \
  "resource.type=\"cloud_run_revision\"
   resource.labels.service_name=\"moderation-fsrs-service\"
   textPayload=~\"$DECK\"" \
  --freshness=2m --format='value(timestamp,textPayload)'
```

Mong đợi log:
```
VIOLATION origin=card.created card=... deck=... reason=text_violation
deck status updated deck=... status=deleted user=...
```

---

## Update models trong GCS

```bash
# Push model mới lên bucket
gcloud storage cp -r ml_model/results/flashcard_text_moderator \
  gs://mempan-cac51-models/

# Restart service để pick up (Cloud Run cache FUSE mount theo revision)
gcloud run services update moderation-fsrs-service \
  --region=asia-southeast1 --clear-base-image --quiet
```

Hoặc trigger 1 deploy mới với cùng image SHA — bất cứ revision mới nào
cũng tạo FUSE mount fresh.

---

## Rollback

```bash
# Xem 5 revision gần nhất
gcloud run revisions list --service=moderation-fsrs-service \
  --region=asia-southeast1 --limit=5

# Route 100% traffic về revision cũ
gcloud run services update-traffic moderation-fsrs-service \
  --region=asia-southeast1 \
  --to-revisions=moderation-fsrs-service-00001-xyz=100
```

---

## Troubleshooting

| Triệu chứng | Nguyên nhân khả dĩ |
|---|---|
| `OOMKilled` trong logs | Memory < 4Gi, hoặc 2 model load đồng thời + request batch lớn |
| Cold start > 90s | Lần đầu sau khi xóa hết revision; FUSE mount phải lazy-read 1.5GB |
| `deck status update failed code=UNAVAILABLE` | `DECK_SERVICE_ADDR` sai, hoặc client dùng `insecure_channel` (xem `app/clients/deck_admin_client.py`) |
| Push sub không deliver | Token mismatch — query `gcloud pubsub subscriptions describe moderation-deck-events-sub` rồi so với secret `pubsub-push-token:latest` |
| Image deploy fail "no matching manifest for linux/amd64" | Quên `--platform=linux/amd64` khi `docker buildx build` |
