"""Event payloads shared with the GoLang publishers/consumers.

Keep these JSON-equivalent to the structs in:
- services/deck-service/internal/publisher/publisher.go
- services/notification-service/internal/events/types.go
"""
from __future__ import annotations

from datetime import datetime
from typing import Literal, Optional

from pydantic import BaseModel, ConfigDict, Field


# ---------- Event-type constants (string keys on the wire) ----------

TYPE_CARD_CREATED = "card.created"
TYPE_CARD_UPDATED = "card.updated"
TYPE_CARD_DELETED = "card.deleted"

TYPE_MODERATION_DECK_DELETED = "moderation.deck_deleted"


# ---------- Inbound (from deck-service) ----------


class CardCreatedEvent(BaseModel):
    model_config = ConfigDict(extra="ignore", populate_by_name=True)

    card_id: str
    deck_id: str
    user_id: str
    note_id: str
    content_front: str = ""
    content_back: str = ""
    image_url: str = ""
    created_at: Optional[datetime] = None


class CardUpdatedEvent(BaseModel):
    model_config = ConfigDict(extra="ignore", populate_by_name=True)

    card_id: str
    deck_id: str
    user_id: str
    note_id: str
    content_front: str = ""
    content_back: str = ""
    image_url: str = ""


# ---------- Outbound (moderation -> notification + admin) ----------


class ModerationDeckDeletedEvent(BaseModel):
    deck_id: str
    user_id: str
    reason: Literal["text_violation", "image_violation", "mixed"]
    violated_card_ids: list[str] = Field(default_factory=list)
    deleted_at: datetime
    moderator_version: str = "xlm-roberta-1.0+vit-base-1.0"
