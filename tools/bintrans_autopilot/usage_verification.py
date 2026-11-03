"""Canonical verification refs for reconciled Cursor usage payloads."""

from __future__ import annotations

import hashlib
import json


def usage_verification_refs(agent_id: str, run_id: str, payload: dict) -> tuple[str, str]:
    canonical = json.dumps(payload, sort_keys=True, separators=(",", ":"))
    digest = hashlib.sha256(canonical.encode("utf-8")).hexdigest()
    verification_id = f"{agent_id}:{run_id}:{digest}"
    return verification_id, digest
