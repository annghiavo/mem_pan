"""Pydantic schemas — used by tests/CLI tools that want a non-gRPC entrypoint."""
from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, Field


class CardTextIn(BaseModel):
    card_id: str
    content: str


class CardImageIn(BaseModel):
    card_id: str
    url: str | None = None
    raw: bytes | None = None


class ModerateDeckIn(BaseModel):
    deck_id: str
    user_id: str
    texts: list[CardTextIn] = Field(default_factory=list)
    images: list[CardImageIn] = Field(default_factory=list)


class ItemVerdictOut(BaseModel):
    card_id: str
    kind: Literal["text", "image"]
    is_violation: bool
    confidence: float
    reason: str = ""


class ModerateDeckOut(BaseModel):
    deck_id: str
    status: Literal["CLEAN", "VIOLATION", "UNSPECIFIED"]
    items: list[ItemVerdictOut]


class ReviewLogIn(BaseModel):
    card_id: str
    review_date: int
    rating: int = Field(ge=1, le=4)
    elapsed_days: int = Field(ge=0)


class OptimizeWeightsIn(BaseModel):
    user_id: str
    review_logs: list[ReviewLogIn]


class OptimizeWeightsOut(BaseModel):
    user_id: str
    weights: list[float]
    num_reviews_used: int
    loss: float
    fsrs_version: str
