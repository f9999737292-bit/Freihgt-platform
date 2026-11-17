#!/usr/bin/env python3
"""BINTRANS 0.1S test-only issuer process (one request per process).

0.1P Transaction A order with two separate, non-atomic commits: witness RESERVE,
then PostgreSQL BEGIN/INSERT/COMMIT, then witness OUTCOME, then the response.

Added in 0.1S: before any reservation the issuer reads the high-water mark
itself (reader credential) and requires the exported journal to be a valid
chain whose incarnation, length, head digest and every pinned digest equal the
high-water history. Otherwise history completeness is not proven and issuance
is denied. A reserve or outcome answered with HIGH_WATER_UNAVAILABLE is an
unknown outcome, never a success.

Fault points end the process with os._exit(137). The process holds only its
own witness key and its own high-water reader key.

Environment: WORKER_CONFIG (JSON, no secrets), WITNESS_AUTHKEY, WITNESS_KEY,
HW_AUTHKEY, HW_KEY. Output: "EVENT {json}" lines and one "RESULT {json}" line.
"""

from __future__ import annotations

import json
import os
import sys
import time
import uuid

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from common import WitnessClient, WitnessUnavailable, payload_hash, verify_chain  # noqa: E402
from pgsession import PgSession, pg_query  # noqa: E402

CFG = json.loads(os.environ["WORKER_CONFIG"])
T0 = time.monotonic()


def event(name: str, **kw) -> None:
    print("EVENT " + json.dumps({"mono": round(time.monotonic() - T0, 6), "t": time.time(), "event": name, **kw},
                                sort_keys=True), flush=True)


def result(code: str, **kw) -> None:
    print("RESULT " + json.dumps({"result": code, **kw}, sort_keys=True), flush=True)
    sys.stdout.flush()
    sys.exit(0)


def crash(point: str) -> None:
    event("fault_crash", point=point)
    sys.stdout.flush()
    os._exit(137)


def q(s: str) -> str:
    return s.replace("'", "''")


def pg_rows(ns: str) -> list[dict]:
    r = pg_query(CFG["container"], CFG["pg_role"], CFG["db"],
                 "SELECT coalesce(jsonb_agg(jsonb_build_object('ticket_id', ticket_id, 'request_id', request_id, "
                 "'payload_hash', payload_hash, 'status', status)), '[]'::jsonb)::text FROM tickets "
                 f"WHERE issuer_namespace_id = '{q(ns)}'", CFG["app"] + "-read")
    event("pg_read_rows", exit=r["exit"], sqlstates=r["sqlstates"])
    if r["exit"] != 0:
        result("ISSUE_DENIED_PG_UNREADABLE", stderr=r["stderr"])
    return json.loads(r["stdout"].strip() or "[]")


def gate_reasons(exp: dict, rows: list[dict], ns: str) -> list[str]:
    reasons = []
    ep = exp["epochs"].get(ns)
    if not ep or ep["holder"] != CFG["principal"] or ep["epoch"] != CFG["epoch"]:
        reasons.append("NOT_CURRENT_EPOCH_HOLDER")
    keys = {k.split("|", 1)[1]: v for k, v in exp["keys"].items() if k.split("|", 1)[0] == ns}
    open_ = sorted(rid for rid, v in keys.items() if v["state"] in ("RESERVED", "IN_DOUBT"))
    if open_:
        reasons.append("OPEN_RESERVATIONS:" + ",".join(open_))
    by_rid = {r["request_id"]: r for r in rows}
    for rid, v in sorted(keys.items()):
        row = by_rid.get(rid)
        if v["state"] == "COMMIT_CONFIRMED" and (row is None or row["ticket_id"] != v["ticket_id"]
                                                 or row["payload_hash"] != v["payload_hash"]):
            reasons.append("CONFIRMED_TICKET_MISSING_OR_DIFFERENT_IN_PG:" + rid)
        if v["state"] in ("PERMANENTLY_DENIED", "ABORT_CONFIRMED") and row is not None:
            reasons.append("TICKET_IN_PG_FOR_UNCONFIRMED_RESERVATION:" + rid)
    for rid in sorted(by_rid):
        if rid not in keys:
            reasons.append("TICKET_IN_PG_WITHOUT_RESERVATION:" + rid)
    return reasons


def call(client: WitnessClient, op: str, **body) -> dict:
    try:
        r = client.call(op, **body)
    except WitnessUnavailable as exc:
        event("witness_unavailable", op=op, error=str(exc))
        raise
    event("witness_" + op, status=r.get("status"), seq=r.get("seq"), reason=r.get("reason"),
          durability=r.get("durability"))
    return r


def export_verified(client: WitnessClient, hw: WitnessClient) -> dict:
    try:
        exp = call(client, "export")
    except WitnessUnavailable as exc:
        result("ISSUE_DENIED_WITNESS_UNAVAILABLE", error=str(exc))
    if exp.get("status") == "HIGH_WATER_UNAVAILABLE":
        result("ISSUE_DENIED_HIGH_WATER_UNAVAILABLE", witness=exp.get("status"), reason=exp.get("reason"))
    if exp.get("status") != "OK":
        result("ISSUE_DENIED_WITNESS_FAILED", witness=exp.get("status"), reason=exp.get("reason"))
    try:
        hr = hw.call("get", journal_id=CFG["journal_id"])
    except WitnessUnavailable as exc:
        event("high_water_unavailable", error=str(exc))
        result("ISSUE_DENIED_HIGH_WATER_UNAVAILABLE", error=str(exc))
    if hr.get("status") != "OK":
        event("high_water_read", status=hr.get("status"), reason=hr.get("reason"))
        result("ISSUE_DENIED_HIGH_WATER_UNAVAILABLE", high_water=hr.get("status"), reason=hr.get("reason"))
    m = hr["mark"]
    recs = exp["records"]
    problem = verify_chain(recs)
    if problem is None and exp.get("journal_incarnation") != m["journal_incarnation"]:
        problem = "INCARNATION_MISMATCH"
    if problem is None and len(recs) != m["seq"]:
        problem = f"LENGTH_MISMATCH journal_len={len(recs)} high_water_seq={m['seq']}"
    if problem is None:
        for s, d in m["history"]:
            if s >= 1 and recs[s - 1]["hash"] != d:
                problem = f"PINNED_DIGEST_MISMATCH seq={s}"
                break
    event("history_check", complete=problem is None, problem=problem, journal_len=len(recs), high_water_seq=m["seq"])
    if problem:
        result("ISSUE_DENIED_HISTORY_UNVERIFIED", problem=problem)
    return exp


def outcome(client: WitnessClient, ns: str, rid: str, tid: str, target: str, evidence: dict, code: str) -> None:
    try:
        r = call(client, "outcome", ns=ns, request_id=rid, ticket_id=tid, outcome=target, evidence=evidence)
    except WitnessUnavailable as exc:
        result("ISSUE_OUTCOME_UNKNOWN", wanted=target, error=str(exc))
    if r.get("status") not in ("APPENDED", "IDEMPOTENT"):
        result("ISSUE_OUTCOME_UNKNOWN", wanted=target, witness=r.get("status"), durability=r.get("durability"))
    result(code, witness=r.get("status"))


def pg_transaction(ns: str, rid: str, ph: str, tid: str, seq: int, fault: str, client: WitnessClient) -> None:
    s = PgSession(CFG["container"], CFG["pg_role"], CFG["db"], CFG["app"] + "-txn")
    event("pg_begin_sent")
    st = s.exec("BEGIN;\nSELECT 'PID=' || pg_backend_pid();")
    pid = next((l.split("PID=")[1].strip() for l in st["stdout"].splitlines() if "PID=" in l), None)
    event("pg_begin", ok=st["ok"], backend_pid=pid)
    st = s.exec(f"INSERT INTO tickets (ticket_id, issuer_namespace_id, request_id, payload_hash, witness_seq, issued_by, "
                f"issued_epoch) VALUES ('{tid}', '{q(ns)}', '{q(rid)}', '{ph}', {seq}, '{CFG['principal']}', {CFG['epoch']});")
    event("pg_insert", ok=st["ok"], sqlstates=st["sqlstates"], stderr=st["stderr"])
    if not st["ok"]:
        rb = s.exec("ROLLBACK;")
        ack = rb["done"] and "ROLLBACK" in rb["stdout"]
        event("pg_rollback", ack=ack)
        s.close()
        if ack:
            outcome(client, ns, rid, tid, "ABORT_CONFIRMED", {"pg_rollback_ack": True, "insert_sqlstates": st["sqlstates"]},
                    "ISSUE_DENIED_PG_CONFLICT")
        outcome(client, ns, rid, tid, "IN_DOUBT", {"reason": "rollback not acknowledged"}, "ISSUE_IN_DOUBT")
    if fault == "crash_before_commit":
        crash("before_pg_commit")
    st = s.exec("COMMIT;")
    ack = st["ok"] and st["stdout"].strip().endswith("COMMIT")
    event("pg_commit", ok=st["ok"], ack=ack, sqlstates=st["sqlstates"])
    s.close()
    if not ack:
        outcome(client, ns, rid, tid, "IN_DOUBT", {"reason": "COMMIT not acknowledged"}, "ISSUE_IN_DOUBT")
    if fault == "crash_after_commit":
        crash("after_pg_commit_before_outcome")


def main() -> None:
    client = WitnessClient(CFG["pipe"], os.environ["WITNESS_AUTHKEY"], CFG["principal"], os.environ["WITNESS_KEY"])
    hw = WitnessClient(CFG["hw_pipe"], os.environ["HW_AUTHKEY"], CFG["principal"], os.environ["HW_KEY"])
    ns, rid, fault = CFG["ns"], CFG["request_id"], CFG.get("fault") or ""
    ph = payload_hash(ns, rid, CFG["payload"])
    event("start", principal=CFG["principal"], epoch=CFG["epoch"], incarnation=CFG["incarnation"], ns=ns,
          request_id=rid, payload_hash=ph, fault=fault, mode=CFG.get("mode", "issue"))

    if CFG.get("mode") == "reserve_only":
        tid = str(uuid.uuid4())
        while time.time() < CFG["start_at"]:
            time.sleep(0.001)
        try:
            r = client.call("reserve", ns=ns, request_id=rid, payload_hash=ph, ticket_id=tid, epoch=CFG["epoch"],
                            incarnation=CFG["incarnation"])
        except WitnessUnavailable as exc:
            result("WITNESS_UNAVAILABLE", error=str(exc))
        event("witness_reserve", status=r.get("status"), seq=r.get("seq"))
        result(r.get("status"), seq=r.get("seq"), ticket_id=tid)

    exp = export_verified(client, hw)

    # Restart recovery: own reservations left RESERVED by an earlier incarnation become IN_DOUBT.
    marked = []
    for k, v in sorted(exp["keys"].items()):
        if v["principal"] == CFG["principal"] and v["state"] == "RESERVED" and v["incarnation"] != CFG["incarnation"]:
            kns, krid = k.split("|", 1)
            try:
                r = call(client, "outcome", ns=kns, request_id=krid, ticket_id=v["ticket_id"], outcome="IN_DOUBT",
                         evidence={"reason": f"restart: incarnation {v['incarnation']} ended without outcome"})
            except WitnessUnavailable as exc:
                result("ISSUE_DENIED_WITNESS_UNAVAILABLE", error=str(exc))
            if r.get("status") not in ("APPENDED", "IDEMPOTENT"):
                result("ISSUE_DENIED_RECOVERY_INCOMPLETE", key=k, witness=r.get("status"))
            marked.append({"key": k, "status": r.get("status")})
    if marked:
        event("restart_recovery", marked_in_doubt=marked)
        exp = export_verified(client, hw)

    cur = exp["keys"].get(f"{ns}|{rid}")
    resume = None
    if cur is not None:
        event("existing_reservation", state=cur["state"], reservation_ticket_id=cur["ticket_id"])
        if cur["payload_hash"] != ph:
            result("ISSUE_DENIED_PAYLOAD_MISMATCH", state=cur["state"])
        if cur["state"] == "COMMIT_CONFIRMED":
            rows = {r["request_id"]: r for r in pg_rows(ns)}
            row = rows.get(rid)
            if row and row["ticket_id"] == cur["ticket_id"] and row["payload_hash"] == ph and row["status"] == "ISSUED":
                result("ISSUE_REPLAY_ISSUED", ticket_id=cur["ticket_id"])
            result("ISSUE_DENIED_RECONCILIATION_REQUIRED", state=cur["state"], pg_row=row)
        if cur["state"] in ("RESERVED", "IN_DOUBT"):
            result("ISSUE_DENIED_IN_DOUBT", state=cur["state"])
        if cur["state"] == "PERMANENTLY_DENIED":
            result("ISSUE_DENIED_PERMANENTLY", state=cur["state"])
        if cur["state"] == "ABORT_CONFIRMED":
            if cur["principal"] != CFG["principal"]:
                result("ISSUE_DENIED_NOT_RESERVATION_OWNER", state=cur["state"])
            resume = cur

    reasons = gate_reasons(exp, pg_rows(ns), ns)
    event("gate", open=not reasons, reasons=reasons)
    if reasons:
        result("ISSUE_DENIED_GATE_CLOSED", reasons=reasons)

    if fault == "crash_before_reserve":
        crash("before_reservation")
    if resume is not None:
        tid = resume["ticket_id"]
        seq = resume["history"][0]
        event("resume_reservation", ticket_id=tid, reservation_seq=seq)
    else:
        tid = str(uuid.uuid4())
        try:
            r = call(client, "reserve", ns=ns, request_id=rid, payload_hash=ph, ticket_id=tid, epoch=CFG["epoch"],
                     incarnation=CFG["incarnation"])
        except WitnessUnavailable as exc:
            result("ISSUE_WITNESS_OUTCOME_UNKNOWN", stage="reserve", error=str(exc))
        if r.get("status") == "HIGH_WATER_UNAVAILABLE":
            result("ISSUE_WITNESS_OUTCOME_UNKNOWN", stage="reserve", witness=r.get("status"),
                   durability=r.get("durability"))
        if r.get("status") == "WITNESS_FAILED":
            result("ISSUE_DENIED_WITNESS_FAILED", witness=r.get("status"), reason=r.get("reason"))
        if r.get("status") != "RESERVED":
            result("ISSUE_DENIED_RESERVATION", witness=r.get("status"), state=r.get("state"))
        seq = r["seq"]
        if fault == "crash_after_reserve":
            crash("after_reservation_before_pg")

    pg_transaction(ns, rid, ph, tid, seq, fault, client)

    try:
        r = call(client, "outcome", ns=ns, request_id=rid, ticket_id=tid, outcome="COMMIT_CONFIRMED",
                 evidence={"pg_commit_ack": True})
    except WitnessUnavailable as exc:
        result("ISSUE_OUTCOME_UNKNOWN", wanted="COMMIT_CONFIRMED", error=str(exc))
    if r.get("status") not in ("APPENDED", "IDEMPOTENT"):
        result("ISSUE_OUTCOME_UNKNOWN", wanted="COMMIT_CONFIRMED", witness=r.get("status"), durability=r.get("durability"))
    result("ISSUE_OK", ticket_id=tid, resumed=resume is not None)


if __name__ == "__main__":
    main()
