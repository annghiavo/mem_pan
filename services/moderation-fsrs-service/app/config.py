"""Configuration loader. Reads threshold files DYNAMICALLY at startup.

Rule 1: never hardcode 0.5 or 0.35 anywhere outside this module.
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
    http_port: int
    max_workers: int
    deck_service_addr: str
    notification_service_addr: str
    pubsub_project_id: str
    pubsub_moderation_topic: str
    pubsub_push_secret: str
    fsrs_pool_workers: int = 2
    fallback_vit_id: str = "google/vit-base-patch16-224"


def _load_text_threshold(model_dir: Path) -> float:
    """Read `recall_threshold` from threshold.json. Spec mandates 0.35."""
    path = model_dir / "threshold.json"
    with path.open("r", encoding="utf-8") as f:
        data = json.load(f)
    value = data.get("recall_threshold")
    if value is None:
        # tolerant fallback inside the file itself (not hardcoded constant)
        value = data.get("best_f1_threshold")
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
        http_port=int(os.environ.get("HTTP_PORT", "8087")),
        max_workers=int(os.environ.get("GRPC_MAX_WORKERS", "8")),
        deck_service_addr=os.environ.get("DECK_SERVICE_ADDR", "deck-service:9091"),
        notification_service_addr=os.environ.get(
            "NOTIFICATION_SERVICE_ADDR", "notification-service:9095"
        ),
        pubsub_project_id=os.environ.get("PUBSUB_PROJECT_ID", "local-dev"),
        pubsub_moderation_topic=os.environ.get(
            "PUBSUB_MODERATION_TOPIC", "moderation-events"
        ),
        pubsub_push_secret=os.environ.get("PUBSUB_PUSH_SECRET", "dev-secret"),
        fsrs_pool_workers=int(os.environ.get("FSRS_POOL_WORKERS", "2")),
    )
    log.info(
        "settings loaded: text_thr=%.3f image_thr=%.3f grpc=%d http=%d topic=%s",
        settings.text_threshold,
        settings.image_threshold,
        settings.grpc_port,
        settings.http_port,
        settings.pubsub_moderation_topic,
    )
    return settings
