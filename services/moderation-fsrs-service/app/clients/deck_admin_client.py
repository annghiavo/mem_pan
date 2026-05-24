"""Async gRPC client for deck-service's existing `AdminUpdateDeckStatus` RPC.

This uses the deck-service proto contract, NOT the moderation_fsrs.proto
DeckCallback. We talk to the real production endpoint that the admin tools
also call so there's exactly one code path that flips a deck's status.

The .proto for deck-service lives in services/deck-service/proto/. We do NOT
ship the Go pb here — instead the client builds the request as a raw protobuf
message via grpc's generic Channel API. That keeps this Python service
decoupled from regenerating Go stubs.
"""
from __future__ import annotations

import logging

import grpc

# Locally-generated stub for the small AdminUpdateDeckStatus contract.
# See `proto/deck_admin.proto` in this service for the canonical schema.
from pb import deck_admin_pb2 as pb
from pb import deck_admin_pb2_grpc as pb_grpc

log = logging.getLogger(__name__)


class DeckAdminClient:
    def __init__(self, addr: str) -> None:
        self.addr = addr
        self._channel: grpc.aio.Channel | None = None
        self._stub: pb_grpc.DeckServiceStub | None = None

    def _ensure(self) -> pb_grpc.DeckServiceStub:
        if self._stub is None:
            self._channel = grpc.aio.insecure_channel(self.addr)
            self._stub = pb_grpc.DeckServiceStub(self._channel)
        return self._stub

    async def update_deck_status(self, deck_id: str, status: str) -> None:
        """status must be one of: active | hidden | deleted."""
        stub = self._ensure()
        req = pb.AdminUpdateDeckStatusRequest(deck_id=deck_id, status=status)
        try:
            resp = await stub.AdminUpdateDeckStatus(req, timeout=3.0)
            log.info(
                "deck status updated deck=%s status=%s user=%s",
                resp.deck_id, resp.status, resp.user_id,
            )
        except grpc.aio.AioRpcError as exc:
            log.error(
                "deck status update failed deck=%s code=%s details=%s",
                deck_id, exc.code(), exc.details(),
            )

    async def close(self) -> None:
        if self._channel is not None:
            await self._channel.close()
            self._channel = None
            self._stub = None
