"""Card-event dispatcher. Triggered by the HTTP Pub/Sub push handler.

Flow:
1. Parse the envelope `{event_type, data(base64-of-inner-json)}`.
2. For card.created / card.updated: run the moderation pipeline on the card's
   text + image.
3. If a violation is detected:
   - gRPC: deck-service.AdminUpdateDeckStatus(deck_id, status="deleted")
   - Pub/Sub publish: "moderation.deck_deleted" -> notification-service + admin-service.
"""
from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import TYPE_CHECKING

from app.events.types import (
    TYPE_CARD_CREATED,
    TYPE_CARD_UPDATED,
    TYPE_MODERATION_DECK_DELETED,
    CardCreatedEvent,
    CardUpdatedEvent,
    ModerationDeckDeletedEvent,
)
from app.utils.metrics import (
    clean_counter,
    violation_counter,
)

if TYPE_CHECKING:
    from app.clients.deck_admin_client import DeckAdminClient
    from app.events.publisher import PubsubPublisher
    from app.models.registry import ModelRegistry

log = logging.getLogger(__name__)


class EventDispatcher:
    def __init__(
        self,
        registry: ModelRegistry,
        deck_admin: DeckAdminClient,
        moderation_publisher: PubsubPublisher,
    ) -> None:
        self.registry = registry
        self.deck_admin = deck_admin
        self.moderation_publisher = moderation_publisher

    # ----- public entrypoint -----

    async def dispatch(self, event_type: str, raw_inner: bytes) -> None:
        if event_type == TYPE_CARD_CREATED:
            event = CardCreatedEvent.model_validate_json(raw_inner)
            await self._moderate_card(
                card_id=event.card_id, deck_id=event.deck_id, user_id=event.user_id,
                content_front=event.content_front, content_back=event.content_back,
                image_url=event.image_url, origin="card.created",
            )
        elif event_type == TYPE_CARD_UPDATED:
            event = CardUpdatedEvent.model_validate_json(raw_inner)
            await self._moderate_card(
                card_id=event.card_id, deck_id=event.deck_id, user_id=event.user_id,
                content_front=event.content_front, content_back=event.content_back,
                image_url=event.image_url, origin="card.updated",
            )
        else:
            log.debug("ignoring event type=%s", event_type)

    # ----- helpers -----

    async def _moderate_card(
        self,
        *,
        card_id: str,
        deck_id: str,
        user_id: str,
        content_front: str,
        content_back: str,
        image_url: str,
        origin: str,
    ) -> None:
        # Run inference synchronously inside the asyncio handler; calls are
        # short enough that thread-pool offload adds more overhead than benefit
        # for a single-card event. (Batch RPC path still uses the executor.)
        text_verdicts = self.registry.text.predict([content_front, content_back])
        image_verdicts = (
            self.registry.image.predict([(card_id, None, image_url)])
            if image_url
            else []
        )

        text_violation = any(v.is_violation for v in text_verdicts)
        image_violation = any(v.is_violation for v in image_verdicts)

        for v in text_verdicts:
            if v.is_violation:
                violation_counter.labels(kind="text").inc()
            else:
                clean_counter.labels(kind="text").inc()
        for v in image_verdicts:
            if v.is_violation:
                violation_counter.labels(kind="image").inc()
            elif not v.decode_error:
                clean_counter.labels(kind="image").inc()

        if not (text_violation or image_violation):
            log.info("clean origin=%s card=%s deck=%s", origin, card_id, deck_id)
            return

        reason = (
            "mixed" if text_violation and image_violation
            else "text_violation" if text_violation
            else "image_violation"
        )
        log.warning(
            "VIOLATION origin=%s card=%s deck=%s user=%s reason=%s",
            origin, card_id, deck_id, user_id, reason,
        )

        # 1) gRPC to deck-service — soft-delete the deck (recall-first: one bad
        #    card deletes the whole deck per spec). The row stays in DB with
        #    content_status='deleted' so admin appeal/restoration is still possible.
        await self.deck_admin.update_deck_status(deck_id=deck_id, status="deleted")

        # 2) Pub/Sub fan-out for notification + audit.
        await self.moderation_publisher.publish(
            event_type=TYPE_MODERATION_DECK_DELETED,
            payload=ModerationDeckDeletedEvent(
                deck_id=deck_id,
                user_id=user_id,
                reason=reason,
                violated_card_ids=[card_id],
                deleted_at=datetime.now(timezone.utc),
            ).model_dump(mode="json"),
        )
