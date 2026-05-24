"""ViT-base image moderator. Loaded ONCE at startup (Rule 2).

Two safety hatches:
- Rule 4: corrupted bytes / failed HTTP fetch -> ImageVerdict(decode_error=True,
  is_violation=False). Never raises into the gRPC handler.
- Rule 5: empty/broken preprocessor_config.json -> fall back to upstream
  `google/vit-base-patch16-224` processor.
"""
from __future__ import annotations

import io
import logging
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable

import httpx
import torch
from PIL import Image, UnidentifiedImageError
from transformers import AutoImageProcessor, AutoModelForImageClassification

from app.utils.metrics import image_decode_error

log = logging.getLogger(__name__)

IMG_BATCH = 8


@dataclass
class ImageVerdict:
    is_violation: bool
    confidence: float
    decode_error: bool = False


def _load_processor(model_dir: Path, fallback_id: str):
    """Rule 5: fallback to upstream ViT if preprocessor_config.json is empty/broken."""
    cfg_path = model_dir / "preprocessor_config.json"
    try:
        if not cfg_path.exists() or cfg_path.stat().st_size == 0:
            raise ValueError("preprocessor_config.json missing or empty")
        processor = AutoImageProcessor.from_pretrained(str(model_dir))
        if processor is None:
            raise ValueError("processor instantiation returned None")
        return processor
    except Exception as exc:  # noqa: BLE001
        log.warning("preprocessor fallback -> %s (cause: %s)", fallback_id, exc)
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
            self._infer_batch([Image.new("RGB", (224, 224))])
        log.info("ViT-base ready (threshold=%.3f)", threshold)

    @torch.inference_mode()
    def _infer_batch(self, images: list[Image.Image]) -> list[float]:
        inputs = self.processor(images=images, return_tensors="pt")
        inputs = {k: v.to(self.device) for k, v in inputs.items()}
        logits = self.model(**inputs).logits
        probs = torch.softmax(logits, dim=-1)[:, 1]
        return probs.cpu().tolist()

    # ---------- I/O helpers ----------

    def _fetch_bytes(self, url: str) -> bytes:
        with httpx.Client(timeout=self.http_timeout) as client:
            r = client.get(url)
            r.raise_for_status()
            return r.content

    def _decode(self, raw: bytes) -> Image.Image | None:
        try:
            probe = Image.open(io.BytesIO(raw))
            probe.verify()
            return Image.open(io.BytesIO(raw)).convert("RGB")
        except (UnidentifiedImageError, OSError, ValueError) as exc:
            log.warning("image decode failed: %s", exc)
            image_decode_error.inc()
            return None

    # ---------- Public API ----------

    def predict(
        self,
        items: Iterable[tuple[str, bytes | None, str | None]],
    ) -> list[ImageVerdict]:
        """Each item is (card_id, raw_bytes_or_None, url_or_None)."""
        items_list = list(items)
        results: list[ImageVerdict] = [
            ImageVerdict(False, 0.0, decode_error=True) for _ in items_list
        ]
        decoded: list[tuple[int, Image.Image]] = []

        for i, (card_id, raw, url) in enumerate(items_list):
            try:
                payload = raw
                if not payload and url:
                    payload = self._fetch_bytes(url)
                if not payload:
                    log.warning("empty image payload card_id=%s", card_id)
                    image_decode_error.inc()
                    continue
                img = self._decode(payload)
                if img is None:
                    continue
                decoded.append((i, img))
                # placeholder until inference fills it
                results[i] = ImageVerdict(False, 0.0, decode_error=False)
            except httpx.HTTPError as exc:
                log.warning("image fetch failed card_id=%s: %s", card_id, exc)
                image_decode_error.inc()

        for start in range(0, len(decoded), IMG_BATCH):
            chunk = decoded[start : start + IMG_BATCH]
            try:
                probs = self._infer_batch([img for _, img in chunk])
            except Exception:  # noqa: BLE001
                log.exception("image inference failed on batch of %d", len(chunk))
                probs = [0.0] * len(chunk)
            for (i, _), p in zip(chunk, probs):
                results[i] = ImageVerdict(
                    is_violation=p >= self.threshold,
                    confidence=float(p),
                    decode_error=False,
                )
        return results
