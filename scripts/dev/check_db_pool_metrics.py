#!/usr/bin/env python3
"""Cross-platform check for db_pool_* Prometheus metrics."""

from __future__ import annotations

import sys
import urllib.error
import urllib.request

SERVICES = (
    ("identity-service", 8081),
    ("company-service", 8082),
    ("transport-order-service", 8083),
    ("rfx-service", 8084),
    ("shipment-service", 8085),
    ("document-service", 8086),
    ("billing-register-service", 8087),
)

REQUIRED_METRICS = (
    "db_pool_open_connections",
    "db_pool_in_use_connections",
    "db_pool_idle_connections",
    "db_pool_wait_count_total",
    "db_pool_wait_duration_seconds_total",
    "db_pool_max_open_connections",
)


def check_service(name: str, port: int) -> tuple[bool, list[str], str | None]:
    url = f"http://localhost:{port}/metrics"
    try:
        with urllib.request.urlopen(url, timeout=10) as response:
            body = response.read().decode("utf-8", errors="replace")
    except urllib.error.URLError:
        return False, list(REQUIRED_METRICS), "UNAVAILABLE"

    missing = [metric for metric in REQUIRED_METRICS if metric not in body]
    if missing:
        return False, missing, None

    return True, [], None


def main() -> int:
    print("Checking DB pool metrics...")
    all_ok = True

    for name, port in SERVICES:
        ok, missing, status = check_service(name, port)
        if status == "UNAVAILABLE":
            print(f"{name}: UNAVAILABLE http://localhost:{port}/metrics")
            all_ok = False
            continue
        if not ok:
            print(f"{name}: MISSING db_pool metrics ({', '.join(missing)})")
            all_ok = False
            continue
        print(f"{name}: OK")

    if all_ok:
        print("DB pool metrics check completed: OK")
        return 0

    print("DB pool metrics check completed: FAILED")
    return 1


if __name__ == "__main__":
    sys.exit(main())
