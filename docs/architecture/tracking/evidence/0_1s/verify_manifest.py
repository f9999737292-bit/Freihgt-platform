#!/usr/bin/env python3
"""Generate or verify MANIFEST and SHA256SUMS for the 0.1S evidence pack.

--generate  writes MANIFEST and SHA256SUMS from files tracked by Git under this
            directory (plus MANIFEST itself).
--verify    hashes the committed Git blob bytes at --rev (default HEAD) and checks
            them against SHA256SUMS, then checks completeness: every tracked file
            except SHA256SUMS is listed, and every attempt has its required files.
"""

from __future__ import annotations

import argparse
import hashlib
import pathlib
import subprocess
import sys

HERE = pathlib.Path(__file__).resolve().parent
ATTEMPTS = ("attempt_1", "attempt_2", "attempt_3")
REQUIRED_TOP = ["README.md", "HIGH_WATER_PROTOCOL.md", "CRASH_WINDOW_MATRIX.md", "PRIVILEGED_BYPASS.md",
                "REMAINING_UNPROVEN.md", "run_proof.py", "witness_server.py", "hw_server.py", "server_base.py",
                "issuer_worker.py", "common.py", "pgsession.py", "verify_manifest.py", "sql/schema.sql",
                "attempts/summary.json", "attempts/run_console.txt", "MANIFEST"]
REQUIRED_ATTEMPT = ["RESULTS.md", "assertions.json", "run_meta.json", "pg_server_log.txt",
                    "setup/00_docker_run.cmd.txt", "setup/00_docker_run.exit", "setup/01_inspect.stdout.txt",
                    "setup/05_version.stdout.txt", "setup/99_docker_rm.exit", "traces/_order.json",
                    "traces/T1_CONSISTENT.json", "traces/T2_LAST_RECORD_DELETED.json",
                    "traces/T3_TRUNCATED_TO_CHECKPOINT.json", "traces/T4_RESTORED_ONE_BELOW.json",
                    "traces/T5_SAME_SEQ_OTHER_DIGEST.json", "traces/T6_OTHER_INCARNATION.json",
                    "traces/T7_HW_UNAVAILABLE.json", "traces/T8_HW_CORRUPTED.json", "traces/T9_HW_LOWER_REJECTED.json",
                    "traces/T10_CONCURRENT_RESERVE.json", "traces/J_WIT_001.json",
                    "traces/CW1_BEFORE_RESERVATION.json", "traces/CW2_AFTER_PREPARE_BEFORE_APPEND.json",
                    "traces/CW3_BETWEEN_APPEND_AND_HW.json", "traces/CW4_BETWEEN_HW_AND_APPEND.json",
                    "traces/CW5_AFTER_HW_BEFORE_REPLY.json", "traces/CW6_AFTER_PG_COMMIT.json",
                    "traces/CW7A_OUTCOME_BEFORE_APPEND.json", "traces/CW7B_OUTCOME_AFTER_APPEND_BEFORE_HW.json",
                    "traces/CW8_DURING_RECONCILIATION.json", "traces/STALE_PG_COMBINED.json",
                    "traces/STALE_PG_COMBINED_pg_restore.exit", "traces/P1_JOURNAL_ONLY.json",
                    "traces/P2_HIGH_WATER_ONLY.json", "traces/P3_BOTH_ROLLED_BACK.json", "traces/P4_WITNESS_CREDENTIAL.json",
                    "servers/jw-w/witness_log.jsonl", "servers/jw-w/tamper_jwit001.json", "servers/jw-hw/hw_log.jsonl",
                    "servers/jw-w/server_run2.stdout.txt", "servers/sp-w/witness_log.jsonl", "servers/sp-hw/hw_log.jsonl",
                    "servers/cw3-w/server_run2.stdout.txt", "servers/cw4-w/server_run2.stdout.txt"]


def git(*args: str) -> bytes:
    return subprocess.run(["git", "-C", str(HERE), *args], check=True, capture_output=True).stdout


def repo_prefix() -> str:
    return git("rev-parse", "--show-prefix").decode().strip()


def tracked() -> list[str]:
    out = git("ls-files", "-z", "--", ".").decode()
    return sorted(p for p in out.split("\0") if p)


def generate() -> None:
    files = sorted((set(tracked()) | {"MANIFEST"}) - {"SHA256SUMS"})
    lines = ["# BINTRANS 0.1S evidence manifest", "# path<TAB>category", "# SHA256SUMS covers every path below.", ""]
    for p in files:
        cat = "attempt" if p.startswith("attempts/attempt_") else "failed_debug_attempt" if p.startswith("attempts/debug_") \
              else "aggregate" if p.startswith("attempts/") else "sql" if p.startswith("sql/") \
              else "harness" if p.endswith(".py") else "doc"
        lines.append(f"{p}\t{cat}")
    lines.append("SHA256SUMS\tchecksums (not self-listed)")
    (HERE / "MANIFEST").write_text("\n".join(lines) + "\n", encoding="utf-8", newline="\n")
    sums = []
    for p in files:
        data = (HERE / p).read_bytes()
        sums.append(f"{hashlib.sha256(data).hexdigest()}  {p}")
    (HERE / "SHA256SUMS").write_text("\n".join(sums) + "\n", encoding="utf-8", newline="\n")
    print(f"generated MANIFEST and SHA256SUMS for {len(files)} files")


def verify(rev: str) -> int:
    prefix = repo_prefix()
    errors = []
    listed = {}
    sums_blob = git("cat-file", "blob", f"{rev}:{prefix}SHA256SUMS").decode()
    for line in sums_blob.splitlines():
        digest, path = line.split("  ", 1)
        listed[path] = digest
    ok = 0
    paths = sorted(listed)
    batch = subprocess.run(["git", "-C", str(HERE), "cat-file", "--batch"], check=True, capture_output=True,
                           input="".join(f"{rev}:{prefix}{p}\n" for p in paths).encode()).stdout
    pos = 0
    for path in paths:
        end = batch.index(b"\n", pos)
        header = batch[pos:end].decode().split()
        if len(header) != 3 or header[1] != "blob":
            errors.append(f"MISSING_BLOB {path}")
            pos = end + 1
            continue
        size = int(header[2])
        blob = batch[end + 1:end + 1 + size]
        pos = end + 1 + size + 1
        if hashlib.sha256(blob).hexdigest() == listed[path]:
            ok += 1
        else:
            errors.append(f"MISMATCH {path}")
    names = git("ls-tree", "-r", "--name-only", "--full-tree", rev, "--", prefix.rstrip("/")).decode().splitlines()
    in_tree = {n[len(prefix):] for n in names}
    for p in sorted(in_tree - {"SHA256SUMS"} - set(listed)):
        errors.append(f"UNLISTED {p}")
    for p in sorted(set(listed) - in_tree):
        errors.append(f"MISSING_FROM_TREE {p}")
    manifest = git("cat-file", "blob", f"{rev}:{prefix}MANIFEST").decode()
    manifest_paths = {l.split("\t", 1)[0] for l in manifest.splitlines() if l and not l.startswith("#")}
    for p in sorted(in_tree - manifest_paths):
        errors.append(f"NOT_IN_MANIFEST {p}")
    for p in REQUIRED_TOP:
        if p not in in_tree:
            errors.append(f"REQUIRED_MISSING {p}")
    for a in ATTEMPTS:
        for p in REQUIRED_ATTEMPT:
            if f"attempts/{a}/{p}" not in in_tree:
                errors.append(f"REQUIRED_MISSING attempts/{a}/{p}")
    print(f"rev={rev} listed={len(listed)} hash_ok={ok} tree_files={len(in_tree)}")
    for e in errors:
        print(e)
    print("SHA256_MANIFEST_VERIFIED=" + ("YES" if not errors else "NO"))
    return 0 if not errors else 1


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--generate", action="store_true")
    ap.add_argument("--verify", action="store_true")
    ap.add_argument("--rev", default="HEAD")
    args = ap.parse_args()
    if args.generate:
        generate()
    if args.verify:
        return verify(args.rev)
    return 0


if __name__ == "__main__":
    sys.exit(main())
