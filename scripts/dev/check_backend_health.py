#!/usr/bin/env python3
"""Cross-platform health check for all backend services."""

from __future__ import annotations

import json
import sys
import urllib.error
import urllib.request

SERVICES = (
    ("api-gateway", 8080),
    ("identity-service", 8081),
    ("company-service", 8082),
    ("transport-order-service", 8083),
    ("rfx-service", 8084),
    ("shipment-service", 8085),
    ("document-service", 8086),
    ("billing-register-service", 8087),
)


def check_service(name: str, port: int) -> bool:
    url = f"http://localhost:{port}/health"
    try:
        with urllib.request.urlopen(url, timeout=10) as response:
            if response.status != 200:
                print(f"{name}: UNAVAILABLE {url} (HTTP {response.status})")
                return False
            body = response.read().decode("utf-8", errors="replace")
            payload = json.loads(body)
            if payload.get("status") != "ok":
                print(f"{name}: UNHEALTHY {url} (status={payload.get('status')!r})")
                return False
    except (urllib.error.URLError, json.JSONDecodeError, KeyError):
        print(f"{name}: UNAVAILABLE {url}")
        return False

    print(f"{name}: OK")
    return True


def main() -> int:
    print("Checking health endpoints...")
    all_ok = True
    for name, port in SERVICES:
        if not check_service(name, port):
            all_ok = False

    if all_ok:
        print("Health check completed: OK")
        return 0

    print("Health check completed: FAILED")
    return 1


if __name__ == "__main__":
    sys.exit(main())
