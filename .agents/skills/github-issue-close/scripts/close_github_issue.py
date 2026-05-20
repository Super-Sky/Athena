#!/usr/bin/env python3
"""Compose a final closeout comment and optionally close one GitHub issue."""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import urllib.error
from typing import Any, Iterable
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


def build_close_note(
    *,
    branch: str | None,
    commit: str | None,
    pr_url: str | None,
    issue_requirements: list[str],
    reconciled: list[str],
    completed: list[str],
    verification: list[str],
    remaining: list[str],
    decision: str,
) -> str:
    lines = ["Issue closeout reconciliation completed.", ""]
    if branch:
        lines.append(f"- 分支：`{branch}`")
    if commit:
        lines.append(f"- 提交：`{commit}`")
    if pr_url:
        lines.append(f"- PR：{pr_url}")

    if issue_requirements:
        lines.extend(["", "Issue 原始要求 / 完成标准：", ""])
        lines.extend([f"- {item}" for item in issue_requirements])

    if reconciled:
        lines.extend(["", "需求对账结果：", ""])
        lines.extend([f"- {item}" for item in reconciled])

    if completed:
        lines.extend(["", "已完成范围：", ""])
        lines.extend([f"- {item}" for item in completed])

    if verification:
        lines.extend(["", "验证证据：", ""])
        lines.extend([f"- {item}" for item in verification])

    if remaining:
        lines.extend(["", "仍未完成 / 后续动作：", ""])
        lines.extend([f"- {item}" for item in remaining])

    lines.extend(["", "关闭判断：", "", f"- {decision}"])
    return "\n".join(lines).strip() + "\n"


def close_allowed(*, remaining: list[str], allow_incomplete: bool) -> bool:
    return allow_incomplete or not remaining


def gh_available() -> bool:
    return shutil.which("gh") is not None


def post_comment_with_gh(project: str, number: str, body: str) -> None:
    subprocess.check_call(
        [
            "gh",
            "api",
            f"repos/{project}/issues/{number}/comments",
            "--method",
            "POST",
            "--field",
            f"body={body}",
        ]
    )


def close_with_gh(project: str, number: str) -> dict[str, Any]:
    out = subprocess.check_output(
        [
            "gh",
            "api",
            f"repos/{project}/issues/{number}",
            "--method",
            "PATCH",
            "--field",
            "state=closed",
        ],
        text=True,
        stderr=subprocess.STDOUT,
    )
    return json.loads(out)


def request_json(api_base: str, project: str, number: str, *, method: str, payload: dict[str, Any]) -> dict[str, Any]:
    url = f"{api_base}/repos/{project}/issues/{number}"
    if method == "POST":
        url = f"{url}/comments"
    req = Request(url, data=json.dumps(payload).encode("utf-8"), method=method)
    req.add_header("Authorization", f"Bearer {resolve_token()}")
    req.add_header("Accept", "application/vnd.github+json")
    req.add_header("X-GitHub-Api-Version", "2022-11-28")
    req.add_header("Content-Type", "application/json")
    try:
        with urlopen(req, timeout=30) as resp:
            body = resp.read().decode("utf-8")
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"github issue close request failed: {exc.code} {detail}") from exc
    return json.loads(body) if body else {}


def post_comment_with_http(api_base: str, project: str, number: str, body: str) -> None:
    request_json(api_base, project, number, method="POST", payload={"body": body})


def close_with_http(api_base: str, project: str, number: str) -> dict[str, Any]:
    return request_json(api_base, project, number, method="PATCH", payload={"state": "closed"})


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Preview and optionally close one GitHub issue.")
    parser.add_argument("--issue", required=True)
    parser.add_argument("--branch")
    parser.add_argument("--commit")
    parser.add_argument("--pr-url")
    parser.add_argument("--mr-url", help="Compatibility alias for --pr-url")
    parser.add_argument("--issue-requirement", action="append", default=[], help="Original issue requirement or completion criterion. Can be repeated or semicolon-separated.")
    parser.add_argument("--reconciled", action="append", default=[], help="Requirement-to-evidence reconciliation item. Can be repeated or semicolon-separated.")
    parser.add_argument("--completed", action="append", default=[])
    parser.add_argument("--verification", action="append", default=[])
    parser.add_argument("--remaining", action="append", default=[])
    parser.add_argument("--decision", required=True)
    parser.add_argument("--allow-incomplete", action="store_true", help="Allow remote close even when --remaining is present.")
    parser.add_argument("--post-note", action="store_true")
    parser.add_argument("--close", action="store_true")
    parser.add_argument("--json", action="store_true")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        base, project, number = parse_issue_ref(args.issue)
        issue_requirements = split_items(args.issue_requirement)
        reconciled = split_items(args.reconciled)
        completed = split_items(args.completed)
        verification = split_items(args.verification)
        remaining = split_items(args.remaining)
        pr_url = args.pr_url or args.mr_url
        if args.close and not issue_requirements:
            raise RuntimeError("refusing to close issue without original issue requirements; run issue-intake first and pass --issue-requirement")
        if args.close and not reconciled:
            raise RuntimeError("refusing to close issue without requirement reconciliation; pass --reconciled evidence mappings")
        note = build_close_note(
            branch=args.branch,
            commit=args.commit,
            pr_url=pr_url,
            issue_requirements=issue_requirements,
            reconciled=reconciled,
            completed=completed,
            verification=verification,
            remaining=remaining,
            decision=args.decision,
        )
        allowed = close_allowed(remaining=remaining, allow_incomplete=args.allow_incomplete)
        if args.close and not allowed:
            raise RuntimeError("refusing to close issue while remaining work is listed; remove --remaining or pass --allow-incomplete after explicit approval")

        result: dict[str, Any] = {
            "issue": args.issue,
            "project_path": project,
            "issue_number": number,
            "close_allowed": allowed,
            "note": note,
            "note_posted": False,
            "closed": False,
        }

        if args.post_note or args.close:
            base_url = resolve_base_url(base)
            api_base = resolve_api_base(base_url)
            if gh_available():
                try:
                    if args.post_note:
                        post_comment_with_gh(project, number, note)
                        result["note_posted"] = True
                    if args.close:
                        result["issue"] = close_with_gh(project, number)
                        result["closed"] = True
                except Exception:
                    if args.post_note:
                        post_comment_with_http(api_base, project, number, note)
                        result["note_posted"] = True
                    if args.close:
                        result["issue"] = close_with_http(api_base, project, number)
                        result["closed"] = True
            else:
                if args.post_note:
                    post_comment_with_http(api_base, project, number, note)
                    result["note_posted"] = True
                if args.close:
                    result["issue"] = close_with_http(api_base, project, number)
                    result["closed"] = True

        if args.json:
            print(json.dumps(result, ensure_ascii=False, indent=2))
        else:
            print(note, end="")
            print(f"\nClose allowed: {'yes' if allowed else 'no'}")
            if result["closed"]:
                issue = result.get("issue")
                if isinstance(issue, dict):
                    print(issue.get("html_url") or json.dumps(issue, ensure_ascii=False))
                else:
                    print("Issue closed")
        return 0
    except Exception as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
