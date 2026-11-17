#!/usr/bin/env python3
"""BINTRANS 0.1R test-only issuance witness server.

One process owns one store directory. It holds an exclusive OS lock on the store
for its whole lifetime, keeps a hash-chained JSONL log with fsync per append,
and serves signed requests over a local named pipe (no network). There is no
update or delete operation. On start it re-verifies the whole log and the
operator checkpoint; any inconsistency puts it in a FAILED mode where every
request is refused.

This is a model for experiments. A file lock, a local log, and HMAC keys on the
same machine are not a production-grade append-only witness.

Environment: WITNESS_KEYS_JSON (principal -> hex key), WITNESS_AUTHKEY (hex).
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import threading
import time
import uuid
from multiprocessing import AuthenticationError
from multiprocessing.connection import Listener

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from common import ALLOWED, ROLE, canon, pipe_address, sha256_hex, verify  # noqa: E402

GENESIS = "0" * 64


def log(event: str, **kw) -> None:
    print(json.dumps({"t": time.time(), "event": event, **kw}, sort_keys=True), flush=True)


class StoreLock:
    def __init__(self, path: str) -> None:
        self.f = open(path, "a+b")
        try:
            if os.name == "nt":
                import msvcrt
                self.f.seek(0)
                msvcrt.locking(self.f.fileno(), msvcrt.LK_NBLCK, 1)
            else:
                import fcntl
                fcntl.flock(self.f.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError:
            self.f.close()
            raise


class Witness:
    def __init__(self, store: str, checkpoint_dir: str, keys: dict) -> None:
        self.log_path = os.path.join(store, "witness_log.jsonl")
        self.cp_path = os.path.join(checkpoint_dir, "accepted_checkpoint.json")
        self.keys = keys
        self.records: list[dict] = []
        self.keys_state: dict[tuple, dict] = {}
        self.epochs: dict[str, dict] = {}
        self.checkpoint = {"seq": 0, "head_hash": GENESIS}
        self.failed: str | None = None
        self.fault: str | None = None
        self.nonces: set[str] = set()
        self.lock = threading.Lock()
        self._load()
        self.fh = open(self.log_path, "ab")

    # ---- loading and verification -----------------------------------
    def _load(self) -> None:
        if not os.path.exists(self.log_path):
            open(self.log_path, "wb").close()
        prev = GENESIS
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
                body = {k: v for k, v in rec.items() if k != "hash"}
                if sha256_hex(canon(body)) != rec.get("hash"):
                    return self._fail(f"HASH_MISMATCH seq={i}")
                req = rec.get("request", {})
                key = self.keys.get(req.get("principal"))
                signed = {k: req.get(k) for k in ("op", "principal", "nonce", "body")}
                if key is None or not verify(key, signed, req.get("mac")):
                    return self._fail(f"REQUEST_MAC_INVALID seq={i}")
                field = self._record_request_mismatch(rec, req)
                if field:
                    return self._fail(f"RECORD_REQUEST_MISMATCH seq={i} field={field}")
                err = self._replay(rec)
                if err:
                    return self._fail(f"STATE_INVALID seq={i} {err}")
                self.records.append(rec)
                self.nonces.add(req.get("nonce"))
                prev = rec["hash"]
        if os.path.exists(self.cp_path):
            try:
                cp = json.loads(open(self.cp_path, "rb").read().decode("utf-8"))
            except (UnicodeDecodeError, json.JSONDecodeError):
                return self._fail("CHECKPOINT_UNPARSEABLE")
            req = cp.get("request", {})
            signed = {k: req.get(k) for k in ("op", "principal", "nonce", "body")}
            if req.get("principal") != "operator" or not verify(self.keys["operator"], signed, req.get("mac")):
                return self._fail("CHECKPOINT_MAC_INVALID")
            seq = req["body"]["seq"]
            head = req["body"]["head_hash"]
            if seq > len(self.records):
                return self._fail(f"ROLLBACK_DETECTED log_len={len(self.records)} accepted_seq={seq}")
            if seq > 0 and self.records[seq - 1]["hash"] != head:
                return self._fail(f"ROLLBACK_DETECTED fork_at_seq={seq}")
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

    def _fail(self, reason: str) -> None:
        self.failed = reason
        log("witness_failed", reason=reason)

    def _replay(self, rec: dict) -> str | None:
        t = rec.get("type")
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

    # ---- append -------------------------------------------------------
    def _append(self, rec: dict, req: dict) -> dict:
        rec["seq"] = len(self.records) + 1
        rec["prev_hash"] = self.records[-1]["hash"] if self.records else GENESIS
        rec["ts"] = time.time()
        rec["request"] = req
        rec["hash"] = sha256_hex(canon(rec))
        err = self._replay(rec)
        if err:
            raise RuntimeError(err)
        self.fh.write(canon(rec) + b"\n")
        self.fh.flush()
        os.fsync(self.fh.fileno())
        self.records.append(rec)
        if self.fault == "crash_after_append_before_reply":
            log("fault_crash", seq=rec["seq"])
            os._exit(137)
        return rec

    # ---- request handling --------------------------------------------
    def handle(self, req: dict) -> dict:
        op = req.get("op")
        principal = req.get("principal")
        if self.failed and op not in ("health", "shutdown"):
            return {"status": "WITNESS_FAILED", "reason": self.failed}
        key = self.keys.get(principal)
        signed = {k: req.get(k) for k in ("op", "principal", "nonce", "body")}
        if key is None or not verify(key, signed, req.get("mac")):
            return {"status": "AUTH_FAILED"}
        if req.get("nonce") in self.nonces:
            return {"status": "NONCE_REPLAYED"}
        self.nonces.add(req.get("nonce"))
        role = ROLE[principal]
        body = req.get("body") or {}
        fn = getattr(self, "op_" + str(op), None)
        if fn is None:
            return {"status": "UNSUPPORTED_OP", "op": op}
        return fn(role, principal, body, req)

    def op_health(self, role, principal, body, req):
        return {"status": "FAILED" if self.failed else "OK", "reason": self.failed,
                "records": len(self.records), "checkpoint": self.checkpoint}

    def op_export(self, role, principal, body, req):
        return {"status": "OK", "records": self.records, "checkpoint": self.checkpoint,
                "epochs": self.epochs,
                "keys": {f"{k[0]}|{k[1]}": {"state": v["state"], "ticket_id": v["reservation"]["ticket_id"],
                                            "payload_hash": v["reservation"]["payload_hash"],
                                            "principal": v["reservation"]["principal"],
                                            "epoch": v["reservation"]["epoch"],
                                            "incarnation": v["reservation"]["incarnation"],
                                            "history": v["history"]}
                         for k, v in self.keys_state.items()}}

    def op_epoch_set(self, role, principal, body, req):
        if role != "operator":
            return {"status": "ROLE_FORBIDDEN"}
        ns, epoch, holder = body.get("ns"), body.get("epoch"), body.get("holder")
        cur = self.epochs.get(ns)
        if ROLE.get(holder) != "issuer" or not isinstance(epoch, int):
            return {"status": "BAD_REQUEST"}
        if cur and epoch <= cur["epoch"]:
            return {"status": "EPOCH_NOT_MONOTONIC", "current": cur}
        rec = self._append({"type": "EPOCH", "ns": ns, "epoch": epoch, "holder": holder}, req)
        return {"status": "EPOCH_SET", "seq": rec["seq"]}

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
        rec = self._append({"type": "RESERVE", "ns": ns, "request_id": rid, "payload_hash": ph, "ticket_id": tid,
                            "principal": principal, "epoch": ep["epoch"], "incarnation": body.get("incarnation")}, req)
        return {"status": "RESERVED", "seq": rec["seq"], "epoch": ep["epoch"]}

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
        rec = self._append({"type": "OUTCOME", "ns": ns, "request_id": rid, "ticket_id": tid, "outcome": target,
                            "evidence": body.get("evidence"), "principal": principal}, req)
        return {"status": "APPENDED", "seq": rec["seq"], "state": target}

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
        self.fault = body.get("fault")
        return {"status": "FAULT_ARMED", "name": self.fault}

    def op_shutdown(self, role, principal, body, req):
        if ROLE.get(principal) != "fault":
            return {"status": "ROLE_FORBIDDEN"}
        return {"status": "SHUTDOWN"}


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--store", required=True)
    ap.add_argument("--checkpoint-dir", required=True)
    ap.add_argument("--pipe", required=True)
    args = ap.parse_args()
    keys = json.loads(os.environ["WITNESS_KEYS_JSON"])
    authkey = bytes.fromhex(os.environ["WITNESS_AUTHKEY"])
    os.makedirs(args.store, exist_ok=True)
    os.makedirs(args.checkpoint_dir, exist_ok=True)
    try:
        lock = StoreLock(os.path.join(args.store, "store.lock"))
    except OSError:
        log("store_locked", store=os.path.basename(args.store))
        return 3
    w = Witness(args.store, args.checkpoint_dir, keys)
    address, family = pipe_address(args.pipe)
    listener = Listener(address, family=family, authkey=authkey)
    log("ready", pid=os.getpid(), records=len(w.records), failed=w.failed, checkpoint=w.checkpoint)

    def serve(conn) -> None:
        try:
            req = conn.recv()
            with w.lock:
                try:
                    resp = w.handle(req)
                except Exception as exc:  # reported, never hidden
                    resp = {"status": "INTERNAL_ERROR", "error": f"{type(exc).__name__}: {exc}"}
                log("request", op=req.get("op"), principal=req.get("principal"),
                    body_ns=(req.get("body") or {}).get("ns"), body_request_id=(req.get("body") or {}).get("request_id"),
                    status=resp.get("status"), seq=resp.get("seq"))
                conn.send(resp)
                if resp.get("status") == "SHUTDOWN":
                    w.fh.close()
                    lock.f.close()
                    log("stopped")
                    os._exit(0)
        except (EOFError, OSError):
            pass
        finally:
            conn.close()

    while True:
        try:
            conn = listener.accept()
        except AuthenticationError:
            log("pipe_auth_failed")
            continue
        except OSError:
            break
        threading.Thread(target=serve, args=(conn,), daemon=True).start()
    return 0


if __name__ == "__main__":
    sys.exit(main())
