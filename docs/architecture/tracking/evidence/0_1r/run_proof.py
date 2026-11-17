#!/usr/bin/env python3
"""BINTRANS 0.1R isolated external-witness failure proof (orchestrator).

Per attempt: one disposable PostgreSQL 16 container (--network none, no
published ports) and several test-only witness server processes, each with its
own store under a fresh temporary directory outside the repository. Issuer
requests run in separate processes (issuer_worker.py) with fault injection.
Only resources created here are removed. Standard library only.

All stores live on the same machine, so nothing here proves an independent
failure domain.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import pathlib
import re
import secrets
import shutil
import subprocess
import sys
import tempfile
import threading
import time
import uuid

HERE = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
from common import WitnessClient, WitnessUnavailable, payload_hash  # noqa: E402
from pgsession import pg_query  # noqa: E402

IMAGE = "postgres:16"
LABEL = "bintrans.proof=0.1r"
ADMIN = "wproof_admin"
ISSUER_ROLE = "wproof_issuer"
PRINCIPALS = ("issuer_a", "issuer_b", "operator", "auditor", "harness")

# Independent statement of the 0.1P transition table (role, from, to) -> allowed.
EXPECTED_ALLOWED = {
    ("issuer", "RESERVED", "COMMIT_CONFIRMED"), ("issuer", "RESERVED", "ABORT_CONFIRMED"),
    ("issuer", "RESERVED", "IN_DOUBT"), ("issuer", "ABORT_CONFIRMED", "COMMIT_CONFIRMED"),
    ("operator", "RESERVED", "IN_DOUBT"), ("operator", "IN_DOUBT", "COMMIT_CONFIRMED"),
    ("operator", "IN_DOUBT", "PERMANENTLY_DENIED"), ("operator", "ABORT_CONFIRMED", "PERMANENTLY_DENIED"),
}
STATES = ("RESERVED", "COMMIT_CONFIRMED", "ABORT_CONFIRMED", "IN_DOUBT", "PERMANENTLY_DENIED")
TARGETS = ("COMMIT_CONFIRMED", "ABORT_CONFIRMED", "IN_DOUBT", "PERMANENTLY_DENIED")


def now_iso() -> str:
    return dt.datetime.now(dt.timezone.utc).isoformat(timespec="microseconds")


def payload(rid: str, revision: int = 1) -> dict:
    return {"operation": "ISSUE_TRACKING_BINDING", "sub": "user-0.1r-test", "tenant_id": "tenant-0.1r-test",
            "shipment": f"shp-{rid}", "execution_ids": ["exe-2", "exe-1"], "expected_epoch": 1,
            "expected_revision": revision}


class Attempt:
    def __init__(self, index: int, out: pathlib.Path) -> None:
        self.index = index
        self.dir = out / f"attempt_{index}"
        self.dir.mkdir(parents=True, exist_ok=False)
        hx = secrets.token_hex(4)
        self.hex = hx
        self.container = f"bintrans-proof-0-1r-{hx}"
        self.db = f"wproof_{hx}"
        self.db_restore = f"wproof_r_{hx}"
        self.tmp = pathlib.Path(tempfile.mkdtemp(prefix=f"bintrans-0.1r-{hx}-"))
        self.keys = {p: secrets.token_hex(32) for p in PRINCIPALS}
        self.authkey = secrets.token_hex(32)
        self.pg_password = secrets.token_urlsafe(24)
        self.secrets = list(self.keys.values()) + [self.authkey, self.pg_password]
        self.assertions: list[dict] = []
        self.traces: dict[str, list] = {}
        self.witnesses: dict[str, dict] = {}
        self.worker_no = 0
        self.findings: list[dict] = []
        self.stop_scope = False
        self.meta: dict = {"attempt": index, "container": self.container, "db": self.db, "db_restore": self.db_restore,
                           "image": IMAGE, "network": "none", "published_ports": "none",
                           "witness_tmp_root": "<per-attempt temp dir outside repository>",
                           "host_os": sys.platform, "python": sys.version.split()[0]}

    def redact(self, text: str) -> str:
        for s in self.secrets:
            text = text.replace(s, "<REDACTED>")
        return text

    def write(self, rel: str, text: str) -> None:
        p = self.dir / rel
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(self.redact(text), encoding="utf-8", newline="\n")

    def trace(self, scenario: str, kind: str, **kw) -> dict:
        rec = {"t": now_iso(), "kind": kind, **kw}
        self.traces.setdefault(scenario, []).append(rec)
        return rec

    def check(self, aid: str, scenario: str, kind: str, description: str, cond: bool, observed) -> bool:
        self.assertions.append({"id": aid, "scenario": scenario, "kind": kind, "description": description,
                                "observed": observed, "result": "PASS" if cond else "FAIL"})
        return cond

    def finding(self, fid: str, text: str, evidence) -> None:
        self.findings.append({"id": fid, "text": text, "evidence": evidence})

    def env(self, **extra) -> dict:
        env = {k: v for k, v in os.environ.items()
               if not k.startswith("PG") and k not in ("DATABASE_URL", "CONTROL_TOWER_DATABASE_URL")}
        env.update(extra)
        return env


# --------------------------------------------------------------------------
# PostgreSQL container
# --------------------------------------------------------------------------

def run(att: Attempt, cmd: list[str], save: str | None = None, stdin: str | None = None, env: dict | None = None,
        timeout: int = 180) -> dict:
    p = subprocess.run(cmd, input=stdin.encode() if stdin is not None else None, capture_output=True,
                       env=env or att.env(), timeout=timeout)
    out = p.stdout.decode("utf-8", "replace").replace("\r\n", "\n")
    err = p.stderr.decode("utf-8", "replace").replace("\r\n", "\n")
    if save:
        att.write(f"{save}.cmd.txt", " ".join(cmd) + "\n")
        att.write(f"{save}.stdout.txt", out)
        att.write(f"{save}.stderr.txt", err)
        att.write(f"{save}.exit", f"{p.returncode}\n")
    return {"exit": p.returncode, "stdout": out, "stderr": err}


def admin_sql(att: Attempt, db: str, sql: str, save: str | None = None) -> dict:
    r = pg_query(att.container, ADMIN, db, sql, "harness-admin")
    if save:
        att.write(f"{save}.sql", sql + "\n")
        att.write(f"{save}.stdout.txt", r["stdout"])
        att.write(f"{save}.stderr.txt", r["stderr"])
        att.write(f"{save}.exit", f"{r['exit']}\n")
    return r


def start_pg(att: Attempt) -> None:
    prefix = "%m [%p] sqlstate=%e user=%u db=%d app=%a "
    cmd = ["docker", "run", "-d", "--pull", "never", "--name", att.container, "--label", LABEL,
           "--label", f"bintrans.proof.attempt={att.index}", "--network", "none",
           "-e", f"POSTGRES_USER={ADMIN}", "-e", "POSTGRES_PASSWORD", "-e", f"POSTGRES_DB={att.db}",
           IMAGE, "postgres", "-c", "log_connections=on", "-c", "log_disconnections=on",
           "-c", f"log_line_prefix={prefix}"]
    r = run(att, cmd, "setup/00_docker_run", env=att.env(POSTGRES_PASSWORD=att.pg_password))
    if r["exit"] != 0:
        raise RuntimeError("docker run failed")
    deadline = time.monotonic() + 90
    while time.monotonic() < deadline:
        logs = run(att, ["docker", "logs", att.container])
        ready = run(att, ["docker", "exec", att.container, "pg_isready", "-h", "127.0.0.1", "-U", ADMIN])
        if "PostgreSQL init process complete" in logs["stdout"] + logs["stderr"] and ready["exit"] == 0:
            break
        time.sleep(1)
    else:
        raise RuntimeError("postgres not ready")
    insp = run(att, ["docker", "inspect", att.container, "--format",
                     "{{.HostConfig.NetworkMode}}|{{json .HostConfig.PortBindings}}|{{.Image}}"], "setup/01_inspect")
    parts = insp["stdout"].strip().split("|")
    att.meta.update({"network_mode": parts[0], "port_bindings": parts[1], "image_id": parts[2]})
    admin_sql(att, att.db, f"CREATE ROLE {ISSUER_ROLE} LOGIN;", "setup/02_role")
    admin_sql(att, att.db, f'CREATE DATABASE "{att.db_restore}";', "setup/03_restore_db")
    schema = (HERE / "sql" / "schema.sql").read_text(encoding="utf-8")
    for i, db in enumerate((att.db, att.db_restore)):
        r = run(att, ["docker", "exec", "-i", att.container, "psql", "-X", "-v", "ON_ERROR_STOP=1", "-U", ADMIN,
                      "-d", db, "-f", "-"], f"setup/04_schema_{i}", stdin=schema)
        if r["exit"] != 0:
            raise RuntimeError("schema failed")
    v = admin_sql(att, att.db, "SELECT version();", "setup/05_version")
    att.meta["server_version"] = v["stdout"].strip()


def stop_pg(att: Attempt) -> None:
    logs = run(att, ["docker", "logs", "-t", att.container])
    att.write("pg_server_log.txt", logs["stdout"] + logs["stderr"])
    run(att, ["docker", "rm", "-f", att.container], "setup/99_docker_rm")
    left = run(att, ["docker", "ps", "-a", "--filter", f"name=^{att.container}$", "--format", "{{.Names}}"])
    att.meta["container_removed"] = left["stdout"].strip() == ""


def pg_rows(att: Attempt, ns: str, db: str | None = None) -> list[dict]:
    r = admin_sql(att, db or att.db,
                  "SELECT coalesce(jsonb_agg(jsonb_build_object('ticket_id', ticket_id, 'request_id', request_id, "
                  "'payload_hash', payload_hash, 'witness_seq', witness_seq, 'issued_by', issued_by) "
                  f"ORDER BY request_id), '[]'::jsonb)::text FROM tickets WHERE issuer_namespace_id = '{ns}'")
    return json.loads(r["stdout"].strip() or "[]")


def txn_connections(att: Attempt, app: str) -> int:
    logs = run(att, ["docker", "logs", att.container])
    text = logs["stdout"] + logs["stderr"]
    return len(re.findall(rf"connection authorized: .*application_name={re.escape(app)}-txn\b", text))


def any_connections(att: Attempt, app: str) -> int:
    logs = run(att, ["docker", "logs", att.container])
    text = logs["stdout"] + logs["stderr"]
    return len(re.findall(rf"connection authorized: .*application_name={re.escape(app)}-", text))


# --------------------------------------------------------------------------
# Witness servers
# --------------------------------------------------------------------------

def start_witness(att: Attempt, name: str, expect_ready: bool = True) -> dict:
    w = att.witnesses.setdefault(name, {"runs": 0, "pipe": f"bintrans-0-1r-{att.hex}-{name}",
                                        "store": att.tmp / name / "store", "cp": att.tmp / name / "checkpoint"})
    w["runs"] += 1
    run_no = w["runs"]
    env = att.env(WITNESS_KEYS_JSON=json.dumps(att.keys), WITNESS_AUTHKEY=att.authkey)
    proc = subprocess.Popen([sys.executable, str(HERE / "witness_server.py"), "--store", str(w["store"]),
                             "--checkpoint-dir", str(w["cp"]), "--pipe", w["pipe"]],
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env)
    lines: list[str] = []
    errs: list[str] = []
    ready = threading.Event()

    def rd(stream, sink, flag):
        for raw in iter(stream.readline, b""):
            line = raw.decode("utf-8", "replace").rstrip("\r\n")
            sink.append(line)
            if flag and '"event": "ready"' in line:
                ready.set()
        if flag:
            ready.set()

    threading.Thread(target=rd, args=(proc.stdout, lines, True), daemon=True).start()
    threading.Thread(target=rd, args=(proc.stderr, errs, False), daemon=True).start()
    ready.wait(20)
    handle = {"proc": proc, "stdout": lines, "stderr": errs, "run": run_no}
    w["current"] = handle
    w.setdefault("handles", []).append(handle)
    if expect_ready:
        time.sleep(0.1)
    return handle


def stop_witness(att: Attempt, name: str, how: str = "shutdown") -> int | None:
    w = att.witnesses[name]
    h = w["current"]
    if h["proc"].poll() is None:
        if how == "shutdown":
            try:
                client(att, name, "harness").call("shutdown")
            except WitnessUnavailable:
                pass
        else:
            h["proc"].kill()
        try:
            h["proc"].wait(15)
        except subprocess.TimeoutExpired:
            h["proc"].kill()
            h["proc"].wait()
    time.sleep(0.2)
    return h["proc"].returncode


def save_witness(att: Attempt, name: str) -> None:
    w = att.witnesses[name]
    for h in w.get("handles", []):
        if h["proc"].poll() is None:
            h["proc"].kill()
            h["proc"].wait()
        time.sleep(0.1)
        att.write(f"witness/{name}/server_run{h['run']}.stdout.txt", "\n".join(h["stdout"]) + "\n")
        att.write(f"witness/{name}/server_run{h['run']}.stderr.txt", "\n".join(h["stderr"]) + "\n")
        att.write(f"witness/{name}/server_run{h['run']}.exit", f"{h['proc'].returncode}\n")
    logp = w["store"] / "witness_log.jsonl"
    if logp.exists():
        att.write(f"witness/{name}/witness_log.jsonl", logp.read_text(encoding="utf-8"))
    cpp = w["cp"] / "accepted_checkpoint.json"
    if cpp.exists():
        att.write(f"witness/{name}/accepted_checkpoint.json", cpp.read_text(encoding="utf-8"))


def client(att: Attempt, name: str, principal: str, key: str | None = None) -> WitnessClient:
    return WitnessClient(att.witnesses[name]["pipe"], att.authkey, principal, key or att.keys[principal])


def wcall(att: Attempt, scenario: str, name: str, principal: str, op: str, key: str | None = None, **body) -> dict:
    try:
        r = client(att, name, principal, key).call(op, **body)
    except WitnessUnavailable as exc:
        r = {"status": "UNAVAILABLE", "error": str(exc)}
    att.trace(scenario, "witness_call", witness=name, principal=principal, op=op, body=body,
              response={k: v for k, v in r.items() if k not in ("records",)})
    return r


def export(att: Attempt, name: str) -> dict:
    try:
        return client(att, name, "auditor").call("export")
    except WitnessUnavailable as exc:
        return {"status": "UNAVAILABLE", "error": str(exc)}


def key_state(att: Attempt, name: str, ns: str, rid: str) -> str | None:
    e = export(att, name)
    k = (e.get("keys") or {}).get(f"{ns}|{rid}")
    return k["state"] if k else None


def reserve_records(att: Attempt, name: str, ns: str, rid: str) -> int:
    e = export(att, name)
    return sum(1 for r in e.get("records", []) if r["type"] == "RESERVE" and r["ns"] == ns and r["request_id"] == rid)


def record_count(att: Attempt, name: str) -> int:
    return len(export(att, name).get("records", []))


# --------------------------------------------------------------------------
# Issuer workers
# --------------------------------------------------------------------------

def worker_cfg(att: Attempt, name: str, principal: str, ns: str, rid: str, **kw) -> dict:
    cfg = {"pipe": att.witnesses[name]["pipe"], "principal": principal, "epoch": kw.pop("epoch", 1),
           "incarnation": kw.pop("incarnation", str(uuid.uuid4())), "ns": ns, "request_id": rid,
           "payload": kw.pop("payload", payload(rid)), "fault": kw.pop("fault", ""), "container": att.container,
           "db": kw.pop("db", att.db), "pg_role": ISSUER_ROLE, "mode": kw.pop("mode", "issue")}
    cfg.update(kw)
    return cfg


def spawn(att: Attempt, label: str, cfg: dict) -> dict:
    att.worker_no += 1
    tag = f"w{att.worker_no:03d}-{label}"
    cfg.setdefault("app", f"a{att.index}-{tag}")
    env = att.env(WORKER_CONFIG=json.dumps(cfg), WITNESS_AUTHKEY=att.authkey, WITNESS_KEY=att.keys[cfg["principal"]])
    proc = subprocess.Popen([sys.executable, str(HERE / "issuer_worker.py")], stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, env=env)
    h = {"tag": tag, "cfg": cfg, "proc": proc, "out": [], "err": [], "t_start": now_iso()}

    def rd(stream, sink):
        for raw in iter(stream.readline, b""):
            sink.append(raw.decode("utf-8", "replace").rstrip("\r\n"))

    h["threads"] = [threading.Thread(target=rd, args=(proc.stdout, h["out"]), daemon=True),
                    threading.Thread(target=rd, args=(proc.stderr, h["err"]), daemon=True)]
    for t in h["threads"]:
        t.start()
    return h


def finish(att: Attempt, scenario: str, h: dict, timeout: int = 60) -> dict:
    try:
        h["proc"].wait(timeout)
    except subprocess.TimeoutExpired:
        h["proc"].kill()
        h["proc"].wait()
    for t in h["threads"]:
        t.join(5)
    events = [json.loads(l[6:]) for l in h["out"] if l.startswith("EVENT ")]
    res = next((json.loads(l[7:]) for l in h["out"] if l.startswith("RESULT ")), None)
    base = f"workers/{h['tag']}"
    att.write(f"{base}.config.json", json.dumps(h["cfg"], indent=2, sort_keys=True) + "\n")
    att.write(f"{base}.stdout.txt", "\n".join(h["out"]) + "\n")
    att.write(f"{base}.stderr.txt", "\n".join(h["err"]) + "\n")
    att.write(f"{base}.exit", f"{h['proc'].returncode}\n")
    summary = {"tag": h["tag"], "app": h["cfg"]["app"], "principal": h["cfg"]["principal"], "ns": h["cfg"]["ns"],
               "request_id": h["cfg"]["request_id"], "fault": h["cfg"].get("fault"), "exit": h["proc"].returncode,
               "result": res, "events": [e["event"] for e in events], "t_start": h["t_start"], "t_end": now_iso()}
    att.trace(scenario, "worker", **summary)
    summary["_events"] = events
    return summary


def issue(att: Attempt, scenario: str, label: str, name: str, principal: str, ns: str, rid: str, **kw) -> dict:
    return finish(att, scenario, spawn(att, label, worker_cfg(att, name, principal, ns, rid, **kw)))


def rcode(s: dict) -> str | None:
    return (s.get("result") or {}).get("result")


def ev_index(s: dict, name: str, **match) -> int:
    for i, e in enumerate(s["_events"]):
        if e["event"] == name and all(e.get(k) == v for k, v in match.items()):
            return i
    return -1


def reconcile(att: Attempt, scenario: str, name: str, ns: str, rid: str, db: str | None = None) -> dict:
    """Operator reconciliation for one IN_DOUBT key. Never writes ABORT_CONFIRMED."""
    e = export(att, name)
    k = e["keys"][f"{ns}|{rid}"]
    row = next((r for r in pg_rows(att, ns, db) if r["request_id"] == rid), None)
    auto_abort = wcall(att, scenario, name, "operator", "outcome", ns=ns, request_id=rid, ticket_id=k["ticket_id"],
                       outcome="ABORT_CONFIRMED", evidence={"reason": "tombstone absent", "pg_rollback_ack": True})
    if row and row["ticket_id"] == k["ticket_id"] and row["payload_hash"] == k["payload_hash"]:
        r = wcall(att, scenario, name, "operator", "outcome", ns=ns, request_id=rid, ticket_id=k["ticket_id"],
                  outcome="COMMIT_CONFIRMED", evidence={"tombstone_ticket_id": row["ticket_id"],
                                                        "tombstone_payload_hash": row["payload_hash"]})
        decision = "COMMIT_CONFIRMED"
    else:
        r = wcall(att, scenario, name, "operator", "outcome", ns=ns, request_id=rid, ticket_id=k["ticket_id"],
                  outcome="PERMANENTLY_DENIED",
                  evidence={"reason": "tombstone absent or different; abort cannot be proven",
                            "pg_row": row})
        decision = "PERMANENTLY_DENIED"
    out = {"key": f"{ns}|{rid}", "witness_state_before": k["state"], "pg_row": row, "auto_abort": auto_abort["status"],
           "decision": decision, "append": r["status"]}
    att.trace(scenario, "reconcile", **out)
    return out


def epoch(att: Attempt, scenario: str, name: str, ns: str, ep: int, holder: str) -> dict:
    return wcall(att, scenario, name, "operator", "epoch_set", ns=ns, epoch=ep, holder=holder)


# --------------------------------------------------------------------------
# Scenarios
# --------------------------------------------------------------------------

def sc_state_machine(att: Attempt) -> None:
    S, W = "W1_STATE_MACHINE", "sm"
    start_witness(att, W)
    ns = "ns-sm"
    epoch(att, S, W, ns, 1, "issuer_a")
    ph = payload_hash(ns, "k1", payload("k1"))
    t1, t2 = str(uuid.uuid4()), str(uuid.uuid4())

    def call(principal, op, key=None, **b):
        return wcall(att, S, W, principal, op, key=key, **b)

    r = call("issuer_a", "reserve", ns=ns, request_id="k1", payload_hash=ph, ticket_id=t1, epoch=1, incarnation="i1")
    att.check("W1-RESERVE", S, "POSITIVE_CONTROL", "First reserve of (ns, k1) is RESERVED", r["status"] == "RESERVED", r)
    n0 = record_count(att, W)
    r = call("issuer_a", "reserve", ns=ns, request_id="k1", payload_hash=ph, ticket_id=t2, epoch=1, incarnation="i2")
    att.check("W1-NO-SECOND-RESERVE", S, "DENIAL", "Second reserve of the same key is DUPLICATE and appends nothing",
              r["status"] == "DUPLICATE" and record_count(att, W) == n0, {"response": r, "records": [n0, record_count(att, W)]})
    r = call("issuer_b", "reserve", ns=ns, request_id="k1", payload_hash=ph, ticket_id=t2, epoch=1, incarnation="i3")
    att.check("W1-NON-HOLDER-RESERVE", S, "DENIAL", "A non-holder issuer cannot reserve", r["status"] == "NOT_EPOCH_HOLDER", r)
    for p in ("operator", "auditor"):
        r = call(p, "reserve", ns=ns, request_id="kx", payload_hash=ph, ticket_id=t2, epoch=1, incarnation="i4")
        att.check(f"W1-{p.upper()}-CANNOT-RESERVE", S, "DENIAL", f"{p} cannot reserve", r["status"] == "ROLE_FORBIDDEN", r)
    r = call("issuer_a", "reserve", key=secrets.token_hex(32), ns=ns, request_id="kx", payload_hash=ph, ticket_id=t2,
             epoch=1, incarnation="i5")
    att.check("W1-BAD-SIGNATURE", S, "DENIAL", "A request signed with the wrong key is AUTH_FAILED", r["status"] == "AUTH_FAILED", r)
    r = call("issuer_a", "outcome", ns=ns, request_id="k1", ticket_id=t2, outcome="COMMIT_CONFIRMED",
             evidence={"pg_commit_ack": True})
    att.check("W1-TICKET-MISMATCH", S, "DENIAL", "Outcome citing a different ticket_id is rejected", r["status"] == "TICKET_MISMATCH", r)
    r = call("issuer_b", "outcome", ns=ns, request_id="k1", ticket_id=t1, outcome="COMMIT_CONFIRMED",
             evidence={"pg_commit_ack": True})
    att.check("W1-NOT-OWNER", S, "DENIAL", "Another issuer cannot append the outcome", r["status"] == "NOT_RESERVATION_OWNER", r)
    r = call("issuer_a", "outcome", ns=ns, request_id="k1", ticket_id=t1, outcome="COMMIT_CONFIRMED", evidence={})
    att.check("W1-EVIDENCE-REQUIRED", S, "DENIAL", "COMMIT_CONFIRMED without a commit acknowledgement is rejected",
              r["status"] == "EVIDENCE_REQUIRED", r)
    r = call("issuer_a", "outcome", ns=ns, request_id="k1", ticket_id=t1, outcome="COMMIT_CONFIRMED",
             evidence={"pg_commit_ack": True})
    att.check("W1-COMMIT", S, "POSITIVE_CONTROL", "Owner appends COMMIT_CONFIRMED for its reservation", r["status"] == "APPENDED", r)
    n1 = record_count(att, W)
    r = call("issuer_a", "outcome", ns=ns, request_id="k1", ticket_id=t1, outcome="COMMIT_CONFIRMED",
             evidence={"pg_commit_ack": True})
    att.check("W1-OUTCOME-IDEMPOTENT", S, "POSITIVE_CONTROL", "Repeating the same outcome is IDEMPOTENT and appends nothing",
              r["status"] == "IDEMPOTENT" and record_count(att, W) == n1, {"response": r, "records": [n1, record_count(att, W)]})
    for op in ("delete", "update", "truncate", "rewrite", "release"):
        r = call("operator", op, ns=ns, request_id="k1")
        att.check(f"W1-NO-{op.upper()}", S, "DENIAL", f"There is no '{op}' operation (UNSUPPORTED_OP)",
                  r["status"] == "UNSUPPORTED_OP", r)
    r = call("issuer_a", "outcome", ns=ns, request_id="nope", ticket_id=t1, outcome="IN_DOUBT", evidence={"reason": "x"})
    att.check("W1-NO-RESERVATION", S, "DENIAL", "Outcome without a reservation is rejected", r["status"] == "NO_RESERVATION", r)
    r = call("operator", "epoch_set", ns=ns, epoch=1, holder="issuer_b")
    att.check("W1-EPOCH-MONOTONIC", S, "DENIAL", "Epoch cannot be set to a value not above the current one",
              r["status"] == "EPOCH_NOT_MONOTONIC", r)
    r = call("issuer_a", "epoch_set", ns=ns, epoch=9, holder="issuer_a")
    att.check("W1-EPOCH-ROLE", S, "DENIAL", "An issuer cannot set the fencing epoch", r["status"] == "ROLE_FORBIDDEN", r)
    e = export(att, W)
    head = e["records"][-1]
    r = call("operator", "checkpoint_accept", seq=head["seq"], head_hash=head["hash"])
    att.check("W1-CHECKPOINT", S, "POSITIVE_CONTROL", "Operator records the accepted sequence", r["status"] == "CHECKPOINT_ACCEPTED", r)
    r = call("operator", "checkpoint_accept", seq=head["seq"] - 1, head_hash=e["records"][-2]["hash"])
    att.check("W1-CHECKPOINT-NO-LOWER", S, "DENIAL", "The accepted sequence cannot be lowered",
              r["status"] == "CHECKPOINT_LOWER_REJECTED", r)
    r = call("operator", "checkpoint_accept", seq=head["seq"], head_hash="f" * 64)
    att.check("W1-CHECKPOINT-HASH", S, "DENIAL", "A checkpoint with a wrong head hash is rejected",
              r["status"] == "CHECKPOINT_HASH_MISMATCH", r)
    r = call("issuer_a", "checkpoint_accept", seq=head["seq"], head_hash=head["hash"])
    att.check("W1-CHECKPOINT-ROLE", S, "DENIAL", "An issuer cannot move the checkpoint", r["status"] == "ROLE_FORBIDDEN", r)
    r = call("issuer_a", "fault", fault="crash_after_append_before_reply")
    att.check("W1-FAULT-ROLE", S, "DENIAL", "Only the test harness principal can arm faults", r["status"] == "ROLE_FORBIDDEN", r)

    ph2 = payload_hash(ns, "k2", payload("k2"))
    t3 = str(uuid.uuid4())
    call("issuer_a", "reserve", ns=ns, request_id="k2", payload_hash=ph2, ticket_id=t3, epoch=1, incarnation="i6")
    time.sleep(3)
    r = call("issuer_a", "reserve", ns=ns, request_id="k2", payload_hash=ph2, ticket_id=str(uuid.uuid4()), epoch=1,
             incarnation="i7")
    att.check("W1-NO-TIMEOUT-RELEASE", S, "DENIAL", "A RESERVED key is still taken after a 3 s wait (no expiry)",
              r["status"] == "DUPLICATE" and r["state"] == "RESERVED", r)
    rc = stop_witness(att, W)
    start_witness(att, W)
    e2 = export(att, W)
    k1, k2 = e2["keys"].get(f"{ns}|k1"), e2["keys"].get(f"{ns}|k2")
    att.check("W1-SURVIVES-RESTART", S, "POSITIVE_CONTROL",
              "After restart: k1 COMMIT_CONFIRMED with full history (reserve + outcome), k2 still RESERVED, same record count",
              rc == 0 and k1["state"] == "COMMIT_CONFIRMED" and len(k1["history"]) == 2 and k2["state"] == "RESERVED"
              and len(e2["records"]) == len(e["records"]) + 1 and e2["checkpoint"]["seq"] == head["seq"],
              {"shutdown_exit": rc, "k1": k1, "k2": k2, "records": len(e2["records"]), "checkpoint": e2["checkpoint"]})

    # Transition matrix: every (from_state, target, role).
    mismatches = []
    rows = []
    n = 0
    for fs in STATES:
        for target in TARGETS:
            for role, principal in (("issuer", "issuer_a"), ("operator", "operator")):
                n += 1
                rid = f"m{n:02d}"
                tid = str(uuid.uuid4())
                h = payload_hash(ns, rid, payload(rid))
                call("issuer_a", "reserve", ns=ns, request_id=rid, payload_hash=h, ticket_id=tid, epoch=1, incarnation="im")
                if fs == "COMMIT_CONFIRMED":
                    call("issuer_a", "outcome", ns=ns, request_id=rid, ticket_id=tid, outcome=fs, evidence={"pg_commit_ack": True})
                elif fs == "ABORT_CONFIRMED":
                    call("issuer_a", "outcome", ns=ns, request_id=rid, ticket_id=tid, outcome=fs, evidence={"pg_rollback_ack": True})
                elif fs in ("IN_DOUBT", "PERMANENTLY_DENIED"):
                    call("issuer_a", "outcome", ns=ns, request_id=rid, ticket_id=tid, outcome="IN_DOUBT", evidence={"reason": "setup"})
                    if fs == "PERMANENTLY_DENIED":
                        call("operator", "outcome", ns=ns, request_id=rid, ticket_id=tid, outcome=fs, evidence={"reason": "setup"})
                if key_state(att, W, ns, rid) != fs:
                    mismatches.append({"setup_failed": rid, "from": fs})
                    continue
                if target == "COMMIT_CONFIRMED":
                    ev = {"pg_commit_ack": True} if role == "issuer" else {"tombstone_ticket_id": tid, "tombstone_payload_hash": h}
                elif target == "ABORT_CONFIRMED":
                    ev = {"pg_rollback_ack": True}
                else:
                    ev = {"reason": "matrix"}
                r = call(principal, "outcome", ns=ns, request_id=rid, ticket_id=tid, outcome=target, evidence=ev)
                want = "IDEMPOTENT" if fs == target else "APPENDED" if (role, fs, target) in EXPECTED_ALLOWED else "INVALID_TRANSITION"
                rows.append({"from": fs, "to": target, "role": role, "expected": want, "observed": r["status"]})
                if r["status"] != want:
                    mismatches.append(rows[-1])
    att.write("traces/W1_transition_matrix.json", json.dumps(rows, indent=2) + "\n")
    att.check("W1-TRANSITION-MATRIX", S, "POSITIVE_CONTROL",
              "All 40 (from_state, outcome, role) cases match the 0.1P table: allowed -> APPENDED, same state -> IDEMPOTENT, "
              "everything else -> INVALID_TRANSITION", not mismatches and len(rows) == 40,
              {"cases": len(rows), "mismatches": mismatches})
    bad = [r for r in rows if r["from"] == "IN_DOUBT" and r["to"] == "ABORT_CONFIRMED"]
    att.check("W1-IN-DOUBT-NEVER-ABORT", S, "DENIAL", "IN_DOUBT -> ABORT_CONFIRMED is rejected for issuer and operator",
              len(bad) == 2 and all(r["observed"] == "INVALID_TRANSITION" for r in bad), bad)
    term = [r for r in rows if r["from"] in ("COMMIT_CONFIRMED", "PERMANENTLY_DENIED") and r["to"] != r["from"]]
    att.check("W1-TERMINAL-STATES", S, "DENIAL", "COMMIT_CONFIRMED and PERMANENTLY_DENIED accept no other outcome",
              all(r["observed"] == "INVALID_TRANSITION" for r in term) and len(term) == 12, len(term))
    stop_witness(att, W)


def sc_concurrency(att: Attempt) -> None:
    S, W = "W2_CONCURRENT_RESERVE", "conc"
    start_witness(att, W)
    ns = "ns-conc"
    epoch(att, S, W, ns, 1, "issuer_a")
    for rnd in range(1, 4):
        rid = f"race-{rnd}"
        start_at = time.time() + 4
        hs = [spawn(att, f"race{rnd}-{i}", worker_cfg(att, W, "issuer_a", ns, rid, mode="reserve_only", start_at=start_at))
              for i in range(8)]
        res = [finish(att, S, h) for h in hs]
        codes = [rcode(r) for r in res]
        n_res = reserve_records(att, W, ns, rid)
        ok = codes.count("RESERVED") == 1 and codes.count("DUPLICATE") == 7 and n_res == 1
        att.check(f"W2-RACE-{rnd}", S, "POSITIVE_CONTROL",
                  f"Round {rnd}: 8 concurrent processes reserve one key; exactly one RESERVED, 7 DUPLICATE, one RESERVE record",
                  ok, {"results": codes, "reserve_records": n_res})
        if codes.count("RESERVED") > 1 or n_res > 1:
            att.finding("F-DUP-RESERVE", "Witness accepted two reservations for one key", {"round": rnd, "codes": codes})
            att.stop_scope = True
    other = start_witness(att, W, expect_ready=False)
    try:
        other["proc"].wait(15)
    except subprocess.TimeoutExpired:
        other["proc"].kill()
    att.witnesses[W]["current"] = att.witnesses[W]["handles"][0]
    healthy = export(att, W).get("status")
    att.check("W2-SINGLE-WRITER", S, "DENIAL", "A second witness process on the same store refuses to start (exit 3, store_locked)",
              other["proc"].returncode == 3 and any("store_locked" in l for l in other["stdout"]) and healthy == "OK",
              {"exit": other["proc"].returncode, "stdout": other["stdout"], "first_still_serving": healthy})
    stop_witness(att, W)


def sc_e2e(att: Attempt) -> None:
    W = "e2e"
    start_witness(att, W)

    # OK path and ordering.
    S, ns = "S0_NORMAL", "ns-ok"
    epoch(att, S, W, ns, 1, "issuer_a")
    s = issue(att, S, "ok", W, "issuer_a", ns, "k1")
    rows = pg_rows(att, ns)
    i_res, i_beg = ev_index(s, "witness_reserve", status="RESERVED"), ev_index(s, "pg_begin_sent")
    att.check("S0-ISSUE-OK", S, "POSITIVE_CONTROL", "Normal issuance returns ISSUE_OK with the ticket in PostgreSQL and COMMIT_CONFIRMED",
              rcode(s) == "ISSUE_OK" and len(rows) == 1 and rows[0]["ticket_id"] == s["result"]["ticket_id"]
              and key_state(att, W, ns, "k1") == "COMMIT_CONFIRMED", {"result": s["result"], "rows": rows})
    att.check("S0-RESERVE-BEFORE-PG", S, "ORDERING", "Witness RESERVED is received before the PostgreSQL ticket transaction begins",
              0 <= i_res < i_beg, {"events": s["events"]})

    # A: crash after reservation, before PostgreSQL.
    S, ns = "S_A_CRASH_AFTER_RESERVATION", "ns-a"
    epoch(att, S, W, ns, 1, "issuer_a")
    s1 = issue(att, S, "a-crash", W, "issuer_a", ns, "k1", fault="crash_after_reserve")
    st, rows, conns = key_state(att, W, ns, "k1"), pg_rows(att, ns), txn_connections(att, s1["app"])
    att.check("SA-STATE-AFTER-CRASH", S, "POSITIVE_CONTROL",
              "Process died after RESERVED: exit 137, witness RESERVED, no PostgreSQL ticket transaction, no ticket",
              s1["exit"] == 137 and s1["result"] is None and st == "RESERVED" and not rows and conns == 0,
              {"exit": s1["exit"], "state": st, "rows": rows, "txn_connections": conns})
    s2 = issue(att, S, "a-restart", W, "issuer_a", ns, "k1")
    att.check("SA-RESTART-IN-DOUBT", S, "DENIAL", "Restarted process marks the old reservation IN_DOUBT and issues nothing",
              rcode(s2) == "ISSUE_DENIED_IN_DOUBT" and key_state(att, W, ns, "k1") == "IN_DOUBT" and not pg_rows(att, ns),
              s2["result"])
    s2b = issue(att, S, "a-other-key", W, "issuer_a", ns, "k2")
    att.check("SA-GATE-CLOSED-WHILE-IN-DOUBT", S, "DENIAL", "A new request in the namespace is refused while a key is IN_DOUBT",
              rcode(s2b) == "ISSUE_DENIED_GATE_CLOSED" and key_state(att, W, ns, "k2") is None, s2b["result"])
    rec = reconcile(att, S, W, ns, "k1")
    att.check("SA-RECONCILE", S, "POSITIVE_CONTROL",
              "Reconciliation: automatic ABORT_CONFIRMED rejected; tombstone absent -> PERMANENTLY_DENIED",
              rec["auto_abort"] == "INVALID_TRANSITION" and rec["decision"] == "PERMANENTLY_DENIED" and rec["append"] == "APPENDED", rec)
    s3 = issue(att, S, "a-retry", W, "issuer_a", ns, "k1")
    s4 = issue(att, S, "a-new-key", W, "issuer_a", ns, "k2")
    att.check("SA-RETRY-DENIED", S, "DENIAL", "Retry of the same request after reconciliation is ISSUE_DENIED_PERMANENTLY",
              rcode(s3) == "ISSUE_DENIED_PERMANENTLY" and reserve_records(att, W, ns, "k1") == 1, s3["result"])
    att.check("SA-NEW-KEY-AFTER-RECONCILE", S, "POSITIVE_CONTROL", "After reconciliation a new request_id issues normally",
              rcode(s4) == "ISSUE_OK" and [r["request_id"] for r in pg_rows(att, ns)] == ["k2"], s4["result"])

    # Crash before PostgreSQL COMMIT.
    S, ns = "S_CRASH_BEFORE_PG_COMMIT", "ns-pc"
    epoch(att, S, W, ns, 1, "issuer_a")
    s1 = issue(att, S, "pc-crash", W, "issuer_a", ns, "k1", fault="crash_before_commit")
    time.sleep(1.5)
    rows, st = pg_rows(att, ns), key_state(att, W, ns, "k1")
    att.check("SPC-STATE", S, "POSITIVE_CONTROL",
              "Died after INSERT before COMMIT: exit 137, open transaction rolled back by the server, witness RESERVED",
              s1["exit"] == 137 and "pg_insert" in s1["events"] and not rows and st == "RESERVED",
              {"exit": s1["exit"], "events": s1["events"], "rows": rows, "state": st})
    s2 = issue(att, S, "pc-restart", W, "issuer_a", ns, "k1")
    rec = reconcile(att, S, W, ns, "k1")
    s3 = issue(att, S, "pc-retry", W, "issuer_a", ns, "k1")
    att.check("SPC-NO-SECOND-TICKET", S, "DENIAL",
              "Restart -> IN_DOUBT; reconciliation -> PERMANENTLY_DENIED (never ABORT); retry denied; zero tickets; one reservation",
              rcode(s2) == "ISSUE_DENIED_IN_DOUBT" and rec["decision"] == "PERMANENTLY_DENIED" and rec["auto_abort"] == "INVALID_TRANSITION"
              and rcode(s3) == "ISSUE_DENIED_PERMANENTLY" and not pg_rows(att, ns) and reserve_records(att, W, ns, "k1") == 1,
              {"restart": s2["result"], "reconcile": rec, "retry": s3["result"]})

    # B: PostgreSQL COMMIT done, outcome append missing.
    S, ns = "S_B_COMMIT_WITHOUT_OUTCOME", "ns-b"
    epoch(att, S, W, ns, 1, "issuer_a")
    s1 = issue(att, S, "b-crash", W, "issuer_a", ns, "k1", fault="crash_after_commit")
    rows, st = pg_rows(att, ns), key_state(att, W, ns, "k1")
    att.check("SB-STATE", S, "POSITIVE_CONTROL",
              "Died after COMMIT before outcome: ticket row present, witness still RESERVED, no response/ticket id returned",
              s1["exit"] == 137 and s1["result"] is None and len(rows) == 1 and st == "RESERVED",
              {"exit": s1["exit"], "rows": rows, "state": st})
    s2 = issue(att, S, "b-retry-1", W, "issuer_a", ns, "k1")
    s2b = issue(att, S, "b-other-key", W, "issuer_a", ns, "k2")
    att.check("SB-NO-GRANT-WHILE-IN-DOUBT", S, "DENIAL",
              "Before reconciliation the retry gets no ticket (IN_DOUBT) and other requests are refused (gate closed)",
              rcode(s2) == "ISSUE_DENIED_IN_DOUBT" and rcode(s2b) == "ISSUE_DENIED_GATE_CLOSED",
              {"retry": s2["result"], "other": s2b["result"]})
    e = export(att, W)["keys"][f"{ns}|k1"]
    bad = wcall(att, S, W, "operator", "outcome", ns=ns, request_id="k1", ticket_id=e["ticket_id"], outcome="COMMIT_CONFIRMED",
                evidence={"tombstone_ticket_id": str(uuid.uuid4()), "tombstone_payload_hash": e["payload_hash"]})
    att.check("SB-RECONCILE-EVIDENCE-BOUND", S, "DENIAL", "Operator COMMIT_CONFIRMED citing a different tombstone ticket is rejected",
              bad["status"] == "EVIDENCE_MISMATCH", bad)
    rec = reconcile(att, S, W, ns, "k1")
    s3 = issue(att, S, "b-retry-2", W, "issuer_a", ns, "k1")
    att.check("SB-RECONCILE-COMMIT", S, "POSITIVE_CONTROL",
              "Matching tombstone -> COMMIT_CONFIRMED; retry replays the same ticket; still one row and one reservation",
              rec["decision"] == "COMMIT_CONFIRMED" and rec["append"] == "APPENDED" and rcode(s3) == "ISSUE_REPLAY_ISSUED"
              and s3["result"]["ticket_id"] == rows[0]["ticket_id"] and len(pg_rows(att, ns)) == 1
              and reserve_records(att, W, ns, "k1") == 1, {"reconcile": rec, "retry": s3["result"]})

    # C: PostgreSQL ROLLBACK after reservation.
    S, ns = "S_C_ROLLBACK", "ns-c"
    epoch(att, S, W, ns, 1, "issuer_a")
    s1 = issue(att, S, "c-rollback", W, "issuer_a", ns, "k1", fault="rollback")
    e = export(att, W)["keys"][f"{ns}|k1"]
    att.check("SC-ABORT-CONFIRMED", S, "POSITIVE_CONTROL", "Observed ROLLBACK -> ABORT_CONFIRMED; reservation kept; no ticket",
              rcode(s1) == "ISSUE_ABORTED" and e["state"] == "ABORT_CONFIRMED" and not pg_rows(att, ns), {"result": s1["result"], "key": e})
    r = wcall(att, S, W, "issuer_a", "reserve", ns=ns, request_id="k1", payload_hash=e["payload_hash"],
              ticket_id=str(uuid.uuid4()), epoch=1, incarnation="direct")
    att.check("SC-NO-NEW-RESERVATION", S, "DENIAL", "ABORT_CONFIRMED does not free the key: a new reserve is DUPLICATE",
              r["status"] == "DUPLICATE", r)
    s2 = issue(att, S, "c-mismatch", W, "issuer_a", ns, "k1", payload=payload("k1", revision=2))
    att.check("SC-PAYLOAD-MISMATCH", S, "DENIAL", "Same request_id with a different payload is ISSUE_DENIED_PAYLOAD_MISMATCH",
              rcode(s2) == "ISSUE_DENIED_PAYLOAD_MISMATCH", s2["result"])
    s3 = issue(att, S, "c-resume", W, "issuer_a", ns, "k1")
    rows = pg_rows(att, ns)
    att.check("SC-RESUME-SAME-RESERVATION", S, "POSITIVE_CONTROL",
              "Retry resumes the same reservation: ISSUE_OK with the reserved ticket_id, one row, one RESERVE record",
              rcode(s3) == "ISSUE_OK" and s3["result"].get("resumed") is True and s3["result"]["ticket_id"] == e["ticket_id"]
              and len(rows) == 1 and reserve_records(att, W, ns, "k1") == 1, {"result": s3["result"], "rows": rows})

    # D1: COMMIT reached the server, acknowledgement never read.
    S, ns = "S_D1_UNKNOWN_COMMIT_COMMITTED", "ns-d1"
    epoch(att, S, W, ns, 1, "issuer_a")
    s1 = issue(att, S, "d1", W, "issuer_a", ns, "k1", fault="commit_ack_unobserved")
    time.sleep(1.5)
    rows = pg_rows(att, ns)
    att.check("SD1-IN-DOUBT", S, "POSITIVE_CONTROL", "Unobserved COMMIT acknowledgement -> IN_DOUBT, no ticket id returned, row exists",
              rcode(s1) == "ISSUE_IN_DOUBT" and key_state(att, W, ns, "k1") == "IN_DOUBT" and len(rows) == 1,
              {"result": s1["result"], "rows": rows})
    s2 = issue(att, S, "d1-retry-1", W, "issuer_a", ns, "k1")
    rec = reconcile(att, S, W, ns, "k1")
    s3 = issue(att, S, "d1-retry-2", W, "issuer_a", ns, "k1")
    att.check("SD1-RECONCILE", S, "POSITIVE_CONTROL",
              "Retry before reconciliation gets nothing; auto-abort rejected; matching row -> COMMIT_CONFIRMED; replay same ticket",
              rcode(s2) == "ISSUE_DENIED_IN_DOUBT" and rec["auto_abort"] == "INVALID_TRANSITION" and rec["decision"] == "COMMIT_CONFIRMED"
              and rcode(s3) == "ISSUE_REPLAY_ISSUED" and s3["result"]["ticket_id"] == rows[0]["ticket_id"] and len(pg_rows(att, ns)) == 1,
              {"retry1": s2["result"], "reconcile": rec, "retry2": s3["result"]})

    # D2: backend terminated before COMMIT; client sees a lost connection.
    S, ns = "S_D2_UNKNOWN_COMMIT_NOT_COMMITTED", "ns-d2"
    epoch(att, S, W, ns, 1, "issuer_a")
    go = att.tmp / "d2.go"
    h = spawn(att, "d2", worker_cfg(att, W, "issuer_a", ns, "k1", fault="backend_terminated_before_commit", go_file=str(go)))
    pid = None
    deadline = time.monotonic() + 30
    while time.monotonic() < deadline and pid is None:
        for l in list(h["out"]):
            if l.startswith("EVENT ") and '"waiting_for_go"' in l:
                pid = json.loads(l[6:]).get("backend_pid")
        time.sleep(0.05)
    term = admin_sql(att, att.db, f"SELECT pg_terminate_backend({int(pid)}, 5000);") if pid else {"stdout": ""}
    att.trace(S, "harness_terminate_backend", pid=pid, result=term["stdout"].strip())
    go.write_text("go")
    s1 = finish(att, S, h)
    rows = pg_rows(att, ns)
    commit_ev = next((e for e in s1["_events"] if e["event"] == "pg_commit"), {})
    att.check("SD2-IN-DOUBT", S, "POSITIVE_CONTROL",
              "Connection lost during COMMIT -> IN_DOUBT, no ticket id returned; the row does not exist",
              rcode(s1) == "ISSUE_IN_DOUBT" and term["stdout"].strip() == "t" and commit_ev.get("lost") is True
              and key_state(att, W, ns, "k1") == "IN_DOUBT" and not rows,
              {"result": s1["result"], "terminate": term["stdout"].strip(), "commit_event": commit_ev, "rows": rows})
    rec = reconcile(att, S, W, ns, "k1")
    s2 = issue(att, S, "d2-retry", W, "issuer_a", ns, "k1")
    att.check("SD2-NO-AUTO-ABORT", S, "DENIAL",
              "Unknown outcome is never auto-converted to ABORT_CONFIRMED; it becomes PERMANENTLY_DENIED; retry gets no ticket",
              rec["auto_abort"] == "INVALID_TRANSITION" and rec["decision"] == "PERMANENTLY_DENIED"
              and rcode(s2) == "ISSUE_DENIED_PERMANENTLY" and not pg_rows(att, ns), {"reconcile": rec, "retry": s2["result"]})

    # E: HTTP response lost after COMMIT_CONFIRMED.
    S, ns = "S_E_RESPONSE_LOST", "ns-e"
    epoch(att, S, W, ns, 1, "issuer_a")
    s1 = issue(att, S, "e-lost", W, "issuer_a", ns, "k1", fault="lose_response")
    rows = pg_rows(att, ns)
    n = record_count(att, W)
    s2 = issue(att, S, "e-retry", W, "issuer_a", ns, "k1")
    s3 = issue(att, S, "e-mismatch", W, "issuer_a", ns, "k1", payload=payload("k1", revision=2))
    att.check("SE-REPLAY", S, "POSITIVE_CONTROL",
              "Lost response: retry returns ISSUE_REPLAY_ISSUED with the same ticket; no new witness record; one row",
              rcode(s1) == "RESPONSE_LOST" and "ticket_id" not in s1["result"] and rcode(s2) == "ISSUE_REPLAY_ISSUED"
              and s2["result"]["ticket_id"] == rows[0]["ticket_id"] and record_count(att, W) == n and len(pg_rows(att, ns)) == 1,
              {"lost": s1["result"], "retry": s2["result"], "records": [n, record_count(att, W)]})
    att.check("SE-PAYLOAD-MISMATCH", S, "DENIAL", "Retry with a different payload is ISSUE_DENIED_PAYLOAD_MISMATCH",
              rcode(s3) == "ISSUE_DENIED_PAYLOAD_MISMATCH", s3["result"])
    stop_witness(att, W)


def sc_lost_reserve_ack(att: Attempt) -> None:
    S, W, ns = "S_WITNESS_CRASH_AFTER_APPEND", "lostack", "ns-l"
    start_witness(att, W)
    epoch(att, S, W, ns, 1, "issuer_a")
    wcall(att, S, W, "harness", "fault", fault="crash_after_append_before_reply")
    s1 = issue(att, S, "l-unknown", W, "issuer_a", ns, "k1")
    att.witnesses[W]["current"]["proc"].wait(10)
    wexit = att.witnesses[W]["current"]["proc"].returncode
    conns = txn_connections(att, s1["app"])
    att.check("SL-NO-PG-WITHOUT-ACK", S, "DENIAL",
              "Witness died after the durable append but before replying: issuer gets no ack and does not start PostgreSQL",
              rcode(s1) == "ISSUE_WITNESS_OUTCOME_UNKNOWN" and wexit == 137 and conns == 0 and not pg_rows(att, ns),
              {"result": s1["result"], "witness_exit": wexit, "txn_connections": conns})
    start_witness(att, W)
    st = key_state(att, W, ns, "k1")
    s2 = issue(att, S, "l-restart", W, "issuer_a", ns, "k1")
    rec = reconcile(att, S, W, ns, "k1")
    att.check("SL-DURABLE-RESERVATION", S, "POSITIVE_CONTROL",
              "After witness restart the unacknowledged reservation is RESERVED; retry -> IN_DOUBT; reconciliation -> PERMANENTLY_DENIED",
              st == "RESERVED" and rcode(s2) == "ISSUE_DENIED_IN_DOUBT" and rec["decision"] == "PERMANENTLY_DENIED"
              and reserve_records(att, W, ns, "k1") == 1, {"state_after_restart": st, "retry": s2["result"], "reconcile": rec})
    stop_witness(att, W)


def sc_unavailable(att: Attempt) -> None:
    S, W, ns = "S_WITNESS_UNAVAILABLE", "unavail", "ns-u"
    start_witness(att, W)
    epoch(att, S, W, ns, 1, "issuer_a")
    n = record_count(att, W)
    stop_witness(att, W)
    s1 = issue(att, S, "u-down", W, "issuer_a", ns, "k1")
    conns = any_connections(att, s1["app"])
    att.check("SU-DENIED", S, "DENIAL", "Witness down: ISSUE_DENIED_WITNESS_UNAVAILABLE and no PostgreSQL connection at all",
              rcode(s1) == "ISSUE_DENIED_WITNESS_UNAVAILABLE" and conns == 0 and not pg_rows(att, ns),
              {"result": s1["result"], "pg_connections": conns})
    start_witness(att, W)
    att.check("SU-NO-SIDE-EFFECT", S, "POSITIVE_CONTROL", "After restart the witness has the same records and no reservation for k1",
              record_count(att, W) == n and key_state(att, W, ns, "k1") is None, {"records": [n, record_count(att, W)]})
    stop_witness(att, W)


def seed(att: Attempt, S: str, W: str, ns: str, checkpoint_at: int) -> list[dict]:
    epoch(att, S, W, ns, 1, "issuer_a")
    for rid in ("k1", "k2"):
        tid = str(uuid.uuid4())
        wcall(att, S, W, "issuer_a", "reserve", ns=ns, request_id=rid, payload_hash=payload_hash(ns, rid, payload(rid)),
              ticket_id=tid, epoch=1, incarnation="seed")
        wcall(att, S, W, "issuer_a", "outcome", ns=ns, request_id=rid, ticket_id=tid, outcome="COMMIT_CONFIRMED",
              evidence={"pg_commit_ack": True})
    recs = export(att, W)["records"]
    cp = recs[checkpoint_at - 1]
    wcall(att, S, W, "operator", "checkpoint_accept", seq=cp["seq"], head_hash=cp["hash"])
    return recs


def sc_corruption(att: Attempt) -> None:
    S, ns = "S_WITNESS_CORRUPTION", "ns-x"
    from common import canon, sha256_hex

    def lines_of(W):
        return (att.witnesses[W]["store"] / "witness_log.jsonl").read_text(encoding="utf-8").splitlines()

    def put(W, lines):
        (att.witnesses[W]["store"] / "witness_log.jsonl").write_text("\n".join(lines) + "\n", encoding="utf-8", newline="\n")

    def rehash(lines):
        out, prev = [], "0" * 64
        for l in lines:
            r = json.loads(l)
            r["prev_hash"] = prev
            r.pop("hash", None)
            r["hash"] = sha256_hex(canon(r))
            prev = r["hash"]
            out.append(canon(r).decode())
        return out

    variants = []

    def variant(tag, mutate, expect_reason, cp_at=5):
        W = f"corrupt-{tag}"
        start_witness(att, W)
        seed(att, S, W, ns, cp_at)
        stop_witness(att, W)
        before = lines_of(W)
        att.write(f"witness/{W}/tamper/before.jsonl", "\n".join(before) + "\n")
        mutate(W, before)
        after = lines_of(W)
        att.write(f"witness/{W}/tamper/after.jsonl", "\n".join(after) + "\n")
        start_witness(att, W)
        hl = export(att, W)
        s = issue(att, S, f"x-{tag}", W, "issuer_a", ns, "k9")
        conns = any_connections(att, s["app"])
        ok = hl.get("status") == "WITNESS_FAILED" and expect_reason in (hl.get("reason") or "") \
            and rcode(s) == "ISSUE_DENIED_WITNESS_FAILED" and conns == 0
        att.check(f"SX-{tag.upper()}", S, "DENIAL",
                  f"Tamper '{tag}' is detected at load ({expect_reason}); every request refused; issuer denies without touching PostgreSQL",
                  ok, {"witness": {k: hl.get(k) for k in ("status", "reason")}, "issuer": s["result"], "pg_connections": conns})
        variants.append(tag)
        stop_witness(att, W)

    def flip(W, lines):
        r = json.loads(lines[1])
        r["payload_hash"] = ("0" if r["payload_hash"][0] != "0" else "1") + r["payload_hash"][1:]
        lines[1] = canon(r).decode()
        put(W, lines)

    def rewrite_record_only(W, lines):
        r = json.loads(lines[1])
        r["payload_hash"] = "a" * 64
        lines[1] = canon(r).decode()
        put(W, rehash(lines))

    def rewrite_signed_body(W, lines):
        r = json.loads(lines[1])
        r["payload_hash"] = "b" * 64
        r["request"]["body"]["payload_hash"] = "b" * 64
        lines[1] = canon(r).decode()
        put(W, rehash(lines))

    def reorder(W, lines):
        lines[3], lines[4] = lines[4], lines[3]
        put(W, lines)

    def truncate_below_checkpoint(W, lines):
        put(W, lines[:3])

    def forge_checkpoint(W, lines):
        p = att.witnesses[W]["cp"] / "accepted_checkpoint.json"
        cp = json.loads(p.read_text(encoding="utf-8"))
        cp["request"]["body"]["seq"] = 2
        cp["request"]["body"]["head_hash"] = json.loads(lines[1])["hash"]
        p.write_text(canon(cp).decode() + "\n", encoding="utf-8", newline="\n")
        put(W, lines[:2])

    def garbage(W, lines):
        lines[2] = lines[2][: len(lines[2]) // 2]
        put(W, lines)

    variant("flip", flip, "HASH_MISMATCH")
    variant("rewrite-record", rewrite_record_only, "RECORD_REQUEST_MISMATCH")
    variant("rewrite-signed", rewrite_signed_body, "REQUEST_MAC_INVALID")
    variant("reorder", reorder, "SEQUENCE_VIOLATION")
    variant("truncate-below-checkpoint", truncate_below_checkpoint, "ROLLBACK_DETECTED")
    variant("forge-checkpoint", forge_checkpoint, "CHECKPOINT_MAC_INVALID")
    variant("garbage", garbage, "UNPARSEABLE_RECORD")

    # Tail loss after the accepted checkpoint is not detectable by this witness.
    W = "corrupt-tail-loss"
    start_witness(att, W)
    recs = seed(att, S, W, ns, 3)
    stop_witness(att, W)
    before = lines_of(W)
    att.write(f"witness/{W}/tamper/before.jsonl", "\n".join(before) + "\n")
    put(W, before[:3])
    att.write(f"witness/{W}/tamper/after.jsonl", "\n".join(lines_of(W)) + "\n")
    start_witness(att, W)
    hl = export(att, W)
    k1 = wcall(att, S, W, "issuer_a", "reserve", ns=ns, request_id="k1", payload_hash=payload_hash(ns, "k1", payload("k1")),
               ticket_id=str(uuid.uuid4()), epoch=1, incarnation="after-tamper")
    k2 = wcall(att, S, W, "issuer_a", "reserve", ns=ns, request_id="k2", payload_hash=payload_hash(ns, "k2", payload("k2")),
               ticket_id=str(uuid.uuid4()), epoch=1, incarnation="after-tamper")
    reproduced = hl.get("status") == "OK" and k1["status"] == "DUPLICATE" and k2["status"] == "RESERVED"
    att.check("SX-TAIL-LOSS-UNDETECTED", S, "COUNTEREXAMPLE_REPRODUCED",
              "Removing records after the accepted checkpoint is NOT detected: witness loads OK and k2 (lost tail) can be reserved "
              "a second time; k1 (covered by the checkpoint) stays DUPLICATE",
              reproduced, {"health": {k: hl.get(k) for k in ("status", "reason")}, "k1": k1, "k2": k2,
                           "records_before": len(recs), "records_after_tamper": 3})
    if k2["status"] == "RESERVED":
        att.finding("F-TAIL-LOSS", "Witness storage rollback after the last accepted checkpoint is undetectable in this model; "
                    "the lost key was reserved a second time (two successful reservations of one key across a storage rollback).",
                    {"key": f"{ns}|k2", "second_reserve": k2})
    stop_witness(att, W)


def sc_restore(att: Attempt) -> None:
    S, W, ns, db = "S_STALE_PG_RESTORE", "restore", "ns-r", att.db_restore
    start_witness(att, W)
    epoch(att, S, W, ns, 1, "issuer_a")
    s1 = issue(att, S, "r-k1", W, "issuer_a", ns, "k1", db=db)
    dump = run(att, ["docker", "exec", att.container, "pg_dump", "-U", ADMIN, "-Fc", "-d", db, "-f", "/tmp/r0.dump"],
               "traces/S_STALE_PG_RESTORE_pg_dump")
    s2 = issue(att, S, "r-k2", W, "issuer_a", ns, "k2", db=db)
    s3 = issue(att, S, "r-k3-crash", W, "issuer_a", ns, "k3", db=db, fault="crash_after_commit")
    before = pg_rows(att, ns, db)
    admin_sql(att, att.db, f"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '{db}';")
    r1 = run(att, ["docker", "exec", att.container, "dropdb", "-U", ADMIN, db], "traces/S_STALE_PG_RESTORE_dropdb")
    r2 = run(att, ["docker", "exec", att.container, "createdb", "-U", ADMIN, db], "traces/S_STALE_PG_RESTORE_createdb")
    r3 = run(att, ["docker", "exec", att.container, "pg_restore", "-U", ADMIN, "-d", db, "/tmp/r0.dump"],
             "traces/S_STALE_PG_RESTORE_pg_restore")
    after = pg_rows(att, ns, db)
    att.check("SR-SETUP", S, "POSITIVE_CONTROL",
              "Before restore: k1, k2 issued, k3 committed in PostgreSQL but witness RESERVED; after restoring the old dump only k1 remains",
              rcode(s1) == "ISSUE_OK" and rcode(s2) == "ISSUE_OK" and s3["exit"] == 137 and dump["exit"] == 0
              and r1["exit"] == 0 and r2["exit"] == 0 and r3["exit"] == 0
              and [r["request_id"] for r in before] == ["k1", "k2", "k3"] and [r["request_id"] for r in after] == ["k1"],
              {"before": [r["request_id"] for r in before], "after": [r["request_id"] for r in after]})
    s4 = issue(att, S, "r-k2-retry", W, "issuer_a", ns, "k2", db=db)
    att.check("SR-CONFIRMED-NOT-REISSUED", S, "DENIAL",
              "k2 is COMMIT_CONFIRMED in the witness but missing in the restored DB: no new ticket, reconciliation required",
              rcode(s4) == "ISSUE_DENIED_RECONCILIATION_REQUIRED" and [r["request_id"] for r in pg_rows(att, ns, db)] == ["k1"],
              s4["result"])
    s5 = issue(att, S, "r-k3-retry", W, "issuer_a", ns, "k3", db=db)
    rec = reconcile(att, S, W, ns, "k3", db)
    att.check("SR-RESERVED-NOT-FREED", S, "DENIAL",
              "k3 (committed before the restore, row now gone) stays blocked: retry IN_DOUBT; auto-abort rejected; "
              "-> PERMANENTLY_DENIED, not a free request_id",
              rcode(s5) == "ISSUE_DENIED_IN_DOUBT" and rec["auto_abort"] == "INVALID_TRANSITION" and rec["decision"] == "PERMANENTLY_DENIED",
              {"retry": s5["result"], "reconcile": rec})
    s6 = issue(att, S, "r-k4-new", W, "issuer_a", ns, "k4", db=db)
    att.check("SR-GATE-CLOSED", S, "DENIAL",
              "A new request after the stale restore is refused: gate CLOSED because a confirmed ticket is missing in PostgreSQL",
              rcode(s6) == "ISSUE_DENIED_GATE_CLOSED" and any("CONFIRMED_TICKET_MISSING" in x for x in s6["result"]["reasons"])
              and key_state(att, W, ns, "k4") is None, s6["result"])
    naive = admin_sql(att, db, "BEGIN; INSERT INTO tickets (ticket_id, issuer_namespace_id, request_id, payload_hash, "
                               f"witness_seq, issued_by, issued_epoch) VALUES ('{uuid.uuid4()}', '{ns}', 'k2', '{'c' * 64}', 0, "
                               "'naive', 1); SELECT 'INSERTED=' || count(*) FROM tickets WHERE request_id = 'k2'; ROLLBACK;")
    att.check("SR-PG-ALONE-WOULD-ACCEPT", S, "COUNTEREXAMPLE_REPRODUCED",
              "Without the witness, the restored database accepts a second ticket for k2 (insert succeeds; rolled back by harness)",
              naive["exit"] == 0 and "INSERTED=1" in naive["stdout"], {"stdout": naive["stdout"], "stderr": naive["stderr"]})
    att.check("SR-FINAL", S, "POSITIVE_CONTROL", "Final restored DB holds only k1; each key has exactly one witness reservation",
              [r["request_id"] for r in pg_rows(att, ns, db)] == ["k1"]
              and all(reserve_records(att, W, ns, k) == 1 for k in ("k1", "k2", "k3")) and reserve_records(att, W, ns, "k4") == 0,
              [r["request_id"] for r in pg_rows(att, ns, db)])
    stop_witness(att, W)


def sc_split_brain(att: Attempt) -> None:
    S, W = "S_SPLIT_BRAIN", "sb"
    start_witness(att, W)
    ns = "ns-sb"
    epoch(att, S, W, ns, 1, "issuer_a")
    s1 = issue(att, S, "sb-a-k1", W, "issuer_a", ns, "k1")
    t0 = str(uuid.uuid4())
    ph0 = payload_hash(ns, "k0", payload("k0"))
    r0 = wcall(att, S, W, "issuer_a", "reserve", ns=ns, request_id="k0", payload_hash=ph0, ticket_id=t0, epoch=1, incarnation="a-old")
    h = epoch(att, S, W, ns, 2, "issuer_b")
    att.check("SS-HANDOVER", S, "POSITIVE_CONTROL", "A issued k1 and holds an in-flight reservation k0; operator moves epoch 2 to B",
              rcode(s1) == "ISSUE_OK" and r0["status"] == "RESERVED" and h["status"] == "EPOCH_SET", {"k1": s1["result"], "k0": r0, "epoch": h})
    ins = pg_query(att.container, ISSUER_ROLE, att.db,
                   "INSERT INTO tickets (ticket_id, issuer_namespace_id, request_id, payload_hash, witness_seq, issued_by, issued_epoch) "
                   f"VALUES ('{t0}', '{ns}', 'k0', '{ph0}', {r0['seq']}, 'issuer_a', 1);", "a-old-primary")
    oc = wcall(att, S, W, "issuer_a", "outcome", ns=ns, request_id="k0", ticket_id=t0, outcome="COMMIT_CONFIRMED",
               evidence={"pg_commit_ack": ins["exit"] == 0})
    att.check("SS-OLD-COMMIT-NOT-FENCED", S, "COUNTEREXAMPLE_REPRODUCED",
              "After handover, old issuer A's PostgreSQL COMMIT for its earlier reservation k0 still succeeds; the witness does not stop it "
              "and accepts A's outcome for that existing reservation",
              ins["exit"] == 0 and oc["status"] == "APPENDED", {"insert": ins["stdout"].strip() or ins["stderr"], "outcome": oc})
    s2 = issue(att, S, "sb-a-stale", W, "issuer_a", ns, "k2", epoch=1)
    ra = wcall(att, S, W, "issuer_a", "reserve", ns=ns, request_id="k2", payload_hash=payload_hash(ns, "k2", payload("k2")),
               ticket_id=str(uuid.uuid4()), epoch=2, incarnation="a-claims-2")
    att.check("SS-OLD-ISSUER-CANNOT-RESERVE", S, "DENIAL",
              "Old issuer A cannot start new issuance (gate: not epoch holder) and the witness refuses its reserve even if it claims epoch 2",
              rcode(s2) == "ISSUE_DENIED_GATE_CLOSED" and ra["status"] == "NOT_EPOCH_HOLDER"
              and key_state(att, W, ns, "k2") is None, {"worker": s2["result"], "direct": ra})
    s3 = issue(att, S, "sb-b-k5", W, "issuer_b", ns, "k5", epoch=2)
    att.check("SS-NEW-ISSUER-WORKS", S, "POSITIVE_CONTROL", "New holder B issues k5 under epoch 2", rcode(s3) == "ISSUE_OK", s3["result"])
    byp = pg_query(att.container, ISSUER_ROLE, att.db,
                   "INSERT INTO tickets (ticket_id, issuer_namespace_id, request_id, payload_hash, witness_seq, issued_by, issued_epoch) "
                   f"VALUES ('{uuid.uuid4()}', '{ns}', 'k9', '{'d' * 64}', 0, 'issuer_a', 1);", "a-bypass")
    s4 = issue(att, S, "sb-b-k4", W, "issuer_b", ns, "k4", epoch=2)
    att.check("SS-BYPASS-DETECTED-NOT-PREVENTED", S, "COUNTEREXAMPLE_REPRODUCED",
              "A writes ticket k9 straight into PostgreSQL with no reservation: PostgreSQL accepts it; B's next issuance sees "
              "TICKET_IN_PG_WITHOUT_RESERVATION and the gate stays CLOSED",
              byp["exit"] == 0 and rcode(s4) == "ISSUE_DENIED_GATE_CLOSED"
              and any("TICKET_IN_PG_WITHOUT_RESERVATION:k9" in x for x in s4["result"]["reasons"]),
              {"bypass": byp["stdout"].strip() or byp["stderr"], "b": s4["result"]})

    ns2 = "ns-sb-race"
    epoch(att, S, W, ns2, 1, "issuer_a")
    epoch(att, S, W, ns2, 2, "issuer_b")
    for rnd in range(1, 4):
        rid = f"race-{rnd}"
        start_at = time.time() + 4
        hs = [spawn(att, f"sbrace{rnd}-a{i}", worker_cfg(att, W, "issuer_a", ns2, rid, mode="reserve_only", epoch=1, start_at=start_at))
              for i in range(4)]
        hs += [spawn(att, f"sbrace{rnd}-b{i}", worker_cfg(att, W, "issuer_b", ns2, rid, mode="reserve_only", epoch=2, start_at=start_at))
               for i in range(4)]
        res = [finish(att, S, x) for x in hs]
        a = [rcode(r) for r in res if r["principal"] == "issuer_a"]
        b = [rcode(r) for r in res if r["principal"] == "issuer_b"]
        n_res = reserve_records(att, W, ns2, rid)
        att.check(f"SS-RACE-{rnd}", S, "POSITIVE_CONTROL",
                  f"Round {rnd}: 4 old-epoch A + 4 new-epoch B processes race on one key: A all NOT_EPOCH_HOLDER, exactly one B RESERVED",
                  all(x == "NOT_EPOCH_HOLDER" for x in a) and b.count("RESERVED") == 1 and b.count("DUPLICATE") == 3 and n_res == 1,
                  {"a": a, "b": b, "reserve_records": n_res})
        if n_res > 1:
            att.finding("F-DUP-RESERVE", "Witness accepted two reservations for one key", {"round": rnd})
            att.stop_scope = True
    stop_witness(att, W)


# --------------------------------------------------------------------------

def run_attempt(index: int, out: pathlib.Path) -> dict:
    att = Attempt(index, out)
    att.meta["t_start"] = now_iso()
    error = None
    try:
        start_pg(att)
        for fn in (sc_state_machine, sc_concurrency, sc_e2e, sc_lost_reserve_ack, sc_unavailable, sc_restore,
                   sc_split_brain, sc_corruption):
            if att.stop_scope:
                att.assertions.append({"id": f"SCOPE-STOP-{fn.__name__}", "scenario": "HARNESS", "kind": "NOT_RUN",
                                       "description": "Skipped after a duplicate-reservation finding", "observed": "skipped",
                                       "result": "NOT_RUN"})
                continue
            att.trace("_order", "scenario_start", name=fn.__name__)
            fn(att)
    except Exception as exc:
        error = f"{type(exc).__name__}: {exc}"
        att.check("HARNESS-ERROR", "HARNESS", "POSITIVE_CONTROL", "Harness completed without exception", False, error)
    finally:
        for name in list(att.witnesses):
            try:
                save_witness(att, name)
            except Exception as exc:  # recorded
                att.check(f"SAVE-{name}", "HARNESS", "POSITIVE_CONTROL", "witness evidence saved", False, str(exc))
        stop_pg(att)
        shutil.rmtree(att.tmp, ignore_errors=True)
        att.meta["witness_tmp_removed"] = not att.tmp.exists()
    att.check("HYGIENE-CONTAINER-REMOVED", "HYGIENE", "POSITIVE_CONTROL", "PostgreSQL container removed",
              att.meta.get("container_removed") is True, att.meta.get("container_removed"))
    att.check("HYGIENE-TMP-REMOVED", "HYGIENE", "POSITIVE_CONTROL", "Witness temp directory removed",
              att.meta.get("witness_tmp_removed") is True, att.meta.get("witness_tmp_removed"))
    att.meta["t_end"] = now_iso()
    att.meta["harness_error"] = error
    att.meta["findings"] = att.findings
    for sc, tr in att.traces.items():
        att.write(f"traces/{sc}.json", json.dumps(tr, indent=2, sort_keys=True, default=str) + "\n")
    leaks = [str(p.relative_to(att.dir)) for p in att.dir.rglob("*") if p.is_file()
             and any(s in p.read_text(encoding="utf-8", errors="replace") for s in att.secrets)]
    att.check("HYGIENE-SECRET-SCAN", "HYGIENE", "POSITIVE_CONTROL", "No generated key, pipe authkey, or password in any evidence file",
              not leaks, leaks or "none")
    att.write("assertions.json", json.dumps(att.assertions, indent=2, default=str) + "\n")
    att.write("run_meta.json", json.dumps(att.meta, indent=2, sort_keys=True, default=str) + "\n")
    counts: dict = {}
    for a in att.assertions:
        counts[a["result"]] = counts.get(a["result"], 0) + 1
    rows = ["| ID | Scenario | Kind | Result | Description |", "| --- | --- | --- | --- | --- |"]
    for a in att.assertions:
        rows.append(f"| `{a['id']}` | {a['scenario']} | {a['kind']} | {a['result']} | {a['description'].replace('|', '/')} |")
    att.write("RESULTS.md", f"# Attempt {index} results\n\n- PostgreSQL container: `{att.container}` (network `none`, no published "
              f"ports), removed: {att.meta.get('container_removed')}\n- Server: {att.meta.get('server_version')}\n"
              f"- Started {att.meta['t_start']}, finished {att.meta['t_end']}\n- Harness error: {error}\n"
              f"- Counts: {json.dumps(counts, sort_keys=True)}\n- Findings: {json.dumps([f['id'] for f in att.findings])}\n\n"
              "`COUNTEREXAMPLE_REPRODUCED` with PASS means the predicted unsafe or unproven behavior was observed.\n\n"
              + "\n".join(rows) + "\n")
    return {"attempt": index, "counts": counts, "failed": [a["id"] for a in att.assertions if a["result"] == "FAIL"],
            "findings": [f["id"] for f in att.findings], "container": att.container,
            "container_removed": att.meta.get("container_removed"), "server_version": att.meta.get("server_version"),
            "network_mode": att.meta.get("network_mode"), "port_bindings": att.meta.get("port_bindings"),
            "image_id": att.meta.get("image_id"), "harness_error": error}


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--attempts", type=int, default=3)
    ap.add_argument("--out", default=str(HERE / "attempts"))
    args = ap.parse_args()
    out = pathlib.Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    summary = []
    for i in range(1, args.attempts + 1):
        print(f"attempt {i} ...", flush=True)
        s = run_attempt(i, out)
        print(f"attempt {i}: {s['counts']} failed={s['failed']} findings={s['findings']}", flush=True)
        summary.append(s)
    (out / "summary.json").write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n", encoding="utf-8", newline="\n")
    return 0 if all(not s["failed"] for s in summary) else 1


if __name__ == "__main__":
    sys.exit(main())
