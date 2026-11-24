"""Append-only hash-chained audit journal.

Records can be added. They cannot be updated or deleted through this type.
A rewritten or truncated file fails verification and the gate stays blocked.
This file is not a trusted reviewer. It only preserves what was submitted.
"""

from __future__ import annotations

import hashlib
import json
import os
import threading
from pathlib import Path

_GENESIS = "0" * 64


class AuditJournalError(Exception):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


def _digest(prev: str, kind: str, body: dict) -> str:
    encoded = json.dumps(body, sort_keys=True, separators=(",", ":"), ensure_ascii=True)
    return hashlib.sha256(f"{prev}\n{kind}\n{encoded}".encode("ascii")).hexdigest()


class AuditJournal:
    def __init__(self, path: Path):
        self.path = Path(path)
        self.path.parent.mkdir(parents=True, exist_ok=True)
        if not self.path.exists():
            self.path.write_text("", encoding="utf-8")
        self._lock = threading.Lock()

    def verify(self) -> tuple[dict, ...]:
        text = self.path.read_text(encoding="utf-8")
        if text == "":
            return ()
        if not text.endswith("\n") or "\r" in text:
            raise AuditJournalError("journal_tampered")
        records: list[dict] = []
        prev = _GENESIS
        for line in text.split("\n"):
            if line == "":
                continue
            try:
                record = json.loads(line)
                kind = str(record["kind"])
                body = record["body"]
                claimed_prev = str(record["prev"])
                claimed_hash = str(record["hash"])
            except (KeyError, TypeError, json.JSONDecodeError):
                raise AuditJournalError("journal_tampered") from None
            if not isinstance(body, dict):
                raise AuditJournalError("journal_tampered")
            digest = _digest(prev, kind, body)
            if claimed_prev != prev or claimed_hash != digest:
                raise AuditJournalError("journal_tampered")
            prev = digest
            records.append(record)
        return tuple(records)

    def append(self, kind: str, body: dict) -> str:
        if kind not in {"packet", "block"}:
            raise AuditJournalError("invalid_input")
        with self._lock:
            records = self.verify()
            prev = records[-1]["hash"] if records else _GENESIS
            digest = _digest(prev, kind, body)
            line = json.dumps(
                {"body": body, "hash": digest, "kind": kind, "prev": prev},
                sort_keys=True,
                separators=(",", ":"),
                ensure_ascii=True,
            )
            with self.path.open("a", encoding="utf-8", newline="\n") as handle:
                handle.write(line + "\n")
                handle.flush()
                os.fsync(handle.fileno())
            return digest

    def clear(self) -> None:
        raise AuditJournalError("journal_immutable")

    def delete(self) -> None:
        raise AuditJournalError("journal_immutable")

    def update(self, *_args, **_kwargs) -> None:
        raise AuditJournalError("journal_immutable")
