"""Lightweight loaders. Kept minimal — real pipelines live in the ml_model repo."""
from __future__ import annotations

import json
from pathlib import Path
from typing import Iterator

from app.dataset.schema import LabeledImageSample, LabeledTextSample


def load_text_jsonl(path: Path) -> Iterator[LabeledTextSample]:
    with path.open("r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            yield LabeledTextSample.model_validate_json(line)


def load_image_jsonl(path: Path) -> Iterator[LabeledImageSample]:
    with path.open("r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            yield LabeledImageSample.model_validate_json(line)


def dump_jsonl(path: Path, samples) -> None:
    with path.open("w", encoding="utf-8") as f:
        for s in samples:
            f.write(json.dumps(s.model_dump()) + "\n")
