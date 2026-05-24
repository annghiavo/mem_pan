"""End-to-end wiring test: simulate the full card.created -> moderation ->
deck-service.AdminUpdateDeckStatus + Pub/Sub moderation-events fan-out.

This does NOT load the real torch models — it injects fake verdicts into the
EventDispatcher and stands up:
- a tiny gRPC server impersonating deck-service.AdminUpdateDeckStatus
- a tiny aiohttp server impersonating the Pub/Sub emulator's `:publish` endpoint
- the moderation HTTP server (the real `create_app`) receiving a push envelope

If all three pieces light up in sequence, the wiring is correct end-to-end.
"""
from __future__ import annotations

import asyncio
import base64
import json
from dataclasses import dataclass, field
from datetime import datetime, timezone

import grpc
import pytest
from aiohttp import web
from aiohttp.test_utils import TestClient, TestServer

from app.clients.deck_admin_client import DeckAdminClient
from app.events.dispatcher import EventDispatcher
from app.events.publisher import PubsubPublisher
from app.events.types import (
    TYPE_CARD_CREATED,
    TYPE_MODERATION_DECK_DELETED,
)
from app.http_server import create_app
from pb import deck_admin_pb2 as deck_pb
from pb import deck_admin_pb2_grpc as deck_pb_grpc


# ---------- Fakes ----------


@dataclass
class FakeVerdict:
    is_violation: bool
    confidence: float
    decode_error: bool = False


class FakeText:
    def __init__(self, verdicts):
        self.verdicts = list(verdicts)

    def predict(self, texts):
        out = self.verdicts[: len(list(texts))]
        self.verdicts = self.verdicts[len(out):]
        return out


class FakeImage:
    def __init__(self, verdicts):
        self.verdicts = list(verdicts)

    def predict(self, items):
        items = list(items)
        out = self.verdicts[: len(items)]
        self.verdicts = self.verdicts[len(items):]
        return out


@dataclass
class FakeRegistry:
    text: FakeText
    image: FakeImage


# ---------- Mock deck-service gRPC ----------


class _MockDeckServicer(deck_pb_grpc.DeckServiceServicer):
    def __init__(self) -> None:
        self.calls: list[tuple[str, str]] = []

    async def AdminUpdateDeckStatus(self, request, context):  # noqa: N802
        self.calls.append((request.deck_id, request.status))
        return deck_pb.AdminUpdateDeckStatusResponse(
            deck_id=request.deck_id, status=request.status, user_id="fake-user-id",
        )


async def _start_mock_deck() -> tuple[grpc.aio.Server, _MockDeckServicer, str]:
    servicer = _MockDeckServicer()
    server = grpc.aio.server()
    deck_pb_grpc.add_DeckServiceServicer_to_server(servicer, server)
    port = server.add_insecure_port("127.0.0.1:0")
    await server.start()
    return server, servicer, f"127.0.0.1:{port}"


# ---------- Mock Pub/Sub emulator (only the :publish endpoint) ----------


@dataclass
class _MockPubsub:
    captured: list[dict] = field(default_factory=list)

    async def handle(self, request: web.Request) -> web.Response:
        body = await request.json()
        for msg in body.get("messages", []):
            self.captured.append(msg)
        return web.json_response({"messageIds": ["mock-id"]})


async def _start_mock_pubsub() -> tuple[TestClient, _MockPubsub]:
    state = _MockPubsub()
    app = web.Application()
    app.router.add_post(
        "/v1/projects/{project}/topics/{topic}:publish",
        state.handle,
    )
    server = TestServer(app)
    client = TestClient(server)
    await client.start_server()
    return client, state


# ---------- The test ----------


def test_card_created_violation_triggers_grpc_and_pubsub():
    async def run() -> None:
        deck_server, deck_servicer, deck_addr = await _start_mock_deck()
        pubsub_client, pubsub_state = await _start_mock_pubsub()
        try:
            # Wire the moderation service against the mock backends.
            deck_admin = DeckAdminClient(addr=deck_addr)
            host = pubsub_client.host
            port = pubsub_client.port
            publisher = PubsubPublisher(
                project_id="local-dev",
                topic_id="moderation-events",
                emulator_host=f"{host}:{port}",
            )
            registry = FakeRegistry(
                text=FakeText([FakeVerdict(True, 0.93), FakeVerdict(False, 0.05)]),
                image=FakeImage([]),
            )
            dispatcher = EventDispatcher(
                registry=registry,
                deck_admin=deck_admin,
                moderation_publisher=publisher,
            )
            app = create_app(dispatcher, push_secret="dev-secret")
            mod_server = TestServer(app)
            mod_client = TestClient(mod_server)
            await mod_client.start_server()
            try:
                inner = json.dumps({
                    "card_id": "card-X",
                    "deck_id": "deck-Y",
                    "user_id": "user-Z",
                    "note_id": "note-1",
                    "content_front": "abusive content!",
                    "content_back": "",
                    "image_url": "",
                    "created_at": "2026-05-24T08:00:00Z",
                }).encode("utf-8")
                envelope = json.dumps({
                    "event_type": TYPE_CARD_CREATED,
                    "data": base64.b64encode(inner).decode("ascii"),
                }).encode("utf-8")
                push_body = {
                    "message": {
                        "data": base64.b64encode(envelope).decode("ascii"),
                        "messageId": "test-msg-1",
                        "publishTime": "2026-05-24T08:00:00Z",
                    },
                }

                resp = await mod_client.post(
                    "/internal/pubsub",
                    params={"token": "dev-secret"},
                    json=push_body,
                )
                assert resp.status == 204

                # The dispatcher fires both side effects synchronously, so by
                # the time the HTTP response is back, both mocks have observed
                # the call.
                assert deck_servicer.calls == [("deck-Y", "deleted")], \
                    f"deck-service should have been called; got {deck_servicer.calls}"
                assert len(pubsub_state.captured) == 1, \
                    f"pubsub should have one message; got {pubsub_state.captured}"

                # Decode the outer-base64 -> envelope -> inner-base64 -> payload
                msg = pubsub_state.captured[0]
                outer = base64.b64decode(msg["data"])
                env = json.loads(outer)
                assert env["event_type"] == TYPE_MODERATION_DECK_DELETED
                inner_payload = json.loads(base64.b64decode(env["data"]))
                assert inner_payload["deck_id"] == "deck-Y"
                assert inner_payload["user_id"] == "user-Z"
                assert inner_payload["reason"] == "text_violation"
                assert inner_payload["violated_card_ids"] == ["card-X"]
            finally:
                await mod_client.close()
                await deck_admin.close()
                await publisher.close()
        finally:
            await deck_server.stop(grace=1)
            await pubsub_client.close()

    asyncio.run(run())


def test_clean_card_skips_grpc_and_pubsub():
    async def run() -> None:
        deck_server, deck_servicer, deck_addr = await _start_mock_deck()
        pubsub_client, pubsub_state = await _start_mock_pubsub()
        try:
            deck_admin = DeckAdminClient(addr=deck_addr)
            publisher = PubsubPublisher(
                project_id="local-dev",
                topic_id="moderation-events",
                emulator_host=f"{pubsub_client.host}:{pubsub_client.port}",
            )
            registry = FakeRegistry(
                text=FakeText([FakeVerdict(False, 0.02), FakeVerdict(False, 0.05)]),
                image=FakeImage([]),
            )
            dispatcher = EventDispatcher(
                registry=registry,
                deck_admin=deck_admin,
                moderation_publisher=publisher,
            )
            app = create_app(dispatcher, push_secret="dev-secret")
            mod_server = TestServer(app)
            mod_client = TestClient(mod_server)
            await mod_client.start_server()
            try:
                inner = json.dumps({
                    "card_id": "card-A",
                    "deck_id": "deck-B",
                    "user_id": "user-C",
                    "note_id": "note-1",
                    "content_front": "what a beautiful morning",
                    "content_back": "rất đẹp",
                }).encode("utf-8")
                envelope = json.dumps({
                    "event_type": TYPE_CARD_CREATED,
                    "data": base64.b64encode(inner).decode("ascii"),
                }).encode("utf-8")
                push_body = {"message": {
                    "data": base64.b64encode(envelope).decode("ascii"),
                    "messageId": "test-msg-2",
                    "publishTime": "2026-05-24T08:00:00Z",
                }}
                resp = await mod_client.post(
                    "/internal/pubsub",
                    params={"token": "dev-secret"},
                    json=push_body,
                )
                assert resp.status == 204
                assert deck_servicer.calls == []
                assert pubsub_state.captured == []
            finally:
                await mod_client.close()
                await deck_admin.close()
                await publisher.close()
        finally:
            await deck_server.stop(grace=1)
            await pubsub_client.close()

    asyncio.run(run())
