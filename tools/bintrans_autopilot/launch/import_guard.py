"""Static import boundary checks for launch package (RT-19b)."""

from __future__ import annotations

import ast
from pathlib import Path

_LAUNCH_ROOT = Path(__file__).resolve().parent
_FORBIDDEN_IMPORT_ROOTS = frozenset(
    {
        "urllib",
        "urllib.request",
        "http.client",
        "httpx",
        "aiohttp",
        "requests",
        "subprocess",
        "bintrans_autopilot.cursor_api",
        "bintrans_autopilot.budget",
        "bintrans_autopilot.budget_controller",
    }
)
_FORBIDDEN_CALLS = frozenset({"getenv", "getenviron"})


def _iter_py_files() -> list[Path]:
    return sorted(_LAUNCH_ROOT.rglob("*.py"))


def forbidden_import_violations() -> list[str]:
    violations: list[str] = []
    for path in _iter_py_files():
        tree = ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
        for node in ast.walk(tree):
            if isinstance(node, ast.Import):
                for alias in node.names:
                    root = alias.name.split(".", 1)[0]
                    if alias.name in _FORBIDDEN_IMPORT_ROOTS or root in {
                        "urllib",
                        "httpx",
                        "aiohttp",
                        "requests",
                        "subprocess",
                    }:
                        violations.append(f"{path.name}: import {alias.name}")
            elif isinstance(node, ast.ImportFrom):
                mod = node.module or ""
                if mod in _FORBIDDEN_IMPORT_ROOTS or mod.split(".", 1)[0] in {
                    "urllib",
                    "httpx",
                    "aiohttp",
                    "requests",
                    "subprocess",
                }:
                    violations.append(f"{path.name}: from {mod} import ...")
            elif isinstance(node, ast.Call) and isinstance(node.func, ast.Attribute):
                if node.func.attr in _FORBIDDEN_CALLS:
                    violations.append(f"{path.name}: os.{node.func.attr}(...)")
    return violations


def launch_package_loaded_cursor_api(modules_before: set[str], modules_after: set[str]) -> bool:
    loaded = modules_after - modules_before
    return any(name.startswith("bintrans_autopilot.cursor_api") for name in loaded)
