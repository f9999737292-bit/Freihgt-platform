"""Shared helpers for the BINTRANS 0.1S test-only witness + high-water model.

Not a production witness. Requests are signed with per-principal HMAC keys that
exist only in process memory for one attempt; keys are never written to disk.
The witness journal and the high-water authority use separate key sets and
separate pipe authkeys.
"""

from __future__ import annotations

import hashlib
import hmac
import json
import os
import secrets
import tempfile
import time
from multiprocessing import AuthenticationError
from multiprocessing.connection import Client

GENESIS_HASH = "0" * 64
STATES = ("RESERVED", "COMMIT_CONFIRMED", "ABORT_CONFIRMED", "IN_DOUBT", "PERMANENTLY_DENIED")
OUTCOMES = ("COMMIT_CONFIRMED", "ABORT_CONFIRMED", "IN_DOUBT", "PERMANENTLY_DENIED")

# Witness journal principals.
ROLE = {
    "issuer_a": "issuer",
    "issuer_b": "issuer",
    "operator": "operator",
    "auditor": "reader",
    "harness": "fault",
}

# High-water authority principals. Only the witness service may write marks;
# issuers, operator and auditor may only read.
HW_ROLE = {
    "witness_svc": "writer",
    "issuer_a": "reader",
    "issuer_b": "reader",
    "operator": "reader",
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


def signed_request(principal: str, key_hex: str, op: str, body: dict) -> dict:
    req = {"op": op, "principal": principal, "nonce": secrets.token_hex(8), "body": body}
    req["mac"] = sign(key_hex, {k: req[k] for k in ("op", "principal", "nonce", "body")})
    return req


def request_valid(keys: dict, req: dict | None) -> bool:
    if not isinstance(req, dict):
        return False
    key = keys.get(req.get("principal"))
    return key is not None and verify(key, {k: req.get(k) for k in ("op", "principal", "nonce", "body")}, req.get("mac"))


def build_record(prev_hash: str, seq: int, fields: dict, req: dict | None) -> dict:
    """Record layout shared by the witness journal and the high-water log."""
    rec = dict(fields)
    rec["seq"] = seq
    rec["prev_hash"] = prev_hash
    rec["ts"] = time.time()
    rec["request"] = req
    rec["hash"] = sha256_hex(canon(rec))
    return rec


def record_hash_ok(rec: dict) -> bool:
    return sha256_hex(canon({k: v for k, v in rec.items() if k != "hash"})) == rec.get("hash")


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
    """Signed request client for either server (witness journal or high-water authority)."""

    def __init__(self, pipe_name: str, authkey_hex: str, principal: str, key_hex: str, timeout: float = 5.0) -> None:
        self.address, self.family = pipe_address(pipe_name)
        self.authkey = bytes.fromhex(authkey_hex)
        self.principal = principal
        self.key = key_hex
        self.timeout = timeout

    def call(self, op: str, **body) -> dict:
        req = signed_request(self.principal, self.key, op, body)
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


def verify_chain(records: list[dict]) -> str | None:
    """Independent client-side check of an exported journal: sequence, links, hashes."""
    prev = GENESIS_HASH
    for i, rec in enumerate(records, 1):
        if rec.get("seq") != i:
            return f"SEQUENCE_VIOLATION seq={i}"
        if rec.get("prev_hash") != prev:
            return f"CHAIN_BROKEN seq={i}"
        if not record_hash_ok(rec):
            return f"HASH_MISMATCH seq={i}"
        prev = rec["hash"]
    return None
