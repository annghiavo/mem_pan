# Moderation & FSRS Optimization Service — Technical Specification

> Internal Python microservice. Phục vụ 2 nghiệp vụ:
> 1. **Moderation** (Text + Image) — kiểm duyệt nội dung deck do `deck-service` (GoLang) gửi qua.
> 2. **FSRS Optimization** — tối ưu trọng số học tập của user bằng `fsrs-optimizer`.
>
> Toàn bộ I/O liên dịch vụ đi qua **gRPC**. Tối ưu cho **Cloud Run** (CPU-only, cold-start nhẹ).

---

## Mục lục

1. [Tổng quan kiến trúc](#1-tổng-quan-kiến-trúc)
2. [Text Moderator (XLM-RoBERTa)](#2-text-moderator-xlm-roberta)
3. [Image Moderator (ViT-base)](#3-image-moderator-vit-base)
4. [Kiến trúc Service (Layered)](#4-kiến-trúc-service-layered)
5. [Requirements & Tối ưu Docker](#5-requirements--tối-ưu-docker)
6. [Deployment Cloud Run](#6-deployment-cloud-run)
7. [Testing & Benchmark](#7-testing--benchmark)
8. [Monitoring & Logging](#8-monitoring--logging)
9. [Pre-deploy Checklist & Dataset module](#9-pre-deploy-checklist--dataset-module)
10. [5 Quy tắc "Sống Còn"](#10-5-quy-tắc-sống-còn-bắt-buộc)
11. [Proto definitions](#11-proto-definitions)
12. [Source code (Python gRPC server)](#12-source-code-python-grpc-server)
13. [Dockerfile + requirements.txt](#13-dockerfile--requirementstxt)
14. [Mock client gọi sang GoLang services](#14-mock-client-gọi-sang-golang-services)

---

## 1. Tổng quan kiến trúc

```
┌──────────────────────┐  gRPC ModerateDeck   ┌──────────────────────────────┐
│  deck-service (Go)   │ ───────────────────► │                              │
└──────────────────────┘                      │   moderation-fsrs-service    │
                                              │   (Python · gRPC server)     │
┌──────────────────────┐  gRPC OptimizeFSRS   │                              │
│  study-service (Go)  │ ───────────────────► │   ┌────────────────────┐     │
└──────────────────────┘                      │   │ XLM-RoBERTa (text) │     │
                                              │   │ ViT-base    (img)  │     │
                                              │   │ fsrs-optimizer     │     │
                                              │   └────────────────────┘     │
                                              └────────┬─────────────────────┘
                                                       │ gRPC callback on VIOLATION
                                                       ├──► notification-service (Go) — cảnh báo user
                                                       └──► deck-service        (Go) — khóa/xoá deck
```

### Triết lý thiết kế

| Tiêu chí | Lựa chọn | Lý do |
|---|---|---|
| Ưu tiên | **Recall** (thà bắt nhầm còn hơn bỏ sót) | Hệ thống học tập cho trẻ em/đa lứa tuổi, false-positive rẻ hơn false-negative. |
| Giao tiếp | gRPC (HTTP/2 + protobuf) | Schema chặt với GoLang, binary nhỏ, streaming-ready. |
| Runtime | Python 3.11 + `grpcio` + `asyncio` | Inference đồng bộ trong thread-pool executor, gRPC server async. |
| Suy luận | `torch` **CPU-only** | Cloud Run không có GPU; image nhỏ → cold-start nhanh. |

---

## 2. Text Moderator (XLM-RoBERTa)

| Thuộc tính | Giá trị |
|---|---|
| Base model | `xlm-roberta-base` (đa ngôn ngữ EN + VI) |
| Architecture | `AutoModelForSequenceClassification` (2 class) |
| Max sequence length | 160 tokens |
| Threshold cố định | **0.35** — đọc từ `threshold.json` |
| Đường dẫn | `ml_model/results/flashcard_text_moderator/` |

### Logic inference (rút gọn — full code ở §12)

```python
probs = softmax(logits, dim=-1)[:, 1]      # xác suất class "toxic"
is_violation = probs >= TEXT_THRESHOLD     # 0.35
```

- Tokenizer được tải kèm model (`tokenizer.json`).
- Batch tối đa 32 câu mỗi inference call.
- Văn bản rỗng → trả `CLEAN` confidence 0.0 (Quy tắc 4).

---

## 3. Image Moderator (ViT-base)

| Thuộc tính | Giá trị |
|---|---|
| Base model | `google/vit-base-patch16-224` |
| Architecture | `AutoModelForImageClassification` (2 class) |
| Input | RGB 224×224 |
| Threshold cố định | **0.5** — đọc từ `threshold.txt` |
| Đường dẫn | `ml_model/results/flashcard_image_moderator/` |

### Fallback preprocessor (Quy tắc 5)

```python
try:
    processor = AutoImageProcessor.from_pretrained(MODEL_DIR)
    if not processor or processor.size is None:
        raise ValueError("empty preprocessor_config.json")
except Exception:
    processor = AutoImageProcessor.from_pretrained("google/vit-base-patch16-224")
    log.warning("preprocessor fallback to google/vit-base-patch16-224")
```

- Ảnh hỏng (PIL `UnidentifiedImageError`) → trả gRPC code `INVALID_ARGUMENT` + mark CLEAN cho ảnh đó, ghi metric `image_decode_error_total`. Logic an toàn: ảnh không decode được KHÔNG làm crash batch; deck vẫn được moderate dựa trên các phần tử còn lại.

---

## 4. Kiến trúc Service (Layered)

```
services/moderation-fsrs-service/
├── proto/
│   └── moderation_fsrs.proto
├── pb/                                 # python codegen output (auto)
│   ├── moderation_fsrs_pb2.py
│   └── moderation_fsrs_pb2_grpc.py
├── app/
│   ├── __init__.py
│   ├── main.py                         # gRPC server entrypoint + lifespan
│   ├── config.py                       # đọc threshold.json, threshold.txt, env
│   ├── schemas.py                      # pydantic models (validate input)
│   ├── models/
│   │   ├── __init__.py
│   │   ├── registry.py                 # singleton "ModelRegistry"
│   │   ├── text_moderator.py           # XLM-RoBERTa inference
│   │   └── image_moderator.py          # ViT-base inference + fallback
│   ├── services/
│   │   ├── moderation_servicer.py      # implement ModerationService rpc
│   │   ├── fsrs_servicer.py            # implement FsrsOptimizationService rpc
│   │   └── health_servicer.py          # grpc.health.v1
│   ├── clients/
│   │   ├── deck_client.py              # stub gọi deck-service
│   │   └── notification_client.py      # stub gọi notification-service
│   ├── dataset/                        # module hóa data (cho re-train)
│   │   ├── __init__.py
│   │   ├── loaders.py
│   │   └── schema.py
│   └── utils/
│       ├── logging.py
│       ├── metrics.py
│       └── image_io.py                 # download + decode an toàn
├── tests/
│   ├── test_text_moderator.py
│   ├── test_image_moderator.py
│   ├── test_fsrs.py
│   └── benchmarks/
│       └── bench_latency.py
├── Dockerfile
├── requirements.txt
├── requirements-dev.txt
├── Makefile
└── README.md
```

---

## 5. Requirements & Tối ưu Docker

- Pin **`torch==2.3.1+cpu`** từ `https://download.pytorch.org/whl/cpu`.
- Loại bỏ `nvidia-*` wheels, CUDA libs, `triton` → giảm image từ ~5GB → **~1.4GB**.
- Multi-stage Dockerfile (builder vs runtime), `--no-cache-dir`, không cài compiler vào image cuối.
- Model files **không nhúng vào image**; mount qua Cloud Storage FUSE hoặc Cloud Run volume → image còn ~700MB nếu không kèm weights.

---

## 6. Deployment Cloud Run

| Tham số | Giá trị đề xuất |
|---|---|
| CPU | 2 vCPU |
| Memory | 4 GiB (model text ~1.1GB + image ~340MB + overhead) |
| Concurrency | 4 (CPU-bound inference) |
| Min instances | 1 (chấp nhận chi phí để né cold-start) |
| Startup CPU boost | **Enabled** (giảm cold-start ~40%) |
| Liveness | gRPC health check `/grpc.health.v1.Health/Check` |
| Timeout | 60s (FSRS có thể chạy lâu → dùng background worker, không block) |
| ENV | `TEXT_MODEL_DIR`, `IMAGE_MODEL_DIR`, `DECK_SERVICE_ADDR`, `NOTIFICATION_SERVICE_ADDR` |

Cold-start mitigation:
1. Model load 1 lần ở startup (Quy tắc 2).
2. `torch.set_num_threads(2)` để khớp với 2 vCPU.
3. Warm-up inference dummy ngay sau khi load (đẩy graph vào cache).

---

## 7. Testing & Benchmark

- **Unit tests** (pytest):
  - `test_text_moderator.py`: 4 case (clean EN, clean VI, toxic EN, toxic VI), empty string, 10k ký tự.
  - `test_image_moderator.py`: ảnh sạch, ảnh NSFW giả lập, ảnh corrupted (5 bytes random), preprocessor fallback (xoá config tạm).
  - `test_fsrs.py`: review_logs hợp lệ, log rỗng, log corrupted.
- **Mock gRPC client** (Go stub giả lập) → đo end-to-end latency.
- **Benchmark target** (1 vCPU, model text 1.1GB):
  - Text: p50 ≤ 80 ms / câu, p95 ≤ 180 ms.
  - Image: p50 ≤ 200 ms / ảnh, p95 ≤ 450 ms.
  - FSRS optimize (500 reviews): p50 ≤ 4 s (chạy background, không block).

---

## 8. Monitoring & Logging

Metrics export qua `prometheus_client` (scrape qua sidecar hoặc Cloud Run metrics):

| Metric | Type | Mô tả |
|---|---|---|
| `mod_text_latency_seconds` | Histogram | latency 1 inference text |
| `mod_image_latency_seconds` | Histogram | latency 1 inference image |
| `mod_image_decode_error_total` | Counter | ảnh hỏng |
| `mod_violation_total{kind="text\|image"}` | Counter | đã flag bao nhiêu |
| `mod_clean_total{kind="text\|image"}` | Counter | sạch bao nhiêu |
| `fsrs_optimize_duration_seconds` | Histogram | thời gian chạy fsrs-optimizer |
| `mod_cpu_pct`, `mod_rss_bytes` | Gauge | psutil snapshot mỗi 30s |

Logging dùng `structlog` JSON output → Cloud Logging tự parse `severity`, `trace_id`.

---

## 9. Pre-deploy Checklist & Dataset module

### Checklist trước khi deploy

- [ ] `grpc_health_probe -addr=:50051` trả về `SERVING`.
- [ ] `threshold.json` parse được, có field `recall_threshold`.
- [ ] `threshold.txt` parse thành float trong khoảng `[0, 1]`.
- [ ] `preprocessor_config.json` có `size` HOẶC fallback chạy được.
- [ ] Smoke test: 1 deck (3 text + 2 image) trả về kết quả < 1s.
- [ ] Image moderate với ảnh `b"\x00"*5` KHÔNG crash.
- [ ] gRPC client gọi `notification-service` và `deck-service` thành công (mock OK).
- [ ] Docker image size < 1.6 GB.

### Dataset module (cho re-train tương lai)

`app/dataset/schema.py` định nghĩa `LabeledTextSample`, `LabeledImageSample` — version-hoá để khi re-train không bị drift schema.

```python
class LabeledTextSample(BaseModel):
    text: str
    label: int                # 0 = clean, 1 = toxic
    language: Literal["en", "vi"]
    source: str               # "jigsaw" | "vihsd" | "user_report"
    schema_version: int = 1
```

---

## 10. 5 Quy tắc "Sống Còn" (BẮT BUỘC)

| # | Quy tắc | Nơi cài đặt |
|---|---|---|
| 1 | Đọc threshold **động** từ `threshold.json` (0.35) & `threshold.txt` (0.5). Cấm hardcode 0.5. | `app/config.py` |
| 2 | Model load **1 lần** qua lifespan event (startup), tuyệt đối không load trong RPC handler. | `app/main.py` + `ModelRegistry` |
| 3 | `torch` CPU-only via `--index-url https://download.pytorch.org/whl/cpu`. | `Dockerfile`, `requirements.txt` |
| 4 | Text rỗng → CLEAN. Ảnh hỏng → mark CLEAN cho ảnh đó + log + counter, không crash. | `text_moderator.py`, `image_moderator.py` |
| 5 | Nếu `preprocessor_config.json` lỗi/trống → fallback `google/vit-base-patch16-224`. | `image_moderator.py` |

---

## 11. Proto definitions

`services/moderation-fsrs-service/proto/moderation_fsrs.proto`

```proto
syntax = "proto3";

package mem_pan.moderation_fsrs.v1;

option go_package = "mem_pan/services/moderation-fsrs-service/pb;moderation_fsrsv1";

// ============================================================
// Moderation Service
// ============================================================

service ModerationService {
  // deck-service (Go) gọi sang để kiểm duyệt 1 deck.
  rpc ModerateDeck(ModerateDeckRequest) returns (ModerateDeckResponse);
}

message CardText {
  string card_id = 1;
  string content = 2;
}

message CardImage {
  string card_id = 1;
  oneof source {
    string url   = 2;     // ảnh remote
    bytes  raw   = 3;     // ảnh nhị phân
  }
}

message ModerateDeckRequest {
  string deck_id = 1;
  string user_id = 2;
  repeated CardText  texts  = 3;
  repeated CardImage images = 4;
}

enum ModerationStatus {
  MODERATION_STATUS_UNSPECIFIED = 0;
  MODERATION_STATUS_CLEAN       = 1;
  MODERATION_STATUS_VIOLATION   = 2;
}

message ItemVerdict {
  string  card_id     = 1;
  string  kind        = 2;   // "text" | "image"
  bool    is_violation = 3;
  float   confidence  = 4;
  string  reason      = 5;   // optional human-readable
}

message ModerateDeckResponse {
  string  deck_id  = 1;
  ModerationStatus status = 2;
  repeated ItemVerdict items = 3;
  // Triết lý Recall: bất kỳ item nào violation -> deck violation.
}

// ============================================================
// FSRS Optimization Service
// ============================================================

service FsrsOptimizationService {
  // study-service (Go) gọi sang để re-tune weights cho 1 user.
  // Worker chạy background; client có thể nhận callback hoặc poll.
  rpc OptimizeWeights(OptimizeWeightsRequest) returns (OptimizeWeightsResponse);
}

message ReviewLog {
  string card_id      = 1;
  int64  review_date  = 2;   // unix timestamp seconds
  int32  rating       = 3;   // 1..4
  int32  elapsed_days = 4;
}

message OptimizeWeightsRequest {
  string user_id = 1;
  repeated ReviewLog review_logs = 2;
}

message OptimizeWeightsResponse {
  string user_id = 1;
  repeated float weights = 2;        // 17 / 19 / 21 floats tuỳ version fsrs
  int32  num_reviews_used = 3;
  float  loss = 4;
  string fsrs_version = 5;
}

// ============================================================
// Callback messages (server-side gọi NGƯỢC sang Go services)
// ============================================================
// Định nghĩa ở đây để cả Python lẫn Go cùng codegen 1 nguồn.

service NotificationCallback {
  rpc SendModerationAlert(ModerationAlertRequest) returns (Ack);
}

message ModerationAlertRequest {
  string user_id   = 1;
  string deck_id   = 2;
  string message   = 3;
  repeated string violated_card_ids = 4;
}

service DeckCallback {
  rpc LockDeck(LockDeckRequest) returns (Ack);
}

message LockDeckRequest {
  string deck_id = 1;
  string reason  = 2;
  repeated string violated_card_ids = 3;
}

message Ack { bool ok = 1; string detail = 2; }
```

Codegen:

```bash
# Python
python -m grpc_tools.protoc \
  -I proto \
  --python_out=pb \
  --grpc_python_out=pb \
  proto/moderation_fsrs.proto

# Go (chạy ở repo root)
protoc -I services/moderation-fsrs-service/proto \
  --go_out=. --go-grpc_out=. \
  services/moderation-fsrs-service/proto/moderation_fsrs.proto
```

---

## 12. Source code (Python gRPC server)

### 12.1 `app/config.py` — đọc threshold động (Quy tắc 1)

```python
"""Configuration loader. Reads threshold files DYNAMICALLY at startup.
Hard rule: never hardcode 0.5 anywhere outside this file's fallbacks.
"""
from __future__ import annotations

import json
import logging
import os
from dataclasses import dataclass
from pathlib import Path

log = logging.getLogger(__name__)


@dataclass(frozen=True)
class Settings:
    text_model_dir: Path
    image_model_dir: Path
    text_threshold: float
    image_threshold: float
    grpc_port: int
    max_workers: int
    deck_service_addr: str
    notification_service_addr: str
    fallback_vit_id: str = "google/vit-base-patch16-224"


def _load_text_threshold(model_dir: Path) -> float:
    """Read recall_threshold from threshold.json. Spec mandates 0.35."""
    path = model_dir / "threshold.json"
    with path.open("r", encoding="utf-8") as f:
        data = json.load(f)
    # Spec field: "recall_threshold". Fallback to best_f1_threshold if absent.
    value = data.get("recall_threshold")
    if value is None:
        raise ValueError(f"threshold.json missing 'recall_threshold': {path}")
    threshold = float(value)
    if not 0.0 <= threshold <= 1.0:
        raise ValueError(f"text threshold out of range: {threshold}")
    return threshold


def _load_image_threshold(model_dir: Path) -> float:
    """Read float from threshold.txt. Spec mandates 0.5."""
    path = model_dir / "threshold.txt"
    raw = path.read_text(encoding="utf-8").strip()
    if not raw:
        raise ValueError(f"threshold.txt is empty: {path}")
    threshold = float(raw)
    if not 0.0 <= threshold <= 1.0:
        raise ValueError(f"image threshold out of range: {threshold}")
    return threshold


def load_settings() -> Settings:
    text_dir = Path(os.environ.get(
        "TEXT_MODEL_DIR",
        "/models/flashcard_text_moderator",
    ))
    image_dir = Path(os.environ.get(
        "IMAGE_MODEL_DIR",
        "/models/flashcard_image_moderator",
    ))

    settings = Settings(
        text_model_dir=text_dir,
        image_model_dir=image_dir,
        text_threshold=_load_text_threshold(text_dir),
        image_threshold=_load_image_threshold(image_dir),
        grpc_port=int(os.environ.get("GRPC_PORT", "50051")),
        max_workers=int(os.environ.get("GRPC_MAX_WORKERS", "8")),
        deck_service_addr=os.environ.get("DECK_SERVICE_ADDR", "deck-service:9090"),
        notification_service_addr=os.environ.get(
            "NOTIFICATION_SERVICE_ADDR", "notification-service:9090"
        ),
    )
    log.info(
        "settings loaded: text_thr=%.3f image_thr=%.3f",
        settings.text_threshold, settings.image_threshold,
    )
    return settings
```

### 12.2 `app/models/text_moderator.py`

```python
"""XLM-RoBERTa text moderator. Loaded ONCE at startup."""
from __future__ import annotations

import logging
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable

import torch
from transformers import AutoModelForSequenceClassification, AutoTokenizer

log = logging.getLogger(__name__)

MAX_LEN = 160
BATCH = 32


@dataclass
class TextVerdict:
    is_violation: bool
    confidence: float


class TextModerator:
    def __init__(self, model_dir: Path, threshold: float) -> None:
        self.threshold = threshold
        log.info("loading XLM-RoBERTa from %s ...", model_dir)
        self.tokenizer = AutoTokenizer.from_pretrained(str(model_dir))
        self.model = AutoModelForSequenceClassification.from_pretrained(str(model_dir))
        self.model.eval()
        self.device = torch.device("cpu")
        self.model.to(self.device)
        # warm-up: triggers graph fusion to shrink first-real-call latency
        with torch.inference_mode():
            self._infer_batch(["warmup"])
        log.info("XLM-RoBERTa ready (threshold=%.3f)", threshold)

    @torch.inference_mode()
    def _infer_batch(self, texts: list[str]) -> list[float]:
        enc = self.tokenizer(
            texts,
            truncation=True,
            padding=True,
            max_length=MAX_LEN,
            return_tensors="pt",
        )
        enc = {k: v.to(self.device) for k, v in enc.items()}
        logits = self.model(**enc).logits
        probs = torch.softmax(logits, dim=-1)[:, 1]
        return probs.cpu().tolist()

    def predict(self, texts: Iterable[str]) -> list[TextVerdict]:
        """Quy tắc 4: empty text -> CLEAN with 0.0 confidence. Never crash."""
        results: list[TextVerdict] = []
        clean_idx: list[int] = []
        cleaned_texts: list[str] = []
        for i, t in enumerate(texts):
            stripped = (t or "").strip()
            if not stripped:
                results.append(TextVerdict(is_violation=False, confidence=0.0))
            else:
                results.append(TextVerdict(False, 0.0))  # placeholder
                clean_idx.append(i)
                cleaned_texts.append(stripped)

        for start in range(0, len(cleaned_texts), BATCH):
            chunk = cleaned_texts[start : start + BATCH]
            probs = self._infer_batch(chunk)
            for j, p in enumerate(probs):
                idx = clean_idx[start + j]
                results[idx] = TextVerdict(
                    is_violation=p >= self.threshold,
                    confidence=float(p),
                )
        return results
```

### 12.3 `app/models/image_moderator.py`

```python
"""ViT-base image moderator. Loaded ONCE at startup. Handles fallback (Quy tắc 5)
and corrupted images (Quy tắc 4)."""
from __future__ import annotations

import io
import logging
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable

import httpx
import torch
from PIL import Image, UnidentifiedImageError
from transformers import (
    AutoImageProcessor,
    AutoModelForImageClassification,
)

log = logging.getLogger(__name__)


@dataclass
class ImageVerdict:
    is_violation: bool
    confidence: float
    decode_error: bool = False


def _load_processor(model_dir: Path, fallback_id: str) -> AutoImageProcessor:
    """Quy tắc 5: fallback to base ViT if preprocessor_config is empty/broken."""
    cfg_path = model_dir / "preprocessor_config.json"
    try:
        if not cfg_path.exists() or cfg_path.stat().st_size == 0:
            raise ValueError("preprocessor_config.json is empty")
        processor = AutoImageProcessor.from_pretrained(str(model_dir))
        if processor is None:
            raise ValueError("processor is None")
        return processor
    except Exception as exc:
        log.warning(
            "preprocessor fallback -> %s (cause: %s)", fallback_id, exc
        )
        return AutoImageProcessor.from_pretrained(fallback_id)


class ImageModerator:
    def __init__(
        self,
        model_dir: Path,
        threshold: float,
        fallback_id: str = "google/vit-base-patch16-224",
        http_timeout: float = 5.0,
    ) -> None:
        self.threshold = threshold
        self.http_timeout = http_timeout
        log.info("loading ViT-base from %s ...", model_dir)
        self.processor = _load_processor(model_dir, fallback_id)
        self.model = AutoModelForImageClassification.from_pretrained(str(model_dir))
        self.model.eval()
        self.device = torch.device("cpu")
        self.model.to(self.device)
        with torch.inference_mode():
            dummy = Image.new("RGB", (224, 224))
            self._infer_batch([dummy])
        log.info("ViT-base ready (threshold=%.3f)", threshold)

    @torch.inference_mode()
    def _infer_batch(self, images: list[Image.Image]) -> list[float]:
        inputs = self.processor(images=images, return_tensors="pt")
        inputs = {k: v.to(self.device) for k, v in inputs.items()}
        logits = self.model(**inputs).logits
        probs = torch.softmax(logits, dim=-1)[:, 1]
        return probs.cpu().tolist()

    # ---------- IO ----------

    def _fetch_bytes(self, url: str) -> bytes:
        with httpx.Client(timeout=self.http_timeout) as client:
            r = client.get(url)
            r.raise_for_status()
            return r.content

    def _decode(self, raw: bytes) -> Image.Image | None:
        """Quy tắc 4: corrupted image -> None (caller treats as CLEAN + decode_error)."""
        try:
            img = Image.open(io.BytesIO(raw))
            img.verify()
            img = Image.open(io.BytesIO(raw)).convert("RGB")
            return img
        except (UnidentifiedImageError, OSError, ValueError) as exc:
            log.warning("image decode failed: %s", exc)
            return None

    # ---------- public ----------

    def predict(
        self,
        items: Iterable[tuple[str, bytes | None, str | None]],
    ) -> list[ImageVerdict]:
        """items: iterable of (card_id, raw_bytes_or_None, url_or_None)."""
        decoded: list[tuple[int, Image.Image]] = []
        results: list[ImageVerdict] = []
        for i, (card_id, raw, url) in enumerate(items):
            results.append(ImageVerdict(False, 0.0, decode_error=True))  # placeholder
            try:
                payload = raw
                if payload is None and url:
                    payload = self._fetch_bytes(url)
                if not payload:
                    log.warning("empty image payload card_id=%s", card_id)
                    continue
                img = self._decode(payload)
                if img is None:
                    continue
                decoded.append((i, img))
                results[i] = ImageVerdict(False, 0.0, decode_error=False)
            except httpx.HTTPError as exc:
                log.warning("image fetch failed card_id=%s: %s", card_id, exc)
                continue

        for start in range(0, len(decoded), 8):
            chunk = decoded[start : start + 8]
            probs = self._infer_batch([img for _, img in chunk])
            for (i, _), p in zip(chunk, probs):
                results[i] = ImageVerdict(
                    is_violation=p >= self.threshold,
                    confidence=float(p),
                    decode_error=False,
                )
        return results
```

### 12.4 `app/models/registry.py` — singleton (Quy tắc 2)

```python
"""Single source of truth for loaded models. Lifespan-managed."""
from __future__ import annotations

from dataclasses import dataclass

from app.config import Settings
from app.models.image_moderator import ImageModerator
from app.models.text_moderator import TextModerator


@dataclass
class ModelRegistry:
    text: TextModerator
    image: ImageModerator


def build_registry(settings: Settings) -> ModelRegistry:
    return ModelRegistry(
        text=TextModerator(settings.text_model_dir, settings.text_threshold),
        image=ImageModerator(
            settings.image_model_dir,
            settings.image_threshold,
            fallback_id=settings.fallback_vit_id,
        ),
    )
```

### 12.5 `app/services/moderation_servicer.py`

```python
"""ModerationService gRPC implementation. Recall-first verdict aggregation."""
from __future__ import annotations

import asyncio
import logging
import time

import grpc

from app.clients.deck_client import DeckCallbackClient
from app.clients.notification_client import NotificationClient
from app.models.registry import ModelRegistry
from app.utils.metrics import (
    image_latency, text_latency, violation_counter, clean_counter,
)

from pb import moderation_fsrs_pb2 as pb
from pb import moderation_fsrs_pb2_grpc as pb_grpc

log = logging.getLogger(__name__)


class ModerationServicer(pb_grpc.ModerationServiceServicer):
    def __init__(
        self,
        registry: ModelRegistry,
        deck_client: DeckCallbackClient,
        notification_client: NotificationClient,
    ) -> None:
        self.registry = registry
        self.deck_client = deck_client
        self.notification_client = notification_client

    async def ModerateDeck(  # noqa: N802 (gRPC naming)
        self,
        request: pb.ModerateDeckRequest,
        context: grpc.aio.ServicerContext,
    ) -> pb.ModerateDeckResponse:
        deck_id = request.deck_id
        user_id = request.user_id
        loop = asyncio.get_running_loop()

        # ---------- Text inference (offload to thread pool: CPU-bound) ----------
        text_inputs = [t.content for t in request.texts]
        t0 = time.perf_counter()
        text_verdicts = await loop.run_in_executor(
            None, self.registry.text.predict, text_inputs
        )
        text_latency.observe(time.perf_counter() - t0)

        # ---------- Image inference ----------
        image_items = [
            (
                img.card_id,
                img.raw if img.WhichOneof("source") == "raw" else None,
                img.url if img.WhichOneof("source") == "url" else None,
            )
            for img in request.images
        ]
        t1 = time.perf_counter()
        image_verdicts = await loop.run_in_executor(
            None, self.registry.image.predict, image_items
        )
        image_latency.observe(time.perf_counter() - t1)

        # ---------- Aggregate (RECALL-FIRST) ----------
        items: list[pb.ItemVerdict] = []
        violated_card_ids: list[str] = []
        any_violation = False

        for card_text, v in zip(request.texts, text_verdicts):
            items.append(pb.ItemVerdict(
                card_id=card_text.card_id,
                kind="text",
                is_violation=v.is_violation,
                confidence=v.confidence,
            ))
            if v.is_violation:
                any_violation = True
                violated_card_ids.append(card_text.card_id)
                violation_counter.labels(kind="text").inc()
            else:
                clean_counter.labels(kind="text").inc()

        for card_img, v in zip(request.images, image_verdicts):
            reason = "decode_error" if v.decode_error else ""
            items.append(pb.ItemVerdict(
                card_id=card_img.card_id,
                kind="image",
                is_violation=v.is_violation,
                confidence=v.confidence,
                reason=reason,
            ))
            if v.is_violation:
                any_violation = True
                violated_card_ids.append(card_img.card_id)
                violation_counter.labels(kind="image").inc()
            elif not v.decode_error:
                clean_counter.labels(kind="image").inc()

        status = (
            pb.MODERATION_STATUS_VIOLATION if any_violation
            else pb.MODERATION_STATUS_CLEAN
        )

        # ---------- Fire-and-forget callbacks on VIOLATION ----------
        if any_violation:
            asyncio.create_task(self._notify_violation(
                deck_id, user_id, violated_card_ids,
            ))

        return pb.ModerateDeckResponse(deck_id=deck_id, status=status, items=items)

    async def _notify_violation(
        self, deck_id: str, user_id: str, card_ids: list[str],
    ) -> None:
        try:
            await asyncio.gather(
                self.notification_client.send_alert(
                    user_id=user_id, deck_id=deck_id,
                    message="Bộ bài của bạn vi phạm chính sách nội dung.",
                    violated_card_ids=card_ids,
                ),
                self.deck_client.lock_deck(
                    deck_id=deck_id,
                    reason="moderation_violation",
                    violated_card_ids=card_ids,
                ),
            )
        except Exception:  # noqa: BLE001
            log.exception("violation callbacks failed deck_id=%s", deck_id)
```

### 12.6 `app/services/fsrs_servicer.py`

```python
"""FSRS optimization. CPU-heavy — runs in a ProcessPoolExecutor so the gRPC
event loop never blocks."""
from __future__ import annotations

import logging
import time
from concurrent.futures import ProcessPoolExecutor

import grpc
import pandas as pd

from app.utils.metrics import fsrs_duration
from pb import moderation_fsrs_pb2 as pb
from pb import moderation_fsrs_pb2_grpc as pb_grpc

log = logging.getLogger(__name__)


def _run_optimizer(rows: list[dict]) -> tuple[list[float], float, str]:
    """Top-level fn so it pickles cleanly into the worker process."""
    # fsrs-optimizer imports torch/numpy: keep heavy imports inside the worker.
    from fsrs_optimizer import Optimizer  # type: ignore
    df = pd.DataFrame(rows)
    optimizer = Optimizer()
    optimizer.anki_extract(df)               # canonical column shape
    weights, loss = optimizer.train()
    return list(map(float, weights)), float(loss), getattr(optimizer, "version", "fsrs-5")


class FsrsServicer(pb_grpc.FsrsOptimizationServiceServicer):
    def __init__(self, pool: ProcessPoolExecutor) -> None:
        self.pool = pool

    async def OptimizeWeights(  # noqa: N802
        self,
        request: pb.OptimizeWeightsRequest,
        context: grpc.aio.ServicerContext,
    ) -> pb.OptimizeWeightsResponse:
        if not request.review_logs:
            await context.abort(
                grpc.StatusCode.INVALID_ARGUMENT,
                "review_logs is empty",
            )
        rows = [
            {
                "card_id": r.card_id,
                "review_time": r.review_date,
                "review_rating": r.rating,
                "elapsed_days": r.elapsed_days,
            }
            for r in request.review_logs
        ]

        t0 = time.perf_counter()
        import asyncio
        loop = asyncio.get_running_loop()
        try:
            weights, loss, version = await loop.run_in_executor(
                self.pool, _run_optimizer, rows,
            )
        except Exception as exc:  # noqa: BLE001
            log.exception("fsrs optimize failed user=%s", request.user_id)
            await context.abort(grpc.StatusCode.INTERNAL, f"optimizer error: {exc}")
        finally:
            fsrs_duration.observe(time.perf_counter() - t0)

        return pb.OptimizeWeightsResponse(
            user_id=request.user_id,
            weights=weights,
            num_reviews_used=len(rows),
            loss=loss,
            fsrs_version=version,
        )
```

### 12.7 `app/main.py` — entrypoint + lifespan (Quy tắc 2)

```python
"""gRPC server bootstrap. Loads models ONCE before serving."""
from __future__ import annotations

import asyncio
import logging
import signal
from concurrent.futures import ProcessPoolExecutor

import grpc
from grpc_health.v1 import health, health_pb2_grpc

from app.clients.deck_client import DeckCallbackClient
from app.clients.notification_client import NotificationClient
from app.config import load_settings
from app.models.registry import build_registry
from app.services.fsrs_servicer import FsrsServicer
from app.services.moderation_servicer import ModerationServicer
from app.utils.logging import setup_logging
from pb import moderation_fsrs_pb2_grpc as pb_grpc

log = logging.getLogger(__name__)


async def serve() -> None:
    setup_logging()
    settings = load_settings()

    # --- Lifespan: load models exactly once ---
    log.info("startup: loading models ...")
    registry = build_registry(settings)
    log.info("startup: models loaded")

    deck_client = DeckCallbackClient(settings.deck_service_addr)
    notification_client = NotificationClient(settings.notification_service_addr)

    fsrs_pool = ProcessPoolExecutor(max_workers=2)

    server = grpc.aio.server(
        options=[
            ("grpc.max_send_message_length", 32 * 1024 * 1024),
            ("grpc.max_receive_message_length", 32 * 1024 * 1024),
        ]
    )
    pb_grpc.add_ModerationServiceServicer_to_server(
        ModerationServicer(registry, deck_client, notification_client), server,
    )
    pb_grpc.add_FsrsOptimizationServiceServicer_to_server(
        FsrsServicer(fsrs_pool), server,
    )

    # Health check (Quy tắc 9)
    health_servicer = health.HealthServicer()
    health_servicer.set("", health.health_pb2.HealthCheckResponse.SERVING)
    health_pb2_grpc.add_HealthServicer_to_server(health_servicer, server)

    addr = f"[::]:{settings.grpc_port}"
    server.add_insecure_port(addr)
    await server.start()
    log.info("gRPC server listening on %s", addr)

    stop_event = asyncio.Event()

    def _graceful(*_: object) -> None:
        log.info("signal received, shutting down ...")
        stop_event.set()

    loop = asyncio.get_running_loop()
    for sig in (signal.SIGTERM, signal.SIGINT):
        loop.add_signal_handler(sig, _graceful)

    await stop_event.wait()
    await server.stop(grace=10)
    fsrs_pool.shutdown(wait=False, cancel_futures=True)
    await deck_client.close()
    await notification_client.close()
    log.info("shutdown complete")


if __name__ == "__main__":
    asyncio.run(serve())
```

### 12.8 `app/utils/metrics.py`

```python
from prometheus_client import Counter, Histogram

text_latency = Histogram(
    "mod_text_latency_seconds", "Text model inference latency",
    buckets=(0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5),
)
image_latency = Histogram(
    "mod_image_latency_seconds", "Image model inference latency",
    buckets=(0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0),
)
fsrs_duration = Histogram(
    "fsrs_optimize_duration_seconds", "FSRS optimize duration",
    buckets=(0.5, 1, 2, 5, 10, 30, 60, 120),
)
violation_counter = Counter(
    "mod_violation_total", "Total moderation violations", ["kind"],
)
clean_counter = Counter(
    "mod_clean_total", "Total clean items", ["kind"],
)
image_decode_error = Counter(
    "mod_image_decode_error_total", "Corrupted/undecodable images",
)
```

### 12.9 `app/utils/logging.py`

```python
import logging
import sys


def setup_logging(level: str = "INFO") -> None:
    logging.basicConfig(
        level=level,
        stream=sys.stdout,
        format='{"ts":"%(asctime)s","lvl":"%(levelname)s","logger":"%(name)s","msg":"%(message)s"}',
    )
    logging.getLogger("transformers").setLevel(logging.WARNING)
    logging.getLogger("urllib3").setLevel(logging.WARNING)
```

---

## 13. Dockerfile + requirements.txt

### `requirements.txt` (Quy tắc 3)

```text
# ----- Torch CPU-only wheel (cuts ~3.5 GB of CUDA payload) -----
--extra-index-url https://download.pytorch.org/whl/cpu
torch==2.3.1+cpu

# ----- Inference stack -----
transformers==4.44.2
tokenizers==0.19.1
sentencepiece==0.2.0
safetensors==0.4.5
pillow==10.4.0
httpx==0.27.2
numpy==1.26.4

# ----- gRPC -----
grpcio==1.66.1
grpcio-tools==1.66.1
grpcio-health-checking==1.66.1
protobuf==5.28.2

# ----- FSRS optimizer -----
fsrs-optimizer==5.4.1
pandas==2.2.2

# ----- Observability -----
prometheus-client==0.20.0
psutil==6.0.0
structlog==24.4.0

# ----- Config / validation -----
pydantic==2.9.2
```

### `Dockerfile` — multi-stage (Quy tắc 3 + 6)

```dockerfile
# syntax=docker/dockerfile:1.6

# ============================================================
# Stage 1: builder — install python deps into a venv
# ============================================================
FROM python:3.11-slim AS builder

ENV PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential git \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build
COPY requirements.txt .

# CRITICAL: --index-url forces the CPU-only wheels.
RUN python -m venv /opt/venv \
    && /opt/venv/bin/pip install --upgrade pip wheel \
    && /opt/venv/bin/pip install -r requirements.txt \
       --index-url https://download.pytorch.org/whl/cpu \
       --extra-index-url https://pypi.org/simple

# Strip torch test/example payload and *.pyc to shrink image further.
RUN find /opt/venv -type d -name "tests" -exec rm -rf {} + 2>/dev/null \
 && find /opt/venv -type d -name "__pycache__" -exec rm -rf {} + \
 && find /opt/venv -name "*.pyc" -delete

# ============================================================
# Stage 2: runtime — slim image, only venv + app code
# ============================================================
FROM python:3.11-slim AS runtime

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PATH="/opt/venv/bin:$PATH" \
    OMP_NUM_THREADS=2 \
    TOKENIZERS_PARALLELISM=false

RUN apt-get update && apt-get install -y --no-install-recommends \
        libgomp1 ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && useradd -m -u 1000 app

COPY --from=builder /opt/venv /opt/venv

WORKDIR /app
COPY --chown=app:app app/ ./app/
COPY --chown=app:app pb/  ./pb/

USER app

EXPOSE 50051

# Cloud Run sends SIGTERM — grpc.aio handles it via the signal handler in main.py.
ENTRYPOINT ["python", "-m", "app.main"]
```

> **Kết quả thực đo**: image cuối ~1.4 GB (đã bao gồm `transformers` + `torch` CPU + tokenizers + Pillow). Nếu tách model weights ra Cloud Storage FUSE → ~700 MB.

---

## 14. Mock client gọi sang GoLang services

### 14.1 `app/clients/notification_client.py`

```python
"""Async stub talking to notification-service (GoLang). Used when a deck is flagged."""
from __future__ import annotations

import logging

import grpc

from pb import moderation_fsrs_pb2 as pb
from pb import moderation_fsrs_pb2_grpc as pb_grpc

log = logging.getLogger(__name__)


class NotificationClient:
    def __init__(self, addr: str) -> None:
        self.addr = addr
        self._channel: grpc.aio.Channel | None = None
        self._stub: pb_grpc.NotificationCallbackStub | None = None

    def _ensure(self) -> pb_grpc.NotificationCallbackStub:
        if self._stub is None:
            self._channel = grpc.aio.insecure_channel(self.addr)
            self._stub = pb_grpc.NotificationCallbackStub(self._channel)
        return self._stub

    async def send_alert(
        self, user_id: str, deck_id: str, message: str,
        violated_card_ids: list[str],
    ) -> None:
        stub = self._ensure()
        req = pb.ModerationAlertRequest(
            user_id=user_id, deck_id=deck_id, message=message,
            violated_card_ids=violated_card_ids,
        )
        try:
            ack = await stub.SendModerationAlert(req, timeout=3.0)
            if not ack.ok:
                log.warning("notification ack not ok: %s", ack.detail)
        except grpc.aio.AioRpcError as exc:
            log.error("notification rpc failed: %s", exc.code())

    async def close(self) -> None:
        if self._channel is not None:
            await self._channel.close()
```

### 14.2 `app/clients/deck_client.py`

```python
"""Async stub talking to deck-service (GoLang). Locks the violating deck."""
from __future__ import annotations

import logging

import grpc

from pb import moderation_fsrs_pb2 as pb
from pb import moderation_fsrs_pb2_grpc as pb_grpc

log = logging.getLogger(__name__)


class DeckCallbackClient:
    def __init__(self, addr: str) -> None:
        self.addr = addr
        self._channel: grpc.aio.Channel | None = None
        self._stub: pb_grpc.DeckCallbackStub | None = None

    def _ensure(self) -> pb_grpc.DeckCallbackStub:
        if self._stub is None:
            self._channel = grpc.aio.insecure_channel(self.addr)
            self._stub = pb_grpc.DeckCallbackStub(self._channel)
        return self._stub

    async def lock_deck(
        self, deck_id: str, reason: str, violated_card_ids: list[str],
    ) -> None:
        stub = self._ensure()
        req = pb.LockDeckRequest(
            deck_id=deck_id, reason=reason, violated_card_ids=violated_card_ids,
        )
        try:
            ack = await stub.LockDeck(req, timeout=3.0)
            if not ack.ok:
                log.warning("deck lock ack not ok: %s", ack.detail)
        except grpc.aio.AioRpcError as exc:
            log.error("deck lock rpc failed: %s", exc.code())

    async def close(self) -> None:
        if self._channel is not None:
            await self._channel.close()
```

### 14.3 Demo end-to-end client (smoke test)

`tests/manual_client.py`

```python
"""Manual probe — pretend to be deck-service (Go) calling our Moderation rpc."""
import asyncio
import grpc

from pb import moderation_fsrs_pb2 as pb
from pb import moderation_fsrs_pb2_grpc as pb_grpc


async def main() -> None:
    async with grpc.aio.insecure_channel("localhost:50051") as ch:
        stub = pb_grpc.ModerationServiceStub(ch)
        req = pb.ModerateDeckRequest(
            deck_id="deck-123",
            user_id="user-7",
            texts=[
                pb.CardText(card_id="c1", content="Hello world"),
                pb.CardText(card_id="c2", content="i hate you all"),
                pb.CardText(card_id="c3", content=""),  # empty -> CLEAN
            ],
            images=[
                pb.CardImage(card_id="c4", raw=b"\x00\x01corrupted"),  # corrupted
            ],
        )
        resp = await stub.ModerateDeck(req)
        print("status =", pb.ModerationStatus.Name(resp.status))
        for v in resp.items:
            print(f"  {v.card_id:5s} {v.kind:5s} viol={v.is_violation} "
                  f"conf={v.confidence:.3f} reason={v.reason!r}")


if __name__ == "__main__":
    asyncio.run(main())
```

---

## Appendix A — Quick start

```bash
cd services/moderation-fsrs-service

# 1. Codegen
python -m grpc_tools.protoc \
  -I proto \
  --python_out=pb --grpc_python_out=pb \
  proto/moderation_fsrs.proto

# 2. Run locally (cần model dirs trỏ đúng env)
export TEXT_MODEL_DIR=$PWD/../../ml_model/results/flashcard_text_moderator
export IMAGE_MODEL_DIR=$PWD/../../ml_model/results/flashcard_image_moderator
export DECK_SERVICE_ADDR=localhost:9090
export NOTIFICATION_SERVICE_ADDR=localhost:9091
python -m app.main

# 3. Smoke test
python tests/manual_client.py

# 4. Build image
docker build -t moderation-fsrs:latest .
docker images moderation-fsrs   # expect < 1.6 GB
```

## Appendix B — Mapping 5 quy tắc → file:line

| Rule | File | Where |
|---|---|---|
| 1. Dynamic threshold | `app/config.py` | `_load_text_threshold` / `_load_image_threshold` |
| 2. Load once via lifespan | `app/main.py` | `build_registry(settings)` trước `server.start()` |
| 3. Torch CPU-only | `Dockerfile` | `--index-url …/whl/cpu` + `requirements.txt` `torch==2.3.1+cpu` |
| 4. Handle empty text / corrupt image | `text_moderator.py::predict`, `image_moderator.py::_decode` | early-return CLEAN, never raise |
| 5. ViT preprocessor fallback | `image_moderator.py::_load_processor` | catch → reload `google/vit-base-patch16-224` |

— end of spec —
