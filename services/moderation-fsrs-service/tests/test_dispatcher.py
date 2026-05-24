"""End-to-end logic test for EventDispatcher. Uses fakes for the model registry,
deck client, and pub/sub publisher so we exercise the control flow without any
network or torch dependency.
"""
from __future__ import annotations

import asyncio
import json
from dataclasses import dataclass

import pytest

from app.events.dispatcher import EventDispatcher
from app.events.types import (
    TYPE_CARD_CREATED,
    TYPE_CARD_UPDATED,
    TYPE_MODERATION_DECK_DELETED,
)


@dataclass
class FakeVerdict:
    is_violation: bool
    confidence: float
    decode_error: bool = False


class FakeText:
    def __init__(self, verdicts: list[FakeVerdict]) -> None:
        self.verdicts = verdicts
        self.calls: list[list[str]] = []

    def predict(self, texts):
        self.calls.append(list(texts))
        out = self.verdicts[: len(self.calls[-1])]
        self.verdicts = self.verdicts[len(self.calls[-1]):]
        return out


class FakeImage:
    def __init__(self, verdicts: list[FakeVerdict]) -> None:
        self.verdicts = verdicts
        self.calls: list[list[tuple]] = []

    def predict(self, items):
        items = list(items)
        self.calls.append(items)
        out = self.verdicts[: len(items)]
        self.verdicts = self.verdicts[len(items):]
        return out


class FakeRegistry:
    def __init__(self, text, image):
        self.text = text
        self.image = image


class FakeDeckAdmin:
    def __init__(self) -> None:
        self.calls: list[tuple[str, str]] = []

    async def update_deck_status(self, deck_id, status):
        self.calls.append((deck_id, status))


class FakePublisher:
    def __init__(self) -> None:
        self.events: list[tuple[str, dict]] = []

    async def publish(self, event_type, payload):
        self.events.append((event_type, payload))


def _inner_bytes(payload: dict) -> bytes:
    return json.dumps(payload).encode("utf-8")


# ----- tests -----


def test_clean_card_does_not_trigger_callbacks():
    text = FakeText([FakeVerdict(False, 0.10), FakeVerdict(False, 0.05)])
    image = FakeImage([])
    deck = FakeDeckAdmin()
    publisher = FakePublisher()
    disp = EventDispatcher(
        registry=FakeRegistry(text, image),
        deck_admin=deck,
        moderation_publisher=publisher,
    )
    payload = _inner_bytes({
        "card_id": "c1", "deck_id": "d1", "user_id": "u1", "note_id": "n1",
        "content_front": "hello", "content_back": "world",
    })
    asyncio.run(disp.dispatch(TYPE_CARD_CREATED, payload))
    assert deck.calls == []
    assert publisher.events == []


def test_text_violation_locks_deck_and_publishes_event():
    text = FakeText([FakeVerdict(True, 0.95), FakeVerdict(False, 0.10)])
    image = FakeImage([])
    deck = FakeDeckAdmin()
    publisher = FakePublisher()
    disp = EventDispatcher(
        registry=FakeRegistry(text, image),
        deck_admin=deck,
        moderation_publisher=publisher,
    )
    payload = _inner_bytes({
        "card_id": "c1", "deck_id": "deck-7", "user_id": "user-42", "note_id": "n1",
        "content_front": "you suck", "content_back": "",
    })
    asyncio.run(disp.dispatch(TYPE_CARD_UPDATED, payload))

    assert deck.calls == [("deck-7", "deleted")]
    assert len(publisher.events) == 1
    event_type, body = publisher.events[0]
    assert event_type == TYPE_MODERATION_DECK_DELETED
    assert body["deck_id"] == "deck-7"
    assert body["user_id"] == "user-42"
    assert body["reason"] == "text_violation"
    assert body["violated_card_ids"] == ["c1"]


def test_image_only_violation_marks_image_reason():
    text = FakeText([FakeVerdict(False, 0.05), FakeVerdict(False, 0.02)])
    image = FakeImage([FakeVerdict(True, 0.93)])
    deck = FakeDeckAdmin()
    publisher = FakePublisher()
    disp = EventDispatcher(
        registry=FakeRegistry(text, image),
        deck_admin=deck,
        moderation_publisher=publisher,
    )
    payload = _inner_bytes({
        "card_id": "c2", "deck_id": "d2", "user_id": "u2", "note_id": "n2",
        "content_front": "ok", "content_back": "ok",
        "image_url": "https://example.com/x.png",
    })
    asyncio.run(disp.dispatch(TYPE_CARD_CREATED, payload))

    assert deck.calls == [("d2", "deleted")]
    assert publisher.events[0][1]["reason"] == "image_violation"


def test_mixed_violation_reports_mixed():
    text = FakeText([FakeVerdict(True, 0.81), FakeVerdict(False, 0.01)])
    image = FakeImage([FakeVerdict(True, 0.77)])
    deck = FakeDeckAdmin()
    publisher = FakePublisher()
    disp = EventDispatcher(
        registry=FakeRegistry(text, image),
        deck_admin=deck,
        moderation_publisher=publisher,
    )
    payload = _inner_bytes({
        "card_id": "c3", "deck_id": "d3", "user_id": "u3", "note_id": "n3",
        "content_front": "bad text", "content_back": "",
        "image_url": "https://example.com/bad.png",
    })
    asyncio.run(disp.dispatch(TYPE_CARD_CREATED, payload))
    assert publisher.events[0][1]["reason"] == "mixed"


def test_unknown_event_type_is_dropped():
    text = FakeText([])
    image = FakeImage([])
    deck = FakeDeckAdmin()
    publisher = FakePublisher()
    disp = EventDispatcher(
        registry=FakeRegistry(text, image),
        deck_admin=deck,
        moderation_publisher=publisher,
    )
    asyncio.run(disp.dispatch("user.registered", _inner_bytes({"foo": "bar"})))
    assert deck.calls == []
    assert publisher.events == []
