"""Versioned dataset schemas — keeps re-training reproducible."""
from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, Field


class LabeledTextSample(BaseModel):
    text: str
    label: int = Field(ge=0, le=1)  # 0 = clean, 1 = toxic
    language: Literal["en", "vi"]
    source: str                     # "jigsaw" | "vihsd" | "user_report"
    schema_version: int = 1


class LabeledImageSample(BaseModel):
    image_path: str                 # local fs or gs:// URI
    label: int = Field(ge=0, le=1)
    source: str                     # "nsfw_kaggle" | "violence_realworld" | "user_report"
    schema_version: int = 1
