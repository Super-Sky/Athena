#!/usr/bin/env python3
"""Read one GitLab issue through glab or the GitLab REST API."""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass
from typing import Any


ISSUE_URL_RE = re.compile(
    r"^(?P<base>https?://[^/]+)/(?P<project>.+)/-/issues/(?P<iid>\d+)$"
)
PROJECT_REF_RE = re.compile(r"^(?P<project>[^#]+)#(?P<iid>\d+)$")


@dataclass
class AuthHeaders:
    headers: dict[str, str]
    source: str


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


def resolve_base_url(explicit_base: str | None) -> str:
    if explicit_base:
        return explicit_base
    configured = os.getenv("GITLAB_BASE_URL", "").strip()
    if configured:
        return configured.rstrip("/")
    raise RuntimeError(
        "missing GitLab base URL; provide one issue URL or set GITLAB_BASE_URL for project#iid refs"
    )


def resolve_auth_headers() -> AuthHeaders:
    token = os.getenv("GITLAB_TOKEN", "").strip()
    if token:
        return AuthHeaders(headers={"PRIVATE-TOKEN": token}, source="gitlab-token")

    private_token = os.getenv("GITLAB_PRIVATE_TOKEN", "").strip()
    if private_token:
        return AuthHeaders(
            headers={"PRIVATE-TOKEN": private_token},
            source="private-token",
        )

    bearer_token = os.getenv("GITLAB_BEARER_TOKEN", "").strip()
    if bearer_token:
        value = (
            bearer_token
            if bearer_token.lower().startswith("bearer ")
            else f"Bearer {bearer_token}"
        )
        return AuthHeaders(headers={"Authorization": value}, source="bearer-token")

    session_cookie = os.getenv("GITLAB_SESSION_COOKIE", "").strip()
    if session_cookie:
        return AuthHeaders(headers={"Cookie": session_cookie}, source="session-cookie")

    return AuthHeaders(headers={}, source="anonymous")


def glab_available() -> bool:
    return shutil.which("glab") is not None


def run_glab(project: str, iid: str, *, include_notes: bool) -> tuple[dict[str, Any], list[dict[str, Any]] | None]:
    issue_cmd = ["glab", "api", f"projects/{urllib.parse.quote(project, safe='')}/issues/{iid}"]
    issue_out = subprocess.check_output(issue_cmd, text=True, stderr=subprocess.STDOUT)
    issue = json.loads(issue_out)
    notes: list[dict[str, Any]] | None = None
    if include_notes:
        notes_cmd = [
            "glab",
            "api",
            f"projects/{urllib.parse.quote(project, safe='')}/issues/{iid}/notes",
        ]
        notes_out = subprocess.check_output(notes_cmd, text=True, stderr=subprocess.STDOUT)
        raw_notes = json.loads(notes_out)
        if not isinstance(raw_notes, list):
            raise RuntimeError("glab notes response is not one list")
        notes = notes_to_summary(raw_notes)
    return issue, notes


def request_json(url: str, headers: dict[str, str], *, auth_source: str) -> Any:
    request = urllib.request.Request(url=url, method="GET")
    request.add_header("Accept", "application/json")
    for key, value in headers.items():
        request.add_header(key, value)
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            body = response.read().decode("utf-8")
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        if exc.code == 404 and auth_source == "anonymous":
            raise RuntimeError(
                "gitlab request failed: 404 Project Not Found; the issue may be private, set "
                "GITLAB_TOKEN, GITLAB_PRIVATE_TOKEN, GITLAB_BEARER_TOKEN, or GITLAB_SESSION_COOKIE"
            ) from exc
        raise RuntimeError(f"gitlab request failed: {exc.code} {body}") from exc
    except urllib.error.URLError as exc:
        raise RuntimeError(f"gitlab request failed: {exc.reason}") from exc

    try:
        return json.loads(body)
    except json.JSONDecodeError as exc:
        raise RuntimeError(
            "gitlab did not return JSON; auth may be missing or a login page was returned, set "
            "GITLAB_TOKEN, GITLAB_PRIVATE_TOKEN, GITLAB_BEARER_TOKEN, or GITLAB_SESSION_COOKIE"
        ) from exc


def build_issue_api_url(base_url: str, project_path: str, issue_iid: str) -> str:
    encoded_project = urllib.parse.quote(project_path, safe="")
    return f"{base_url}/api/v4/projects/{encoded_project}/issues/{issue_iid}"


def build_notes_api_url(base_url: str, project_path: str, issue_iid: str) -> str:
    encoded_project = urllib.parse.quote(project_path, safe="")
    return f"{base_url}/api/v4/projects/{encoded_project}/issues/{issue_iid}/notes"


def notes_to_summary(notes: list[dict[str, Any]]) -> list[dict[str, Any]]:
    return [
        {
            "author": ((note.get("author") or {}).get("username") or ""),
            "created_at": note.get("created_at", ""),
            "body": note.get("body", ""),
            "system": bool(note.get("system")),
        }
        for note in notes
    ]


def normalize_issue(
    issue: dict[str, Any],
    *,
    source_ref: str,
    project_path: str,
    auth_source: str,
    notes: list[dict[str, Any]] | None,
) -> dict[str, Any]:
    description = issue.get("description") or ""
    lines = [line.rstrip() for line in description.splitlines()]
    non_empty = [line for line in lines if line.strip()]
    return {
        "source_ref": source_ref,
        "auth_source": auth_source,
        "project_path": project_path,
        "issue_iid": issue.get("iid"),
        "title": issue.get("title", ""),
        "description": description,
        "description_preview": "\n".join(non_empty[:20]),
        "state": issue.get("state", ""),
        "task_status": issue.get("task_status"),
        "labels": issue.get("labels", []),
        "web_url": issue.get("web_url", ""),
        "author": ((issue.get("author") or {}).get("username") or ""),
        "assignees": [
            assignee.get("username", "")
            for assignee in issue.get("assignees") or []
            if assignee.get("username")
        ],
        "milestone": ((issue.get("milestone") or {}).get("title") or ""),
        "references": issue.get("references", {}),
        "notes": notes or [],
    }


def render_markdown(payload: dict[str, Any]) -> str:
    lines = [
        f"# {payload.get('title', '')}",
        "",
        f"- Source ref: {payload.get('source_ref', '')}",
        f"- Auth source: {payload.get('auth_source', '')}",
        f"- Project: {payload.get('project_path', '')}",
        f"- Issue IID: {payload.get('issue_iid', '')}",
        f"- State: {payload.get('state', '')}",
    ]
    task_status = payload.get("task_status")
    if task_status:
        lines.append(f"- Tasks: {task_status}")
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
    notes = payload.get("notes") or []
    if notes:
        lines.extend(["", "## Notes", ""])
        for note in notes:
            author = note.get("author", "")
            created_at = note.get("created_at", "")
            body = note.get("body", "").strip()
            lines.append(f"- [{created_at}] @{author}: {body}")
    return "\n".join(lines) + "\n"


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Fetch one GitLab issue through glab or the GitLab API."
    )
    parser.add_argument(
        "issue",
        help="GitLab issue URL or project reference like group/project#123",
    )
    parser.add_argument("--include-notes", action="store_true", help="Fetch issue notes as well")
    parser.add_argument(
        "--format",
        choices=["json", "markdown"],
        default="json",
        help="Output format",
    )
    return parser


def fetch_issue(issue_ref: str, *, include_notes: bool) -> dict[str, Any]:
    explicit_base, project_path, issue_iid = parse_issue_ref(issue_ref)
    if glab_available():
        try:
            issue, notes = run_glab(project_path, issue_iid, include_notes=include_notes)
            return normalize_issue(
                issue,
                source_ref=issue_ref,
                project_path=project_path,
                auth_source="glab",
                notes=notes,
            )
        except Exception:
            pass

    base_url = resolve_base_url(explicit_base)
    auth = resolve_auth_headers()
    issue = request_json(
        build_issue_api_url(base_url, project_path, issue_iid),
        auth.headers,
        auth_source=auth.source,
    )
    notes: list[dict[str, Any]] | None = None
    if include_notes:
        raw_notes = request_json(
            build_notes_api_url(base_url, project_path, issue_iid),
            auth.headers,
            auth_source=auth.source,
        )
        if not isinstance(raw_notes, list):
            raise RuntimeError("gitlab notes API did not return one list")
        notes = notes_to_summary(raw_notes)
    return normalize_issue(
        issue,
        source_ref=issue_ref,
        project_path=project_path,
        auth_source=auth.source,
        notes=notes,
    )


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        payload = fetch_issue(args.issue, include_notes=args.include_notes)
        if args.format == "markdown":
            sys.stdout.write(render_markdown(payload))
        else:
            json.dump(payload, sys.stdout, ensure_ascii=False, indent=2)
            sys.stdout.write("\n")
        return 0
    except Exception as exc:
        print(str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
