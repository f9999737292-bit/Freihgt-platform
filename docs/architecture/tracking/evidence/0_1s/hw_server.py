#!/usr/bin/env python3
"""BINTRANS 0.1S test-only external high-water authority.

Holds, per witness journal, an append-only history of marks
(journal_incarnation, seq, head_digest). Only the principal `witness_svc` may
write, and only by INIT (once per journal) or ADVANCE by exactly one sequence
number from the current mark (compare-and-set on the previous mark). There is
no lower, set, reset, delete or rewrite operation. The store is its own
hash-chained log with every write request signed and stored; it is fully
re-verified on start and any inconsistency puts the server in FAILED mode.

Independence from the witness journal is LOGICAL ONLY in this model: separate
process, store directory, lock, keys and pipe authkey, on the same machine.
Rollback of this store itself is outside the model (see FAILURE_DOMAIN.md).

Environment: HW_KEYS_JSON (principal -> hex key), HW_AUTHKEY (hex).
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import threading

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from common import GENESIS_HASH, HW_ROLE, build_record, canon, record_hash_ok, request_valid  # noqa: E402
from server_base import StoreLock, append_line, log, serve  # noqa: E402


class HighWater:
    def __init__(self, store: str, keys: dict) -> None:
        self.path = os.path.join(store, "hw_log.jsonl")
        self.keys = keys
        self.records: list[dict] = []
        self.marks: dict[str, dict] = {}
        self.failed: str | None = None
        self.nonces: set[str] = set()
        self.lock = threading.Lock()
        self._load()
        self.fh = open(self.path, "ab")

    def _fail(self, reason: str) -> None:
        self.failed = reason
        log("hw_failed", reason=reason)

    def _load(self) -> None:
        if not os.path.exists(self.path):
            open(self.path, "wb").close()
        prev = GENESIS_HASH
        with open(self.path, "rb") as f:
            for i, raw in enumerate(f, 1):
                try:
                    rec = json.loads(raw.decode("utf-8"))
                except (UnicodeDecodeError, json.JSONDecodeError):
                    return self._fail(f"HW_UNPARSEABLE_RECORD line={i}")
                if rec.get("seq") != i:
                    return self._fail(f"HW_SEQUENCE_VIOLATION line={i}")
                if rec.get("prev_hash") != prev:
                    return self._fail(f"HW_CHAIN_BROKEN seq={i}")
                if not record_hash_ok(rec):
                    return self._fail(f"HW_HASH_MISMATCH seq={i}")
                req = rec.get("request")
                if not request_valid(self.keys, req) or HW_ROLE.get(req.get("principal")) != "writer":
                    return self._fail(f"HW_REQUEST_MAC_INVALID seq={i}")
                body = req.get("body") or {}
                if rec.get("type") != {"init": "INIT", "advance": "ADVANCE"}.get(req.get("op")):
                    return self._fail(f"HW_RECORD_REQUEST_MISMATCH seq={i} field=type")
                for f_ in ("journal_id", "journal_incarnation", "mark_seq", "mark_digest", "prev_digest"):
                    if rec.get(f_) != body.get({"mark_seq": "seq", "mark_digest": "digest",
                                                "journal_incarnation": "incarnation"}.get(f_, f_)):
                        return self._fail(f"HW_RECORD_REQUEST_MISMATCH seq={i} field={f_}")
                err = self._apply(rec)
                if err:
                    return self._fail(f"HW_STATE_INVALID seq={i} {err}")
                self.records.append(rec)
                self.nonces.add(req.get("nonce"))
                prev = rec["hash"]

    def _apply(self, rec: dict) -> str | None:
        jid = rec["journal_id"]
        cur = self.marks.get(jid)
        if rec["type"] == "INIT":
            if cur is not None:
                return "DOUBLE_INIT"
            if rec["mark_seq"] != 0 or rec["mark_digest"] != GENESIS_HASH:
                return "INIT_NOT_ZERO"
            self.marks[jid] = {"journal_incarnation": rec["journal_incarnation"], "seq": 0, "digest": GENESIS_HASH,
                               "history": [[0, GENESIS_HASH]]}
            return None
        if rec["type"] == "ADVANCE":
            if cur is None:
                return "ADVANCE_WITHOUT_INIT"
            if rec["journal_incarnation"] != cur["journal_incarnation"]:
                return "INCARNATION_CHANGED"
            if rec["mark_seq"] != cur["seq"] + 1 or rec["prev_digest"] != cur["digest"]:
                return "NOT_SUCCESSOR"
            cur["seq"], cur["digest"] = rec["mark_seq"], rec["mark_digest"]
            cur["history"].append([rec["mark_seq"], rec["mark_digest"]])
            return None
        return f"UNKNOWN_TYPE {rec['type']}"

    def _append(self, fields: dict, req: dict) -> dict:
        rec = build_record(self.records[-1]["hash"] if self.records else GENESIS_HASH, len(self.records) + 1, fields, req)
        err = self._apply(rec)
        if err:
            raise RuntimeError(err)
        append_line(self.fh, canon(rec))
        self.records.append(rec)
        return rec

    def handle(self, req: dict) -> dict:
        op = req.get("op")
        principal = req.get("principal")
        if self.failed and op not in ("health", "shutdown"):
            return {"status": "HW_FAILED", "reason": self.failed}
        if not request_valid(self.keys, req):
            return {"status": "AUTH_FAILED"}
        if req.get("nonce") in self.nonces:
            return {"status": "NONCE_REPLAYED"}
        self.nonces.add(req.get("nonce"))
        fn = getattr(self, "op_" + str(op), None)
        if fn is None:
            return {"status": "UNSUPPORTED_OP", "op": op}
        return fn(HW_ROLE[principal], req.get("body") or {}, req)

    def _view(self, jid: str) -> dict | None:
        m = self.marks.get(jid)
        if m is None:
            return None
        return {"journal_id": jid, "journal_incarnation": m["journal_incarnation"], "seq": m["seq"],
                "digest": m["digest"], "history": m["history"]}

    def op_health(self, role, body, req):
        return {"status": "FAILED" if self.failed else "OK", "reason": self.failed, "records": len(self.records)}

    def op_get(self, role, body, req):
        if role not in ("reader", "writer"):
            return {"status": "ROLE_FORBIDDEN"}
        v = self._view(body.get("journal_id"))
        return {"status": "OK", "mark": v} if v else {"status": "NO_MARK"}

    def op_init(self, role, body, req):
        if role != "writer":
            return {"status": "ROLE_FORBIDDEN"}
        jid, inc = body.get("journal_id"), body.get("incarnation")
        if not jid or not inc or body.get("seq") != 0 or body.get("digest") != GENESIS_HASH:
            return {"status": "BAD_REQUEST"}
        cur = self.marks.get(jid)
        if cur is not None:
            return {"status": "ALREADY_INITIALIZED", "mark": self._view(jid)}
        self._append({"type": "INIT", "journal_id": jid, "journal_incarnation": inc, "mark_seq": 0,
                      "mark_digest": GENESIS_HASH, "prev_digest": None}, req)
        return {"status": "INITIALIZED", "mark": self._view(jid)}

    def op_advance(self, role, body, req):
        if role != "writer":
            return {"status": "ROLE_FORBIDDEN"}
        jid, inc = body.get("journal_id"), body.get("incarnation")
        seq, digest = body.get("seq"), body.get("digest")
        prev_seq, prev_digest = body.get("prev_seq"), body.get("prev_digest")
        cur = self.marks.get(jid)
        if cur is None:
            return {"status": "NO_MARK"}
        if inc != cur["journal_incarnation"]:
            return {"status": "INCARNATION_MISMATCH", "mark": self._view(jid)}
        if not isinstance(seq, int) or not isinstance(digest, str) or len(digest) != 64:
            return {"status": "BAD_REQUEST"}
        if seq <= cur["seq"]:
            known = dict((s, d) for s, d in cur["history"])
            if known.get(seq) == digest:
                return {"status": "IDEMPOTENT", "mark": self._view(jid)}
            if seq < cur["seq"]:
                return {"status": "NOT_MONOTONIC", "mark": self._view(jid)}
            return {"status": "CONFLICT", "mark": self._view(jid)}
        if seq != cur["seq"] + 1:
            return {"status": "GAP", "mark": self._view(jid)}
        if prev_seq != cur["seq"] or prev_digest != cur["digest"]:
            return {"status": "PREV_MISMATCH", "mark": self._view(jid)}
        self._append({"type": "ADVANCE", "journal_id": jid, "journal_incarnation": inc, "mark_seq": seq,
                      "mark_digest": digest, "prev_digest": prev_digest}, req)
        return {"status": "ADVANCED", "mark": {k: v for k, v in self._view(jid).items() if k != "history"}}

    def op_shutdown(self, role, body, req):
        if role != "fault":
            return {"status": "ROLE_FORBIDDEN"}
        return {"status": "SHUTDOWN"}


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--store", required=True)
    ap.add_argument("--pipe", required=True)
    args = ap.parse_args()
    keys = json.loads(os.environ["HW_KEYS_JSON"])
    authkey = bytes.fromhex(os.environ["HW_AUTHKEY"])
    os.makedirs(args.store, exist_ok=True)
    try:
        lock = StoreLock(os.path.join(args.store, "store.lock"))
    except OSError:
        log("store_locked", store=os.path.basename(args.store))
        return 3
    hw = HighWater(args.store, keys)
    log("ready", pid=os.getpid(), records=len(hw.records), failed=hw.failed,
        marks={j: {"seq": m["seq"], "digest": m["digest"]} for j, m in hw.marks.items()})

    def on_shutdown() -> None:
        hw.fh.close()
        lock.f.close()

    return serve(hw, args.pipe, authkey, on_shutdown)


if __name__ == "__main__":
    sys.exit(main())
