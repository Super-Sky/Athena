#!/usr/bin/env python3
"""Read one GitHub issue through gh or the GitHub REST API."""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import urllib.error
import urllib.request
from dataclasses import dataclass
from typing import Any
from urllib.parse import urlparse


ISSUE_URL_RE = re.compile(
    r"^(?P<base>https?://[^/]+)/(?P<project>[^/]+/[^/]+)/issues/(?P<number>\d+)$"
)
PROJECT_REF_RE = re.compile(r"^(?P<project>[^#]+)#(?P<number>\d+)$")


@dataclass
class AuthHeaders:
    headers: dict[str, str]
    source: str


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


def resolve_auth_headers() -> AuthHeaders:
    for key in ("GITHUB_TOKEN", "GH_TOKEN", "GITHUB_PAT"):
        token = os.getenv(key, "").strip()
        if token:
            return AuthHeaders(headers={"Authorization": f"Bearer {token}"}, source=key.lower())
    bearer_token = os.getenv("GITHUB_BEARER_TOKEN", "").strip()
    if bearer_token:
        value = bearer_token if bearer_token.lower().startswith("bearer ") else f"Bearer {bearer_token}"
        return AuthHeaders(headers={"Authorization": value}, source="github-bearer-token")
    return AuthHeaders(headers={}, source="anonymous")


def gh_available() -> bool:
    return shutil.which("gh") is not None


def run_gh(
    project: str,
    number: str,
    *,
    include_comments: bool,
) -> tuple[dict[str, Any], list[dict[str, Any]] | None]:
    issue_cmd = ["gh", "api", f"repos/{project}/issues/{number}"]
    issue_out = subprocess.check_output(issue_cmd, text=True, stderr=subprocess.STDOUT)
    issue = json.loads(issue_out)
    comments: list[dict[str, Any]] | None = None
    if include_comments:
        comments_cmd = ["gh", "api", f"repos/{project}/issues/{number}/comments"]
        comments_out = subprocess.check_output(comments_cmd, text=True, stderr=subprocess.STDOUT)
        raw_comments = json.loads(comments_out)
        if not isinstance(raw_comments, list):
            raise RuntimeError("gh comments response is not one list")
        comments = comments_to_summary(raw_comments)
    return issue, comments


def request_json(url: str, headers: dict[str, str], *, auth_source: str) -> Any:
    request = urllib.request.Request(url=url, method="GET")
    request.add_header("Accept", "application/vnd.github+json")
    request.add_header("X-GitHub-Api-Version", "2022-11-28")
    for key, value in headers.items():
        request.add_header(key, value)
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            body = response.read().decode("utf-8")
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        if exc.code == 404 and auth_source == "anonymous":
            raise RuntimeError(
                "github request failed: 404 Not Found; the issue may be private, set "
                "GITHUB_TOKEN, GH_TOKEN, GITHUB_PAT, or GITHUB_BEARER_TOKEN"
            ) from exc
        raise RuntimeError(f"github request failed: {exc.code} {body}") from exc
    except urllib.error.URLError as exc:
        raise RuntimeError(f"github request failed: {exc.reason}") from exc

    try:
        return json.loads(body)
    except json.JSONDecodeError as exc:
        raise RuntimeError(
            "github did not return JSON; auth may be missing or a login page was returned, set "
            "GITHUB_TOKEN, GH_TOKEN, GITHUB_PAT, or GITHUB_BEARER_TOKEN"
        ) from exc


def build_issue_api_url(api_base: str, project_path: str, issue_number: str) -> str:
    return f"{api_base}/repos/{project_path}/issues/{issue_number}"


def build_comments_api_url(api_base: str, project_path: str, issue_number: str) -> str:
    return f"{api_base}/repos/{project_path}/issues/{issue_number}/comments"


def comments_to_summary(comments: list[dict[str, Any]]) -> list[dict[str, Any]]:
    return [
        {
            "author": ((comment.get("user") or {}).get("login") or ""),
            "created_at": comment.get("created_at", ""),
            "body": comment.get("body", ""),
            "system": False,
        }
        for comment in comments
    ]


def normalize_issue(
    issue: dict[str, Any],
    *,
    source_ref: str,
    project_path: str,
    auth_source: str,
    comments: list[dict[str, Any]] | None,
) -> dict[str, Any]:
    description = issue.get("body") or ""
    lines = [line.rstrip() for line in description.splitlines()]
    non_empty = [line for line in lines if line.strip()]
    labels = [label.get("name", "") for label in issue.get("labels") or [] if isinstance(label, dict)]
    number = issue.get("number")
    full_ref = f"{project_path}#{number}" if number is not None else source_ref
    return {
        "source_ref": source_ref,
        "auth_source": auth_source,
        "project_path": project_path,
        "issue_number": number,
        "title": issue.get("title", ""),
        "description": description,
        "description_preview": "\n".join(non_empty[:20]),
        "state": issue.get("state", ""),
        "labels": labels,
        "web_url": issue.get("html_url", ""),
        "author": ((issue.get("user") or {}).get("login") or ""),
        "assignees": [
            assignee.get("login", "")
            for assignee in issue.get("assignees") or []
            if assignee.get("login")
        ],
        "milestone": ((issue.get("milestone") or {}).get("title") or ""),
        "references": {"full": full_ref, "short": f"#{number}" if number is not None else ""},
        "comments": comments or [],
    }


def render_markdown(payload: dict[str, Any]) -> str:
    lines = [
        f"# {payload.get('title', '')}",
        "",
        f"- Source ref: {payload.get('source_ref', '')}",
        f"- Auth source: {payload.get('auth_source', '')}",
        f"- Repository: {payload.get('project_path', '')}",
        f"- Issue Number: {payload.get('issue_number', '')}",
        f"- State: {payload.get('state', '')}",
    ]
    labels = payload.get("labels") or []
    if labels:
        lines.append(f"- Labels: {', '.join(labels)}")
    assignees = payload.get("assignees") or []
    if assignees:
        lines.append(f"- Assignees: {', '.join(assignees)}")
    lines.extend(
        [
            f"- Web URL: {payload.get('web_url', '')}",
            "",
            "## Description",
            "",
            payload.get("description", "").strip() or "(empty)",
        ]
    )
    comments = payload.get("comments") or []
    if comments:
        lines.extend(["", "## Comments", ""])
        for comment in comments:
            author = comment.get("author", "")
            created_at = comment.get("created_at", "")
            body = comment.get("body", "").strip()
            lines.append(f"- [{created_at}] @{author}: {body}")
    return "\n".join(lines) + "\n"


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Fetch one GitHub issue through gh or the GitHub API."
    )
    parser.add_argument(
        "issue",
        help="GitHub issue URL or repository reference like owner/repo#123",
    )
    parser.add_argument("--include-comments", action="store_true", help="Fetch issue comments as well")
    parser.add_argument("--include-notes", action="store_true", help="Compatibility alias for --include-comments")
    parser.add_argument(
        "--format",
        choices=["json", "markdown"],
        default="json",
        help="Output format",
    )
    return parser


def fetch_issue(issue_ref: str, *, include_comments: bool) -> dict[str, Any]:
    explicit_base, project_path, issue_number = parse_issue_ref(issue_ref)
    if gh_available():
        try:
            issue, comments = run_gh(project_path, issue_number, include_comments=include_comments)
            return normalize_issue(
                issue,
                source_ref=issue_ref,
                project_path=project_path,
                auth_source="gh",
                comments=comments,
            )
        except Exception:
            pass

    base_url = resolve_base_url(explicit_base)
    api_base = resolve_api_base(base_url)
    auth = resolve_auth_headers()
    issue = request_json(build_issue_api_url(api_base, project_path, issue_number), auth.headers, auth_source=auth.source)
    comments: list[dict[str, Any]] | None = None
    if include_comments:
        raw_comments = request_json(
            build_comments_api_url(api_base, project_path, issue_number),
            auth.headers,
            auth_source=auth.source,
        )
        if not isinstance(raw_comments, list):
            raise RuntimeError("github comments API did not return one list")
        comments = comments_to_summary(raw_comments)
    return normalize_issue(
        issue,
        source_ref=issue_ref,
        project_path=project_path,
        auth_source=auth.source,
        comments=comments,
    )


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        include_comments = args.include_comments or args.include_notes
        payload = fetch_issue(args.issue, include_comments=include_comments)
        if args.format == "json":
            print(json.dumps(payload, ensure_ascii=False, indent=2))
        else:
            print(render_markdown(payload), end="")
        return 0
    except Exception as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
