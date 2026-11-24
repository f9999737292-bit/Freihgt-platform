#!/usr/bin/env python3
"""BINTRANS 0.1Q isolated DB identity and fencing proof.

Starts a disposable PostgreSQL 16 container (--network none, no published
ports), builds the test-only role model from sql/, runs the reconnect-first
handover scenarios with concurrent psql sessions, records raw output, and
removes only the container it created.

Standard library only. Passwords are generated per attempt, passed to docker
through the process environment (never on the command line), and redacted from
every file written. Nothing here touches staging, production, or platform
migrations.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import pathlib
import re
import secrets
import subprocess
import sys
import threading
import time

HERE = pathlib.Path(__file__).resolve().parent
SQL_DIR = HERE / "sql"
IMAGE = "postgres:16"
LABEL = "bintrans.proof=0.1q"

ISSUER = "proof_issuer_login"
ISSUER_B = "proof_issuer_b_login"
WRITER = "proof_writer_login"
OTHER = "proof_other_service_login"
FENCING = "proof_fencing_login"
FENCING_DIRECT = "proof_fencing_direct_login"
SHARED = "proof_shared_login"
OWNER = "proof_authority_owner"
FENCE_OWNER = "proof_fence_owner"
ADMIN = "proof_admin"

PW_ENV = {
    ISSUER: "PROOF_PW_ISSUER",
    ISSUER_B: "PROOF_PW_ISSUER_B",
    WRITER: "PROOF_PW_WRITER",
    OTHER: "PROOF_PW_OTHER",
    FENCING: "PROOF_PW_FENCING",
    FENCING_DIRECT: "PROOF_PW_FENCING_DIRECT",
    SHARED: "PROOF_PW_SHARED",
    OWNER: "PROOF_PW_OWNER",
    FENCE_OWNER: "PROOF_PW_FENCE_OWNER",
}
EXTRA_PW = ["PROOF_PW_TAKEOVER", "PROOF_PW_ATTEMPT", "POSTGRES_PASSWORD"]

SQLSTATE_RE = re.compile(r"\b(ERROR|FATAL):\s+([0-9A-Z]{5}):")
CONNECT_MSG_MAP = [
    ("is not permitted to log in", "28000"),
    ("permission denied for database", "42501"),
    ("password authentication failed", "28P01"),
]
LOST_MARKERS = ("connection to server was lost", "server closed the connection unexpectedly")


def now_iso() -> str:
    return dt.datetime.now(dt.timezone.utc).isoformat(timespec="microseconds")


def h(req: str) -> str:
    return f"encode(sha256(convert_to('0.1q:{req}', 'UTF8')), 'hex')"


class Redactor:
    def __init__(self) -> None:
        self.secrets: list[str] = []
        self.hits = 0

    def add(self, value: str) -> None:
        self.secrets.append(value)

    def __call__(self, text: str) -> str:
        for s in self.secrets:
            if s and s in text:
                self.hits += text.count(s)
                text = text.replace(s, "<REDACTED>")
        return text


class Attempt:
    def __init__(self, index: int, out: pathlib.Path) -> None:
        self.index = index
        self.dir = out / f"attempt_{index}"
        self.dir.mkdir(parents=True, exist_ok=False)
        (self.dir / "sessions").mkdir()
        (self.dir / "setup").mkdir()
        suffix = secrets.token_hex(4)
        self.container = f"bintrans-proof-0-1q-{suffix}"
        self.main_db = f"proof_0_1q_{suffix}"
        self.pub_db = f"proof_pub_{suffix}"
        self.redact = Redactor()
        self.pw: dict[str, str] = {}
        for env_name in list(PW_ENV.values()) + EXTRA_PW:
            value = secrets.token_urlsafe(24)
            self.pw[env_name] = value
            self.redact.add(value)
        self.steps: list[dict] = []
        self.assertions: list[dict] = []
        self.timeline: list[dict] = []
        self.sessions: list["Session"] = []
        self.step_no = 0
        self.lock = threading.Lock()
        self.meta: dict = {
            "attempt": index,
            "container": self.container,
            "main_db": self.main_db,
            "pub_db": self.pub_db,
            "image": IMAGE,
            "network": "none",
            "published_ports": "none",
            "label": LABEL,
        }

    # ---- environment -------------------------------------------------
    def env(self, role: str | None = None, app: str | None = None, pw_env: str | None = None) -> dict:
        env = {k: v for k, v in os.environ.items()
               if not k.startswith("PG") and k not in ("DATABASE_URL", "CONTROL_TOWER_DATABASE_URL")}
        env.update(self.pw)
        if pw_env:
            env["PGPASSWORD"] = self.pw[pw_env]
        elif role in PW_ENV:
            env["PGPASSWORD"] = self.pw[PW_ENV[role]]
        env["PGAPPNAME"] = app or "proof-harness"
        return env

    def mark(self, event: str, **kw) -> dict:
        rec = {"t": now_iso(), "mono": time.monotonic(), "event": event, **kw}
        self.timeline.append(rec)
        return rec

    def next_step(self) -> int:
        with self.lock:
            self.step_no += 1
            return self.step_no

    # ---- assertions --------------------------------------------------
    def check(self, aid: str, test: str, kind: str, description: str, cond: bool, observed) -> bool:
        self.assertions.append({
            "id": aid, "test": test, "kind": kind, "description": description,
            "observed": observed, "result": "PASS" if cond else "FAIL",
        })
        return cond

    def observe(self, aid: str, test: str, description: str, observed) -> None:
        self.assertions.append({
            "id": aid, "test": test, "kind": "OBSERVATION", "description": description,
            "observed": observed, "result": "OBSERVED",
        })

    def write(self, rel: str, text: str) -> None:
        path = self.dir / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(self.redact(text), encoding="utf-8", newline="\n")


def run_cmd(att: Attempt, label: str, cmd: list[str], env: dict | None = None,
            stdin: str | None = None, timeout: int = 120, save: str | None = None) -> dict:
    t0 = now_iso()
    p = subprocess.run(cmd, input=stdin.encode() if stdin is not None else None,
                       capture_output=True, env=env, timeout=timeout)
    out = p.stdout.decode("utf-8", "replace").replace("\r\n", "\n")
    err = p.stderr.decode("utf-8", "replace").replace("\r\n", "\n")
    rec = {"label": label, "argv": cmd, "t_start": t0, "t_end": now_iso(),
           "exit": p.returncode, "stdout": att.redact(out), "stderr": att.redact(err)}
    if save:
        att.write(f"{save}.cmd.txt", " ".join(cmd) + "\n")
        att.write(f"{save}.stdout.txt", out)
        att.write(f"{save}.stderr.txt", err)
        att.write(f"{save}.exit", f"{p.returncode}\n")
    return rec


class Session:
    """A persistent psql process inside the container.

    Each step is followed by \\echo and \\warn markers so that stdout and stderr
    can be split per step even though the streams are read asynchronously.
    """

    def __init__(self, att: Attempt, name: str, role: str, tcp: bool = True,
                 app: str | None = None, db: str | None = None, extra_env: tuple = ()) -> None:
        self.att = att
        self.name = name
        self.role = role
        self.tcp = tcp
        self.app = app or f"proof-{name}"
        self.db = db or att.main_db
        cmd = ["docker", "exec", "-i", "-e", "PGPASSWORD", "-e", "PGAPPNAME"]
        for e in extra_env:
            cmd += ["-e", e]
        cmd += [att.container, "stdbuf", "-o0", "-e0", "psql", "-X", "-v", "VERBOSITY=verbose",
                "-P", "pager=off", "-U", role, "-d", self.db]
        if tcp:
            cmd += ["-h", "127.0.0.1"]
        self.argv = cmd
        self.out: list[tuple[float, str]] = []
        self.err: list[tuple[float, str]] = []
        self.inp: list[str] = []
        self.cv = threading.Condition()
        self.out_pos = 0
        self.err_pos = 0
        self.exit_code: int | None = None
        self.pid: int | None = None
        att.mark("session_start", session=name, role=role, transport="tcp" if tcp else "socket",
                 application_name=self.app)
        self.p = subprocess.Popen(cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                  stderr=subprocess.PIPE, env=att.env(role, self.app))
        threading.Thread(target=self._reader, args=(self.p.stdout, self.out), daemon=True).start()
        threading.Thread(target=self._reader, args=(self.p.stderr, self.err), daemon=True).start()
        att.sessions.append(self)

    def _reader(self, stream, sink) -> None:
        for raw in iter(stream.readline, b""):
            line = raw.decode("utf-8", "replace").rstrip("\r\n")
            with self.cv:
                sink.append((time.monotonic(), line))
                self.cv.notify_all()
        with self.cv:
            sink.append((time.monotonic(), "\x00EOF"))
            self.cv.notify_all()

    def alive(self) -> bool:
        return self.p.poll() is None

    def send(self, label: str, sql: str, expect: str = "ok", test: str = "") -> dict:
        n = self.att.next_step()
        marker = f"__M{n}__"
        payload = f"{sql}\n\\echo {marker}\n\\warn {marker}\n"
        step = {"step": n, "session": self.name, "role": self.role, "test": test, "label": label,
                "sql": sql, "expect": expect, "marker": marker, "t_sent": now_iso(),
                "mono_sent": time.monotonic(), "done": False}
        self.inp.append(f"-- step {n} [{label}] sent {step['t_sent']}\n{sql}\n")
        try:
            self.p.stdin.write(payload.encode("utf-8"))
            self.p.stdin.flush()
        except (BrokenPipeError, OSError) as exc:
            step["write_error"] = type(exc).__name__
        self.att.steps.append(step)
        return step

    def _has_marker(self, sink, start, marker) -> int | None:
        for i in range(start, len(sink)):
            if sink[i][1] == marker:
                return i
        return None

    def _eof(self, sink) -> bool:
        return bool(sink) and sink[-1][1] == "\x00EOF"

    def wait(self, step: dict, timeout: float = 20.0) -> dict:
        deadline = time.monotonic() + timeout
        marker = step["marker"]
        with self.cv:
            while True:
                io_ = self._has_marker(self.out, self.out_pos, marker)
                ie = self._has_marker(self.err, self.err_pos, marker)
                if io_ is not None and ie is not None:
                    break
                if self._eof(self.out) and self._eof(self.err):
                    break
                left = deadline - time.monotonic()
                if left <= 0:
                    break
                self.cv.wait(min(left, 0.2))
            done = io_ is not None and ie is not None
            out_end = io_ if io_ is not None else len(self.out)
            err_end = ie if ie is not None else len(self.err)
            if not done and not (self._eof(self.out) and self._eof(self.err)):
                # Still running (e.g. blocked on a lock): leave output for a later wait.
                step.update({"done": False, "pending": True, "t_checked": now_iso()})
                return step
            out_lines = [l for _, l in self.out[self.out_pos:out_end] if l != "\x00EOF"]
            err_lines = [l for _, l in self.err[self.err_pos:err_end] if l != "\x00EOF"]
            self.out_pos = out_end + (1 if io_ is not None else 0)
            self.err_pos = err_end + (1 if ie is not None else 0)
        if not done:
            try:
                self.exit_code = self.p.wait(timeout=10)
            except subprocess.TimeoutExpired:
                self.exit_code = None
        stdout = self.att.redact("\n".join(out_lines))
        stderr = self.att.redact("\n".join(err_lines))
        states = [m.group(2) for m in SQLSTATE_RE.finditer(stderr)]
        lost = any(x in stderr for x in LOST_MARKERS)
        step.update({
            "done": done, "pending": False, "t_done": now_iso(), "mono_done": time.monotonic(),
            "stdout": stdout, "stderr": stderr, "sqlstates": states,
            "ok": done and not states, "connection_lost": lost or not done,
            "process_exit": self.exit_code,
        })
        return step

    def exec(self, label: str, sql: str, expect: str = "ok", test: str = "", timeout: float = 20.0) -> dict:
        return self.wait(self.send(label, sql, expect, test), timeout)

    def value(self, label: str, expr: str, test: str = "") -> str | None:
        st = self.exec(label, f"SELECT 'VAL=' || ({expr})::text AS v;", test=test)
        m = re.search(r"VAL=(\S*)", st.get("stdout", ""))
        st["value"] = m.group(1) if m else None
        return st["value"]

    def backend_pid(self) -> int | None:
        v = self.value(f"{self.name} backend pid", "pg_backend_pid()")
        self.pid = int(v) if v and v.isdigit() else None
        self.att.mark("session_pid", session=self.name, role=self.role, pid=self.pid)
        return self.pid

    def close(self) -> None:
        if self.alive():
            try:
                self.p.stdin.write(b"\\q\n")
                self.p.stdin.flush()
                self.p.stdin.close()
            except OSError:
                pass
            try:
                self.exit_code = self.p.wait(timeout=15)
            except subprocess.TimeoutExpired:
                self.p.kill()
                self.exit_code = self.p.wait()
        else:
            self.exit_code = self.p.returncode
        time.sleep(0.2)
        base = f"sessions/{self.name}"
        self.att.write(f"{base}.argv.txt", " ".join(self.argv) + "\n")
        self.att.write(f"{base}.input.sql", "".join(self.inp))
        self.att.write(f"{base}.stdout.txt", "\n".join(l for _, l in self.out if l != "\x00EOF") + "\n")
        self.att.write(f"{base}.stderr.txt", "\n".join(l for _, l in self.err if l != "\x00EOF") + "\n")
        self.att.write(f"{base}.exit", f"{self.exit_code}\n")


def connect_attempt(att: Attempt, label: str, role: str, tcp: bool = True, db: str | None = None,
                    pw_env: str | None = None, test: str = "", expect: str = "ok") -> dict:
    db = db or att.main_db
    cmd = ["docker", "exec", "-e", "PGPASSWORD", "-e", "PGAPPNAME", att.container, "psql", "-X",
           "-v", "VERBOSITY=verbose", "-U", role, "-d", db]
    if tcp:
        cmd += ["-h", "127.0.0.1"]
    cmd += ["-c", "SELECT 'VAL=' || session_user || ',' || current_user || ',' || pg_backend_pid() AS v;"]
    n = att.next_step()
    t_sent = now_iso()
    mono_sent = time.monotonic()
    rec = run_cmd(att, label, cmd, env=att.env(role, "proof-reconnect", pw_env))
    mapped = None
    for needle, code in CONNECT_MSG_MAP:
        if needle in rec["stderr"]:
            mapped = code
            break
    m = re.search(r"VAL=(\S+)", rec["stdout"])
    step = {"step": n, "session": "one-shot", "kind": "connect_attempt", "role": role, "db": db,
            "transport": "tcp" if tcp else "socket", "test": test, "label": label, "expect": expect,
            "argv": cmd, "t_sent": t_sent, "mono_sent": mono_sent, "t_done": now_iso(),
            "mono_done": time.monotonic(), "exit": rec["exit"], "stdout": rec["stdout"],
            "stderr": rec["stderr"], "connected": rec["exit"] == 0 and m is not None,
            "sqlstate_from_message": mapped, "value": m.group(1) if m else None}
    att.steps.append(step)
    att.mark("connect_attempt", label=label, role=role, transport=step["transport"],
             connected=step["connected"], sqlstate=mapped)
    return step


def first_state(step: dict) -> str | None:
    states = step.get("sqlstates") or []
    return states[0] if states else None


# --------------------------------------------------------------------------
# Container lifecycle
# --------------------------------------------------------------------------

def start_container(att: Attempt) -> None:
    prefix = "%m [%p] sqlstate=%e user=%u db=%d app=%a "
    cmd = ["docker", "run", "-d", "--pull", "never", "--name", att.container,
           "--label", LABEL, "--label", f"bintrans.proof.attempt={att.index}",
           "--network", "none",
           "-e", f"POSTGRES_USER={ADMIN}", "-e", "POSTGRES_PASSWORD", "-e", f"POSTGRES_DB={att.main_db}",
           IMAGE, "postgres", "-c", "log_connections=on", "-c", "log_disconnections=on",
           "-c", f"log_line_prefix={prefix}"]
    rec = run_cmd(att, "docker run", cmd, env=att.env(), save="setup/00_docker_run")
    if rec["exit"] != 0:
        raise RuntimeError("docker run failed")
    att.mark("container_started", container=att.container)
    deadline = time.monotonic() + 90
    while time.monotonic() < deadline:
        logs = run_cmd(att, "docker logs", ["docker", "logs", att.container], env=att.env())
        ready = run_cmd(att, "pg_isready", ["docker", "exec", att.container, "pg_isready", "-h",
                                            "127.0.0.1", "-U", ADMIN, "-d", att.main_db], env=att.env())
        if "PostgreSQL init process complete" in logs["stdout"] + logs["stderr"] and ready["exit"] == 0:
            break
        time.sleep(1)
    else:
        raise RuntimeError("postgres did not become ready")
    att.mark("postgres_ready")
    inspect = run_cmd(att, "docker inspect", ["docker", "inspect", att.container, "--format",
                      "{{.HostConfig.NetworkMode}}|{{json .HostConfig.PortBindings}}|{{json .NetworkSettings.Ports}}|{{.Image}}"],
                      env=att.env(), save="setup/01_inspect_network")
    parts = inspect["stdout"].strip().split("|")
    att.meta["inspect_network_mode"] = parts[0] if parts else None
    att.meta["inspect_port_bindings"] = parts[1] if len(parts) > 1 else None
    att.meta["inspect_ports"] = parts[2] if len(parts) > 2 else None
    att.meta["image_id"] = parts[3] if len(parts) > 3 else None

    hba = ("local all proof_admin trust\n"
           "local all all scram-sha-256\n"
           "host all all 127.0.0.1/32 scram-sha-256\n"
           "host all all ::1/128 scram-sha-256\n")
    att.write("setup/02_pg_hba.conf", hba)
    run_cmd(att, "write pg_hba", ["docker", "exec", "-i", att.container, "sh", "-c",
                                  'cat > "$PGDATA/pg_hba.conf"'], env=att.env(), stdin=hba,
            save="setup/02_write_pg_hba")
    psql_admin(att, "SELECT pg_reload_conf();", "setup/03_reload")
    psql_admin(att, f'CREATE DATABASE "{att.pub_db}";', "setup/04_create_pub_db")
    for name in ("00_roles", "01_schema"):
        sql = (SQL_DIR / f"{name}.sql").read_text(encoding="utf-8")
        rec = psql_admin(att, sql, f"setup/{name}", file_mode=True)
        if rec["exit"] != 0:
            raise RuntimeError(f"{name}.sql failed")
    version = psql_admin(att, "SELECT version();", "setup/05_version")
    att.meta["server_version"] = version["stdout"].strip().splitlines()[-1].strip() if version["stdout"].strip() else None


def psql_admin(att: Attempt, sql: str, save: str, file_mode: bool = False) -> dict:
    cmd = ["docker", "exec", "-i", "-e", "PGAPPNAME"] + [x for e in PW_ENV.values() for x in ("-e", e)] + \
          ["-e", "PROOF_PW_TAKEOVER", "-e", "PROOF_PW_ATTEMPT",
           att.container, "psql", "-X", "-v", "VERBOSITY=verbose", "-v", "ON_ERROR_STOP=1",
           "-v", f"main_db={att.main_db}", "-v", f"pub_db={att.pub_db}",
           "-U", ADMIN, "-d", att.main_db, "-t", "-A", "-f", "-"]
    return run_cmd(att, save, cmd, env=att.env(ADMIN, "proof-admin-setup"), stdin=sql, save=save)


def remove_container(att: Attempt) -> None:
    logs = run_cmd(att, "docker logs", ["docker", "logs", "-t", att.container], env=att.env())
    att.write("server_log.txt", logs["stdout"] + logs["stderr"])
    rm = run_cmd(att, "docker rm", ["docker", "rm", "-f", att.container], env=att.env(), save="setup/99_docker_rm")
    left = run_cmd(att, "docker ps", ["docker", "ps", "-a", "--filter", f"name=^{att.container}$",
                                      "--format", "{{.Names}}"], env=att.env())
    att.meta["container_removed"] = rm["exit"] == 0 and left["stdout"].strip() == ""
    att.mark("container_removed", removed=att.meta["container_removed"])


# --------------------------------------------------------------------------
# Scenarios
# --------------------------------------------------------------------------

def admin_ns(adm: Session, ns: str, holder: str, epoch: int = 1, state: str = "OPEN", gate: str = "OPEN") -> None:
    adm.exec(f"harness seeds namespace {ns}",
             f"INSERT INTO proof.issuance_epoch VALUES ('{ns}', {epoch}, '{holder}');\n"
             f"INSERT INTO proof.handover_state (issuer_namespace_id, state, gate) VALUES ('{ns}', '{state}', '{gate}');",
             test="setup")


def pid_present(adm: Session, pid: int, label: str) -> bool:
    # adm runs in autocommit, so every statement reads a fresh activity snapshot.
    v = adm.value(label, f"EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = {pid})")
    return v == "true"


def s1_identity_and_catalog(att: Attempt, adm: Session) -> None:
    T = "T01_IDENTITY_CATALOG"
    for role in (ISSUER, ISSUER_B, WRITER, OTHER, FENCING, FENCING_DIRECT, SHARED):
        st = connect_attempt(att, f"{role} login", role, test=T)
        parts = (st["value"] or ",,").split(",")
        att.check(f"T01-LOGIN-{role}", T, "POSITIVE_CONTROL",
                  f"{role} logs in with its own credentials; session_user = current_user = {role}",
                  st["connected"] and parts[0] == role and parts[1] == role,
                  {"connected": st["connected"], "value": st["value"]})
    for role in (OWNER, FENCE_OWNER):
        st = connect_attempt(att, f"{role} login attempt", role, test=T, expect="28000")
        att.check(f"T01-NOLOGIN-{role}", T, "DENIAL", f"{role} is NOLOGIN; login is rejected with 28000",
                  not st["connected"] and st["sqlstate_from_message"] == "28000",
                  {"connected": st["connected"], "sqlstate": st["sqlstate_from_message"], "stderr": st["stderr"]})

    sql = (SQL_DIR / "02_catalog.sql").read_text(encoding="utf-8")
    rec = psql_admin(att, sql, "catalog/02_catalog")
    cat = {}
    for line in rec["stdout"].splitlines():
        if line.startswith("CATALOG_JSON "):
            _, name, payload = line.split(" ", 2)
            cat[name] = json.loads(payload)
    att.write("catalog/catalog.json", json.dumps(cat, indent=2, sort_keys=True) + "\n")

    roles = {r["rolname"]: r for r in cat["roles"]}
    runtime = (ISSUER, ISSUER_B, WRITER, OTHER, FENCING)
    bad = [r for r in runtime if roles[r]["rolsuper"] or roles[r]["rolcreaterole"] or roles[r]["rolbypassrls"]
           or roles[r]["rolreplication"] or roles[r]["rolcreatedb"] or not roles[r]["rolcanlogin"]]
    att.check("T01-CAT-RUNTIME-ATTRS", T, "POSITIVE_CONTROL",
              "Runtime logins (issuer, issuer_b, writer, other service, fencing) are LOGIN and have no "
              "SUPERUSER/CREATEROLE/CREATEDB/REPLICATION/BYPASSRLS", not bad,
              {r: roles[r] for r in runtime})
    att.check("T01-CAT-OWNERS-NOLOGIN", T, "POSITIVE_CONTROL", "Owner roles are NOLOGIN",
              not roles[OWNER]["rolcanlogin"] and not roles[FENCE_OWNER]["rolcanlogin"],
              {OWNER: roles[OWNER], FENCE_OWNER: roles[FENCE_OWNER]})
    leaks = [m for m in cat["role_matrix"] if m["member"] or m["usage"] or m["set"]]
    att.check("T01-CAT-NO-MEMBERSHIP", T, "POSITIVE_CONTROL",
              "No runtime login is a member of, can use the privileges of, or can SET ROLE to any owner, "
              "issuer, writer, fencing, shared, or pg_signal_backend role", not leaks, leaks or "none")
    runtime_members = [m for m in cat["memberships"] if m["member"] in runtime]
    att.check("T01-CAT-NO-AUTH-MEMBERS", T, "POSITIVE_CONTROL",
              "pg_auth_members has no rows whose member is a runtime login", not runtime_members,
              runtime_members or "none")
    funcs = {f["function"]: f for f in cat["functions"]}
    unfixed = [k for k, f in funcs.items() if not f["prosecdef"] or f["proconfig"] != ["search_path=pg_catalog"]]
    att.check("T01-CAT-SECDEF-SEARCH-PATH", T, "POSITIVE_CONTROL",
              "Every proof/proof_fence function is SECURITY DEFINER with proconfig = {search_path=pg_catalog}",
              not unfixed, unfixed or "none")
    public_exec = sorted(k for k, f in funcs.items() if f["public_execute"])
    att.check("T01-CAT-PUBLIC-EXECUTE", T, "POSITIVE_CONTROL",
              "Only the deliberate counterexample proof.unprotected_issue(text) keeps PUBLIC EXECUTE",
              public_exec == ["proof.unprotected_issue(text)"], public_exec)
    dbs = {d["datname"]: d for d in cat["databases"]}
    att.check("T01-CAT-DB-CONNECT", T, "POSITIVE_CONTROL",
              "Main database has no PUBLIC CONNECT; the counterexample database keeps PUBLIC CONNECT",
              dbs[att.main_db]["public_connect"] is False and dbs[att.pub_db]["public_connect"] is True,
              {"main": dbs[att.main_db], "pub": dbs[att.pub_db]})
    protected = {"proof.tickets", "proof.issuance_epoch", "proof.handover_state", "proof_fence.allowlist"}
    write_leaks = [p for p in cat["table_privileges"] if p["table"] in protected
                   and (p["insert"] or p["update"] or p["delete"] or p["truncate"])]
    att.check("T01-CAT-NO-PROTECTED-DML", T, "POSITIVE_CONTROL",
              "No runtime login has INSERT/UPDATE/DELETE/TRUNCATE on tickets, issuance_epoch, handover_state, allowlist",
              not write_leaks, write_leaks or "none")
    issuer_sel = [p for p in cat["table_privileges"] if p["role"] in (ISSUER, ISSUER_B) and p["select"]]
    att.check("T01-CAT-ISSUER-NO-BUSINESS", T, "POSITIVE_CONTROL",
              "Issuer logins can only SELECT proof.issuance_epoch; no business-table rights",
              sorted({p["table"] for p in issuer_sel}) == ["proof.issuance_epoch"], issuer_sel)
    cols = {(c["role"], c["column"]): c["update"] for c in cat["column_privileges"]}
    att.check("T01-CAT-WRITER-COLUMNS", T, "POSITIVE_CONTROL",
              "Writer may UPDATE business_rows.note only, not owner_epoch or id",
              cols[(WRITER, "note")] and not cols[(WRITER, "owner_epoch")] and not cols[(WRITER, "id")],
              {k[1]: v for k, v in cols.items() if k[0] == WRITER})
    exe = {(e["role"], e["function"]): e["execute"] for e in cat["function_execute"]}
    allowed = {
        ISSUER: {"proof.issue_ticket(text,text,text,bigint)", "proof.issue_ticket_snapshot(text,text,text,bigint)"},
        ISSUER_B: {"proof.issue_ticket(text,text,text,bigint)", "proof.issue_ticket_snapshot(text,text,text,bigint)"},
        WRITER: {"proof.consume_ticket(bigint)"},
        OTHER: set(),
        FENCING: {"proof.set_handover(text,text)", "proof.transition_epoch(text,bigint,name)",
                  "proof.try_open_gate(text)", "proof_fence.deny_login(name)", "proof_fence.census(name)",
                  "proof_fence.terminate(name,integer)"},
    }
    mismatch = []
    for (role, fn), can in exe.items():
        want = fn in allowed[role] or fn == "proof.unprotected_issue(text)"
        if can != want:
            mismatch.append({"role": role, "function": fn, "execute": can, "expected": want})
    att.check("T01-CAT-EXECUTE-MATRIX", T, "POSITIVE_CONTROL",
              "EXECUTE matrix matches the model: issuer=issue only, writer=consume only, fencing=handover/fence "
              "only, other service=none (plus PUBLIC counterexample)", not mismatch, mismatch or "none")
    creates = [s for s in cat["schema_privileges"] if s["create"]]
    att.check("T01-CAT-NO-SCHEMA-CREATE", T, "POSITIVE_CONTROL",
              "No runtime login has CREATE on proof, proof_fence, or public", not creates, creates or "none")
    att.meta["catalog_server"] = cat.get("server")


def s2_set_role(att: Attempt) -> None:
    T = "T02_SET_ROLE"
    cases = [
        (WRITER, ["SET ROLE proof_issuer_login", "SET ROLE proof_authority_owner", "SET ROLE proof_fencing_login",
                  "SET ROLE proof_fence_owner", "SET SESSION AUTHORIZATION proof_issuer_login"]),
        (ISSUER, ["SET ROLE proof_authority_owner", "SET ROLE proof_writer_login", "SET ROLE proof_fence_owner"]),
        (FENCING, ["SET ROLE proof_issuer_login", "SET ROLE proof_fence_owner", "SET ROLE proof_authority_owner"]),
        (FENCING_DIRECT, ["SET ROLE proof_issuer_b_login"]),
        (OTHER, ["SET ROLE proof_issuer_login", "SET ROLE proof_writer_login"]),
    ]
    for role, stmts in cases:
        s = Session(att, f"t02_{role}", role, app="proof-t02")
        for i, stmt in enumerate(stmts, 1):
            st = s.exec(f"{role}: {stmt}", stmt + ";", expect="42501", test=T)
            att.check(f"T02-{role}-{i}", T, "DENIAL", f"{role}: {stmt} is rejected with 42501",
                      first_state(st) == "42501", {"sqlstates": st["sqlstates"], "stderr": st["stderr"]})
        v = s.value(f"{role}: identity after denied SET ROLE", "session_user || ',' || current_user", test=T)
        att.check(f"T02-{role}-IDENTITY", T, "POSITIVE_CONTROL",
                  f"{role}: session_user and current_user unchanged after denied SET ROLE",
                  v == f"{role},{role}", v)
        s.close()


def s3_direct_dml(att: Attempt) -> dict:
    T = "T03_DIRECT_DML"
    res: dict = {}

    def expect(s: Session, aid: str, label: str, sql: str, state: str | None, kind: str = "DENIAL",
               msg: str | None = None) -> dict:
        st = s.exec(label, sql, expect=state or "ok", test=T)
        if state is None:
            cond = st["ok"]
        else:
            cond = first_state(st) == state and (msg is None or msg in st["stderr"])
        att.check(aid, T, kind, label + (f" -> {state}" if state else " -> succeeds"), cond,
                  {"sqlstates": st["sqlstates"], "stdout": st["stdout"], "stderr": st["stderr"]})
        return st

    w = Session(att, "t03_writer", WRITER, app="proof-writer", extra_env=("PROOF_PW_ATTEMPT",))
    expect(w, "T03-W-INSERT-TICKET", "writer INSERT into proof.tickets",
           f"INSERT INTO proof.tickets (issuer_namespace_id, request_id, payload_hash, issued_epoch, issued_by) "
           f"VALUES ('ns-x', 'w-1', {h('w-1')}, 1, 'proof_issuer_login');", "42501")
    expect(w, "T03-W-UPDATE-TICKET", "writer UPDATE proof.tickets status",
           "UPDATE proof.tickets SET status = 'ISSUED';", "42501")
    expect(w, "T03-W-CALL-ISSUE", "writer calls proof.issue_ticket",
           f"SELECT proof.issue_ticket('ns-x', 'w-2', {h('w-2')}, 1);", "42501")
    expect(w, "T03-W-CALL-ISSUE-SNAP", "writer calls proof.issue_ticket_snapshot",
           f"SELECT proof.issue_ticket_snapshot('ns-x', 'w-3', {h('w-3')}, 1);", "42501")
    expect(w, "T03-W-UPDATE-EPOCH-COL", "writer UPDATE business_rows.owner_epoch (sensitive column)",
           "UPDATE proof.business_rows SET owner_epoch = 99 WHERE id = 1;", "42501")
    expect(w, "T03-W-UPDATE-NOTE", "writer UPDATE business_rows.note (granted column)",
           "UPDATE proof.business_rows SET note = 'writer-ok' WHERE id = 1;", None, "POSITIVE_CONTROL")
    expect(w, "T03-W-DELETE", "writer DELETE business_rows", "DELETE FROM proof.business_rows;", "42501")
    expect(w, "T03-W-EPOCH", "writer UPDATE proof.issuance_epoch",
           "UPDATE proof.issuance_epoch SET epoch = epoch + 1;", "42501")
    expect(w, "T03-W-HANDOVER", "writer calls proof.set_handover",
           "SELECT proof.set_handover('ns-x', 'QUIESCED');", "42501")
    expect(w, "T03-W-FENCE", "writer calls proof_fence.deny_login",
           "SELECT proof_fence.deny_login('proof_issuer_login');", "42501")
    expect(w, "T03-W-CREATE-FN", "writer CREATE FUNCTION in schema proof",
           "CREATE FUNCTION proof.evil() RETURNS int LANGUAGE sql AS 'SELECT 1';", "42501")
    expect(w, "T03-W-CONSUME", "writer calls proof.consume_ticket (granted)",
           "SELECT proof.consume_ticket(-1);", None, "POSITIVE_CONTROL")
    st = expect(w, "T03-W-UNPROTECTED", "COUNTEREXAMPLE: writer calls proof.unprotected_issue (PUBLIC EXECUTE left at default)",
                "SELECT proof.unprotected_issue('writer-via-public');", None, "COUNTEREXAMPLE_REPRODUCED")
    res["writer_unprotected_ok"] = st["ok"]
    w.close()

    i = Session(att, "t03_issuer", ISSUER, app="proof-issuer")
    expect(i, "T03-I-INSERT-TICKET", "issuer INSERT into proof.tickets directly",
           f"INSERT INTO proof.tickets (issuer_namespace_id, request_id, payload_hash, issued_epoch, issued_by) "
           f"VALUES ('ns-x', 'i-1', {h('i-1')}, 1, 'proof_issuer_login');", "42501")
    expect(i, "T03-I-UPDATE-EPOCH", "issuer UPDATE proof.issuance_epoch directly",
           "UPDATE proof.issuance_epoch SET epoch = epoch + 1;", "42501")
    expect(i, "T03-I-SELECT-BUSINESS", "issuer SELECT proof.business_rows", "SELECT * FROM proof.business_rows;", "42501")
    expect(i, "T03-I-UPDATE-BUSINESS", "issuer UPDATE proof.business_rows",
           "UPDATE proof.business_rows SET note = 'x';", "42501")
    expect(i, "T03-I-CONSUME", "issuer calls proof.consume_ticket", "SELECT proof.consume_ticket(1);", "42501")
    expect(i, "T03-I-FENCE", "issuer calls proof_fence.census",
           "SELECT proof_fence.census('proof_issuer_login');", "42501")
    i.close()

    o = Session(att, "t03_other", OTHER, app="proof-other")
    expect(o, "T03-O-CALL-ISSUE", "other service calls proof.issue_ticket",
           f"SELECT proof.issue_ticket('ns-x', 'o-1', {h('o-1')}, 1);", "42501")
    expect(o, "T03-O-SELECT-TICKETS", "other service SELECT proof.tickets", "SELECT * FROM proof.tickets;", "42501")
    expect(o, "T03-O-CONTROL-INSERT", "other service INSERT into its own control table",
           "INSERT INTO proof.control_rows (note) VALUES ('other-ok');", None, "POSITIVE_CONTROL")
    st = expect(o, "T03-O-UNPROTECTED", "COUNTEREXAMPLE: other service calls proof.unprotected_issue via PUBLIC",
                "SELECT proof.unprotected_issue('other-via-public');", None, "COUNTEREXAMPLE_REPRODUCED")
    res["other_unprotected_ok"] = st["ok"]
    o.close()

    f = Session(att, "t03_fencing", FENCING, app="proof-fencing", extra_env=("PROOF_PW_ATTEMPT",))
    expect(f, "T03-F-INSERT-TICKET", "fencing INSERT into proof.tickets",
           f"INSERT INTO proof.tickets (issuer_namespace_id, request_id, payload_hash, issued_epoch, issued_by) "
           f"VALUES ('ns-x', 'f-1', {h('f-1')}, 1, 'proof_issuer_login');", "42501")
    expect(f, "T03-F-CALL-ISSUE", "fencing calls proof.issue_ticket",
           f"SELECT proof.issue_ticket('ns-x', 'f-2', {h('f-2')}, 1);", "42501")
    expect(f, "T03-F-ALTER-PASSWORD", "fencing ALTER ROLE proof_issuer_login PASSWORD (takeover attempt)",
           "\\getenv pw PROOF_PW_ATTEMPT\nALTER ROLE proof_issuer_login PASSWORD :'pw';", "42501")
    expect(f, "T03-F-ALTER-NOLOGIN-DIRECT", "fencing ALTER ROLE proof_issuer_login NOLOGIN directly (outside definer)",
           "ALTER ROLE proof_issuer_login NOLOGIN;", "42501")
    expect(f, "T03-F-SELF-GRANT", "fencing GRANT proof_issuer_login TO itself",
           "GRANT proof_issuer_login TO proof_fencing_login WITH SET TRUE;", "42501")
    victim = Session(att, "t03_victim_other", OTHER, app="proof-other")
    victim.backend_pid()
    expect(f, "T03-F-TERMINATE-RAW", "fencing calls pg_terminate_backend directly on a live other-service backend (no pg_signal_backend)",
           f"SELECT pg_terminate_backend({victim.pid});", "42501")
    alive = pid_present(att.adm, victim.pid, "victim still present after denied raw terminate")
    att.check("T03-F-TERMINATE-RAW-VICTIM-ALIVE", T, "POSITIVE_CONTROL",
              "The other-service backend survives the denied raw terminate", alive, alive)
    victim.close()
    f.close()

    i2 = Session(att, "t03_issuer_fn", ISSUER, app="proof-issuer")
    adm = att.adm
    admin_ns(adm, "ns-t03", ISSUER)
    expect(i2, "T03-I-NO-DIGEST", "issuer issue_ticket without payload digest -> ISSUE_DENIED_AUTH",
           "SELECT proof.issue_ticket('ns-t03', 'i-nohash', NULL, 1);", "P0001", "DENIAL", "ISSUE_DENIED_AUTH")
    st = expect(i2, "T03-I-ISSUE-OK", "issuer issue_ticket with digest and current epoch (granted path)",
                f"SELECT proof.issue_ticket('ns-t03', 'i-ok', {h('i-ok')}, 1);", None, "POSITIVE_CONTROL")
    expect(i2, "T03-I-DUPLICATE", "issuer issue_ticket same (ns, request_id) again -> unique violation",
           f"SELECT proof.issue_ticket('ns-t03', 'i-ok', {h('i-ok')}, 1);", "23505")
    v = adm.value("issued_by of i-ok", "(SELECT issued_by || ',' || issued_epoch FROM proof.tickets WHERE request_id = 'i-ok')")
    att.check("T03-I-ISSUED-BY", T, "POSITIVE_CONTROL",
              "Ticket records issued_by = session_user of the issuer login (not the definer owner)",
              v == f"{ISSUER},1", v)
    i2.close()
    return res


def s4_handover(att: Attempt) -> dict:
    T = "T04_RECONNECT_FIRST_HANDOVER"
    adm = att.adm
    out: dict = {}
    admin_ns(adm, "ns-main", ISSUER)
    admin_ns(adm, "ns-n4", ISSUER)

    iss1 = Session(att, "iss1_open_txn", ISSUER, app="proof-issuer")
    iss2 = Session(att, "iss2_pool_idle", ISSUER, app="proof-issuer")
    iss3 = Session(att, "iss3_pool_socket", ISSUER, tcp=False, app="proof-issuer")
    iss4 = Session(att, "iss4_inflight_commit", ISSUER, app="proof-issuer")
    oth = Session(att, "oth_spoofed_appname", OTHER, app="proof-issuer")
    wr = Session(att, "wr_writer", WRITER, app="proof-writer")
    fen = Session(att, "fen_controller", FENCING, app="proof-fencing")
    for s in (iss1, iss2, iss3, iss4, oth, wr, fen):
        s.backend_pid()
    issuer_pids = {iss1.pid, iss2.pid, iss3.pid, iss4.pid}
    out["pids"] = {s.name: s.pid for s in (iss1, iss2, iss3, iss4, oth, wr, fen)}

    st = iss1.exec("iss1 BEGIN + issue ticket req-open-1 (left uncommitted)",
                   f"BEGIN;\nSELECT proof.issue_ticket('ns-main', 'req-open-1', {h('req-open-1')}, 1);", test=T)
    att.check("T04-PRE-ISS1-OPEN-TXN", T, "POSITIVE_CONTROL", "Old issuer opens a transaction that issues a ticket",
              st["ok"], st["stdout"])
    st = iss4.exec("iss4 BEGIN + issue ticket req-n4 (left uncommitted)",
                   f"BEGIN;\nSELECT proof.issue_ticket('ns-n4', 'req-n4', {h('req-n4')}, 1);", test=T)
    att.check("T04-PRE-ISS4-OPEN-TXN", T, "POSITIVE_CONTROL", "Second old-issuer transaction issues a ticket", st["ok"], st["stdout"])
    st = oth.exec("other service BEGIN + insert (left uncommitted)",
                  "BEGIN;\nINSERT INTO proof.control_rows (note) VALUES ('other-in-flight');", test=T)
    att.check("T04-PRE-OTHER-OPEN-TXN", T, "POSITIVE_CONTROL", "Other service holds an open transaction", st["ok"], st["stdout"])

    # Phase A: CLOSE
    att.mark("phase", phase="A_CLOSE")
    st = fen.exec("Phase A: close gates", "SELECT proof.set_handover('ns-main', 'HANDOVER_PENDING');\n"
                  "SELECT proof.set_handover('ns-n4', 'HANDOVER_PENDING');", test=T)
    att.check("T04-A-CLOSE", T, "POSITIVE_CONTROL", "Fencing controller closes the issuance gates", st["ok"], st["stdout"])
    st = iss2.exec("old issuer issues after gate close", f"SELECT proof.issue_ticket('ns-main', 'req-after-close', {h('req-after-close')}, 1);",
                   expect="P0001", test=T)
    att.check("T04-A-NEW-ISSUE-DENIED", T, "DENIAL", "A new issuance attempt after CLOSE fails with GATE_CLOSED",
              first_state(st) == "P0001" and "GATE_CLOSED" in st["stderr"], st["stderr"])

    # Phase B: DENY RECONNECT, confirmed by a real connection attempt, before any census.
    att.mark("phase", phase="B_DENY_RECONNECT")
    st_deny = fen.exec("Phase B: deny login for old issuer", "SELECT proof_fence.deny_login('proof_issuer_login');", test=T)
    att.check("T04-B-DENY", T, "POSITIVE_CONTROL", "Fencing controller sets NOLOGIN on the old issuer via the definer function",
              st_deny["ok"] and "NOLOGIN:proof_issuer_login" in st_deny["stdout"], st_deny["stdout"])
    att.mark("deny_committed", step=st_deny["step"])
    st_rc = connect_attempt(att, "Phase B: verify reconnect denied (tcp)", ISSUER, test=T, expect="28000")
    att.check("T04-B-RECONNECT-DENIED", T, "DENIAL", "Old issuer reconnect after ban fails with 28000 (not permitted to log in)",
              not st_rc["connected"] and st_rc["sqlstate_from_message"] == "28000",
              {"stderr": st_rc["stderr"], "exit": st_rc["exit"]})
    fen.exec("Phase B: record RECONNECT_DENIED", "SELECT proof.set_handover('ns-main', 'RECONNECT_DENIED');", test=T)
    out["reconnect_denied"] = (not st_rc["connected"]) and st_rc["sqlstate_from_message"] == "28000"

    # Test B: the ban does not touch the existing backend.
    st = iss1.exec("iss1 still alive inside its transaction after ban", "SELECT 'VAL=' || txid_current_if_assigned()::text;", test=T)
    alive = pid_present(adm, iss1.pid, "iss1 pid present after ban")
    att.check("T04-B-EXISTING-SURVIVES-BAN", T, "COUNTEREXAMPLE_REPRODUCED",
              "Existing old-issuer backend with an open transaction stays alive and usable after NOLOGIN",
              st["ok"] and alive, {"stdout": st["stdout"], "pid_present": alive})

    # N4: in-flight commit after CLOSE + DENY.
    st = iss4.exec("iss4 COMMIT after close + deny", "COMMIT;", test=T)
    cnt = adm.value("count req-n4 committed", "(SELECT count(*) FROM proof.tickets WHERE request_id = 'req-n4')")
    att.check("T04-N4-INFLIGHT-COMMIT", T, "COUNTEREXAMPLE_REPRODUCED",
              "Old issuer COMMIT before quiesce succeeds even after CLOSE and DENY (gate/ban are not commit fences)",
              st["ok"] and "COMMIT" in st["stdout"] and cnt == "1", {"stdout": st["stdout"], "committed_rows": cnt})

    # Test D: pool reconnect.
    pool = []
    for k in range(3):
        pool.append(connect_attempt(att, f"pool reconnect tcp #{k + 1}", ISSUER, test=T, expect="28000"))
    for k in range(2):
        pool.append(connect_attempt(att, f"pool reconnect socket #{k + 1}", ISSUER, tcp=False, test=T, expect="28000"))
    att.check("T04-D-POOL-RECONNECT", T, "DENIAL", "Five pool reconnects (3 tcp, 2 unix socket) are all denied with 28000",
              all(not p["connected"] and p["sqlstate_from_message"] == "28000" for p in pool),
              [{"transport": p["transport"], "connected": p["connected"], "sqlstate": p["sqlstate_from_message"]} for p in pool])
    out["pool_denied"] = all(not p["connected"] and p["sqlstate_from_message"] == "28000" for p in pool)

    # Phase C: QUIESCE. Census by role OID, after the ban was verified.
    att.mark("phase", phase="C_QUIESCE")
    st_census = fen.exec("Phase C: census by role OID", "SELECT 'VAL=' || proof_fence.census('proof_issuer_login')::text;", test=T)
    m = re.search(r"VAL=\{([0-9,]*)\}", st_census["stdout"])
    census = {int(x) for x in m.group(1).split(",") if x} if m else set()
    att.check("T04-C-CENSUS-EXACT", T, "POSITIVE_CONTROL",
              "Census by role OID returns exactly the four old-issuer backends", census == issuer_pids,
              {"census": sorted(census), "expected": sorted(issuer_pids)})
    att.check("T04-C-CENSUS-EXCLUDES-OTHERS", T, "POSITIVE_CONTROL",
              "Census excludes the other service (spoofed application_name), writer, and controller",
              not census & {oth.pid, wr.pid, fen.pid}, sorted(census))
    order_ok = st_deny["mono_done"] < st_rc["mono_done"] < st_census["mono_sent"]
    att.check("T04-ORDER-DENY-BEFORE-CENSUS", T, "ORDERING",
              "Deny committed, then reconnect denial observed, then census started",
              order_ok, {"deny_done": st_deny["t_done"], "reconnect_denied": st_rc["t_done"],
                         "census_sent": st_census["t_sent"]})
    out["deny_before_census"] = order_ok and out["reconnect_denied"]
    v = adm.value("pids by application_name = proof-issuer",
                  "(SELECT array_agg(pid ORDER BY pid) FROM pg_stat_activity WHERE application_name = 'proof-issuer')")
    app_pids = {int(x) for x in re.findall(r"\d+", v or "")}
    att.check("T04-C-APPNAME-SPOOF", T, "COUNTEREXAMPLE_REPRODUCED",
              "A census by application_name would include the other service (spoofed name) and is unsafe",
              oth.pid in app_pids, {"application_name_pids": sorted(app_pids), "other_service_pid": oth.pid})

    st = fen.exec("premature QUIESCED", "SELECT proof.set_handover('ns-main', 'QUIESCED');", test=T)
    st = fen.exec("transition while old backends remain",
                  "SELECT proof.transition_epoch('ns-main', 1, 'proof_issuer_b_login');", expect="P0001", test=T)
    att.check("T04-C-NO-EPOCH-WITH-OLD-BACKENDS", T, "DENIAL",
              "New epoch is refused while old-issuer backends remain (OLD_BACKENDS_PRESENT)",
              first_state(st) == "P0001" and "OLD_BACKENDS_PRESENT" in st["stderr"], st["stderr"])
    out["epoch_refused_with_old_backends"] = first_state(st) == "P0001" and "OLD_BACKENDS_PRESENT" in st["stderr"]

    st = fen.exec("terminate other-service pid under issuer target",
                  f"SELECT proof_fence.terminate('proof_issuer_login', {oth.pid});", expect="P0001", test=T)
    att.check("T04-C-WRONG-PID-REFUSED", T, "DENIAL", "Controller refuses to terminate a pid not owned by the target role",
              first_state(st) == "P0001" and "PID_NOT_OWNED_BY_TARGET" in st["stderr"], st["stderr"])
    st = fen.exec("terminate with non-allowlisted target",
                  f"SELECT proof_fence.terminate('proof_other_service_login', {oth.pid});", expect="P0001", test=T)
    att.check("T04-C-TARGET-NOT-ALLOWLISTED", T, "DENIAL", "Controller refuses a non-allowlisted target role",
              first_state(st) == "P0001" and "TARGET_NOT_ALLOWLISTED" in st["stderr"], st["stderr"])

    signals = {}
    first_seen = {}
    for pid in sorted(census):
        st = fen.exec(f"terminate pid {pid}", f"SELECT 'VAL=' || proof_fence.terminate('proof_issuer_login', {pid})::text;", test=T)
        signals[pid] = "VAL=true" in st["stdout"]
        att.mark("terminate_signal_returned", pid=pid, returned=signals[pid])
        first_seen[pid] = pid_present(adm, pid, f"independent check right after signal for {pid}")
    att.check("T04-C-SIGNALS-SENT", T, "POSITIVE_CONTROL", "pg_terminate_backend returned true for every census pid",
              all(signals.values()) and len(signals) == 4, signals)
    att.observe("T04-C-SIGNAL-VS-GONE", T,
                "Was the pid still visible to an independent observer immediately after terminate returned true?",
                {str(k): v for k, v in first_seen.items()})
    polls = 0
    remaining = set(census)
    deadline = time.monotonic() + 15
    while remaining and time.monotonic() < deadline:
        polls += 1
        cur = adm.value(f"independent poll #{polls} pids",
                        f"(SELECT coalesce(array_agg(pid ORDER BY pid), '{{}}') FROM pg_stat_activity WHERE pid = ANY('{{{','.join(map(str, census))}}}'::int[]))")
        remaining = {int(x) for x in re.findall(r"\d+", cur or "")}
        if remaining:
            time.sleep(0.1)
    att.mark("old_backends_gone_confirmed", remaining=sorted(remaining), polls=polls)
    att.check("T04-C-BACKENDS-GONE", T, "POSITIVE_CONTROL",
              "Independent observer (separate admin session) confirms all old-issuer pids are gone",
              not remaining, {"remaining": sorted(remaining), "polls": polls})
    out["backends_gone"] = not remaining

    client = {}
    for s, sql in ((iss1, "COMMIT;"), (iss2, "SELECT 1;"), (iss3, "SELECT 1;"), (iss4, "SELECT 1;")):
        st = s.exec(f"{s.name} client after termination", sql, test=T, timeout=15)
        client[s.name] = {"done": st["done"], "sqlstates": st["sqlstates"], "lost": st["connection_lost"],
                          "exit": st.get("process_exit"), "stderr": st["stderr"]}
    att.check("T04-C-CLIENTS-SEE-TERMINATION", T, "POSITIVE_CONTROL",
              "Every terminated client observes connection loss (FATAL 57P01 or lost connection) and psql exits",
              all(not c["done"] and c["lost"] for c in client.values()), client)
    att.check("T04-B-ISS1-57P01", T, "POSITIVE_CONTROL",
              "The open-transaction client reports FATAL 57P01 (administrator termination) on COMMIT",
              "57P01" in client["iss1_open_txn"]["sqlstates"], client["iss1_open_txn"])
    cnt = adm.value("req-open-1 rows after termination", "(SELECT count(*) FROM proof.tickets WHERE request_id = 'req-open-1')")
    att.check("T04-B-ROLLBACK", T, "POSITIVE_CONTROL", "The terminated open transaction was rolled back (0 rows for req-open-1)",
              cnt == "0", cnt)
    out["rollback"] = cnt == "0"

    alive_o = pid_present(adm, oth.pid, "other service pid after termination")
    alive_w = pid_present(adm, wr.pid, "writer pid after termination")
    st_o = oth.exec("other service COMMIT after handover terminations", "COMMIT;", test=T)
    cnt_o = adm.value("other-in-flight rows", "(SELECT count(*) FROM proof.control_rows WHERE note = 'other-in-flight')")
    st_w = wr.exec("writer still works", "UPDATE proof.business_rows SET note = 'writer-after-handover' WHERE id = 1;", test=T)
    ok = alive_o and alive_w and st_o["ok"] and "COMMIT" in st_o["stdout"] and cnt_o == "1" and st_w["ok"]
    att.check("T04-C-OTHER-SERVICES-UNAFFECTED", T, "POSITIVE_CONTROL",
              "Other service (spoofed application_name) and writer sessions survive and commit",
              ok, {"other_alive": alive_o, "writer_alive": alive_w, "other_commit": st_o["stdout"],
                   "other_rows": cnt_o, "writer_update": st_w["stdout"]})
    out["others_unaffected"] = ok

    # Phase D: TRANSITION
    att.mark("phase", phase="D_TRANSITION")
    v = fen.value("Phase D: transition epoch", "proof.transition_epoch('ns-main', 1, 'proof_issuer_b_login')", test=T)
    att.check("T04-D-TRANSITION", T, "POSITIVE_CONTROL",
              "After ban + confirmed termination, transition to epoch 2 / proof_issuer_b_login succeeds", v == "2", v)

    # Phase E: OPEN must stay closed without a witness.
    att.mark("phase", phase="E_OPEN_ATTEMPT")
    v = fen.value("Phase E: try_open_gate", "proof.try_open_gate('ns-main')", test=T)
    gate = adm.value("gate after try_open_gate", "(SELECT gate || ',' || state FROM proof.handover_state WHERE issuer_namespace_id = 'ns-main')")
    att.check("T04-E-GATE-STAYS-CLOSED", T, "DENIAL",
              "Gate stays CLOSED because no external witness confirmation exists (witness NOT_RUN)",
              v == "CLOSED:WITNESS_NOT_CONFIRMED" and gate == "CLOSED,TRANSITIONED", {"result": v, "gate_state": gate})
    out["gate_closed"] = v == "CLOSED:WITNESS_NOT_CONFIRMED" and gate == "CLOSED,TRANSITIONED"
    ib = Session(att, "ib_new_holder", ISSUER_B, app="proof-issuer-b")
    st = ib.exec("new holder issues while gate closed", f"SELECT proof.issue_ticket('ns-main', 'req-b-1', {h('req-b-1')}, 2);",
                 expect="P0001", test=T)
    att.check("T04-E-NEW-HOLDER-BLOCKED", T, "DENIAL", "New epoch holder cannot issue while the gate is CLOSED",
              first_state(st) == "P0001" and "GATE_CLOSED" in st["stderr"], st["stderr"])
    ib.close()
    olds = [connect_attempt(att, "old identity after handover (tcp)", ISSUER, test=T, expect="28000"),
            connect_attempt(att, "old identity after handover (socket)", ISSUER, tcp=False, test=T, expect="28000")]
    att.check("T04-E-OLD-IDENTITY-REJECTED", T, "DENIAL", "Old issuer credentials cannot open a session after handover",
              all(not o["connected"] and o["sqlstate_from_message"] == "28000" for o in olds),
              [{"transport": o["transport"], "stderr": o["stderr"]} for o in olds])
    out["old_identity_rejected"] = all(not o["connected"] for o in olds)
    for s in (iss1, iss2, iss3, iss4, oth, wr, fen):
        s.close()
    exits = {s.name: s.exit_code for s in (iss1, iss2, iss3, iss4)}
    att.check("T04-C-CLIENT-EXIT-CODES", T, "POSITIVE_CONTROL", "Terminated psql clients exit with code 2 (connection lost)",
              all(e == 2 for e in exits.values()), exits)
    return out


def s5_insufficient_bans(att: Attempt) -> dict:
    T = "T05_INSUFFICIENT_BANS"
    adm = att.adm
    out = {}
    admin_ns(adm, "ns-revoke", ISSUER_B)
    ib1 = Session(att, "ib1_revoke_active", ISSUER_B, app="proof-issuer-b")
    ib1.backend_pid()
    adm.exec("REVOKE CONNECT main FROM issuer_b", f'REVOKE CONNECT ON DATABASE "{att.main_db}" FROM {ISSUER_B};', test=T)
    att.mark("revoke_connect_committed", role=ISSUER_B)
    st = ib1.exec("existing backend issues after REVOKE CONNECT",
                  f"SELECT proof.issue_ticket('ns-revoke', 'req-r1', {h('req-r1')}, 1);", test=T)
    att.check("T05-N1-REVOKE-ACTIVE-BACKEND", T, "COUNTEREXAMPLE_REPRODUCED",
              "REVOKE CONNECT does not affect an already-open backend; it still issues and commits",
              st["ok"], st["stdout"])
    out["revoke_active_commits"] = st["ok"]
    rc = connect_attempt(att, "issuer_b reconnect main after REVOKE CONNECT", ISSUER_B, test=T, expect="42501")
    att.check("T05-N1-REVOKE-BLOCKS-NEW", T, "DENIAL", "New connection after REVOKE CONNECT fails with 42501",
              not rc["connected"] and rc["sqlstate_from_message"] == "42501", rc["stderr"])
    adm.exec("REVOKE CONNECT pub FROM issuer_b", f'REVOKE CONNECT ON DATABASE "{att.pub_db}" FROM {ISSUER_B};', test=T)
    rc = connect_attempt(att, "issuer_b connect pub db (PUBLIC CONNECT retained)", ISSUER_B, db=att.pub_db, test=T)
    att.check("T05-N2-INSUFFICIENT-REVOKE", T, "COUNTEREXAMPLE_REPRODUCED",
              "REVOKE CONNECT from one role is insufficient while PUBLIC keeps CONNECT: connection succeeds",
              rc["connected"], {"value": rc["value"], "stderr": rc["stderr"]})
    out["insufficient_revoke_reproduced"] = rc["connected"]
    adm.exec("REVOKE CONNECT main FROM shared superuser", f'REVOKE CONNECT ON DATABASE "{att.main_db}" FROM {SHARED};', test=T)
    rc = connect_attempt(att, "shared superuser connect after REVOKE CONNECT", SHARED, test=T)
    att.check("T05-N3-SUPERUSER-IGNORES-REVOKE", T, "COUNTEREXAMPLE_REPRODUCED",
              "A superuser login ignores REVOKE CONNECT", rc["connected"], {"value": rc["value"], "stderr": rc["stderr"]})
    adm.exec("restore CONNECT", f'GRANT CONNECT ON DATABASE "{att.main_db}" TO {ISSUER_B}, {SHARED};', test=T)
    ib1.close()
    return out


def s6_epoch(att: Attempt) -> dict:
    T = "T06_EPOCH"
    adm = att.adm
    out = {}

    # E1: holding the epoch row FOR UPDATE does not stop the holder from committing.
    admin_ns(adm, "ns-e1", ISSUER_B)
    ib = Session(att, "e1_holder", ISSUER_B, app="proof-issuer-b")
    naive = Session(att, "e1_naive_epoch_change", ADMIN, tcp=False, app="proof-naive-controller")
    naive.backend_pid()
    st = ib.exec("E1 holder BEGIN + issue (locks epoch row)",
                 f"BEGIN;\nSELECT proof.issue_ticket('ns-e1', 'req-e1', {h('req-e1')}, 1);", test=T)
    pend = naive.send("E1 naive epoch change waits for row lock",
                      "UPDATE proof.issuance_epoch SET epoch = epoch + 1 WHERE issuer_namespace_id = 'ns-e1';", test=T)
    waiting = None
    for _ in range(50):
        waiting = adm.value("E1 wait_event_type of epoch changer",
                            f"(SELECT coalesce(wait_event_type, 'none') FROM pg_stat_activity WHERE pid = {naive.pid})")
        if waiting == "Lock":
            break
        time.sleep(0.1)
    naive.wait(pend, timeout=0.5)
    blocked = pend.get("pending", False)
    st_c = ib.exec("E1 holder COMMIT while epoch change waits", "COMMIT;", test=T)
    naive.wait(pend, timeout=10)
    row = adm.value("E1 ticket epoch vs current epoch",
                    "(SELECT t.issued_epoch || ',' || e.epoch FROM proof.tickets t JOIN proof.issuance_epoch e "
                    "ON e.issuer_namespace_id = t.issuer_namespace_id WHERE t.request_id = 'req-e1')")
    ok = st["ok"] and waiting == "Lock" and blocked and st_c["ok"] and "COMMIT" in st_c["stdout"] and pend["ok"] and row == "1,2"
    att.check("T06-E1-LOCK-NOT-FENCE", T, "COUNTEREXAMPLE_REPRODUCED",
              "Epoch change waits on the holder's FOR UPDATE lock; the holder still commits under epoch 1, then the epoch becomes 2",
              ok, {"changer_wait_event_type": waiting, "changer_blocked": blocked, "holder_commit": st_c["stdout"],
                   "changer_result": pend.get("stdout"), "ticket_epoch,current_epoch": row})
    out["lock_not_fence"] = ok

    # E2: stale snapshot commit under REPEATABLE READ with a plain epoch read.
    admin_ns(adm, "ns-e2", ISSUER_B)
    ib.exec("E2 BEGIN RR + read epoch", "BEGIN ISOLATION LEVEL REPEATABLE READ;\n"
            "SELECT epoch FROM proof.issuance_epoch WHERE issuer_namespace_id = 'ns-e2';", test=T)
    adm.exec("E2 epoch change commits", "UPDATE proof.issuance_epoch SET epoch = 2 WHERE issuer_namespace_id = 'ns-e2';", test=T)
    st = ib.exec("E2 snapshot issue with stale epoch", f"SELECT proof.issue_ticket_snapshot('ns-e2', 'req-e2', {h('req-e2')}, 1);", test=T)
    st_c = ib.exec("E2 COMMIT", "COMMIT;", test=T)
    row = adm.value("E2 ticket epoch vs current", "(SELECT t.issued_epoch || ',' || e.epoch FROM proof.tickets t JOIN proof.issuance_epoch e "
                    "ON e.issuer_namespace_id = t.issuer_namespace_id WHERE t.request_id = 'req-e2')")
    ok = st["ok"] and st_c["ok"] and row == "1,2"
    att.check("T06-E2-STALE-SNAPSHOT-COMMIT", T, "COUNTEREXAMPLE_REPRODUCED",
              "A snapshot read of the epoch commits a ticket under epoch 1 after epoch 2 was committed",
              ok, {"issue": st["stdout"], "commit": st_c["stdout"], "ticket_epoch,current_epoch": row})
    out["stale_commit_reproduced"] = ok

    admin_ns(adm, "ns-e2b", ISSUER_B)
    ib.exec("E2b BEGIN RR + read epoch", "BEGIN ISOLATION LEVEL REPEATABLE READ;\n"
            "SELECT epoch FROM proof.issuance_epoch WHERE issuer_namespace_id = 'ns-e2b';", test=T)
    adm.exec("E2b epoch change commits", "UPDATE proof.issuance_epoch SET epoch = 2 WHERE issuer_namespace_id = 'ns-e2b';", test=T)
    st = ib.exec("E2b FOR UPDATE issue under RR", f"SELECT proof.issue_ticket('ns-e2b', 'req-e2b', {h('req-e2b')}, 1);", expect="40001", test=T)
    ib.exec("E2b ROLLBACK", "ROLLBACK;", test=T)
    att.check("T06-E2B-RR-FOR-UPDATE", T, "DENIAL", "FOR UPDATE on a concurrently changed epoch row under RR fails with 40001",
              first_state(st) == "40001", st["stderr"])
    out["rr_for_update_40001"] = first_state(st) == "40001"

    admin_ns(adm, "ns-e2c", ISSUER_B)
    adm.exec("E2c epoch change commits", "UPDATE proof.issuance_epoch SET epoch = 2 WHERE issuer_namespace_id = 'ns-e2c';", test=T)
    st = ib.exec("E2c READ COMMITTED issue with stale expected epoch",
                 f"SELECT proof.issue_ticket('ns-e2c', 'req-e2c', {h('req-e2c')}, 1);", expect="P0001", test=T)
    att.check("T06-E2C-RC-STALE-REJECTED", T, "DENIAL", "A statement that starts after the epoch change is rejected (STALE_EPOCH)",
              first_state(st) == "P0001" and "STALE_EPOCH" in st["stderr"], st["stderr"])
    ib.close()
    naive.close()

    # E4: controller dies mid-transition; outcome unknown to the client; gate stays closed.
    admin_ns(adm, "ns-e4", ISSUER)
    f2 = Session(att, "e4_controller_crash", FENCING, app="proof-fencing")
    f2.backend_pid()
    f2.exec("E4 phase A close", "SELECT proof.set_handover('ns-e4', 'HANDOVER_PENDING');", test=T)
    st = f2.exec("E4 BEGIN transition (uncommitted)", "BEGIN;\nSELECT proof.set_handover('ns-e4', 'QUIESCED');\n"
                 "SELECT proof.transition_epoch('ns-e4', 1, 'proof_issuer_b_login');", test=T)
    v = adm.value("E4 harness terminates controller mid-transition", f"pg_terminate_backend({f2.pid}, 5000)")
    gone = not pid_present(adm, f2.pid, "E4 controller pid gone")
    st_c = f2.exec("E4 controller COMMIT after crash", "COMMIT;", test=T, timeout=10)
    row = adm.value("E4 persisted state", "(SELECT h.state || ',' || h.gate || ',' || e.epoch || ',' || e.holder FROM proof.handover_state h "
                    "JOIN proof.issuance_epoch e USING (issuer_namespace_id) WHERE issuer_namespace_id = 'ns-e4')")
    f3 = Session(att, "e4_controller_retry", FENCING, app="proof-fencing")
    g = f3.value("E4 try_open_gate after crash", "proof.try_open_gate('ns-e4')", test=T)
    f3.close()
    f2.close()
    ok = st["ok"] and v == "true" and gone and not st_c["done"] and row == f"HANDOVER_PENDING,CLOSED,1,{ISSUER}" and g == "CLOSED:STATE_HANDOVER_PENDING"
    att.check("T06-E4-UNKNOWN-OUTCOME", T, "POSITIVE_CONTROL",
              "Controller terminated mid-transition: client never sees COMMIT, epoch unchanged, state HANDOVER_PENDING, gate CLOSED",
              ok, {"transition_in_txn": st["stdout"], "terminated": v, "pid_gone": gone,
                   "client_commit_done": st_c["done"], "client_stderr": st_c["stderr"], "persisted": row, "try_open_gate": g})
    out["unknown_outcome_closed"] = ok
    return out


def s7_bypass(att: Attempt) -> dict:
    T = "T07_SHARED_LOGIN_AND_DIRECT_CONTROLLER"
    adm = att.adm
    out = {}
    sh = Session(att, "sh_shared_superuser", SHARED, app="proof-shared")
    sh.backend_pid()
    st1 = sh.exec("shared login direct INSERT stale ticket with forged issued_by",
                  "INSERT INTO proof.tickets (issuer_namespace_id, request_id, payload_hash, issued_epoch, issued_by) "
                  "VALUES ('ns-main', 'req-shared-direct', repeat('a', 64), 1, 'proof_issuer_login');", test=T)
    st2 = sh.exec("shared login opens the closed gate directly",
                  "UPDATE proof.handover_state SET gate = 'OPEN' WHERE issuer_namespace_id = 'ns-main';", test=T)
    st3 = sh.exec("shared login assumes issuer identity",
                  "SET SESSION AUTHORIZATION proof_issuer_login;\nSELECT 'VAL=' || session_user || ',' || current_user;\n"
                  "RESET SESSION AUTHORIZATION;", test=T)
    st4 = sh.exec("shared login re-enables old issuer LOGIN", "ALTER ROLE proof_issuer_login LOGIN;", test=T)
    rc = connect_attempt(att, "old issuer reconnect after shared login undid the ban", ISSUER, test=T)
    adm.exec("create test-only probe: unrestricted terminate via pg_signal_backend",
             "SET ROLE proof_fence_owner;\n"
             "BEGIN;\nCREATE FUNCTION proof_fence.probe_terminate_any(p_pid int) RETURNS boolean LANGUAGE sql "
             "SECURITY DEFINER SET search_path = pg_catalog AS 'SELECT pg_terminate_backend(p_pid)';\n"
             "REVOKE ALL ON FUNCTION proof_fence.probe_terminate_any(int) FROM PUBLIC;\n"
             "GRANT EXECUTE ON FUNCTION proof_fence.probe_terminate_any(int) TO proof_fencing_login;\nCOMMIT;\nRESET ROLE;", test=T)
    fen = Session(att, "fen_probe", FENCING, app="proof-fencing")
    st5 = fen.exec("pg_signal_backend holder terminates a superuser backend",
                   f"SELECT proof_fence.probe_terminate_any({sh.pid});", expect="42501", test=T)
    sh_alive = pid_present(adm, sh.pid, "shared superuser backend still present")
    fen.close()
    bypass = st1["ok"] and st2["ok"] and st3["ok"] and f"VAL={ISSUER},{ISSUER}" in st3["stdout"] and st4["ok"] and rc["connected"]
    att.check("T07-E-SHARED-LOGIN-BYPASS", T, "COUNTEREXAMPLE_REPRODUCED",
              "A shared superuser login bypasses every control: forged direct INSERT, gate reopened, issuer identity "
              "assumed, NOLOGIN undone and old issuer reconnects",
              bypass, {"direct_insert": st1["stdout"], "gate_update": st2["stdout"], "assume_identity": st3["stdout"],
                       "alter_login": st4["stdout"], "old_issuer_reconnect": rc["value"]})
    att.check("T07-E-SUPERUSER-NOT-TERMINABLE", T, "COUNTEREXAMPLE_REPRODUCED",
              "pg_signal_backend cannot terminate a superuser backend (42501); the shared login stays connected",
              first_state(st5) == "42501" and sh_alive, {"stderr": st5["stderr"], "still_present": sh_alive})
    out["shared_bypass"] = bypass and first_state(st5) == "42501" and sh_alive
    sh.close()

    fd = Session(att, "fd_direct_controller", FENCING_DIRECT, app="proof-fencing-direct", extra_env=("PROOF_PW_TAKEOVER",))
    st6 = fd.exec("direct CREATEROLE controller resets issuer_b password",
                  "\\getenv pw PROOF_PW_TAKEOVER\nALTER ROLE proof_issuer_b_login PASSWORD :'pw';", test=T)
    rc = connect_attempt(att, "login as issuer_b with controller-chosen password", ISSUER_B, pw_env="PROOF_PW_TAKEOVER", test=T)
    att.check("T07-F-DIRECT-CONTROLLER-TAKEOVER", T, "COUNTEREXAMPLE_REPRODUCED",
              "A controller holding CREATEROLE + ADMIN on an issuer role can reset its password and log in as that issuer",
              st6["ok"] and rc["connected"], {"alter": st6["stdout"], "stderr": st6["stderr"], "login": rc["value"]})
    st7 = fd.exec("direct controller self-grants issuer_b WITH SET TRUE",
                  "GRANT proof_issuer_b_login TO proof_fencing_direct_login WITH INHERIT TRUE, SET TRUE;", test=T)
    st8 = fd.exec("direct controller SET ROLE issuer_b after self-grant",
                  "SET ROLE proof_issuer_b_login;\nSELECT 'VAL=' || session_user || ',' || current_user;", test=T)
    att.observe("T07-F-DIRECT-CONTROLLER-SELF-GRANT", T,
                "Outcome of a CREATEROLE + ADMIN controller granting the issuer role to itself WITH SET TRUE, then SET ROLE",
                {"grant_sqlstates": st7["sqlstates"], "grant_stdout": st7["stdout"], "grant_stderr": st7["stderr"],
                 "set_role_sqlstates": st8["sqlstates"], "set_role_stdout": st8["stdout"]})
    out["direct_takeover"] = st6["ok"] and rc["connected"]
    fd.close()
    return out


def final_state(att: Attempt) -> None:
    adm = att.adm
    st = adm.exec("final state dump",
                  "SELECT 'STATE_JSON ' || json_build_object("
                  "'tickets', (SELECT json_agg(t ORDER BY ticket_id) FROM (SELECT ticket_id, issuer_namespace_id, request_id, status, issued_epoch, issued_by FROM proof.tickets) t),"
                  "'epochs', (SELECT json_agg(e ORDER BY issuer_namespace_id) FROM proof.issuance_epoch e),"
                  "'handover', (SELECT json_agg(h ORDER BY issuer_namespace_id) FROM proof.handover_state h),"
                  "'roles', (SELECT json_agg(json_build_object('rolname', rolname, 'rolcanlogin', rolcanlogin) ORDER BY rolname) FROM pg_roles WHERE rolname LIKE 'proof%'))::jsonb::text;")
    m = re.search(r"STATE_JSON (\{.*\})", st["stdout"])
    att.write("final_state.json", json.dumps(json.loads(m.group(1)), indent=2, sort_keys=True) + "\n" if m else "{}\n")


def server_log_checks(att: Attempt) -> None:
    log = (att.dir / "server_log.txt").read_text(encoding="utf-8")
    n_28000 = len(re.findall(rf"sqlstate=28000 user={ISSUER} .*FATAL:\s+role \"{ISSUER}\" is not permitted to log in", log))
    n_42501 = len(re.findall(rf"sqlstate=42501 user={ISSUER_B} .*FATAL:\s+permission denied for database", log))
    n_57p01 = len(re.findall(rf"sqlstate=57P01 user={ISSUER} .*FATAL:\s+terminating connection due to administrator command", log))
    att.check("T08-LOG-28000", "T08_SERVER_LOG", "POSITIVE_CONTROL",
              "Server log records exactly 8 old-issuer reconnect denials with SQLSTATE 28000 "
              "(1 phase-B verification + 5 pool reconnects + 2 post-handover attempts)",
              n_28000 == 8, n_28000)
    att.check("T08-LOG-42501", "T08_SERVER_LOG", "POSITIVE_CONTROL",
              "Server log records the REVOKE CONNECT denial for issuer_b with SQLSTATE 42501", n_42501 >= 1, n_42501)
    att.check("T08-LOG-57P01", "T08_SERVER_LOG", "POSITIVE_CONTROL",
              "Server log records administrator termination (57P01) for exactly the four old-issuer backends", n_57p01 == 4, n_57p01)


def secret_scan(att: Attempt) -> None:
    hits = []
    for p in att.dir.rglob("*"):
        if p.is_file():
            text = p.read_text(encoding="utf-8", errors="replace")
            for s in att.redact.secrets:
                if s in text:
                    hits.append(str(p.relative_to(att.dir)))
    att.check("T09-SECRET-SCAN", "T09_HYGIENE", "POSITIVE_CONTROL",
              "No generated password value appears in any evidence file of this attempt", not hits, hits or "none")


def run_attempt(index: int, out: pathlib.Path) -> dict:
    att = Attempt(index, out)
    att.meta["t_start"] = now_iso()
    results: dict = {}
    error = None
    try:
        start_container(att)
        att.adm = Session(att, "adm_observer", ADMIN, tcp=False, app="proof-observer")
        att.adm.backend_pid()
        s1_identity_and_catalog(att, att.adm)
        s2_set_role(att)
        results["t03"] = s3_direct_dml(att)
        results["t04"] = s4_handover(att)
        results["t05"] = s5_insufficient_bans(att)
        results["t06"] = s6_epoch(att)
        results["t07"] = s7_bypass(att)
        final_state(att)
    except Exception as exc:  # recorded, never hidden
        error = f"{type(exc).__name__}: {exc}"
        att.check("HARNESS-ERROR", "HARNESS", "POSITIVE_CONTROL", "Harness completed without exception", False, error)
    finally:
        for s in list(att.sessions):
            if s.exit_code is None and s.p.poll() is None:
                s.close()
        remove_container(att)
    server_log_checks(att)
    att.check("T09-CONTAINER-REMOVED", "T09_HYGIENE", "POSITIVE_CONTROL",
              "The attempt container was removed and no container with its name remains", att.meta["container_removed"],
              att.meta["container_removed"])
    att.meta["t_end"] = now_iso()
    att.meta["harness_error"] = error
    att.write("steps.jsonl", "".join(json.dumps(s, sort_keys=True, default=str) + "\n" for s in att.steps))
    att.write("timeline.json", json.dumps(att.timeline, indent=2, default=str) + "\n")
    att.write("results.json", json.dumps(results, indent=2, sort_keys=True, default=str) + "\n")
    secret_scan(att)
    att.meta["redaction_replacements"] = att.redact.hits
    att.write("assertions.json", json.dumps(att.assertions, indent=2, default=str) + "\n")
    att.write("run_meta.json", json.dumps(att.meta, indent=2, sort_keys=True, default=str) + "\n")
    write_results_md(att)
    secret_scan_final = [p for p in att.dir.rglob("*") if p.is_file()
                         and any(s in p.read_text(encoding="utf-8", errors="replace") for s in att.redact.secrets)]
    if secret_scan_final:
        raise SystemExit(f"secret leak detected in attempt {index}")
    return {"attempt": index, "meta": att.meta, "results": results, "assertions": att.assertions}


def write_results_md(att: Attempt) -> None:
    rows = ["| ID | Kind | Result | Description |", "| --- | --- | --- | --- |"]
    for a in att.assertions:
        rows.append(f"| `{a['id']}` | {a['kind']} | {a['result']} | {a['description'].replace('|', '/')} |")
    counts = {}
    for a in att.assertions:
        counts[a["result"]] = counts.get(a["result"], 0) + 1
    text = (f"# Attempt {att.index} results\n\n"
            f"- Container: `{att.container}` (network `none`, no published ports), removed: {att.meta.get('container_removed')}\n"
            f"- Databases: `{att.main_db}`, `{att.pub_db}`\n"
            f"- Server: {att.meta.get('server_version')}\n"
            f"- Started: {att.meta.get('t_start')}; finished: {att.meta.get('t_end')}\n"
            f"- Harness error: {att.meta.get('harness_error')}\n"
            f"- Assertion counts: {json.dumps(counts, sort_keys=True)}\n\n"
            "`COUNTEREXAMPLE_REPRODUCED` with PASS means the unsafe behavior was observed as predicted. "
            "`OBSERVATION` rows record an outcome without a pass/fail expectation.\n\n" + "\n".join(rows) + "\n")
    att.write("RESULTS.md", text)


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
        r = run_attempt(i, out)
        counts = {}
        for a in r["assertions"]:
            counts[a["result"]] = counts.get(a["result"], 0) + 1
        print(f"attempt {i}: {counts}", flush=True)
        summary.append({"attempt": i, "counts": counts, "results": r["results"],
                        "failed": [a["id"] for a in r["assertions"] if a["result"] == "FAIL"],
                        "container": r["meta"]["container"], "removed": r["meta"].get("container_removed"),
                        "server_version": r["meta"].get("server_version"),
                        "network_mode": r["meta"].get("inspect_network_mode"),
                        "port_bindings": r["meta"].get("inspect_port_bindings"),
                        "image_id": r["meta"].get("image_id")})
    (out / "summary.json").write_text(json.dumps(summary, indent=2, sort_keys=True, default=str) + "\n",
                                      encoding="utf-8", newline="\n")
    return 0 if all(not s["failed"] for s in summary) else 1


if __name__ == "__main__":
    sys.exit(main())
