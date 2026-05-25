"""ModerationService gRPC implementation. Recall-first verdict aggregation:
ANY item violation -> whole deck flagged as VIOLATION.

Side-effects on violation are unified with the event-driven path so a deck
deleted via direct RPC produces the SAME Pub/Sub fan-out as one deleted via the
`card.created` event consumer:

  - gRPC: deck-service.AdminUpdateDeckStatus(deck_id, "deleted")
  - Pub/Sub: "moderation.deck_deleted" -> notification + admin
"""
from __future__ import annotations

import asyncio
import logging
import time
from datetime import datetime, timezone

import grpc

from app.clients.deck_admin_client import DeckAdminClient
from app.events.publisher import PubsubPublisher
from app.events.types import (
    TYPE_MODERATION_DECK_DELETED,
    ModerationDeckDeletedEvent,
)
from app.models.registry import ModelRegistry
from app.utils.metrics import (
    clean_counter,
    image_latency,
    text_latency,
    violation_counter,
)
from pb import moderation_fsrs_pb2 as pb
from pb import moderation_fsrs_pb2_grpc as pb_grpc

log = logging.getLogger(__name__)


class ModerationServicer(pb_grpc.ModerationServiceServicer):
    def __init__(
        self,
        registry: ModelRegistry,
        deck_admin: DeckAdminClient,
        moderation_publisher: PubsubPublisher,
    ) -> None:
        self.registry = registry
        self.deck_admin = deck_admin
        self.moderation_publisher = moderation_publisher

    async def ModerateDeck(  # noqa: N802 (gRPC naming)
        self,
        request: pb.ModerateDeckRequest,
        context: grpc.aio.ServicerContext,
    ) -> pb.ModerateDeckResponse:
        loop = asyncio.get_running_loop()
        deck_id = request.deck_id
        user_id = request.user_id

        # ---------- Text inference ----------
        text_inputs = [t.content for t in request.texts]
        t0 = time.perf_counter()
        text_verdicts = await loop.run_in_executor(
            None, self.registry.text.predict, text_inputs,
        )
        text_latency.observe(time.perf_counter() - t0)

        # ---------- Image inference ----------
        image_items = [
            (
                img.card_id,
                bytes(img.raw) if img.WhichOneof("source") == "raw" else None,
                img.url if img.WhichOneof("source") == "url" else None,
            )
            for img in request.images
        ]
        t1 = time.perf_counter()
        image_verdicts = await loop.run_in_executor(
            None, self.registry.image.predict, image_items,
        )
        image_latency.observe(time.perf_counter() - t1)

        # ---------- Aggregate (RECALL-FIRST) ----------
        items: list[pb.ItemVerdict] = []
        violated_card_ids: list[str] = []
        text_violation = False
        image_violation = False

        for card_text, v in zip(request.texts, text_verdicts):
            items.append(pb.ItemVerdict(
                card_id=card_text.card_id,
                kind="text",
                is_violation=v.is_violation,
                confidence=v.confidence,
            ))
            if v.is_violation:
                text_violation = True
                violated_card_ids.append(card_text.card_id)
                violation_counter.labels(kind="text").inc()
            else:
                clean_counter.labels(kind="text").inc()

        for card_img, v in zip(request.images, image_verdicts):
            reason = "decode_error" if v.decode_error else ""
            items.append(pb.ItemVerdict(
                card_id=card_img.card_id,
                kind="image",
                is_violation=v.is_violation,
                confidence=v.confidence,
                reason=reason,
            ))
            if v.is_violation:
                image_violation = True
                violated_card_ids.append(card_img.card_id)
                violation_counter.labels(kind="image").inc()
            elif not v.decode_error:
                clean_counter.labels(kind="image").inc()

        any_violation = text_violation or image_violation
        status = (
            pb.MODERATION_STATUS_VIOLATION if any_violation
            else pb.MODERATION_STATUS_CLEAN
        )

        if any_violation:
            reason = (
                "mixed" if text_violation and image_violation
                else "text_violation" if text_violation
                else "image_violation"
            )
            # Fire-and-forget: don't block the RPC response on downstream side-effects.
            asyncio.create_task(self._on_violation(
                deck_id=deck_id, user_id=user_id,
                reason=reason, violated_card_ids=violated_card_ids,
            ))

        log.info(
            "moderate deck=%s user=%s status=%s violated=%d items=%d",
            deck_id, user_id, pb.ModerationStatus.Name(status),
            len(violated_card_ids), len(items),
        )
        return pb.ModerateDeckResponse(deck_id=deck_id, status=status, items=items)

    async def _on_violation(
        self,
        *,
        deck_id: str,
        user_id: str,
        reason: str,
        violated_card_ids: list[str],
    ) -> None:
        try:
            deck_name = await self.deck_admin.update_deck_status(
                deck_id=deck_id, status="deleted",
            )
            await self.moderation_publisher.publish(
                event_type=TYPE_MODERATION_DECK_DELETED,
                payload=ModerationDeckDeletedEvent(
                    deck_id=deck_id, user_id=user_id, deck_name=deck_name,
                    reason=reason, violated_card_ids=violated_card_ids,
                    deleted_at=datetime.now(timezone.utc),
                ).model_dump(mode="json"),
            )
        except Exception:  # noqa: BLE001
            log.exception("violation side-effects failed deck_id=%s", deck_id)
