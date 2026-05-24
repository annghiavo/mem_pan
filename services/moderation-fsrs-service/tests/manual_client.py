"""Manual probe — pretend to be deck-service (Go) calling our ModerateDeck rpc.

Run after `python -m app.main` is up locally.
"""
from __future__ import annotations

import asyncio
import io
import os
import sys
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
sys.path.insert(0, str(ROOT / "pb"))

import grpc  # noqa: E402

from pb import moderation_fsrs_pb2 as pb  # noqa: E402
from pb import moderation_fsrs_pb2_grpc as pb_grpc  # noqa: E402


def _tiny_png(color: tuple[int, int, int] = (10, 20, 30)) -> bytes:
    buf = io.BytesIO()
    Image.new("RGB", (32, 32), color).save(buf, format="PNG")
    return buf.getvalue()


async def main() -> None:
    addr = os.environ.get("MODERATION_ADDR", "localhost:50051")
    async with grpc.aio.insecure_channel(addr) as ch:
        stub = pb_grpc.ModerationServiceStub(ch)
        req = pb.ModerateDeckRequest(
            deck_id="deck-123",
            user_id="user-7",
            texts=[
                pb.CardText(card_id="c1", content="Hello world"),
                pb.CardText(card_id="c2", content="i hate you all you trash"),
                pb.CardText(card_id="c3", content=""),  # empty -> CLEAN
            ],
            images=[
                pb.CardImage(card_id="c4", raw=_tiny_png()),
                pb.CardImage(card_id="c5", raw=b"\x00\x01corrupted"),
            ],
        )
        resp = await stub.ModerateDeck(req, timeout=30.0)
        print("deck status =", pb.ModerationStatus.Name(resp.status))
        for v in resp.items:
            print(
                f"  {v.card_id:6s} {v.kind:5s} viol={v.is_violation} "
                f"conf={v.confidence:.3f} reason={v.reason!r}"
            )


if __name__ == "__main__":
    asyncio.run(main())
