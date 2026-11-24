"""Process scaffolding shared by the 0.1S test-only servers.

Exclusive OS lock on the store for the process lifetime, JSON event log on
stdout, and a serialized request loop over a local named pipe (no network).
"""

from __future__ import annotations

import json
import os
import sys
import threading
import time
from multiprocessing import AuthenticationError
from multiprocessing.connection import Listener

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from common import pipe_address  # noqa: E402


def log(event: str, **kw) -> None:
    print(json.dumps({"t": time.time(), "event": event, **kw}, sort_keys=True, default=str), flush=True)


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


def append_line(fh, rec_bytes: bytes) -> None:
    fh.write(rec_bytes + b"\n")
    fh.flush()
    os.fsync(fh.fileno())


def serve(server, pipe: str, authkey: bytes, on_shutdown) -> int:
    """server.handle(req) -> resp is called under server.lock, one request at a time."""
    address, family = pipe_address(pipe)
    listener = Listener(address, family=family, authkey=authkey)

    def one(conn) -> None:
        try:
            req = conn.recv()
            with server.lock:
                try:
                    resp = server.handle(req)
                except Exception as exc:  # reported, never hidden
                    resp = {"status": "INTERNAL_ERROR", "error": f"{type(exc).__name__}: {exc}"}
                body = req.get("body") or {}
                log("request", op=req.get("op"), principal=req.get("principal"), body_ns=body.get("ns"),
                    body_request_id=body.get("request_id"), body_seq=body.get("seq"), status=resp.get("status"),
                    seq=resp.get("seq"), reason=resp.get("reason"))
                conn.send(resp)
                if resp.get("status") == "SHUTDOWN":
                    on_shutdown()
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
        threading.Thread(target=one, args=(conn,), daemon=True).start()
    return 0
