"""Append-only hash-chained audit journal.

A changed byte, a torn line, or a missing trailing newline fails verification.
The chain does not prove the file is complete. A shorter prefix that still
hashes, a rebuilt chain, or a deleted file is not anchored outside this
process. A missing file is not recreated. There is no HMAC key.
"""

from __future__ import annotations

import hashlib
import json
import os
import threading
from pathlib import Path

from bintrans_autopilot.file_lock import exclusive_file_lock

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
        self._lock_path = self.path.with_name(self.path.name + ".lock")
        self._thread = threading.Lock()
        if not self.path.is_file():
            raise AuditJournalError("journal_missing")

    @classmethod
    def create(cls, path: Path) -> AuditJournal:
        path = Path(path)
        path.parent.mkdir(parents=True, exist_ok=True)
        lock_path = path.with_name(path.name + ".lock")
        with exclusive_file_lock(lock_path):
            if path.exists():
                raise AuditJournalError("journal_exists")
            path.write_text("", encoding="utf-8")
        return cls(path)

    def _read(self) -> str:
        try:
            return self.path.read_text(encoding="utf-8")
        except FileNotFoundError:
            raise AuditJournalError("journal_missing") from None
        except OSError:
            raise AuditJournalError("journal_tampered") from None

    def _verify_unlocked(self) -> tuple[dict, ...]:
        text = self._read()
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

    def verify(self) -> tuple[dict, ...]:
        with self._thread:
            with exclusive_file_lock(self._lock_path):
                return self._verify_unlocked()

    def append(self, kind: str, body: dict) -> str:
        if kind not in {"packet", "block"}:
            raise AuditJournalError("invalid_input")
        with self._thread:
            with exclusive_file_lock(self._lock_path):
                records = self._verify_unlocked()
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
