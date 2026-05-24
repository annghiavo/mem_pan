"""Tests for TextModerator. Uses a fake model wrapper to avoid loading the 1.1GB
XLM-RoBERTa weights in CI — the unit under test is the empty-string short-circuit
and the threshold comparison logic.
"""
from __future__ import annotations

import pytest

pytest.importorskip("torch")  # skip the file if torch isn't installed locally.

from app.models.text_moderator import TextModerator, TextVerdict  # noqa: E402


class _FakeText(TextModerator):
    """Bypasses real model load; we only exercise predict() control flow."""

    def __init__(self, threshold: float, scripted_probs: list[float]) -> None:
        self.threshold = threshold
        self._probs = list(scripted_probs)
        self._calls: list[list[str]] = []

    def _infer_batch(self, texts):  # type: ignore[override]
        self._calls.append(list(texts))
        out, self._probs = self._probs[: len(texts)], self._probs[len(texts):]
        return out


def test_empty_inputs_return_clean():
    moderator = _FakeText(threshold=0.35, scripted_probs=[])
    out = moderator.predict(["", "   ", None])  # type: ignore[list-item]
    assert all(isinstance(v, TextVerdict) for v in out)
    assert all(not v.is_violation for v in out)
    assert all(v.confidence == 0.0 for v in out)
    # No batches were dispatched — empties never reached the model.
    assert moderator._calls == []


def test_threshold_boundaries():
    moderator = _FakeText(threshold=0.35, scripted_probs=[0.34, 0.35, 0.99])
    out = moderator.predict(["safe", "edge", "toxic"])
    assert out[0].is_violation is False
    assert out[1].is_violation is True   # >= threshold
    assert out[2].is_violation is True
    assert moderator._calls == [["safe", "edge", "toxic"]]


def test_mixed_with_empty():
    moderator = _FakeText(threshold=0.35, scripted_probs=[0.10, 0.80])
    out = moderator.predict(["clean", "", "evil"])
    assert out[0].is_violation is False
    assert out[1].is_violation is False  # empty -> CLEAN, never seen by model
    assert out[2].is_violation is True
    # Model only saw the 2 non-empty entries
    assert moderator._calls == [["clean", "evil"]]


def test_inference_exception_falls_back_to_clean():
    class Broken(_FakeText):
        def _infer_batch(self, texts):
            raise RuntimeError("OOM")

    moderator = Broken(threshold=0.35, scripted_probs=[])
    out = moderator.predict(["abc"])
    assert out[0].is_violation is False
    assert out[0].confidence == 0.0
