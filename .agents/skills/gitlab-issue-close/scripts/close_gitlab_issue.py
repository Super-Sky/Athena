#!/usr/bin/env python3
"""Compose a final closeout note and optionally close one GitLab issue."""

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
        return url_match.group("base"), url_match.group("project"), url_match.group("iid")
    ref_match = PROJECT_REF_RE.match(raw)
    if ref_match:
        return None, ref_match.group("project"), ref_match.group("iid")
    raise ValueError(f"unsupported issue reference: {raw}")


def resolve_base_url(explicit_base: str | None) -> str:
    if explicit_base:
        return explicit_base.rstrip("/")
    configured = os.getenv("GITLAB_BASE_URL", "").strip()
    if configured:
        return configured.rstrip("/")
    raise RuntimeError("missing GitLab base URL; use issue URL or set GITLAB_BASE_URL")


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
    mr_url: str | None,
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
    if mr_url:
        lines.append(f"- MR：{mr_url}")

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


def glab_available() -> bool:
    return shutil.which("glab") is not None


def post_note_with_glab(project: str, iid: str, body: str) -> None:
    subprocess.check_call([
        "glab",
        "api",
        f"projects/{quote(project, safe='')}/issues/{iid}/notes",
        "--method",
        "POST",
        "--field",
        f"body={body}",
    ])


def close_with_glab(project: str, iid: str) -> dict[str, Any]:
    out = subprocess.check_output([
        "glab",
        "api",
        f"projects/{quote(project, safe='')}/issues/{iid}",
        "--method",
        "PUT",
        "--field",
        "state_event=close",
    ], text=True, stderr=subprocess.STDOUT)
    return json.loads(out)


def request_json(base: str, project: str, iid: str, *, method: str, payload: dict[str, Any]) -> dict[str, Any]:
    token = os.getenv("GITLAB_TOKEN") or os.getenv("GITLAB_PRIVATE_TOKEN")
    if not token:
        raise RuntimeError("missing GITLAB_TOKEN or GITLAB_PRIVATE_TOKEN")
    if method == "POST":
        url = f"{base}/api/v4/projects/{quote(project, safe='')}/issues/{iid}/notes"
    else:
        url = f"{base}/api/v4/projects/{quote(project, safe='')}/issues/{iid}"
    req = Request(url, data=json.dumps(payload).encode("utf-8"), method=method)
    req.add_header("PRIVATE-TOKEN", token)
    req.add_header("Content-Type", "application/json")
    req.add_header("Accept", "application/json")
    try:
        with urlopen(req, timeout=30) as resp:
            body = resp.read().decode("utf-8")
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"gitlab issue close request failed: {exc.code} {detail}") from exc
    return json.loads(body) if body else {}


def post_note_with_http(base: str, project: str, iid: str, body: str) -> None:
    request_json(base, project, iid, method="POST", payload={"body": body})


def close_with_http(base: str, project: str, iid: str) -> dict[str, Any]:
    return request_json(base, project, iid, method="PUT", payload={"state_event": "close"})


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Preview and optionally close one GitLab issue.")
    parser.add_argument("--issue", required=True)
    parser.add_argument("--branch")
    parser.add_argument("--commit")
    parser.add_argument("--mr-url")
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
        base, project, iid = parse_issue_ref(args.issue)
        issue_requirements = split_items(args.issue_requirement)
        reconciled = split_items(args.reconciled)
        completed = split_items(args.completed)
        verification = split_items(args.verification)
        remaining = split_items(args.remaining)
        if args.close and not issue_requirements:
            raise RuntimeError("refusing to close issue without original issue requirements; run issue-intake first and pass --issue-requirement")
        if args.close and not reconciled:
            raise RuntimeError("refusing to close issue without requirement reconciliation; pass --reconciled evidence mappings")
        note = build_close_note(
            branch=args.branch,
            commit=args.commit,
            mr_url=args.mr_url,
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
            "issue_iid": iid,
            "close_allowed": allowed,
            "note": note,
            "note_posted": False,
            "closed": False,
        }

        if args.post_note or args.close:
            if glab_available():
                try:
                    if args.post_note:
                        post_note_with_glab(project, iid, note)
                        result["note_posted"] = True
                    if args.close:
                        result["issue"] = close_with_glab(project, iid)
                        result["closed"] = True
                except Exception:
                    if not base:
                        raise
                    http_base = resolve_base_url(base)
                    if args.post_note:
                        post_note_with_http(http_base, project, iid, note)
                        result["note_posted"] = True
                    if args.close:
                        result["issue"] = close_with_http(http_base, project, iid)
                        result["closed"] = True
            else:
                http_base = resolve_base_url(base)
                if args.post_note:
                    post_note_with_http(http_base, project, iid, note)
                    result["note_posted"] = True
                if args.close:
                    result["issue"] = close_with_http(http_base, project, iid)
                    result["closed"] = True

        if args.json:
            print(json.dumps(result, ensure_ascii=False, indent=2))
        else:
            print(note, end="")
            print(f"\nClose allowed: {'yes' if allowed else 'no'}")
            if result["closed"]:
                issue = result.get("issue")
                if isinstance(issue, dict):
                    print(issue.get("web_url") or json.dumps(issue, ensure_ascii=False))
                else:
                    print("Issue closed")
        return 0
    except Exception as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
