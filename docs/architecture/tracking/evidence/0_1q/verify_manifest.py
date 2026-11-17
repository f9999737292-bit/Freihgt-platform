#!/usr/bin/env python3
"""Generate or verify MANIFEST and SHA256SUMS for the 0.1Q evidence pack.

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
REQUIRED_TOP = ["README.md", "PRODUCT_DIFFERENCES.md", "run_proof.py", "verify_manifest.py",
                "sql/00_roles.sql", "sql/01_schema.sql", "sql/02_catalog.sql",
                "attempts/summary.json", "attempts/run_console.txt", "MANIFEST"]
REQUIRED_ATTEMPT = ["RESULTS.md", "assertions.json", "steps.jsonl", "timeline.json", "results.json",
                    "run_meta.json", "server_log.txt", "final_state.json", "catalog/catalog.json",
                    "catalog/02_catalog.stdout.txt", "catalog/02_catalog.stderr.txt", "catalog/02_catalog.exit",
                    "setup/00_roles.stdout.txt", "setup/00_roles.stderr.txt", "setup/00_roles.exit",
                    "setup/01_schema.stdout.txt", "setup/01_schema.stderr.txt", "setup/01_schema.exit",
                    "setup/01_inspect_network.stdout.txt", "setup/02_pg_hba.conf", "setup/99_docker_rm.exit",
                    "sessions/iss1_open_txn.stderr.txt", "sessions/iss1_open_txn.exit",
                    "sessions/fen_controller.input.sql", "sessions/fen_controller.stdout.txt"]


def git(*args: str) -> bytes:
    return subprocess.run(["git", "-C", str(HERE), *args], check=True, capture_output=True).stdout


def repo_prefix() -> str:
    return git("rev-parse", "--show-prefix").decode().strip()


def tracked() -> list[str]:
    out = git("ls-files", "-z", "--", ".").decode()
    return sorted(p for p in out.split("\0") if p)


def generate() -> None:
    files = sorted((set(tracked()) | {"MANIFEST"}) - {"SHA256SUMS"})
    lines = ["# BINTRANS 0.1Q evidence manifest", "# path<TAB>category", "# SHA256SUMS covers every path below.", ""]
    for p in files:
        cat = "attempt" if p.startswith("attempts/attempt_") else "aggregate" if p.startswith("attempts/") else \
              "sql" if p.startswith("sql/") else "harness" if p.endswith(".py") else "doc"
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
