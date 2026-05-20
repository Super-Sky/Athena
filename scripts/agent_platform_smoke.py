#!/usr/bin/env python3
import json
import os
import subprocess
import sys


BASE_URL = os.getenv("ATHENA_BASE_URL", "http://127.0.0.1:8080")


def curl(args: list[str], timeout: int = 60) -> str:
    return subprocess.check_output(["curl", "-sS", *args], text=True, timeout=timeout)


def get_json(path: str) -> dict:
    return json.loads(curl([BASE_URL + path], timeout=15))


def post_json(path: str, payload: dict, timeout: int = 90) -> dict:
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


def post_stream(path: str, payload: dict, timeout: int = 90) -> str:
    return curl(
        [
            "-N",
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


def expect(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def run() -> None:
    health = get_json("/healthz")
    expect(health.get("status") == "ok", f"healthz failed: {health}")

    default_chat = post_json("/api/chat/respond", {"query": "总结当前风险重点"})
    expect(default_chat.get("status") == "completed", f"default chat status: {default_chat}")
    expect(default_chat.get("session_id"), f"default chat missing session_id: {default_chat}")
    result = default_chat.get("result") or {}
    expect(result.get("main_answer"), f"default chat missing main_answer: {default_chat}")
    expect(result.get("answer"), f"default chat missing answer: {default_chat}")
    expect(result.get("structured_result") is not None, f"default chat missing structured_result: {default_chat}")
    expect(result.get("follow_up_suggestions"), f"default chat missing follow_up_suggestions: {default_chat}")

    waiting = post_json(
        "/api/chat/respond",
        {
            "query": "查询用户画像",
            "enabled_skills": ["user_overview"],
        },
    )
    expect(waiting.get("status") == "waiting_for_information", f"waiting status: {waiting}")
    expect(waiting.get("action_type") == "information_request", f"waiting action_type: {waiting}")
    wait_state = waiting.get("wait_state") or {}
    expect(wait_state.get("resume_token"), f"waiting missing resume_token: {waiting}")

    invalid = post_json(
        "/api/chat/respond",
        {
            "query": "继续之前的对话",
            "session_id": "sess-missing",
        },
    )
    expect(invalid.get("status") == "invalid_session", f"invalid_session status: {invalid}")
    error_detail = invalid.get("error_detail") or {}
    expect(error_detail.get("code") == "invalid_session", f"invalid_session code: {invalid}")
    expect(error_detail.get("reason") == "not_found", f"invalid_session reason: {invalid}")
    expect(error_detail.get("client_action") == "start_new_session", f"invalid_session client_action: {invalid}")

    stream_output = post_stream(
        "/api/chat/stream",
        {
            "query": "生成工作流计划",
            "task_type": "workflow_step_request",
            "workspace_id": "ws-1",
            "main_session_id": "sess-1",
            "workflow_run_id": "run-1",
            "step_id": "step-1",
            "input_payload": {"phase": "triage"},
        },
    )
    for want in ["progress_step", "workflow_plan", "card_created", "right_panel_view", "next_questions", "completed", "done"]:
        expect(want in stream_output, f"stream missing {want}: {stream_output}")

    print(json.dumps({"status": "ok", "base_url": BASE_URL}, ensure_ascii=False))


if __name__ == "__main__":
    try:
        run()
    except Exception as exc:
        print(str(exc), file=sys.stderr)
        sys.exit(1)
