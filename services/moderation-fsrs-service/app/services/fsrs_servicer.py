"""FSRS optimization. CPU-heavy — runs in a ProcessPoolExecutor so the gRPC
event loop never blocks on a long training run.
"""
from __future__ import annotations

import asyncio
import logging
import time
from concurrent.futures import ProcessPoolExecutor

import grpc

from app.utils.metrics import fsrs_duration
from pb import moderation_fsrs_pb2 as pb
from pb import moderation_fsrs_pb2_grpc as pb_grpc

log = logging.getLogger(__name__)


def _run_optimizer(rows: list[dict]) -> tuple[list[float], float, str]:
    """Top-level fn so it pickles cleanly into the worker process.

    Returns (weights, loss, fsrs_version).
    """
    # Heavy imports happen inside the worker — keeps gRPC process slim until needed.
    import pandas as pd
    from fsrs_optimizer import Optimizer  # type: ignore

    df = pd.DataFrame(rows)
    optimizer = Optimizer()
    # The library exposes a couple of entrypoints depending on version. Try the
    # most common one first; fall back to direct .train(df).
    if hasattr(optimizer, "anki_extract"):
        optimizer.anki_extract(df)
        weights, loss = optimizer.train()
    else:
        weights, loss = optimizer.train(df)  # type: ignore[arg-type]
    version = getattr(optimizer, "version", "fsrs-5")
    return [float(w) for w in weights], float(loss), str(version)


class FsrsServicer(pb_grpc.FsrsOptimizationServiceServicer):
    def __init__(self, pool: ProcessPoolExecutor) -> None:
        self.pool = pool

    async def OptimizeWeights(  # noqa: N802
        self,
        request: pb.OptimizeWeightsRequest,
        context: grpc.aio.ServicerContext,
    ) -> pb.OptimizeWeightsResponse:
        if not request.review_logs:
            await context.abort(
                grpc.StatusCode.INVALID_ARGUMENT,
                "review_logs is empty",
            )

        rows = [
            {
                "card_id": r.card_id,
                "review_time": r.review_date,
                "review_rating": r.rating,
                "elapsed_days": r.elapsed_days,
            }
            for r in request.review_logs
        ]

        loop = asyncio.get_running_loop()
        t0 = time.perf_counter()
        try:
            weights, loss, version = await loop.run_in_executor(
                self.pool, _run_optimizer, rows,
            )
        except Exception as exc:  # noqa: BLE001
            log.exception("fsrs optimize failed user=%s", request.user_id)
            await context.abort(grpc.StatusCode.INTERNAL, f"optimizer error: {exc}")
            return pb.OptimizeWeightsResponse()  # unreachable, satisfies type checker
        finally:
            fsrs_duration.observe(time.perf_counter() - t0)

        log.info(
            "fsrs optimize user=%s reviews=%d loss=%.4f version=%s",
            request.user_id, len(rows), loss, version,
        )
        return pb.OptimizeWeightsResponse(
            user_id=request.user_id,
            weights=weights,
            num_reviews_used=len(rows),
            loss=loss,
            fsrs_version=version,
        )
