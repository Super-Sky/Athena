#!/usr/bin/env python3
"""Smoke-test the Control Plane RuntimeContract foundation and optional Web DOM."""

from __future__ import annotations

import argparse
import http.cookiejar
import json
import os
import sys
import urllib.error
import urllib.request
from typing import Any


DEFAULT_BASE_URL = os.getenv("ATHENA_BASE_URL", "http://127.0.0.1:8090")


class SmokeFailure(AssertionError):
    """Raised when the smoke test observes a broken contract."""


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Validate Athena runtime foundation readout surfaces.")
    parser.add_argument("--base-url", default=DEFAULT_BASE_URL, help="Athena backend base URL.")
    parser.add_argument("--web-url", default=os.getenv("ATHENA_WEB_URL", ""), help="Optional Web console URL for DOM checks.")
    parser.add_argument("--control-plane-token", default=os.getenv("CONTROL_PLANE_TOKEN", ""), help="Optional control-plane login token.")
    parser.add_argument("--timeout-seconds", type=int, default=30, help="HTTP timeout per request.")
    return parser.parse_args()


class RuntimeFoundationClient:
    def __init__(self, base_url: str, timeout: int) -> None:
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))

    def request_json(self, method: str, path: str, payload: dict[str, Any] | None = None) -> Any:
        data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
        request = urllib.request.Request(
            self.base_url + path,
            data=data,
            method=method,
            headers={"Content-Type": "application/json"},
        )
        try:
            with self.opener.open(request, timeout=self.timeout) as response:
                body = response.read().decode("utf-8")
        except urllib.error.HTTPError as exc:
            detail = exc.read().decode("utf-8", errors="replace")
            raise SmokeFailure(f"{method} {path} failed with {exc.code}: {detail}") from exc
        return json.loads(body)

    def login(self, token: str) -> None:
        if not token:
            return
        auth = self.request_json("POST", "/api/control-plane/login", {"token": token})
        expect(bool(auth.get("authenticated")), f"control-plane login failed: {auth}")


def expect(condition: bool, message: str) -> None:
    if not condition:
        raise SmokeFailure(message)


def assert_endpoint(spec: dict[str, Any], method: str, path: str) -> None:
    operations = spec.get("paths", {}).get(path, {})
    expect(method.lower() in operations, f"OpenAPI missing {method} {path}")


def validate_foundation(foundation: dict[str, Any]) -> None:
    contracts = foundation.get("contracts") or []
    task_types = foundation.get("task_types") or []
    hooks = foundation.get("hook_bindings") or []
    active_truths = foundation.get("active_system_truths") or []
    capabilities = set(foundation.get("store_capabilities") or [])
    expect(contracts, "foundation has no runtime contracts")
    expect(task_types, "foundation has no task type registrations")
    expect(hooks, "foundation has no hook bindings")
    expect(active_truths, "foundation has no active System Truth pointers")
    for capability in {"runtime_contracts", "task_type_registry", "hook_bindings", "system_truth_lifecycle"}:
        expect(capability in capabilities, f"foundation missing store capability {capability}")
    expect(any(item.get("task_type") == "runtime_validation" for item in contracts), "runtime_validation contract missing")
    expect(any(item.get("type_key") == "runtime_validation" for item in task_types), "runtime_validation task type missing")
    expect(any(item.get("binding_ref") == "runtime_contract_guard" for item in hooks), "runtime_contract_guard hook missing")


def validate_runtime_records(client: RuntimeFoundationClient, run_id: str) -> dict[str, int]:
    detail = client.request_json("GET", f"/api/control-plane/runtime/runs/{run_id}")
    steps = client.request_json("GET", f"/api/control-plane/runtime/runs/{run_id}/steps").get("items") or []
    lifecycle = client.request_json("GET", f"/api/control-plane/runtime/runs/{run_id}/lifecycle").get("items") or []
    traces = client.request_json("GET", f"/api/control-plane/runtime/runs/{run_id}/traces").get("items") or []
    usage = client.request_json("GET", f"/api/control-plane/runtime/runs/{run_id}/usage").get("items") or []
    projections = client.request_json("GET", f"/api/control-plane/runtime/runs/{run_id}/projections").get("items") or []
    checkpoints = client.request_json("GET", f"/api/control-plane/runtime/runs/{run_id}/checkpoints").get("items") or []
    expect(detail.get("id") == run_id, f"runtime run detail id mismatch: {detail}")
    expect(steps, "runtime run has no persisted steps")
    expect(lifecycle, "runtime run has no lifecycle events")
    expect(traces, "runtime run has no traces")
    expect(usage, "runtime run has no usage records")
    expect(projections, "runtime run has no projection candidates")
    for checkpoint in checkpoints:
        expect("payload" not in checkpoint, f"checkpoint readout leaked raw payload: {checkpoint}")
        expect("resume_token" not in checkpoint, f"checkpoint readout leaked raw resume token: {checkpoint}")
    return {
        "steps": len(steps),
        "lifecycle": len(lifecycle),
        "traces": len(traces),
        "usage": len(usage),
        "projections": len(projections),
        "checkpoints": len(checkpoints),
    }


def run_api_smoke(base_url: str, token: str, timeout: int) -> dict[str, Any]:
    client = RuntimeFoundationClient(base_url, timeout)
    health = client.request_json("GET", "/healthz")
    expect(health.get("status") == "ok", f"healthz failed: {health}")
    client.login(token)
    bootstrap = client.request_json("GET", "/api/control-plane/bootstrap")
    expect(bootstrap.get("swagger_spec_url"), "bootstrap missing swagger_spec_url")
    spec = client.request_json("GET", "/swagger/openapi.json")
    for method, path in [
        ("POST", "/api/control-plane/runtime/validation-runs"),
        ("GET", "/api/control-plane/runtime/contracts/foundation"),
        ("GET", "/api/control-plane/runtime/runs"),
        ("GET", "/api/control-plane/runtime/runs/{runID}/steps"),
        ("GET", "/api/control-plane/runtime/runs/{runID}/traces"),
        ("GET", "/api/control-plane/runtime/runs/{runID}/usage"),
        ("GET", "/api/control-plane/runtime/runs/{runID}/projections"),
        ("GET", "/api/control-plane/runtime/runs/{runID}/checkpoints"),
    ]:
        assert_endpoint(spec, method, path)

    foundation = client.request_json("GET", "/api/control-plane/runtime/contracts/foundation")
    validate_foundation(foundation)
    validation = client.request_json(
        "POST",
        "/api/control-plane/runtime/validation-runs",
        {
            "workspace_id": "system-validation-smoke",
            "scene": "system_validation",
            "prompt": "validate runtime foundation smoke",
            "metadata": {"source": "control_plane_runtime_foundation_smoke"},
        },
    )
    run = validation.get("run") or {}
    run_id = run.get("id")
    expect(bool(run_id), f"validation run missing run id: {validation}")
    expect((validation.get("validation_mcp") or {}).get("result"), "validation run missing validation_mcp result")
    expect((validation.get("sandbox") or {}).get("sandbox_ref"), "validation run missing sandbox ref")
    counts = validate_runtime_records(client, run_id)
    return {
        "status": "ok",
        "base_url": base_url,
        "run_id": run_id,
        "foundation": {
            "contracts": len(foundation.get("contracts") or []),
            "task_types": len(foundation.get("task_types") or []),
            "hook_bindings": len(foundation.get("hook_bindings") or []),
            "active_system_truths": len(foundation.get("active_system_truths") or []),
        },
        "records": counts,
    }


def run_dom_smoke(web_url: str, token: str) -> dict[str, Any]:
    try:
        from playwright.sync_api import sync_playwright
    except ModuleNotFoundError as exc:
        raise SmokeFailure("Python Playwright is required for --web-url DOM validation") from exc

    with sync_playwright() as playwright:
        browser = playwright.chromium.launch()
        try:
            page = browser.new_page(viewport={"width": 1440, "height": 1100})
            page.goto(web_url.rstrip("/") + "/", wait_until="networkidle")
            if token and page.locator('input[name="control-plane-token"]').count() > 0:
                page.fill('input[name="control-plane-token"]', token)
                page.get_by_role("button", name="登录控制面").click()
                page.wait_for_load_state("networkidle")
            expect_unique_visible_test_id(page, "nav-system-validation").click()
            page.wait_for_selector('[data-testid="system-validation-panel"]', state="visible")
            for test_id in [
                "system-validation-panel",
                "runtime-readout",
                "runtime-checkpoint-readout",
                "runtime-contract-foundation",
                "runtime-foundation-capabilities",
                "runtime-contract-editor",
                "runtime-task-type-editor",
                "runtime-hook-binding-editor",
                "runtime-validation-trigger",
                "runtime-foundation-save",
                "runtime-foundation-refresh",
            ]:
                expect_unique_visible_test_id(page, test_id)
        finally:
            browser.close()
    return {"web_url": web_url, "dom": "ok"}


def expect_unique_visible_test_id(page: Any, test_id: str) -> Any:
    locator = page.get_by_test_id(test_id)
    count = locator.count()
    expect(count == 1, f"DOM data-testid={test_id} count={count}, want 1")
    expect(locator.is_visible(), f"DOM data-testid={test_id} is not visible")
    return locator


def main() -> int:
    args = parse_args()
    try:
        result = run_api_smoke(args.base_url, args.control_plane_token, args.timeout_seconds)
        if args.web_url:
            result["web"] = run_dom_smoke(args.web_url, args.control_plane_token)
        print(json.dumps(result, ensure_ascii=False, indent=2))
        return 0
    except Exception as exc:
        print(str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
