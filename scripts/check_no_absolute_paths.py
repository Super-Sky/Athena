#!/usr/bin/env python3
"""Fail when repository docs or skills contain personal absolute filesystem paths."""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


DEFAULT_TARGETS = [
    "docs",
    ".agents",
    ".claude",
    ".github",
    "README.md",
    "AGENTS.md",
    "CLAUDE.md",
]

SKIP_DIRS = {
    ".git",
    ".cache",
    "node_modules",
    "__pycache__",
    "dist",
    "build",
    "output",
    ".next",
}

SKIP_PATH_PARTS = {
    (".claude", "worktrees"),
}

PATTERNS = [
    re.compile(r"/Users/[^/\s]+/"),
    re.compile(r"/home/[^/\s]+/"),
    re.compile(r"[A-Za-z]:\\Users\\[^\\\s]+\\"),
]


def should_skip(path: Path) -> bool:
    if any(part in SKIP_DIRS for part in path.parts):
        return True
    parts = path.parts
    return any(
        parts[index : index + len(skip_parts)] == skip_parts
        for skip_parts in SKIP_PATH_PARTS
        for index in range(len(parts) - len(skip_parts) + 1)
    )


def iter_files(targets: list[str]) -> list[Path]:
    files: list[Path] = []
    for raw in targets:
        path = Path(raw)
        if not path.exists():
            continue
        if path.is_file():
            files.append(path)
            continue
        for child in path.rglob("*"):
            if child.is_file() and not should_skip(child):
                files.append(child)
    return files


def find_matches(path: Path) -> list[tuple[int, str, str]]:
    try:
        text = path.read_text(encoding="utf-8")
    except UnicodeDecodeError:
        return []
    results: list[tuple[int, str, str]] = []
    for line_no, line in enumerate(text.splitlines(), start=1):
        for pattern in PATTERNS:
            match = pattern.search(line)
            if match:
                results.append((line_no, match.group(0), line.strip()))
                break
    return results


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Check repository docs and skills for personal absolute paths."
    )
    parser.add_argument(
        "targets",
        nargs="*",
        default=DEFAULT_TARGETS,
        help="Files or directories to scan",
    )
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    failures: list[str] = []
    for path in iter_files(args.targets):
        for line_no, matched, line in find_matches(path):
            failures.append(f"{path}:{line_no}: matched {matched!r} in {line}")
    if failures:
        print("absolute path check failed:", file=sys.stderr)
        for failure in failures:
            print(f"  {failure}", file=sys.stderr)
        return 1
    print("absolute path check passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
