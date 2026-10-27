"""Operator entrypoint. Exits before any Cursor call while autopilot is off."""

from __future__ import annotations

import os
import sys
from datetime import datetime, timezone

from bintrans_autopilot.config import load_config, runtime_enabled
from bintrans_autopilot.cursor_api import CursorClient, UrllibTransport
from bintrans_autopilot.dispatcher import Dispatcher
from bintrans_autopilot.github_api import GithubRest
from bintrans_autopilot.report import render_report
from bintrans_autopilot.workflow_guard import context_from_environ, evaluate_workflow_context


def _now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def _window(now: datetime) -> tuple[int, int]:
    start = datetime(now.year, now.month, now.day, tzinfo=timezone.utc)
    return int(start.timestamp() * 1000), int(now.timestamp() * 1000)


def main(argv: list[str] | None = None) -> int:
    if os.environ.get("GITHUB_ACTIONS", "").strip().lower() == "true":
        trusted, untrusted_reason = evaluate_workflow_context(context_from_environ(os.environ))
        if not trusted:
            print(f"UNTRUSTED_WORKFLOW={untrusted_reason}")
            print("PAID_AGENTS_LAUNCHED=NO")
            return 2
    args = list(sys.argv[1:] if argv is None else argv)
    command = args[0] if args else "dispatch"
    config = load_config()
    enabled, reason = runtime_enabled(config, os.environ)
    if command == "report" and not enabled:
        print("AUTOPILOT_ENABLED=NO")
        print("CURSOR_USAGE=NOT_RUN")
        print("ENTERPRISE_COMPLETION_PERCENT=UNCHANGED")
        return 0
    if not enabled:
        print("AUTOPILOT_ENABLED=NO")
        print("PAID_AGENTS_LAUNCHED=NO")
        print(f"REASON={reason}")
        return 0
    token = os.environ.get("GITHUB_TOKEN", "").strip()
    repository = os.environ.get("GITHUB_REPOSITORY", "f9999737292-bit/Freihgt-platform").strip()
    api_key = os.environ.get("CURSOR_API_KEY", "").strip()
    admin_key = os.environ.get("CURSOR_ADMIN_API_KEY", "").strip()
    if not api_key or not token:
        print("FAIL_CLOSED=missing_credential")
        print("PAID_AGENTS_LAUNCHED=NO")
        return 2
    github = GithubRest(token, repository)
    cursor = CursorClient(api_key, UrllibTransport(), admin_key)
    now = datetime.now(timezone.utc)
    if command == "report":
        usage_note = "NOT_RUN"
        print(render_report(github.list_issues(), _now(), usage_note))
        return 0
    result = Dispatcher(config, github, cursor, dict(os.environ), _now(), _window(now)).run()
    print(f"AUTOPILOT_ENABLED=YES")
    print(f"REASON={result.reason}")
    print(f"LAUNCHED={','.join(result.launched)}")
    print(f"FAIL_CLOSED={result.fail_closed}")
    if result.fail_closed:
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
