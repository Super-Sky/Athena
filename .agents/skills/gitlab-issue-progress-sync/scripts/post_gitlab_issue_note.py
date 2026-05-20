#!/usr/bin/env python3
"""Compose and optionally post a structured progress note back to a GitLab issue."""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
from typing import Iterable
from urllib.parse import quote
from urllib.request import Request, urlopen


ISSUE_URL_RE = re.compile(
    r"^(?P<base>https?://[^/]+)/(?P<project>.+)/-/issues/(?P<iid>\d+)$"
)
PROJECT_REF_RE = re.compile(r"^(?P<project>[^#]+)#(?P<iid>\d+)$")


def parse_issue_ref(raw: str) -> tuple[str | None, str, str]:
    raw = raw.strip()
    url_match = ISSUE_URL_RE.match(raw)
    if url_match:
        return (
            url_match.group("base"),
            url_match.group("project"),
            url_match.group("iid"),
        )
    ref_match = PROJECT_REF_RE.match(raw)
    if ref_match:
        return None, ref_match.group("project"), ref_match.group("iid")
    raise ValueError(f"unsupported issue reference: {raw}")


def split_items(values: Iterable[str]) -> list[str]:
    items: list[str] = []
    for raw in values:
        for piece in raw.split(";"):
            piece = piece.strip()
            if piece:
                items.append(piece)
    return items


def build_note(
    *,
    event: str,
    branch: str,
    commit: str | None,
    mr_url: str | None,
    completed: list[str],
    pending: list[str],
    next_steps: list[str],
) -> str:
    title = {
        "push": "当前仓工作分支已推送，准备进入审查：",
        "mr": "当前仓 MR 已创建，进入评审：",
        "merge": "当前仓变更已合入主线：",
    }[event]

    lines = [title, ""]
    lines.append(f"- 分支：`{branch}`")
    if commit:
        lines.append(f"- 提交：`{commit}`")
    if mr_url:
        lines.append(f"- MR：{mr_url}")

    if completed:
        lines.extend(["", "本仓已完成：", ""])
        lines.extend([f"- {item}" for item in completed])

    if pending:
        lines.extend(["", "本仓未完成或仍待处理：", ""])
        lines.extend([f"- {item}" for item in pending])

    if next_steps:
        lines.extend(["", "下一步：", ""])
        lines.extend([f"- {item}" for item in next_steps])

    return "\n".join(lines).strip() + "\n"


def glab_available() -> bool:
    return shutil.which("glab") is not None


def post_with_glab(project: str, iid: str, body: str) -> None:
    cmd = [
        "glab",
        "api",
        f"projects/{quote(project, safe='')}/issues/{iid}/notes",
        "--method",
        "POST",
        "--field",
        f"body={body}",
    ]
    subprocess.check_call(cmd)


def post_with_http(base: str, project: str, iid: str, body: str) -> None:
    token = os.getenv("GITLAB_TOKEN") or os.getenv("GITLAB_PRIVATE_TOKEN")
    if not token:
        raise RuntimeError("missing GITLAB_TOKEN or GITLAB_PRIVATE_TOKEN")

    url = f"{base}/api/v4/projects/{quote(project, safe='')}/issues/{iid}/notes"
    data = json.dumps({"body": body}).encode("utf-8")
    req = Request(url, data=data, method="POST")
    req.add_header("PRIVATE-TOKEN", token)
    req.add_header("Content-Type", "application/json")
    with urlopen(req) as resp:
        resp.read()


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Compose and optionally post a progress note to one GitLab issue."
    )
    parser.add_argument("--issue", required=True)
    parser.add_argument("--event", choices=["push", "mr", "merge"], required=True)
    parser.add_argument("--branch", required=True)
    parser.add_argument("--commit")
    parser.add_argument("--mr-url")
    parser.add_argument("--completed", action="append", default=[])
    parser.add_argument("--pending", action="append", default=[])
    parser.add_argument("--next", dest="next_steps", action="append", default=[])
    parser.add_argument("--post", action="store_true")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        base, project, iid = parse_issue_ref(args.issue)
        body = build_note(
            event=args.event,
            branch=args.branch,
            commit=args.commit,
            mr_url=args.mr_url,
            completed=split_items(args.completed),
            pending=split_items(args.pending),
            next_steps=split_items(args.next_steps),
        )
        if args.post:
            if glab_available():
                try:
                    post_with_glab(project, iid, body)
                    return 0
                except Exception:
                    pass
            if not base:
                raise RuntimeError(
                    "missing issue base URL for HTTP fallback; use issue URL or configure glab"
                )
            post_with_http(base, project, iid, body)
        else:
            print(body, end="")
        return 0
    except Exception as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
