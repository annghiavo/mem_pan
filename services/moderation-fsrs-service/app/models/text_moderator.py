"""XLM-RoBERTa text moderator. Loaded ONCE at startup (Rule 2).

Empty/whitespace input is short-circuited to CLEAN (Rule 4) before any tensor
work, so the model never sees broken inputs.
"""
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
        # Warm-up: triggers graph fusion so first real call has stable latency.
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
        """Empty/whitespace -> CLEAN with 0.0 confidence. Never raises."""
        texts_list = list(texts)
        results: list[TextVerdict] = [TextVerdict(False, 0.0)] * len(texts_list)
        non_empty_idx: list[int] = []
        non_empty_vals: list[str] = []
        for i, t in enumerate(texts_list):
            stripped = (t or "").strip()
            if stripped:
                non_empty_idx.append(i)
                non_empty_vals.append(stripped)

        for start in range(0, len(non_empty_vals), BATCH):
            chunk = non_empty_vals[start : start + BATCH]
            try:
                probs = self._infer_batch(chunk)
            except Exception:  # noqa: BLE001
                log.exception("text inference failed on batch of %d", len(chunk))
                # Fail-open per safety policy (we'd rather miss than crash deck).
                probs = [0.0] * len(chunk)
            for j, p in enumerate(probs):
                idx = non_empty_idx[start + j]
                results[idx] = TextVerdict(
                    is_violation=p >= self.threshold,
                    confidence=float(p),
                )
        return results
