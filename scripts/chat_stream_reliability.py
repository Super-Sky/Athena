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


def post_json(path: str, payload: dict) -> dict:
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
            timeout=60,
        )
    )


def post_delete(path: str) -> dict:
    return json.loads(curl(["-X", "DELETE", BASE_URL + path], timeout=30))


def post_sse(payload: dict) -> list[dict]:
    body = curl(
        [
            "-N",
            "-X",
            "POST",
            BASE_URL + "/api/chat/stream",
            "-H",
            "Content-Type: application/json",
            "-d",
            json.dumps(payload, ensure_ascii=False),
        ],
        timeout=120,
    )
    events = []
    for line in body.splitlines():
        if line.startswith("data:"):
            events.append(json.loads(line[5:]))
    return events


def find_default_model_record_id() -> tuple[str, str, str]:
    providers = get_json("/api/models/providers")["items"]
    for provider in providers:
        if provider["name"] == "阿里云孙建测试勿动":
            for model in provider["models"]:
                if model["is_default"]:
                    return provider["id"], model["id"], model["model_id"]
    raise RuntimeError("default Aliyun provider/model not found")


@dataclass
class ScenarioResult:
    name: str
    passed: int
    total: int
    details: list[str]


def summarize_events(events: list[dict]) -> str:
    return " -> ".join(f"{e.get('type')}({e.get('status', '')})" for e in events)


def events_of_type(events: list[dict], event_type: str) -> list[dict]:
    return [event for event in events if event.get("type") == event_type]


def first_event(events: list[dict], event_type: str) -> dict | None:
    matches = events_of_type(events, event_type)
    return matches[0] if matches else None


def last_event(events: list[dict], event_type: str) -> dict | None:
    matches = events_of_type(events, event_type)
    return matches[-1] if matches else None


def extract_resume_token(events: list[dict]) -> str:
    for event in events:
        wait_state = event.get("wait_state")
        if isinstance(wait_state, dict) and wait_state.get("resume_token"):
            return wait_state["resume_token"]
    raise RuntimeError(f"resume_token not found in events: {summarize_events(events)}")


def extract_session_id(events: list[dict]) -> str:
    for event in events:
        session_id = event.get("session_id")
        if isinstance(session_id, str) and session_id:
            return session_id
    raise RuntimeError(f"session_id not found in events: {summarize_events(events)}")


def run_invalid_model(default_model_record_id: str) -> ScenarioResult:
    print("running invalid_model_direct", file=sys.stderr)
    passed = 0
    details = []
    for idx in range(ITERATIONS):
        events = post_sse(
            {
                "disable_fast_path": True,
                "enabled_skills": [],
                "enabled_tools": [],
                "model_id": "model-missing",
                "prompt_template": "Return a structured invalid-model error if the requested model does not exist.",
                "query": "analyze the current risk posture",
                "timeout_after_seconds": 300,
            }
        )
        started = first_event(events, "request_started")
        error = first_event(events, "error")
        done = last_event(events, "done")
        progress_steps = events_of_type(events, "progress_step")
        ok = (
            started is not None
            and error is not None
            and error["status"] == "invalid_model"
            and error["error"] == "requested model was not found"
            and error["error_detail"]["detail"]["model_id"] == "model-missing"
            and done is not None
            and done["status"] == "invalid_model"
            and any(step.get("status") == "failed" for step in progress_steps)
        )
        if ok:
            passed += 1
        else:
            details.append(f"iteration {idx+1}: {summarize_events(events)}")
    return ScenarioResult("invalid_model_direct", passed, ITERATIONS, details)


def run_waiting_variants(default_model_record_id: str) -> list[ScenarioResult]:
    scenarios = [
        ("waiting_profile_missing_user", "show user profile"),
        ("waiting_risk_missing_user", "assess whether this user should enter manual review"),
        ("waiting_orders_missing_user", "show pending orders and risk flags for this account"),
        ("waiting_compliance_missing_user", "investigate whether this account has compliance risk"),
        ("waiting_fraud_missing_user", "check whether this account shows fraud indicators"),
        ("waiting_chargeback_missing_user", "summarize chargeback exposure for this account"),
        ("waiting_vip_risk_missing_user", "judge whether this VIP account is high risk"),
        ("waiting_account_takeover_missing_user", "assess whether this account takeover alert is credible"),
    ]
    results = []
    for name, query in scenarios:
        print(f"running {name}", file=sys.stderr)
        passed = 0
        details = []
        for idx in range(ITERATIONS):
            events = post_sse(
                {
                    "disable_fast_path": True,
                    "enabled_skills": ["user_overview"],
                    "enabled_tools": [],
                    "model_id": default_model_record_id,
                    "prompt_template": "If user_id is missing, ask for supplemental information instead of guessing.",
                    "query": query,
                    "timeout_after_seconds": 300,
                }
            )
            started = first_event(events, "request_started")
            action = first_event(events, "action")
            done = last_event(events, "done")
            progress_steps = events_of_type(events, "progress_step")
            ok = (
                started is not None
                and started["detail"]["fast_path"]["disabled"] is True
                and action is not None
                and action["status"] == "waiting_for_information"
                and action["action_type"] == "information_request"
                and action["action"]["expected_result"]["resume_token_required"] is True
                and action["wait_state"]["resume_token"]
                and action["structured_output"]["contract_id"] == "structured-output.v1"
                and done is not None
                and done["status"] == "waiting_for_information"
                and any(step.get("status") == "waiting" for step in progress_steps)
            )
            if ok:
                passed += 1
            else:
                details.append(f"iteration {idx+1}: {summarize_events(events)}")
        results.append(ScenarioResult(name, passed, ITERATIONS, details))
    return results


def open_waiting(default_model_record_id: str, query: str) -> list[dict]:
    return post_sse(
        {
            "disable_fast_path": True,
            "enabled_skills": ["user_overview"],
            "enabled_tools": [],
            "model_id": default_model_record_id,
            "prompt_template": "If user_id is missing, ask for supplemental information instead of guessing.",
            "query": query,
            "timeout_after_seconds": 300,
        }
    )


def run_resume_variants(default_model_record_id: str) -> list[ScenarioResult]:
    scenarios = [
        ("resume_profile_overview", "show user profile, orders, and risk flags"),
        ("resume_manual_review_decision", "assess whether the user should enter manual review"),
        ("resume_order_status", "show the user's open orders"),
        ("resume_fraud_summary", "check whether this account shows fraud indicators"),
        ("resume_chargeback_exposure", "summarize chargeback exposure for this account"),
        ("resume_vip_manual_review", "judge whether this VIP account should enter manual review"),
    ]
    results = []
    for name, query in scenarios:
        print(f"running {name}", file=sys.stderr)
        passed = 0
        details = []
        for idx in range(ITERATIONS):
            waiting_events = open_waiting(default_model_record_id, query)
            session_id = extract_session_id(waiting_events)
            resume_token = extract_resume_token(waiting_events)
            events = post_sse(
                {
                    "disable_fast_path": True,
                    "enabled_skills": ["user_overview"],
                    "enabled_tools": [],
                    "model_id": default_model_record_id,
                    "prompt_template": "Continue the same analysis after supplement data is provided.",
                    "session_id": session_id,
                    "supplement": {
                        "data": {"user_id": "u1001"},
                        "outcome": "provided",
                        "resume": {
                            "stage": "capability_resolution",
                            "resume_token": resume_token,
                        },
                    },
                    "supplement_outcome": "provided",
                    "timeout_after_seconds": 300,
                }
            )
            event_types = [event["type"] for event in events]
            progress_steps = events_of_type(events, "progress_step")
            done = last_event(events, "done")
            ok = (
                "error" not in event_types
                and "tool_calls" in event_types
                and "tool_result" in event_types
                and "stream_chunk" in event_types
                and done is not None
                and done["status"] == "completed"
                and done["structured_output"]["contract_id"] == "structured-output.v1"
                and any(step.get("status") == "completed" and step.get("progress_step", {}).get("step_id") == "turn_processing" for step in progress_steps)
            )
            if ok:
                passed += 1
            else:
                details.append(f"iteration {idx+1}: {summarize_events(events)}")
        results.append(ScenarioResult(name, passed, ITERATIONS, details))
    return results


def run_blocked_pending_wait(default_model_record_id: str) -> ScenarioResult:
    print("running blocked_pending_wait", file=sys.stderr)
    passed = 0
    details = []
    for idx in range(ITERATIONS):
        waiting_events = open_waiting(default_model_record_id, "show user profile")
        session_id = extract_session_id(waiting_events)
        _ = extract_resume_token(waiting_events)
        events = post_sse(
            {
                "disable_fast_path": True,
                "enabled_skills": ["user_overview"],
                "enabled_tools": [],
                "model_id": default_model_record_id,
                "prompt_template": "Do not resume unless a matching resume token is provided.",
                "query": "continue analysis without resume token",
                "session_id": session_id,
                "timeout_after_seconds": 300,
            }
        )
        started = first_event(events, "request_started")
        progress_steps = events_of_type(events, "progress_step")
        action = first_event(events, "action")
        done = last_event(events, "done")
        detail = (action or {}).get("detail") or {}
        ok = (
            started is not None
            and action is not None
            and action["status"] == "waiting_for_information"
            and detail["blocked_by_pending_wait"] is True
            and detail["resume_required"] is True
            and detail["queued_for_follow_up"] is True
            and done is not None
            and done["status"] == "waiting_for_information"
            and any(step.get("status") == "waiting" for step in progress_steps)
        )
        if ok:
            passed += 1
        else:
            details.append(f"iteration {idx+1}: {summarize_events(events)}")
    return ScenarioResult("blocked_pending_wait", passed, ITERATIONS, details)


def run_invalid_resume_token(default_model_record_id: str) -> ScenarioResult:
    print("running invalid_resume_token", file=sys.stderr)
    passed = 0
    details = []
    for idx in range(ITERATIONS):
        waiting_events = open_waiting(default_model_record_id, "show user profile")
        session_id = extract_session_id(waiting_events)
        events = post_sse(
            {
                "disable_fast_path": True,
                "enabled_skills": ["user_overview"],
                "enabled_tools": [],
                "model_id": default_model_record_id,
                "session_id": session_id,
                "supplement": {
                    "data": {"user_id": "u1001"},
                    "outcome": "provided",
                    "resume": {
                        "stage": "capability_resolution",
                        "resume_token": "resume-bad-token",
                    },
                },
                "supplement_outcome": "provided",
                "timeout_after_seconds": 300,
            }
        )
        started = first_event(events, "request_started")
        error = first_event(events, "error")
        done = last_event(events, "done")
        progress_steps = events_of_type(events, "progress_step")
        ok = (
            started is not None
            and error is not None
            and error["status"] == "invalid_resume_token"
            and done is not None
            and done["status"] == "invalid_resume_token"
            and any(step.get("status") == "failed" for step in progress_steps)
        )
        if ok:
            passed += 1
        else:
            details.append(f"iteration {idx+1}: {summarize_events(events)}")
    return ScenarioResult("invalid_resume_token", passed, ITERATIONS, details)


def run_queue_overflow(default_model_record_id: str) -> ScenarioResult:
    print("running queue_overflow", file=sys.stderr)
    passed = 0
    details = []
    for idx in range(ITERATIONS):
        waiting_events = open_waiting(default_model_record_id, "show user profile")
        session_id = extract_session_id(waiting_events)
        last_events = []
        for message_idx in range(35):
            last_events = post_sse(
                {
                    "disable_fast_path": True,
                    "enabled_skills": ["user_overview"],
                    "enabled_tools": [],
                    "model_id": default_model_record_id,
                    "prompt_template": "Do not resume unless a matching resume token is provided.",
                    "query": f"queued message {message_idx + 1}",
                    "session_id": session_id,
                    "timeout_after_seconds": 300,
                }
            )
        started = first_event(last_events, "request_started")
        progress_steps = events_of_type(last_events, "progress_step")
        action = first_event(last_events, "action")
        done = last_event(last_events, "done")
        detail = (action or {}).get("detail") or {}
        ok = (
            started is not None
            and action is not None
            and detail["blocked_by_pending_wait"] is True
            and detail["queued_for_follow_up"] is True
            and detail["queue_overflow"]["dropped_oldest"] is True
            and done is not None
            and done["status"] == "waiting_for_information"
            and any(step.get("status") == "waiting" for step in progress_steps)
        )
        if ok:
            passed += 1
        else:
            details.append(f"iteration {idx+1}: {summarize_events(last_events)}")
    return ScenarioResult("queue_overflow", passed, ITERATIONS, details)


def render_report(results: list[ScenarioResult], provider_id: str, model_record_id: str, model_id: str) -> str:
    lines = [
        "# Chat Stream Reliability Report",
        "",
        f"- base_url: `{BASE_URL}`",
        f"- provider_id: `{provider_id}`",
        f"- default_model_record_id: `{model_record_id}`",
        f"- default_provider_model_id: `{model_id}`",
        f"- iterations_per_scenario: `{ITERATIONS}`",
        "",
        "| Scenario | Passed | Total | Pass Rate |",
        "| --- | ---: | ---: | ---: |",
    ]
    for result in results:
        rate = (result.passed / result.total) * 100
        lines.append(f"| `{result.name}` | {result.passed} | {result.total} | {rate:.0f}% |")
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
    provider_id, model_record_id, model_id = find_default_model_record_id()
    results = []
    results.append(run_invalid_model(model_record_id))
    results.extend(run_waiting_variants(model_record_id))
    results.extend(run_resume_variants(model_record_id))
    results.append(run_blocked_pending_wait(model_record_id))
    results.append(run_invalid_resume_token(model_record_id))
    results.append(run_queue_overflow(model_record_id))

    report = render_report(results, provider_id, model_record_id, model_id)
    print(report)
    with open("docs/v0.1.0/checklists/chat-stream-可靠性报告.md", "w", encoding="utf-8") as handle:
        handle.write(report + "\n")

    failed = [result for result in results if (result.passed / result.total) < 0.9]
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
