"""Classify one prepared Actions run and print only the sanitized rows.

Live GitHub access is refused. Emergency stop is evaluated before any file is opened.
"""

from __future__ import annotations

import json
import os
import re
import sys
from pathlib import Path

from bintrans_monitor.engine import (
    EXIT_EMERGENCY_STOP,
    EXIT_SECURITY,
    EXIT_UNKNOWN,
    EXPECTED_REPOSITORY,
    FixtureTransport,
    MonitorError,
    SecurityDeny,
    StopSource,
    evaluate_stop,
    reject_mutation,
    run_monitor,
)
from bintrans_monitor.github_adapter import assemble
from bintrans_monitor.report_sanitizer import sanitize


def _stop_from_environ() -> StopSource:
    if os.environ.get("AUTOPILOT_EMERGENCY_STOP_UNREADABLE") == "1":
        return StopSource(os.environ.get("AUTOPILOT_EMERGENCY_STOP"), unreadable=True)
    if "AUTOPILOT_EMERGENCY_STOP" not in os.environ:
        return StopSource(None)
    return StopSource(os.environ.get("AUTOPILOT_EMERGENCY_STOP"))


def jobs_on_page(path: Path) -> int:
    payload = json.loads(path.read_text(encoding="utf-8"))
    jobs = payload.get("jobs")
    if not isinstance(jobs, list):
        raise MonitorError("jobs_page_rejected")
    return len(jobs)


_PAGE_LIMIT = 50
_RUN_ID = re.compile(r"[0-9]{1,12}")
_SHA = re.compile(r"[0-9a-f]{40}")


def _page_index(path: Path) -> int:
    number = path.stem.removeprefix("jobs-")
    if not number.isdigit() or str(int(number)) != number:
        raise MonitorError("jobs_page_rejected")
    value = int(number)
    if value < 1 or value > _PAGE_LIMIT:
        raise MonitorError("jobs_page_limit")
    return value


def _load_pages(directory: Path) -> list[dict]:
    """Load jobs-1.json, jobs-2.json, ... in numeric order, at most 50 pages."""
    indexed = [(_page_index(path), path) for path in directory.glob("jobs-*.json")]
    if len(indexed) > _PAGE_LIMIT:
        raise MonitorError("jobs_page_limit")
    indexed.sort(key=lambda item: item[0])
    expected = list(range(1, len(indexed) + 1))
    if [item[0] for item in indexed] != expected:
        raise MonitorError("jobs_page_order")
    return [json.loads(path.read_text(encoding="utf-8")) for _, path in indexed]


def _emit(document: dict) -> None:
    sys.stdout.write(json.dumps(document, sort_keys=True, separators=(",", ":")) + "\n")


def main(argv: list[str] | None = None) -> int:
    args = list(sys.argv[1:] if argv is None else argv)
    stop = _stop_from_environ()
    allowed, reason = evaluate_stop(stop.read())
    if not allowed:
        _emit(
            {
                "classification": "DENIED",
                "confidence_or_uncertainty": "ROOT_CAUSE_UNCONFIRMED",
                "error_signature": reason,
                "status": "DENIED",
            }
        )
        return EXIT_EMERGENCY_STOP
    try:
        return _run(args, stop)
    except SecurityDeny:
        _emit({"classification": "ERROR", "error_signature": "security_deny", "status": "ERROR"})
        return EXIT_SECURITY
    except MonitorError as exc:
        _emit({"classification": "ERROR", "error_signature": str(exc), "status": "ERROR"})
        return EXIT_UNKNOWN
    except (OSError, ValueError, TypeError, json.JSONDecodeError):
        _emit({"classification": "ERROR", "error_signature": "classifier_failed", "status": "ERROR"})
        return EXIT_UNKNOWN


def _run(args: list[str], stop: StopSource) -> int:
    if args[:1] == ["--reject"]:
        reject_mutation(args[1] if len(args) > 1 else "")
        return 0
    if args[:1] == ["--jobs-on-page"]:
        count = jobs_on_page(Path(args[1]))
        sys.stdout.write(f"{count}\n")
        return 0
    flags = _flags(args)
    pages = _load_pages(Path(flags["api_jobs_dir"]))
    run = json.loads(Path(flags["api_run"]).read_text(encoding="utf-8"))
    payload = assemble(
        run,
        pages,
        flags.get("repository") or EXPECTED_REPOSITORY,
        int(flags["run_id"]),
    )
    report = run_monitor(
        stop,
        FixtureTransport(payload),
        payload["repository"],
        expected_sha=flags.get("expected_sha"),
    )
    for row in sanitize(report):
        _emit(row)
    return report.exit_code


def _flags(args: list[str]) -> dict[str, str]:
    found: dict[str, str] = {}
    index = 0
    while index < len(args):
        key = args[index]
        if not key.startswith("--"):
            raise ValueError("cli_rejected")
        name = key[2:].replace("-", "_")
        if index + 1 >= len(args):
            raise ValueError("cli_rejected")
        found[name] = args[index + 1]
        index += 2
    for required in ("api_run", "api_jobs_dir", "run_id", "expected_sha"):
        if required not in found:
            raise ValueError("cli_rejected")
    if _RUN_ID.fullmatch(found["run_id"]) is None:
        raise ValueError("cli_rejected")
    if _SHA.fullmatch(found["expected_sha"]) is None:
        raise ValueError("cli_rejected")
    return found
