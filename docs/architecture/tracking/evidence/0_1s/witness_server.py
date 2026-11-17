#!/usr/bin/env python3
"""BINTRANS 0.1S test-only issuance witness with an external high-water check.

Same journal model as 0.1R (exclusive store lock, hash-chained JSONL with fsync
per append, signed requests, no update/delete, full verification on load,
operator checkpoint), plus:

* The journal starts with a GENESIS record carrying a random journal
  incarnation id.
* Every append is covered by a mark in the separate high-water authority
  (hw_server.py). A write is acknowledged only after the mark covering it has
  been accepted there.
* On start, and before serving any request while not CONSISTENT, the witness
  compares the journal with the full high-water history: incarnation, length,
  and the digest at every pinned sequence. Any mismatch is a permanent FAILED
  state for this process (no automatic recovery). An unreachable or failed
  high-water authority closes the witness (HIGH_WATER_UNAVAILABLE) until the
  check succeeds.

Write orders (--order):
  append_first  journal append (fsync) -> high-water advance -> reply   [selected]
  hw_first      high-water advance -> journal append (fsync) -> reply   [comparison only]

The journal and the record it may be ahead by are never declared one atomic
transaction: with append_first the journal may be ahead of the mark by exactly
one unacknowledged record (PREPARED); it is rolled forward only if it chains
from the pinned head.

Environment: WITNESS_KEYS_JSON, WITNESS_AUTHKEY, HW_AUTHKEY, HW_KEY (the
witness_svc key for the high-water authority).
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import threading
import uuid

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from common import (ALLOWED, GENESIS_HASH, ROLE, WitnessClient, WitnessUnavailable, build_record,  # noqa: E402
                    canon, record_hash_ok, request_valid)
from server_base import StoreLock, append_line, log, serve  # noqa: E402

WRITE_OPS = ("epoch_set", "reserve", "outcome", "checkpoint_accept")


class Witness:
    def __init__(self, store: str, checkpoint_dir: str, keys: dict, journal_id: str, order: str,
                 hw: WitnessClient) -> None:
        self.log_path = os.path.join(store, "witness_log.jsonl")
        self.cp_path = os.path.join(checkpoint_dir, "accepted_checkpoint.json")
        self.keys = keys
        self.journal_id = journal_id
        self.order = order
        self.hw = hw
        self.records: list[dict] = []
        self.keys_state: dict[tuple, dict] = {}
        self.epochs: dict[str, dict] = {}
        self.checkpoint = {"seq": 0, "head_hash": GENESIS_HASH}
        self.journal_incarnation: str | None = None
        self.failed: str | None = None
        self.hw_state = "UNCHECKED"
        self.hw_reason: str | None = None
        self.hw_mark: dict | None = None
        self.fault: str | None = None
        self.fault_on: str | None = None
        self.nonces: set[str] = set()
        self.lock = threading.Lock()
        self._load()
        self.fh = open(self.log_path, "ab")
        if not self.failed:
            self._sync_hw()

    # ---- local verification (as 0.1R) -------------------------------
    def _fail(self, reason: str) -> None:
        if self.failed is None:
            self.failed = reason
            log("witness_failed", reason=reason)

    def _load(self) -> None:
        if not os.path.exists(self.log_path):
            open(self.log_path, "wb").close()
        prev = GENESIS_HASH
        with open(self.log_path, "rb") as f:
            for i, raw in enumerate(f, 1):
                try:
                    rec = json.loads(raw.decode("utf-8"))
                except (UnicodeDecodeError, json.JSONDecodeError):
                    return self._fail(f"UNPARSEABLE_RECORD line={i}")
                if rec.get("seq") != i:
                    return self._fail(f"SEQUENCE_VIOLATION line={i} seq={rec.get('seq')}")
                if rec.get("prev_hash") != prev:
                    return self._fail(f"CHAIN_BROKEN seq={i}")
                if not record_hash_ok(rec):
                    return self._fail(f"HASH_MISMATCH seq={i}")
                if (rec.get("type") == "GENESIS") != (i == 1):
                    return self._fail(f"GENESIS_POSITION seq={i}")
                if i > 1:
                    req = rec.get("request")
                    if not request_valid(self.keys, req):
                        return self._fail(f"REQUEST_MAC_INVALID seq={i}")
                    field = self._record_request_mismatch(rec, req)
                    if field:
                        return self._fail(f"RECORD_REQUEST_MISMATCH seq={i} field={field}")
                    self.nonces.add(req.get("nonce"))
                err = self._replay(rec)
                if err:
                    return self._fail(f"STATE_INVALID seq={i} {err}")
                self.records.append(rec)
                prev = rec["hash"]
        if os.path.exists(self.cp_path):
            try:
                cp = json.loads(open(self.cp_path, "rb").read().decode("utf-8"))
            except (UnicodeDecodeError, json.JSONDecodeError):
                return self._fail("CHECKPOINT_UNPARSEABLE")
            req = cp.get("request", {})
            if req.get("principal") != "operator" or not request_valid(self.keys, req):
                return self._fail("CHECKPOINT_MAC_INVALID")
            seq, head = req["body"]["seq"], req["body"]["head_hash"]
            if seq > len(self.records):
                return self._fail(f"ROLLBACK_DETECTED source=operator_checkpoint journal_len={len(self.records)} "
                                  f"checkpoint_seq={seq}")
            if seq > 0 and self.records[seq - 1]["hash"] != head:
                return self._fail(f"FORK_DETECTED source=operator_checkpoint seq={seq}")
            self.checkpoint = {"seq": seq, "head_hash": head}

    @staticmethod
    def _record_request_mismatch(rec: dict, req: dict) -> str | None:
        body = req.get("body") or {}
        spec = {
            "EPOCH": ("epoch_set", ("ns", "epoch", "holder")),
            "RESERVE": ("reserve", ("ns", "request_id", "payload_hash", "ticket_id", "incarnation", "epoch")),
            "OUTCOME": ("outcome", ("ns", "request_id", "ticket_id", "outcome", "evidence")),
        }.get(rec.get("type"))
        if spec is None:
            return "type"
        if req.get("op") != spec[0]:
            return "op"
        for f in spec[1]:
            if rec.get(f) != body.get(f):
                return f
        if rec.get("type") != "EPOCH" and rec.get("principal") != req.get("principal"):
            return "principal"
        return None

    def _replay(self, rec: dict) -> str | None:
        t = rec.get("type")
        if t == "GENESIS":
            if rec.get("journal_id") != self.journal_id:
                return "JOURNAL_ID_MISMATCH"
            self.journal_incarnation = rec.get("journal_incarnation")
            return None
        if t == "EPOCH":
            self.epochs[rec["ns"]] = {"epoch": rec["epoch"], "holder": rec["holder"]}
            return None
        k = (rec.get("ns"), rec.get("request_id"))
        if t == "RESERVE":
            if k in self.keys_state:
                return "DUPLICATE_RESERVE_IN_LOG"
            self.keys_state[k] = {"state": "RESERVED", "reservation": rec, "history": [rec["seq"]]}
            return None
        if t == "OUTCOME":
            cur = self.keys_state.get(k)
            if cur is None:
                return "OUTCOME_WITHOUT_RESERVATION"
            if rec["ticket_id"] != cur["reservation"]["ticket_id"]:
                return "OUTCOME_TICKET_MISMATCH"
            role = ROLE.get(rec["request"]["principal"])
            if (role, cur["state"], rec["outcome"]) not in ALLOWED:
                return "OUTCOME_TRANSITION_INVALID"
            cur["state"] = rec["outcome"]
            cur["history"].append(rec["seq"])
            return None
        return f"UNKNOWN_TYPE {t}"

    # ---- high-water comparison ---------------------------------------
    def _hw(self, op: str, **body) -> dict:
        try:
            return self.hw.call(op, **body)
        except WitnessUnavailable as exc:
            return {"status": "HW_UNREACHABLE", "error": str(exc)}

    def _unavailable(self, reason: str) -> None:
        self.hw_state = "HW_UNAVAILABLE"
        self.hw_reason = reason
        log("hw_unavailable", reason=reason)

    def _advance(self, rec: dict) -> dict:
        m = self.hw_mark
        return self._hw("advance", journal_id=self.journal_id, incarnation=self.journal_incarnation,
                        seq=rec["seq"], digest=rec["hash"], prev_seq=m["seq"], prev_digest=m["digest"])

    def _write_genesis(self, incarnation: str) -> dict:
        rec = build_record(GENESIS_HASH, 1, {"type": "GENESIS", "journal_id": self.journal_id,
                                              "journal_incarnation": incarnation}, None)
        append_line(self.fh, canon(rec))
        self._replay(rec)
        self.records.append(rec)
        return rec

    def _sync_hw(self) -> None:
        """Compare journal with the high-water history. Sets CONSISTENT, HW_UNAVAILABLE, or FAILED."""
        r = self._hw("get", journal_id=self.journal_id)
        if r["status"] in ("HW_UNREACHABLE", "HW_FAILED"):
            return self._unavailable(f"{r['status']} {r.get('error') or r.get('reason')}")
        n = len(self.records)
        if r["status"] == "NO_MARK":
            if n:
                return self._fail(f"HIGH_WATER_MISSING journal_len={n}")
            inc = str(uuid.uuid4())
            ri = self._hw("init", journal_id=self.journal_id, incarnation=inc, seq=0, digest=GENESIS_HASH)
            if ri["status"] != "INITIALIZED":
                return self._unavailable(f"HW_INIT {ri['status']}")
            log("hw_initialized", journal_incarnation=inc)
            return self._sync_hw()
        if r["status"] != "OK":
            return self._unavailable(f"HW_GET {r['status']}")
        m = r["mark"]
        self.hw_mark = {k: m[k] for k in ("journal_incarnation", "seq", "digest")}
        if n == 0 and m["seq"] == 0:
            self._write_genesis(m["journal_incarnation"])
            log("genesis_written", journal_incarnation=m["journal_incarnation"])
            n = 1
        if n < m["seq"]:
            return self._fail(f"ROLLBACK_DETECTED source=high_water journal_len={n} high_water_seq={m['seq']}")
        if self.journal_incarnation != m["journal_incarnation"]:
            return self._fail("INCARNATION_MISMATCH source=high_water")
        for s, d in m["history"]:
            if s >= 1 and self.records[s - 1]["hash"] != d:
                return self._fail(f"FORK_DETECTED source=high_water seq={s}")
        if n == m["seq"]:
            self.hw_state, self.hw_reason = "CONSISTENT", None
            return None
        if n == m["seq"] + 1:
            ra = self._advance(self.records[-1])
            if ra["status"] in ("ADVANCED", "IDEMPOTENT"):
                self.hw_mark = {"journal_incarnation": m["journal_incarnation"], "seq": n, "digest": self.records[-1]["hash"]}
                self.hw_state, self.hw_reason = "CONSISTENT", None
                log("roll_forward", seq=n, type=self.records[-1]["type"])
                return None
            if ra["status"] in ("HW_UNREACHABLE", "HW_FAILED"):
                self.hw_state, self.hw_reason = "PENDING", f"{ra['status']} prepared_seq={n}"
                return None
            return self._fail(f"HW_ADVANCE_REJECTED {ra['status']} seq={n}")
        return self._fail(f"UNCOVERED_SUFFIX journal_len={n} high_water_seq={m['seq']}")

    def _ready(self) -> dict | None:
        if self.failed:
            return {"status": "WITNESS_FAILED", "reason": self.failed}
        if self.hw_state != "CONSISTENT":
            self._sync_hw()
        if self.failed:
            return {"status": "WITNESS_FAILED", "reason": self.failed}
        if self.hw_state != "CONSISTENT":
            return {"status": "HIGH_WATER_UNAVAILABLE", "reason": self.hw_reason, "hw_state": self.hw_state}
        return None

    # ---- append -------------------------------------------------------
    def _maybe_crash(self, point: str, rec_type: str) -> None:
        if self.fault == point and (self.fault_on in (None, rec_type)):
            log("fault_crash", point=point, type=rec_type, journal_len=len(self.records),
                hw_seq=(self.hw_mark or {}).get("seq"))
            os._exit(137)

    def _append(self, fields: dict, req: dict) -> tuple[dict | None, dict | None]:
        t = fields["type"]
        self._maybe_crash("crash_before_append", t)
        rec = build_record(self.records[-1]["hash"], len(self.records) + 1, fields, req)
        if self.order == "hw_first":
            ra = self._advance(rec)
            if ra["status"] != "ADVANCED":
                if ra["status"] in ("HW_UNREACHABLE", "HW_FAILED"):
                    self._unavailable(f"{ra['status']} before append (hw_first)")
                    return None, {"status": "HIGH_WATER_UNAVAILABLE", "durability": "UNKNOWN_HW_FIRST"}
                self._fail(f"HW_ADVANCE_REJECTED {ra['status']} seq={rec['seq']}")
                return None, {"status": "WITNESS_FAILED", "reason": self.failed}
            self.hw_mark = {"journal_incarnation": self.journal_incarnation, "seq": rec["seq"], "digest": rec["hash"]}
            self._maybe_crash("crash_after_hw_before_append", t)
            append_line(self.fh, canon(rec))
            err = self._replay(rec)
            if err:
                raise RuntimeError(err)
            self.records.append(rec)
            self._maybe_crash("crash_after_hw_before_reply", t)
            return rec, None
        append_line(self.fh, canon(rec))
        err = self._replay(rec)
        if err:
            raise RuntimeError(err)
        self.records.append(rec)
        self._maybe_crash("crash_after_append_before_hw", t)
        ra = self._advance(rec)
        if ra["status"] == "ADVANCED":
            self.hw_mark = {"journal_incarnation": self.journal_incarnation, "seq": rec["seq"], "digest": rec["hash"]}
            self._maybe_crash("crash_after_hw_before_reply", t)
            return rec, None
        if ra["status"] in ("HW_UNREACHABLE", "HW_FAILED"):
            self.hw_state, self.hw_reason = "PENDING", f"{ra['status']} prepared_seq={rec['seq']}"
            log("prepared_not_covered", seq=rec["seq"], type=t)
            return None, {"status": "HIGH_WATER_UNAVAILABLE", "durability": "PREPARED", "journal_seq": rec["seq"]}
        self._fail(f"HW_ADVANCE_REJECTED {ra['status']} seq={rec['seq']}")
        return None, {"status": "WITNESS_FAILED", "reason": self.failed}

    # ---- request handling --------------------------------------------
    def handle(self, req: dict) -> dict:
        op = req.get("op")
        principal = req.get("principal")
        if not request_valid(self.keys, req):
            return {"status": "AUTH_FAILED"}
        if req.get("nonce") in self.nonces:
            return {"status": "NONCE_REPLAYED"}
        self.nonces.add(req.get("nonce"))
        role = ROLE[principal]
        body = req.get("body") or {}
        fn = getattr(self, "op_" + str(op), None)
        if fn is None:
            return {"status": "UNSUPPORTED_OP", "op": op}
        if op in WRITE_OPS:
            blocked = self._ready()
            if blocked:
                return blocked
        return fn(role, principal, body, req)

    def _keys_view(self) -> dict:
        return {f"{k[0]}|{k[1]}": {"state": v["state"], "ticket_id": v["reservation"]["ticket_id"],
                                   "payload_hash": v["reservation"]["payload_hash"],
                                   "principal": v["reservation"]["principal"], "epoch": v["reservation"]["epoch"],
                                   "incarnation": v["reservation"]["incarnation"], "history": v["history"]}
                for k, v in self.keys_state.items()}

    def op_health(self, role, principal, body, req):
        blocked = self._ready()
        return {"status": blocked["status"] if blocked else "OK", "reason": (blocked or {}).get("reason"),
                "hw_state": self.hw_state, "records": len(self.records), "high_water": self.hw_mark,
                "checkpoint": self.checkpoint}

    def op_export(self, role, principal, body, req):
        blocked = self._ready()
        out = {"status": blocked["status"] if blocked else "OK", "reason": (blocked or {}).get("reason"),
               "journal_id": self.journal_id, "journal_incarnation": self.journal_incarnation,
               "hw_state": self.hw_state, "high_water": self.hw_mark, "records": self.records,
               "checkpoint": self.checkpoint}
        if not blocked:
            out.update({"epochs": self.epochs, "keys": self._keys_view()})
        return out

    def op_epoch_set(self, role, principal, body, req):
        if role != "operator":
            return {"status": "ROLE_FORBIDDEN"}
        ns, epoch, holder = body.get("ns"), body.get("epoch"), body.get("holder")
        cur = self.epochs.get(ns)
        if ROLE.get(holder) != "issuer" or not isinstance(epoch, int):
            return {"status": "BAD_REQUEST"}
        if cur and epoch <= cur["epoch"]:
            return {"status": "EPOCH_NOT_MONOTONIC", "current": cur}
        rec, err = self._append({"type": "EPOCH", "ns": ns, "epoch": epoch, "holder": holder}, req)
        return err or {"status": "EPOCH_SET", "seq": rec["seq"]}

    def op_reserve(self, role, principal, body, req):
        if role != "issuer":
            return {"status": "ROLE_FORBIDDEN"}
        ns, rid = body.get("ns"), body.get("request_id")
        ph, tid = body.get("payload_hash"), body.get("ticket_id")
        if not (isinstance(ph, str) and len(ph) == 64):
            return {"status": "BAD_REQUEST", "reason": "payload_hash"}
        try:
            uuid.UUID(str(tid))
        except ValueError:
            return {"status": "BAD_REQUEST", "reason": "ticket_id"}
        ep = self.epochs.get(ns)
        if ep is None:
            return {"status": "NO_EPOCH"}
        if ep["holder"] != principal:
            return {"status": "NOT_EPOCH_HOLDER", "holder": ep["holder"], "epoch": ep["epoch"]}
        if body.get("epoch") != ep["epoch"]:
            return {"status": "STALE_EPOCH", "current": ep["epoch"]}
        cur = self.keys_state.get((ns, rid))
        if cur is not None:
            r = cur["reservation"]
            same = (r["principal"] == principal and r["ticket_id"] == tid and r["payload_hash"] == ph
                    and r["incarnation"] == body.get("incarnation"))
            if same and cur["state"] == "RESERVED":
                return {"status": "ALREADY_RESERVED_SAME", "seq": r["seq"], "state": cur["state"]}
            return {"status": "DUPLICATE", "state": cur["state"], "reservation_seq": r["seq"]}
        rec, err = self._append({"type": "RESERVE", "ns": ns, "request_id": rid, "payload_hash": ph, "ticket_id": tid,
                                 "principal": principal, "epoch": ep["epoch"], "incarnation": body.get("incarnation")}, req)
        return err or {"status": "RESERVED", "seq": rec["seq"], "epoch": ep["epoch"], "high_water_seq": self.hw_mark["seq"]}

    def op_outcome(self, role, principal, body, req):
        if role not in ("issuer", "operator"):
            return {"status": "ROLE_FORBIDDEN"}
        ns, rid = body.get("ns"), body.get("request_id")
        target, tid, ev = body.get("outcome"), body.get("ticket_id"), body.get("evidence") or {}
        cur = self.keys_state.get((ns, rid))
        if cur is None:
            return {"status": "NO_RESERVATION"}
        r = cur["reservation"]
        if tid != r["ticket_id"]:
            return {"status": "TICKET_MISMATCH"}
        if role == "issuer" and r["principal"] != principal:
            return {"status": "NOT_RESERVATION_OWNER"}
        if cur["state"] == target:
            return {"status": "IDEMPOTENT", "state": cur["state"]}
        if (role, cur["state"], target) not in ALLOWED:
            return {"status": "INVALID_TRANSITION", "from": cur["state"], "to": target, "role": role}
        if role == "issuer" and target == "COMMIT_CONFIRMED" and ev.get("pg_commit_ack") is not True:
            return {"status": "EVIDENCE_REQUIRED"}
        if role == "issuer" and target == "ABORT_CONFIRMED" and ev.get("pg_rollback_ack") is not True:
            return {"status": "EVIDENCE_REQUIRED"}
        if role == "operator" and target == "COMMIT_CONFIRMED":
            if ev.get("tombstone_ticket_id") != r["ticket_id"] or ev.get("tombstone_payload_hash") != r["payload_hash"]:
                return {"status": "EVIDENCE_MISMATCH"}
        if target in ("IN_DOUBT", "PERMANENTLY_DENIED") and not ev.get("reason"):
            return {"status": "EVIDENCE_REQUIRED"}
        rec, err = self._append({"type": "OUTCOME", "ns": ns, "request_id": rid, "ticket_id": tid, "outcome": target,
                                 "evidence": body.get("evidence"), "principal": principal}, req)
        return err or {"status": "APPENDED", "seq": rec["seq"], "state": target}

    def op_checkpoint_accept(self, role, principal, body, req):
        if role != "operator":
            return {"status": "ROLE_FORBIDDEN"}
        seq, head = body.get("seq"), body.get("head_hash")
        if not isinstance(seq, int) or seq < 0 or seq > len(self.records):
            return {"status": "BAD_REQUEST"}
        if seq < self.checkpoint["seq"]:
            return {"status": "CHECKPOINT_LOWER_REJECTED", "current": self.checkpoint}
        if seq > 0 and self.records[seq - 1]["hash"] != head:
            return {"status": "CHECKPOINT_HASH_MISMATCH"}
        tmp = self.cp_path + ".tmp"
        with open(tmp, "wb") as f:
            f.write(canon({"request": req}) + b"\n")
            f.flush()
            os.fsync(f.fileno())
        os.replace(tmp, self.cp_path)
        self.checkpoint = {"seq": seq, "head_hash": head}
        return {"status": "CHECKPOINT_ACCEPTED", "checkpoint": self.checkpoint}

    def op_fault(self, role, principal, body, req):
        if role != "fault":
            return {"status": "ROLE_FORBIDDEN"}
        self.fault, self.fault_on = body.get("fault"), body.get("on_type")
        return {"status": "FAULT_ARMED", "name": self.fault, "on_type": self.fault_on}

    def op_shutdown(self, role, principal, body, req):
        if role != "fault":
            return {"status": "ROLE_FORBIDDEN"}
        return {"status": "SHUTDOWN"}


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--store", required=True)
    ap.add_argument("--checkpoint-dir", required=True)
    ap.add_argument("--pipe", required=True)
    ap.add_argument("--hw-pipe", required=True)
    ap.add_argument("--journal-id", required=True)
    ap.add_argument("--order", choices=("append_first", "hw_first"), default="append_first")
    args = ap.parse_args()
    keys = json.loads(os.environ["WITNESS_KEYS_JSON"])
    authkey = bytes.fromhex(os.environ["WITNESS_AUTHKEY"])
    hw = WitnessClient(args.hw_pipe, os.environ["HW_AUTHKEY"], "witness_svc", os.environ["HW_KEY"], timeout=3.0)
    os.makedirs(args.store, exist_ok=True)
    os.makedirs(args.checkpoint_dir, exist_ok=True)
    try:
        lock = StoreLock(os.path.join(args.store, "store.lock"))
    except OSError:
        log("store_locked", store=os.path.basename(args.store))
        return 3
    w = Witness(args.store, args.checkpoint_dir, keys, args.journal_id, args.order, hw)
    log("ready", pid=os.getpid(), order=args.order, records=len(w.records), failed=w.failed, hw_state=w.hw_state,
        hw_reason=w.hw_reason, high_water=w.hw_mark, checkpoint=w.checkpoint)

    def on_shutdown() -> None:
        w.fh.close()
        lock.f.close()

    return serve(w, args.pipe, authkey, on_shutdown)


if __name__ == "__main__":
    sys.exit(main())
