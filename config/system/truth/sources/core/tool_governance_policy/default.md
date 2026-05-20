---
id: default_tool_governance
name: Default Tool Governance
summary: Core pre-execution decisions for runtime tool requests.
default_decision: allow
decision_model: first_match
rules:
  - id: deny_credential_export
    match_tool: credential_export
    match_scope: secret
    match_operation: read
    match_risk: high
    decision: deny
    reason: Raw credential export is never allowed.
  - id: redact_external_read
    match_tool: demo_browser
    match_scope: external_web
    match_operation: read
    match_risk: medium
    decision: allow_with_redaction
    reason: External reads may proceed with credential-bearing fields redacted.
    redact_fields:
      - headers.authorization
      - query.credentials
  - id: redact_validation_mcp_risk_lookup
    match_tool: risk_signal_lookup
    match_scope: validation_mcp
    match_operation: invoke
    match_risk: medium
    decision: allow_with_redaction
    reason: Validation MCP risk lookups may proceed with credential-bearing fields redacted.
    redact_fields:
      - input.credentials
      - metadata.authorization_token
  - id: require_sandbox_for_local_write
    match_tool: local_shell
    match_scope: workspace
    match_operation: write
    match_risk: high
    decision: require_sandbox_ref
    reason: High-risk workspace writes require an explicit sandbox reference.
    sandbox_ref: workspace-write
---

## Purpose

Keep tool execution governance in system truth so runtime requests can be classified before execution.

## Decisions

- `allow`: proceed without extra controls.
- `deny`: block the request.
- `allow_with_redaction`: proceed only after configured fields are redacted.
- `require_sandbox_ref`: proceed only when the request names the sandbox boundary.
