"""Prometheus metrics. Scrape via sidecar or Cloud Run metrics endpoint."""
from __future__ import annotations

from prometheus_client import Counter, Gauge, Histogram

text_latency = Histogram(
    "mod_text_latency_seconds",
    "Text model inference latency per request",
    buckets=(0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5),
)
image_latency = Histogram(
    "mod_image_latency_seconds",
    "Image model inference latency per request",
    buckets=(0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0),
)
fsrs_duration = Histogram(
    "fsrs_optimize_duration_seconds",
    "FSRS optimizer wall-clock duration",
    buckets=(0.5, 1, 2, 5, 10, 30, 60, 120),
)
violation_counter = Counter(
    "mod_violation_total",
    "Total moderation violations",
    ["kind"],
)
clean_counter = Counter(
    "mod_clean_total",
    "Total clean items",
    ["kind"],
)
image_decode_error = Counter(
    "mod_image_decode_error_total",
    "Corrupted/undecodable images",
)
cpu_pct = Gauge("mod_cpu_pct", "Current process CPU percent")
rss_bytes = Gauge("mod_rss_bytes", "Current process RSS bytes")
