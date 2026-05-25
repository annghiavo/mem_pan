"""Async Pub/Sub HTTP publisher. Matches the wire format of the GoLang
`httpPublisher` in deck-service/internal/publisher/publisher.go.

GoLang json.Marshal of a `[]byte` field uses base64. So the wrapper looks like:

    envelope = {"event_type": <type>, "data": "<b64-of-inner-payload-json>"}
    body     = {"messages": [{"data": "<b64-of-envelope-json>"}]}
"""
from __future__ import annotations

import base64
import json
import logging
import os
import time
from typing import Any

import httpx

log = logging.getLogger(__name__)

_METADATA_TOKEN_URL = (
    "http://metadata.google.internal/computeMetadata/v1"
    "/instance/service-accounts/default/token"
)
_METADATA_HEADERS = {"Metadata-Flavor": "Google"}


class PubsubPublisher:
    def __init__(
        self,
        project_id: str,
        topic_id: str,
        emulator_host: str | None = None,
    ) -> None:
        self.project_id = project_id
        self.topic_id = topic_id
        host = emulator_host or os.environ.get("PUBSUB_EMULATOR_HOST")
        self._use_emulator = bool(host)
        base = f"http://{host}" if host else "https://pubsub.googleapis.com"
        self._endpoint = f"{base}/v1/projects/{project_id}/topics/{topic_id}:publish"
        self._client: httpx.AsyncClient | None = None
        # Cached metadata-server OAuth token (Cloud Run only).
        self._token: str | None = None
        self._token_expires_at: float = 0.0

    def _ensure(self) -> httpx.AsyncClient:
        if self._client is None:
            self._client = httpx.AsyncClient(timeout=5.0)
        return self._client

    async def _auth_header(self) -> dict[str, str]:
        # Emulator + local dev: no auth required.
        if self._use_emulator:
            return {}
        # Refresh ~5 minutes before expiry to avoid edge-case 401s.
        if self._token and time.time() < self._token_expires_at - 300:
            return {"Authorization": f"Bearer {self._token}"}
        client = self._ensure()
        try:
            resp = await client.get(_METADATA_TOKEN_URL, headers=_METADATA_HEADERS)
            resp.raise_for_status()
        except httpx.HTTPError as exc:
            # Not on Cloud Run / metadata server unreachable — fall back to
            # unauthenticated request (will 403 in prod, but lets tests run).
            log.warning("metadata token fetch failed: %s", exc)
            return {}
        body = resp.json()
        self._token = body["access_token"]
        self._token_expires_at = time.time() + body.get("expires_in", 3600)
        return {"Authorization": f"Bearer {self._token}"}

    async def publish(self, event_type: str, payload: dict[str, Any]) -> None:
        inner = json.dumps(payload, default=str, separators=(",", ":")).encode("utf-8")
        envelope = json.dumps({
            "event_type": event_type,
            "data": base64.b64encode(inner).decode("ascii"),
        }).encode("utf-8")
        body = {
            "messages": [
                {"data": base64.b64encode(envelope).decode("ascii")},
            ],
        }
        client = self._ensure()
        headers = await self._auth_header()
        resp = await client.post(self._endpoint, json=body, headers=headers)
        if resp.status_code >= 300:
            log.error(
                "pubsub publish failed: status=%d type=%s body=%s",
                resp.status_code, event_type, resp.text[:200],
            )
            resp.raise_for_status()
        log.info("pubsub published type=%s topic=%s", event_type, self.topic_id)

    async def close(self) -> None:
        if self._client is not None:
            await self._client.aclose()
            self._client = None
