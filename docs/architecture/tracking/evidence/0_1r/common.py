"""Shared helpers for the BINTRANS 0.1R test-only witness model.

Not a production witness. Requests are signed with per-principal HMAC keys that
exist only in process memory for one attempt; keys are never written to disk.
"""

from __future__ import annotations

import hashlib
import hmac
import json
import os
import secrets
import tempfile
from multiprocessing import AuthenticationError
from multiprocessing.connection import Client

STATES = ("RESERVED", "COMMIT_CONFIRMED", "ABORT_CONFIRMED", "IN_DOUBT", "PERMANENTLY_DENIED")
OUTCOMES = ("COMMIT_CONFIRMED", "ABORT_CONFIRMED", "IN_DOUBT", "PERMANENTLY_DENIED")

ROLE = {
    "issuer_a": "issuer",
    "issuer_b": "issuer",
    "operator": "operator",
    "auditor": "reader",
    "harness": "fault",
}

# (role, from_state, to_state) transitions the witness accepts. Anything else is
# INVALID_TRANSITION. IN_DOUBT -> ABORT_CONFIRMED is deliberately absent.
ALLOWED = {
    ("issuer", "RESERVED", "COMMIT_CONFIRMED"),
    ("issuer", "RESERVED", "ABORT_CONFIRMED"),
    ("issuer", "RESERVED", "IN_DOUBT"),
    ("issuer", "ABORT_CONFIRMED", "COMMIT_CONFIRMED"),
    ("operator", "RESERVED", "IN_DOUBT"),
    ("operator", "IN_DOUBT", "COMMIT_CONFIRMED"),
    ("operator", "IN_DOUBT", "PERMANENTLY_DENIED"),
    ("operator", "ABORT_CONFIRMED", "PERMANENTLY_DENIED"),
}


def canon(obj) -> bytes:
    return json.dumps(obj, sort_keys=True, separators=(",", ":"), ensure_ascii=True).encode("utf-8")


def sha256_hex(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def sign(key_hex: str, obj) -> str:
    return hmac.new(bytes.fromhex(key_hex), canon(obj), hashlib.sha256).hexdigest()


def verify(key_hex: str, obj, mac: str) -> bool:
    return hmac.compare_digest(sign(key_hex, obj), mac or "")


def pipe_address(name: str) -> tuple[str, str]:
    if os.name == "nt":
        return "\\\\.\\pipe\\" + name, "AF_PIPE"
    return os.path.join(tempfile.gettempdir(), name + ".sock"), "AF_UNIX"


def payload_hash(ns: str, request_id: str, payload: dict) -> str:
    """0.1P R6 canonical payload: UTF-8 lines in the fixed order, SHA-256."""
    lines = [
        ns,
        request_id,
        payload["operation"],
        payload["sub"],
        payload["tenant_id"],
        payload["shipment"],
        ",".join(sorted(payload["execution_ids"])),
        str(payload["expected_epoch"]),
        str(payload["expected_revision"]),
    ]
    return sha256_hex("\n".join(lines).encode("utf-8"))


class WitnessUnavailable(Exception):
    pass


class WitnessClient:
    def __init__(self, pipe_name: str, authkey_hex: str, principal: str, key_hex: str, timeout: float = 5.0) -> None:
        self.address, self.family = pipe_address(pipe_name)
        self.authkey = bytes.fromhex(authkey_hex)
        self.principal = principal
        self.key = key_hex
        self.timeout = timeout

    def call(self, op: str, **body) -> dict:
        req = {"op": op, "principal": self.principal, "nonce": secrets.token_hex(8), "body": body}
        req["mac"] = sign(self.key, {k: req[k] for k in ("op", "principal", "nonce", "body")})
        try:
            conn = Client(self.address, family=self.family, authkey=self.authkey)
        except (OSError, EOFError, AuthenticationError) as exc:
            raise WitnessUnavailable(f"connect:{type(exc).__name__}") from None
        try:
            conn.send(req)
            if not conn.poll(self.timeout):
                raise WitnessUnavailable("timeout")
            return conn.recv()
        except (OSError, EOFError) as exc:
            raise WitnessUnavailable(f"io:{type(exc).__name__}") from None
        finally:
            try:
                conn.close()
            except OSError:
                pass
