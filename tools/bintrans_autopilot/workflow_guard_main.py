"""Fail the process before any secret is read when the workflow context is not main."""

from __future__ import annotations

import os

from bintrans_autopilot.workflow_guard import context_from_environ, evaluate_workflow_context


def main() -> int:
    ok, reason = evaluate_workflow_context(context_from_environ(os.environ))
    if not ok:
        print(f"UNTRUSTED_WORKFLOW={reason}")
        print("PAID_AGENTS_LAUNCHED=NO")
        return 2
    print("TRUSTED_WORKFLOW=main")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
