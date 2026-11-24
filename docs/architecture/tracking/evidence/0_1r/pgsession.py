"""Minimal psql session driver for the 0.1R disposable PostgreSQL container.

Runs psql inside the container through `docker exec -i` over the Unix socket.
Each step is followed by \\echo and \\warn markers so stdout and stderr can be
split per step.
"""

from __future__ import annotations

import re
import subprocess
import threading
import time

SQLSTATE_RE = re.compile(r"\b(ERROR|FATAL):\s+([0-9A-Z]{5}):")
LOST = ("connection to server was lost", "server closed the connection unexpectedly")


def pg_query(container: str, role: str, db: str, sql: str, app: str, timeout: int = 60) -> dict:
    cmd = ["docker", "exec", "-e", "PGAPPNAME=" + app, container, "psql", "-X", "-A", "-t",
           "-v", "VERBOSITY=verbose", "-v", "ON_ERROR_STOP=1", "-U", role, "-d", db, "-c", sql]
    p = subprocess.run(cmd, capture_output=True, timeout=timeout)
    out = p.stdout.decode("utf-8", "replace").replace("\r\n", "\n")
    err = p.stderr.decode("utf-8", "replace").replace("\r\n", "\n")
    return {"argv": cmd, "exit": p.returncode, "stdout": out, "stderr": err,
            "sqlstates": [m.group(2) for m in SQLSTATE_RE.finditer(err)]}


class PgSession:
    def __init__(self, container: str, role: str, db: str, app: str) -> None:
        self.argv = ["docker", "exec", "-i", "-e", "PGAPPNAME=" + app, container, "stdbuf", "-o0", "-e0",
                     "psql", "-X", "-v", "VERBOSITY=verbose", "-P", "pager=off", "-U", role, "-d", db]
        self.p = subprocess.Popen(self.argv, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.out: list[str] = []
        self.err: list[str] = []
        self.cv = threading.Condition()
        self.n = 0
        self.op = 0
        self.ep = 0
        threading.Thread(target=self._read, args=(self.p.stdout, self.out), daemon=True).start()
        threading.Thread(target=self._read, args=(self.p.stderr, self.err), daemon=True).start()

    def _read(self, stream, sink) -> None:
        for raw in iter(stream.readline, b""):
            with self.cv:
                sink.append(raw.decode("utf-8", "replace").rstrip("\r\n"))
                self.cv.notify_all()
        with self.cv:
            sink.append("\x00EOF")
            self.cv.notify_all()

    def send_nowait(self, sql: str) -> None:
        self.p.stdin.write((sql + "\n").encode("utf-8"))
        self.p.stdin.flush()

    def exec(self, sql: str, timeout: float = 20.0) -> dict:
        self.n += 1
        marker = f"__S{self.n}__"
        try:
            self.p.stdin.write(f"{sql}\n\\echo {marker}\n\\warn {marker}\n".encode("utf-8"))
            self.p.stdin.flush()
        except OSError:
            pass
        deadline = time.monotonic() + timeout
        with self.cv:
            while True:
                done = marker in self.out[self.op:] and marker in self.err[self.ep:]
                eof = bool(self.out) and self.out[-1] == "\x00EOF" and bool(self.err) and self.err[-1] == "\x00EOF"
                if done or eof or time.monotonic() > deadline:
                    break
                self.cv.wait(0.2)
            oi = self.out.index(marker, self.op) if marker in self.out[self.op:] else len(self.out)
            ei = self.err.index(marker, self.ep) if marker in self.err[self.ep:] else len(self.err)
            out = [l for l in self.out[self.op:oi] if l != "\x00EOF"]
            err = [l for l in self.err[self.ep:ei] if l != "\x00EOF"]
            self.op, self.ep = oi + 1, ei + 1
        stderr = "\n".join(err)
        states = [m.group(2) for m in SQLSTATE_RE.finditer(stderr)]
        return {"sql": sql, "done": done, "stdout": "\n".join(out), "stderr": stderr, "sqlstates": states,
                "ok": done and not states, "lost": (not done) or any(x in stderr for x in LOST)}

    def abandon(self) -> None:
        """Close stdin without reading any further output (client never sees the reply)."""
        try:
            self.p.stdin.close()
        except OSError:
            pass

    def close(self) -> int | None:
        try:
            self.p.stdin.write(b"\\q\n")
            self.p.stdin.flush()
            self.p.stdin.close()
        except OSError:
            pass
        try:
            return self.p.wait(timeout=15)
        except subprocess.TimeoutExpired:
            self.p.kill()
            return self.p.wait()
