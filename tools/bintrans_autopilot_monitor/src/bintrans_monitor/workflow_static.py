"""Static checks for the unpublished monitor workflow candidate."""

from __future__ import annotations

from pathlib import Path

def forbidden_snippets() -> tuple[str, ...]:
    path = Path(__file__).with_name("forbidden_snippets.txt")
    rows = []
    for line in path.read_text(encoding="utf-8").splitlines():
        stripped = line.strip()
        if stripped and not stripped.startswith("#"):
            rows.append(stripped)
    rows.append("\n  push:")
    return tuple(rows)

def github_api_calls(text: str) -> list[str]:
    calls = []
    for line in text.splitlines():
        stripped = line.strip()
        if not stripped.startswith("gh api --method GET "):
            continue
        quoted = stripped.split('"', 2)
        if len(quoted) < 2:
            calls.append(stripped)
            continue
        calls.append(quoted[1])
    return calls


def permissions_for_calls(calls: list[str], text: str = "") -> dict[str, str]:
    """Map each GET path to the workflow permission that endpoint needs."""
    needed: dict[str, str] = {}
    if "actions/checkout@" in text:
        needed["contents"] = "read"
    for call in calls:
        if "/actions/" in call:
            needed["actions"] = "read"
        elif "/pulls" in call:
            needed["pull-requests"] = "read"
        elif "/issues" in call:
            needed["issues"] = "read"
        elif "/contents/" in call or "/git/" in call:
            needed["contents"] = "read"
        else:
            needed[f"unmapped:{call}"] = "read"
    if "bintrans_monitor.cli" in text:
        needed["actions"] = "read"
    return needed


def _block_keys(text: str, header: str) -> list[str]:
    lines = text.splitlines()
    keys: list[str] = []
    inside = False
    for line in lines:
        if line == header or line.startswith(header):
            inside = True
            continue
        if inside:
            if line and not line.startswith(" "):
                break
            if line.startswith("  ") and not line.startswith("   ") and ":" in line:
                keys.append(line.strip().split(":", 1)[0])
    return keys


def inspect_workflow(text: str) -> dict:
    triggers = _block_keys(text, "on:")
    permissions = {}
    for key in _block_keys(text, "permissions:"):
        for line in text.splitlines():
            stripped = line.strip()
            if stripped.startswith(key + ":"):
                permissions[key] = stripped.split(":", 1)[1].strip()
    calls = github_api_calls(text)
    required = permissions_for_calls(calls, text)
    stop_at = text.find("AUTOPILOT_EMERGENCY_STOP")
    get_at = text.find("gh api --method GET")
    uses_at = text.find("\n        uses:")
    if uses_at < 0:
        uses_at = text.find("\n      uses:")
    pinned = "actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683" in text
    return {
        "triggers": triggers,
        "permissions": permissions,
        "api_calls": calls,
        "required_permissions": required,
        "stop_before_get": stop_at != -1 and get_at != -1 and stop_at < get_at,
        "explicit_false_test": '[ "${AUTOPILOT_EMERGENCY_STOP}" != "false" ]' in text,
        "exit_2": "exit 2" in text,
        "ubuntu_latest": "ubuntu-latest" in text,
        "timeout_10": "timeout-minutes: 10" in text,
        "forbidden_hits": [snippet for snippet in forbidden_snippets() if snippet in text],
        "dispatch_only": triggers == ["workflow_dispatch"],
        "permissions_exact": permissions == required and all(value == "read" for value in permissions.values()),
        "continue_on_error": "continue-on-error" in text,
        "always_if": "if: always()" in text or "always()" in text,
        "uses_steps": uses_at != -1,
        "checkout_after_stop": stop_at != -1 and uses_at != -1 and stop_at < uses_at,
        "classifier_invoked": "python3 -m bintrans_monitor.cli" in text,
        "checkout_pinned": pinned and "persist-credentials: false" in text and "ref: ${{ github.sha }}" in text,
        "raw_json_logged": "cat \"$tmp" in text or "set -x" in text or "echo \"$GH_TOKEN\"" in text,
    }


def inspect_workflow_file(path: Path) -> dict:
    return inspect_workflow(path.read_text(encoding="utf-8"))
