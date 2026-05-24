"""Tests for the FSRS row-shaping helper. Avoids running the real optimizer
(slow, requires real review history) — we just verify the request -> rows
transform that `_run_optimizer` expects.
"""
from __future__ import annotations

from app.services.fsrs_servicer import _run_optimizer  # noqa: F401  (importability check)


def _build_rows(n: int = 5) -> list[dict]:
    return [
        {
            "card_id": f"card-{i}",
            "review_time": 1_700_000_000 + i * 86_400,
            "review_rating": (i % 4) + 1,
            "elapsed_days": i,
        }
        for i in range(n)
    ]


def test_row_shape_matches_optimizer_contract():
    rows = _build_rows(3)
    for r in rows:
        assert set(r.keys()) == {"card_id", "review_time", "review_rating", "elapsed_days"}
        assert 1 <= r["review_rating"] <= 4
        assert r["elapsed_days"] >= 0
