"""HTTP server smoke tests: Pub/Sub push envelope parsing + token gating."""
from __future__ import annotations

import asyncio
import base64
import json
from dataclasses import dataclass

import pytest
from aiohttp.test_utils import TestClient, TestServer

from app.http_server import create_app


@dataclass
class _Captured:
    event_type: str
    inner: bytes


class StubDispatcher:
    def __init__(self) -> None:
        self.captured: list[_Captured] = []

    async def dispatch(self, event_type: str, inner: bytes) -> None:
        self.captured.append(_Captured(event_type, inner))


def _build_push_body(event_type: str, payload: dict) -> dict:
    inner = json.dumps(payload).encode("utf-8")
    envelope = json.dumps({
        "event_type": event_type,
        "data": base64.b64encode(inner).decode("ascii"),
    }).encode("utf-8")
    return {
        "message": {
            "data": base64.b64encode(envelope).decode("ascii"),
            "messageId": "test-1",
            "publishTime": "2025-05-24T00:00:00Z",
        },
    }


async def _serve(dispatcher: StubDispatcher, secret: str):
    app = create_app(dispatcher, secret)
    server = TestServer(app)
    client = TestClient(server)
    await client.start_server()
    return client


def test_pubsub_push_dispatches_inner_payload() -> None:
    async def run() -> None:
        disp = StubDispatcher()
        client = await _serve(disp, "dev-secret")
        try:
            body = _build_push_body("card.created", {
                "card_id": "c1", "deck_id": "d1", "user_id": "u1", "note_id": "n1",
                "content_front": "hello",
            })
            resp = await client.post("/internal/pubsub", params={"token": "dev-secret"}, json=body)
            assert resp.status == 204
            assert len(disp.captured) == 1
            assert disp.captured[0].event_type == "card.created"
            inner = json.loads(disp.captured[0].inner)
            assert inner["card_id"] == "c1"
        finally:
            await client.close()
    asyncio.run(run())


def test_pubsub_push_rejects_bad_token() -> None:
    async def run() -> None:
        disp = StubDispatcher()
        client = await _serve(disp, "dev-secret")
        try:
            resp = await client.post(
                "/internal/pubsub",
                params={"token": "wrong"},
                json=_build_push_body("card.created", {"card_id": "x"}),
            )
            assert resp.status == 401
            assert disp.captured == []
        finally:
            await client.close()
    asyncio.run(run())


def test_healthz_endpoint() -> None:
    async def run() -> None:
        disp = StubDispatcher()
        client = await _serve(disp, "dev-secret")
        try:
            resp = await client.get("/healthz")
            assert resp.status == 200
            text = await resp.text()
            assert text == "ok"
        finally:
            await client.close()
    asyncio.run(run())


def test_metrics_endpoint_serves_prometheus_text() -> None:
    async def run() -> None:
        disp = StubDispatcher()
        client = await _serve(disp, "dev-secret")
        try:
            resp = await client.get("/metrics")
            assert resp.status == 200
            text = await resp.text()
            # Default Prometheus text content includes a HELP line for each metric.
            assert "mod_text_latency_seconds" in text or "# HELP" in text
        finally:
            await client.close()
    asyncio.run(run())
