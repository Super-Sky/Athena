#!/usr/bin/env python3
"""Compose and optionally create one GitLab issue using the repository issue template."""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import urllib.error
from dataclasses import dataclass
from typing import Any
from urllib.parse import quote, urlparse
from urllib.request import Request, urlopen


PROJECT_URL_RE = re.compile(r"^(?P<base>https?://[^/]+)/(?P<project>.+)$")
VALID_ISSUE_TYPES = {
    "需要新增能力",
    "需要增强现有能力",
    "当前功能有问题",
    "本地运行或环境有问题",
    "文档或方向需要调整",
}
VALID_URGENCY = {"高", "中", "低"}


@dataclass
class ProjectRef:
    base_url: str | None
    project_path: str


def parse_project_ref(*, project: str | None, project_url: str | None) -> ProjectRef:
    if project_url:
        raw = project_url.strip().rstrip("/")
        match = PROJECT_URL_RE.match(raw)
        if not match:
            raise ValueError(f"unsupported GitLab project URL: {project_url}")
        return ProjectRef(base_url=match.group("base"), project_path=match.group("project"))
    if project:
        raw = project.strip().strip("/")
        if not raw or raw.startswith("http://") or raw.startswith("https://") or "/" not in raw:
            raise ValueError(f"unsupported GitLab project path: {project}")
        return ProjectRef(base_url=None, project_path=raw)
    raise ValueError("--project or --project-url is required")


def resolve_base_url(explicit_base: str | None) -> str:
    if explicit_base:
        return explicit_base.rstrip("/")
    configured = os.getenv("GITLAB_BASE_URL", "").strip()
    if configured:
        return configured.rstrip("/")
    raise RuntimeError("missing GitLab base URL; provide --project-url or set GITLAB_BASE_URL")


def glab_available() -> bool:
    return shutil.which("glab") is not None


def split_items(values: list[str]) -> list[str]:
    items: list[str] = []
    for raw in values:
        for piece in raw.split(";"):
            piece = piece.strip()
            if piece:
                items.append(piece)
    return items


def checked(label: str, selected: str) -> str:
    return "x" if label == selected else " "


def render_issue_body(
    *,
    issue_type: str,
    background: str,
    problem: str,
    request: str,
    current_result: str,
    expected_result: str,
    urgency: str,
    supplemental: list[str],
    handling_mode: str | None = None,
    target_version: str | None = None,
    delivery_unit: str | None = None,
    canonical_issue: str | None = None,
    primary_repository: str | None = None,
    external_references: list[str] | None = None,
    owner: str | None = None,
    collaborators: list[str] | None = None,
    current_status: str | None = None,
    next_action: str | None = None,
) -> str:
    if issue_type not in VALID_ISSUE_TYPES:
        raise ValueError(f"issue type must be one of: {', '.join(sorted(VALID_ISSUE_TYPES))}")
    if urgency not in VALID_URGENCY:
        raise ValueError(f"urgency must be one of: {', '.join(sorted(VALID_URGENCY))}")

    references = external_references or []
    lines = [
        "## 提单信息",
        "",
        "- 问题类型：",
        f"  - [{checked('需要新增能力', issue_type)}] 需要新增能力",
        f"  - [{checked('需要增强现有能力', issue_type)}] 需要增强现有能力",
        f"  - [{checked('当前功能有问题', issue_type)}] 当前功能有问题",
        f"  - [{checked('本地运行或环境有问题', issue_type)}] 本地运行或环境有问题",
        f"  - [{checked('文档或方向需要调整', issue_type)}] 文档或方向需要调整",
        "- 标题：",
        f"- 使用场景或业务背景：{background or '待补充'}",
        f"- 当前问题：{problem or '待补充'}",
        f"- 希望系统支持或改进什么：{request or '待补充'}",
        f"- 当前结果：{current_result or '待补充'}",
        f"- 预期结果：{expected_result or '待补充'}",
        "- 紧急程度：",
        f"  - [{checked('高', urgency)}] 高",
        f"  - [{checked('中', urgency)}] 中",
        f"  - [{checked('低', urgency)}] 低",
        "- 补充材料（截图、链接、示例）：",
    ]
    if supplemental:
        lines.extend([f"  - {item}" for item in supplemental])
    else:
        lines.append("  - 无")

    lines.extend(
        [
            "",
            "## 维护者分诊",
            "",
            "- 处理方式：",
            f"  - [{checked('进入版本规划', handling_mode or '')}] 进入版本规划",
            f"  - [{checked('进入功能实现', handling_mode or '')}] 进入功能实现",
            f"  - [{checked('进入缺陷修复', handling_mode or '')}] 进入缺陷修复",
            f"  - [{checked('进入本地运行处理', handling_mode or '')}] 进入本地运行处理",
            f"  - [{checked('进入文档或治理调整', handling_mode or '')}] 进入文档或治理调整",
            f"- 所属版本：{target_version or ''}",
            f"- 所属交付单元：{delivery_unit or ''}",
            f"- canonical issue：{canonical_issue or ''}",
            "- 主责任仓库：",
            f"  - [{checked('athena', primary_repository or '')}] athena",
            "- 外部参考（可选，非流程前置）：",
            *(f"  - {item}" for item in references),
            f"- owner：{owner or ''}",
            f"- 协作研发：{'; '.join(collaborators or [])}",
            f"- 当前状态：{current_status or ''}",
            f"- 下一个动作：{next_action or ''}",
        ]
    )
    return "\n".join(lines).strip() + "\n"


def create_issue_with_glab(project: str, title: str, body: str, labels: list[str], assignee: str | None, milestone: str | None) -> dict[str, Any]:
    cmd = ["glab", "api", f"projects/{quote(project, safe='')}/issues", "--method", "POST", "--field", f"title={title}", "--field", f"description={body}"]
    if labels:
        cmd.extend(["--field", f"labels={','.join(labels)}"])
    if assignee:
        cmd.extend(["--field", f"assignee_username={assignee}"])
    if milestone:
        cmd.extend(["--field", f"milestone={milestone}"])
    out = subprocess.check_output(cmd, text=True, stderr=subprocess.STDOUT)
    return json.loads(out)


def create_issue_with_http(base_url: str, project: str, title: str, body: str, labels: list[str], assignee: str | None, milestone: str | None) -> dict[str, Any]:
    token = os.getenv("GITLAB_TOKEN") or os.getenv("GITLAB_PRIVATE_TOKEN")
    if not token:
        raise RuntimeError("missing GITLAB_TOKEN or GITLAB_PRIVATE_TOKEN")
    payload: dict[str, Any] = {"title": title, "description": body}
    if labels:
        payload["labels"] = ",".join(labels)
    if assignee:
        payload["assignee_username"] = assignee
    if milestone:
        payload["milestone"] = milestone
    url = f"{base_url}/api/v4/projects/{quote(project, safe='')}/issues"
    req = Request(url, data=json.dumps(payload).encode("utf-8"), method="POST")
    req.add_header("PRIVATE-TOKEN", token)
    req.add_header("Content-Type", "application/json")
    req.add_header("Accept", "application/json")
    try:
        with urlopen(req, timeout=30) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"gitlab issue create failed: {exc.code} {detail}") from exc


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Compose and optionally create one GitLab issue.")
    target = parser.add_mutually_exclusive_group(required=True)
    target.add_argument("--project", help="GitLab project path like group/project. Requires GITLAB_BASE_URL when posting without glab.")
    target.add_argument("--project-url", help="Full GitLab project URL like https://git.example.com/group/project")
    parser.add_argument("--title", required=True)
    parser.add_argument("--issue-type", required=True, choices=sorted(VALID_ISSUE_TYPES))
    parser.add_argument("--background", default="")
    parser.add_argument("--problem", default="")
    parser.add_argument("--request", default="")
    parser.add_argument("--current-result", default="")
    parser.add_argument("--expected-result", default="")
    parser.add_argument("--urgency", required=True, choices=sorted(VALID_URGENCY))
    parser.add_argument("--supplemental", action="append", default=[])
    parser.add_argument("--handling-mode")
    parser.add_argument("--target-version")
    parser.add_argument("--delivery-unit")
    parser.add_argument("--canonical-issue", help="Canonical Athena issue reference like example-org/athena#3.")
    parser.add_argument("--primary-repository", choices=["athena"], default="athena")
    parser.add_argument("--external-reference", action="append", default=[], help="Optional non-blocking external reference. Can be repeated or separated by semicolons.")
    parser.add_argument("--owner")
    parser.add_argument("--collaborator", action="append", default=[])
    parser.add_argument("--current-status")
    parser.add_argument("--next-action")
    parser.add_argument("--labels", action="append", default=[], help="Comma or semicolon separated labels. Can be repeated.")
    parser.add_argument("--assignee")
    parser.add_argument("--milestone")
    parser.add_argument("--json", action="store_true", help="Print preview or create result as JSON.")
    parser.add_argument("--post", action="store_true")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        target = parse_project_ref(project=args.project, project_url=args.project_url)
        labels = split_items(args.labels)
        body = render_issue_body(
            issue_type=args.issue_type,
            background=args.background.strip(),
            problem=args.problem.strip(),
            request=args.request.strip(),
            current_result=args.current_result.strip(),
            expected_result=args.expected_result.strip(),
            urgency=args.urgency,
            supplemental=split_items(args.supplemental),
            handling_mode=args.handling_mode,
            target_version=args.target_version,
            delivery_unit=args.delivery_unit,
            canonical_issue=args.canonical_issue,
            primary_repository=args.primary_repository,
            external_references=split_items(args.external_reference),
            owner=args.owner,
            collaborators=args.collaborator,
            current_status=args.current_status,
            next_action=args.next_action,
        )
        if not args.post:
            preview = {
                "project_path": target.project_path,
                "base_url": target.base_url,
                "title": args.title,
                "description": body,
                "labels": labels,
                "assignee": args.assignee,
                "milestone": args.milestone,
            }
            if args.json:
                print(json.dumps(preview, ensure_ascii=False, indent=2))
            else:
                print(f"Project: {target.project_path}")
                if target.base_url:
                    print(f"Base URL: {target.base_url}")
                print(f"Title: {args.title}")
                if labels:
                    print(f"Labels: {', '.join(labels)}")
                if args.assignee:
                    print(f"Assignee: {args.assignee}")
                if args.milestone:
                    print(f"Milestone: {args.milestone}")
                print("\n" + body, end="")
            return 0

        if glab_available():
            try:
                created = create_issue_with_glab(target.project_path, args.title, body, labels, args.assignee, args.milestone)
            except Exception:
                if not target.base_url:
                    raise
                created = create_issue_with_http(resolve_base_url(target.base_url), target.project_path, args.title, body, labels, args.assignee, args.milestone)
        else:
            created = create_issue_with_http(resolve_base_url(target.base_url), target.project_path, args.title, body, labels, args.assignee, args.milestone)
        if args.json:
            print(json.dumps(created, ensure_ascii=False, indent=2))
        else:
            print(created.get("web_url") or json.dumps(created, ensure_ascii=False))
        return 0
    except Exception as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
