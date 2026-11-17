#!/usr/bin/env python3
"""BINTRANS 0.1S witness rollback-detection proof (orchestrator).

Per attempt: one disposable PostgreSQL 16 container (--network none, no
published ports), and pairs of test-only servers: a witness journal
(witness_server.py) and a separate external high-water authority
(hw_server.py), each with its own store directory, lock, keys and pipe authkey
under a fresh temporary directory outside the repository. Issuer requests run in
separate processes (issuer_worker.py). Only resources created here are removed.

Both servers run on the same machine: their independence is LOGICAL ONLY.
Nothing here proves an independent failure domain or power-loss durability.
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
from common import (GENESIS_HASH, WitnessClient, WitnessUnavailable, build_record, canon,  # noqa: E402
                    payload_hash, sha256_hex, signed_request, verify_chain)
from pgsession import pg_query  # noqa: E402

IMAGE = "postgres:16"
LABEL = "bintrans.proof=0.1s"
ADMIN = "wproof_admin"
ISSUER_ROLE = "wproof_issuer"
W_PRINCIPALS = ("issuer_a", "issuer_b", "operator", "auditor", "harness")
HW_PRINCIPALS = ("witness_svc", "issuer_a", "issuer_b", "operator", "auditor", "harness")
T_SCENARIOS = ("T1_CONSISTENT", "T2_LAST_RECORD_DELETED", "T3_TRUNCATED_TO_CHECKPOINT", "T4_RESTORED_ONE_BELOW",
               "T5_SAME_SEQ_OTHER_DIGEST", "T6_OTHER_INCARNATION", "T7_HW_UNAVAILABLE", "T8_HW_CORRUPTED",
               "T9_HW_LOWER_REJECTED", "T10_CONCURRENT_RESERVE", "J_WIT_001")


def now_iso() -> str:
    return dt.datetime.now(dt.timezone.utc).isoformat(timespec="microseconds")


def payload(rid: str, revision: int = 1) -> dict:
    return {"operation": "ISSUE_TRACKING_BINDING", "sub": "user-0.1s-test", "tenant_id": "tenant-0.1s-test",
            "shipment": f"shp-{rid}", "execution_ids": ["exe-2", "exe-1"], "expected_epoch": 1,
            "expected_revision": revision}


class Attempt:
    def __init__(self, index: int, out: pathlib.Path) -> None:
        self.index = index
        self.dir = out / f"attempt_{index}"
        self.dir.mkdir(parents=True, exist_ok=False)
        hx = secrets.token_hex(4)
        self.hex = hx
        self.container = f"bintrans-proof-0-1s-{hx}"
        self.db = f"wproof_{hx}"
        self.db_restore = f"wproof_r_{hx}"
        self.db_priv = f"wproof_p_{hx}"
        self.tmp = pathlib.Path(tempfile.mkdtemp(prefix=f"bintrans-0.1s-{hx}-"))
        self.keys = {p: secrets.token_hex(32) for p in W_PRINCIPALS}
        self.hw_keys = {p: secrets.token_hex(32) for p in HW_PRINCIPALS}
        self.authkey = secrets.token_hex(32)
        self.hw_authkey = secrets.token_hex(32)
        self.pg_password = secrets.token_urlsafe(24)
        self.secrets = (list(self.keys.values()) + list(self.hw_keys.values())
                        + [self.authkey, self.hw_authkey, self.pg_password])
        self.assertions: list[dict] = []
        self.traces: dict[str, list] = {}
        self.servers: dict[str, dict] = {}
        self.worker_no = 0
        self.findings: list[dict] = []
        self.stop_scope = False
        self.meta: dict = {"attempt": index, "container": self.container, "db": self.db, "db_restore": self.db_restore,
                           "db_priv": self.db_priv, "image": IMAGE, "network": "none", "published_ports": "none",
                           "server_tmp_root": "<per-attempt temp dir outside repository>",
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

    def not_run(self, aid: str, scenario: str, description: str, reason: str) -> None:
        self.assertions.append({"id": aid, "scenario": scenario, "kind": "NOT_RUN", "description": description,
                                "observed": reason, "result": "NOT_RUN"})

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
    admin_sql(att, att.db, f'CREATE DATABASE "{att.db_priv}";', "setup/03_priv_db")
    schema = (HERE / "sql" / "schema.sql").read_text(encoding="utf-8")
    for i, db in enumerate((att.db, att.db_restore, att.db_priv)):
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


def rids(rows: list[dict]) -> list[str]:
    return [r["request_id"] for r in rows]


def server_log_text(att: Attempt) -> str:
    logs = run(att, ["docker", "logs", att.container])
    return logs["stdout"] + logs["stderr"]


def txn_connections(att: Attempt, app: str) -> int:
    return len(re.findall(rf"connection authorized: .*application_name={re.escape(app)}-txn\b", server_log_text(att)))


def any_connections(att: Attempt, app: str) -> int:
    return len(re.findall(rf"connection authorized: .*application_name={re.escape(app)}-", server_log_text(att)))


def restore_db(att: Attempt, scenario: str, db: str, dump: str) -> dict:
    admin_sql(att, att.db, f"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '{db}';")
    r1 = run(att, ["docker", "exec", att.container, "dropdb", "-U", ADMIN, db], f"traces/{scenario}_dropdb")
    r2 = run(att, ["docker", "exec", att.container, "createdb", "-U", ADMIN, db], f"traces/{scenario}_createdb")
    r3 = run(att, ["docker", "exec", att.container, "pg_restore", "-U", ADMIN, "-d", db, dump],
             f"traces/{scenario}_pg_restore")
    return {"dropdb": r1["exit"], "createdb": r2["exit"], "pg_restore": r3["exit"]}


def dump_db(att: Attempt, scenario: str, db: str, dump: str) -> int:
    return run(att, ["docker", "exec", att.container, "pg_dump", "-U", ADMIN, "-Fc", "-d", db, "-f", dump],
               f"traces/{scenario}_pg_dump")["exit"]


# --------------------------------------------------------------------------
# Servers: witness journal and high-water authority
# --------------------------------------------------------------------------

def start_server(att: Attempt, name: str, kind: str, hw: str | None = None, order: str | None = None) -> dict:
    s = att.servers.setdefault(name, {"kind": kind, "runs": 0, "pipe": f"bintrans-0-1s-{att.hex}-{name}",
                                      "store": att.tmp / name / "store", "cp": att.tmp / name / "checkpoint",
                                      "hw": hw, "order": order or "append_first", "journal_id": f"journal-{name}"})
    if order:
        s["order"] = order
    s["runs"] += 1
    if kind == "hw":
        argv = [sys.executable, str(HERE / "hw_server.py"), "--store", str(s["store"]), "--pipe", s["pipe"]]
        env = att.env(HW_KEYS_JSON=json.dumps(att.hw_keys), HW_AUTHKEY=att.hw_authkey)
    else:
        argv = [sys.executable, str(HERE / "witness_server.py"), "--store", str(s["store"]), "--checkpoint-dir",
                str(s["cp"]), "--pipe", s["pipe"], "--hw-pipe", att.servers[s["hw"]]["pipe"],
                "--journal-id", s["journal_id"], "--order", s["order"]]
        env = att.env(WITNESS_KEYS_JSON=json.dumps(att.keys), WITNESS_AUTHKEY=att.authkey,
                      HW_AUTHKEY=att.hw_authkey, HW_KEY=att.hw_keys["witness_svc"])
    proc = subprocess.Popen(argv, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env)
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
    handle = {"proc": proc, "stdout": lines, "stderr": errs, "run": s["runs"]}
    s["current"] = handle
    s.setdefault("handles", []).append(handle)
    time.sleep(0.1)
    return handle


def pair(att: Attempt, base: str, order: str = "append_first") -> tuple[str, str]:
    W, H = f"{base}-w", f"{base}-hw"
    start_server(att, H, "hw")
    start_server(att, W, "witness", hw=H, order=order)
    return W, H


def cl(att: Attempt, name: str, principal: str) -> WitnessClient:
    s = att.servers[name]
    if s["kind"] == "hw":
        return WitnessClient(s["pipe"], att.hw_authkey, principal, att.hw_keys[principal])
    return WitnessClient(s["pipe"], att.authkey, principal, att.keys[principal])


def stop_server(att: Attempt, name: str, how: str = "shutdown") -> int | None:
    h = att.servers[name]["current"]
    if h["proc"].poll() is None:
        if how == "shutdown":
            try:
                cl(att, name, "harness").call("shutdown")
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


def restart(att: Attempt, name: str) -> dict:
    stop_server(att, name)
    s = att.servers[name]
    return start_server(att, name, s["kind"], s.get("hw"), s.get("order"))


def wait_exit(att: Attempt, name: str, timeout: float = 10) -> int | None:
    h = att.servers[name]["current"]
    try:
        h["proc"].wait(timeout)
    except subprocess.TimeoutExpired:
        return None
    time.sleep(0.2)
    return h["proc"].returncode


def server_events(att: Attempt, name: str, event: str) -> list[dict]:
    out = []
    for line in att.servers[name]["current"]["stdout"]:
        try:
            e = json.loads(line)
        except json.JSONDecodeError:
            continue
        if e.get("event") == event:
            out.append(e)
    return out


def save_servers(att: Attempt) -> None:
    for name, s in att.servers.items():
        for h in s.get("handles", []):
            if h["proc"].poll() is None:
                h["proc"].kill()
                h["proc"].wait()
            time.sleep(0.05)
            att.write(f"servers/{name}/server_run{h['run']}.stdout.txt", "\n".join(h["stdout"]) + "\n")
            att.write(f"servers/{name}/server_run{h['run']}.stderr.txt", "\n".join(h["stderr"]) + "\n")
            att.write(f"servers/{name}/server_run{h['run']}.exit", f"{h['proc'].returncode}\n")
        for f in ("witness_log.jsonl", "hw_log.jsonl"):
            p = s["store"] / f
            if p.exists():
                att.write(f"servers/{name}/{f}", p.read_text(encoding="utf-8", errors="replace"))
        cp = s["cp"] / "accepted_checkpoint.json"
        if cp.exists():
            att.write(f"servers/{name}/accepted_checkpoint.json", cp.read_text(encoding="utf-8"))


def call(att: Attempt, scenario: str, name: str, principal: str, op: str, **body) -> dict:
    try:
        r = cl(att, name, principal).call(op, **body)
    except WitnessUnavailable as exc:
        r = {"status": "UNAVAILABLE", "error": str(exc)}
    att.trace(scenario, "server_call", server=name, principal=principal, op=op, body=body,
              response={k: v for k, v in r.items() if k not in ("records",)})
    return r


def export(att: Attempt, W: str) -> dict:
    try:
        return cl(att, W, "auditor").call("export")
    except WitnessUnavailable as exc:
        return {"status": "UNAVAILABLE", "error": str(exc)}


def health(att: Attempt, scenario: str, name: str) -> dict:
    return call(att, scenario, name, "auditor", "health")


def hget(att: Attempt, H: str, W: str) -> dict | None:
    try:
        r = cl(att, H, "auditor").call("get", journal_id=att.servers[W]["journal_id"])
    except WitnessUnavailable:
        return None
    return r.get("mark")


def key_state(att: Attempt, W: str, ns: str, rid: str) -> str | None:
    k = (export(att, W).get("keys") or {}).get(f"{ns}|{rid}")
    return k["state"] if k else None


def jpath(att: Attempt, W: str) -> pathlib.Path:
    return att.servers[W]["store"] / "witness_log.jsonl"


def hpath(att: Attempt, H: str) -> pathlib.Path:
    return att.servers[H]["store"] / "hw_log.jsonl"


def file_lines(p: pathlib.Path) -> list[bytes]:
    return p.read_bytes().splitlines() if p.exists() else []


def put_lines(p: pathlib.Path, lines: list[bytes]) -> None:
    p.write_bytes(b"".join(l + b"\n" for l in lines))


def file_recs(p: pathlib.Path) -> list[dict]:
    return [json.loads(l) for l in file_lines(p)]


def reserve_records_in_file(att: Attempt, W: str, ns: str, rid: str) -> int:
    return sum(1 for r in file_recs(jpath(att, W)) if r.get("type") == "RESERVE" and r.get("ns") == ns
               and r.get("request_id") == rid)


def tamper_note(att: Attempt, name: str, tag: str, desc: dict) -> None:
    att.write(f"servers/{name}/tamper_{tag}.json", json.dumps(desc, indent=2, sort_keys=True) + "\n")


# direct witness operations ---------------------------------------------------

def epoch(att: Attempt, S: str, W: str, ns: str, ep: int, holder: str) -> dict:
    return call(att, S, W, "operator", "epoch_set", ns=ns, epoch=ep, holder=holder)


def dres(att: Attempt, S: str, W: str, ns: str, rid: str, principal: str = "issuer_a", ep: int = 1,
         inc: str = "direct") -> tuple[dict, str]:
    tid = str(uuid.uuid4())
    r = call(att, S, W, principal, "reserve", ns=ns, request_id=rid, payload_hash=payload_hash(ns, rid, payload(rid)),
             ticket_id=tid, epoch=ep, incarnation=inc)
    return r, tid


def dout(att: Attempt, S: str, W: str, principal: str, ns: str, rid: str, tid: str, outcome: str, ev: dict) -> dict:
    return call(att, S, W, principal, "outcome", ns=ns, request_id=rid, ticket_id=tid, outcome=outcome, evidence=ev)


def checkpoint_here(att: Attempt, S: str, W: str) -> dict:
    recs = export(att, W)["records"]
    return call(att, S, W, "operator", "checkpoint_accept", seq=len(recs), head_hash=recs[-1]["hash"])


def seed(att: Attempt, S: str, W: str, ns: str = "ns-t") -> dict:
    """Journal: 1 GENESIS, 2 EPOCH, 3 RESERVE k1, 4 OUTCOME k1 (checkpoint), 5 RESERVE k2, 6 OUTCOME k2."""
    e = epoch(att, S, W, ns, 1, "issuer_a")
    r1, t1 = dres(att, S, W, ns, "k1")
    o1 = dout(att, S, W, "issuer_a", ns, "k1", t1, "COMMIT_CONFIRMED", {"pg_commit_ack": True})
    cp = checkpoint_here(att, S, W)
    r2, t2 = dres(att, S, W, ns, "k2")
    backup = jpath(att, W).read_bytes()
    o2 = dout(att, S, W, "issuer_a", ns, "k2", t2, "COMMIT_CONFIRMED", {"pg_commit_ack": True})
    ok = (e["status"] == "EPOCH_SET" and r1["status"] == "RESERVED" and o1["status"] == "APPENDED"
          and cp["status"] == "CHECKPOINT_ACCEPTED" and r2["status"] == "RESERVED" and o2["status"] == "APPENDED")
    return {"ok": ok, "cp_seq": (cp.get("checkpoint") or {}).get("seq"), "k2_ticket": t2, "backup_at_seq5": backup,
            "statuses": [e["status"], r1["status"], o1["status"], cp["status"], r2["status"], o2["status"]]}


def forge_reserve(att: Attempt, prev_hash: str, seq: int, ns: str, rid: str, principal: str = "issuer_a") -> dict:
    tid = str(uuid.uuid4())
    ph = payload_hash(ns, rid, payload(rid))
    body = {"ns": ns, "request_id": rid, "payload_hash": ph, "ticket_id": tid, "epoch": 1, "incarnation": "forged"}
    req = signed_request(principal, att.keys[principal], "reserve", body)
    fields = {"type": "RESERVE", "ns": ns, "request_id": rid, "payload_hash": ph, "ticket_id": tid,
              "principal": principal, "epoch": 1, "incarnation": "forged"}
    return build_record(prev_hash, seq, fields, req)


def reincarnate(recs: list[dict], journal_id: str) -> list[dict]:
    out = [build_record(GENESIS_HASH, 1, {"type": "GENESIS", "journal_id": journal_id,
                                          "journal_incarnation": str(uuid.uuid4())}, None)]
    for r in recs[1:]:
        fields = {k: v for k, v in r.items() if k not in ("seq", "prev_hash", "ts", "request", "hash")}
        out.append(build_record(out[-1]["hash"], r["seq"], fields, r["request"]))
    return out


# --------------------------------------------------------------------------
# Issuer workers
# --------------------------------------------------------------------------

def worker_cfg(att: Attempt, W: str, principal: str, ns: str, rid: str, **kw) -> dict:
    s = att.servers[W]
    cfg = {"pipe": s["pipe"], "hw_pipe": att.servers[s["hw"]]["pipe"], "journal_id": s["journal_id"],
           "principal": principal, "epoch": kw.pop("epoch", 1), "incarnation": kw.pop("incarnation", str(uuid.uuid4())),
           "ns": ns, "request_id": rid, "payload": kw.pop("payload", payload(rid)), "fault": kw.pop("fault", ""),
           "container": att.container, "db": kw.pop("db", att.db), "pg_role": ISSUER_ROLE, "mode": kw.pop("mode", "issue")}
    cfg.update(kw)
    return cfg


def spawn(att: Attempt, label: str, cfg: dict) -> dict:
    att.worker_no += 1
    tag = f"w{att.worker_no:03d}-{label}"
    cfg.setdefault("app", f"a{att.index}-{tag}")
    env = att.env(WORKER_CONFIG=json.dumps(cfg), WITNESS_AUTHKEY=att.authkey, WITNESS_KEY=att.keys[cfg["principal"]],
                  HW_AUTHKEY=att.hw_authkey, HW_KEY=att.hw_keys[cfg["principal"]])
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


def issue(att: Attempt, scenario: str, label: str, W: str, principal: str, ns: str, rid: str, **kw) -> dict:
    return finish(att, scenario, spawn(att, label, worker_cfg(att, W, principal, ns, rid, **kw)))


def rcode(s: dict) -> str | None:
    return (s.get("result") or {}).get("result")


def no_pg(att: Attempt, s: dict) -> int:
    return any_connections(att, s["app"])


def reconcile(att: Attempt, scenario: str, W: str, ns: str, rid: str, db: str | None = None) -> dict:
    """Operator reconciliation for one IN_DOUBT key. Never writes ABORT_CONFIRMED."""
    e = export(att, W)
    k = e["keys"][f"{ns}|{rid}"]
    row = next((r for r in pg_rows(att, ns, db) if r["request_id"] == rid), None)
    auto_abort = call(att, scenario, W, "operator", "outcome", ns=ns, request_id=rid, ticket_id=k["ticket_id"],
                      outcome="ABORT_CONFIRMED", evidence={"reason": "tombstone absent", "pg_rollback_ack": True})
    if row and row["ticket_id"] == k["ticket_id"] and row["payload_hash"] == k["payload_hash"]:
        r = call(att, scenario, W, "operator", "outcome", ns=ns, request_id=rid, ticket_id=k["ticket_id"],
                 outcome="COMMIT_CONFIRMED", evidence={"tombstone_ticket_id": row["ticket_id"],
                                                       "tombstone_payload_hash": row["payload_hash"]})
        decision = "COMMIT_CONFIRMED"
    else:
        r = call(att, scenario, W, "operator", "outcome", ns=ns, request_id=rid, ticket_id=k["ticket_id"],
                 outcome="PERMANENTLY_DENIED",
                 evidence={"reason": "tombstone absent or different; abort cannot be proven", "pg_row": row})
        decision = "PERMANENTLY_DENIED"
    out = {"key": f"{ns}|{rid}", "witness_state_before": k["state"], "pg_row": row, "auto_abort": auto_abort["status"],
           "decision": decision, "append": r["status"]}
    att.trace(scenario, "reconcile", **out)
    return out


# --------------------------------------------------------------------------
# T1-T10: rollback detection
# --------------------------------------------------------------------------

def sc_t1(att: Attempt) -> None:
    S = "T1_CONSISTENT"
    W, H = pair(att, "t1")
    info = seed(att, S, W)
    restart(att, W)
    h = health(att, S, W)
    m = hget(att, H, W)
    dup, _ = dres(att, S, W, "ns-t", "k2")
    epoch(att, S, W, "ns-t1", 1, "issuer_a")
    s = issue(att, S, "t1-k3", W, "issuer_a", "ns-t1", "k3")
    hist = next((e for e in s["_events"] if e["event"] == "history_check"), {})
    att.check("T1-CONSISTENT", S, "POSITIVE_CONTROL",
              "Journal complete and equal to the high-water mark: witness CONSISTENT after restart, k2 cannot be reserved "
              "again (DUPLICATE), a new key in a fresh namespace is issued after the issuer's own history check",
              info["ok"] and h["status"] == "OK" and h["hw_state"] == "CONSISTENT" and m["seq"] == 6
              and h["records"] == 6 and dup["status"] == "DUPLICATE" and rcode(s) == "ISSUE_OK" and hist.get("complete") is True,
              {"seed": info["statuses"], "health": h, "high_water_seq": m["seq"], "dup": dup, "worker": s["result"],
               "history_check": hist})
    stop_server(att, W)
    stop_server(att, H)


def tamper_case(att: Attempt, S: str, aid: str, base: str, desc: str, mutate, expect: str,
                drop_checkpoint: bool = False) -> None:
    W, H = pair(att, base)
    info = seed(att, S, W)
    m0 = hget(att, H, W)
    stop_server(att, W)
    p = jpath(att, W)
    before = file_lines(p)
    note = mutate(p, before, info)
    after = file_lines(p)
    local = verify_chain([json.loads(l) for l in after])
    if drop_checkpoint:
        (att.servers[W]["cp"] / "accepted_checkpoint.json").unlink()
    tamper_note(att, W, base, {"description": desc, "lines_before": len(before), "lines_after": len(after),
                               "operator_checkpoint_deleted": drop_checkpoint,
                               "local_chain_check_on_tampered_journal": local or "VALID", **(note or {})})
    start_server(att, W, "witness", hw=H)
    h = health(att, S, W)
    rk2, _ = dres(att, S, W, "ns-t", "k2")
    s = issue(att, S, f"{base}-k2", W, "issuer_a", "ns-t", "k2")
    m1 = hget(att, H, W)
    final = file_lines(p)
    reason = h.get("reason") or ""
    att.check(f"{aid}-DETECTED", S, "DETECTION", f"{desc}: the witness refuses to serve with reason {expect}",
              info["ok"] and h["status"] == "WITNESS_FAILED" and reason.startswith(expect),
              {"health": h, "local_chain_check": local or "VALID"})
    att.check(f"{aid}-NO-DUPLICATE", S, "DENIAL",
              "Re-reserving k2 is refused (WITNESS_FAILED), nothing is appended, the mark is unchanged",
              rk2["status"] == "WITNESS_FAILED" and len(final) == len(after) and m1 == m0,
              {"reserve_k2": rk2, "journal_lines": [len(after), len(final)], "mark_unchanged": m1 == m0})
    att.check(f"{aid}-GATE-CLOSED", S, "DENIAL",
              "The issuer is denied before any PostgreSQL connection",
              rcode(s) == "ISSUE_DENIED_WITNESS_FAILED" and no_pg(att, s) == 0,
              {"worker": s["result"], "pg_connections": no_pg(att, s)})
    stop_server(att, W)
    stop_server(att, H)


def sc_t2_t6(att: Attempt) -> None:
    def drop_last(p, lines, info):
        put_lines(p, lines[:-1])
        return {"removed_seq": len(lines)}

    def to_checkpoint(p, lines, info):
        put_lines(p, lines[:info["cp_seq"]])
        return {"checkpoint_seq": info["cp_seq"]}

    def restore_backup(p, lines, info):
        p.write_bytes(info["backup_at_seq5"])
        return {"backup_taken_at_seq": 5}

    def fork(p, lines, info):
        recs = [json.loads(l) for l in lines[:5]]
        forged = forge_reserve(att, recs[-1]["hash"], 6, "ns-t", "k3")
        put_lines(p, lines[:5] + [canon(forged)])
        return {"replaced_seq": 6, "original_hash": json.loads(lines[5])["hash"], "forged_hash": forged["hash"]}

    def other_incarnation(p, lines, info):
        recs = reincarnate([json.loads(l) for l in lines], json.loads(lines[0])["journal_id"])
        put_lines(p, [canon(r) for r in recs])
        return {"original_incarnation": json.loads(lines[0])["journal_incarnation"],
                "forged_incarnation": recs[0]["journal_incarnation"]}

    tamper_case(att, "T2_LAST_RECORD_DELETED", "T2", "t2", "Last confirmed record (seq 6) deleted", drop_last,
                "ROLLBACK_DETECTED source=high_water")
    tamper_case(att, "T3_TRUNCATED_TO_CHECKPOINT", "T3", "t3",
                "Journal truncated exactly to the old operator checkpoint (seq 4)", to_checkpoint,
                "ROLLBACK_DETECTED source=high_water")
    tamper_case(att, "T4_RESTORED_ONE_BELOW", "T4", "t4",
                "Journal replaced by a byte copy taken at seq 5 (one below the mark)", restore_backup,
                "ROLLBACK_DETECTED source=high_water")
    tamper_case(att, "T5_SAME_SEQ_OTHER_DIGEST", "T5", "t5",
                "Journal with the same length but a validly signed, validly chained different record at seq 6", fork,
                "FORK_DETECTED source=high_water seq=6")
    tamper_case(att, "T6_OTHER_INCARNATION", "T6A", "t6a",
                "Journal rebuilt with the same records under a different GENESIS incarnation (valid chain); operator "
                "checkpoint left in place", other_incarnation, "FORK_DETECTED source=operator_checkpoint")
    tamper_case(att, "T6_OTHER_INCARNATION", "T6B", "t6b",
                "Journal rebuilt under a different GENESIS incarnation and the local operator checkpoint deleted too",
                other_incarnation, "INCARNATION_MISMATCH source=high_water", drop_checkpoint=True)


def sc_t7(att: Attempt) -> None:
    S = "T7_HW_UNAVAILABLE"
    W, H = pair(att, "t7")
    info = seed(att, S, W)
    stop_server(att, W)
    stop_server(att, H)
    start_server(att, W, "witness", hw=H)
    h1 = health(att, S, W)
    r1, _ = dres(att, S, W, "ns-t", "k3")
    s1 = issue(att, S, "t7-k3-hwdown", W, "issuer_a", "ns-t", "k3")
    n1 = len(file_lines(jpath(att, W)))
    att.check("T7-HW-DOWN-AT-START", S, "FAIL_CLOSED",
              "High-water authority down when the witness starts: witness HIGH_WATER_UNAVAILABLE, reserve refused without "
              "append, issuer denied before PostgreSQL",
              info["ok"] and h1["status"] == "HIGH_WATER_UNAVAILABLE" and r1["status"] == "HIGH_WATER_UNAVAILABLE" and n1 == 6
              and rcode(s1) == "ISSUE_DENIED_HIGH_WATER_UNAVAILABLE" and no_pg(att, s1) == 0,
              {"health": h1, "reserve": r1, "journal_lines": n1, "worker": s1["result"], "pg_connections": no_pg(att, s1)})
    start_server(att, H, "hw")
    h2 = health(att, S, W)
    att.check("T7-RECOVERS-WHEN-HW-RETURNS", S, "POSITIVE_CONTROL",
              "When the authority returns, the witness re-verifies and becomes CONSISTENT without restart",
              h2["status"] == "OK" and h2["hw_state"] == "CONSISTENT", h2)
    stop_server(att, H)
    s2 = issue(att, S, "t7-k6-issuer-reads-hw", W, "issuer_a", "ns-t", "k6")
    att.check("T7-ISSUER-OWN-HW-CHECK", S, "FAIL_CLOSED",
              "Witness still CONSISTENT in memory, authority down: the issuer's own high-water read fails and it is denied "
              "before PostgreSQL",
              rcode(s2) == "ISSUE_DENIED_HIGH_WATER_UNAVAILABLE" and "high_water_unavailable" in s2["events"]
              and no_pg(att, s2) == 0, {"worker": s2["result"], "events": s2["events"]})
    r2, t2 = dres(att, S, W, "ns-t", "k4")
    n2 = len(file_lines(jpath(att, W)))
    r3, _ = dres(att, S, W, "ns-t", "k5")
    n3 = len(file_lines(jpath(att, W)))
    att.check("T7-PREPARED-NOT-ACKNOWLEDGED", S, "FAIL_CLOSED",
              "Authority lost mid-operation: the k4 record is journaled (PREPARED) but the reply is HIGH_WATER_UNAVAILABLE, "
              "not RESERVED; the next write (k5) is refused without append",
              r2["status"] == "HIGH_WATER_UNAVAILABLE" and r2.get("durability") == "PREPARED" and n2 == 7
              and r3["status"] == "HIGH_WATER_UNAVAILABLE" and n3 == 7,
              {"k4": r2, "k5": r3, "journal_lines": [n2, n3]})
    start_server(att, H, "hw")
    h3 = health(att, S, W)
    time.sleep(0.2)
    rf = server_events(att, W, "roll_forward")
    m = hget(att, H, W)
    st = key_state(att, W, "ns-t", "k4")
    r4, _ = dres(att, S, W, "ns-t", "k4")
    att.check("T7-ROLL-FORWARD", S, "POSITIVE_CONTROL",
              "Authority back: the single PREPARED record chains from the pinned head and is rolled forward; k4 is RESERVED "
              "and cannot be reserved again",
              h3["status"] == "OK" and len(rf) == 1 and m["seq"] == 7 and st == "RESERVED" and r4["status"] == "DUPLICATE",
              {"health": h3, "roll_forward": rf, "high_water_seq": m["seq"], "k4": st, "reserve_again": r4})
    stop_server(att, W)
    stop_server(att, H)


def sc_t8(att: Attempt) -> None:
    S = "T8_HW_CORRUPTED"

    def flip(recs):
        line = recs[-1]
        line["mark_digest"] = ("0" if line["mark_digest"][0] != "0" else "1") + line["mark_digest"][1:]
        return line, "mark_digest changed, hash not recomputed"

    def rewrite(recs):
        line = recs[-1]
        line["mark_digest"] = "a" * 64
        line["hash"] = sha256_hex(canon({k: v for k, v in line.items() if k != "hash"}))
        return line, "mark_digest changed and record hash recomputed (signed request unchanged)"

    def resign(recs):
        line = recs[-1]
        line["mark_digest"] = "b" * 64
        line["request"]["body"]["digest"] = "b" * 64
        line["hash"] = sha256_hex(canon({k: v for k, v in line.items() if k != "hash"}))
        return line, "mark_digest and signed request body changed, record hash recomputed, MAC not recomputable"

    for tag, fn, expect in (("a", flip, "HW_HASH_MISMATCH"), ("b", rewrite, "HW_RECORD_REQUEST_MISMATCH"),
                            ("c", resign, "HW_REQUEST_MAC_INVALID")):
        W, H = pair(att, f"t8{tag}")
        info = seed(att, S, W)
        stop_server(att, W)
        stop_server(att, H)
        p = hpath(att, H)
        recs = file_recs(p)
        _, desc = fn(recs)
        put_lines(p, [canon(r) for r in recs])
        tamper_note(att, H, f"t8{tag}", {"description": desc, "record_seq": recs[-1]["seq"]})
        start_server(att, H, "hw")
        hh = health(att, S, H)
        start_server(att, W, "witness", hw=H)
        hw_ = health(att, S, W)
        r, _ = dres(att, S, W, "ns-t", "k2")
        s = issue(att, S, f"t8{tag}-k2", W, "issuer_a", "ns-t", "k2")
        att.check(f"T8{tag.upper()}-DETECTED", S, "DETECTION",
                  f"High-water store tampered ({desc}): the authority loads in FAILED mode with {expect}",
                  info["ok"] and hh["status"] == "FAILED" and (hh.get("reason") or "").startswith(expect), hh)
        att.check(f"T8{tag.upper()}-GATE-CLOSED", S, "FAIL_CLOSED",
                  "The witness reports HIGH_WATER_UNAVAILABLE, refuses k2 and the issuer is denied before PostgreSQL",
                  hw_["status"] == "HIGH_WATER_UNAVAILABLE" and r["status"] == "HIGH_WATER_UNAVAILABLE"
                  and rcode(s) == "ISSUE_DENIED_HIGH_WATER_UNAVAILABLE" and no_pg(att, s) == 0,
                  {"witness_health": hw_, "reserve": r, "worker": s["result"]})
        stop_server(att, W)
        stop_server(att, H)


def sc_t9(att: Attempt) -> None:
    S = "T9_HW_LOWER_REJECTED"
    W, H = pair(att, "t9")
    info = seed(att, S, W)
    jid = att.servers[W]["journal_id"]
    m = hget(att, H, W)
    n_before = health(att, S, H)["records"]
    inc = m["journal_incarnation"]
    succ = {"journal_id": jid, "incarnation": inc, "seq": m["seq"] + 1, "digest": "c" * 64, "prev_seq": m["seq"],
            "prev_digest": m["digest"]}
    res = {
        "lower_seq": call(att, S, H, "witness_svc", "advance", **{**succ, "seq": 3, "digest": "f" * 64}),
        "same_seq_other_digest": call(att, S, H, "witness_svc", "advance", **{**succ, "seq": m["seq"]}),
        "gap": call(att, S, H, "witness_svc", "advance", **{**succ, "seq": m["seq"] + 2}),
        "wrong_prev": call(att, S, H, "witness_svc", "advance", **{**succ, "prev_digest": "d" * 64}),
        "other_incarnation": call(att, S, H, "witness_svc", "advance", **{**succ, "incarnation": "other"}),
        "reinit": call(att, S, H, "witness_svc", "init", journal_id=jid, incarnation="other", seq=0, digest=GENESIS_HASH),
        "issuer_advance": call(att, S, H, "issuer_a", "advance", **succ),
        "operator_advance": call(att, S, H, "operator", "advance", **succ),
        "auditor_advance": call(att, S, H, "auditor", "advance", **succ),
        "issuer_init": call(att, S, H, "issuer_a", "init", journal_id="journal-x", incarnation="x", seq=0,
                            digest=GENESIS_HASH),
    }
    for op in ("set", "lower", "reset", "delete", "truncate", "rollback", "rewrite"):
        res[f"op_{op}"] = call(att, S, H, "witness_svc", op, journal_id=jid, seq=1, digest=GENESIS_HASH)
    res["idempotent_current"] = call(att, S, H, "witness_svc", "advance", **{**succ, "seq": m["seq"], "digest": m["digest"]})
    expected = {"lower_seq": "NOT_MONOTONIC", "same_seq_other_digest": "CONFLICT", "gap": "GAP",
                "wrong_prev": "PREV_MISMATCH", "other_incarnation": "INCARNATION_MISMATCH", "reinit": "ALREADY_INITIALIZED",
                "issuer_advance": "ROLE_FORBIDDEN", "operator_advance": "ROLE_FORBIDDEN", "auditor_advance": "ROLE_FORBIDDEN",
                "issuer_init": "ROLE_FORBIDDEN", "idempotent_current": "IDEMPOTENT",
                **{f"op_{op}": "UNSUPPORTED_OP" for op in ("set", "lower", "reset", "delete", "truncate", "rollback",
                                                          "rewrite")}}
    mism = {k: res[k]["status"] for k in expected if res[k]["status"] != expected[k]}
    m2 = hget(att, H, W)
    n_after = health(att, S, H)["records"]
    att.check("T9-LOWER-REJECTED", S, "DENIAL",
              "Every attempt to lower, overwrite, skip, fork, re-initialise or delete the mark is rejected; non-writer "
              "principals cannot write; the mark and the authority log are unchanged",
              info["ok"] and not mism and m2 == m and n_after == n_before,
              {"observed": {k: v["status"] for k, v in res.items()}, "mismatches": mism, "records": [n_before, n_after]})
    restart(att, W)
    h = health(att, S, W)
    r, _ = dres(att, S, W, "ns-t", "k3")
    att.check("T9-WITNESS-STILL-CONSISTENT", S, "POSITIVE_CONTROL",
              "After the rejected attempts the witness restarts CONSISTENT and serves a new reservation",
              h["status"] == "OK" and r["status"] == "RESERVED", {"health": h, "reserve": r})
    stop_server(att, W)
    stop_server(att, H)


def sc_t10(att: Attempt) -> None:
    S = "T10_CONCURRENT_RESERVE"
    W, H = pair(att, "t10")
    ns = "ns-c"
    epoch(att, S, W, ns, 1, "issuer_a")
    for rnd in range(1, 4):
        rid = f"race-{rnd}"
        start_at = time.time() + 4
        hs = [spawn(att, f"t10r{rnd}-{i}", worker_cfg(att, W, "issuer_a", ns, rid, mode="reserve_only", start_at=start_at))
              for i in range(8)]
        res = [rcode(finish(att, S, h)) for h in hs]
        n = reserve_records_in_file(att, W, ns, rid)
        m = hget(att, H, W)
        jl = len(file_lines(jpath(att, W)))
        ok = att.check(f"T10-RACE-{rnd}", S, "POSITIVE_CONTROL",
                       f"Round {rnd}: 8 processes reserve one key at once: exactly one RESERVED, seven DUPLICATE, one "
                       "RESERVE record, mark equal to journal length",
                       res.count("RESERVED") == 1 and res.count("DUPLICATE") == 7 and n == 1 and m["seq"] == jl,
                       {"results": res, "reserve_records": n, "high_water_seq": m["seq"], "journal_lines": jl})
        if not ok and n > 1:
            att.finding("F-DUP-RESERVE", "Two successful reservations of one key under concurrency", {"round": rnd})
            att.stop_scope = True
    first_w = att.servers[W]["current"]
    first_h = att.servers[H]["current"]
    second_h = start_server(att, H, "hw")
    second_h["proc"].wait(15)
    second_w = start_server(att, W, "witness", hw=H)
    second_w["proc"].wait(15)
    att.servers[H]["current"], att.servers[W]["current"] = first_h, first_w
    att.check("T10-SINGLE-WRITER", S, "DENIAL",
              "A second authority or witness process on the same store exits with store_locked (code 3)",
              second_h["proc"].returncode == 3 and second_w["proc"].returncode == 3
              and health(att, S, W)["status"] == "OK" and health(att, S, H)["status"] == "OK",
              {"hw_second_exit": second_h["proc"].returncode, "witness_second_exit": second_w["proc"].returncode})
    stop_server(att, W)
    stop_server(att, H)


# --------------------------------------------------------------------------
# J-WIT-001 regression
# --------------------------------------------------------------------------

def sc_jwit001(att: Attempt) -> None:
    S = "J_WIT_001"
    W, H = pair(att, "jw")
    ns = "ns-x"
    e = epoch(att, S, W, ns, 1, "issuer_a")                                                    # 1
    r1, t1 = dres(att, S, W, ns, "k1")                                                         # 2
    o1 = dout(att, S, W, "issuer_a", ns, "k1", t1, "COMMIT_CONFIRMED", {"pg_commit_ack": True})  # 3
    cp = checkpoint_here(att, S, W)
    r2, t2 = dres(att, S, W, ns, "k2")                                                         # 4
    o2 = dout(att, S, W, "issuer_a", ns, "k2", t2, "COMMIT_CONFIRMED", {"pg_commit_ack": True})  # 5
    m = hget(att, H, W)                                                                        # 6
    recs = file_recs(jpath(att, W))
    att.check("JW-SETUP", S, "POSITIVE_CONTROL",
              "Steps 1-6: epoch, k1 reserved+confirmed, operator checkpoint, k2 reserved+confirmed; the independent mark "
              "equals the journal head",
              e["status"] == "EPOCH_SET" and r1["status"] == "RESERVED" and o1["status"] == "APPENDED"
              and cp["status"] == "CHECKPOINT_ACCEPTED" and r2["status"] == "RESERVED" and o2["status"] == "APPENDED"
              and m["seq"] == len(recs) and m["digest"] == recs[-1]["hash"],
              {"statuses": [e["status"], r1["status"], o1["status"], cp["status"], r2["status"], o2["status"]],
               "checkpoint": cp.get("checkpoint"), "high_water": {"seq": m["seq"], "digest": m["digest"]},
               "journal_len": len(recs)})
    stop_server(att, W)                                                                        # 7
    p = jpath(att, W)
    lines = file_lines(p)
    cp_seq = cp["checkpoint"]["seq"]
    put_lines(p, lines[:cp_seq])
    trunc = [json.loads(l) for l in file_lines(p)]
    tamper_note(att, W, "jwit001", {"description": "journal truncated to the operator checkpoint",
                                    "lines_before": len(lines), "lines_after": len(trunc), "checkpoint_seq": cp_seq})
    att.check("JW-LOCAL-CHECKS-PASS-ON-TRUNCATED", S, "POSITIVE_CONTROL",
              "Control: the truncated journal is a valid chain that still satisfies the operator checkpoint, so the 0.1R "
              "local checks alone would accept it",
              verify_chain(trunc) is None and len(trunc) == cp_seq and trunc[-1]["hash"] == cp["checkpoint"]["head_hash"],
              {"chain": verify_chain(trunc) or "VALID", "len": len(trunc)})
    start_server(att, W, "witness", hw=H)                                                      # 8
    h = health(att, S, W)
    r9, _ = dres(att, S, W, ns, "k2")                                                          # 9
    s = issue(att, S, "jw-k2", W, "issuer_a", ns, "k2")
    s2 = issue(att, S, "jw-k7", W, "issuer_a", ns, "k7")
    detected = h["status"] == "WITNESS_FAILED" and (h.get("reason") or "").startswith("ROLLBACK_DETECTED source=high_water")
    accepted = r9["status"] == "RESERVED" or reserve_records_in_file(att, W, ns, "k2") > 0
    att.check("JW-TAIL-LOSS-DETECTED", S, "DETECTION", "TAIL_LOSS_DETECTED=YES after restart", detected, h)
    att.check("JW-DUPLICATE-NOT-ACCEPTED", S, "DENIAL",
              "DUPLICATE_RESERVATION_ACCEPTED=NO: reserve k2 is refused and the journal holds no k2 reservation",
              not accepted and r9["status"] == "WITNESS_FAILED",
              {"reserve_k2": r9, "k2_reserve_records": reserve_records_in_file(att, W, ns, "k2")})
    att.check("JW-GATE-CLOSED", S, "DENIAL",
              "ISSUER_ISSUANCE_GATE=CLOSED: the issuer is denied for k2 and for a new key, with no PostgreSQL connection",
              rcode(s) == "ISSUE_DENIED_WITNESS_FAILED" and rcode(s2) == "ISSUE_DENIED_WITNESS_FAILED"
              and no_pg(att, s) == 0 and no_pg(att, s2) == 0, {"k2": s["result"], "k7": s2["result"]})
    if accepted:
        att.finding("F-JWIT001-OPEN", "J-WIT-001 regression: the truncated witness accepted a second reservation of k2",
                    {"reserve": r9})
        att.stop_scope = True
    stop_server(att, W)
    stop_server(att, H)


# --------------------------------------------------------------------------
# Crash-window matrix
# --------------------------------------------------------------------------

def arm(att: Attempt, S: str, W: str, fault: str, on_type: str | None) -> dict:
    return call(att, S, W, "harness", "fault", fault=fault, on_type=on_type)


def sc_crash_windows(att: Attempt) -> None:
    # CW1 issuer stops before reservation
    S = "CW1_BEFORE_RESERVATION"
    W, H = pair(att, "cw1")
    ns = "ns-cw1"
    epoch(att, S, W, ns, 1, "issuer_a")
    n0 = len(file_lines(jpath(att, W)))
    s = issue(att, S, "cw1-crash", W, "issuer_a", ns, "k1", fault="crash_before_reserve")
    n1 = len(file_lines(jpath(att, W)))
    s2 = issue(att, S, "cw1-retry", W, "issuer_a", ns, "k1")
    att.check("CW1", S, "POSITIVE_CONTROL",
              "Issuer stops before reserve: no record, no PostgreSQL transaction; the key is still free and the retry issues",
              s["exit"] == 137 and n1 == n0 and txn_connections(att, s["app"]) == 0 and rcode(s2) == "ISSUE_OK",
              {"exit": s["exit"], "journal": [n0, n1], "retry": s2["result"]})
    stop_server(att, W)
    stop_server(att, H)

    # CW2 witness stops after preparing the reservation, before journal append
    S = "CW2_AFTER_PREPARE_BEFORE_APPEND"
    W, H = pair(att, "cw2")
    ns = "ns-cw2"
    epoch(att, S, W, ns, 1, "issuer_a")
    m0 = hget(att, H, W)
    n0 = len(file_lines(jpath(att, W)))
    arm(att, S, W, "crash_before_append", "RESERVE")
    s = issue(att, S, "cw2-k1", W, "issuer_a", ns, "k1")
    wx = wait_exit(att, W)
    n1 = len(file_lines(jpath(att, W)))
    m1 = hget(att, H, W)
    start_server(att, W, "witness", hw=H)
    h = health(att, S, W)
    st = key_state(att, W, ns, "k1")
    s2 = issue(att, S, "cw2-retry", W, "issuer_a", ns, "k1")
    att.check("CW2", S, "POSITIVE_CONTROL",
              "Witness stops after validating, before append: issuer sees an unknown outcome and opens no transaction; "
              "nothing was journaled or marked, so after restart the key is free and the retry issues",
              rcode(s) == "ISSUE_WITNESS_OUTCOME_UNKNOWN" and wx == 137 and n1 == n0 and m1 == m0
              and txn_connections(att, s["app"]) == 0 and h["status"] == "OK" and st is None and rcode(s2) == "ISSUE_OK",
              {"worker": s["result"], "witness_exit": wx, "journal": [n0, n1], "mark_unchanged": m1 == m0, "health": h,
               "retry": s2["result"]})
    stop_server(att, W)
    stop_server(att, H)

    # CW3 witness stops between journal append and high-water update (selected order)
    S = "CW3_BETWEEN_APPEND_AND_HW"
    W, H = pair(att, "cw3")
    ns = "ns-cw3"
    epoch(att, S, W, ns, 1, "issuer_a")
    arm(att, S, W, "crash_after_append_before_hw", "RESERVE")
    s = issue(att, S, "cw3-k1", W, "issuer_a", ns, "k1")
    wx = wait_exit(att, W)
    jl = len(file_lines(jpath(att, W)))
    m = hget(att, H, W)
    start_server(att, W, "witness", hw=H)
    h = health(att, S, W)
    time.sleep(0.2)
    rf = server_events(att, W, "roll_forward")
    st = key_state(att, W, ns, "k1")
    m2 = hget(att, H, W)
    att.check("CW3-NO-ISSUE-OK", S, "FAIL_CLOSED",
              "Crash between journal append and high-water update: the issuer never receives RESERVED, reports an unknown "
              "outcome and opens no PostgreSQL transaction; the journal is one record ahead of the mark",
              rcode(s) == "ISSUE_WITNESS_OUTCOME_UNKNOWN" and wx == 137 and jl == m["seq"] + 1
              and txn_connections(att, s["app"]) == 0,
              {"worker": s["result"], "witness_exit": wx, "journal_len": jl, "high_water_seq": m["seq"]})
    s2 = issue(att, S, "cw3-retry", W, "issuer_a", ns, "k1")
    s3 = issue(att, S, "cw3-other", W, "issuer_a", ns, "k2")
    rec = reconcile(att, S, W, ns, "k1")
    att.check("CW3-ROLL-FORWARD-NO-REISSUE", S, "DENIAL",
              "After restart the PREPARED record is rolled forward (mark = journal); k1 is RESERVED, the retry goes IN_DOUBT "
              "and is denied, other keys are gated, reconciliation cannot abort and ends PERMANENTLY_DENIED",
              h["status"] == "OK" and len(rf) == 1 and m2["seq"] == jl and st == "RESERVED"
              and rcode(s2) == "ISSUE_DENIED_IN_DOUBT" and rcode(s3) == "ISSUE_DENIED_GATE_CLOSED"
              and rec["auto_abort"] == "INVALID_TRANSITION" and rec["decision"] == "PERMANENTLY_DENIED"
              and rec["append"] == "APPENDED",
              {"health": h, "roll_forward": rf, "k1": st, "retry": s2["result"], "other": s3["result"], "reconcile": rec})
    stop_server(att, W)
    stop_server(att, H)

    # CW4 comparison order: high-water update before journal append
    S = "CW4_BETWEEN_HW_AND_APPEND"
    W, H = pair(att, "cw4", order="hw_first")
    ns = "ns-cw4"
    epoch(att, S, W, ns, 1, "issuer_a")
    s0 = issue(att, S, "cw4-k0", W, "issuer_a", ns, "k0")
    arm(att, S, W, "crash_after_hw_before_append", "RESERVE")
    s = issue(att, S, "cw4-k1", W, "issuer_a", ns, "k1")
    wx = wait_exit(att, W)
    jl = len(file_lines(jpath(att, W)))
    m = hget(att, H, W)
    start_server(att, W, "witness", hw=H, order="hw_first")
    h = health(att, S, W)
    s2 = issue(att, S, "cw4-retry", W, "issuer_a", ns, "k1")
    s3 = issue(att, S, "cw4-other", W, "issuer_a", ns, "k2")
    att.check("CW4-HW-FIRST-WORKS-NORMALLY", S, "POSITIVE_CONTROL", "Control: the hw_first order issues normally",
              rcode(s0) == "ISSUE_OK", s0["result"])
    att.check("CW4-FAIL-CLOSED-NOT-LIVE", S, "FAIL_CLOSED",
              "Crash between high-water update and journal append: no transaction; after restart the mark is ahead of "
              "the journal, which is indistinguishable from tail loss, so the witness stays FAILED and every issuance "
              "is denied (safe, but needs manual recovery; this is why append_first is selected)",
              rcode(s) == "ISSUE_WITNESS_OUTCOME_UNKNOWN" and wx == 137 and m["seq"] == jl + 1
              and txn_connections(att, s["app"]) == 0 and h["status"] == "WITNESS_FAILED"
              and (h.get("reason") or "").startswith("ROLLBACK_DETECTED source=high_water")
              and rcode(s2) == "ISSUE_DENIED_WITNESS_FAILED" and rcode(s3) == "ISSUE_DENIED_WITNESS_FAILED",
              {"worker": s["result"], "witness_exit": wx, "journal_len": jl, "high_water_seq": m["seq"], "health": h,
               "retry": s2["result"], "other": s3["result"]})
    stop_server(att, W)
    stop_server(att, H)

    # CW5 witness stops after the high-water update, before replying
    S = "CW5_AFTER_HW_BEFORE_REPLY"
    W, H = pair(att, "cw5")
    ns = "ns-cw5"
    epoch(att, S, W, ns, 1, "issuer_a")
    arm(att, S, W, "crash_after_hw_before_reply", "RESERVE")
    s = issue(att, S, "cw5-k1", W, "issuer_a", ns, "k1")
    wx = wait_exit(att, W)
    jl = len(file_lines(jpath(att, W)))
    m = hget(att, H, W)
    start_server(att, W, "witness", hw=H)
    h = health(att, S, W)
    time.sleep(0.2)
    rf = server_events(att, W, "roll_forward")
    st = key_state(att, W, ns, "k1")
    s2 = issue(att, S, "cw5-retry", W, "issuer_a", ns, "k1")
    rec = reconcile(att, S, W, ns, "k1")
    att.check("CW5", S, "FAIL_CLOSED",
              "Crash after the mark covers the reservation, before the reply: issuer unknown outcome, no transaction; after "
              "restart the journal equals the mark (no roll-forward), k1 RESERVED -> IN_DOUBT -> PERMANENTLY_DENIED",
              rcode(s) == "ISSUE_WITNESS_OUTCOME_UNKNOWN" and wx == 137 and jl == m["seq"]
              and txn_connections(att, s["app"]) == 0 and h["status"] == "OK" and not rf and st == "RESERVED"
              and rcode(s2) == "ISSUE_DENIED_IN_DOUBT" and rec["auto_abort"] == "INVALID_TRANSITION"
              and rec["decision"] == "PERMANENTLY_DENIED",
              {"worker": s["result"], "journal_len": jl, "high_water_seq": m["seq"], "k1": st, "retry": s2["result"],
               "reconcile": rec})
    stop_server(att, W)
    stop_server(att, H)

    # CW6 issuer stops after PostgreSQL COMMIT
    S = "CW6_AFTER_PG_COMMIT"
    W, H = pair(att, "cw6")
    ns = "ns-cw6"
    epoch(att, S, W, ns, 1, "issuer_a")
    s = issue(att, S, "cw6-k1", W, "issuer_a", ns, "k1", fault="crash_after_commit")
    rows = rids(pg_rows(att, ns))
    st = key_state(att, W, ns, "k1")
    s2 = issue(att, S, "cw6-retry", W, "issuer_a", ns, "k1")
    rec = reconcile(att, S, W, ns, "k1")
    s3 = issue(att, S, "cw6-replay", W, "issuer_a", ns, "k1")
    att.check("CW6", S, "POSITIVE_CONTROL",
              "Issuer stops after COMMIT, before outcome: row exists, key RESERVED; retry IN_DOUBT; reconciliation against "
              "the tombstone gives COMMIT_CONFIRMED; then the retry replays the same ticket",
              s["exit"] == 137 and rows == ["k1"] and st == "RESERVED" and rcode(s2) == "ISSUE_DENIED_IN_DOUBT"
              and rec["decision"] == "COMMIT_CONFIRMED" and rec["auto_abort"] == "INVALID_TRANSITION"
              and rcode(s3) == "ISSUE_REPLAY_ISSUED",
              {"rows": rows, "k1": st, "retry": s2["result"], "reconcile": rec, "replay": s3["result"]})
    stop_server(att, W)
    stop_server(att, H)

    # CW7 witness stops before / after the outcome append
    for tag, fault in (("a", "crash_before_append"), ("b", "crash_after_append_before_hw")):
        S = f"CW7{tag.upper()}_OUTCOME_{'BEFORE_APPEND' if tag == 'a' else 'AFTER_APPEND_BEFORE_HW'}"
        W, H = pair(att, f"cw7{tag}")
        ns = f"ns-cw7{tag}"
        epoch(att, S, W, ns, 1, "issuer_a")
        arm(att, S, W, fault, "OUTCOME")
        s = issue(att, S, f"cw7{tag}-k1", W, "issuer_a", ns, "k1")
        wx = wait_exit(att, W)
        rows = rids(pg_rows(att, ns))
        start_server(att, W, "witness", hw=H)
        h = health(att, S, W)
        time.sleep(0.2)
        rf = server_events(att, W, "roll_forward")
        st = key_state(att, W, ns, "k1")
        if tag == "a":
            s2 = issue(att, S, "cw7a-retry", W, "issuer_a", ns, "k1")
            rec = reconcile(att, S, W, ns, "k1")
            s3 = issue(att, S, "cw7a-replay", W, "issuer_a", ns, "k1")
            att.check("CW7A", S, "FAIL_CLOSED",
                      "Witness stops before appending COMMIT_CONFIRMED: issuer reports ISSUE_OUTCOME_UNKNOWN (never ISSUE_OK); "
                      "row exists, key RESERVED -> IN_DOUBT; reconciliation COMMIT_CONFIRMED; replay of the same ticket",
                      rcode(s) == "ISSUE_OUTCOME_UNKNOWN" and wx == 137 and rows == ["k1"] and h["status"] == "OK"
                      and not rf and st == "RESERVED" and rcode(s2) == "ISSUE_DENIED_IN_DOUBT"
                      and rec["decision"] == "COMMIT_CONFIRMED" and rcode(s3) == "ISSUE_REPLAY_ISSUED",
                      {"worker": s["result"], "rows": rows, "k1": st, "retry": s2["result"], "reconcile": rec,
                       "replay": s3["result"]})
        else:
            s2 = issue(att, S, "cw7b-replay", W, "issuer_a", ns, "k1")
            att.check("CW7B", S, "FAIL_CLOSED",
                      "Witness stops after appending COMMIT_CONFIRMED, before the mark: issuer reports ISSUE_OUTCOME_UNKNOWN; "
                      "after restart the outcome is rolled forward and the retry replays the same ticket",
                      rcode(s) == "ISSUE_OUTCOME_UNKNOWN" and wx == 137 and rows == ["k1"] and h["status"] == "OK"
                      and len(rf) == 1 and st == "COMMIT_CONFIRMED" and rcode(s2) == "ISSUE_REPLAY_ISSUED",
                      {"worker": s["result"], "rows": rows, "k1": st, "roll_forward": rf, "replay": s2["result"]})
        stop_server(att, W)
        stop_server(att, H)

    # CW8 stop during reconciliation
    S = "CW8_DURING_RECONCILIATION"
    W, H = pair(att, "cw8")
    ns = "ns-cw8"
    epoch(att, S, W, ns, 1, "issuer_a")
    s = issue(att, S, "cw8-k1", W, "issuer_a", ns, "k1", fault="crash_after_reserve")
    s1 = issue(att, S, "cw8-retry", W, "issuer_a", ns, "k1")
    k = export(att, W)["keys"][f"{ns}|k1"]
    row = next((r for r in pg_rows(att, ns) if r["request_id"] == "k1"), None)
    att.trace(S, "reconciler_stopped_after_read", key=k, pg_row=row)
    s2 = issue(att, S, "cw8-gate", W, "issuer_a", ns, "k2")
    att.check("CW8-RECONCILER-STOPS-BEFORE-APPEND", S, "FAIL_CLOSED",
              "Reconciler reads state and PostgreSQL, then stops before writing: the key stays IN_DOUBT and the gate stays "
              "closed",
              s["exit"] == 137 and rcode(s1) == "ISSUE_DENIED_IN_DOUBT" and k["state"] == "IN_DOUBT" and row is None
              and rcode(s2) == "ISSUE_DENIED_GATE_CLOSED", {"k1": k["state"], "gate": s2["result"]})
    arm(att, S, W, "crash_after_append_before_hw", "OUTCOME")
    rec1 = reconcile(att, S, W, ns, "k1")
    wx = wait_exit(att, W)
    start_server(att, W, "witness", hw=H)
    h = health(att, S, W)
    time.sleep(0.2)
    rf = server_events(att, W, "roll_forward")
    st = key_state(att, W, ns, "k1")
    rec2 = reconcile(att, S, W, ns, "k1")
    s3 = issue(att, S, "cw8-after", W, "issuer_a", ns, "k1")
    att.check("CW8-WITNESS-STOPS-DURING-DECISION", S, "FAIL_CLOSED",
              "Witness stops after journaling the operator's PERMANENTLY_DENIED, before the mark: the reconciler sees no "
              "reply; after restart the decision is rolled forward; re-running reconciliation is idempotent and never "
              "aborts; k1 stays denied",
              rec1["auto_abort"] == "INVALID_TRANSITION" and rec1["append"] == "UNAVAILABLE" and wx == 137
              and h["status"] == "OK" and len(rf) == 1 and st == "PERMANENTLY_DENIED"
              and rec2["auto_abort"] == "INVALID_TRANSITION" and rec2["append"] == "IDEMPOTENT"
              and rcode(s3) == "ISSUE_DENIED_PERMANENTLY",
              {"first": rec1, "witness_exit": wx, "roll_forward": rf, "k1": st, "second": rec2, "retry": s3["result"]})
    stop_server(att, W)
    stop_server(att, H)


# --------------------------------------------------------------------------
# Combined: witness tail loss + stale PostgreSQL restore
# --------------------------------------------------------------------------

def sc_stale_pg_combined(att: Attempt) -> None:
    S = "STALE_PG_COMBINED"
    failed = [a["id"] for a in att.assertions if a["scenario"] in T_SCENARIOS and a["result"] != "PASS"]
    if failed:
        att.not_run("SP-ALL", S, "Combined tail-loss + stale PostgreSQL restore test",
                    f"precondition not met: T1-T10/J-WIT-001 not all PASS ({failed})")
        return
    W, H = pair(att, "sp")
    ns, db = "ns-sp", att.db_restore
    epoch(att, S, W, ns, 1, "issuer_a")
    s1 = issue(att, S, "sp-k1", W, "issuer_a", ns, "k1", db=db)
    d = dump_db(att, S, db, "/tmp/sp0.dump")
    cp = checkpoint_here(att, S, W)
    s2 = issue(att, S, "sp-k2", W, "issuer_a", ns, "k2", db=db)
    t2 = (s2.get("result") or {}).get("ticket_id")
    full_copy = jpath(att, W).read_bytes()
    stop_server(att, W)
    lines = file_lines(jpath(att, W))
    put_lines(jpath(att, W), lines[:cp["checkpoint"]["seq"]])
    tamper_note(att, W, "sp", {"description": "journal truncated to checkpoint and PostgreSQL restored from an older dump",
                               "lines_before": len(lines), "lines_after": cp["checkpoint"]["seq"]})
    rr = restore_db(att, S, db, "/tmp/sp0.dump")
    rows_after = rids(pg_rows(att, ns, db))
    start_server(att, W, "witness", hw=H)
    h = health(att, S, W)
    r2, _ = dres(att, S, W, ns, "k2")
    s3 = issue(att, S, "sp-k2-reuse", W, "issuer_a", ns, "k2", db=db)
    s4 = issue(att, S, "sp-k3-new", W, "issuer_a", ns, "k3", db=db)
    rows_final = pg_rows(att, ns, db)
    att.check("SP-SETUP", S, "POSITIVE_CONTROL",
              "k1 issued, dump taken, checkpoint, k2 issued; after the stale restore PostgreSQL holds only k1",
              rcode(s1) == "ISSUE_OK" and d == 0 and cp["status"] == "CHECKPOINT_ACCEPTED" and rcode(s2) == "ISSUE_OK"
              and rr == {"dropdb": 0, "createdb": 0, "pg_restore": 0} and rows_after == ["k1"],
              {"k1": s1["result"], "k2": s2["result"], "restore": rr, "rows_after_restore": rows_after})
    att.check("SP-TAIL-LOSS-DETECTED", S, "DETECTION", "Witness tail loss detected against the high-water mark",
              h["status"] == "WITNESS_FAILED" and (h.get("reason") or "").startswith("ROLLBACK_DETECTED source=high_water"), h)
    att.check("SP-NO-SECOND-RESERVED", S, "DENIAL", "A second reservation of k2 is refused",
              r2["status"] == "WITNESS_FAILED" and reserve_records_in_file(att, W, ns, "k2") == 0, r2)
    att.check("SP-NO-SECOND-TICKET", S, "DENIAL",
              "Reusing k2 and issuing a new key are denied before PostgreSQL; no second ticket exists",
              rcode(s3) == "ISSUE_DENIED_WITNESS_FAILED" and rcode(s4) == "ISSUE_DENIED_WITNESS_FAILED"
              and no_pg(att, s3) == 0 and no_pg(att, s4) == 0 and rids(rows_final) == ["k1"],
              {"k2": s3["result"], "k3": s4["result"], "rows": rids(rows_final)})
    stop_server(att, W)
    jpath(att, W).write_bytes(full_copy)
    tamper_note(att, W, "sp-recovery", {"description": "authorised recovery: complete journal copy put back"})
    start_server(att, W, "witness", hw=H)
    h2 = health(att, S, W)
    s5 = issue(att, S, "sp-k2-after-recovery", W, "issuer_a", ns, "k2", db=db)
    s6 = issue(att, S, "sp-k3-after-recovery", W, "issuer_a", ns, "k3", db=db)
    att.check("SP-GATE-CLOSED-UNTIL-RECONCILED", S, "DENIAL",
              "After the complete journal is restored the witness is CONSISTENT again, but issuance stays closed: k2 needs "
              "reconciliation (confirmed ticket missing in the stale database) and new keys are gated",
              h2["status"] == "OK" and rcode(s5) == "ISSUE_DENIED_RECONCILIATION_REQUIRED"
              and rcode(s6) == "ISSUE_DENIED_GATE_CLOSED"
              and any(x.startswith("CONFIRMED_TICKET_MISSING_OR_DIFFERENT_IN_PG:k2") for x in s6["result"]["reasons"])
              and rids(pg_rows(att, ns, db)) == ["k1"] and reserve_records_in_file(att, W, ns, "k2") == 1,
              {"health": h2, "k2": s5["result"], "k3": s6["result"], "k2_original_ticket": t2})
    stop_server(att, W)
    stop_server(att, H)


# --------------------------------------------------------------------------
# Privileged bypass
# --------------------------------------------------------------------------

def sc_privileged(att: Attempt) -> None:
    # P1 journal writer with all witness keys, extending past the mark
    S = "P1_JOURNAL_ONLY"
    W, H = pair(att, "p1")
    info = seed(att, S, W)
    stop_server(att, W)
    lines = file_lines(jpath(att, W))
    recs = [json.loads(l) for l in lines[:4]]
    for i, rid in enumerate(("k3", "k4", "k5")):
        recs.append(forge_reserve(att, recs[-1]["hash"], 5 + i, "ns-t", rid))
    put_lines(jpath(att, W), [canon(r) for r in recs])
    tamper_note(att, W, "p1", {"description": "truncate to seq 4, append 3 forged valid records (journal longer than mark)",
                               "lines_after": len(recs)})
    start_server(att, W, "witness", hw=H)
    h = health(att, S, W)
    r, _ = dres(att, S, W, "ns-t", "k2")
    att.check("P1-DETECTED", S, "DETECTION",
              "Attacker with journal write access and every witness key replaces the tail with a longer valid chain: the "
              "pinned digest at seq 5 no longer matches -> FORK_DETECTED; k2 cannot be reserved",
              info["ok"] and h["status"] == "WITNESS_FAILED" and (h.get("reason") or "").startswith("FORK_DETECTED source=high_water seq=5")
              and r["status"] == "WITNESS_FAILED", {"health": h, "reserve_k2": r})
    stop_server(att, W)
    stop_server(att, H)

    # P2 high-water store only
    S = "P2_HIGH_WATER_ONLY"
    W, H = pair(att, "p2a")
    info = seed(att, S, W)
    stop_server(att, W)
    stop_server(att, H)
    hl = file_lines(hpath(att, H))
    put_lines(hpath(att, H), hl[:4])
    tamper_note(att, H, "p2a", {"description": "authority log rolled back to its first 4 records (mark seq 3)",
                                "lines_before": len(hl)})
    start_server(att, H, "hw")
    start_server(att, W, "witness", hw=H)
    h = health(att, S, W)
    r, _ = dres(att, S, W, "ns-t", "k2")
    att.check("P2A-HW-ROLLBACK-FAILS-CLOSED", S, "FAIL_CLOSED",
              "Authority store rolled back by more than one record: the journal is ahead by an unacknowledged-looking "
              "suffix that cannot be rolled forward -> UNCOVERED_SUFFIX, witness refuses",
              info["ok"] and h["status"] == "WITNESS_FAILED" and (h.get("reason") or "").startswith("UNCOVERED_SUFFIX")
              and r["status"] == "WITNESS_FAILED", {"health": h, "reserve_k2": r})
    stop_server(att, W)
    stop_server(att, H)
    W, H = pair(att, "p2b")
    info = seed(att, S, W)
    stop_server(att, W)
    stop_server(att, H)
    hpath(att, H).unlink()
    tamper_note(att, H, "p2b", {"description": "authority log deleted"})
    start_server(att, H, "hw")
    start_server(att, W, "witness", hw=H)
    h = health(att, S, W)
    m = hget(att, H, W)
    att.check("P2B-HW-WIPED-NO-REINIT", S, "FAIL_CLOSED",
              "Authority store deleted: the witness with a non-empty journal refuses (HIGH_WATER_MISSING) and does not "
              "re-initialise the mark",
              info["ok"] and h["status"] == "WITNESS_FAILED" and (h.get("reason") or "").startswith("HIGH_WATER_MISSING")
              and m is None, {"health": h, "mark": m})
    stop_server(att, W)
    stop_server(att, H)

    # P3 both stores rolled back together
    S = "P3_BOTH_ROLLED_BACK"
    W, H = pair(att, "p3")
    ns = "ns-p3"
    epoch(att, S, W, ns, 1, "issuer_a")
    s1 = issue(att, S, "p3-k1", W, "issuer_a", ns, "k1")
    snap_j, snap_h = jpath(att, W).read_bytes(), hpath(att, H).read_bytes()
    s2 = issue(att, S, "p3-k2", W, "issuer_a", ns, "k2")
    stop_server(att, W)
    stop_server(att, H)
    jpath(att, W).write_bytes(snap_j)
    hpath(att, H).write_bytes(snap_h)
    tamper_note(att, W, "p3", {"description": "journal and authority log both restored to the same earlier snapshot"})
    start_server(att, H, "hw")
    start_server(att, W, "witness", hw=H)
    h = health(att, S, W)
    s3 = issue(att, S, "p3-k2-again", W, "issuer_a", ns, "k2")
    r, _ = dres(att, S, W, ns, "k2")
    att.check("P3A-UNDETECTED-BY-WITNESS", S, "COUNTEREXAMPLE_REPRODUCED",
              "Both stores rolled back consistently: the witness is CONSISTENT and accepts a second successful reservation "
              "of k2 (outside the model: the authority itself must not be rollback-able)",
              rcode(s1) == "ISSUE_OK" and rcode(s2) == "ISSUE_OK" and h["status"] == "OK" and r["status"] == "RESERVED",
              {"health": h, "reserve_k2": r})
    att.check("P3A-PG-ROW-STILL-GATES", S, "DENIAL",
              "PostgreSQL was not rolled back: the issuer's gate sees k2's row without a reservation and refuses",
              rcode(s3) == "ISSUE_DENIED_GATE_CLOSED"
              and "TICKET_IN_PG_WITHOUT_RESERVATION:k2" in s3["result"].get("reasons", []), s3["result"])
    stop_server(att, W)
    stop_server(att, H)

    W, H = pair(att, "p3b")
    ns, db = "ns-p3b", att.db_priv
    epoch(att, S, W, ns, 1, "issuer_a")
    s1 = issue(att, S, "p3b-k1", W, "issuer_a", ns, "k1", db=db)
    snap_j, snap_h = jpath(att, W).read_bytes(), hpath(att, H).read_bytes()
    d = dump_db(att, "P3B", db, "/tmp/p3b.dump")
    s2 = issue(att, S, "p3b-k2", W, "issuer_a", ns, "k2", db=db)
    t_first = (s2.get("result") or {}).get("ticket_id")
    stop_server(att, W)
    stop_server(att, H)
    jpath(att, W).write_bytes(snap_j)
    hpath(att, H).write_bytes(snap_h)
    rr = restore_db(att, "P3B", db, "/tmp/p3b.dump")
    tamper_note(att, W, "p3b", {"description": "journal, authority log and PostgreSQL all restored to the same earlier point"})
    start_server(att, H, "hw")
    start_server(att, W, "witness", hw=H)
    s3 = issue(att, S, "p3b-k2-again", W, "issuer_a", ns, "k2", db=db)
    t_second = (s3.get("result") or {}).get("ticket_id")
    att.check("P3B-SECOND-TICKET", S, "COUNTEREXAMPLE_REPRODUCED",
              "Journal, authority and PostgreSQL all rolled back to the same point: k2 is issued again with a different "
              "ticket id. No local mechanism detects a consistent rollback of every store",
              rcode(s1) == "ISSUE_OK" and rcode(s2) == "ISSUE_OK" and d == 0 and rr == {"dropdb": 0, "createdb": 0, "pg_restore": 0}
              and rcode(s3) == "ISSUE_OK" and t_second != t_first,
              {"first_ticket": t_first, "second_ticket": t_second, "restore": rr})
    stop_server(att, W)
    stop_server(att, H)

    # P4 stolen witness_svc credential + journal write access
    S = "P4_WITNESS_CREDENTIAL"
    W, H = pair(att, "p4")
    info = seed(att, S, W)
    m = hget(att, H, W)
    stop_server(att, W)
    lines = file_lines(jpath(att, W))
    recs = [json.loads(l) for l in lines[:4]]
    for i, rid in enumerate(("k3", "k4", "k5")):
        recs.append(forge_reserve(att, recs[-1]["hash"], 5 + i, "ns-t", rid))
    put_lines(jpath(att, W), [canon(r) for r in recs])
    adv = call(att, S, H, "witness_svc", "advance", journal_id=att.servers[W]["journal_id"],
               incarnation=m["journal_incarnation"], seq=7, digest=recs[-1]["hash"], prev_seq=m["seq"],
               prev_digest=m["digest"])
    low = call(att, S, H, "witness_svc", "advance", journal_id=att.servers[W]["journal_id"],
               incarnation=m["journal_incarnation"], seq=4, digest="9" * 64, prev_seq=3, prev_digest=recs[2]["hash"])
    tamper_note(att, W, "p4", {"description": "journal tail replaced by forged valid records; mark advanced to the forged "
                                              "head with the stolen witness_svc credential",
                               "advance": adv["status"], "lower": low["status"]})
    start_server(att, W, "witness", hw=H)
    h = health(att, S, W)
    r, _ = dres(att, S, W, "ns-t", "k2")
    att.check("P4-HISTORY-PIN-DETECTS", S, "DETECTION",
              "With the witness credential the attacker can advance the mark to a forged head (the authority cannot check "
              "content) but cannot lower it; the earlier pinned digests still expose the rewrite (FORK_DETECTED seq=5)",
              info["ok"] and adv["status"] == "ADVANCED" and low["status"] == "NOT_MONOTONIC"
              and h["status"] == "WITNESS_FAILED" and (h.get("reason") or "").startswith("FORK_DETECTED source=high_water seq=5")
              and r["status"] == "WITNESS_FAILED", {"advance": adv["status"], "lower": low["status"], "health": h})
    stop_server(att, W)
    stop_server(att, H)


# --------------------------------------------------------------------------

def run_attempt(index: int, out: pathlib.Path) -> dict:
    att = Attempt(index, out)
    att.meta["t_start"] = now_iso()
    error = None
    try:
        start_pg(att)
        for fn in (sc_t1, sc_t2_t6, sc_t7, sc_t8, sc_t9, sc_t10, sc_jwit001, sc_crash_windows, sc_stale_pg_combined,
                   sc_privileged):
            if att.stop_scope:
                att.not_run(f"SCOPE-STOP-{fn.__name__}", "HARNESS", fn.__name__,
                            "skipped after a duplicate-reservation finding")
                continue
            att.trace("_order", "scenario_start", name=fn.__name__)
            fn(att)
    except Exception as exc:
        error = f"{type(exc).__name__}: {exc}"
        att.check("HARNESS-ERROR", "HARNESS", "POSITIVE_CONTROL", "Harness completed without exception", False, error)
    finally:
        try:
            save_servers(att)
        except Exception as exc:  # recorded
            att.check("SAVE-SERVERS", "HARNESS", "POSITIVE_CONTROL", "server evidence saved", False, str(exc))
        stop_pg(att)
        shutil.rmtree(att.tmp, ignore_errors=True)
        att.meta["server_tmp_removed"] = not att.tmp.exists()
    att.check("HYGIENE-CONTAINER-REMOVED", "HYGIENE", "POSITIVE_CONTROL", "PostgreSQL container removed",
              att.meta.get("container_removed") is True, att.meta.get("container_removed"))
    att.check("HYGIENE-TMP-REMOVED", "HYGIENE", "POSITIVE_CONTROL", "Server temp directory removed",
              att.meta.get("server_tmp_removed") is True, att.meta.get("server_tmp_removed"))
    att.meta["t_end"] = now_iso()
    att.meta["harness_error"] = error
    att.meta["findings"] = att.findings
    for sc, tr in att.traces.items():
        att.write(f"traces/{sc}.json", json.dumps(tr, indent=2, sort_keys=True, default=str) + "\n")
    leaks = [str(p.relative_to(att.dir)) for p in att.dir.rglob("*") if p.is_file()
             and any(s in p.read_text(encoding="utf-8", errors="replace") for s in att.secrets)]
    att.check("HYGIENE-SECRET-SCAN", "HYGIENE", "POSITIVE_CONTROL",
              "No generated key, pipe authkey, or password in any evidence file", not leaks, leaks or "none")
    att.write("assertions.json", json.dumps(att.assertions, indent=2, default=str) + "\n")
    att.write("run_meta.json", json.dumps(att.meta, indent=2, sort_keys=True, default=str) + "\n")
    counts: dict = {}
    for a in att.assertions:
        counts[a["result"]] = counts.get(a["result"], 0) + 1
    rows = ["| ID | Scenario | Kind | Result | Description |", "| --- | --- | --- | --- | --- |"]
    for a in att.assertions:
        rows.append(f"| `{a['id']}` | {a['scenario']} | {a['kind']} | {a['result']} | {a['description'].replace('|', '/')} |")
    att.write("RESULTS.md", f"# Attempt {index} results\n\n- PostgreSQL container: `{att.container}` (network `none`, no "
              f"published ports), removed: {att.meta.get('container_removed')}\n- Server: {att.meta.get('server_version')}\n"
              f"- Started {att.meta['t_start']}, finished {att.meta['t_end']}\n- Harness error: {error}\n"
              f"- Counts: {json.dumps(counts, sort_keys=True)}\n- Findings: {json.dumps([f['id'] for f in att.findings])}\n\n"
              "`COUNTEREXAMPLE_REPRODUCED` with PASS means the predicted unsafe or unproven behavior was observed.\n\n"
              + "\n".join(rows) + "\n")
    return {"attempt": index, "counts": counts, "failed": [a["id"] for a in att.assertions if a["result"] == "FAIL"],
            "not_run": [a["id"] for a in att.assertions if a["result"] == "NOT_RUN"],
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
        print(f"attempt {i}: {s['counts']} failed={s['failed']} not_run={s['not_run']} findings={s['findings']}",
              flush=True)
        summary.append(s)
    (out / "summary.json").write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n", encoding="utf-8", newline="\n")
    return 0 if all(not s["failed"] for s in summary) else 1


if __name__ == "__main__":
    sys.exit(main())
