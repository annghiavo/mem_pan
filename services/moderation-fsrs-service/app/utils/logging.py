"""Structured stdout logging — Cloud Logging compatible."""
from __future__ import annotations

import logging
import sys


def setup_logging(level: str = "INFO") -> None:
    logging.basicConfig(
        level=level,
        stream=sys.stdout,
        format='{"ts":"%(asctime)s","lvl":"%(levelname)s","logger":"%(name)s","msg":%(message)r}',
    )
    logging.getLogger("transformers").setLevel(logging.WARNING)
    logging.getLogger("urllib3").setLevel(logging.WARNING)
    logging.getLogger("httpx").setLevel(logging.WARNING)
