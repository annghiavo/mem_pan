"""HTTP server fronting the moderation service for two reasons:
1. Pub/Sub push delivery (Google Cloud requires HTTPS endpoints).
2. Liveness probes that don't need grpc_health_probe.

Endpoints:
  POST /internal/pubsub?token=<shared-secret>  — Pub/Sub push receiver
  GET  /healthz                                 — readiness probe
  GET  /metrics                                 — Prometheus scrape

The Pub/Sub push body shape (envelope-of-envelope) is:

  {"message": {"data": "<b64-of-outer>", "messageId": "...", "publishTime": "..."}}

`outer` itself is `{"event_type": "...", "data": "<b64-of-inner-payload>"}` —
matching the format produced by deck-service's `httpPublisher`.
"""
from __future__ import annotations

import base64
import json
import logging
import os
from typing import Any

from typing import Protocol

from aiohttp import web
from prometheus_client import CONTENT_TYPE_LATEST, generate_latest


class EventDispatcher(Protocol):
    """Minimal interface the HTTP server needs from a dispatcher.

    Declared here as a Protocol so test code can stub the dispatcher without
    importing the concrete (torch-dependent) implementation.
    """
    async def dispatch(self, event_type: str, inner: bytes) -> None: ...


_DISPATCHER_KEY: web.AppKey[EventDispatcher] = web.AppKey("dispatcher", EventDispatcher)
_SECRET_KEY: web.AppKey[str] = web.AppKey("push_secret", str)

log = logging.getLogger(__name__)


def create_app(dispatcher: EventDispatcher, push_secret: str) -> web.Application:
    app = web.Application()
    app[_DISPATCHER_KEY] = dispatcher
    app[_SECRET_KEY] = push_secret

    app.router.add_get("/healthz", _healthz)
    app.router.add_get("/metrics", _metrics)
    app.router.add_post("/internal/pubsub", _pubsub_push)
    return app


async def _healthz(_request: web.Request) -> web.Response:
    return web.Response(text="ok")


async def _metrics(_request: web.Request) -> web.Response:
    return web.Response(
        body=generate_latest(),
        content_type=CONTENT_TYPE_LATEST.split(";")[0],
    )


async def _pubsub_push(request: web.Request) -> web.Response:
    # Token gate — matches the convention used by the Go consumers.
    expected = request.app[_SECRET_KEY]
    if expected and request.query.get("token") != expected:
        return web.Response(status=401, text="invalid token")

    try:
        body: dict[str, Any] = await request.json()
    except json.JSONDecodeError:
        return web.Response(status=400, text="invalid json")

    message = body.get("message") or {}
    data_b64 = message.get("data")
    if not data_b64:
        # Pub/Sub sometimes delivers empty messages on subscription setup.
        return web.Response(status=204)

    try:
        outer = base64.b64decode(data_b64)
        envelope = json.loads(outer)
        event_type = envelope.get("event_type", "")
        inner_b64 = envelope.get("data", "")
        inner = base64.b64decode(inner_b64) if inner_b64 else b""
    except Exception as exc:  # noqa: BLE001
        log.exception("malformed pubsub envelope: %s", exc)
        # 200 -> ack so Pub/Sub does not retry a poison message forever.
        return web.Response(status=200, text="dropped")

    try:
        await request.app[_DISPATCHER_KEY].dispatch(event_type, inner)
    except Exception:  # noqa: BLE001
        log.exception("dispatch failed event_type=%s", event_type)
        # 5xx -> Pub/Sub will retry with backoff.
        return web.Response(status=500, text="dispatch error")

    return web.Response(status=204)


async def serve_http(
    dispatcher: EventDispatcher,
    *,
    port: int,
    push_secret: str,
) -> tuple[web.AppRunner, web.TCPSite]:
    """Start the HTTP server and return the runner+site for graceful shutdown."""
    app = create_app(dispatcher, push_secret)
    runner = web.AppRunner(app)
    await runner.setup()
    site = web.TCPSite(runner, host="0.0.0.0", port=port)  # noqa: S104
    await site.start()
    log.info("HTTP server listening on :%d", port)
    return runner, site
