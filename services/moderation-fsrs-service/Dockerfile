# syntax=docker/dockerfile:1.6

# ============================================================
# Stage 1: builder
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

# CRITICAL (Rule 3): pin to the CPU-only PyTorch wheel index.
RUN python -m venv /opt/venv \
 && /opt/venv/bin/pip install --upgrade pip wheel \
 && /opt/venv/bin/pip install -r requirements.txt \
        --index-url https://download.pytorch.org/whl/cpu \
        --extra-index-url https://pypi.org/simple

# Shrink the venv — remove tests, caches, *.pyc.
RUN find /opt/venv -type d -name "tests" -exec rm -rf {} + 2>/dev/null \
 && find /opt/venv -type d -name "__pycache__" -exec rm -rf {} + \
 && find /opt/venv -name "*.pyc" -delete

# Codegen pb (so the image carries pre-generated stubs, no protoc at runtime).
COPY proto/ /build/proto/
RUN mkdir -p /build/pb \
 && /opt/venv/bin/python -m grpc_tools.protoc \
        -I /build/proto \
        --python_out=/build/pb \
        --grpc_python_out=/build/pb \
        /build/proto/moderation_fsrs.proto \
        /build/proto/deck_admin.proto \
 && touch /build/pb/__init__.py

# ============================================================
# Stage 2: runtime
# ============================================================
FROM python:3.11-slim AS runtime

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PATH="/opt/venv/bin:$PATH" \
    OMP_NUM_THREADS=2 \
    TOKENIZERS_PARALLELISM=false \
    GRPC_PORT=50051

RUN apt-get update && apt-get install -y --no-install-recommends \
        libgomp1 ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && useradd -m -u 1000 app

COPY --from=builder /opt/venv /opt/venv
COPY --from=builder /build/pb /app/pb

WORKDIR /app
COPY --chown=app:app app/ ./app/

USER app

EXPOSE 50051 8087

# Cloud Run sends SIGTERM; handled by signal hooks in app.main.
ENTRYPOINT ["python", "-m", "app.main"]
