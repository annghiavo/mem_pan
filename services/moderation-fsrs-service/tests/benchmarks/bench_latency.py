"""Latency benchmark — fires N moderate-deck calls and reports p50/p95/p99.
Usage:
    python tests/benchmarks/bench_latency.py --addr localhost:50051 --n 100
"""
from __future__ import annotations

import argparse
import asyncio
import io
import statistics
import sys
import time
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT))
sys.path.insert(0, str(ROOT / "pb"))

import grpc  # noqa: E402

from pb import moderation_fsrs_pb2 as pb  # noqa: E402
from pb import moderation_fsrs_pb2_grpc as pb_grpc  # noqa: E402


def _png(color: tuple[int, int, int]) -> bytes:
    buf = io.BytesIO()
    Image.new("RGB", (224, 224), color).save(buf, format="PNG")
    return buf.getvalue()


async def one_call(stub: pb_grpc.ModerationServiceStub, idx: int) -> float:
    req = pb.ModerateDeckRequest(
        deck_id=f"bench-{idx}",
        user_id="bench-user",
        texts=[
            pb.CardText(card_id="t1", content="The quick brown fox jumps over"),
            pb.CardText(card_id="t2", content="Học máy là một lĩnh vực thú vị"),
        ],
        images=[pb.CardImage(card_id="i1", raw=_png((idx % 255, 100, 200)))],
    )
    t0 = time.perf_counter()
    await stub.ModerateDeck(req, timeout=30.0)
    return time.perf_counter() - t0


def _percentile(samples: list[float], p: float) -> float:
    if not samples:
        return float("nan")
    s = sorted(samples)
    k = max(0, min(len(s) - 1, int(round((p / 100) * (len(s) - 1)))))
    return s[k]


async def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--addr", default="localhost:50051")
    ap.add_argument("--n", type=int, default=50)
    ap.add_argument("--concurrency", type=int, default=4)
    args = ap.parse_args()

    async with grpc.aio.insecure_channel(args.addr) as ch:
        stub = pb_grpc.ModerationServiceStub(ch)
        # warm-up
        await one_call(stub, -1)

        sem = asyncio.Semaphore(args.concurrency)

        async def worker(i: int) -> float:
            async with sem:
                return await one_call(stub, i)

        latencies = await asyncio.gather(*(worker(i) for i in range(args.n)))

    print(f"n={args.n} concurrency={args.concurrency}")
    print(f"p50={_percentile(latencies, 50)*1000:.1f} ms")
    print(f"p95={_percentile(latencies, 95)*1000:.1f} ms")
    print(f"p99={_percentile(latencies, 99)*1000:.1f} ms")
    print(f"mean={statistics.fmean(latencies)*1000:.1f} ms")


if __name__ == "__main__":
    asyncio.run(main())
