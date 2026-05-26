// api.ts centralizes browser-side calls to Athena's control-plane HTTP endpoints.
// api.ts 负责集中封装浏览器端对 Athena 控制面 HTTP 接口的调用。
import type {
  BootstrapPayload,
  CompiledAssetsPackageManifest,
  ConfigVersionDetail,
  ConfigVersionSummary,
  ControlPlaneAuthStatus,
  ControlPlaneLoginInput,
  GovernanceConfig,
  ModelTestResult,
  ProviderDefinition,
  ProviderInput,
  ProviderModelRecord,
  ProviderModelInput,
  ProviderModelPatchInput,
  ProviderPatchInput,
  RuntimeContractFoundation,
  RuntimeContractUpsertInput,
  RuntimeCheckpointReadout,
  RuntimeHookBindingUpsertInput,
  RuntimeLifecycleEvent,
  RuntimeProjectionCandidate,
  RuntimeRun,
  RuntimeStep,
  RuntimeTaskTypeUpsertInput,
  RuntimeTuning,
  RuntimeTrace,
  RuntimeUsage,
  RuntimeValidationRunInput,
  RuntimeValidationRunResponse,
  SceneConfig,
  SkillConfig,
  SkillItem,
  SkillPackageDetail,
  SkillPackageFilesInput,
  SkillPackageMetadata,
  SkillPackageRollbackResult,
  SystemResourceCompileResult,
  SystemResourceCreateInput,
  SystemResourceDebugPayload,
  SystemResourceDetail,
  SystemResourceVersionSummary,
  SystemResourceVersionDetail,
  SystemResourceAuditEntry,
  SystemResourceExportInfo,
  SystemResourceMetadataPatch,
  SystemResourceMutationResult,
  SystemResourceParseResult,
  SystemResourcePipeline,
  SystemResourceSource,
  ToolGovernanceDecision,
  ToolGovernanceDecisionRequest,
  ToolGovernancePolicy,
  ToolConfig,
  ValidationMCPInvocationRequest,
  ValidationMCPInvocationResponse,
  ValidationMCPServerInfo,
  ValidationMCPToolSchema
} from "./types";

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {})
    },
    ...init
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `${response.status} ${response.statusText}`);
  }
  return response.json() as Promise<T>;
}

async function requestForm<T>(url: string, formData: FormData, method: "POST" | "PUT") {
  const response = await fetch(url, {
    method,
    credentials: "include",
    body: formData
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `${response.status} ${response.statusText}`);
  }
  return response.json() as Promise<T>;
}

async function requestText(url: string, init?: RequestInit): Promise<string> {
  const response = await fetch(url, {
    credentials: "include",
    headers: {
      ...(init?.headers ?? {})
    },
    ...init
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `${response.status} ${response.statusText}`);
  }
  return response.text();
}

async function requestBlob(url: string, init?: RequestInit): Promise<{ blob: Blob; headers: Headers }> {
  const response = await fetch(url, {
    credentials: "include",
    headers: {
      ...(init?.headers ?? {})
    },
    ...init
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `${response.status} ${response.statusText}`);
  }
  return {
    blob: await response.blob(),
    headers: response.headers
  };
}

export function loadControlPlaneAuthStatus() {
  return request<ControlPlaneAuthStatus>("/api/control-plane/auth/status");
}

export function loginControlPlane(input: ControlPlaneLoginInput) {
  return request<ControlPlaneAuthStatus>("/api/control-plane/login", {
    method: "POST",
    body: JSON.stringify(input)
  });
}

export function logoutControlPlane() {
  return request<ControlPlaneAuthStatus>("/api/control-plane/logout", {
    method: "POST"
  });
}

export function loadBootstrap() {
  return request<BootstrapPayload>("/api/control-plane/bootstrap");
}

export function saveScene(scene: SceneConfig) {
  return request<SceneConfig>(`/api/control-plane/scenes/${encodeURIComponent(scene.id)}`, {
    method: "PUT",
    body: JSON.stringify(scene)
  });
}

export function saveSkill(skill: SkillConfig) {
  return request<SkillConfig>(`/api/control-plane/skills/${encodeURIComponent(skill.name)}`, {
    method: "PUT",
    body: JSON.stringify(skill)
  });
}

export function loadVisibleSkills() {
  return request<{ items: SkillItem[] }>("/api/skills");
}

export function loadSkillPackages() {
  return request<{ items: SkillPackageMetadata[] }>("/api/skills/packages");
}

export function loadSkillPackage(packageID: string) {
  return request<SkillPackageDetail>(`/api/skills/packages/${encodeURIComponent(packageID)}`);
}

export function loadSkillPackageRevisions(packageID: string) {
  return request<{ items: SkillPackageMetadata[] }>(`/api/skills/packages/${encodeURIComponent(packageID)}/revisions`);
}

export function uploadSkillPackageFiles(input: SkillPackageFilesInput) {
  return request<SkillPackageMetadata>("/api/skills/packages", {
    method: "POST",
    body: JSON.stringify(input)
  });
}

export function replaceSkillPackageFiles(packageID: string, input: SkillPackageFilesInput) {
  return request<SkillPackageMetadata>(`/api/skills/packages/${encodeURIComponent(packageID)}`, {
    method: "PUT",
    body: JSON.stringify(input)
  });
}

export function uploadSkillPackageBundle(file: File) {
  const formData = new FormData();
  formData.append("bundle", file);
  return requestForm<SkillPackageMetadata>("/api/skills/packages", formData, "POST");
}

export function replaceSkillPackageBundle(packageID: string, file: File) {
  const formData = new FormData();
  formData.append("bundle", file);
  return requestForm<SkillPackageMetadata>(`/api/skills/packages/${encodeURIComponent(packageID)}`, formData, "PUT");
}

export function patchSkillPackageEnabled(packageID: string, enabled: boolean) {
  return request<SkillPackageMetadata>(`/api/skills/packages/${encodeURIComponent(packageID)}`, {
    method: "PATCH",
    body: JSON.stringify({ enabled })
  });
}

export function rollbackSkillPackage(packageID: string, revision: number) {
  return request<SkillPackageRollbackResult>(`/api/skills/packages/${encodeURIComponent(packageID)}/rollback`, {
    method: "POST",
    body: JSON.stringify({ revision })
  });
}

export function saveRuntime(runtime: RuntimeTuning) {
  return request<RuntimeTuning>("/api/control-plane/runtime-config", {
    method: "PUT",
    body: JSON.stringify(runtime)
  });
}

export function loadRuntimeRuns(limit = 20) {
  const query = limit > 0 ? `?limit=${encodeURIComponent(String(limit))}` : "";
  return request<{ items: RuntimeRun[] }>(`/api/control-plane/runtime/runs${query}`);
}

export function loadRuntimeRun(runID: string) {
  return request<RuntimeRun>(`/api/control-plane/runtime/runs/${encodeURIComponent(runID)}`);
}

export function loadRuntimeSteps(runID: string) {
  return request<{ items: RuntimeStep[] }>(`/api/control-plane/runtime/runs/${encodeURIComponent(runID)}/steps`);
}

export function loadRuntimeLifecycleEvents(runID: string) {
  return request<{ items: RuntimeLifecycleEvent[] }>(`/api/control-plane/runtime/runs/${encodeURIComponent(runID)}/lifecycle`);
}

export function loadRuntimeTraces(runID: string, limit = 100) {
  const query = limit > 0 ? `?limit=${encodeURIComponent(String(limit))}` : "";
  return request<{ items: RuntimeTrace[] }>(`/api/control-plane/runtime/runs/${encodeURIComponent(runID)}/traces${query}`);
}

export function loadRuntimeUsage(runID: string, limit = 100) {
  const query = limit > 0 ? `?limit=${encodeURIComponent(String(limit))}` : "";
  return request<{ items: RuntimeUsage[] }>(`/api/control-plane/runtime/runs/${encodeURIComponent(runID)}/usage${query}`);
}

export function loadRuntimeProjectionCandidates(runID: string, limit = 100) {
  const query = limit > 0 ? `?limit=${encodeURIComponent(String(limit))}` : "";
  return request<{ items: RuntimeProjectionCandidate[] }>(`/api/control-plane/runtime/runs/${encodeURIComponent(runID)}/projections${query}`);
}

export function loadRuntimeCheckpoints(runID: string) {
  return request<{ items: RuntimeCheckpointReadout[] }>(`/api/control-plane/runtime/runs/${encodeURIComponent(runID)}/checkpoints`);
}

export function loadRuntimeContractFoundation() {
  return request<RuntimeContractFoundation>("/api/control-plane/runtime/contracts/foundation");
}

export function saveRuntimeContract(contractID: string, input: RuntimeContractUpsertInput) {
  return request(`/api/control-plane/runtime/contracts/${encodeURIComponent(contractID)}`, {
    method: "PUT",
    body: JSON.stringify(input)
  });
}

export function saveRuntimeTaskType(typeKey: string, input: RuntimeTaskTypeUpsertInput) {
  return request(`/api/control-plane/runtime/task-types/${encodeURIComponent(typeKey)}`, {
    method: "PUT",
    body: JSON.stringify(input)
  });
}

export function saveRuntimeHookBinding(bindingID: string, input: RuntimeHookBindingUpsertInput) {
  return request(`/api/control-plane/runtime/hook-bindings/${encodeURIComponent(bindingID)}`, {
    method: "PUT",
    body: JSON.stringify(input)
  });
}

export function createRuntimeValidationRun(input: RuntimeValidationRunInput) {
  return request<RuntimeValidationRunResponse>("/api/control-plane/runtime/validation-runs", {
    method: "POST",
    body: JSON.stringify(input)
  });
}

export function saveTool(tool: ToolConfig) {
  return request<ToolConfig>(`/api/control-plane/tools/${encodeURIComponent(tool.name)}`, {
    method: "PUT",
    body: JSON.stringify(tool)
  });
}

export function saveGovernance(governance: GovernanceConfig) {
  return request<GovernanceConfig>("/api/control-plane/governance", {
    method: "PUT",
    body: JSON.stringify(governance)
  });
}

export function loadToolGovernancePolicy() {
  return request<ToolGovernancePolicy>("/api/control-plane/tool-governance/policy");
}

export function loadToolGovernanceDecisions() {
  return request<{ items: ToolGovernanceDecision[] }>("/api/control-plane/tool-governance/decisions");
}

export function evaluateToolGovernance(input: ToolGovernanceDecisionRequest) {
  return request<ToolGovernanceDecision>("/api/control-plane/tool-governance/evaluate", {
    method: "POST",
    body: JSON.stringify(input)
  });
}

export function loadValidationMCPServer() {
  return request<ValidationMCPServerInfo>("/api/control-plane/validation-mcp/server");
}

export function loadValidationMCPTools() {
  return request<{ items: ValidationMCPToolSchema[] }>("/api/control-plane/validation-mcp/tools");
}

export function invokeValidationMCPTool(input: ValidationMCPInvocationRequest) {
  return request<ValidationMCPInvocationResponse>("/api/control-plane/validation-mcp/invocations", {
    method: "POST",
    body: JSON.stringify(input)
  });
}

export function loadConfigVersions() {
  return request<{ items: ConfigVersionSummary[] }>("/api/control-plane/config-versions");
}

export function loadConfigVersion(versionID: string) {
  return request<ConfigVersionDetail>(`/api/control-plane/config-versions/${encodeURIComponent(versionID)}`);
}

export function rollbackConfigVersion(versionID: string) {
  return request<ConfigVersionDetail>(`/api/control-plane/config-versions/${encodeURIComponent(versionID)}/rollback`, {
    method: "POST"
  });
}

export function loadSystemResources() {
  return request<{ items: SystemResourceDetail[] }>("/api/system-resources");
}

export function syncSystemResources() {
  return request<{ items: SystemResourceDetail[] }>("/api/system-resources/sync", {
    method: "POST"
  });
}

export function buildSystemAssetsPackage() {
  return request<CompiledAssetsPackageManifest>("/api/system-resources/build-package", {
    method: "POST"
  });
}

export function loadSystemResource(assetID: string) {
  return request<SystemResourceDetail>(`/api/system-resources/${encodeURIComponent(assetID)}`);
}

export function loadSystemResourceVersions(assetID: string) {
  return request<{ items: SystemResourceVersionSummary[] }>(`/api/system-resources/${encodeURIComponent(assetID)}/versions`);
}

export function loadSystemResourceVersion(assetID: string, versionID: string) {
  return request<SystemResourceVersionDetail>(`/api/system-resources/${encodeURIComponent(assetID)}/versions/${encodeURIComponent(versionID)}`);
}

export function rollbackSystemResourceVersion(assetID: string, versionID: string) {
  return request<SystemResourceMutationResult>(`/api/system-resources/${encodeURIComponent(assetID)}/versions/${encodeURIComponent(versionID)}/rollback`, {
    method: "POST"
  });
}

export function loadSystemResourceAudit(assetID: string) {
  return request<{ items: SystemResourceAuditEntry[] }>(`/api/system-resources/${encodeURIComponent(assetID)}/audit`);
}

export function createSystemResource(input: SystemResourceCreateInput) {
  return request<SystemResourceMutationResult>("/api/system-resources", {
    method: "POST",
    body: JSON.stringify(input)
  });
}

export function deleteSystemResource(assetID: string) {
  return request<{ deleted: boolean; asset_id: string }>(`/api/system-resources/${encodeURIComponent(assetID)}`, {
    method: "DELETE"
  });
}

export function patchSystemResourceMetadata(assetID: string, patch: SystemResourceMetadataPatch) {
  return request<SystemResourceDetail>(`/api/system-resources/${encodeURIComponent(assetID)}/metadata`, {
    method: "PATCH",
    body: JSON.stringify(patch)
  });
}

export function loadSystemResourceSource(assetID: string) {
  return request<SystemResourceSource>(`/api/system-resources/${encodeURIComponent(assetID)}/source`);
}

export function saveSystemResourceSource(assetID: string, input: SystemResourceSource) {
  return request<SystemResourceMutationResult>(`/api/system-resources/${encodeURIComponent(assetID)}/source`, {
    method: "PUT",
    body: JSON.stringify(input)
  });
}

export function parseSystemResource(assetID: string) {
  return request<SystemResourceMutationResult>(`/api/system-resources/${encodeURIComponent(assetID)}/parse`, {
    method: "POST"
  });
}

export function compileSystemResource(assetID: string) {
  return request<SystemResourceMutationResult>(`/api/system-resources/${encodeURIComponent(assetID)}/compile`, {
    method: "POST"
  });
}

export function activateSystemResource(assetID: string) {
  return request<SystemResourceMutationResult>(`/api/system-resources/${encodeURIComponent(assetID)}/activate`, {
    method: "POST"
  });
}

export function loadSystemResourcePipeline(assetID: string) {
  return request<SystemResourcePipeline>(`/api/system-resources/${encodeURIComponent(assetID)}/pipeline`);
}

export function loadSystemResourceParseResult(assetID: string) {
  return request<SystemResourceParseResult>(`/api/system-resources/${encodeURIComponent(assetID)}/parse-result`);
}

export function loadSystemResourceCompileResult(assetID: string) {
  return request<SystemResourceCompileResult>(`/api/system-resources/${encodeURIComponent(assetID)}/compile-result`);
}

export function loadSystemResourceDebugPayload(assetID: string, endpoint: string) {
  const query = endpoint.trim() ? `?endpoint=${encodeURIComponent(endpoint.trim())}` : "";
  return request<SystemResourceDebugPayload>(`/api/system-resources/${encodeURIComponent(assetID)}/debug-payload${query}`);
}

export async function downloadSystemResource(assetID: string) {
  const result = await requestBlob(`/api/system-resources/${encodeURIComponent(assetID)}/download`);
  const contentDisposition = result.headers.get("Content-Disposition") ?? "";
  const fileMatch = /filename="?([^"]+)"?/.exec(contentDisposition);
  return {
    blob: result.blob,
    filename: fileMatch?.[1] ?? `${assetID}.txt`
  };
}

export async function exportSystemResources() {
  const result = await requestBlob("/api/system-resources/export");
  const truthDirVersion = result.headers.get("X-Truth-Dir-Version") ?? "";
  const contentDisposition = result.headers.get("Content-Disposition") ?? "";
  const fileMatch = /filename="?([^"]+)"?/.exec(contentDisposition);
  const exportFile = fileMatch?.[1] ?? "system-resources-export.zip";
  return {
    blob: result.blob,
    info: {
      truth_dir_version: truthDirVersion,
      export_file: exportFile
    } satisfies SystemResourceExportInfo
  };
}

export function loadOpenAPISpec() {
  return request<Record<string, unknown>>("/swagger/openapi.json");
}

export function loadModelProviders() {
  return request<{ items: ProviderDefinition[] }>("/api/models/providers");
}

export function createModelProvider(input: ProviderInput) {
  return request<ProviderDefinition>("/api/models/providers", {
    method: "POST",
    body: JSON.stringify(input)
  });
}

export function updateModelProvider(providerID: string, input: ProviderInput) {
  return request<ProviderDefinition>(`/api/models/providers/${encodeURIComponent(providerID)}`, {
    method: "PUT",
    body: JSON.stringify(input)
  });
}

export function patchModelProvider(providerID: string, input: ProviderPatchInput) {
  return request<ProviderDefinition>(`/api/models/providers/${encodeURIComponent(providerID)}`, {
    method: "PATCH",
    body: JSON.stringify(input)
  });
}

export function deleteModelProvider(providerID: string) {
  return request<{ status: string }>(`/api/models/providers/${encodeURIComponent(providerID)}`, {
    method: "DELETE"
  });
}

export function createProviderModel(providerID: string, input: ProviderModelInput) {
  return request<ProviderModelRecord>(`/api/models/providers/${encodeURIComponent(providerID)}/models`, {
    method: "POST",
    body: JSON.stringify(input)
  });
}

export function updateProviderModel(providerID: string, recordID: string, input: ProviderModelInput) {
  return request<ProviderModelRecord>(`/api/models/providers/${encodeURIComponent(providerID)}/models/${encodeURIComponent(recordID)}`, {
    method: "PUT",
    body: JSON.stringify(input)
  });
}

export function patchProviderModel(providerID: string, recordID: string, input: ProviderModelPatchInput) {
  return request<ProviderModelRecord>(`/api/models/providers/${encodeURIComponent(providerID)}/models/${encodeURIComponent(recordID)}`, {
    method: "PATCH",
    body: JSON.stringify(input)
  });
}

export function deleteProviderModel(providerID: string, recordID: string) {
  return request<{ status: string }>(`/api/models/providers/${encodeURIComponent(providerID)}/models/${encodeURIComponent(recordID)}`, {
    method: "DELETE"
  });
}

export function testProviderModel(providerID: string, recordID: string) {
  return request<ModelTestResult>(`/api/models/providers/${encodeURIComponent(providerID)}/models/${encodeURIComponent(recordID)}/test`, {
    method: "POST"
  });
}

export async function invokeDebugEndpoint(method: string, path: string, bodyText?: string) {
  const init: RequestInit = { method, headers: {}, credentials: "include" };
  const trimmed = (bodyText ?? "").trim();
  if (trimmed !== "" && method !== "GET" && method !== "HEAD") {
    try {
      init.body = JSON.stringify(JSON.parse(trimmed));
      init.headers = { "Content-Type": "application/json" };
    } catch {
      init.body = trimmed;
      init.headers = { "Content-Type": "text/plain" };
    }
  }
  const response = await fetch(path, init);
  const text = await response.text();
  return {
    ok: response.ok,
    status: response.status,
    statusText: response.statusText,
    body: text
  };
}
