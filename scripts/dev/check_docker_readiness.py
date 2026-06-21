#!/usr/bin/env python3
"""Cross-platform Docker and disk readiness check."""

from __future__ import annotations

import shutil
import subprocess
import sys

LOW_DISK_GB = 10


def run_command(args: list[str]) -> tuple[bool, str]:
    try:
        result = subprocess.run(
            args,
            capture_output=True,
            text=True,
            timeout=60,
            check=False,
        )
    except FileNotFoundError:
        return False, f"command not found: {args[0]}"
    except subprocess.TimeoutExpired:
        return False, "command timed out"

    output = (result.stdout or "") + (result.stderr or "")
    output = output.strip()
    if result.returncode != 0:
        return False, output or f"exit code {result.returncode}"
    return True, output


def format_gb(bytes_value: int) -> str:
    return f"{bytes_value / (1024 ** 3):.2f} GB"


def main() -> int:
    print("Docker readiness check")
    print("=" * 40)

    cli_ok, cli_output = run_command(["docker", "version"])
    print(f"Docker CLI: {'OK' if cli_ok else 'FAILED'}")
    if not cli_ok:
        print(cli_output)

    daemon_ok, daemon_output = run_command(["docker", "info"])
    print(f"Docker daemon: {'OK' if daemon_ok else 'FAILED'}")
    if not daemon_ok:
        print("Docker Desktop is not running or unavailable")
        if daemon_output:
            print(daemon_output)

    compose_ok, compose_output = run_command(["docker", "compose", "version"])
    print(f"Docker compose: {'OK' if compose_ok else 'FAILED'}")
    if not compose_ok and compose_output:
        print(compose_output)

    usage = shutil.disk_usage(".")
    free_gb = usage.free / (1024 ** 3)
    print(f"Project disk free: {format_gb(usage.free)}")
    if free_gb < LOW_DISK_GB:
        print(
            "WARNING: Low disk space. Run docker-clean-safe and restart Docker Desktop."
        )

    if daemon_ok:
        df_ok, df_output = run_command(["docker", "system", "df"])
        print(f"Docker system df: {'printed' if df_ok else 'failed'}")
        if df_ok and df_output:
            print(df_output)
        elif not df_ok and df_output:
            print(df_output)
    else:
        print("Docker system df: skipped (daemon unavailable)")

    print("=" * 40)
    if cli_ok and daemon_ok and compose_ok:
        print("Docker readiness: OK")
        return 0

    print("Docker readiness: FAILED")
    return 1


if __name__ == "__main__":
    sys.exit(main())
