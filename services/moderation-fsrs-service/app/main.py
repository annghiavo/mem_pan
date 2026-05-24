"""gRPC + HTTP server bootstrap. Loads models ONCE before serving (Rule 2).

The service exposes two listeners:
- gRPC on $GRPC_PORT: direct ModerateDeck / OptimizeWeights from sister services.
- HTTP on $HTTP_PORT: Pub/Sub push receiver for card.created / card.updated
  events from deck-service, plus /healthz + /metrics.

On a moderation violation, this service:
- gRPC-calls deck-service.AdminUpdateDeckStatus(deck_id, "deleted")
- Pub/Sub-publishes "moderation.deck_deleted" so notification-service (FCM alert
  to owner) and admin-service (audit log) react in parallel.
"""
from __future__ import annotations

import asyncio
import logging
import signal
import sys
from concurrent.futures import ProcessPoolExecutor
from pathlib import Path

# pb/*.py uses sibling-style imports — make pb/ importable before any pb import.
_PB_DIR = Path(__file__).resolve().parents[1] / "pb"
if str(_PB_DIR) not in sys.path:
    sys.path.insert(0, str(_PB_DIR))

import grpc
from grpc_health.v1 import health, health_pb2, health_pb2_grpc

from app.clients.deck_admin_client import DeckAdminClient
from app.config import load_settings
from app.events.dispatcher import EventDispatcher
from app.events.publisher import PubsubPublisher
from app.http_server import serve_http
from app.models.registry import build_registry
from app.services.fsrs_servicer import FsrsServicer
from app.services.moderation_servicer import ModerationServicer
from app.utils.logging import setup_logging
from pb import moderation_fsrs_pb2_grpc as pb_grpc

log = logging.getLogger(__name__)


async def serve() -> None:
    setup_logging()
    settings = load_settings()

    # --- Lifespan: build all heavy state exactly once ---
    log.info("startup: loading models ...")
    registry = build_registry(settings)
    log.info("startup: models loaded")

    deck_admin = DeckAdminClient(settings.deck_service_addr)
    moderation_publisher = PubsubPublisher(
        project_id=settings.pubsub_project_id,
        topic_id=settings.pubsub_moderation_topic,
    )
    dispatcher = EventDispatcher(
        registry=registry,
        deck_admin=deck_admin,
        moderation_publisher=moderation_publisher,
    )

    fsrs_pool = ProcessPoolExecutor(max_workers=settings.fsrs_pool_workers)

    # --- gRPC server ---
    grpc_server = grpc.aio.server(
        options=[
            ("grpc.max_send_message_length", 32 * 1024 * 1024),
            ("grpc.max_receive_message_length", 32 * 1024 * 1024),
            ("grpc.keepalive_time_ms", 30_000),
            ("grpc.keepalive_timeout_ms", 10_000),
        ],
    )
    # ModerationServicer keeps its original direct-RPC contract; it uses the
    # same deck-admin client + Pub/Sub publisher for callbacks so both the
    # event-driven path and the direct RPC path produce identical side-effects.
    pb_grpc.add_ModerationServiceServicer_to_server(
        ModerationServicer(registry, deck_admin, moderation_publisher),
        grpc_server,
    )
    pb_grpc.add_FsrsOptimizationServiceServicer_to_server(
        FsrsServicer(fsrs_pool),
        grpc_server,
    )
    health_servicer = health.HealthServicer()
    health_servicer.set("", health_pb2.HealthCheckResponse.SERVING)
    health_pb2_grpc.add_HealthServicer_to_server(health_servicer, grpc_server)

    grpc_addr = f"[::]:{settings.grpc_port}"
    grpc_server.add_insecure_port(grpc_addr)
    await grpc_server.start()
    log.info("gRPC listening on %s", grpc_addr)

    # --- HTTP server (Pub/Sub push + health + metrics) ---
    runner, _site = await serve_http(
        dispatcher,
        port=settings.http_port,
        push_secret=settings.pubsub_push_secret,
    )

    # --- Graceful shutdown ---
    stop_event = asyncio.Event()

    def _graceful(*_: object) -> None:
        log.info("signal received, shutting down ...")
        stop_event.set()

    loop = asyncio.get_running_loop()
    for sig in (signal.SIGTERM, signal.SIGINT):
        try:
            loop.add_signal_handler(sig, _graceful)
        except NotImplementedError:
            pass

    await stop_event.wait()
    log.info("draining gRPC (grace=10s) + HTTP")
    await grpc_server.stop(grace=10)
    await runner.cleanup()
    fsrs_pool.shutdown(wait=False, cancel_futures=True)
    await deck_admin.close()
    await moderation_publisher.close()
    log.info("shutdown complete")


def main() -> None:
    asyncio.run(serve())


if __name__ == "__main__":
    main()
