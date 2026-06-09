"""นิยาม Prometheus metrics ที่ expose ที่ GET /metrics (Telemetry)

มี 3 กลุ่ม metric บน default registry:

* **HTTP** — request count + latency บันทึกโดย middleware ใน app.main
  ใช้ route template เป็น path label เพื่อควบคุม cardinality ให้ผูกกับจำนวน route ไม่ใช่จำนวน URL จริง
* **Agent** — ดึงจาก scan payload ที่ agent push มาผ่าน POST /agents/scans
  สถาปัตยกรรมห้าม scrape agent โดยตรง (docs/02-architecture.md: "ไม่ส่ง telemetry นอก backend")
  ดังนั้น backend แปลง scan data ให้เป็น metrics แทน
* **Business KPIs** — fleet gauge ที่ refresh เป็นระยะจาก DB ผ่าน metrics_service
  ต้องรันใน process เดียวกับ backend เพราะ Prometheus gauge อยู่ใน memory ของ process นั้น
"""

from __future__ import annotations

from prometheus_client import Counter, Gauge, Histogram

# ── HTTP ──────────────────────────────────────────────────────────────────────
http_requests_total = Counter(
    "softsentry_http_requests_total",
    "HTTP requests handled by the backend.",
    ["method", "path", "status"],
)
http_request_duration_seconds = Histogram(
    "softsentry_http_request_duration_seconds",
    "HTTP request latency in seconds.",
    ["method", "path"],
)

# ── Agent (ดึงจาก scan payload ที่ push เข้ามา) ──────────────────────────────
agent_scans_total = Counter(
    "softsentry_agent_scans_total",
    "Inventory scans ingested from agents.",
    ["scan_type"],
)
agent_scan_errors_total = Counter(
    "softsentry_agent_scan_errors_total",
    "Scan ingestions that raised during processing.",
)
agent_scan_duration_seconds = Histogram(
    "softsentry_agent_scan_duration_seconds",
    "Agent-side scan duration (completed_at - started_at), in seconds.",
    buckets=(0.5, 1, 2, 5, 10, 30, 60, 120, 300),
)
agent_scan_software_count = Histogram(
    "softsentry_agent_scan_software_count",
    "Software entries reported per ingested scan.",
    buckets=(0, 10, 25, 50, 100, 200, 400, 800),
)

# ── Business KPIs (refresh โดย metrics_service) ───────────────────────────────
machines = Gauge(
    "softsentry_machines",
    "Machines by derived status (online/stale/offline).",
    ["status"],
)
software_unique = Gauge(
    "softsentry_software_unique",
    "Distinct active software names across the fleet.",
)
alerts_open = Gauge(
    "softsentry_alerts_open",
    "Open (active+acknowledged) alerts by severity.",
    ["severity"],
)
vulnerabilities_open = Gauge(
    "softsentry_vulnerabilities_open",
    "Open vulnerabilities (>=medium confidence) by severity.",
    ["severity"],
)
licenses = Gauge(
    "softsentry_licenses",
    "Licenses by compliance status.",
    ["status"],
)
business_metrics_refreshed_timestamp_seconds = Gauge(
    "softsentry_business_metrics_refreshed_timestamp_seconds",
    "Unix time of the last successful business-metrics refresh (staleness signal).",
)


def observe_scan(*, scan_type: str, duration_seconds: float, software_count: int) -> None:
    """บันทึก scan ที่ ingest เสร็จเข้า agent metric family."""
    agent_scans_total.labels(scan_type=scan_type).inc()
    # clamp clock skew / timestamp ที่ผิดลำดับให้เป็น zero แทนที่จะ drop
    agent_scan_duration_seconds.observe(max(duration_seconds, 0.0))
    agent_scan_software_count.observe(software_count)
