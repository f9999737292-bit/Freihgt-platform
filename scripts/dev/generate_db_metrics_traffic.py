#!/usr/bin/env python3
"""Generate sample API traffic through the gateway for DB metrics."""

from __future__ import annotations

import urllib.error
import urllib.request

TENANT_ID = "74519f22-ff9b-4a8b-8fff-a958c689682f"
URLS = (
    f"http://localhost:8080/api/v1/companies?tenant_id={TENANT_ID}",
    f"http://localhost:8080/api/v1/transport-orders?tenant_id={TENANT_ID}",
    f"http://localhost:8080/api/v1/shipments?tenant_id={TENANT_ID}",
    f"http://localhost:8080/api/v1/billing-registers?tenant_id={TENANT_ID}",
)


def request_url(url: str) -> tuple[bool, int | None]:
    request = urllib.request.Request(url, headers={"Accept": "application/json"})
    try:
        with urllib.request.urlopen(request, timeout=15) as response:
            return False, response.status
    except urllib.error.HTTPError as error:
        return error.code == 401, error.code
    except urllib.error.URLError:
        return False, None


def main() -> None:
    print("Generating sample DB traffic...")
    saw_401 = False

    for url in URLS:
        is_401, status = request_url(url)
        if status is None:
            print(f"{url} -> UNAVAILABLE")
            continue
        print(f"{url} -> {status}")
        if is_401:
            saw_401 = True

    if saw_401:
        print(
            "WARNING: API returned 401. Generate traffic after login or "
            "temporarily use AUTH_ENABLED=false for local metrics check."
        )

    print("Sample DB traffic generated")


if __name__ == "__main__":
    main()
