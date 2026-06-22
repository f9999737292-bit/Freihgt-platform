#!/usr/bin/env python3
"""Check whether common Freight Platform ports are free or in use."""

from __future__ import annotations

import socket
import sys

PORTS = (
    (3000, "web-admin"),
    (3001, "grafana"),
    (5432, "postgres"),
    (8080, "api-gateway"),
    (8081, "identity-service"),
    (8082, "company-service"),
    (8083, "transport-order-service"),
    (8084, "rfx-service"),
    (8085, "shipment-service"),
    (8086, "document-service"),
    (8087, "billing-register-service"),
    (8088, "low-code-service"),
    (9090, "prometheus"),
)


def port_status(port: int) -> str:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        try:
            sock.bind(("127.0.0.1", port))
            return "FREE"
        except OSError:
            return "IN USE"


def main() -> int:
    print("Port availability check (127.0.0.1)")
    print("=" * 40)

    in_use = 0
    for port, name in PORTS:
        status = port_status(port)
        print(f"{port} {name}: {status}")
        if status == "IN USE":
            in_use += 1

    print("=" * 40)
    print(f"Summary: {len(PORTS) - in_use} free, {in_use} in use")
    print("Note: IN USE is expected when the platform is already running.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
