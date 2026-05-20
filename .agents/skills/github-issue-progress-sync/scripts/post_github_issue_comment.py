#!/usr/bin/env python3
"""Compose and optionally post a structured progress comment back to a GitHub issue."""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import urllib.error
from typing import Iterable
from urllib.parse import urlparse
from urllib.request import Request, urlopen


ISSUE_URL_RE = re.compile(
    r"^(?P<base>https?://[^/]+)/(?P<project>[^/]+/[^/]+)/issues/(?P<number>\d+)$"
)
PROJECT_REF_RE = re.compile(r"^(?P<project>[^#]+)#(?P<number>\d+)$")


def normalize_project_path(raw: str) -> str:
    project = raw.strip().strip("/")
    if project.endswith(".git"):
        project = project[: -len(".git")]
    if project.count("/") != 1:
        raise ValueError(f"unsupported repository reference: {raw}")
    owner, repo = project.split("/", 1)
    if not owner or not repo:
        raise ValueError(f"unsupported repository reference: {raw}")
    return f"{owner}/{repo}"


def parse_issue_ref(raw: str) -> tuple[str | None, str, str]:
    raw = raw.strip()
    url_match = ISSUE_URL_RE.match(raw)
    if url_match:
        return (
            url_match.group("base"),
            normalize_project_path(url_match.group("project")),
            url_match.group("number"),
        )
    ref_match = PROJECT_REF_RE.match(raw)
    if ref_match:
        return None, normalize_project_path(ref_match.group("project")), ref_match.group("number")
    raise ValueError(f"unsupported issue reference: {raw}")


def resolve_base_url(explicit_base: str | None) -> str:
    if explicit_base:
        return explicit_base.rstrip("/")
    configured = os.getenv("GITHUB_BASE_URL", "").strip()
    if configured:
        return configured.rstrip("/")
    return "https://github.com"


def resolve_api_base(base_url: str) -> str:
    configured = os.getenv("GITHUB_API_BASE_URL", "").strip()
    if configured:
        return configured.rstrip("/")
    host = (urlparse(base_url).hostname or "").lower()
    if host == "github.com":
        return "https://api.github.com"
    if base_url.rstrip("/").endswith("/api/v3"):
        return base_url.rstrip("/")
    return f"{base_url.rstrip('/')}/api/v3"


def resolve_token() -> str:
    for key in ("GITHUB_TOKEN", "GH_TOKEN", "GITHUB_PAT"):
        token = os.getenv(key, "").strip()
        if token:
            return token
    raise RuntimeError("missing GitHub token; set GITHUB_TOKEN, GH_TOKEN, or GITHUB_PAT")


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
    pr_url: str | None,
    completed: list[str],
    pending: list[str],
    next_steps: list[str],
) -> str:
    title = {
        "push": "当前仓工作分支已推送，准备进入审查：",
        "pr": "当前仓 PR 已创建，进入评审：",
        "merge": "当前仓变更已合入主线：",
    }[event]

    lines = [title, ""]
    lines.append(f"- 分支：`{branch}`")
    if commit:
        lines.append(f"- 提交：`{commit}`")
    if pr_url:
        lines.append(f"- PR：{pr_url}")

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


def gh_available() -> bool:
    return shutil.which("gh") is not None


def post_with_gh(project: str, number: str, body: str) -> None:
    cmd = [
        "gh",
        "api",
        f"repos/{project}/issues/{number}/comments",
        "--method",
        "POST",
        "--field",
        f"body={body}",
    ]
    subprocess.check_call(cmd)


def post_with_http(api_base: str, project: str, number: str, body: str) -> None:
    url = f"{api_base}/repos/{project}/issues/{number}/comments"
    data = json.dumps({"body": body}).encode("utf-8")
    req = Request(url, data=data, method="POST")
    req.add_header("Authorization", f"Bearer {resolve_token()}")
    req.add_header("Accept", "application/vnd.github+json")
    req.add_header("X-GitHub-Api-Version", "2022-11-28")
    req.add_header("Content-Type", "application/json")
    try:
        with urlopen(req, timeout=30) as resp:
            resp.read()
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"github issue comment failed: {exc.code} {detail}") from exc


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Compose and optionally post a progress comment to one GitHub issue."
    )
    parser.add_argument("--issue", required=True)
    parser.add_argument("--event", choices=["push", "pr", "merge"], required=True)
    parser.add_argument("--branch", required=True)
    parser.add_argument("--commit")
    parser.add_argument("--pr-url")
    parser.add_argument("--mr-url", help="Compatibility alias for --pr-url")
    parser.add_argument("--completed", action="append", default=[])
    parser.add_argument("--pending", action="append", default=[])
    parser.add_argument("--next", dest="next_steps", action="append", default=[])
    parser.add_argument("--post", action="store_true")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        base, project, number = parse_issue_ref(args.issue)
        pr_url = args.pr_url or args.mr_url
        body = build_note(
            event=args.event,
            branch=args.branch,
            commit=args.commit,
            pr_url=pr_url,
            completed=split_items(args.completed),
            pending=split_items(args.pending),
            next_steps=split_items(args.next_steps),
        )
        if args.post:
            base_url = resolve_base_url(base)
            api_base = resolve_api_base(base_url)
            if gh_available():
                try:
                    post_with_gh(project, number, body)
                    return 0
                except Exception:
                    pass
            post_with_http(api_base, project, number, body)
        else:
            print(body, end="")
        return 0
    except Exception as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
