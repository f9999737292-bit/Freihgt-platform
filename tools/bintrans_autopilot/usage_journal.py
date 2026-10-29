"""Historical Cloud Agent usage records.

This file is the only stored usage journal. It is not the live budget stop in
`budget.py` and it does not change `autopilot.config.json`. A missing amount
stays null. A recorded zero is the server value for that run.

The writer is local. It is not a multi-host ledger.
"""

from __future__ import annotations

import json
import os
import re
from dataclasses import dataclass
from decimal import Decimal, InvalidOperation
from pathlib import Path

JOURNAL_PATH = Path(__file__).with_name("usage_journal.json")
_AGENT_ID = re.compile(r"^bc-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")
_RUN_ID = re.compile(r"^run-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")
_SECRET = re.compile(r"(cursor_[A-Za-z0-9]|eyJ[A-Za-z0-9_-]|Bearer\s+|api[_-]?key\s*[:=])", re.IGNORECASE)
_TOKEN_FIELDS = (
    "input_tokens",
    "output_tokens",
    "cache_read_tokens",
    "cache_write_tokens",
    "total_tokens",
)


class UsageJournalError(Exception):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


@dataclass(frozen=True)
class UsageRecord:
    agent_id: str
    run_id: str
    input_tokens: int | None
    output_tokens: int | None
    cache_read_tokens: int | None
    cache_write_tokens: int | None
    total_tokens: int | None
    raw_cost_cents: str | None
    charged_cents: str | None
    source: str
    source_http_status: int

    def to_dict(self) -> dict:
        return {
            "agent_id": self.agent_id,
            "run_id": self.run_id,
            "input_tokens": self.input_tokens,
            "output_tokens": self.output_tokens,
            "cache_read_tokens": self.cache_read_tokens,
            "cache_write_tokens": self.cache_write_tokens,
            "total_tokens": self.total_tokens,
            "raw_cost_cents": self.raw_cost_cents,
            "charged_cents": self.charged_cents,
            "source": self.source,
            "source_http_status": self.source_http_status,
        }


@dataclass(frozen=True)
class RecordResult:
    status: str
    record: UsageRecord


def record_usage(path: Path, raw: dict) -> RecordResult:
    """Insert a run once. The same agent id and run id with the same payload is unchanged."""
    incoming = _normalize(raw)
    _reject_secrets(incoming.to_dict())
    data = _load(path)
    key = (incoming.agent_id, incoming.run_id)
    for existing in data["records"]:
        if (existing.get("agent_id"), existing.get("run_id")) != key:
            continue
        if existing == incoming.to_dict():
            return RecordResult("unchanged", incoming)
        raise UsageJournalError("conflict")
    data["records"].append(incoming.to_dict())
    _write(path, data)
    return RecordResult("created", incoming)


def load_records(path: Path) -> list[UsageRecord]:
    return [_from_dict(item) for item in _load(path)["records"]]


def _normalize(raw: dict) -> UsageRecord:
    agent_id = str(raw.get("agent_id") or "")
    run_id = str(raw.get("run_id") or "")
    if not _AGENT_ID.fullmatch(agent_id) or not _RUN_ID.fullmatch(run_id):
        raise UsageJournalError("invalid_identity")
    source = str(raw.get("source") or "")
    if source == "":
        raise UsageJournalError("invalid_source")
    status = raw.get("source_http_status")
    if isinstance(status, bool) or not isinstance(status, int) or not 100 <= status <= 599:
        raise UsageJournalError("invalid_source")
    tokens = {name: _optional_int(raw.get(name), name) for name in _TOKEN_FIELDS}
    components = (
        tokens["input_tokens"],
        tokens["output_tokens"],
        tokens["cache_read_tokens"],
        tokens["cache_write_tokens"],
    )
    if any(item is None for item in components):
        if tokens["total_tokens"] is not None:
            raise UsageJournalError("unknown_tokens")
    elif tokens["total_tokens"] != sum(components):
        raise UsageJournalError("token_mismatch")
    return UsageRecord(
        agent_id=agent_id,
        run_id=run_id,
        input_tokens=tokens["input_tokens"],
        output_tokens=tokens["output_tokens"],
        cache_read_tokens=tokens["cache_read_tokens"],
        cache_write_tokens=tokens["cache_write_tokens"],
        total_tokens=tokens["total_tokens"],
        raw_cost_cents=_amount(raw, "raw_cost_cents"),
        charged_cents=_amount(raw, "charged_cents"),
        source=source,
        source_http_status=status,
    )


def _optional_int(value: object, name: str) -> int | None:
    del name
    if value is None:
        return None
    if isinstance(value, bool) or not isinstance(value, int) or value < 0:
        raise UsageJournalError("invalid_tokens")
    return value


def _amount(raw: dict, name: str) -> str | None:
    if name not in raw or raw[name] is None:
        return None
    value = raw[name]
    if isinstance(value, bool):
        raise UsageJournalError("invalid_amount")
    try:
        amount = Decimal(str(value).strip())
    except (InvalidOperation, AttributeError) as exc:
        raise UsageJournalError("invalid_amount") from exc
    if amount < 0:
        raise UsageJournalError("invalid_amount")
    text = format(amount, "f")
    if "." in text:
        text = text.rstrip("0").rstrip(".")
    return text or "0"


def _reject_secrets(payload: dict) -> None:
    for value in payload.values():
        if isinstance(value, str) and _SECRET.search(value):
            raise UsageJournalError("secret_rejected")


def _from_dict(raw: dict) -> UsageRecord:
    return _normalize(raw)


def _load(path: Path) -> dict:
    if not path.exists():
        return {"version": 1, "records": []}
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise UsageJournalError("corrupt") from exc
    if not isinstance(data, dict) or data.get("version") != 1 or not isinstance(data.get("records"), list):
        raise UsageJournalError("corrupt")
    return data


def _write(path: Path, data: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_name(path.name + ".tmp")
    payload = (json.dumps(data, indent=2, sort_keys=True) + "\n").encode("utf-8")
    fd = os.open(temporary, os.O_CREAT | os.O_TRUNC | os.O_RDWR, 0o600)
    try:
        os.write(fd, payload)
        os.fsync(fd)
    finally:
        os.close(fd)
    os.replace(temporary, path)
