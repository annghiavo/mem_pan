#!/usr/bin/env python3
"""demo-optimize-weights.py — gọi thẳng RPC OptimizeWeights để demo phần lõi FSRS.

Bối cảnh: RPC FsrsOptimizationService.OptimizeWeights ĐÃ chạy trong
moderation-fsrs-service (app/services/fsrs_servicer.py), nhưng hiện CHƯA có service
nào tự gọi nó (không cron/subscriber/endpoint). Script này đóng vai "study-service"
gọi sang để hội đồng thấy: review logs vào -> 21 trọng số mới + loss đi ra.

Cách chạy (dùng venv của chính service để có sẵn grpc + pb stubs):
    cd services/moderation-fsrs-service
    # đảm bảo service đang chạy gRPC ở 50051 (docker-compose hoặc chạy local)
    .venv/bin/python ../../scripts/demo/demo-optimize-weights.py
hoặc trỏ tới địa chỉ khác:
    FSRS_ADDR=localhost:50051 USER_ID=demo-user N_REVIEWS=600 \
        .venv/bin/python ../../scripts/demo/demo-optimize-weights.py
"""
from __future__ import annotations

import os
import random
import sys
import time
from pathlib import Path

# pb/ nằm ở gốc service; thêm vào sys.path để import được stub đã generate.
SERVICE_ROOT = Path(__file__).resolve().parents[2] / "services" / "moderation-fsrs-service"
sys.path.insert(0, str(SERVICE_ROOT))

import grpc  # noqa: E402
from pb import moderation_fsrs_pb2 as pb  # noqa: E402
from pb import moderation_fsrs_pb2_grpc as pb_grpc  # noqa: E402

ADDR = os.environ.get("FSRS_ADDR", "localhost:50051")
USER_ID = os.environ.get("USER_ID", "demo-user")
N_REVIEWS = int(os.environ.get("N_REVIEWS", "600"))
N_CARDS = int(os.environ.get("N_CARDS", "40"))


def make_review_logs(n: int, n_cards: int) -> list[pb.ReviewLog]:
    """Sinh review logs giả lập đủ phong phú để optimizer hội tụ.

    Mỗi thẻ được ôn nhiều lần với khoảng cách tăng dần; rating ngả về 'tốt'
    (FSRS cần phân bố rating đa dạng nhưng thiên về 3/4 như người học thật).
    """
    rng = random.Random(42)
    logs: list[pb.ReviewLog] = []
    now = int(time.time())
    per_card = max(1, n // n_cards)
    for c in range(n_cards):
        card_id = f"card-{c:03d}"
        t = now - rng.randint(60, 90) * 86400  # bắt đầu 2-3 tháng trước
        elapsed = 0
        for _ in range(per_card):
            rating = rng.choices([1, 2, 3, 4], weights=[1, 1, 5, 3])[0]
            logs.append(pb.ReviewLog(
                card_id=card_id,
                review_date=t,
                rating=rating,
                elapsed_days=elapsed,
            ))
            # khoảng cách lần ôn sau: ngắn nếu quên (1), dài dần nếu nhớ tốt
            elapsed = 1 if rating == 1 else min(elapsed * 2 + 1, 90)
            t += elapsed * 86400
    rng.shuffle(logs)
    return logs[:n]


def main() -> int:
    logs = make_review_logs(N_REVIEWS, N_CARDS)
    print(f"-> gọi OptimizeWeights tới {ADDR}")
    print(f"   user_id={USER_ID}  review_logs={len(logs)}  cards={N_CARDS}")

    with grpc.insecure_channel(ADDR) as channel:
        stub = pb_grpc.FsrsOptimizationServiceStub(channel)
        req = pb.OptimizeWeightsRequest(user_id=USER_ID, review_logs=logs)
        t0 = time.perf_counter()
        try:
            resp = stub.OptimizeWeights(req, timeout=120)
        except grpc.RpcError as e:
            print(f"!! RPC lỗi: {e.code()} — {e.details()}", file=sys.stderr)
            return 1
        dt = time.perf_counter() - t0

    print(f"\n<- xong sau {dt:.1f}s")
    print(f"   fsrs_version   : {resp.fsrs_version}")
    print(f"   num_reviews    : {resp.num_reviews_used}")
    print(f"   training loss  : {resp.loss:.4f}")
    print(f"   weights ({len(resp.weights)}):")
    for i, w in enumerate(resp.weights):
        print(f"     w[{i:2d}] = {w:.4f}")
    print("\n(Trong sản phẩm hoàn chỉnh, study-service sẽ DeactivateWeights cũ + "
          "InsertWeights bộ mới này vào bảng user_fsrs_weights, và lần ReviewCard "
          "kế tiếp dùng ngay trọng số mới.)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
