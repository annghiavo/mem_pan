"""Tests for ImageModerator. Mocks the heavy ViT — we cover Rule 4 (corrupted
images) and Rule 5 (preprocessor fallback) without downloading anything."""
from __future__ import annotations

import io
from pathlib import Path

import pytest

pytest.importorskip("torch")  # skip the file if torch isn't installed locally.

from PIL import Image  # noqa: E402

from app.models.image_moderator import (  # noqa: E402
    ImageModerator,
    ImageVerdict,
    _load_processor,
)


# ----------------- Rule 5: preprocessor fallback -----------------

class _DummyFallback:
    """Stand-in for AutoImageProcessor.from_pretrained returning a sentinel."""
    def __init__(self, model_id: str) -> None:
        self.model_id = model_id


def test_preprocessor_fallback_when_config_empty(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch,
) -> None:
    d = tmp_path / "model"
    d.mkdir()
    (d / "preprocessor_config.json").write_text("", encoding="utf-8")  # empty file

    called_with: list[str] = []

    def fake_from_pretrained(name: str):
        called_with.append(name)
        return _DummyFallback(name)

    # Patch the symbol used inside image_moderator.
    monkeypatch.setattr(
        "app.models.image_moderator.AutoImageProcessor.from_pretrained",
        staticmethod(fake_from_pretrained),
    )

    result = _load_processor(d, fallback_id="google/vit-base-patch16-224")
    assert isinstance(result, _DummyFallback)
    assert called_with == ["google/vit-base-patch16-224"]


def test_preprocessor_uses_model_dir_when_valid(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch,
) -> None:
    d = tmp_path / "model"
    d.mkdir()
    (d / "preprocessor_config.json").write_text('{"size": 224}', encoding="utf-8")

    calls: list[str] = []

    def fake_from_pretrained(name: str):
        calls.append(name)
        return _DummyFallback(name)

    monkeypatch.setattr(
        "app.models.image_moderator.AutoImageProcessor.from_pretrained",
        staticmethod(fake_from_pretrained),
    )

    result = _load_processor(d, fallback_id="google/vit-base-patch16-224")
    assert isinstance(result, _DummyFallback)
    assert calls == [str(d)]  # used local dir, didn't fall back


# ----------------- Rule 4: corrupted bytes -----------------

class _FakeImage(ImageModerator):
    """Bypass model load entirely; we exercise predict() control flow."""

    def __init__(self, threshold: float, scripted_probs: list[float]) -> None:
        self.threshold = threshold
        self.http_timeout = 1.0
        self._probs = list(scripted_probs)
        self._batches: list[int] = []

    def _infer_batch(self, images):  # type: ignore[override]
        self._batches.append(len(images))
        out, self._probs = self._probs[: len(images)], self._probs[len(images):]
        return out


def _png_bytes(color=(255, 0, 0)) -> bytes:
    buf = io.BytesIO()
    Image.new("RGB", (32, 32), color).save(buf, format="PNG")
    return buf.getvalue()


def test_corrupted_image_marked_decode_error_not_crash():
    mod = _FakeImage(threshold=0.5, scripted_probs=[0.9])
    valid = _png_bytes()
    items = [
        ("c1", b"\x00\x01\x02not-an-image", None),
        ("c2", valid, None),
    ]
    out = mod.predict(items)
    assert len(out) == 2
    assert out[0].decode_error is True
    assert out[0].is_violation is False
    assert out[1].decode_error is False
    assert out[1].is_violation is True
    # Only the valid image reached the model
    assert mod._batches == [1]


def test_empty_payload_marked_decode_error():
    mod = _FakeImage(threshold=0.5, scripted_probs=[])
    out = mod.predict([("c1", b"", None)])
    assert out[0].decode_error is True
    assert out[0].is_violation is False
    assert mod._batches == []


def test_threshold_boundary():
    mod = _FakeImage(threshold=0.5, scripted_probs=[0.49, 0.5])
    out = mod.predict([("c1", _png_bytes(), None), ("c2", _png_bytes(), None)])
    assert out[0].is_violation is False
    assert out[1].is_violation is True
