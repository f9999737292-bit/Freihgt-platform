#!/usr/bin/env python3
"""Cross-platform check for db_query_duration_seconds on instrumented services."""

from __future__ import annotations

import sys
import urllib.error
import urllib.request

SERVICES = (
    ("company-service", 8082),
    ("transport-order-service", 8083),
    ("shipment-service", 8085),
    ("billing-register-service", 8087),
)

REQUIRED_METRICS = (
    "db_query_duration_seconds_bucket",
    "db_query_duration_seconds_sum",
    "db_query_duration_seconds_count",
)


def check_service(name: str, port: int) -> bool:
    url = f"http://localhost:{port}/metrics"
    try:
        with urllib.request.urlopen(url, timeout=10) as response:
            body = response.read().decode("utf-8", errors="replace")
    except urllib.error.URLError:
        print(f"{name}: UNAVAILABLE {url}")
        return False

    missing = [metric for metric in REQUIRED_METRICS if metric not in body]
    if missing:
        print(f"{name}: MISSING {', '.join(missing)}")
        return False

    print(f"{name}: OK")
    return True


def main() -> int:
    print("Checking DB metrics...")
    all_ok = True
    for name, port in SERVICES:
        if not check_service(name, port):
            all_ok = False

    if all_ok:
        print("DB metrics check completed: OK")
        return 0

    print("DB metrics check completed: FAILED")
    return 1


if __name__ == "__main__":
    sys.exit(main())
