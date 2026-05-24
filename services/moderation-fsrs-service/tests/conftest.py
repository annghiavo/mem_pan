"""Shared pytest fixtures. Adds repo root + pb/ to sys.path so imports work
without an install step."""
from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
sys.path.insert(0, str(ROOT / "pb"))
