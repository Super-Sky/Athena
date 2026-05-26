// types.ts defines the frontend view models consumed from the control-plane bootstrap and edit APIs.
// types.ts 定义前端从控制面 bootstrap 和编辑接口消费的视图模型。
export type SceneConfig = {
  id: string;
  description?: string;
  keywords?: string[];
  default_skills?: string[];
  suggested_questions?: string[];
  enabled: boolean;
  match_score?: number;
};

export type SkillConfig = {
  name: string;
  description?: string;
  guidance?: string;
  tool_names?: string[];
  enabled: boolean;
};

export type SkillItem = {
  name: string;
  source: string;
  description?: string;
  guidance?: string;
  tool_names?: string[];
};

export type SkillPackageValidation = {
  valid: boolean;
  errors?: string[];
  warnings?: string[];
};

export type SkillPackageMetadata = {
  id: string;
  name: string;
  revision: number;
  file_count: number;
  file_paths?: string[];
  enabled: boolean;
  validation: SkillPackageValidation;
  uploaded_at?: string;
};

export type SkillPackageDetail = {
  metadata: SkillPackageMetadata;
  files: Record<string, string>;
};

export type SkillPackageFilesInput = {
  name?: string;
  enabled?: boolean;
  files: Record<string, string>;
};

export type SkillPackageRollbackResult = {
  metadata: SkillPackageMetadata;
  rolled_back_from: number;
  current_revision: number;
};

export type ToolConfig = {
  name: string;
  description?: string;
  tool_scope?: string;
  requires_confirmation: boolean;
  side_effect_level?: string;
  input_schema_summary?: string;
  output_schema_summary?: string;
  enabled: boolean;
};

export type GovernanceConfig = {
  choice_required_enabled: boolean;
  automation_fallback_enabled: boolean;
  planning_progress_enabled: boolean;
  fact_quality_gate_enabled: boolean;
  tool_hint_emission_enabled: boolean;
  knowledge_retrieval_emission_enabled: boolean;
  max_planning_steps: number;
  max_tool_hints: number;
};

export type RuntimeTuning = GovernanceConfig;

export type RuntimeRun = {
  id: string;
  task_id: string;
  task_type?: string;
  task_subtype?: string;
  input_kind?: string;
  scene?: string;
  workspace_id?: string;
  app_instance_id?: string;
  status: string;
  idempotency_scope?: string;
  idempotency_key?: string;
  retention_policy?: string;
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  started_at?: string;
  completed_at?: string;
};

export type RuntimeStep = {
  id: string;
  run_id: string;
  sequence: number;
  step_type?: string;
  name?: string;
  status: string;
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  started_at?: string;
  completed_at?: string;
};

export type RuntimeLifecycleEvent = {
  id: string;
  run_id: string;
  step_id?: string;
  event_type: string;
  subject_type: string;
  subject_id: string;
  from_status?: string;
  to_status?: string;
  reason?: string;
  metadata?: Record<string, unknown>;
  occurred_at: string;
};

export type RuntimeTrace = {
  id: string;
  run_id: string;
  step_id?: string;
  trace_type?: string;
  summary: string;
  safe_labels?: Record<string, string>;
  redacted_payload?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
  created_at: string;
};

export type RuntimeUsage = {
  id: string;
  run_id: string;
  step_id?: string;
  resource_type: string;
  provider?: string;
  resource_name?: string;
  unit: string;
  amount: number;
  cost?: number;
  currency?: string;
  metadata?: Record<string, unknown>;
  created_at: string;
};

export type RuntimeProjectionCandidate = {
  id: string;
  run_id: string;
  step_id?: string;
  candidate_kind: string;
  status?: string;
  summary?: string;
  schema_version?: string;
  redacted_payload?: Record<string, unknown>;
  semantic_payload?: Record<string, unknown>;
  artifact_refs?: Record<string, unknown>;
  ui_hints?: Record<string, unknown>;
  materialization_target?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
  created_at: string;
};

export type RuntimeCheckpointReadout = {
  checkpoint_id: string;
  run_id: string;
  stage?: string;
  resume_token_present: boolean;
  payload_size?: number;
  payload_sha256?: string;
  created_at?: string;
  updated_at?: string;
  snapshot_available: boolean;
  source?: string;
};

export type RuntimeContract = {
  id: string;
  name: string;
  version: string;
  status: string;
  task_type: string;
  input_schema?: Record<string, unknown>;
  execution_profile?: Record<string, unknown>;
  exit_policy?: Record<string, unknown>;
  capability_profile?: Record<string, unknown>;
  governance_policy_refs?: Record<string, unknown>;
  hook_bindings?: Record<string, unknown>;
  projection_policy?: Record<string, unknown>;
  system_truth_refs?: Record<string, unknown>;
  idempotency_scope?: string;
  idempotency_key?: string;
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export type RuntimeTaskTypeRegistration = {
  id: string;
  type_key: string;
  display_name?: string;
  description?: string;
  status: string;
  input_schema?: Record<string, unknown>;
  validator_refs?: Record<string, unknown>;
  default_contract_id?: string;
  compatibility?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export type RuntimeHookBinding = {
  id: string;
  contract_id: string;
  hook_point: string;
  binding_kind: string;
  binding_ref: string;
  order_index: number;
  enabled: boolean;
  failure_policy: string;
  config?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export type SystemTruthActiveVersion = {
  id: string;
  asset_id: string;
  compile_result_id: string;
  draft_id: string;
  activated_by?: string;
  reason?: string;
  rollback_from_id?: string;
  metadata?: Record<string, unknown>;
  activated_at: string;
};

export type RuntimeContractFoundation = {
  contracts: RuntimeContract[];
  task_types: RuntimeTaskTypeRegistration[];
  hook_bindings: RuntimeHookBinding[];
  active_system_truths: SystemTruthActiveVersion[];
  store_capabilities: string[];
  unavailable_surfaces?: string[];
};

export type RuntimeContractUpsertInput = {
  name: string;
  version: string;
  status: string;
  task_type: string;
  input_schema?: Record<string, unknown>;
  execution_profile?: Record<string, unknown>;
  exit_policy?: Record<string, unknown>;
  capability_profile?: Record<string, unknown>;
  governance_policy_refs?: Record<string, unknown>;
  hook_bindings?: Record<string, unknown>;
  projection_policy?: Record<string, unknown>;
  system_truth_refs?: Record<string, unknown>;
  idempotency_scope?: string;
  idempotency_key?: string;
  metadata?: Record<string, unknown>;
};

export type RuntimeTaskTypeUpsertInput = {
  id?: string;
  display_name?: string;
  description?: string;
  status: string;
  input_schema?: Record<string, unknown>;
  validator_refs?: Record<string, unknown>;
  default_contract_id?: string;
  compatibility?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
};

export type RuntimeHookBindingUpsertInput = {
  contract_id: string;
  hook_point: string;
  binding_kind: string;
  binding_ref: string;
  order_index: number;
  enabled: boolean;
  failure_policy: string;
  config?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
};

export type RuntimeValidationRunInput = {
  workspace_id?: string;
  scene?: string;
  prompt?: string;
  metadata?: Record<string, unknown>;
};

export type ExternalSandboxRef = {
  ref_id: string;
  mode: string;
  provider: string;
  boundary: string;
  status: string;
  operation: string;
  resource: string;
  metadata?: Record<string, unknown>;
};

export type ExternalSandboxAuditSummary = {
  summary: string;
  credential_scope: string;
  context_scope: string;
  nested_execution: string;
  state_integrity: string;
  allowed_output_classes?: string[];
  safe_labels?: Record<string, string>;
};

export type ExternalSandboxStructuredResult = {
  status: string;
  result_type: string;
  summary: string;
  output?: Record<string, unknown>;
  redacted_input?: Record<string, unknown>;
};

export type ExternalSandboxValidationResult = {
  sandbox_ref: ExternalSandboxRef;
  structured_result: ExternalSandboxStructuredResult;
  audit_summary: ExternalSandboxAuditSummary;
  projection?: Record<string, unknown>;
};

export type RuntimeValidationRunResponse = {
  run: RuntimeRun;
  step: RuntimeStep;
  events: RuntimeLifecycleEvent[];
  trace: RuntimeTrace;
  usage: RuntimeUsage;
  projection: RuntimeProjectionCandidate;
  validation_mcp: ValidationMCPInvocationResponse;
  validation_mcp_trace: RuntimeTrace;
  validation_mcp_usage: RuntimeUsage;
  validation_mcp_projection: RuntimeProjectionCandidate;
  sandbox: ExternalSandboxValidationResult;
  sandbox_event: RuntimeLifecycleEvent;
  sandbox_trace: RuntimeTrace;
  sandbox_usage: RuntimeUsage;
  sandbox_projection: RuntimeProjectionCandidate;
};

export type ConfigVersionSummary = {
  version_id: string;
  created_at: string;
  created_by?: string;
  summary?: string;
};

export type ConfigVersionDetail = ConfigVersionSummary & {
  document: {
    scenes?: SceneConfig[];
    skills?: SkillConfig[];
    tools?: ToolConfig[];
    governance?: GovernanceConfig;
    runtime?: RuntimeTuning;
  };
};

export type TruthDirInfo = {
  path?: string;
  version?: string;
};

export type ControlPlaneAuthStatus = {
  authenticated: boolean;
  lock_state?: string;
  remaining_attempts?: number;
  failed_attempts?: number;
  session_expires_at?: string;
  truth_dir?: TruthDirInfo;
};

export type ControlPlaneLoginInput = {
  token: string;
};

export type SystemResourceSummary = {
  asset_id: string;
  asset_type: string;
  asset_name?: string;
  scope?: string;
  source_kind?: string;
  status?: string;
  truth_dir_version?: string;
  compiled_version?: string;
  updated_at?: string;
  read_only?: boolean;
};

export type SystemResourceParseResult = {
  asset_id?: string;
  status?: string;
  summary?: string;
  warnings?: string[];
  errors?: string[];
  parsed?: Record<string, unknown>;
  source_hash?: string;
  updated_at?: string;
};

export type SystemResourceCompileResult = {
  asset_id?: string;
  status?: string;
  summary?: string;
  guidance_text?: string;
  source_checksum?: string;
  compiled_checksum?: string;
  compiled_version?: string;
  truth_dir_version?: string;
  payload?: Record<string, unknown>;
  updated_at?: string;
};

export type SystemResourcePipeline = {
  pipeline_id?: string;
  asset_id?: string;
  status?: string;
  current_step?: string;
  progress_percent?: number;
  started_at?: string;
  updated_at?: string;
  warnings?: string[];
  errors?: string[];
};

export type SystemResourceDetail = SystemResourceSummary & {
  source_path?: string;
  metadata?: Record<string, unknown>;
  parse_result?: SystemResourceParseResult;
  compile_result?: SystemResourceCompileResult;
  pipeline?: SystemResourcePipeline;
};

export type SystemResourceVersionSummary = {
  version_id: string;
  asset_id: string;
  action?: string;
  summary?: string;
  created_at?: string;
  truth_dir_version?: string;
  compiled_version?: string;
  source_checksum?: string;
  compiled_checksum?: string;
  rolled_back_from?: string;
};

export type SystemResourceVersionDetail = SystemResourceVersionSummary & {
  resource: SystemResourceDetail;
  source_content?: string;
};

export type SystemResourceAuditEntry = {
  event_id: string;
  asset_id: string;
  action?: string;
  summary?: string;
  created_at?: string;
  truth_dir_version?: string;
  compiled_version?: string;
  source_checksum?: string;
  compiled_checksum?: string;
  rolled_back_from?: string;
  detail?: Record<string, unknown>;
};

export type SystemResourceSource = {
  asset_id?: string;
  source_content?: string;
  message?: string;
  updated_at?: string;
};

export type SystemResourceMetadataPatch = {
  asset_type?: string;
  asset_name?: string;
  scope?: string;
  source_kind?: string;
  read_only?: boolean;
  metadata?: Record<string, unknown>;
};

export type SystemResourceCreateInput = {
  asset_id: string;
  asset_type?: string;
  asset_name?: string;
  scope?: string;
  source_kind?: string;
  read_only?: boolean;
  source_content?: string;
  message?: string;
  metadata?: Record<string, unknown>;
};

export type SystemResourceMutationResult = {
  asset_id?: string;
  accepted: boolean;
  pipeline: SystemResourcePipeline;
};

export type SystemResourceDebugPayload = {
  endpoint: string;
  payload: Record<string, unknown>;
};

export type ToolGovernanceRule = {
  rule_id?: string;
  match_tool?: string;
  match_scope?: string;
  match_operation?: string;
  match_risk?: string;
  decision: string;
  reason?: string;
  redact_fields?: string[];
  sandbox_ref?: string;
  metadata?: Record<string, unknown>;
};

export type ToolGovernancePolicy = {
  policy_id?: string;
  asset_id?: string;
  name?: string;
  default_decision?: string;
  decision_model?: string;
  rules?: ToolGovernanceRule[];
  compiled_version?: string;
  truth_dir_version?: string;
  source_checksum?: string;
  updated_at?: string;
  metadata?: Record<string, unknown>;
};

export type ToolGovernanceDecisionRequest = {
  tool_name: string;
  tool_scope?: string;
  operation?: string;
  risk_level?: string;
  metadata?: Record<string, unknown>;
};

export type ToolGovernanceDecision = {
  decision_id: string;
  decision: string;
  reason?: string;
  matched_rule_id?: string;
  policy_asset_id?: string;
  policy_version?: string;
  tool_name?: string;
  tool_scope?: string;
  operation?: string;
  risk_level?: string;
  redact_fields?: string[];
  sandbox_ref?: string;
  evaluated_at?: string;
  truth_dir_version?: string;
  metadata?: Record<string, unknown>;
};

export type ValidationMCPToolSchema = {
  name: string;
  description?: string;
  tool_scope?: string;
  operation?: string;
  risk_level?: string;
  input_schema?: Record<string, unknown>;
  output_schema?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
};

export type ValidationMCPServerInfo = {
  server_id: string;
  name: string;
  transport: string;
  status: string;
  tools?: ValidationMCPToolSchema[];
  metadata?: Record<string, unknown>;
};

export type ValidationMCPInvocationRequest = {
  tool_name: string;
  input?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
};

export type ValidationMCPInvocationTrace = {
  trace_id: string;
  trace_type: string;
  summary: string;
  safe_labels?: Record<string, string>;
  redacted_payload?: Record<string, unknown>;
};

export type ValidationMCPInvocationResult = {
  invocation_id: string;
  server_id: string;
  tool_name: string;
  status: string;
  result_summary: string;
  output?: Record<string, unknown>;
  trace: ValidationMCPInvocationTrace;
  applied_redaction: boolean;
  metadata?: Record<string, unknown>;
};

export type ValidationMCPInvocationResponse = {
  server: ValidationMCPServerInfo;
  tool: ValidationMCPToolSchema;
  request: ValidationMCPInvocationRequest;
  governance_decision: ToolGovernanceDecision;
  result: ValidationMCPInvocationResult;
};

export type SystemResourceExportInfo = {
  truth_dir_version?: string;
  export_file?: string;
  asset_count?: number;
};

export type CompiledAssetsPackageItem = {
  asset_id: string;
  asset_type?: string;
  asset_name?: string;
  source_path?: string;
  compiled_version?: string;
  compiled_checksum?: string;
  truth_dir_version?: string;
  package_file?: string;
};

export type CompiledAssetsPackageManifest = {
  output_dir?: string;
  truth_dir_path?: string;
  truth_dir_version?: string;
  built_at?: string;
  asset_count?: number;
  assets?: CompiledAssetsPackageItem[];
};

export type SystemResourceDraft = {
  asset_id: string;
  asset_type: string;
  asset_name: string;
  scope: string;
  source_kind: string;
  read_only: boolean;
  metadata_text: string;
  source_content: string;
  message: string;
};

export type BootstrapPayload = {
  scenes: SceneConfig[];
  skills: SkillConfig[];
  tools: ToolConfig[];
  system_resources: SystemResourceSummary[];
  governance: GovernanceConfig;
  runtime: RuntimeTuning;
  config_versions: ConfigVersionSummary[];
  swagger_spec_url: string;
};

export type ProviderModelRecord = {
  id: string;
  provider_id: string;
  model_id: string;
  display_name: string;
  enabled: boolean;
  is_default: boolean;
  is_fallback: boolean;
};

export type ProviderDefinition = {
  id: string;
  name: string;
  base_url?: string;
  protocol: string;
  request_timeout_seconds?: number;
  enabled: boolean;
  api_key_configured: boolean;
  api_key_masked?: string;
  headers?: Record<string, string>;
  models?: ProviderModelRecord[];
};

export type ProviderModelInput = {
  model_id: string;
  display_name: string;
  enabled?: boolean;
  is_default: boolean;
  is_fallback: boolean;
};

export type ProviderInput = {
  name: string;
  base_url?: string;
  protocol: string;
  api_key?: string;
  request_timeout_seconds?: number;
  headers?: Record<string, string>;
  enabled?: boolean;
  models?: ProviderModelInput[];
};

export type ProviderPatchInput = {
  enabled?: boolean;
};

export type ProviderModelPatchInput = {
  enabled?: boolean;
  is_default?: boolean;
  is_fallback?: boolean;
};

export type ModelTestResult = {
  provider_id: string;
  model_record_id: string;
  provider_name: string;
  model_id: string;
  display_name: string;
  available: boolean;
  duration_ms: number;
  error?: string;
};
