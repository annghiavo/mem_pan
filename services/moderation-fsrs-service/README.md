# moderation-fsrs-service

Internal Python gRPC service. Two RPCs:

- `ModerationService.ModerateDeck` — text (XLM-RoBERTa) + image (ViT-base) moderation.
- `FsrsOptimizationService.OptimizeWeights` — periodic FSRS weight tuning.

Spec: [`doc/MODERATION_SERVICE_SPEC.md`](../../doc/MODERATION_SERVICE_SPEC.md).

## Layout

```
.
├── proto/moderation_fsrs.proto    # shared with GoLang clients
├── pb/                            # generated stubs (Python)
├── app/
│   ├── main.py                    # lifespan: load models once, then serve
│   ├── config.py                  # dynamic threshold loader
│   ├── models/                    # text + image inference wrappers
│   ├── services/                  # gRPC servicers
│   ├── clients/                   # callback clients -> Go services
│   └── dataset/                   # versioned re-train schemas
├── tests/
└── Dockerfile
```

## Local dev

```bash
# 1. install (CPU-only torch wheel)
make venv
make install

# 2. codegen
make proto

# 3. point at model dirs and run
export TEXT_MODEL_DIR=$PWD/../../ml_model/results/flashcard_text_moderator
export IMAGE_MODEL_DIR=$PWD/../../ml_model/results/flashcard_image_moderator
make run

# 4. smoke test in another terminal
python tests/manual_client.py
```

## Tests

```bash
make proto        # tests import from pb/, so generate stubs first
make test         # unit tests; do NOT load real model weights
```

## Docker

```bash
make docker
make docker-size   # expect ~1.4 GB
```

## Environment variables

| Var | Default | Purpose |
|---|---|---|
| `TEXT_MODEL_DIR` | `/models/flashcard_text_moderator` | XLM-RoBERTa weights dir |
| `IMAGE_MODEL_DIR` | `/models/flashcard_image_moderator` | ViT-base weights dir |
| `GRPC_PORT` | `50051` | listen port |
| `GRPC_MAX_WORKERS` | `8` | thread pool for sync inference offload |
| `FSRS_POOL_WORKERS` | `2` | process pool for fsrs-optimizer |
| `DECK_SERVICE_ADDR` | `deck-service:9090` | Go callback target |
| `NOTIFICATION_SERVICE_ADDR` | `notification-service:9090` | Go callback target |

## 5 critical rules

| # | Rule | Where |
|---|---|---|
| 1 | thresholds from disk, never hardcoded | `app/config.py` |
| 2 | models load once at startup | `app/main.py` → `build_registry()` before `server.start()` |
| 3 | torch CPU-only wheel | `Dockerfile` `--index-url …/whl/cpu` |
| 4 | empty text / corrupt image -> CLEAN (no crash) | `text_moderator.py::predict`, `image_moderator.py::_decode` |
| 5 | preprocessor fallback to `google/vit-base-patch16-224` | `image_moderator.py::_load_processor` |

## Health check

The server registers `grpc.health.v1.Health`. From the Cloud Run readiness probe:

```
grpc_health_probe -addr=:50051
```
