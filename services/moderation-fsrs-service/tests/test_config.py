"""Rule 1 verification: thresholds must come from disk, never hardcoded."""
from __future__ import annotations

import json
import os
from pathlib import Path

import pytest

from app.config import (
    _load_image_threshold,
    _load_text_threshold,
    load_settings,
)


def _make_text_dir(tmp_path: Path, recall_threshold: float = 0.35) -> Path:
    d = tmp_path / "text_model"
    d.mkdir()
    (d / "threshold.json").write_text(
        json.dumps({"recall_threshold": recall_threshold, "best_f1_threshold": 0.62}),
        encoding="utf-8",
    )
    return d


def _make_image_dir(tmp_path: Path, threshold: float = 0.5) -> Path:
    d = tmp_path / "image_model"
    d.mkdir()
    (d / "threshold.txt").write_text(f"{threshold}", encoding="utf-8")
    return d


def test_text_threshold_from_json(tmp_path: Path) -> None:
    d = _make_text_dir(tmp_path, 0.35)
    assert _load_text_threshold(d) == pytest.approx(0.35)


def test_text_threshold_falls_back_to_best_f1(tmp_path: Path) -> None:
    d = tmp_path / "m"
    d.mkdir()
    (d / "threshold.json").write_text(
        json.dumps({"best_f1_threshold": 0.41}), encoding="utf-8"
    )
    assert _load_text_threshold(d) == pytest.approx(0.41)


def test_image_threshold_from_txt(tmp_path: Path) -> None:
    d = _make_image_dir(tmp_path, 0.5)
    assert _load_image_threshold(d) == pytest.approx(0.5)


def test_image_threshold_rejects_empty(tmp_path: Path) -> None:
    d = tmp_path / "m"
    d.mkdir()
    (d / "threshold.txt").write_text("   ", encoding="utf-8")
    with pytest.raises(ValueError):
        _load_image_threshold(d)


def test_image_threshold_rejects_out_of_range(tmp_path: Path) -> None:
    d = tmp_path / "m"
    d.mkdir()
    (d / "threshold.txt").write_text("1.5", encoding="utf-8")
    with pytest.raises(ValueError):
        _load_image_threshold(d)


def test_load_settings_reads_env(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    t_dir = _make_text_dir(tmp_path, 0.35)
    i_dir = _make_image_dir(tmp_path, 0.5)
    monkeypatch.setenv("TEXT_MODEL_DIR", str(t_dir))
    monkeypatch.setenv("IMAGE_MODEL_DIR", str(i_dir))
    monkeypatch.setenv("GRPC_PORT", "60000")
    s = load_settings()
    assert s.text_threshold == pytest.approx(0.35)
    assert s.image_threshold == pytest.approx(0.5)
    assert s.grpc_port == 60000
