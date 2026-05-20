#!/usr/bin/env python3
import json
import os
import subprocess
import sys
import time
from dataclasses import dataclass


BASE_URL = os.getenv("ATHENA_BASE_URL", "http://127.0.0.1:8080")
ITERATIONS = 5
RUN_ID = int(time.time())


def curl(args: list[str], timeout: int = 120) -> str:
    return subprocess.check_output(["curl", "-sS", *args], text=True, timeout=timeout)


def get_json(path: str) -> dict:
    return json.loads(curl([BASE_URL + path], timeout=30))


def post_json(path: str, payload: dict, timeout: int = 120) -> dict:
    return json.loads(
        curl(
            [
                "-X",
                "POST",
                BASE_URL + path,
                "-H",
                "Content-Type: application/json",
                "-d",
                json.dumps(payload, ensure_ascii=False),
            ],
            timeout=timeout,
        )
    )


def find_default_model_record_id() -> tuple[str, str]:
    providers = get_json("/api/models/providers")["items"]
    for provider in providers:
        if provider["name"] == "阿里云孙建测试勿动":
            for item in provider["models"]:
                if item["is_default"]:
                    return provider["id"], item["id"]
    raise RuntimeError("default provider/model not found")


@dataclass
class ScenarioResult:
    name: str
    passed: int
    total: int
    details: list[str]


def run_invalid_model() -> ScenarioResult:
    passed = 0
    details = []
    for idx in range(ITERATIONS):
        payload = {
            "query": "analyze the current risk posture",
            "model_id": "model-missing",
            "strict_schema_validation": True,
        }
        result = post_json("/api/chat/respond", payload, timeout=30)
        ok = (
            result["status"] == "invalid_model"
            and result["error"] == "requested model was not found"
            and result["error_detail"]["detail"]["model_id"] == "model-missing"
            and result["schema_validation"]["failure_stage"] == "initial_error"
        )
        if ok:
            passed += 1
        else:
            details.append(f"iteration {idx+1}: {json.dumps(result, ensure_ascii=False)}")
    return ScenarioResult("respond_invalid_model", passed, ITERATIONS, details)


def run_waiting(default_model_record_id: str) -> ScenarioResult:
    passed = 0
    details = []
    for idx in range(ITERATIONS):
        payload = {
            "query": "show user profile",
            "model_id": default_model_record_id,
            "enabled_skills": ["user_overview"],
            "disable_fast_path": True,
            "strict_schema_validation": True,
        }
        result = post_json("/api/chat/respond", payload, timeout=30)
        ok = (
            result["status"] == "waiting_for_information"
            and result["action_type"] == "information_request"
            and result["wait_state"]["resume_token"]
            and result["schema_validation"]["valid"] is False
        )
        if ok:
            passed += 1
        else:
            details.append(f"iteration {idx+1}: {json.dumps(result, ensure_ascii=False)}")
    return ScenarioResult("respond_waiting", passed, ITERATIONS, details)


def run_strict_tool_side_effect_guard(default_model_record_id: str) -> ScenarioResult:
    passed = 0
    details = []
    for idx in range(ITERATIONS):
        payload = {
            "query": "show user profile",
            "model_id": default_model_record_id,
            "enabled_skills": ["user_overview"],
            "disable_fast_path": True,
            "strict_schema_validation": True,
            "supplement": {
                "data": {"user_id": "u1001"},
                "outcome": "provided",
            },
            "supplement_outcome": "provided",
        }
        result = post_json("/api/chat/respond", payload, timeout=120)
        ok = (
            result["status"] == "schema_validation_failed"
            and result["schema_validation"]["valid"] is False
            and result["schema_validation"]["failure_stage"] == "tool_side_effect_guard"
            and result["detail"]["tool_side_effects"] is True
            and result["detail"]["retry_count"] == 2
        )
        if ok:
            passed += 1
        else:
            details.append(f"iteration {idx+1}: {json.dumps(result, ensure_ascii=False)}")
    return ScenarioResult("respond_strict_tool_side_effect_guard", passed, ITERATIONS, details)


def render(results: list[ScenarioResult], provider_id: str, model_record_id: str) -> str:
    lines = [
        "# Chat Respond Reliability Report",
        "",
        f"- base_url: `{BASE_URL}`",
        f"- provider_id: `{provider_id}`",
        f"- default_model_record_id: `{model_record_id}`",
        f"- iterations_per_scenario: `{ITERATIONS}`",
        "",
        "| Scenario | Passed | Total | Pass Rate |",
        "| --- | ---: | ---: | ---: |",
    ]
    for result in results:
        lines.append(f"| `{result.name}` | {result.passed} | {result.total} | {(result.passed / result.total) * 100:.0f}% |")
    lines.append("")
    lines.append("## Failures")
    lines.append("")
    for result in results:
        if not result.details:
            continue
        lines.append(f"### {result.name}")
        lines.extend(f"- {detail}" for detail in result.details)
        lines.append("")
    return "\n".join(lines)


def main() -> int:
    provider_id, model_record_id = find_default_model_record_id()
    results = [
        run_invalid_model(),
        run_waiting(model_record_id),
        run_strict_tool_side_effect_guard(model_record_id),
    ]
    report = render(results, provider_id, model_record_id)
    print(report)
    with open("docs/v0.1.0/checklists/chat-respond-可靠性报告.md", "w", encoding="utf-8") as handle:
        handle.write(report + "\n")
    failed = [r for r in results if (r.passed / r.total) < 0.9]
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
