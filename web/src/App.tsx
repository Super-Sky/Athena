// App.tsx renders the control-plane console, including scene/skill/tool editing, model governance, governance controls, versions, and Swagger tabs.
// App.tsx 负责渲染控制面控制台，包括场景、skill、tool 编辑、模型治理、治理策略、版本管理和 Swagger 标签页。
import { lazy, Suspense, startTransition, useEffect, useState } from "react";
import {
  activateSystemResource,
  buildSystemAssetsPackage,
  compileSystemResource,
  createSystemResource,
  createModelProvider,
  createProviderModel,
  createRuntimeValidationRun,
  deleteSystemResource,
  deleteModelProvider,
  deleteProviderModel,
  downloadSystemResource,
  evaluateToolGovernance,
  exportSystemResources,
  invokeDebugEndpoint,
  invokeValidationMCPTool,
  loadControlPlaneAuthStatus,
  loadBootstrap,
  loadConfigVersion,
  loadModelProviders,
  loadOpenAPISpec,
  loadRuntimeCheckpoints,
  loadRuntimeContractFoundation,
  loadRuntimeLifecycleEvents,
  loadRuntimeProjectionCandidates,
  loadRuntimeRun,
  loadRuntimeRuns,
  loadRuntimeSteps,
  loadRuntimeTraces,
  loadRuntimeUsage,
  loadToolGovernanceDecisions,
  loadToolGovernancePolicy,
  loadValidationMCPServer,
  loadValidationMCPTools,
  loadSkillPackage,
  loadSkillPackageRevisions,
  loadSkillPackages,
  loadVisibleSkills,
  loadSystemResource,
  loadSystemResourceAudit,
  loadSystemResourceCompileResult,
  loadSystemResourceDebugPayload,
  loadSystemResourceParseResult,
  loadSystemResourcePipeline,
  loadSystemResourceVersion,
  loadSystemResourceVersions,
  loadSystemResources,
  loadSystemResourceSource,
  loginControlPlane,
  logoutControlPlane,
  patchSystemResourceMetadata,
  parseSystemResource,
  patchModelProvider,
  patchProviderModel,
  patchSkillPackageEnabled,
  replaceSkillPackageBundle,
  replaceSkillPackageFiles,
  rollbackConfigVersion,
  rollbackSystemResourceVersion,
  rollbackSkillPackage,
  saveRuntimeContract,
  saveRuntimeHookBinding,
  saveRuntimeTaskType,
  saveGovernance,
  saveScene,
  saveSkill,
  saveSystemResourceSource,
  saveTool,
  syncSystemResources,
  testProviderModel,
  updateModelProvider,
  updateProviderModel,
  uploadSkillPackageBundle,
  uploadSkillPackageFiles
} from "./api";
import type {
  BootstrapPayload,
  CompiledAssetsPackageManifest,
  ConfigVersionDetail,
  ControlPlaneAuthStatus,
  GovernanceConfig,
  ModelTestResult,
  ProviderDefinition,
  ProviderInput,
  ProviderModelInput,
  ProviderModelRecord,
  RuntimeCheckpointReadout,
  RuntimeContractFoundation,
  RuntimeContractUpsertInput,
  RuntimeHookBindingUpsertInput,
  RuntimeLifecycleEvent,
  RuntimeProjectionCandidate,
  RuntimeRun,
  RuntimeStep,
  RuntimeTaskTypeUpsertInput,
  RuntimeTrace,
  RuntimeValidationRunResponse,
  RuntimeUsage,
  SceneConfig,
  SkillConfig,
  SkillItem,
  SkillPackageDetail,
  SkillPackageFilesInput,
  SkillPackageMetadata,
  SystemResourceCompileResult,
  SystemResourceAuditEntry,
  SystemResourceCreateInput,
  SystemResourceDebugPayload,
  SystemResourceDetail,
  SystemResourceDraft,
  SystemResourceMetadataPatch,
  SystemResourceParseResult,
  SystemResourcePipeline,
  SystemResourceSource,
  SystemResourceVersionDetail,
  SystemResourceVersionSummary,
  ToolGovernanceDecision,
  ToolGovernancePolicy,
  ToolConfig,
  ValidationMCPInvocationResponse,
  ValidationMCPServerInfo,
  ValidationMCPToolSchema
} from "./types";

const SwaggerUI = lazy(async () => {
  await import("swagger-ui-react/swagger-ui.css");
  return import("swagger-ui-react");
});

type TabKey = "overview" | "release-readiness" | "scenes" | "skills" | "tools" | "system-resources" | "system-validation" | "models" | "governance" | "versions" | "api-debug" | "swagger";
type AuthPhase = "loading" | "ready" | "unauthenticated";

type ProviderDraft = {
  id?: string;
  name: string;
  base_url: string;
  protocol: string;
  api_key: string;
  request_timeout_seconds: number;
  headers_text: string;
  enabled: boolean;
};

type ModelDraft = {
  id?: string;
  model_id: string;
  display_name: string;
  enabled: boolean;
  is_default: boolean;
  is_fallback: boolean;
};

type OpenAPIEndpoint = {
  key: string;
  method: string;
  path: string;
  summary: string;
};

type APIDebugPreset = {
  endpointKey: string;
  pathParamsText?: string;
  queryText?: string;
  bodyText?: string;
};

type APIDebugQuickPreset = {
  label: string;
  detail: string;
  preset: APIDebugPreset;
  available: boolean;
};

type StreamTranscriptItem = {
  kind: "assistant" | "system" | "error";
  title: string;
  text: string;
  status?: string;
};

type SystemValidationCheck = {
  title: string;
  status: "ok" | "warning" | "error";
  detail: string;
};

type TextComparison = {
  hasBoth: boolean;
  same: boolean;
  beforeCharacters: number;
  afterCharacters: number;
  beforeLines: number;
  afterLines: number;
};

type SystemResourceFolderGroup = {
  key: string;
  label: string;
  description: string;
  assetType?: string;
  createLabel?: string;
  items: SystemResourceDetail[];
};

type SystemResourceCreateMode = {
  assetType: string;
  title: string;
  description: string;
  createLabel: string;
  sourceFolder: string;
  defaultSlug: string;
};

type ReleaseReadinessStatus = "ready" | "warning" | "blocked";

type ReleaseReadinessCheck = {
  title: string;
  status: ReleaseReadinessStatus;
  detail: string;
  actionLabel: string;
  tab: TabKey;
};

const tabs: { key: TabKey; label: string; description: string }[] = [
  { key: "overview", label: "概览", description: "运行态指标与 truth dir 状态" },
  { key: "release-readiness", label: "Release Readiness", description: "v2.0.0 成品门禁、阻塞项和下一步入口" },
  { key: "scenes", label: "场景", description: "编辑场景匹配、默认技能和建议问题" },
  { key: "skills", label: "Skills", description: "维护技能指导、工具引用和开关" },
  { key: "tools", label: "Tools", description: "治理工具契约、作用域和确认策略" },
  { key: "system-resources", label: "System Resources", description: "管理 system truth 主源、编译和审计" },
  { key: "system-validation", label: "System Validation", description: "验证 system truth 一致性、试运行和优化对比" },
  { key: "models", label: "模型", description: "配置模型供应商、模型记录和可用性" },
  { key: "governance", label: "治理策略", description: "调整运行时治理开关和规划阈值" },
  { key: "versions", label: "版本回滚", description: "查看配置快照并执行回滚" },
  { key: "api-debug", label: "接口调试", description: "从 OpenAPI 快速构造和发送请求" },
  { key: "swagger", label: "Swagger", description: "按需加载完整接口文档" }
];

const emptyProviderDraft: ProviderDraft = {
  name: "",
  base_url: "",
  protocol: "openai_compatible",
  api_key: "",
  request_timeout_seconds: 45,
  headers_text: "",
  enabled: true
};

const emptyModelDraft: ModelDraft = {
  model_id: "",
  display_name: "",
  enabled: true,
  is_default: false,
  is_fallback: false
};

const emptySystemResourceDraft: SystemResourceDraft = {
  asset_id: "",
  asset_type: "policy_rule",
  asset_name: "",
  scope: "system",
  source_kind: "control_plane_upload",
  read_only: false,
  metadata_text: "{}",
  source_content: "",
  message: ""
};

export default function App() {
  const [activeTab, setActiveTab] = useState<TabKey>("overview");
  const activeTabInfo = tabs.find((tab) => tab.key === activeTab) ?? tabs[0];
  const [authPhase, setAuthPhase] = useState<AuthPhase>("loading");
  const [authStatus, setAuthStatus] = useState<ControlPlaneAuthStatus | null>(null);
  const [loginToken, setLoginToken] = useState("");
  const [data, setData] = useState<BootstrapPayload | null>(null);
  const [systemResources, setSystemResources] = useState<SystemResourceDetail[]>([]);
  const [systemResourcesLoaded, setSystemResourcesLoaded] = useState(false);
  const [providers, setProviders] = useState<ProviderDefinition[]>([]);
  const [providersLoaded, setProvidersLoaded] = useState(false);
  const [apiEndpoints, setAPIEndpoints] = useState<OpenAPIEndpoint[]>([]);
  const [apiSpecLoaded, setAPISpecLoaded] = useState(false);
  const [apiDebugPreset, setAPIDebugPreset] = useState<APIDebugPreset | null>(null);
  const [error, setError] = useState<string>("");
  const [status, setStatus] = useState<string>("正在加载控制面数据");
  const [versionDetail, setVersionDetail] = useState<ConfigVersionDetail | null>(null);

  async function refreshAuthStatus(nextStatus?: string) {
    const payload = await loadControlPlaneAuthStatus();
    setAuthStatus(payload);
    setAuthPhase(payload.authenticated ? "ready" : "unauthenticated");
    setError("");
    if (nextStatus) {
      setStatus(nextStatus);
    }
    return payload;
  }

  async function refreshBootstrap(nextStatus?: string) {
    const payload = await loadBootstrap();
    setData(payload);
    setError("");
    setStatus(nextStatus ?? "控制面数据已同步");
    return payload;
  }

  async function refreshProviders(nextStatus?: string) {
    const response = await loadModelProviders();
    setProviders(response.items);
    setProvidersLoaded(true);
    if (nextStatus) {
      setStatus(nextStatus);
      setError("");
    }
    return response.items;
  }

  async function refreshAPISpec(nextStatus?: string) {
    const spec = await loadOpenAPISpec();
    const paths = typeof spec.paths === "object" && spec.paths ? (spec.paths as Record<string, Record<string, { summary?: string; description?: string }>>) : {};
    const items = Object.entries(paths)
      .flatMap(([path, methods]) =>
        Object.entries(methods).map(([method, meta]) => ({
          key: `${method.toUpperCase()} ${path}`,
          method: method.toUpperCase(),
          path,
          summary: meta.summary ?? meta.description ?? ""
        }))
      )
      .sort((a, b) => a.path.localeCompare(b.path) || a.method.localeCompare(b.method));
    setAPIEndpoints(items);
    setAPISpecLoaded(true);
    if (nextStatus) {
      setStatus(nextStatus);
      setError("");
    }
    return items;
  }

  async function refreshSystemResources(nextStatus?: string) {
    const response = await loadSystemResources();
    setSystemResources(response.items);
    setSystemResourcesLoaded(true);
    if (nextStatus) {
      setStatus(nextStatus);
      setError("");
    }
    return response.items;
  }

  useEffect(() => {
    startTransition(() => {
      refreshAuthStatus().then((auth) => {
        if (!auth.authenticated) {
          setData(null);
          setStatus("请先登录控制面");
          return;
        }
        return refreshBootstrap();
      }).catch((cause: Error) => {
        setError(cause.message);
        setAuthPhase("unauthenticated");
        setStatus("控制面初始化失败");
      });
    });
  }, []);

  useEffect(() => {
    const needsProviders = activeTab === "overview" || activeTab === "release-readiness" || activeTab === "models";
    if (!needsProviders || providersLoaded || authPhase !== "ready") {
      return;
    }
    startTransition(() => {
      refreshProviders("模型治理数据已同步").catch((cause: Error) => {
        setError(cause.message);
        setStatus("模型治理数据加载失败");
      });
    });
  }, [activeTab, providersLoaded, authPhase]);

  useEffect(() => {
    const needsAPISpec = activeTab === "api-debug" || activeTab === "system-validation" || activeTab === "release-readiness";
    if (!needsAPISpec || apiSpecLoaded) {
      return;
    }
    startTransition(() => {
      refreshAPISpec("接口文档已同步").catch((cause: Error) => {
        setError(cause.message);
        setStatus("接口文档加载失败");
      });
    });
  }, [activeTab, apiSpecLoaded]);

  useEffect(() => {
    const needsSystemResources = activeTab === "overview" || activeTab === "release-readiness" || activeTab === "system-resources" || activeTab === "system-validation";
    if (!needsSystemResources || systemResourcesLoaded || authPhase !== "ready") {
      return;
    }
    startTransition(() => {
      refreshSystemResources("system resources 已同步").catch((cause: Error) => {
        setError(cause.message);
        setStatus("system resources 加载失败");
      });
    });
  }, [activeTab, systemResourcesLoaded, authPhase]);

  async function handleSceneSave(scene: SceneConfig) {
    await saveScene(scene);
    await refreshBootstrap(`场景 ${scene.id} 已保存`);
  }

  async function handleSkillSave(skill: SkillConfig) {
    await saveSkill(skill);
    await refreshBootstrap(`Skill ${skill.name} 已保存`);
  }

  async function handleToolSave(tool: ToolConfig) {
    await saveTool(tool);
    await refreshBootstrap(`Tool ${tool.name} 已保存`);
  }

  async function handleGovernanceSave(governance: GovernanceConfig) {
    await saveGovernance(governance);
    await refreshBootstrap("治理策略已保存");
  }

  async function handleVersionSelect(versionID: string) {
    const detail = await loadConfigVersion(versionID);
    setVersionDetail(detail);
    setStatus(`已加载版本 ${versionID}`);
  }

  async function handleVersionRollback(versionID: string) {
    const detail = await rollbackConfigVersion(versionID);
    setVersionDetail(detail);
    await refreshBootstrap(`已回滚到版本 ${versionID}`);
  }

  async function handleLogin() {
    const auth = await loginControlPlane({ token: loginToken.trim() });
    setAuthStatus(auth);
    setAuthPhase(auth.authenticated ? "ready" : "unauthenticated");
    if (!auth.authenticated) {
      setError("控制面登录失败");
      setStatus("控制面未认证");
      return;
    }
    setLoginToken("");
    setSystemResourcesLoaded(false);
    setProvidersLoaded(false);
    setAPISpecLoaded(false);
    await refreshBootstrap("控制面认证成功");
  }

  async function handleLogout() {
    const auth = await logoutControlPlane();
    setAuthStatus(auth);
    setAuthPhase("unauthenticated");
    setData(null);
    setSystemResources([]);
    setSystemResourcesLoaded(false);
    setProviders([]);
    setProvidersLoaded(false);
    setAPIEndpoints([]);
    setAPISpecLoaded(false);
    setVersionDetail(null);
    setStatus("已退出控制面");
    setError("");
  }

  async function handleProviderSave(draft: ProviderDraft) {
    const payload: ProviderInput = {
      name: draft.name.trim(),
      base_url: draft.base_url.trim(),
      protocol: draft.protocol.trim(),
      api_key: draft.api_key.trim(),
      request_timeout_seconds: draft.request_timeout_seconds,
      headers: parseHeadersText(draft.headers_text),
      enabled: draft.enabled
    };
    if (draft.id) {
      await updateModelProvider(draft.id, payload);
      await refreshProviders(`Provider ${draft.name} 已更新`);
      return;
    }
    await createModelProvider(payload);
    await refreshProviders(`Provider ${draft.name} 已创建`);
  }

  async function handleProviderToggle(providerID: string, enabled: boolean) {
    await patchModelProvider(providerID, { enabled });
    await refreshProviders("Provider 开关已更新");
  }

  async function handleProviderDelete(providerID: string) {
    await deleteModelProvider(providerID);
    await refreshProviders("Provider 已删除");
  }

  async function handleModelSave(providerID: string, draft: ModelDraft) {
    const payload: ProviderModelInput = {
      model_id: draft.model_id.trim(),
      display_name: draft.display_name.trim(),
      enabled: draft.enabled,
      is_default: draft.is_default,
      is_fallback: draft.is_fallback
    };
    if (draft.id) {
      await updateProviderModel(providerID, draft.id, payload);
      await refreshProviders(`模型 ${draft.display_name} 已更新`);
      return;
    }
    await createProviderModel(providerID, payload);
    await refreshProviders(`模型 ${draft.display_name} 已创建`);
  }

  async function handleModelToggle(providerID: string, recordID: string, patch: Partial<Pick<ModelDraft, "enabled" | "is_default" | "is_fallback">>) {
    await patchProviderModel(providerID, recordID, patch);
    await refreshProviders("模型开关已更新");
  }

  async function handleModelDelete(providerID: string, recordID: string) {
    await deleteProviderModel(providerID, recordID);
    await refreshProviders("模型已删除");
  }

  async function handleModelTest(providerID: string, recordID: string) {
    const result = await testProviderModel(providerID, recordID);
    const summary = result.available
      ? `模型 ${result.display_name} 测试成功，耗时 ${result.duration_ms}ms`
      : `模型 ${result.display_name} 测试失败：${result.error ?? "unknown error"}`;
    setStatus(summary);
    setError(result.available ? "" : result.error ?? "model test failed");
    return result;
  }

  async function handleOpenAPIDebug(preset: APIDebugPreset) {
    if (!apiSpecLoaded) {
      await refreshAPISpec("接口文档已同步");
    }
    setAPIDebugPreset(preset);
    setActiveTab("api-debug");
    setStatus(`已将 payload 带入 ${preset.endpointKey}`);
    setError("");
  }

  return (
    <div className="app-shell">
      <aside className="side-nav">
        <div className="brand-block">
          <p className="eyebrow">Athena</p>
          <h1>Control Plane</h1>
          <p className="muted">场景、skill、tool、模型治理、策略、配置版本与 API 文档统一入口。</p>
        </div>
        <div className="status-card">
          <span className="status-label">认证</span>
          <strong>{authPhase === "loading" ? "检查中" : authPhase === "ready" ? "已登录" : "未登录"}</strong>
          {authStatus?.truth_dir?.path ? <span className="muted">truth dir: {authStatus.truth_dir.path}</span> : null}
          {authStatus?.truth_dir?.version ? <span className="muted">truth version: {authStatus.truth_dir.version}</span> : null}
          {authPhase === "ready" ? (
            <button className="secondary-button" onClick={() => handleLogout()} type="button">
              退出登录
            </button>
          ) : null}
        </div>
        <nav className="nav-list">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              aria-current={tab.key === activeTab ? "page" : undefined}
              className={tab.key === activeTab ? "nav-item active" : "nav-item"}
              data-testid={`nav-${tab.key}`}
              onClick={() => setActiveTab(tab.key)}
              type="button"
            >
              <strong>{tab.label}</strong>
              <span>{tab.description}</span>
            </button>
          ))}
        </nav>
        <div className="status-card">
          <span className="status-label">状态</span>
          <strong>{status}</strong>
          {error ? <p className="error-text">{error}</p> : null}
        </div>
      </aside>

      <main className="content-pane">
        <header className="content-header">
          <div>
            <p className="eyebrow">Control Surface</p>
            <h2>{activeTabInfo.label}</h2>
            <p className="muted">{activeTabInfo.description}</p>
          </div>
          <div className="sync-pill">
            <span>Runtime</span>
            <strong>{authPhase === "ready" ? "online" : authPhase}</strong>
          </div>
        </header>
        {authPhase === "loading" ? <section className="panel">正在检查控制面认证状态…</section> : null}
        {authPhase === "unauthenticated" ? (
          <LoginPanel
            error={error}
            loginToken={loginToken}
            lockState={authStatus?.lock_state}
            remainingAttempts={authStatus?.remaining_attempts}
            onLogin={() => handleLogin()}
            onTokenChange={setLoginToken}
          />
        ) : null}
        {authPhase !== "ready" ? null : (
          <>
        {data === null ? <section className="panel">正在初始化控制面…</section> : null}
        {data && activeTab === "overview" ? <OverviewPanel data={data} providers={providers} truthDir={authStatus?.truth_dir} systemResources={systemResources} /> : null}
        {data && activeTab === "release-readiness" ? (
          <ReleaseReadinessPanel
            apiEndpoints={apiEndpoints}
            data={data}
            providers={providers}
            systemResources={systemResources}
            truthDir={authStatus?.truth_dir}
            onSelectTab={setActiveTab}
            onStatus={setStatus}
            onError={setError}
          />
        ) : null}
        {data && activeTab === "scenes" ? <ScenePanel items={data.scenes} onSave={handleSceneSave} /> : null}
        {data && activeTab === "skills" ? (
          <SkillPanel
            bootstrapItems={data.skills}
            onBootstrapSkillSave={handleSkillSave}
            onRefreshBootstrap={refreshBootstrap}
            onStatus={setStatus}
            onError={setError}
          />
        ) : null}
        {data && activeTab === "tools" ? <ToolPanel items={data.tools} onSave={handleToolSave} /> : null}
        {data && activeTab === "system-resources" ? (
          <SystemResourcePanel
            items={systemResources}
            truthDir={authStatus?.truth_dir}
            onOpenAPIDebug={handleOpenAPIDebug}
            onRefresh={async (message?: string) => {
              await refreshSystemResources(message);
              await refreshBootstrap();
            }}
            onStatus={setStatus}
            onError={setError}
          />
        ) : null}
        {data && activeTab === "system-validation" ? (
          <SystemValidationPanel
            data={data}
            items={systemResources}
            truthDir={authStatus?.truth_dir}
            apiEndpoints={apiEndpoints}
            onOpenAPIDebug={handleOpenAPIDebug}
            onRefresh={async (message?: string) => {
              await refreshSystemResources(message);
              await refreshBootstrap();
            }}
            onStatus={setStatus}
            onError={setError}
          />
        ) : null}
        {data && activeTab === "models" ? (
          <ModelPanel
            items={providers}
            onProviderSave={handleProviderSave}
            onProviderToggle={handleProviderToggle}
            onProviderDelete={handleProviderDelete}
            onModelSave={handleModelSave}
            onModelToggle={handleModelToggle}
            onModelDelete={handleModelDelete}
            onModelTest={handleModelTest}
          />
        ) : null}
        {data && activeTab === "governance" ? <GovernancePanel value={data.governance ?? data.runtime} onSave={handleGovernanceSave} /> : null}
        {data && activeTab === "versions" ? (
          <VersionPanel
            items={data.config_versions}
            detail={versionDetail}
            onSelect={handleVersionSelect}
            onRollback={handleVersionRollback}
          />
        ) : null}
        {data && activeTab === "api-debug" ? (
          <APIDebugPanel
            items={apiEndpoints}
            preset={apiDebugPreset}
            onPresetConsumed={() => setAPIDebugPreset(null)}
            onReload={async () => { await refreshAPISpec("接口文档已刷新"); }}
          />
        ) : null}
        {data && activeTab === "swagger" ? <SwaggerPanel specURL={data.swagger_spec_url} /> : null}
          </>
        )}
      </main>
    </div>
  );
}

function OverviewPanel({
  data,
  providers,
  truthDir,
  systemResources
}: {
  data: BootstrapPayload;
  providers: ProviderDefinition[];
  truthDir?: { path?: string; version?: string } | null;
  systemResources: SystemResourceDetail[];
}) {
  const sceneCount = data.scenes?.length ?? 0;
  const skillCount = data.skills?.length ?? 0;
  const toolCount = data.tools?.length ?? 0;
  const bootstrapSystemResourceCount = data.system_resources?.length ?? 0;
  const modelCount = providers.reduce((total, item) => total + (item.models?.length ?? 0), 0);
  return (
    <section className="panel overview-grid">
      <div className="metric-card">
        <span>场景</span>
        <strong>{sceneCount}</strong>
      </div>
      <div className="metric-card">
        <span>Skills</span>
        <strong>{skillCount}</strong>
      </div>
      <div className="metric-card">
        <span>Tools</span>
        <strong>{toolCount}</strong>
      </div>
      <div className="metric-card">
        <span>Provider / 模型</span>
        <strong>
          {providers.length} / {modelCount}
        </strong>
      </div>
      <div className="metric-card">
        <span>System Resources</span>
        <strong>{systemResources.length || bootstrapSystemResourceCount}</strong>
      </div>
      <div className="metric-card">
        <span>Truth Dir</span>
        <strong>{truthDir?.version || "unknown"}</strong>
        <span className="muted">{truthDir?.path || "未暴露路径"}</span>
      </div>
    </section>
  );
}

function ReleaseReadinessPanel({
  data,
  providers,
  systemResources,
  apiEndpoints,
  truthDir,
  onSelectTab,
  onStatus,
  onError
}: {
  data: BootstrapPayload;
  providers: ProviderDefinition[];
  systemResources: SystemResourceDetail[];
  apiEndpoints: OpenAPIEndpoint[];
  truthDir?: { path?: string; version?: string } | null;
  onSelectTab: (tab: TabKey) => void;
  onStatus: (message: string) => void;
  onError: (message: string) => void;
}) {
  const [validationRunning, setValidationRunning] = useState(false);
  const [validationResult, setValidationResult] = useState<RuntimeValidationRunResponse | null>(null);
  const [validationError, setValidationError] = useState("");
  const checks = buildReleaseReadinessChecks(data, providers, systemResources, apiEndpoints, truthDir);
  const readyCount = checks.filter((item) => item.status === "ready").length;
  const warningCount = checks.filter((item) => item.status === "warning").length;
  const blockedCount = checks.filter((item) => item.status === "blocked").length;
  const readinessPercent = Math.round((readyCount / Math.max(checks.length, 1)) * 100);
  const providerModelCount = providers.reduce((total, item) => total + (item.models?.length ?? 0), 0);
  const endpointCount = apiEndpoints.length;
  const resourceCount = systemResources.length || data.system_resources?.length || 0;

  async function runProductValidation() {
    setValidationRunning(true);
    setValidationError("");
    try {
      const result = await createRuntimeValidationRun({
        workspace_id: "release-readiness",
        scene: "release_readiness",
        prompt: "Run the v2.0.0 product-complete release readiness validation path.",
        metadata: {
          ui_surface: "release_readiness",
          release_gate: "v2.0.0",
          truth_dir_version: truthDir?.version || ""
        }
      });
      setValidationResult(result);
      onError("");
      onStatus("Release Readiness 产品验证已完成 runtime、MCP、sandbox 和 persistence 链路");
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : String(cause);
      setValidationError(message);
      onError(message);
      onStatus("Release Readiness 产品验证失败");
    } finally {
      setValidationRunning(false);
    }
  }

  return (
    <section className="panel editor-pane release-readiness-panel">
      <section className="section-card release-hero">
        <div>
          <p className="eyebrow">v2.0.0 release candidate gate</p>
          <h2>成品完整度</h2>
          <p className="section-help">把后台页面、runtime、system truth、模型配置、验证链路和 API 文档合并成一个 release gate 视图。</p>
        </div>
        <div className="release-score">
          <span>{readinessPercent}%</span>
          <strong>{blockedCount === 0 ? "集成分支可继续深化" : "仍有阻塞项"}</strong>
        </div>
      </section>

      <section className="section-card release-validation">
        <div className="section-header">
          <div>
            <h2>Product Runtime Path</h2>
            <p className="section-help">从成品门禁页直接触发 deterministic validation，串起 runtime persistence、Validation MCP、tool governance 和 external_sandbox_ref。</p>
          </div>
          <button className="primary-button" disabled={validationRunning || !hasEndpoint(apiEndpoints, "POST /api/control-plane/runtime/validation-runs")} onClick={runProductValidation} type="button">
            {validationRunning ? "验证中…" : "运行产品验证"}
          </button>
        </div>
        {validationError ? (
          <div className="info-card error">
            <span className="status-label">product validation</span>
            <strong>failed</strong>
            <span>{validationError}</span>
          </div>
        ) : null}
        {validationResult ? (
          <div className="system-meta-grid">
            <div className={runtimeStatusClass(validationResult.run.status)}>
              <span className="status-label">Runtime Run</span>
              <strong>{validationResult.run.status}</strong>
              <span>{validationResult.run.id}</span>
            </div>
            <div className={validationResult.step.status === "completed" ? "info-card success" : "info-card"}>
              <span className="status-label">Graph Step</span>
              <strong>{validationResult.step.name || validationResult.step.step_type || validationResult.step.id}</strong>
              <span>{validationResult.step.status}</span>
            </div>
            <div className={toolGovernanceDecisionClass(validationResult.validation_mcp.governance_decision.decision)}>
              <span className="status-label">Validation MCP</span>
              <strong>{validationResult.validation_mcp.result.tool_name}</strong>
              <span>{validationResult.validation_mcp.governance_decision.decision}</span>
            </div>
            <div className={validationResult.sandbox.sandbox_ref.status === "completed" ? "info-card success" : "info-card"}>
              <span className="status-label">Sandbox</span>
              <strong>{validationResult.sandbox.sandbox_ref.mode}</strong>
              <span>{validationResult.sandbox.audit_summary.state_integrity}</span>
            </div>
          </div>
        ) : (
          <div className="info-card">
            <span className="status-label">product validation</span>
            <strong>not run yet</strong>
            <span>运行后会展示本次 run、graph step、Validation MCP decision 和 sandbox audit。</span>
          </div>
        )}
      </section>

      <div className="overview-grid">
        <div className={blockedCount === 0 ? "metric-card release-metric success" : "metric-card release-metric error"}>
          <span>Blockers</span>
          <strong>{blockedCount}</strong>
        </div>
        <div className="metric-card release-metric warning">
          <span>Warnings</span>
          <strong>{warningCount}</strong>
        </div>
        <div className="metric-card release-metric success">
          <span>Ready Gates</span>
          <strong>{readyCount}</strong>
        </div>
        <div className="metric-card release-metric">
          <span>Evidence Surface</span>
          <strong>{endpointCount}</strong>
          <span className="muted">OpenAPI operations</span>
        </div>
      </div>

      <section className="section-card">
        <div className="section-header">
          <div>
            <h2>Release Gate Checklist</h2>
            <p className="section-help">这些检查不会替代最终测试；它们用于暴露当前版本是否已经具备成品入口。</p>
          </div>
          <span className="muted">{checks.length} gates</span>
        </div>
        <div className="readiness-grid">
          {checks.map((check) => (
            <div className={releaseReadinessClass(check.status)} key={check.title}>
              <span className="status-label">{check.status}</span>
              <strong>{check.title}</strong>
              <span>{check.detail}</span>
              <button className="secondary-button" onClick={() => onSelectTab(check.tab)} type="button">
                {check.actionLabel}
              </button>
            </div>
          ))}
        </div>
      </section>

      <section className="section-card">
        <div className="section-header">
          <div>
            <h2>Product Surface Snapshot</h2>
            <p className="section-help">当前控制面已经加载到的真实数据，不使用 mock。</p>
          </div>
          <span className="muted">{truthDir?.version || "unknown truth version"}</span>
        </div>
        <div className="system-meta-grid">
          <div className="info-card">
            <span className="status-label">System Resources</span>
            <strong>{resourceCount}</strong>
            <span>{summarizeAssetTypes(systemResources.length > 0 ? systemResources : data.system_resources)}</span>
          </div>
          <div className="info-card">
            <span className="status-label">Runtime / Validation APIs</span>
            <strong>{countEndpointPrefix(apiEndpoints, "/api/control-plane/runtime")}</strong>
            <span>{hasEndpoint(apiEndpoints, "POST /api/control-plane/runtime/validation-runs") ? "validation trigger available" : "validation trigger missing"}</span>
          </div>
          <div className="info-card">
            <span className="status-label">Provider / Models</span>
            <strong>{providers.length} / {providerModelCount}</strong>
            <span>{summarizeDefaultModel(providers)}</span>
          </div>
          <div className="info-card">
            <span className="status-label">Versions</span>
            <strong>{data.config_versions?.length ?? 0}</strong>
            <span>{data.config_versions?.[0]?.version_id || "no version snapshot loaded"}</span>
          </div>
        </div>
      </section>
    </section>
  );
}

function buildReleaseReadinessChecks(
  data: BootstrapPayload,
  providers: ProviderDefinition[],
  systemResources: SystemResourceDetail[],
  apiEndpoints: OpenAPIEndpoint[],
  truthDir?: { path?: string; version?: string } | null
): ReleaseReadinessCheck[] {
  const resourceItems = systemResources.length > 0 ? systemResources : data.system_resources ?? [];
  const resourceErrors = systemResources.filter((item) => item.compile_result?.status === "error" || (item.pipeline?.errors?.length ?? 0) > 0).length;
  const enabledProviders = providers.filter((item) => item.enabled);
  const defaultModels = providers.flatMap((item) => item.models ?? []).filter((item) => item.enabled && item.is_default);
  const runtimeEndpoints = [
    "POST /api/control-plane/runtime/validation-runs",
    "GET /api/control-plane/runtime/runs",
    "GET /api/control-plane/runtime/contracts/foundation",
    "GET /api/control-plane/runtime/runs/{runID}",
    "GET /api/control-plane/runtime/runs/{runID}/steps",
    "GET /api/control-plane/runtime/runs/{runID}/lifecycle",
    "GET /api/control-plane/runtime/runs/{runID}/traces",
    "GET /api/control-plane/runtime/runs/{runID}/usage",
    "GET /api/control-plane/runtime/runs/{runID}/projections",
    "GET /api/control-plane/runtime/runs/{runID}/checkpoints"
  ];
  const governanceEndpoints = [
    "GET /api/control-plane/tool-governance/policy",
    "GET /api/control-plane/tool-governance/decisions",
    "POST /api/control-plane/tool-governance/evaluate"
  ];
  const validationEndpoints = [
    "GET /api/control-plane/validation-mcp/server",
    "GET /api/control-plane/validation-mcp/tools",
    "POST /api/control-plane/validation-mcp/invocations"
  ];

  return [
    {
      title: "Control Plane 基础入口",
      status: data.scenes.length > 0 && data.skills.length > 0 ? data.tools.length > 0 ? "ready" : "warning" : "blocked",
      detail: `${data.scenes.length} scenes · ${data.skills.length} skills · ${data.tools.length} tools`,
      actionLabel: "查看概览",
      tab: "overview"
    },
    {
      title: "System Truth 主源",
      status: resourceItems.length === 0 ? "blocked" : resourceErrors > 0 ? "warning" : "ready",
      detail: resourceItems.length === 0 ? "未加载 system resources" : `${resourceItems.length} resources · ${resourceErrors} compile/pipeline errors`,
      actionLabel: "管理资源",
      tab: "system-resources"
    },
    {
      title: "Runtime 持久化与验证入口",
      status: runtimeEndpoints.every((item) => hasEndpoint(apiEndpoints, item)) ? "ready" : "blocked",
      detail: `${runtimeEndpoints.filter((item) => hasEndpoint(apiEndpoints, item)).length}/${runtimeEndpoints.length} runtime endpoints available`,
      actionLabel: "运行验证",
      tab: "system-validation"
    },
    {
      title: "Tool Governance",
      status: governanceEndpoints.every((item) => hasEndpoint(apiEndpoints, item)) ? "ready" : "blocked",
      detail: `${governanceEndpoints.filter((item) => hasEndpoint(apiEndpoints, item)).length}/${governanceEndpoints.length} governance endpoints available`,
      actionLabel: "验证治理",
      tab: "system-validation"
    },
    {
      title: "Validation MCP / Sandbox",
      status: validationEndpoints.every((item) => hasEndpoint(apiEndpoints, item)) && hasEndpoint(apiEndpoints, "POST /api/control-plane/runtime/validation-runs") ? "ready" : "blocked",
      detail: `${validationEndpoints.filter((item) => hasEndpoint(apiEndpoints, item)).length}/${validationEndpoints.length} MCP endpoints · sandbox through validation run`,
      actionLabel: "查看验证链路",
      tab: "system-validation"
    },
    {
      title: "Provider / Model 配置",
      status: providers.length === 0 ? "warning" : enabledProviders.length > 0 && defaultModels.length > 0 ? "ready" : "warning",
      detail: providers.length === 0 ? "尚未配置模型供应商，deterministic validation 可跑但成品体验未闭合" : `${enabledProviders.length} enabled providers · ${defaultModels.length} default models`,
      actionLabel: "配置模型",
      tab: "models"
    },
    {
      title: "API 文档与调试面",
      status: apiEndpoints.length > 0 && Boolean(data.swagger_spec_url) ? "ready" : "blocked",
      detail: apiEndpoints.length > 0 ? `${apiEndpoints.length} OpenAPI operations · Swagger available` : "OpenAPI spec 未加载",
      actionLabel: "接口调试",
      tab: "api-debug"
    },
    {
      title: "版本与回滚证据",
      status: (data.config_versions?.length ?? 0) > 0 ? "ready" : "warning",
      detail: (data.config_versions?.length ?? 0) > 0 ? `${data.config_versions.length} config snapshots · latest ${data.config_versions[0]?.version_id}` : "尚无 config version snapshot",
      actionLabel: "查看版本",
      tab: "versions"
    },
    {
      title: "Truth Dir 可追踪性",
      status: truthDir?.version ? "ready" : "warning",
      detail: truthDir?.version ? `${truthDir.version} · ${truthDir.path || "path hidden"}` : "truth dir version 未暴露",
      actionLabel: "查看资源",
      tab: "system-resources"
    }
  ];
}

function hasEndpoint(apiEndpoints: OpenAPIEndpoint[], key: string) {
  return apiEndpoints.some((item) => item.key === key);
}

function countEndpointPrefix(apiEndpoints: OpenAPIEndpoint[], prefix: string) {
  return apiEndpoints.filter((item) => item.path.startsWith(prefix)).length;
}

function summarizeAssetTypes(items: Array<{ asset_type?: string }>) {
  if (items.length === 0) {
    return "no system resources loaded";
  }
  const counts = items.reduce<Record<string, number>>((acc, item) => {
    const key = item.asset_type || "unknown";
    acc[key] = (acc[key] ?? 0) + 1;
    return acc;
  }, {});
  return Object.entries(counts)
    .sort((left, right) => right[1] - left[1] || left[0].localeCompare(right[0]))
    .slice(0, 4)
    .map(([key, count]) => `${key}: ${count}`)
    .join(" · ");
}

function summarizeDefaultModel(providers: ProviderDefinition[]) {
  const defaultModels = providers.flatMap((item) => item.models ?? []).filter((item) => item.enabled && item.is_default);
  if (providers.length === 0) {
    return "no providers configured";
  }
  return defaultModels.map((item) => item.display_name || item.model_id).join(", ") || "no default model";
}

function releaseReadinessClass(status: ReleaseReadinessStatus) {
  switch (status) {
    case "ready":
      return "info-card readiness-card success";
    case "blocked":
      return "info-card readiness-card error";
    default:
      return "info-card readiness-card warning";
  }
}

function ScenePanel({ items, onSave }: { items: SceneConfig[]; onSave: (scene: SceneConfig) => Promise<void> }) {
  const [selectedID, setSelectedID] = useState(items[0]?.id ?? "");
  const selected = items.find((item) => item.id === selectedID) ?? items[0];
  const [draft, setDraft] = useState<SceneConfig | null>(selected ?? null);

  useEffect(() => {
    setSelectedID(items[0]?.id ?? "");
  }, [items]);

  useEffect(() => {
    setDraft(selected ?? null);
  }, [selected]);

  if (!draft) {
    return <section className="panel">没有可编辑的场景。</section>;
  }

  return (
    <section className="panel split-panel">
      <div className="list-pane">
        {items.map((item) => (
          <button
            key={item.id}
            className={item.id === selectedID ? "list-item active" : "list-item"}
            onClick={() => setSelectedID(item.id)}
            type="button"
          >
            <strong>{item.id}</strong>
            <span>{item.description}</span>
          </button>
        ))}
      </div>
      <div className="editor-pane">
        <label>
          描述
          <textarea name="scene-description" value={draft.description ?? ""} onChange={(event) => setDraft({ ...draft, description: event.target.value })} />
        </label>
        <label>
          Keywords
          <textarea
            name="scene-keywords"
            value={(draft.keywords ?? []).join("\n")}
            onChange={(event) => setDraft({ ...draft, keywords: event.target.value.split("\n").map((item) => item.trim()).filter(Boolean) })}
          />
        </label>
        <label>
          Default Skills
          <textarea
            name="scene-default-skills"
            value={(draft.default_skills ?? []).join("\n")}
            onChange={(event) => setDraft({ ...draft, default_skills: event.target.value.split("\n").map((item) => item.trim()).filter(Boolean) })}
          />
        </label>
        <label>
          Suggested Questions
          <textarea
            name="scene-suggested-questions"
            value={(draft.suggested_questions ?? []).join("\n")}
            onChange={(event) => setDraft({ ...draft, suggested_questions: event.target.value.split("\n").map((item) => item.trim()).filter(Boolean) })}
          />
        </label>
        <label>
          Match Score
          <input
            name="scene-match-score"
            type="number"
            value={draft.match_score ?? 0}
            onChange={(event) => setDraft({ ...draft, match_score: Number(event.target.value) })}
          />
        </label>
        <label className="inline-toggle">
          <input name="scene-enabled" checked={draft.enabled} onChange={(event) => setDraft({ ...draft, enabled: event.target.checked })} type="checkbox" />
          启用场景
        </label>
        <button className="primary-button" onClick={() => onSave(draft)} type="button">
          保存场景
        </button>
      </div>
    </section>
  );
}

function SkillPanel({
  bootstrapItems,
  onBootstrapSkillSave,
  onRefreshBootstrap,
  onStatus,
  onError
}: {
  bootstrapItems: SkillConfig[];
  onBootstrapSkillSave: (skill: SkillConfig) => Promise<void>;
  onRefreshBootstrap: (nextStatus?: string) => Promise<BootstrapPayload>;
  onStatus: (status: string) => void;
  onError: (error: string) => void;
}) {
  const [visibleSkills, setVisibleSkills] = useState<SkillItem[]>([]);
  const [packages, setPackages] = useState<SkillPackageMetadata[]>([]);
  const [revisions, setRevisions] = useState<SkillPackageMetadata[]>([]);
  const [selectedPackageID, setSelectedPackageID] = useState("");
  const [detail, setDetail] = useState<SkillPackageDetail | null>(null);
  const [selectedFilePath, setSelectedFilePath] = useState("SKILL.md");
  const [filePathDraft, setFilePathDraft] = useState("references/guide.md");
  const [newSkillName, setNewSkillName] = useState("uploaded-skill");
  const [newSkillDescription, setNewSkillDescription] = useState("Uploaded skill managed from Athena console.");
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [bootstrapSelectedName, setBootstrapSelectedName] = useState(bootstrapItems[0]?.name ?? "");
  const bootstrapSelected = bootstrapItems.find((item) => item.name === bootstrapSelectedName) ?? bootstrapItems[0];
  const [bootstrapDraft, setBootstrapDraft] = useState<SkillConfig | null>(bootstrapSelected ?? null);

  async function refreshSkillSurface(nextStatus?: string, preferredPackageID?: string) {
    const [visibleResponse, packageResponse] = await Promise.all([loadVisibleSkills(), loadSkillPackages()]);
    setVisibleSkills(visibleResponse.items);
    setPackages(packageResponse.items);
    const nextSelectedID = preferredPackageID || selectedPackageID || packageResponse.items[0]?.id || "";
    if (!nextSelectedID) {
      setSelectedPackageID("");
      setDetail(null);
      setRevisions([]);
      setSelectedFilePath("SKILL.md");
    } else {
      setSelectedPackageID(nextSelectedID);
      const [nextDetail, revisionResponse] = await Promise.all([loadSkillPackage(nextSelectedID), loadSkillPackageRevisions(nextSelectedID)]);
      setDetail(nextDetail);
      setRevisions(revisionResponse.items);
      const paths = Object.keys(nextDetail.files).sort();
      setSelectedFilePath((current) => paths.includes(current) ? current : paths[0] ?? "SKILL.md");
    }
    if (nextStatus) {
      onStatus(nextStatus);
      onError("");
    }
  }

  useEffect(() => {
    startTransition(() => {
      refreshSkillSurface("Skill 管理数据已同步").catch((cause: Error) => {
        onError(cause.message);
        onStatus("Skill 管理数据加载失败");
      });
    });
  }, []);

  useEffect(() => {
    if (!selectedPackageID) {
      return;
    }
    startTransition(() => {
      Promise.all([loadSkillPackage(selectedPackageID), loadSkillPackageRevisions(selectedPackageID)])
        .then(([nextDetail, revisionResponse]) => {
          setDetail(nextDetail);
          setRevisions(revisionResponse.items);
          const paths = Object.keys(nextDetail.files).sort();
          setSelectedFilePath((current) => paths.includes(current) ? current : paths[0] ?? "SKILL.md");
        })
        .catch((cause: Error) => {
          onError(cause.message);
          onStatus("Skill 包详情加载失败");
        });
    });
  }, [selectedPackageID]);

  useEffect(() => {
    setBootstrapSelectedName(bootstrapItems[0]?.name ?? "");
  }, [bootstrapItems]);

  useEffect(() => {
    setBootstrapDraft(bootstrapSelected ?? null);
  }, [bootstrapSelected]);

  function packageFilesPayload(files: Record<string, string>, enabled?: boolean): SkillPackageFilesInput {
    return {
      enabled,
      files
    };
  }

  async function uploadPackageWithDuplicateCheck(action: () => Promise<SkillPackageMetadata>, replaceUploaded: (existing: SkillPackageMetadata) => Promise<SkillPackageMetadata>) {
    try {
      return await action();
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : String(cause);
      const apiMessage = parseAPIErrorMessage(message);
      const duplicatePackage = /skill package name "([^"]+)" already exists/.exec(apiMessage);
      if (!duplicatePackage) {
        throw cause;
      }
      const duplicatedName = duplicatePackage[1];
      return replaceDuplicatePackage(duplicatedName, replaceUploaded, cause);
    }
  }

  async function replaceDuplicatePackage(duplicatedName: string, replaceUploaded: (existing: SkillPackageMetadata) => Promise<SkillPackageMetadata>, cause?: unknown) {
    const currentPackages = packages.length > 0 ? packages : (await loadSkillPackages()).items;
    const existing = currentPackages.find((item) => item.name === duplicatedName);
    if (!existing) {
      if (cause) {
        throw cause;
      }
      throw new Error(`Skill 包 ${duplicatedName} 已存在，但未找到可替换的上传包`);
    }
    const confirmed = window.confirm(`Skill 包 ${duplicatedName} 已存在。是否替换已上传包 ${existing.id}？`);
    if (!confirmed) {
      throw new Error(`已取消替换重复 Skill 包 ${duplicatedName}`);
    }
    return replaceUploaded(existing);
  }

  async function confirmVisibleSkillOverride(name: string) {
    const currentVisibleSkills = visibleSkills.length > 0 ? visibleSkills : (await loadVisibleSkills()).items;
    const existing = currentVisibleSkills.find((item) => item.name === name);
    if (!existing) {
      return;
    }
    const confirmed = window.confirm(`Skill ${name} 当前来自 ${existing.source}。是否创建同名上传包作为调试覆盖？`);
    if (!confirmed) {
      throw new Error(`已取消覆盖 Skill ${name}`);
    }
  }

  function updateSelectedFile(content: string) {
    if (!detail) {
      return;
    }
    setDetail({
      ...detail,
      files: {
        ...detail.files,
        [selectedFilePath]: content
      }
    });
  }

  async function withSkillAction(action: () => Promise<void>) {
    try {
      await action();
      onError("");
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : String(cause);
      onError(message);
      onStatus("Skill 操作失败");
    }
  }

  const selectedPackage = packages.find((item) => item.id === selectedPackageID) ?? packages[0];
  const filePaths = Object.keys(detail?.files ?? {}).sort();
  const selectedFileContent = detail?.files[selectedFilePath] ?? "";
  const validation = detail?.metadata.validation ?? selectedPackage?.validation;

  return (
    <section className="panel skill-management-panel">
      <div className="section-card">
        <div className="section-header">
          <div>
            <h2>Runtime 可见 Skills</h2>
            <p className="section-help">这里展示运行时实际可见的 skill，source 会区分内置、产品托管和用户上传来源。</p>
          </div>
          <button className="secondary-button" onClick={() => withSkillAction(async () => refreshSkillSurface("Skill 管理数据已刷新"))} type="button">
            刷新 Skill
          </button>
        </div>
        <div className="skill-summary-grid">
          <div className="info-card">
            <span className="status-label">visible skills</span>
            <strong>{visibleSkills.length}</strong>
            <span>当前运行时可见 skill 数量。</span>
          </div>
          <div className="info-card">
            <span className="status-label">uploaded packages</span>
            <strong>{packages.length}</strong>
            <span>用户上传并由包治理层维护的 skill 包。</span>
          </div>
          <div className="info-card">
            <span className="status-label">selected package</span>
            <strong>{selectedPackage?.name ?? "-"}</strong>
            <span>{selectedPackage ? `revision ${selectedPackage.revision}` : "暂无已上传包"}</span>
          </div>
        </div>
        <div className="skill-card-grid">
          {visibleSkills.map((item) => (
            <div className="info-card" key={`${item.source}:${item.name}`}>
              <span className="status-label">{item.source}</span>
              <strong>{item.name}</strong>
              <span>{item.description || "无描述"}</span>
              {item.tool_names?.length ? <span>tools: {item.tool_names.join(", ")}</span> : null}
            </div>
          ))}
          {visibleSkills.length === 0 ? <p className="muted">暂无运行时可见 skill。</p> : null}
        </div>
      </div>

      <div className="section-card">
        <div className="section-header">
          <div>
            <h2>上传 Skill 包</h2>
            <p className="section-help">完整 skill 包以 SKILL.md 为入口，可上传 zip，也可从页面生成一个可编辑的文本包。</p>
          </div>
        </div>
        <div className="split-panel embedded-split">
          <div className="editor-pane">
            <label>
              新 Skill 名称
              <input name="skill-package-new-name" value={newSkillName} onChange={(event) => setNewSkillName(event.target.value)} />
            </label>
            <label>
              新 Skill 描述
              <input name="skill-package-new-description" value={newSkillDescription} onChange={(event) => setNewSkillDescription(event.target.value)} />
            </label>
            <button className="primary-button" onClick={() => withSkillAction(async () => {
              const name = newSkillName.trim();
              if (!name) {
                throw new Error("Skill 名称不能为空");
              }
              const description = newSkillDescription.trim();
              const files = {
                "SKILL.md": `---\nname: ${name}\ndescription: ${description}\n---\n\n# ${name}\n\n在这里编写完整 skill guidance。`,
                "references/guide.md": "在这里补充该 skill 的参考材料。"
              };
              const duplicatedPackage = packages.find((item) => item.name === name);
              const metadata = duplicatedPackage
                ? await replaceDuplicatePackage(name, (existing) => replaceSkillPackageFiles(existing.id, { name, files }))
                : await confirmVisibleSkillOverride(name).then(() => uploadSkillPackageFiles({ name, files }));
              await refreshSkillSurface(`Skill 包 ${metadata.name} 已上传`, metadata.id);
              await onRefreshBootstrap();
            })} type="button">
              创建文本 Skill 包
            </button>
          </div>
          <div className="editor-pane">
            <label>
              上传 zip 包
              <input name="skill-package-upload" accept=".zip,application/zip" type="file" onChange={(event) => setUploadFile(event.target.files?.[0] ?? null)} />
            </label>
            <button className="primary-button" disabled={!uploadFile} onClick={() => withSkillAction(async () => {
              if (!uploadFile) {
                return;
              }
              const metadata = await uploadPackageWithDuplicateCheck(
                () => uploadSkillPackageBundle(uploadFile),
                (existing) => replaceSkillPackageBundle(existing.id, uploadFile)
              );
              setUploadFile(null);
              await refreshSkillSurface(`Skill 包 ${metadata.name} 已上传`, metadata.id);
              await onRefreshBootstrap();
            })} type="button">
              上传 Skill zip
            </button>
            <p className="muted">如果上传包与已上传包或内置 skill 重名，会先弹窗确认；确认后上传包会作为调试覆盖项生效。</p>
          </div>
        </div>
      </div>

      <div className="section-card">
        <div className="section-header">
          <div>
            <h2>已上传 Skill 包</h2>
            <p className="section-help">选择一个包后可查看完整文件、编辑 SKILL.md 和引用材料，并保存为新 revision。</p>
          </div>
          {selectedPackage ? (
            <div className="action-row">
              <label className="inline-toggle">
                <input checked={selectedPackage.enabled} onChange={(event) => withSkillAction(async () => {
                  await patchSkillPackageEnabled(selectedPackage.id, event.target.checked);
                  await refreshSkillSurface(`Skill 包 ${selectedPackage.name} 状态已更新`, selectedPackage.id);
                  await onRefreshBootstrap();
                })} type="checkbox" />
                启用上传包
              </label>
            </div>
          ) : null}
        </div>
        <div className="split-panel">
          <div className="list-pane">
            {packages.map((item) => (
              <button
                key={item.id}
                className={item.id === selectedPackageID ? "list-item active" : "list-item"}
                onClick={() => setSelectedPackageID(item.id)}
                type="button"
              >
                <strong>{item.name}</strong>
                <span>{item.id}</span>
                <span>rev {item.revision} · {item.file_count} files · {item.enabled ? "enabled" : "disabled"}</span>
                <span>{item.validation.valid ? "valid" : `invalid: ${(item.validation.errors ?? []).join(", ")}`}</span>
              </button>
            ))}
            {packages.length === 0 ? <p className="muted">暂无已上传 Skill 包。</p> : null}
          </div>
          <div className="editor-pane">
            {detail ? (
              <>
                <div className="skill-summary-grid">
                  <div className="info-card">
                    <span className="status-label">package</span>
                    <strong>{detail.metadata.name}</strong>
                    <span>{detail.metadata.id}</span>
                  </div>
                  <div className="info-card">
                    <span className="status-label">revision</span>
                    <strong>{detail.metadata.revision}</strong>
                    <span>{detail.metadata.uploaded_at || "-"}</span>
                  </div>
                  <div className="info-card">
                    <span className="status-label">validation</span>
                    <strong>{validation?.valid ? "valid" : "invalid"}</strong>
                    <span>{validation?.warnings?.length ? `warnings: ${validation.warnings.join(", ")}` : "no warnings"}</span>
                  </div>
                </div>
                {validation?.errors?.length ? <p className="error-text">{validation.errors.join("；")}</p> : null}
                <div className="split-panel embedded-split">
                  <div className="list-pane compact-list-pane">
                    <strong>包文件</strong>
                    {filePaths.map((path) => (
                      <button
                        key={path}
                        className={path === selectedFilePath ? "list-item active" : "list-item"}
                        onClick={() => setSelectedFilePath(path)}
                        type="button"
                      >
                        <strong>{path}</strong>
                        <span>{detail.files[path].length} chars</span>
                      </button>
                    ))}
                    <label>
                      新文件路径
                      <input name="skill-package-new-file-path" value={filePathDraft} onChange={(event) => setFilePathDraft(event.target.value)} />
                    </label>
                    <button className="secondary-button" onClick={() => {
                      const path = filePathDraft.trim();
                      if (!path || !detail) {
                        return;
                      }
                      setDetail({ ...detail, files: { ...detail.files, [path]: "" } });
                      setSelectedFilePath(path);
                    }} type="button">
                      添加文件
                    </button>
                  </div>
                  <div className="editor-pane">
                    <label>
                      当前文件
                      <input name="skill-package-selected-file" value={selectedFilePath} onChange={(event) => {
                        const nextPath = event.target.value.trim();
                        if (!nextPath || !detail || nextPath === selectedFilePath) {
                          return;
                        }
                        const nextFiles = { ...detail.files };
                        nextFiles[nextPath] = nextFiles[selectedFilePath] ?? "";
                        delete nextFiles[selectedFilePath];
                        setDetail({ ...detail, files: nextFiles });
                        setSelectedFilePath(nextPath);
                      }} />
                    </label>
                    <label>
                      文件内容
                      <textarea className="debug-textarea" name="skill-package-file-content" value={selectedFileContent} onChange={(event) => updateSelectedFile(event.target.value)} rows={18} />
                    </label>
                    <div className="action-row">
                      <button className="primary-button" onClick={() => withSkillAction(async () => {
                        const metadata = await replaceSkillPackageFiles(detail.metadata.id, packageFilesPayload(detail.files, detail.metadata.enabled));
                        await refreshSkillSurface(`Skill 包 ${metadata.name} 已保存为 revision ${metadata.revision}`, metadata.id);
                        await onRefreshBootstrap();
                      })} type="button">
                        保存包文件
                      </button>
                      <button className="secondary-button" disabled={selectedFilePath === "SKILL.md"} onClick={() => {
                        if (!detail || selectedFilePath === "SKILL.md") {
                          return;
                        }
                        const nextFiles = { ...detail.files };
                        delete nextFiles[selectedFilePath];
                        const paths = Object.keys(nextFiles).sort();
                        setDetail({ ...detail, files: nextFiles });
                        setSelectedFilePath(paths[0] ?? "SKILL.md");
                      }} type="button">
                        删除当前文件
                      </button>
                    </div>
                  </div>
                </div>
                <div className="section-card">
                  <div className="section-header">
                    <h3>版本历史</h3>
                  </div>
                  <div className="skill-card-grid">
                    {revisions.map((revision) => (
                      <button className="list-item" key={`${revision.id}:${revision.revision}`} onClick={() => withSkillAction(async () => {
                        const result = await rollbackSkillPackage(detail.metadata.id, revision.revision);
                        await refreshSkillSurface(`Skill 包已从 revision ${result.rolled_back_from} 回滚`, result.metadata.id);
                        await onRefreshBootstrap();
                      })} type="button">
                        <strong>revision {revision.revision}</strong>
                        <span>{revision.uploaded_at || "-"}</span>
                        <span>{revision.enabled ? "enabled" : "disabled"}</span>
                      </button>
                    ))}
                    {revisions.length === 0 ? <p className="muted">暂无历史版本。</p> : null}
                  </div>
                </div>
              </>
            ) : (
              <p className="muted">选择或上传一个 Skill 包后查看完整内容。</p>
            )}
          </div>
        </div>
      </div>

      <div className="section-card">
        <div className="section-header">
          <div>
            <h2>控制面 Skill 配置</h2>
            <p className="section-help">这部分是 bootstrap/config 中的轻量 skill 配置；完整用户 skill 请优先通过上方包管理维护。</p>
          </div>
        </div>
        {bootstrapDraft ? (
          <div className="split-panel embedded-split">
            <div className="list-pane compact-list-pane">
              {bootstrapItems.map((item) => (
                <button
                  key={item.name}
                  className={item.name === bootstrapSelectedName ? "list-item active" : "list-item"}
                  onClick={() => setBootstrapSelectedName(item.name)}
                  type="button"
                >
                  <strong>{item.name}</strong>
                  <span>{item.description}</span>
                </button>
              ))}
            </div>
            <div className="editor-pane">
              <label>
                描述
                <textarea name="skill-description" value={bootstrapDraft.description ?? ""} onChange={(event) => setBootstrapDraft({ ...bootstrapDraft, description: event.target.value })} />
              </label>
              <label>
                Guidance
                <textarea name="skill-guidance" value={bootstrapDraft.guidance ?? ""} onChange={(event) => setBootstrapDraft({ ...bootstrapDraft, guidance: event.target.value })} />
              </label>
              <label>
                Tool Names
                <textarea
                  name="skill-tool-names"
                  value={(bootstrapDraft.tool_names ?? []).join("\n")}
                  onChange={(event) => setBootstrapDraft({ ...bootstrapDraft, tool_names: event.target.value.split("\n").map((item) => item.trim()).filter(Boolean) })}
                />
              </label>
              <label className="inline-toggle">
                <input name="skill-enabled" checked={bootstrapDraft.enabled} onChange={(event) => setBootstrapDraft({ ...bootstrapDraft, enabled: event.target.checked })} type="checkbox" />
                启用 control-plane skill
              </label>
              <button className="secondary-button" onClick={() => withSkillAction(async () => {
                await onBootstrapSkillSave(bootstrapDraft);
                await onRefreshBootstrap(`Skill ${bootstrapDraft.name} 已保存`);
              })} type="button">
                保存轻量配置
              </button>
            </div>
          </div>
        ) : (
          <p className="muted">暂无控制面 skill 配置。</p>
        )}
      </div>
    </section>
  );
}

function ToolPanel({ items, onSave }: { items: ToolConfig[]; onSave: (tool: ToolConfig) => Promise<void> }) {
  const [selectedName, setSelectedName] = useState(items[0]?.name ?? "");
  const selected = items.find((item) => item.name === selectedName) ?? items[0];
  const [draft, setDraft] = useState<ToolConfig | null>(selected ?? null);

  useEffect(() => {
    setSelectedName(items[0]?.name ?? "");
  }, [items]);

  useEffect(() => {
    setDraft(selected ?? null);
  }, [selected]);

  if (!draft) {
    return <section className="panel">没有可编辑的 tool。</section>;
  }

  return (
    <section className="panel split-panel">
      <div className="list-pane">
        {items.map((item) => (
          <button
            key={item.name}
            className={item.name === selectedName ? "list-item active" : "list-item"}
            onClick={() => setSelectedName(item.name)}
            type="button"
          >
            <strong>{item.name}</strong>
            <span>{item.description}</span>
          </button>
        ))}
      </div>
      <div className="editor-pane">
        <label>
          描述
          <textarea name="tool-description" value={draft.description ?? ""} onChange={(event) => setDraft({ ...draft, description: event.target.value })} />
        </label>
        <label>
          Tool Scope
          <input name="tool-scope" value={draft.tool_scope ?? ""} onChange={(event) => setDraft({ ...draft, tool_scope: event.target.value })} />
        </label>
        <label>
          Side Effect Level
          <input name="tool-side-effect-level" value={draft.side_effect_level ?? ""} onChange={(event) => setDraft({ ...draft, side_effect_level: event.target.value })} />
        </label>
        <label>
          Input Schema Summary
          <textarea
            name="tool-input-schema-summary"
            value={draft.input_schema_summary ?? ""}
            onChange={(event) => setDraft({ ...draft, input_schema_summary: event.target.value })}
          />
        </label>
        <label>
          Output Schema Summary
          <textarea
            name="tool-output-schema-summary"
            value={draft.output_schema_summary ?? ""}
            onChange={(event) => setDraft({ ...draft, output_schema_summary: event.target.value })}
          />
        </label>
        <label className="inline-toggle">
          <input
            name="tool-requires-confirmation"
            checked={draft.requires_confirmation}
            onChange={(event) => setDraft({ ...draft, requires_confirmation: event.target.checked })}
            type="checkbox"
          />
          需要确认
        </label>
        <label className="inline-toggle">
          <input name="tool-enabled" checked={draft.enabled} onChange={(event) => setDraft({ ...draft, enabled: event.target.checked })} type="checkbox" />
          启用 tool
        </label>
        <button className="primary-button" onClick={() => onSave(draft)} type="button">
          保存 tool
        </button>
      </div>
    </section>
  );
}

function LoginPanel({
  loginToken,
  error,
  lockState,
  remainingAttempts,
  onTokenChange,
  onLogin
}: {
  loginToken: string;
  error: string;
  lockState?: string;
  remainingAttempts?: number;
  onTokenChange: (value: string) => void;
  onLogin: () => Promise<void>;
}) {
  return (
    <section className="panel auth-panel">
      <div className="section-card">
        <div className="section-header">
          <h2>控制面登录</h2>
        </div>
        <p className="muted">system resources 与 active truth dir 管理入口受最小 token 登录保护。</p>
        <label>
          Control Plane Token
          <input
            name="control-plane-token"
            type="password"
            value={loginToken}
            onChange={(event) => onTokenChange(event.target.value)}
            placeholder="输入 control-plane token"
          />
        </label>
        <div className="info-card">
          <strong>当前锁定状态：{lockState || "unknown"}</strong>
          <span>剩余尝试次数：{remainingAttempts ?? "-"}</span>
          {error ? <code>{error}</code> : null}
        </div>
        <button className="primary-button" onClick={() => onLogin()} type="button">
          登录控制面
        </button>
      </div>
    </section>
  );
}

function SystemResourcePanel({
  items,
  truthDir,
  onOpenAPIDebug,
  onRefresh,
  onStatus,
  onError
}: {
  items: SystemResourceDetail[];
  truthDir?: { path?: string; version?: string } | null;
  onOpenAPIDebug: (preset: APIDebugPreset) => Promise<void>;
  onRefresh: (message?: string) => Promise<void>;
  onStatus: (value: string) => void;
  onError: (value: string) => void;
}) {
  const [selectedAssetID, setSelectedAssetID] = useState<string>(items[0]?.asset_id ?? "__create__:policy_rule");
  const [draft, setDraft] = useState<SystemResourceDraft>(emptySystemResourceDraft);
  const [detail, setDetail] = useState<SystemResourceDetail | null>(null);
  const [source, setSource] = useState<SystemResourceSource | null>(null);
  const [parseResult, setParseResult] = useState<SystemResourceParseResult | null>(null);
  const [compileResult, setCompileResult] = useState<SystemResourceCompileResult | null>(null);
  const [pipeline, setPipeline] = useState<SystemResourcePipeline | null>(null);
  const [versions, setVersions] = useState<SystemResourceVersionSummary[]>([]);
  const [auditEntries, setAuditEntries] = useState<SystemResourceAuditEntry[]>([]);
  const [selectedVersion, setSelectedVersion] = useState<SystemResourceVersionDetail | null>(null);
  const [debugEndpoint, setDebugEndpoint] = useState("/api/runtime/respond");
  const [debugPayload, setDebugPayload] = useState<SystemResourceDebugPayload | null>(null);
  const [exportInfo, setExportInfo] = useState<string>("");
  const [packageManifest, setPackageManifest] = useState<CompiledAssetsPackageManifest | null>(null);
  const [expandedTruthNav, setExpandedTruthNav] = useState(true);
  const [expandedFolders, setExpandedFolders] = useState<Record<string, boolean>>({});

  const createMode = parseSystemResourceCreateMode(selectedAssetID);
  const isCreating = createMode !== null;
  const selectedItem = items.find((item) => item.asset_id === selectedAssetID) ?? null;
  const selectedSourcePath = detail?.source_path || selectedItem?.source_path || "";
  const selectedAssetDir = detail?.asset_id ? `${detail.asset_id}/` : selectedItem?.asset_id ? `${selectedItem.asset_id}/` : "";
  const folderGroups = buildSystemResourceFolderGroups(items);
  const selectedDisplayPath = isCreating ? previewSourcePath(createMode.assetType, draft.asset_id) : selectedSourcePath;
  const selectedPreviewAssetID = isCreating ? previewAssetID(createMode.assetType, draft.asset_id) : selectedAssetID;

  useEffect(() => {
    if (items.length === 0) {
      setSelectedAssetID("__create__:policy_rule");
      return;
    }
    if (!isCreating && !items.some((item) => item.asset_id === selectedAssetID)) {
      setSelectedAssetID(items[0].asset_id);
    }
  }, [isCreating, items, selectedAssetID]);

  useEffect(() => {
    setExpandedFolders((current) => {
      let changed = false;
      const next = { ...current };
      for (const group of folderGroups) {
        if (next[group.key] === undefined) {
          next[group.key] = true;
          changed = true;
        }
      }
      return changed ? next : current;
    });
  }, [folderGroups]);

  useEffect(() => {
    if (createMode) {
      setDraft(createDraftForMode(createMode));
      setDetail(null);
      setSource(null);
      setParseResult(null);
      setCompileResult(null);
      setPipeline(null);
      setVersions([]);
      setAuditEntries([]);
      setSelectedVersion(null);
      setDebugPayload(null);
      return;
    }
    let cancelled = false;
    (async () => {
      try {
        const [nextDetail, nextSource, nextParse, nextCompile, nextPipeline, nextVersions, nextAudit] = await Promise.all([
          loadSystemResource(selectedAssetID),
          loadSystemResourceSource(selectedAssetID),
          loadSystemResourceParseResult(selectedAssetID).catch(() => null),
          loadSystemResourceCompileResult(selectedAssetID).catch(() => null),
          loadSystemResourcePipeline(selectedAssetID).catch(() => null),
          loadSystemResourceVersions(selectedAssetID).catch(() => ({ items: [] })),
          loadSystemResourceAudit(selectedAssetID).catch(() => ({ items: [] }))
        ]);
        if (cancelled) {
          return;
        }
        setDetail(nextDetail);
        setDraft(toSystemResourceDraft(nextDetail, nextSource));
        setSource(nextSource);
        setParseResult(nextParse);
        setCompileResult(nextCompile);
        setPipeline(nextPipeline);
        setVersions(nextVersions.items || []);
        setAuditEntries(nextAudit.items || []);
        setSelectedVersion(null);
        setDebugPayload(null);
      } catch (cause) {
        if (cancelled) {
          return;
        }
        onError(cause instanceof Error ? cause.message : String(cause));
        onStatus("system resource 详情加载失败");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [selectedAssetID, onError, onStatus]);

  async function withPanelAction(action: () => Promise<void>) {
    try {
      await action();
      onError("");
    } catch (cause) {
      onError(cause instanceof Error ? cause.message : String(cause));
    }
  }

  return (
    <section className="panel split-panel system-resource-panel">
      <div className="list-pane">
        <section className="truth-nav-root">
          <button
            aria-expanded={expandedTruthNav}
            className={expandedTruthNav ? "truth-nav-toggle active" : "truth-nav-toggle"}
            onClick={() => setExpandedTruthNav((current) => !current)}
            type="button"
          >
            <strong>Active Truth Dir</strong>
            <span>{truthDir?.path || "未暴露路径"}</span>
            <span>version: {truthDir?.version || "unknown"}</span>
          </button>
          {expandedTruthNav ? (
            <div className="truth-nav-body">
              <p className="section-help">
                <code>sources/</code> 是人维护的 system 主源。点目录展开文件，点文件后直接编辑并编译。
              </p>
              <div className="truth-nav-actions">
                <button
                  className="secondary-button"
                  onClick={() => withPanelAction(async () => {
                    const response = await syncSystemResources();
                    setSelectedVersion(null);
                    setDebugPayload(null);
                    await onRefresh(`已同步 ${response.items.length} 条主源并完成编译`);
                  })}
                  type="button"
                >
                  遍历 sources 并编译
                </button>
                <button
                  className="secondary-button"
                  onClick={() => withPanelAction(async () => {
                    const result = await exportSystemResources();
                    downloadBlob(result.blob, result.info.export_file || "system-resources-export.zip");
                    setExportInfo(`${result.info.export_file} · ${result.info.truth_dir_version || "unknown"}`);
                    onStatus("truth dir 快照已导出");
                  })}
                  type="button"
                >
                  导出 truth dir 快照
                </button>
              </div>
              {exportInfo ? <code>{exportInfo}</code> : null}
              <details className="advanced-section">
                <summary>高级发布操作</summary>
                <div className="advanced-section-body">
                  <p className="section-help">
                    这里只放发布和留档相关动作，不影响当前实例的 source 编辑与即时生效。
                  </p>
                  <div className="action-row">
                    <button
                      className="secondary-button"
                      onClick={() => withPanelAction(async () => {
                        const manifest = await buildSystemAssetsPackage();
                        setPackageManifest(manifest);
                        onStatus(`已构建 ${manifest.asset_count || 0} 条 compiled assets 到 ${manifest.output_dir || "output/system-assets"}`);
                      })}
                      type="button"
                    >
                      构建 compiled 包
                    </button>
                  </div>
                  {packageManifest?.output_dir ? <code>compiled: {packageManifest.output_dir}</code> : null}
                  {packageManifest ? (
                    <label>
                      Build Manifest
                      <textarea name="system-build-manifest" className="debug-textarea compact" value={formatMaybeJSON(packageManifest)} readOnly rows={10} />
                    </label>
                  ) : null}
                </div>
              </details>
              <div className="system-folder-list">
                {folderGroups.map((group) => {
                  const expanded = expandedFolders[group.key] !== false;
                  return (
                    <section className="system-folder-group" key={group.key}>
                      <div className="folder-group-header">
                        <button
                          aria-expanded={expanded}
                          className={expanded ? "folder-toggle active" : "folder-toggle"}
                          onClick={() => setExpandedFolders((current) => ({ ...current, [group.key]: !expanded }))}
                          type="button"
                        >
                          <strong>{group.label}</strong>
                          <span className="folder-description">{group.description}</span>
                        </button>
                        {group.assetType && group.createLabel ? (
                          <button
                            aria-label={group.createLabel}
                            className={createMode?.assetType === group.assetType ? "folder-create-button active" : "folder-create-button"}
                            onClick={() => setSelectedAssetID(`__create__:${group.assetType}`)}
                            title={group.createLabel}
                            type="button"
                          >
                            +
                          </button>
                        ) : null}
                      </div>
                      {expanded ? (
                        <div className="folder-children">
                          {group.items.map((item) => (
                            <button
                              key={item.asset_id}
                              className={item.asset_id === selectedAssetID ? "list-item file-item active" : "list-item file-item"}
                              onClick={() => setSelectedAssetID(item.asset_id)}
                              type="button"
                            >
                              <strong>{sourceFileLabel(item)}</strong>
                              <span>{item.asset_name || item.asset_id}</span>
                              <span>{item.asset_type} · {item.status || "unknown"}</span>
                              <span>{item.compiled_version || item.truth_dir_version || "-"}</span>
                            </button>
                          ))}
                        </div>
                      ) : null}
                    </section>
                  );
                })}
              </div>
            </div>
          ) : null}
        </section>
      </div>
      <div className="editor-pane">
        <div className="section-card">
          <div className="section-header">
            <h2>{isCreating ? createMode.title : `编辑 ${selectedDisplayPath || selectedAssetID}`}</h2>
          </div>
          {isCreating ? (
            <>
              <p className="section-help">{createMode.description}</p>
              <div className="system-meta-grid">
                <div className="info-card system-meta-card">
                  <strong>目标目录</strong>
                  <code>{createMode.sourceFolder}</code>
                </div>
                <div className="info-card system-meta-card">
                  <strong>将生成 Asset ID</strong>
                  <code>{selectedPreviewAssetID}</code>
                </div>
                <div className="info-card system-meta-card">
                  <strong>将写入 source 文件</strong>
                  <code>{selectedDisplayPath}</code>
                </div>
                <div className="info-card system-meta-card">
                  <strong>编译行为</strong>
                  <span>创建后立即进入 parse / compile / activate</span>
                </div>
              </div>
              <label>
                文件名 / 标识
                <input name="system-resource-create-asset-id" value={draft.asset_id} onChange={(event) => setDraft({ ...draft, asset_id: event.target.value })} />
              </label>
              <label>
                显示名（可选）
                <input name="system-resource-create-asset-name" value={draft.asset_name} onChange={(event) => setDraft({ ...draft, asset_name: event.target.value })} />
              </label>
              <label>
                变更说明
                <input name="system-resource-create-message" value={draft.message} onChange={(event) => setDraft({ ...draft, message: event.target.value })} />
              </label>
              <label>
                初始 source 内容
                <textarea name="system-resource-create-source-content" className="debug-textarea" value={draft.source_content} onChange={(event) => setDraft({ ...draft, source_content: event.target.value })} rows={14} />
              </label>
              <details className="advanced-section">
                <summary>高级元数据</summary>
                <div className="advanced-section-body">
                  <label className="inline-toggle">
                    <input name="system-resource-create-read-only" checked={draft.read_only} onChange={(event) => setDraft({ ...draft, read_only: event.target.checked })} type="checkbox" />
                    Read only
                  </label>
                  <label>
                    Metadata JSON
                    <textarea name="system-resource-create-metadata" value={draft.metadata_text} onChange={(event) => setDraft({ ...draft, metadata_text: event.target.value })} />
                  </label>
                </div>
              </details>
              <div className="action-row">
                <button
                  className="primary-button"
                  onClick={() => withPanelAction(async () => {
                    const input = buildCreateInput(createMode, draft);
                    await createSystemResource(input);
                    await onRefresh(`system source ${input.asset_id} 已创建并编译`);
                    setSelectedAssetID(input.asset_id);
                  })}
                  type="button"
                >
                  创建并编译
                </button>
                {items[0] ? (
                  <button className="secondary-button" onClick={() => setSelectedAssetID(items[0].asset_id)} type="button">
                    取消
                  </button>
                ) : null}
              </div>
            </>
          ) : (
            <>
              <p className="section-help">
                当前页面直接对应 <code>{selectedDisplayPath || selectedAssetID}</code>。日常操作以修改 source 文件并重新编译为主，不再把它当成通用资源 CRUD 表单。
              </p>
              <div className="system-meta-grid">
                <div className="info-card system-meta-card">
                  <strong>Source 文件</strong>
                  <code>{selectedDisplayPath || "未绑定 source path"}</code>
                </div>
                <div className="info-card system-meta-card">
                  <strong>Asset ID</strong>
                  <code>{selectedPreviewAssetID}</code>
                </div>
                <div className="info-card system-meta-card">
                  <strong>Asset Type</strong>
                  <code>{detail?.asset_type || selectedItem?.asset_type || "-"}</code>
                </div>
                <div className="info-card system-meta-card">
                  <strong>System 产物目录</strong>
                  <code>{selectedAssetDir || "未生成"}</code>
                </div>
              </div>
              <details className="advanced-section">
                <summary>高级元数据</summary>
                <div className="advanced-section-body">
                  <label>
                    Asset Name
                    <input name="system-resource-asset-name" value={draft.asset_name} onChange={(event) => setDraft({ ...draft, asset_name: event.target.value })} />
                  </label>
                  <label>
                    Scope
                    <input name="system-resource-scope" value={draft.scope} onChange={(event) => setDraft({ ...draft, scope: event.target.value })} />
                  </label>
                  <label>
                    Source Kind
                    <input name="system-resource-source-kind" value={draft.source_kind} onChange={(event) => setDraft({ ...draft, source_kind: event.target.value })} />
                  </label>
                  <label className="inline-toggle">
                    <input name="system-resource-read-only" checked={draft.read_only} onChange={(event) => setDraft({ ...draft, read_only: event.target.checked })} type="checkbox" />
                    Read only
                  </label>
                  <label>
                    Metadata JSON
                    <textarea name="system-resource-metadata" value={draft.metadata_text} onChange={(event) => setDraft({ ...draft, metadata_text: event.target.value })} />
                  </label>
                  <div className="action-row">
                    <button
                      className="secondary-button"
                      onClick={() => withPanelAction(async () => {
                        const patch: SystemResourceMetadataPatch = {
                          asset_type: draft.asset_type.trim(),
                          asset_name: draft.asset_name.trim(),
                          scope: draft.scope.trim(),
                          source_kind: draft.source_kind.trim(),
                          read_only: draft.read_only,
                          metadata: parseObjectText(draft.metadata_text)
                        };
                        await patchSystemResourceMetadata(selectedAssetID, patch);
                        await onRefresh(`system resource ${selectedAssetID} 元数据已更新`);
                      })}
                      type="button"
                    >
                      保存高级元数据
                    </button>
                  </div>
                </div>
              </details>
            </>
          )}
        </div>

        {!isCreating ? (
          <div className="section-card">
            <div className="section-header">
              <h2>真实文件与生效状态</h2>
            </div>
            <div className="system-meta-grid">
              <div className="info-card system-meta-card">
                <strong>Asset ID</strong>
                <code>{detail?.asset_id || selectedItem?.asset_id || "-"}</code>
              </div>
              <div className="info-card system-meta-card">
                <strong>Asset Type</strong>
                <code>{detail?.asset_type || selectedItem?.asset_type || "-"}</code>
              </div>
              <div className="info-card system-meta-card">
                <strong>Source 文件</strong>
                <code>{selectedSourcePath || "未绑定 source path"}</code>
              </div>
              <div className="info-card system-meta-card">
                <strong>System 产物目录</strong>
                <code>{selectedAssetDir || "未生成"}</code>
              </div>
              <div className="info-card system-meta-card">
                <strong>Truth Dir Version</strong>
                <code>{detail?.truth_dir_version || selectedItem?.truth_dir_version || "unknown"}</code>
              </div>
              <div className="info-card system-meta-card">
                <strong>Compiled Version</strong>
                <code>{detail?.compiled_version || selectedItem?.compiled_version || "not compiled"}</code>
              </div>
            </div>
            <div className="system-meta-grid">
              <div className="info-card system-meta-card">
                <strong>Source Kind</strong>
                <span>{detail?.source_kind || selectedItem?.source_kind || "-"}</span>
              </div>
              <div className="info-card system-meta-card">
                <strong>Scope</strong>
                <span>{detail?.scope || selectedItem?.scope || "-"}</span>
              </div>
              <div className="info-card system-meta-card">
                <strong>Read Only</strong>
                <span>{detail?.read_only || selectedItem?.read_only ? "true" : "false"}</span>
              </div>
              <div className="info-card system-meta-card">
                <strong>Updated At</strong>
                <span>{detail?.updated_at || selectedItem?.updated_at || "-"}</span>
              </div>
            </div>
          </div>
        ) : null}

        <div className="section-card">
          <div className="section-header">
            <h2>源内容</h2>
            {!isCreating ? (
              <div className="action-row">
                <button
                  className="secondary-button"
                  onClick={() => withPanelAction(async () => {
                    const latest = await loadSystemResourceSource(selectedAssetID);
                    setSource(latest);
                    setDraft((current) => ({ ...current, source_content: latest.source_content || "", message: latest.message || current.message }));
                    onStatus(`已从 ${selectedSourcePath || "source 文件"} 重新加载当前内容`);
                  })}
                  type="button"
                >
                  从文件重新加载
                </button>
                <button
                  className="secondary-button"
                  onClick={() => withPanelAction(async () => {
                    await saveSystemResourceSource(selectedAssetID, {
                      asset_id: selectedAssetID,
                      source_content: draft.source_content,
                      message: draft.message
                    });
                    await onRefresh(`system resource ${selectedAssetID} source 已保存并进入 pipeline`);
                    const latest = await loadSystemResourceSource(selectedAssetID);
                    setSource(latest);
                    setDraft((current) => ({ ...current, source_content: latest.source_content || "", message: latest.message || current.message }));
                  })}
                  type="button"
                >
                  保存并编译
                </button>
                <button
                  className="secondary-button"
                  onClick={() => withPanelAction(async () => {
                    const file = await downloadSystemResource(selectedAssetID);
                    downloadBlob(file.blob, file.filename);
                    onStatus(`已下载 ${file.filename}`);
                  })}
                  type="button"
                >
                  下载 source
                </button>
              </div>
            ) : null}
          </div>
          <label>
            Message
            <input name="system-resource-message" value={draft.message} onChange={(event) => setDraft({ ...draft, message: event.target.value })} />
          </label>
          {!isCreating ? (
            <label>
              当前 source 文件内容（磁盘真相）
              <textarea name="system-resource-current-source" className="debug-textarea compact" value={source?.source_content || ""} readOnly rows={8} />
            </label>
          ) : null}
          <label>
            编辑草稿
            <textarea name="system-resource-source-draft" className="debug-textarea" value={draft.source_content} onChange={(event) => setDraft({ ...draft, source_content: event.target.value })} rows={14} />
          </label>
          {source?.updated_at ? <span className="muted">最近更新时间：{source.updated_at}</span> : null}
        </div>

        <div className="section-card">
          <div className="section-header">
            <h2>Pipeline / Parse / Compile</h2>
            {!isCreating ? (
              <div className="action-row">
                <button className="secondary-button" onClick={() => withPanelAction(async () => {
                  await parseSystemResource(selectedAssetID);
                  await onRefresh(`system resource ${selectedAssetID} parse 已触发`);
                })} type="button">Parse</button>
                <button className="secondary-button" onClick={() => withPanelAction(async () => {
                  await compileSystemResource(selectedAssetID);
                  await onRefresh(`system resource ${selectedAssetID} compile 已触发`);
                })} type="button">Compile</button>
                <button className="secondary-button" onClick={() => withPanelAction(async () => {
                  await activateSystemResource(selectedAssetID);
                  await onRefresh(`system resource ${selectedAssetID} 已激活`);
                })} type="button">Activate</button>
                <button
                  className="secondary-button"
                  disabled={!parseResult}
                  onClick={() => withPanelAction(async () => {
                    downloadJSON(parseResult, `${selectedAssetID}.parse-result.json`);
                    onStatus(`已下载 ${selectedAssetID} parse 结果`);
                  })}
                  type="button"
                >
                  下载 Parse 结果
                </button>
                <button
                  className="secondary-button"
                  disabled={!compileResult}
                  onClick={() => withPanelAction(async () => {
                    downloadJSON(compileResult, `${selectedAssetID}.compile-result.json`);
                    onStatus(`已下载 ${selectedAssetID} compile 结果`);
                  })}
                  type="button"
                >
                  下载 Compile 结果
                </button>
              </div>
            ) : null}
          </div>
          <div className="system-meta-grid">
            <div className="info-card system-meta-card">
              <strong>Pipeline Status</strong>
              <span>{pipeline?.status || "unknown"}</span>
            </div>
            <div className="info-card system-meta-card">
              <strong>Current Step</strong>
              <span>{pipeline?.current_step || "-"}</span>
            </div>
            <div className="info-card system-meta-card">
              <strong>Parse Status</strong>
              <span>{parseResult?.status || "-"}</span>
            </div>
            <div className="info-card system-meta-card">
              <strong>Compile Status</strong>
              <span>{compileResult?.status || "-"}</span>
            </div>
          </div>
          <label>
            Pipeline
            <textarea name="system-resource-pipeline" className="debug-textarea" value={formatMaybeJSON(pipeline)} readOnly rows={8} />
          </label>
          <label>
            Parse Result
            <textarea name="system-resource-parse-result" className="debug-textarea" value={formatMaybeJSON(parseResult)} readOnly rows={10} />
          </label>
          <label>
            Compile Result
            <textarea name="system-resource-compile-result" className="debug-textarea" value={formatMaybeJSON(compileResult)} readOnly rows={12} />
          </label>
          <label>
            Runtime Payload
            <textarea name="system-resource-runtime-payload" className="debug-textarea compact" value={formatMaybeJSON(compileResult?.payload || null)} readOnly rows={10} />
          </label>
        </div>

        <div className="section-card">
          <div className="section-header">
            <h2>版本 / 审计 / 回滚</h2>
            {!isCreating ? (
              <div className="action-row">
                <button
                  className="secondary-button"
                  disabled={!selectedVersion}
                  onClick={() => withPanelAction(async () => {
                    if (!selectedVersion) {
                      return;
                    }
                    await rollbackSystemResourceVersion(selectedAssetID, selectedVersion.version_id);
                    await onRefresh(`system resource ${selectedAssetID} 已回滚到 ${selectedVersion.version_id}`);
                  })}
                  type="button"
                >
                  回滚到所选版本
                </button>
              </div>
            ) : null}
          </div>
          <div className="split-panel embedded-split">
            <div className="list-pane compact-list-pane">
              <strong>版本快照</strong>
              {versions.length === 0 ? <span className="muted">暂无版本快照</span> : null}
              {versions.map((item) => (
                <button
                  key={item.version_id}
                  className={item.version_id === selectedVersion?.version_id ? "list-item active" : "list-item"}
                  onClick={() => withPanelAction(async () => {
                    const detail = await loadSystemResourceVersion(selectedAssetID, item.version_id);
                    setSelectedVersion(detail);
                    onStatus(`已加载版本 ${item.version_id}`);
                  })}
                  type="button"
                >
                  <strong>{item.action || "snapshot"}</strong>
                  <span>{item.version_id}</span>
                  <span>{item.created_at || "-"}</span>
                </button>
              ))}
            </div>
            <div className="editor-pane">
              <label>
                版本详情
                <textarea name="system-resource-version-detail" className="debug-textarea" value={formatMaybeJSON(selectedVersion)} readOnly rows={12} />
              </label>
              <label>
                审计轨迹
                <textarea name="system-resource-audit-trail" className="debug-textarea" value={formatMaybeJSON(auditEntries)} readOnly rows={10} />
              </label>
            </div>
          </div>
        </div>

        <div className="section-card">
          <div className="section-header">
            <div>
              <h2>调试 Payload</h2>
              <p className="section-help">外层包装用于说明目标接口；发送请求时只使用“实际请求体”。</p>
            </div>
            {!isCreating ? (
              <div className="action-row">
                <select name="system-resource-debug-endpoint" value={debugEndpoint} onChange={(event) => setDebugEndpoint(event.target.value)}>
                  <option value="/api/chat/respond">POST /api/chat/respond</option>
                  <option value="/api/chat/stream">POST /api/chat/stream</option>
                  <option value="/api/runtime/respond">POST /api/runtime/respond</option>
                </select>
                <button className="secondary-button" onClick={() => withPanelAction(async () => {
                  const payload = await loadSystemResourceDebugPayload(selectedAssetID, debugEndpoint);
                  setDebugPayload(payload);
                  onStatus(`已生成 ${debugEndpoint} 调试 payload`);
                })} type="button">
                  生成调试 payload
                </button>
                <button
                  className="secondary-button"
                  disabled={!debugPayload}
                  onClick={() => withPanelAction(async () => {
                    if (!debugPayload) {
                      return;
                    }
                    await copyTextToClipboard(formatMaybeJSON(debugPayload.payload));
                    onStatus("实际请求体已复制到剪贴板");
                  })}
                  type="button"
                >
                  复制实际请求体
                </button>
                <button
                  className="secondary-button"
                  disabled={!debugPayload}
                  onClick={() => withPanelAction(async () => {
                    if (!debugPayload) {
                      return;
                    }
                    await copyTextToClipboard(formatMaybeJSON(debugPayload));
                    onStatus("调试包装已复制到剪贴板");
                  })}
                  type="button"
                >
                  复制外层包装
                </button>
                <button
                  className="secondary-button"
                  disabled={!debugPayload}
                  onClick={() => withPanelAction(async () => {
                    if (!debugPayload) {
                      return;
                    }
                    await onOpenAPIDebug({
                      endpointKey: endpointKeyForDebugPayload(debugPayload.endpoint),
                      bodyText: formatMaybeJSON(debugPayload.payload)
                    });
                  })}
                  type="button"
                >
                  打开接口调试
                </button>
              </div>
            ) : null}
          </div>
          <div className="system-meta-grid">
            <div className="info-card">
              <span className="status-label">target endpoint</span>
              <strong>{normalizeDebugEndpoint(debugPayload?.endpoint || debugEndpoint)}</strong>
              <span>API Debug 会把实际请求体发送到这个接口。</span>
            </div>
            <div className="info-card">
              <span className="status-label">request body</span>
              <strong>{debugPayload?.payload && Object.prototype.hasOwnProperty.call(debugPayload.payload, "query") ? "query ready" : "not generated"}</strong>
              <span>如果把外层包装整体发送，会缺少顶层 query。</span>
            </div>
          </div>
          <div className="split-panel embedded-split">
            <label>
              实际请求体
              <textarea name="system-resource-debug-request-body" className="debug-textarea" value={formatMaybeJSON(debugPayload?.payload)} readOnly rows={16} />
            </label>
            <label>
              外层包装
              <textarea name="system-resource-debug-wrapper" className="debug-textarea compact" value={formatMaybeJSON(debugPayload)} readOnly rows={12} />
            </label>
          </div>
        </div>
      </div>
    </section>
  );
}

function SystemValidationPanel({
  data,
  items,
  truthDir,
  apiEndpoints,
  onOpenAPIDebug,
  onRefresh,
  onStatus,
  onError
}: {
  data: BootstrapPayload;
  items: SystemResourceDetail[];
  truthDir?: { path?: string; version?: string } | null;
  apiEndpoints: OpenAPIEndpoint[];
  onOpenAPIDebug: (preset: APIDebugPreset) => Promise<void>;
  onRefresh: (message?: string) => Promise<void>;
  onStatus: (value: string) => void;
  onError: (value: string) => void;
}) {
  const [selectedAssetID, setSelectedAssetID] = useState<string>(items[0]?.asset_id ?? "");
  const [detail, setDetail] = useState<SystemResourceDetail | null>(null);
  const [source, setSource] = useState<SystemResourceSource | null>(null);
  const [parseResult, setParseResult] = useState<SystemResourceParseResult | null>(null);
  const [compileResult, setCompileResult] = useState<SystemResourceCompileResult | null>(null);
  const [pipeline, setPipeline] = useState<SystemResourcePipeline | null>(null);
  const [debugPayload, setDebugPayload] = useState<SystemResourceDebugPayload | null>(null);
  const [selectedSceneID, setSelectedSceneID] = useState(data.scenes[0]?.id ?? "");
  const [selectedSkillName, setSelectedSkillName] = useState(data.skills[0]?.name ?? "");
  const [promptText, setPromptText] = useState("请基于当前 system truth，简要说明这个场景会如何处理一次真实用户请求。");
  const [trialRunning, setTrialRunning] = useState(false);
  const [trialMeta, setTrialMeta] = useState("");
  const [trialResponse, setTrialResponse] = useState("");
  const [runtimeRuns, setRuntimeRuns] = useState<RuntimeRun[]>([]);
  const [selectedRunID, setSelectedRunID] = useState("");
  const [runtimeRun, setRuntimeRun] = useState<RuntimeRun | null>(null);
  const [runtimeSteps, setRuntimeSteps] = useState<RuntimeStep[]>([]);
  const [runtimeLifecycleEvents, setRuntimeLifecycleEvents] = useState<RuntimeLifecycleEvent[]>([]);
  const [runtimeTraces, setRuntimeTraces] = useState<RuntimeTrace[]>([]);
  const [runtimeUsage, setRuntimeUsage] = useState<RuntimeUsage[]>([]);
  const [runtimeProjections, setRuntimeProjections] = useState<RuntimeProjectionCandidate[]>([]);
  const [runtimeCheckpoints, setRuntimeCheckpoints] = useState<RuntimeCheckpointReadout[]>([]);
  const [runtimeFoundation, setRuntimeFoundation] = useState<RuntimeContractFoundation | null>(null);
  const [runtimeContractDraft, setRuntimeContractDraft] = useState("");
  const [runtimeTaskTypeDraft, setRuntimeTaskTypeDraft] = useState("");
  const [runtimeHookBindingDraft, setRuntimeHookBindingDraft] = useState("");
  const [runtimeFoundationSaveRunning, setRuntimeFoundationSaveRunning] = useState(false);
  const [lastRuntimeValidationRun, setLastRuntimeValidationRun] = useState<RuntimeValidationRunResponse | null>(null);
  const [runtimeReadLoading, setRuntimeReadLoading] = useState(false);
  const [runtimeTriggerRunning, setRuntimeTriggerRunning] = useState(false);
  const [runtimeReadError, setRuntimeReadError] = useState("");
  const [toolGovernancePolicy, setToolGovernancePolicy] = useState<ToolGovernancePolicy | null>(null);
  const [toolGovernanceDecisions, setToolGovernanceDecisions] = useState<ToolGovernanceDecision[]>([]);
  const [toolGovernanceDecision, setToolGovernanceDecision] = useState<ToolGovernanceDecision | null>(null);
  const [toolGovernanceLoading, setToolGovernanceLoading] = useState(false);
  const [toolGovernanceError, setToolGovernanceError] = useState("");
  const [validationMCPServer, setValidationMCPServer] = useState<ValidationMCPServerInfo | null>(null);
  const [validationMCPTools, setValidationMCPTools] = useState<ValidationMCPToolSchema[]>([]);
  const [validationMCPInvocation, setValidationMCPInvocation] = useState<ValidationMCPInvocationResponse | null>(null);
  const [validationMCPLoading, setValidationMCPLoading] = useState(false);
  const [validationMCPError, setValidationMCPError] = useState("");
  const [baselineText, setBaselineText] = useState("");
  const [candidateText, setCandidateText] = useState("");

  const endpointKeys = new Set(apiEndpoints.map((item) => item.key));
  const runtimeRespondBody = formatMaybeJSON(buildRuntimeRespondPayload(promptText, selectedAssetID, truthDir?.version));
  const scenarioRespondBody = formatMaybeJSON(defaultRuntimeScenarioPayload());
  const p0Checks = buildP0ValidationChecks(data, items, endpointKeys, debugPayload);
  const selectedItem = items.find((item) => item.asset_id === selectedAssetID) ?? null;
  const selectedScene = data.scenes.find((item) => item.id === selectedSceneID) ?? data.scenes[0] ?? null;
  const sceneDefaultSkills = new Set(selectedScene?.default_skills ?? []);
  const sceneSkills = data.skills.filter((item) => sceneDefaultSkills.size === 0 || sceneDefaultSkills.has(item.name));
  const selectableSkills = sceneSkills.length > 0 ? sceneSkills : data.skills;
  const checks = buildSystemValidationChecks(data, items);
  const comparison = buildTextComparison(baselineText, candidateText);

  useEffect(() => {
    if (items.length === 0) {
      setSelectedAssetID("");
      return;
    }
    if (!items.some((item) => item.asset_id === selectedAssetID)) {
      setSelectedAssetID(items[0].asset_id);
    }
  }, [items, selectedAssetID]);

  useEffect(() => {
    if (data.scenes.length === 0) {
      setSelectedSceneID("");
      return;
    }
    if (!data.scenes.some((item) => item.id === selectedSceneID)) {
      setSelectedSceneID(data.scenes[0].id);
    }
  }, [data.scenes, selectedSceneID]);

  useEffect(() => {
    const nextSkill = selectableSkills[0]?.name ?? "";
    if (!selectedSkillName || !selectableSkills.some((item) => item.name === selectedSkillName)) {
      setSelectedSkillName(nextSkill);
    }
  }, [selectableSkills, selectedSkillName]);

  useEffect(() => {
    if (!selectedAssetID) {
      setDetail(null);
      setSource(null);
      setParseResult(null);
      setCompileResult(null);
      setPipeline(null);
      setDebugPayload(null);
      return;
    }
    let cancelled = false;
    (async () => {
      const [nextDetail, nextSource, nextParse, nextCompile, nextPipeline, nextDebugPayload] = await Promise.all([
        loadSystemResource(selectedAssetID).catch(() => null),
        loadSystemResourceSource(selectedAssetID).catch(() => null),
        loadSystemResourceParseResult(selectedAssetID).catch(() => null),
        loadSystemResourceCompileResult(selectedAssetID).catch(() => null),
        loadSystemResourcePipeline(selectedAssetID).catch(() => null),
        loadSystemResourceDebugPayload(selectedAssetID, "/api/runtime/respond").catch(() => null)
      ]);
      if (cancelled) {
        return;
      }
      setDetail(nextDetail);
      setSource(nextSource);
      setParseResult(nextParse);
      setCompileResult(nextCompile);
      setPipeline(nextPipeline);
      setDebugPayload(nextDebugPayload);
    })().catch((cause: Error) => {
      if (!cancelled) {
        onError(cause.message);
        onStatus("system validation 资源详情加载失败");
      }
    });
    return () => {
      cancelled = true;
    };
  }, [selectedAssetID, onError, onStatus]);

  useEffect(() => {
    refreshRuntimeRecords().catch((cause: Error) => {
      setRuntimeReadError(cause.message);
    });
    refreshToolGovernance().catch((cause: Error) => {
      setToolGovernanceError(cause.message);
    });
    refreshValidationMCP().catch((cause: Error) => {
      setValidationMCPError(cause.message);
    });
  }, []);

  async function refreshRuntimeRecords(preferredRunID = selectedRunID) {
    setRuntimeReadLoading(true);
    try {
      const [runList, foundation] = await Promise.all([
        loadRuntimeRuns(20),
        loadRuntimeContractFoundation()
      ]);
      const runs = runList.items ?? [];
      const nextRunID = preferredRunID && runs.some((item) => item.id === preferredRunID) ? preferredRunID : runs[0]?.id ?? "";
      setRuntimeFoundation(foundation);
      setRuntimeContractDraft(formatMaybeJSON(foundation.contracts[0] ?? defaultRuntimeContractDraft()));
      setRuntimeTaskTypeDraft(formatMaybeJSON(foundation.task_types[0] ?? defaultRuntimeTaskTypeDraft()));
      setRuntimeHookBindingDraft(formatMaybeJSON(foundation.hook_bindings[0] ?? defaultRuntimeHookBindingDraft(foundation.contracts[0]?.id)));
      setRuntimeRuns(runs);
      setSelectedRunID(nextRunID);
      if (!nextRunID) {
        setRuntimeRun(null);
        setRuntimeSteps([]);
        setRuntimeLifecycleEvents([]);
        setRuntimeTraces([]);
        setRuntimeUsage([]);
        setRuntimeProjections([]);
        setRuntimeCheckpoints([]);
        setRuntimeReadError("");
        return;
      }
      const [nextRun, nextSteps, nextLifecycle, nextTraces, nextUsage, nextProjections, nextCheckpoints] = await Promise.all([
        loadRuntimeRun(nextRunID),
        loadRuntimeSteps(nextRunID),
        loadRuntimeLifecycleEvents(nextRunID),
        loadRuntimeTraces(nextRunID),
        loadRuntimeUsage(nextRunID),
        loadRuntimeProjectionCandidates(nextRunID),
        loadRuntimeCheckpoints(nextRunID)
      ]);
      setRuntimeRun(nextRun);
      setRuntimeSteps(nextSteps.items ?? []);
      setRuntimeLifecycleEvents(nextLifecycle.items ?? []);
      setRuntimeTraces(nextTraces.items ?? []);
      setRuntimeUsage(nextUsage.items ?? []);
      setRuntimeProjections(nextProjections.items ?? []);
      setRuntimeCheckpoints(nextCheckpoints.items ?? []);
      setRuntimeReadError("");
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : String(cause);
      setRuntimeReadError(message);
      setRuntimeFoundation(null);
      setRuntimeContractDraft("");
      setRuntimeTaskTypeDraft("");
      setRuntimeHookBindingDraft("");
      setRuntimeRun(null);
      setRuntimeSteps([]);
      setRuntimeLifecycleEvents([]);
      setRuntimeTraces([]);
      setRuntimeUsage([]);
      setRuntimeProjections([]);
      setRuntimeCheckpoints([]);
    } finally {
      setRuntimeReadLoading(false);
    }
  }

  async function selectRuntimeRun(runID: string) {
    await withValidationAction(async () => {
      await refreshRuntimeRecords(runID);
    });
  }

  async function triggerRuntimeValidationRun() {
    await withValidationAction(async () => {
      setRuntimeTriggerRunning(true);
      try {
        const result = await createRuntimeValidationRun({
          workspace_id: "system-validation",
          scene: "system_validation",
          prompt: promptText,
          metadata: {
            ui_surface: "system_validation",
            truth_dir_version: truthDir?.version || ""
          }
        });
        setLastRuntimeValidationRun(result);
        setValidationMCPInvocation(result.validation_mcp);
        await refreshRuntimeRecords(result.run.id);
        onStatus("Runtime validation 已完成 Eino graph、MCP、sandbox 和持久化记录");
      } finally {
        setRuntimeTriggerRunning(false);
      }
    });
  }

  async function saveRuntimeFoundationDrafts() {
    await withValidationAction(async () => {
      setRuntimeFoundationSaveRunning(true);
      try {
        const contract = parseRuntimeFoundationJSON<RuntimeContractUpsertInput>(runtimeContractDraft, "RuntimeContract");
        const taskType = parseRuntimeFoundationJSON<RuntimeTaskTypeUpsertInput>(runtimeTaskTypeDraft, "TaskTypeRegistration");
        const hook = parseRuntimeFoundationJSON<RuntimeHookBindingUpsertInput>(runtimeHookBindingDraft, "HookBinding");
        const contractID = runtimeFoundation?.contracts[0]?.id || "runtime_validation_contract";
        const typeKey = runtimeFoundation?.task_types[0]?.type_key || "runtime_validation";
        const bindingID = runtimeFoundation?.hook_bindings[0]?.id || "runtime_validation_hook_binding";
        await Promise.all([
          saveRuntimeContract(contractID, contract),
          saveRuntimeTaskType(typeKey, taskType),
          saveRuntimeHookBinding(bindingID, hook)
        ]);
        await refreshRuntimeRecords(selectedRunID);
        onStatus("Runtime foundation 已保存并刷新 readout");
      } finally {
        setRuntimeFoundationSaveRunning(false);
      }
    });
  }

  async function refreshToolGovernance() {
    setToolGovernanceLoading(true);
    try {
      const [policy, decisions] = await Promise.all([
        loadToolGovernancePolicy(),
        loadToolGovernanceDecisions()
      ]);
      setToolGovernancePolicy(policy);
      setToolGovernanceDecisions(decisions.items ?? []);
      setToolGovernanceError("");
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : String(cause);
      setToolGovernanceError(message);
      setToolGovernancePolicy(null);
      setToolGovernanceDecisions([]);
    } finally {
      setToolGovernanceLoading(false);
    }
  }

  async function runToolGovernanceValidation() {
    await withValidationAction(async () => {
      setToolGovernanceLoading(true);
      try {
        const decision = await evaluateToolGovernance({
          tool_name: "demo_browser",
          tool_scope: "external_web",
          operation: "read",
          risk_level: "medium",
          metadata: {
            ui_surface: "system_validation",
            authorization_token: "sample-token-redacted"
          }
        });
        setToolGovernanceDecision(decision);
        await refreshToolGovernance();
        onStatus("Tool Governance validation 已生成并持久化决策记录");
      } finally {
        setToolGovernanceLoading(false);
      }
    });
  }

  async function refreshValidationMCP() {
    setValidationMCPLoading(true);
    try {
      const [serverInfo, tools] = await Promise.all([
        loadValidationMCPServer(),
        loadValidationMCPTools()
      ]);
      setValidationMCPServer(serverInfo);
      setValidationMCPTools(tools.items ?? []);
      setValidationMCPError("");
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : String(cause);
      setValidationMCPError(message);
      setValidationMCPServer(null);
      setValidationMCPTools([]);
    } finally {
      setValidationMCPLoading(false);
    }
  }

  async function runValidationMCPInvocation() {
    await withValidationAction(async () => {
      setValidationMCPLoading(true);
      try {
        const result = await invokeValidationMCPTool({
          tool_name: "risk_signal_lookup",
          input: {
            risk_key: "credential_export",
            credentials: {
              authorization: "sample-token-redacted"
            }
          },
          metadata: {
            ui_surface: "system_validation",
            authorization_token: "sample-token-redacted"
          }
        });
        setValidationMCPInvocation(result);
        await refreshValidationMCP();
        await refreshToolGovernance();
        onStatus("Validation MCP 已完成 schema、governance decision、result 和 trace 验证");
      } finally {
        setValidationMCPLoading(false);
      }
    });
  }

  async function withValidationAction(action: () => Promise<void>) {
    try {
      await action();
      onError("");
    } catch (cause) {
      onError(cause instanceof Error ? cause.message : String(cause));
    }
  }

  async function runTrial() {
    await withValidationAction(async () => {
      setTrialRunning(true);
      try {
        const result = await invokeDebugEndpoint("POST", "/api/runtime/respond", runtimeRespondBody);
        const formatted = formatResponseBody(result.body);
        setTrialMeta(`POST /api/runtime/respond -> ${result.status} ${result.statusText}`);
        setTrialResponse(formatted);
        if (result.ok) {
          setCandidateText(formatted);
          await refreshRuntimeRecords();
          onStatus("System Validation 已完成一次真实 runtime/respond 试运行");
        } else {
          onStatus("System Validation 试运行返回非 2xx 状态");
        }
      } finally {
        setTrialRunning(false);
      }
    });
  }

  async function openP0Endpoint(endpointKey: string, bodyText: string) {
    await withValidationAction(async () => {
      await onOpenAPIDebug({ endpointKey, bodyText });
    });
  }

  return (
    <section className="panel editor-pane" data-testid="system-validation-panel">
      <section className="section-card">
        <div className="section-header">
          <div>
            <h2>System Validation Workspace</h2>
            <p className="section-help">用真实 system resources、bootstrap 数据和 runtime/respond 接口验证 system 内容优化是否生效。</p>
          </div>
          <div className="action-row">
            <button
              className="secondary-button"
              onClick={() => withValidationAction(async () => {
                await onRefresh("System Validation 数据已刷新");
              })}
              type="button"
            >
              刷新验证数据
            </button>
            <button
              className="secondary-button"
              onClick={() => withValidationAction(async () => {
                const response = await syncSystemResources();
                await onRefresh(`已同步 ${response.items.length} 条 system truth 主源`);
              })}
              type="button"
            >
              同步主源并刷新
            </button>
          </div>
        </div>
        <div className="overview-grid">
          {checks.map((check) => (
            <div className={check.status === "ok" ? "info-card success" : check.status === "error" ? "info-card error" : "info-card"} key={check.title}>
              <span className="status-label">{check.status}</span>
              <strong>{check.title}</strong>
              <span>{check.detail}</span>
            </div>
          ))}
        </div>
      </section>

      <section className="section-card">
        <div className="section-header">
          <div>
            <h2>P0 验收向导</h2>
            <p className="section-help">按 Swagger 合约、System Resources payload、runtime respond、scenario respond 四步验收 P0。</p>
          </div>
          <span className="muted">Core Contract Reset</span>
        </div>
        <div className="overview-grid">
          {p0Checks.map((check) => (
            <div className={check.status === "ok" ? "info-card success" : check.status === "error" ? "info-card error" : "info-card"} key={check.title}>
              <span className="status-label">{check.status}</span>
              <strong>{check.title}</strong>
              <span>{check.detail}</span>
            </div>
          ))}
        </div>
        <div className="split-panel embedded-split">
          <div className="info-card">
            <span className="status-label">Step 1</span>
            <strong>Runtime direct respond</strong>
            <span>使用通用 ChatRespondRequest 形态验证 `/api/runtime/respond`，不要发送外层 debug 包装。</span>
            <button className="secondary-button" disabled={!endpointKeys.has("POST /api/runtime/respond")} onClick={() => openP0Endpoint("POST /api/runtime/respond", runtimeRespondBody)} type="button">
              打开 runtime/respond 调试
            </button>
          </div>
          <div className="info-card">
            <span className="status-label">Step 2</span>
            <strong>Scenario compatibility</strong>
            <span>使用 legacy RuntimeScenarioRequest 验证 `/api/runtime/scenario/respond` 仍只承接兼容场景判断。</span>
            <button className="secondary-button" disabled={!endpointKeys.has("POST /api/runtime/scenario/respond")} onClick={() => openP0Endpoint("POST /api/runtime/scenario/respond", scenarioRespondBody)} type="button">
              打开 scenario/respond 调试
            </button>
          </div>
        </div>
        <label>
          P0 Runtime Request Body
          <textarea name="validation-runtime-respond-body" className="debug-textarea compact" value={runtimeRespondBody} readOnly rows={10} />
        </label>
      </section>

      <section className="section-card">
        <div className="section-header">
          <div>
            <h2>Tool Governance Validation</h2>
            <p className="section-help">读取 `tool_governance_policy` system truth，验证 tool 请求执行前可生成 allow / deny / allow_with_redaction / require_sandbox_ref 决策。</p>
          </div>
          <div className="action-row">
            <span className="muted">{toolGovernanceLoading ? "判定中…" : toolGovernanceError ? "unavailable" : `${toolGovernancePolicy?.rules?.length ?? 0} rules`}</span>
            <button className="secondary-button" disabled={toolGovernanceLoading} onClick={runToolGovernanceValidation} type="button">
              验证 Tool Governance
            </button>
            <button className="secondary-button" disabled={toolGovernanceLoading} onClick={() => withValidationAction(async () => refreshToolGovernance())} type="button">
              刷新策略
            </button>
          </div>
        </div>
        {toolGovernanceError ? (
          <div className="info-card error">
            <span className="status-label">tool governance</span>
            <strong>read surface unavailable</strong>
            <span>{toolGovernanceError}</span>
          </div>
        ) : (
          <>
            <div className="system-meta-grid">
              <div className="info-card">
                <span className="status-label">policy</span>
                <strong>{toolGovernancePolicy?.name || toolGovernancePolicy?.policy_id || "no policy loaded"}</strong>
                <span>{toolGovernancePolicy?.asset_id || "-"}</span>
              </div>
              <div className="info-card">
                <span className="status-label">default</span>
                <strong>{toolGovernancePolicy?.default_decision || "allow"}</strong>
                <span>{toolGovernancePolicy?.decision_model || "first_match"}</span>
              </div>
              <div className={toolGovernanceDecisionClass(toolGovernanceDecision?.decision)}>
                <span className="status-label">latest decision</span>
                <strong>{toolGovernanceDecision?.decision || toolGovernanceDecisions[0]?.decision || "-"}</strong>
                <span>{toolGovernanceDecision?.matched_rule_id || toolGovernanceDecisions[0]?.matched_rule_id || "no matched rule"}</span>
              </div>
              <div className="info-card">
                <span className="status-label">decision log</span>
                <strong>{toolGovernanceDecisions.length} records</strong>
                <span>{toolGovernanceDecisions[0]?.tool_name || "no persisted decisions"}</span>
              </div>
            </div>
            <div className="split-panel embedded-split">
              <label>
                Effective Policy
                <textarea name="validation-tool-governance-policy" className="debug-textarea compact" value={formatMaybeJSON(toolGovernancePolicy)} readOnly rows={10} />
              </label>
              <label>
                Decision Log
                <textarea name="validation-tool-governance-decisions" className="debug-textarea compact" value={formatMaybeJSON(toolGovernanceDecisions)} readOnly rows={10} />
              </label>
            </div>
          </>
        )}
      </section>

      <section className="section-card">
        <div className="section-header">
          <div>
            <h2>Validation MCP</h2>
            <p className="section-help">验证 `athena-validation-mcp` 的 tool schema ingestion、治理判定、调用结果和白名单安全 trace。</p>
          </div>
          <div className="action-row">
            <span className="muted">{validationMCPLoading ? "调用中…" : validationMCPError ? "unavailable" : `${validationMCPTools.length} tools`}</span>
            <button className="secondary-button" disabled={validationMCPLoading} onClick={runValidationMCPInvocation} type="button">
              验证 Validation MCP
            </button>
            <button className="secondary-button" disabled={validationMCPLoading} onClick={() => withValidationAction(async () => refreshValidationMCP())} type="button">
              刷新 MCP
            </button>
          </div>
        </div>
        {validationMCPError ? (
          <div className="info-card error">
            <span className="status-label">validation mcp</span>
            <strong>read surface unavailable</strong>
            <span>{validationMCPError}</span>
          </div>
        ) : (
          <>
            <div className="system-meta-grid">
              <div className={validationMCPServer?.status === "ready" ? "info-card success" : "info-card"}>
                <span className="status-label">server</span>
                <strong>{validationMCPServer?.name || "no server loaded"}</strong>
                <span>{validationMCPServer?.transport || "-"}</span>
              </div>
              <div className="info-card">
                <span className="status-label">schema ingestion</span>
                <strong>{validationMCPTools.length} tools</strong>
                <span>{validationMCPTools.map((item) => item.name).join(", ") || "no schemas"}</span>
              </div>
              <div className={toolGovernanceDecisionClass(validationMCPInvocation?.governance_decision?.decision)}>
                <span className="status-label">governance</span>
                <strong>{validationMCPInvocation?.governance_decision?.decision || "-"}</strong>
                <span>{validationMCPInvocation?.governance_decision?.matched_rule_id || "no invocation yet"}</span>
              </div>
              <div className={validationMCPInvocation?.result?.applied_redaction ? "info-card success" : "info-card"}>
                <span className="status-label">trace</span>
                <strong>{validationMCPInvocation?.result?.trace?.trace_type || "not invoked"}</strong>
                <span>{validationMCPInvocation?.result?.applied_redaction ? "redacted payload" : "waiting for invocation"}</span>
              </div>
            </div>
            <div className="split-panel embedded-split">
              <label>
                MCP Tools
                <textarea name="validation-mcp-tools" className="debug-textarea compact" value={formatMaybeJSON(validationMCPTools)} readOnly rows={10} />
              </label>
              <label>
                MCP Invocation
                <textarea name="validation-mcp-invocation" className="debug-textarea compact" value={formatMaybeJSON(validationMCPInvocation)} readOnly rows={10} />
              </label>
            </div>
          </>
        )}
      </section>

      <section className="section-card">
        <div className="section-header">
          <div>
            <h2>MCP / Sandbox Validation</h2>
            <p className="section-help">展示一次 deterministic validation run 中的 Validation MCP 调用、external_sandbox_ref、结构化结果、audit summary 和 projection。</p>
          </div>
          <span className="muted">{lastRuntimeValidationRun ? lastRuntimeValidationRun.sandbox.sandbox_ref.status : "waiting for runtime validation"}</span>
        </div>
        {lastRuntimeValidationRun ? (
          <>
            <div className="system-meta-grid">
              <div className={lastRuntimeValidationRun.validation_mcp.result.status === "success" ? "info-card success" : "info-card"}>
                <span className="status-label">Validation MCP</span>
                <strong>{lastRuntimeValidationRun.validation_mcp.result.tool_name}</strong>
                <span>{lastRuntimeValidationRun.validation_mcp.governance_decision.decision || "no decision"}</span>
              </div>
              <div className={lastRuntimeValidationRun.sandbox.sandbox_ref.status === "completed" ? "info-card success" : "info-card"}>
                <span className="status-label">Sandbox Mode</span>
                <strong>{lastRuntimeValidationRun.sandbox.sandbox_ref.mode}</strong>
                <span>{lastRuntimeValidationRun.sandbox.sandbox_ref.provider}</span>
              </div>
              <div className="info-card">
                <span className="status-label">Execution Ref</span>
                <strong>{lastRuntimeValidationRun.sandbox.sandbox_ref.ref_id}</strong>
                <span>{lastRuntimeValidationRun.sandbox.sandbox_ref.boundary}</span>
              </div>
              <div className="info-card">
                <span className="status-label">Audit</span>
                <strong>{lastRuntimeValidationRun.sandbox.audit_summary.state_integrity}</strong>
                <span>{lastRuntimeValidationRun.sandbox.audit_summary.credential_scope}</span>
              </div>
            </div>
            <div className="split-panel embedded-split">
              <label>
                Structured Result
                <textarea name="validation-sandbox-structured-result" className="debug-textarea compact" value={formatMaybeJSON(lastRuntimeValidationRun.sandbox.structured_result)} readOnly rows={10} />
              </label>
              <label>
                Audit / Projection
                <textarea name="validation-sandbox-audit-projection" className="debug-textarea compact" value={formatMaybeJSON({
                  audit_summary: lastRuntimeValidationRun.sandbox.audit_summary,
                  sandbox_trace: lastRuntimeValidationRun.sandbox_trace,
                  sandbox_projection: lastRuntimeValidationRun.sandbox_projection,
                  validation_mcp_projection: lastRuntimeValidationRun.validation_mcp_projection
                })} readOnly rows={10} />
              </label>
            </div>
          </>
        ) : (
          <div className="info-card">
            <span className="status-label">deterministic validation</span>
            <strong>no run selected</strong>
            <span>生成 Runtime 验证记录后，这里展示本次 MCP / sandbox 结果。</span>
          </div>
        )}
      </section>

      <section className="section-card runtime-readout" data-testid="runtime-readout">
        <div className="section-header">
          <div>
            <h2>Runtime Persistence Readout</h2>
            <p className="section-help">读取 Phase 1 持久化对象和 v2.1 foundation，复核 Eino graph run、step timeline、trace、usage、projection boundary、contract registry 与 system truth active pointer。</p>
          </div>
          <div className="action-row">
            <span className="muted">{runtimeReadLoading ? "读取中…" : runtimeTriggerRunning ? "写入中…" : runtimeFoundationSaveRunning ? "保存中…" : runtimeReadError ? "read unavailable" : `${runtimeRuns.length} runs`}</span>
            <button className="secondary-button" data-testid="runtime-validation-trigger" disabled={runtimeReadLoading || runtimeTriggerRunning || runtimeFoundationSaveRunning} onClick={triggerRuntimeValidationRun} type="button">
              生成 Runtime 验证记录
            </button>
            <button className="secondary-button" data-testid="runtime-foundation-save" disabled={runtimeReadLoading || runtimeFoundationSaveRunning} onClick={saveRuntimeFoundationDrafts} type="button">
              保存 Foundation 编辑
            </button>
            <button className="secondary-button" data-testid="runtime-foundation-refresh" disabled={runtimeReadLoading || runtimeFoundationSaveRunning} onClick={() => withValidationAction(async () => refreshRuntimeRecords())} type="button">
              刷新 Runtime 记录
            </button>
          </div>
        </div>
        {runtimeFoundation ? (
          <>
            <div className="system-meta-grid">
              <div className={runtimeFoundation.contracts.length > 0 ? "info-card success" : "info-card"}>
                <span className="status-label">Contracts</span>
                <strong>{runtimeFoundation.contracts.length}</strong>
                <span>{runtimeFoundation.contracts[0] ? `${runtimeFoundation.contracts[0].name} · ${runtimeFoundation.contracts[0].version}` : "no runtime contract"}</span>
              </div>
              <div className={runtimeFoundation.task_types.length > 0 ? "info-card success" : "info-card"}>
                <span className="status-label">Task Types</span>
                <strong>{runtimeFoundation.task_types.length}</strong>
                <span>{runtimeFoundation.task_types[0]?.type_key || "no task type registry"}</span>
              </div>
              <div className={runtimeFoundation.hook_bindings.length > 0 ? "info-card success" : "info-card"}>
                <span className="status-label">Hook Bindings</span>
                <strong>{runtimeFoundation.hook_bindings.length}</strong>
                <span>{runtimeFoundation.hook_bindings[0] ? `${runtimeFoundation.hook_bindings[0].hook_point} · ${runtimeFoundation.hook_bindings[0].binding_ref}` : "no hook binding"}</span>
              </div>
              <div className={runtimeFoundation.active_system_truths.length > 0 ? "info-card success" : "info-card"}>
                <span className="status-label">System Truth</span>
                <strong>{runtimeFoundation.active_system_truths.length}</strong>
                <span>{runtimeFoundation.active_system_truths[0]?.asset_id || "no active pointer"}</span>
              </div>
            </div>
            <div className="split-panel embedded-split">
              <label>
                Contract Foundation Snapshot
                <textarea
                  name="runtime-contract-foundation"
                  className="debug-textarea compact"
                  data-testid="runtime-contract-foundation"
                  value={formatMaybeJSON({
                    contracts: runtimeFoundation.contracts,
                    task_types: runtimeFoundation.task_types,
                    hook_bindings: runtimeFoundation.hook_bindings,
                    active_system_truths: runtimeFoundation.active_system_truths
                  })}
                  readOnly
                  rows={10}
                />
              </label>
              <label>
                Foundation Capability Surface
                <textarea
                  name="runtime-foundation-capabilities"
                  className="debug-textarea compact"
                  data-testid="runtime-foundation-capabilities"
                  value={formatMaybeJSON({
                    store_capabilities: runtimeFoundation.store_capabilities,
                    unavailable_surfaces: runtimeFoundation.unavailable_surfaces ?? []
                  })}
                  readOnly
                  rows={10}
                />
              </label>
            </div>
            <div className="split-panel embedded-split">
              <label>
                Runtime Contract Editor
                <textarea
                  name="runtime-contract-editor"
                  className="debug-textarea compact"
                  data-testid="runtime-contract-editor"
                  onChange={(event) => setRuntimeContractDraft(event.target.value)}
                  rows={12}
                  value={runtimeContractDraft}
                />
              </label>
              <label>
                Task Type / Hook Editor
                <textarea
                  name="runtime-task-type-editor"
                  className="debug-textarea compact"
                  data-testid="runtime-task-type-editor"
                  onChange={(event) => setRuntimeTaskTypeDraft(event.target.value)}
                  rows={5}
                  value={runtimeTaskTypeDraft}
                />
                <textarea
                  name="runtime-hook-binding-editor"
                  className="debug-textarea compact"
                  data-testid="runtime-hook-binding-editor"
                  onChange={(event) => setRuntimeHookBindingDraft(event.target.value)}
                  rows={6}
                  value={runtimeHookBindingDraft}
                />
              </label>
            </div>
          </>
        ) : null}
        {runtimeReadError ? (
          <div className="info-card error">
            <span className="status-label">runtime store</span>
            <strong>read surface unavailable</strong>
            <span>{runtimeReadError}</span>
          </div>
        ) : runtimeRuns.length === 0 ? (
          <div className="info-card">
            <span className="status-label">runtime runs</span>
            <strong>no persisted runs</strong>
            <span>执行一次真实试运行后，如果后端配置了 runtime persistence store，这里会展示最新 run。</span>
          </div>
        ) : (
          <>
            <div className="runtime-run-strip">
              {runtimeRuns.map((item) => (
                <button className={item.id === selectedRunID ? "runtime-run-chip active" : "runtime-run-chip"} key={item.id} onClick={() => selectRuntimeRun(item.id)} type="button">
                  <span>{item.status}</span>
                  <strong>{item.task_type || item.task_id || item.id}</strong>
                  <small>{formatRuntimeTime(item.created_at)}</small>
                </button>
              ))}
            </div>
            <div className="system-meta-grid">
              <div className={runtimeStatusClass(runtimeRun?.status)}>
                <span className="status-label">Runtime Run</span>
                <strong>{runtimeRun?.status || "-"}</strong>
                <span>{runtimeRun?.id || selectedRunID || "-"}</span>
              </div>
              <div className="info-card">
                <span className="status-label">Step Timeline</span>
                <strong>{runtimeSteps.length} steps</strong>
                <span>{runtimeSteps.map((item) => item.name || item.step_type || item.id).slice(0, 3).join(" / ") || "no steps"}</span>
              </div>
              <div className="info-card">
                <span className="status-label">Trace / Usage</span>
                <strong>{runtimeTraces.length} traces · {runtimeUsage.length} usage</strong>
                <span>{summarizeRuntimeUsage(runtimeUsage)}</span>
              </div>
              <div className="info-card">
                <span className="status-label">Projection</span>
                <strong>{runtimeProjections.length} candidates</strong>
                <span>{runtimeProjectionLabel(runtimeProjections[0])}</span>
              </div>
              <div className={runtimeCheckpoints.some((item) => item.snapshot_available) ? "info-card success" : "info-card"} data-testid="runtime-checkpoint-readout">
                <span className="status-label">Checkpoint</span>
                <strong>{runtimeCheckpoints.length} safe refs</strong>
                <span>{summarizeRuntimeCheckpoints(runtimeCheckpoints)}</span>
              </div>
            </div>
            <div className="runtime-timeline">
              {runtimeSteps.map((step) => (
                <div className="runtime-timeline-row" key={step.id}>
                  <span className="runtime-sequence">{step.sequence}</span>
                  <div>
                    <strong>{step.name || step.step_type || step.id}</strong>
                    <span>{step.status} · {step.step_type || "step"} · {runtimeStepLifecycleCount(runtimeLifecycleEvents, step.id)} events</span>
                  </div>
                  <small>{formatRuntimeTime(step.updated_at || step.created_at)}</small>
                </div>
              ))}
            </div>
            {runtimeCheckpoints.length > 0 ? (
              <div className="runtime-record-list">
                <div className="section-header">
                  <h3>Checkpoint Safe Metadata</h3>
                  <span className="muted">payload hidden</span>
                </div>
                {runtimeCheckpoints.map((checkpoint) => (
                  <div className="runtime-record-row" key={checkpoint.checkpoint_id}>
                    <span className="status-label">{checkpoint.stage || "checkpoint"}</span>
                    <strong>{checkpoint.payload_sha256 ? shortHash(checkpoint.payload_sha256) : checkpoint.snapshot_available ? "snapshot" : "metadata ref"}</strong>
                    <span>{checkpoint.payload_size ? `${checkpoint.payload_size} bytes` : "payload size hidden"} · {checkpoint.resume_token_present ? "resume token present" : "no resume token"} · {checkpoint.source || "runtime"}</span>
                  </div>
                ))}
              </div>
            ) : null}
            <div className="split-panel embedded-split">
              <div className="runtime-record-list">
                <div className="section-header">
                  <h3>Trace Summary</h3>
                  <span className="muted">{runtimeTraces.length}</span>
                </div>
                {runtimeTraces.slice(0, 8).map((trace) => (
                  <div className="runtime-record-row" key={trace.id}>
                    <span className="status-label">{trace.trace_type || "trace"}</span>
                    <strong>{trace.summary}</strong>
                    <span>{runtimeTraceLabel(trace)}</span>
                  </div>
                ))}
              </div>
              <div className="runtime-record-list">
                <div className="section-header">
                  <h3>Usage / Projection</h3>
                  <span className="muted">{runtimeUsage.length} / {runtimeProjections.length}</span>
                </div>
                {runtimeUsage.slice(0, 5).map((usage) => (
                  <div className="runtime-record-row" key={usage.id}>
                    <span className="status-label">{usage.resource_type}</span>
                    <strong>{formatRuntimeAmount(usage.amount)} {usage.unit}</strong>
                    <span>{[usage.provider, usage.resource_name].filter(Boolean).join(" / ") || "generic resource"}</span>
                  </div>
                ))}
                {runtimeProjections.slice(0, 3).map((projection) => (
                  <div className="runtime-record-row" key={projection.id}>
                    <span className="status-label">{projection.candidate_kind}</span>
                    <strong>{projection.status || "candidate"}</strong>
                    <span>{runtimeProjectionLabel(projection)}</span>
                  </div>
                ))}
              </div>
            </div>
          </>
        )}
      </section>

      <section className="section-card">
        <div className="section-header">
          <div>
            <h2>Source / Active / Compiled 取样</h2>
            <p className="section-help">选择一条 system resource 后，页面会读取源码、parse 结果、compile 结果和真实调试 payload。</p>
          </div>
          <span className="muted">truth version: {truthDir?.version || "unknown"}</span>
        </div>
        {items.length === 0 ? (
          <div className="info-card error">当前没有加载到 system resources，先执行刷新或同步。</div>
        ) : (
          <>
            <label>
              验证资源
              <select name="validation-resource" value={selectedAssetID} onChange={(event) => setSelectedAssetID(event.target.value)}>
                {items.map((item) => (
                  <option key={item.asset_id} value={item.asset_id}>
                    {item.asset_id} · {item.status || "unknown"}
                  </option>
                ))}
              </select>
            </label>
            <div className="system-meta-grid">
              <div className="info-card">
                <span className="status-label">asset</span>
                <strong>{detail?.asset_id || selectedItem?.asset_id || "-"}</strong>
                <span>{detail?.asset_type || selectedItem?.asset_type || "unknown"}</span>
              </div>
              <div className="info-card">
                <span className="status-label">source</span>
                <strong>{validationSourceFileLabel(detail || selectedItem)}</strong>
                <span>{detail?.source_path || selectedItem?.source_path || "未暴露 source_path"}</span>
              </div>
              <div className="info-card">
                <span className="status-label">pipeline</span>
                <strong>{pipeline?.status || detail?.pipeline?.status || detail?.status || selectedItem?.status || "unknown"}</strong>
                <span>{pipeline?.current_step || "-"}</span>
              </div>
              <div className="info-card">
                <span className="status-label">compiled</span>
                <strong>{compileResult?.status || detail?.compile_result?.status || "unknown"}</strong>
                <span>{compileResult?.compiled_version || detail?.compiled_version || selectedItem?.compiled_version || "no compiled version"}</span>
              </div>
            </div>
            <label>
              Source Content
              <textarea name="validation-source-content" className="debug-textarea" value={source?.source_content ?? ""} readOnly rows={12} />
            </label>
            <div className="split-panel embedded-split">
              <label>
                Parse Result
                <textarea name="validation-parse-result" className="debug-textarea compact" value={formatMaybeJSON(parseResult || detail?.parse_result)} readOnly rows={10} />
              </label>
              <label>
                Compile Result
                <textarea name="validation-compile-result" className="debug-textarea compact" value={formatMaybeJSON(compileResult || detail?.compile_result)} readOnly rows={10} />
              </label>
            </div>
            <div className="split-panel embedded-split">
              <label>
                Debug Payload 实际请求体
                <textarea name="validation-debug-request-body" className="debug-textarea compact" value={formatMaybeJSON(debugPayload?.payload)} readOnly rows={10} />
              </label>
              <label>
                Debug Payload 外层包装
                <textarea name="validation-debug-wrapper" className="debug-textarea compact" value={formatMaybeJSON(debugPayload)} readOnly rows={10} />
              </label>
            </div>
          </>
        )}
      </section>

      <section className="split-panel">
        <div className="section-card">
          <div className="section-header">
            <h2>单条 Prompt 真实试运行</h2>
            <span className="muted">POST /api/runtime/respond</span>
          </div>
          <label>
            Scene
            <select name="validation-scene" value={selectedSceneID} onChange={(event) => setSelectedSceneID(event.target.value)}>
              {data.scenes.map((item) => (
                <option key={item.id} value={item.id}>{item.id}</option>
              ))}
            </select>
          </label>
          <label>
            Skill
            <select name="validation-skill" value={selectedSkillName} onChange={(event) => setSelectedSkillName(event.target.value)}>
              {selectableSkills.map((item) => (
                <option key={item.name} value={item.name}>{item.name}</option>
              ))}
            </select>
          </label>
          <label>
            Prompt
            <textarea name="validation-prompt" value={promptText} onChange={(event) => setPromptText(event.target.value)} rows={6} />
          </label>
          <label>
            Request Body
            <textarea name="validation-chat-respond-body" className="debug-textarea compact" value={runtimeRespondBody} readOnly rows={10} />
          </label>
          <button className="primary-button" disabled={trialRunning || !selectedSceneID || promptText.trim() === ""} onClick={runTrial} type="button">
            {trialRunning ? "试运行中…" : "执行真实试运行"}
          </button>
        </div>
        <div className="section-card stream-preview-card">
          <div className="section-header">
            <h2>试运行结果</h2>
            <span className="muted">{trialMeta || "尚未执行"}</span>
          </div>
          <textarea name="validation-trial-response" className="debug-textarea" value={trialResponse} readOnly rows={18} />
          <div className="action-row">
            <button className="secondary-button" disabled={!trialResponse} onClick={() => setBaselineText(trialResponse)} type="button">
              设为 Before
            </button>
            <button className="secondary-button" disabled={!trialResponse} onClick={() => setCandidateText(trialResponse)} type="button">
              设为 After
            </button>
          </div>
        </div>
      </section>

      <section className="section-card">
        <div className="section-header">
          <div>
            <h2>优化前后对比</h2>
            <p className="section-help">这里只做可复核文本差异统计，不伪造质量分；Before / After 可粘贴历史响应或使用上面的真实试运行结果。</p>
          </div>
          <span className="muted">{comparison.hasBoth ? (comparison.same ? "内容相同" : "内容不同") : "等待两侧内容"}</span>
        </div>
        <div className="split-panel embedded-split">
          <label>
            Before
            <textarea name="validation-before-text" className="debug-textarea" value={baselineText} onChange={(event) => setBaselineText(event.target.value)} rows={12} />
          </label>
          <label>
            After
            <textarea name="validation-after-text" className="debug-textarea" value={candidateText} onChange={(event) => setCandidateText(event.target.value)} rows={12} />
          </label>
        </div>
        <div className="system-meta-grid">
          <div className="info-card">
            <span className="status-label">Before</span>
            <strong>{comparison.beforeCharacters} chars</strong>
            <span>{comparison.beforeLines} lines</span>
          </div>
          <div className="info-card">
            <span className="status-label">After</span>
            <strong>{comparison.afterCharacters} chars</strong>
            <span>{comparison.afterLines} lines</span>
          </div>
          <div className={comparison.hasBoth && !comparison.same ? "info-card success" : "info-card"}>
            <span className="status-label">Delta</span>
            <strong>{comparison.afterCharacters - comparison.beforeCharacters} chars</strong>
            <span>{comparison.afterLines - comparison.beforeLines} lines</span>
          </div>
          <div className="info-card">
            <span className="status-label">Mode</span>
            <strong>Manual Review</strong>
            <span>人工复核 system 内容优化是否更符合预期。</span>
          </div>
        </div>
      </section>
    </section>
  );
}

function ModelPanel({
  items,
  onProviderSave,
  onProviderToggle,
  onProviderDelete,
  onModelSave,
  onModelToggle,
  onModelDelete,
  onModelTest
}: {
  items: ProviderDefinition[];
  onProviderSave: (draft: ProviderDraft) => Promise<void>;
  onProviderToggle: (providerID: string, enabled: boolean) => Promise<void>;
  onProviderDelete: (providerID: string) => Promise<void>;
  onModelSave: (providerID: string, draft: ModelDraft) => Promise<void>;
  onModelToggle: (providerID: string, recordID: string, patch: Partial<Pick<ModelDraft, "enabled" | "is_default" | "is_fallback">>) => Promise<void>;
  onModelDelete: (providerID: string, recordID: string) => Promise<void>;
  onModelTest: (providerID: string, recordID: string) => Promise<ModelTestResult>;
}) {
  const [selectedProviderID, setSelectedProviderID] = useState<string>(items[0]?.id ?? "__new__");
  const [selectedModelID, setSelectedModelID] = useState<string>("__new__");
  const [providerDraft, setProviderDraft] = useState<ProviderDraft>(items[0] ? toProviderDraft(items[0]) : emptyProviderDraft);
  const [modelDraft, setModelDraft] = useState<ModelDraft>(emptyModelDraft);
  const [lastTest, setLastTest] = useState<ModelTestResult | null>(null);

  const selectedProvider = items.find((item) => item.id === selectedProviderID) ?? null;
  const models = selectedProvider?.models ?? [];
  const selectedModel = models.find((item) => item.id === selectedModelID) ?? null;
  const modelRefs = items.flatMap((provider) => (provider.models ?? []).map((model) => ({ provider, model })));
  const enabledProviders = items.filter((item) => item.enabled);
  const enabledModelRefs = modelRefs.filter((item) => item.provider.enabled && item.model.enabled);
  const defaultModelRefs = enabledModelRefs.filter((item) => item.model.is_default);
  const fallbackModelRefs = enabledModelRefs.filter((item) => item.model.is_fallback);
  const recommendedDefaultRef = enabledModelRefs[0] ?? modelRefs[0] ?? null;

  useEffect(() => {
    if (items.length === 0) {
      setSelectedProviderID("__new__");
      return;
    }
    if (selectedProviderID === "__new__") {
      return;
    }
    if (!items.some((item) => item.id === selectedProviderID)) {
      setSelectedProviderID(items[0].id);
    }
  }, [items, selectedProviderID]);

  useEffect(() => {
    if (selectedProviderID === "__new__") {
      setProviderDraft(emptyProviderDraft);
      setSelectedModelID("__new__");
      setModelDraft(emptyModelDraft);
      setLastTest(null);
      return;
    }
    if (selectedProvider) {
      setProviderDraft(toProviderDraft(selectedProvider));
      setSelectedModelID(selectedProvider.models?.[0]?.id ?? "__new__");
      setLastTest(null);
    }
  }, [selectedProviderID, selectedProvider]);

  useEffect(() => {
    if (!selectedModel) {
      setModelDraft(emptyModelDraft);
      return;
    }
    setModelDraft(toModelDraft(selectedModel));
    setLastTest(null);
  }, [selectedModel]);

  return (
    <section className="panel split-panel">
      <div className="list-pane">
        <button className={selectedProviderID === "__new__" ? "list-item active" : "list-item"} onClick={() => setSelectedProviderID("__new__")} type="button">
          <strong>+ 新建 Provider</strong>
          <span>创建新的模型供应商</span>
        </button>
        {items.map((item) => (
          <button
            key={item.id}
            className={item.id === selectedProviderID ? "list-item active" : "list-item"}
            onClick={() => setSelectedProviderID(item.id)}
            type="button"
          >
            <strong>{item.name}</strong>
            <span>
              {item.protocol} · {item.enabled ? "enabled" : "disabled"} · {(item.models ?? []).length} models
            </span>
          </button>
        ))}
      </div>
      <div className="editor-pane">
        <div className={defaultModelRefs.length > 0 ? "section-card provider-readiness-card" : "section-card provider-readiness-card warning"}>
          <div className="section-header">
            <div>
              <h2>Provider Readiness</h2>
              <p className="section-help">Release Readiness 会把缺少 default model 作为成品 warning；这里可以直接修复。</p>
            </div>
            <span className="muted">{enabledProviders.length} providers · {enabledModelRefs.length} enabled models</span>
          </div>
          <div className="system-meta-grid">
            <div className={defaultModelRefs.length > 0 ? "info-card success" : "info-card warning"}>
              <span className="status-label">Default Model</span>
              <strong>{defaultModelRefs[0]?.model.display_name || defaultModelRefs[0]?.model.model_id || "missing"}</strong>
              <span>{defaultModelRefs[0]?.provider.name || "需要选择一个默认模型"}</span>
            </div>
            <div className="info-card">
              <span className="status-label">Fallback Models</span>
              <strong>{fallbackModelRefs.length}</strong>
              <span>{fallbackModelRefs[0]?.model.display_name || fallbackModelRefs[0]?.model.model_id || "optional"}</span>
            </div>
          </div>
          {defaultModelRefs.length === 0 && recommendedDefaultRef ? (
            <button
              className="secondary-button"
              onClick={() => onModelToggle(recommendedDefaultRef.provider.id, recommendedDefaultRef.model.id, { enabled: true, is_default: true, is_fallback: false })}
              type="button"
            >
              设为 default: {recommendedDefaultRef.model.display_name || recommendedDefaultRef.model.model_id}
            </button>
          ) : null}
        </div>

        <div className="section-card">
          <div className="section-header">
            <h2>{selectedProvider ? "编辑 Provider" : "新建 Provider"}</h2>
            {selectedProvider ? (
              <div className="action-row">
                <button className="secondary-button" onClick={() => onProviderToggle(selectedProvider.id, !selectedProvider.enabled)} type="button">
                  {selectedProvider.enabled ? "禁用 Provider" : "启用 Provider"}
                </button>
                <button className="danger-button" onClick={() => onProviderDelete(selectedProvider.id)} type="button">
                  删除 Provider
                </button>
              </div>
            ) : null}
          </div>
          <label>
            名称
            <input name="provider-name" value={providerDraft.name} onChange={(event) => setProviderDraft({ ...providerDraft, name: event.target.value })} />
          </label>
          <label>
            协议
            <select name="provider-protocol" value={providerDraft.protocol} onChange={(event) => setProviderDraft({ ...providerDraft, protocol: event.target.value })}>
              <option value="openai_compatible">openai_compatible</option>
              <option value="ark">ark</option>
              <option value="anthropic">anthropic</option>
            </select>
          </label>
          <label>
            Base URL
            <input name="provider-base-url" value={providerDraft.base_url} onChange={(event) => setProviderDraft({ ...providerDraft, base_url: event.target.value })} />
          </label>
          <label>
            API Key
            <input
              name="provider-api-key"
              placeholder={selectedProvider?.api_key_masked || "留空则保留现有密钥"}
              value={providerDraft.api_key}
              onChange={(event) => setProviderDraft({ ...providerDraft, api_key: event.target.value })}
            />
          </label>
          <label>
            Timeout Seconds
            <input
              name="provider-timeout-seconds"
              type="number"
              value={providerDraft.request_timeout_seconds}
              onChange={(event) => setProviderDraft({ ...providerDraft, request_timeout_seconds: Number(event.target.value) })}
            />
          </label>
          <label>
            Headers
            <textarea
              name="provider-headers"
              placeholder={"Accept-Encoding: identity\nX-Trace: demo"}
              value={providerDraft.headers_text}
              onChange={(event) => setProviderDraft({ ...providerDraft, headers_text: event.target.value })}
            />
          </label>
          <label className="inline-toggle">
            <input name="provider-enabled" checked={providerDraft.enabled} onChange={(event) => setProviderDraft({ ...providerDraft, enabled: event.target.checked })} type="checkbox" />
            启用 Provider
          </label>
          <button className="primary-button" onClick={() => onProviderSave(providerDraft)} type="button">
            {selectedProvider ? "保存 Provider" : "创建 Provider"}
          </button>
        </div>

        {selectedProvider ? (
          <div className="section-card">
            <div className="section-header">
              <h2>模型列表</h2>
              <button className="secondary-button" onClick={() => setSelectedModelID("__new__")} type="button">
                + 新建模型
              </button>
            </div>
            <div className="list-pane nested-list">
              {models.map((item) => (
                <button
                  key={item.id}
                  className={item.id === selectedModelID ? "list-item active" : "list-item"}
                  onClick={() => setSelectedModelID(item.id)}
                  type="button"
                >
                  <strong>{item.display_name}</strong>
                  <span>
                    {item.model_id} · {item.enabled ? "enabled" : "disabled"}
                    {item.is_default ? " · default" : ""}
                    {item.is_fallback ? " · fallback" : ""}
                  </span>
                </button>
              ))}
            </div>
          </div>
        ) : null}

        {selectedProvider ? (
          <div className="section-card">
            <div className="section-header">
              <h2>{selectedModel ? "编辑模型" : "新建模型"}</h2>
              {selectedModel ? (
                <div className="action-row">
                  <button
                    className="secondary-button"
                    onClick={() => onModelToggle(selectedProvider.id, selectedModel.id, { enabled: !selectedModel.enabled })}
                    type="button"
                  >
                    {selectedModel.enabled ? "禁用模型" : "启用模型"}
                  </button>
                  <button className="danger-button" onClick={() => onModelDelete(selectedProvider.id, selectedModel.id)} type="button">
                    删除模型
                  </button>
                </div>
              ) : null}
            </div>
            <label>
              Model ID
              <input name="model-id" value={modelDraft.model_id} onChange={(event) => setModelDraft({ ...modelDraft, model_id: event.target.value })} />
            </label>
            <label>
              Display Name
              <input name="model-display-name" value={modelDraft.display_name} onChange={(event) => setModelDraft({ ...modelDraft, display_name: event.target.value })} />
            </label>
            <label className="inline-toggle">
              <input name="model-enabled" checked={modelDraft.enabled} onChange={(event) => setModelDraft({ ...modelDraft, enabled: event.target.checked })} type="checkbox" />
              启用模型
            </label>
            <label className="inline-toggle">
              <input name="model-default" checked={modelDraft.is_default} onChange={(event) => setModelDraft({ ...modelDraft, is_default: event.target.checked, is_fallback: event.target.checked ? false : modelDraft.is_fallback })} type="checkbox" />
              设为 default
            </label>
            <label className="inline-toggle">
              <input name="model-fallback" checked={modelDraft.is_fallback} onChange={(event) => setModelDraft({ ...modelDraft, is_fallback: event.target.checked, is_default: event.target.checked ? false : modelDraft.is_default })} type="checkbox" />
              设为 fallback
            </label>
            <div className="action-row">
              <button className="primary-button" onClick={() => onModelSave(selectedProvider.id, modelDraft)} type="button">
                {selectedModel ? "保存模型" : "创建模型"}
              </button>
              {selectedModel ? (
                <button
                  className="secondary-button"
                  onClick={async () => {
                    const result = await onModelTest(selectedProvider.id, selectedModel.id);
                    setLastTest(result);
                  }}
                  type="button"
                >
                  测试可用性
                </button>
              ) : null}
            </div>
            {lastTest ? (
              <div className={lastTest.available ? "info-card success" : "info-card error"}>
                <strong>{lastTest.available ? "测试成功" : "测试失败"}</strong>
                <span>
                  {lastTest.display_name} · {lastTest.duration_ms}ms
                </span>
                {lastTest.error ? <code>{lastTest.error}</code> : null}
              </div>
            ) : null}
          </div>
        ) : null}
      </div>
    </section>
  );
}

function GovernancePanel({ value, onSave }: { value: GovernanceConfig; onSave: (value: GovernanceConfig) => Promise<void> }) {
  const [draft, setDraft] = useState<GovernanceConfig>(value);

  useEffect(() => {
    setDraft(value);
  }, [value]);

  return (
    <section className="panel editor-pane">
      <label className="inline-toggle">
        <input name="governance-choice-required" checked={draft.choice_required_enabled} onChange={(event) => setDraft({ ...draft, choice_required_enabled: event.target.checked })} type="checkbox" />
        启用 choice_required
      </label>
      <label className="inline-toggle">
        <input
          name="governance-automation-fallback"
          checked={draft.automation_fallback_enabled}
          onChange={(event) => setDraft({ ...draft, automation_fallback_enabled: event.target.checked })}
          type="checkbox"
        />
        启用 automation fallback
      </label>
      <label className="inline-toggle">
        <input
          name="governance-planning-progress"
          checked={draft.planning_progress_enabled}
          onChange={(event) => setDraft({ ...draft, planning_progress_enabled: event.target.checked })}
          type="checkbox"
        />
        启用 planning progress
      </label>
      <label className="inline-toggle">
        <input
          name="governance-fact-quality-gate"
          checked={draft.fact_quality_gate_enabled}
          onChange={(event) => setDraft({ ...draft, fact_quality_gate_enabled: event.target.checked })}
          type="checkbox"
        />
        启用 fact quality gate
      </label>
      <label className="inline-toggle">
        <input
          name="governance-tool-hint-emission"
          checked={draft.tool_hint_emission_enabled}
          onChange={(event) => setDraft({ ...draft, tool_hint_emission_enabled: event.target.checked })}
          type="checkbox"
        />
        启用 tool hints 发射
      </label>
      <label className="inline-toggle">
        <input
          name="governance-knowledge-retrieval-emission"
          checked={draft.knowledge_retrieval_emission_enabled}
          onChange={(event) => setDraft({ ...draft, knowledge_retrieval_emission_enabled: event.target.checked })}
          type="checkbox"
        />
        启用 knowledge retrieval 发射
      </label>
      <label>
        Max Planning Steps
        <input
          name="governance-max-planning-steps"
          type="number"
          value={draft.max_planning_steps}
          onChange={(event) => setDraft({ ...draft, max_planning_steps: Number(event.target.value) })}
        />
      </label>
      <label>
        Max Tool Hints
        <input
          name="governance-max-tool-hints"
          type="number"
          value={draft.max_tool_hints}
          onChange={(event) => setDraft({ ...draft, max_tool_hints: Number(event.target.value) })}
        />
      </label>
      <button className="primary-button" onClick={() => onSave(draft)} type="button">
        保存治理策略
      </button>
    </section>
  );
}

function VersionPanel({
  items,
  detail,
  onSelect,
  onRollback
}: {
  items: BootstrapPayload["config_versions"];
  detail: ConfigVersionDetail | null;
  onSelect: (versionID: string) => Promise<void>;
  onRollback: (versionID: string) => Promise<void>;
}) {
  return (
    <section className="panel split-panel">
      <div className="list-pane">
        {items.length === 0 ? <div className="list-item">当前还没有配置版本。</div> : null}
        {items.map((item) => (
          <button key={item.version_id} className="list-item" onClick={() => onSelect(item.version_id)} type="button">
            <strong>{item.version_id}</strong>
            <span>{item.summary || item.created_at}</span>
          </button>
        ))}
      </div>
      <div className="editor-pane">
        {!detail ? <div>请选择一个配置版本查看详情。</div> : null}
        {detail ? (
          <>
            <label>
              Version ID
              <input name="config-version-id" value={detail.version_id} readOnly />
            </label>
            <label>
              Summary
              <input name="config-version-summary" value={detail.summary ?? ""} readOnly />
            </label>
            <label>
              Created At
              <input name="config-version-created-at" value={detail.created_at} readOnly />
            </label>
            <label>
              Document Preview
              <textarea name="config-version-document-preview" value={JSON.stringify(detail.document, null, 2)} readOnly rows={16} />
            </label>
            <button className="primary-button" onClick={() => onRollback(detail.version_id)} type="button">
              回滚到此版本
            </button>
          </>
        ) : null}
      </div>
    </section>
  );
}

function APIDebugPanel({
  items,
  preset,
  onPresetConsumed,
  onReload
}: {
  items: OpenAPIEndpoint[];
  preset: APIDebugPreset | null;
  onPresetConsumed: () => void;
  onReload: () => Promise<void>;
}) {
  const [selectedKey, setSelectedKey] = useState(items[0]?.key ?? "");
  const [filter, setFilter] = useState("");
  const [pathParamsText, setPathParamsText] = useState("{}");
  const [queryText, setQueryText] = useState("");
  const [bodyText, setBodyText] = useState("");
  const [rawResultBody, setRawResultBody] = useState("");
  const [resultText, setResultText] = useState("");
  const [responseMeta, setResponseMeta] = useState("");
  const [seededKey, setSeededKey] = useState("");
  const filtered = items.filter((item) => {
    const needle = filter.trim().toLowerCase();
    if (needle === "") {
      return true;
    }
    return item.key.toLowerCase().includes(needle) || item.summary.toLowerCase().includes(needle);
  });
  const selected = filtered.find((item) => item.key === selectedKey) ?? items.find((item) => item.key === selectedKey) ?? filtered[0] ?? items[0] ?? null;
  const quickPresets = buildP0APIDebugPresets(items);
  const streamDebugEnabled = isChatStreamEndpoint(selected);
  const streamTranscript = streamDebugEnabled ? buildStreamTranscript(rawResultBody) : [];

  useEffect(() => {
    if (!preset) {
      return;
    }
    const target = items.find((item) => item.key === preset.endpointKey);
    if (!target) {
      return;
    }
    setSelectedKey(target.key);
    setPathParamsText(preset.pathParamsText ?? buildDefaultParamsJSON(target.path));
    setQueryText(preset.queryText ?? "");
    setBodyText(preset.bodyText ?? defaultRequestBody(target));
    setRawResultBody("");
    setResultText("");
    setResponseMeta("");
    setSeededKey(target.key);
    onPresetConsumed();
  }, [items, onPresetConsumed, preset]);

  useEffect(() => {
    if (preset) {
      return;
    }
    if (!selected) {
      setSelectedKey("");
      return;
    }
    setSelectedKey(selected.key);
    if (seededKey === selected.key) {
      setSeededKey("");
      return;
    }
    setPathParamsText(buildDefaultParamsJSON(selected.path));
    setQueryText("");
    setBodyText(defaultRequestBody(selected));
    setRawResultBody("");
    setResultText("");
    setResponseMeta("");
  }, [preset, seededKey, selected?.key]);

  function applyPreset(nextPreset: APIDebugPreset) {
    const target = items.find((item) => item.key === nextPreset.endpointKey);
    if (!target) {
      return;
    }
    setSelectedKey(target.key);
    setPathParamsText(nextPreset.pathParamsText ?? buildDefaultParamsJSON(target.path));
    setQueryText(nextPreset.queryText ?? "");
    setBodyText(nextPreset.bodyText ?? defaultRequestBody(target));
    setRawResultBody("");
    setResultText("");
    setResponseMeta("");
    setSeededKey(target.key);
  }

  async function handleInvoke() {
    if (!selected) {
      return;
    }
    const pathParams = safeParseJSON(pathParamsText);
    const resolvedPath = applyPathParams(selected.path, pathParams);
    const finalPath = appendQueryString(resolvedPath, queryText);
    const result = await invokeDebugEndpoint(selected.method, finalPath, bodyText);
    setResponseMeta(`${selected.method} ${finalPath} -> ${result.status} ${result.statusText}`);
    setRawResultBody(result.body);
    setResultText(formatResponseBody(result.body));
  }

  return (
    <section className="panel split-panel">
      <div className="list-pane">
        <label>
          过滤
          <input name="api-debug-filter" value={filter} onChange={(event) => setFilter(event.target.value)} />
        </label>
        <button className="secondary-button" onClick={() => onReload()} type="button">
          刷新 OpenAPI
        </button>
        <div className="section-card stream-preview-card">
          <strong>P0 常用接口</strong>
          <p className="section-help">从这里直接切换到 P0 验收相关请求体。</p>
          {quickPresets.map((item) => (
            <button
              key={item.preset.endpointKey}
              className="secondary-button"
              disabled={!item.available}
              onClick={() => applyPreset(item.preset)}
              type="button"
            >
              {item.label}
            </button>
          ))}
        </div>
        {filtered.map((item) => (
          <button
            key={item.key}
            className={item.key === selectedKey ? "list-item active" : "list-item"}
            onClick={() => setSelectedKey(item.key)}
            type="button"
          >
            <strong>{item.method}</strong>
            <span>{item.path}</span>
            {item.summary ? <span>{item.summary}</span> : null}
          </button>
        ))}
      </div>
      <div className="editor-pane">
        {!selected ? <div>当前没有可调试的接口。</div> : null}
        {selected ? (
          <div className="section-card">
            <div className="section-header">
              <h2>{selected.method} {selected.path}</h2>
            </div>
            {selected.summary ? <p className="muted">{selected.summary}</p> : null}
            <label>
              Path Params JSON
              <textarea name="api-debug-path-params" value={pathParamsText} onChange={(event) => setPathParamsText(event.target.value)} />
            </label>
            <label>
              Query String
              <input name="api-debug-query-string" placeholder="page=1&size=20" value={queryText} onChange={(event) => setQueryText(event.target.value)} />
            </label>
            <label>
              Request Body
              <textarea name="api-debug-request-body" value={bodyText} onChange={(event) => setBodyText(event.target.value)} />
            </label>
            <div className="action-row">
              <button className="primary-button" onClick={() => handleInvoke()} type="button">
                发送请求
              </button>
            </div>
            {responseMeta ? <div className="info-card success"><strong>{responseMeta}</strong></div> : null}
            {streamDebugEnabled ? (
              <div className="section-card stream-preview-card">
                <div className="section-header">
                  <h3>对话预览</h3>
                  <span className="muted">按 progress_step / message / stream_chunk / done 渲染</span>
                </div>
                {streamTranscript.length === 0 ? (
                  <p className="muted">发送 `/api/chat/stream` 请求后，这里会把 SSE 过程流翻译成可读对话。</p>
                ) : (
                  <div className="stream-preview">
                    {streamTranscript.map((item, index) => (
                      <div className={`stream-bubble ${item.kind}`} key={`${item.kind}-${index}`}>
                        <strong>{item.title}</strong>
                        <span>{item.text}</span>
                        {item.status ? <small>状态：{item.status}</small> : null}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            ) : null}
            <label>
              {streamDebugEnabled ? "原始 SSE 响应" : "Response"}
              <textarea name="api-debug-response" className="debug-textarea" value={resultText} readOnly rows={18} />
            </label>
          </div>
        ) : null}
      </div>
    </section>
  );
}

function SwaggerPanel({ specURL }: { specURL: string }) {
  return (
    <section className="panel swagger-panel">
      <Suspense fallback={<div>正在加载 Swagger…</div>}>
        <SwaggerUI url={specURL} />
      </Suspense>
    </section>
  );
}

function toSystemResourceDraft(detail: SystemResourceDetail, source: SystemResourceSource | null): SystemResourceDraft {
  return {
    asset_id: detail.asset_id,
    asset_type: detail.asset_type || "",
    asset_name: detail.asset_name || "",
    scope: detail.scope || "system",
    source_kind: detail.source_kind || "control_plane_upload",
    read_only: !!detail.read_only,
    metadata_text: JSON.stringify(detail.metadata ?? {}, null, 2),
    source_content: source?.source_content || "",
    message: source?.message || ""
  };
}

function parseSystemResourceCreateMode(selection: string): SystemResourceCreateMode | null {
  if (!selection.startsWith("__create__:")) {
    return null;
  }
  return createModeForAssetType(selection.replace("__create__:", "").trim());
}

function createModeForAssetType(assetType: string): SystemResourceCreateMode | null {
  switch (assetType.trim()) {
    case "policy_rule":
      return {
        assetType: "policy_rule",
        title: "新增 Policy Rule",
        description: "新增一份系统规则主源。创建后会进入 core 或 scene 的 policy_rule 路径并执行 parse / compile / activate。",
        createLabel: "新增规则",
        sourceFolder: "sources/core/policy_rule/",
        defaultSlug: "new-rule"
      };
    case "tool_governance_policy":
      return {
        assetType: "tool_governance_policy",
        title: "新增 Tool Governance Policy",
        description: "新增一份 tool 执行前治理策略主源。创建后会写入 core/tool_governance_policy 并执行 parse / compile / activate。",
        createLabel: "新增 Tool Governance",
        sourceFolder: "sources/core/tool_governance_policy/",
        defaultSlug: "default"
      };
    case "scene":
      return {
        assetType: "scene",
        title: "新增 Scene",
        description: "新增一份 scene 定义主源。创建后会直接写入 scenes/<scene_id>/SCENE.md 并进入 parse / compile / activate。",
        createLabel: "新增场景",
        sourceFolder: "sources/scenes/<scene_id>/",
        defaultSlug: "new_scene"
      };
    case "workflow":
      return {
        assetType: "workflow",
        title: "新增 Workflow",
        description: "新增一份 scene workflow 主源。创建后会写入 scenes/<scene_id>/workflow.yaml 并进入 parse / compile / activate。",
        createLabel: "新增 Workflow",
        sourceFolder: "sources/scenes/<scene_id>/",
        defaultSlug: "new_scene"
      };
    case "contract":
      return {
        assetType: "contract",
        title: "新增 Contract",
        description: "新增一份结构契约主源。创建后会写入 scenes/<scene_id>/contract/ 并进入 parse / compile / activate。",
        createLabel: "新增 Contract",
        sourceFolder: "sources/scenes/<scene_id>/contract/",
        defaultSlug: "new_contract"
      };
    case "skill":
      return {
        assetType: "skill",
        title: "新增 Skill Package",
        description: "新增一份标准 skill package 入口。创建后会写入 scenes/<scene_id>/skills/<skill_id>/SKILL.md 并进入 parse / compile / activate。",
        createLabel: "新增 Skill",
        sourceFolder: "sources/scenes/<scene_id>/skills/<skill_id>/",
        defaultSlug: "new_skill"
      };
    case "user_profile":
      return {
        assetType: "user_profile",
        title: "新增 User Profile",
        description: "在 user_profile 目录下新增一份用户画像主源。创建后会直接写入 sources/user_profile/ 并进入 parse / compile / activate。",
        createLabel: "新增 Profile",
        sourceFolder: "sources/user_profile/",
        defaultSlug: "new-user"
      };
    case "memory_view":
      return {
        assetType: "memory_view",
        title: "新增 Memory 文件",
        description: "在 memory_view 目录下新增一份记忆主源文件。该目录支持多份 memory 文件，创建后会直接写入 sources/memory_view/ 并进入 parse / compile / activate。",
        createLabel: "新增 Memory",
        sourceFolder: "sources/memory_view/",
        defaultSlug: "memory-entry"
      };
    default:
      return null;
  }
}

function createDraftForMode(mode: SystemResourceCreateMode): SystemResourceDraft {
  return {
    ...emptySystemResourceDraft,
    asset_id: mode.defaultSlug,
    asset_type: mode.assetType,
    asset_name: "",
    scope: "system",
    source_kind: "truth_dir_source",
    metadata_text: JSON.stringify({ managed_by: "system_sources" }, null, 2),
    source_content: defaultSourceTemplate(mode.assetType, mode.defaultSlug),
    message: `create ${mode.assetType}`
  };
}

function buildCreateInput(mode: SystemResourceCreateMode, draft: SystemResourceDraft): SystemResourceCreateInput {
  const slug = normalizeSourceSlug(draft.asset_id || mode.defaultSlug);
  const assetID = previewAssetID(mode.assetType, slug);
  return {
    asset_id: assetID,
    asset_type: mode.assetType,
    asset_name: (draft.asset_name || slug).trim(),
    scope: draft.scope.trim() || "system",
    source_kind: draft.source_kind.trim() || "truth_dir_source",
    read_only: draft.read_only,
    source_content: draft.source_content,
    message: draft.message,
    metadata: parseObjectText(draft.metadata_text)
  };
}

function buildSystemResourceFolderGroups(items: SystemResourceDetail[]): SystemResourceFolderGroup[] {
  const grouped = new Map<string, SystemResourceFolderGroup>();
  for (const item of items) {
    const meta = sourceFolderMeta(item.source_path);
    const current = grouped.get(meta.key);
    if (current) {
      current.items.push(item);
      continue;
    }
    grouped.set(meta.key, {
      key: meta.key,
      label: meta.label,
      description: meta.description,
      assetType: meta.assetType,
      createLabel: meta.createLabel,
      items: [item]
    });
  }
  return Array.from(grouped.values())
    .map((group) => ({
      ...group,
      items: [...group.items].sort((left, right) => {
        return (left.source_path || left.asset_id).localeCompare(right.source_path || right.asset_id);
      })
    }))
    .sort((left, right) => {
      if (left.key === "__root__") {
        return -1;
      }
      if (right.key === "__root__") {
        return 1;
      }
      return left.label.localeCompare(right.label);
    });
}

function sourceFolderMeta(sourcePath?: string) {
  const cleaned = (sourcePath || "").trim().replace(/^sources\//, "");
  if (!cleaned || !cleaned.includes("/")) {
    return {
      key: "__root__",
      label: "root files",
      description: "系统固定入口文件，定义 persona 与 agent 的默认基线。"
    };
  }
  const folder = cleaned.split("/").slice(0, -1).join("/");
  let mode: SystemResourceCreateMode | null = null;
  if (folder === "core/policy_rule") {
    mode = createModeForAssetType("policy_rule");
  } else if (folder === "core/tool_governance_policy") {
    mode = createModeForAssetType("tool_governance_policy");
  } else if (folder === "core/user_profile") {
    mode = createModeForAssetType("user_profile");
  } else if (folder === "core/memory_view") {
    mode = createModeForAssetType("memory_view");
  }
  const descriptions: Record<string, string> = {
    core: "系统级唯一真相根目录，包含墨思的 persona、agent profile 与跨场景基线。",
    "core/policy_rule": "跨场景通用规则与治理边界，约束 Athena 的行为和输出底线。",
    "core/tool_governance_policy": "runtime tool 执行前治理策略，定义 allow / deny / redaction / sandbox 决策。",
    "core/user_profile": "默认用户画像基线，在没有显式绑定时作为长期上下文输入。",
    "core/memory_view": "默认记忆视图目录，承载可读的长期记忆视图。",
  };
  if (folder.startsWith("scenes/") && folder.endsWith("/contract")) {
    descriptions[folder] = "当前 scene 的结构契约目录，定义输出、evidence 与完成条件。";
  }
  if (folder.startsWith("scenes/") && folder.endsWith("/policy_rule")) {
    descriptions[folder] = "当前 scene 的专属规则目录，定义门禁、边界和失败动作。";
  }
  if (folder.startsWith("scenes/") && folder.includes("/skills/")) {
    descriptions[folder] = "标准 skill package 目录，包含 SKILL.md、scripts、references 与 assets。";
  }
  if (folder.startsWith("scenes/") && !descriptions[folder]) {
    descriptions[folder] = "当前 scene 的主源目录，包含 SCENE.md、workflow.yaml 以及配套 contract / policy_rule / skills。";
  }
  return {
    key: folder,
    label: folder,
    description: descriptions[folder] || mode?.description || "当前目录下的人类可读主源文件。",
    assetType: mode?.assetType,
    createLabel: mode?.createLabel
  };
}

function sourceFileLabel(item: SystemResourceDetail) {
  const path = (item.source_path || "").trim();
  if (path) {
    const segments = path.split("/");
    return segments[segments.length - 1];
  }
  return item.asset_name || item.asset_id;
}

function previewAssetID(assetType: string, slug: string) {
  if (assetType === "tool_governance_policy") {
    return `tool_governance_policy.core.${normalizeSourceSlug(slug)}`;
  }
  return `${assetType}.${normalizeSourceSlug(slug)}`;
}

function previewSourcePath(assetType: string, slug: string) {
  const normalized = normalizeSourceSlug(slug);
  switch (assetType) {
    case "policy_rule":
      return `sources/core/policy_rule/${normalized}.md`;
    case "tool_governance_policy":
      return `sources/core/tool_governance_policy/${normalized}.md`;
    case "user_profile":
      return `sources/core/user_profile/${normalized}.md`;
    case "memory_view":
      return `sources/core/memory_view/${normalized}.md`;
    case "scene":
      return `sources/scenes/${normalized}/SCENE.md`;
    case "workflow":
      return `sources/scenes/${normalized}/workflow.yaml`;
    case "contract":
      return `sources/scenes/default/contract/${normalized}.yaml`;
    case "skill":
      return `sources/scenes/default/skills/${normalized}/SKILL.md`;
    default:
      return `sources/${assetType}/${normalized}.md`;
  }
}

function normalizeSourceSlug(input: string) {
  const normalized = input
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9._/-]+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^[-./]+|[-./]+$/g, "");
  return normalized || "untitled";
}

function defaultSourceTemplate(assetType: string, slug: string) {
  const title = slug
    .split(/[-_.]/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ") || "Untitled";
  switch (assetType) {
    case "policy_rule":
      return `---\nid: ${normalizeSourceSlug(slug)}\nname: ${title}\nsummary: Describe the policy rule.\nseverity: critical\ncheckpoints:\n  - pre_inference\non_fail: deny\n---\n\n## Purpose\n\nDescribe the rule goal.\n\n## Scope\n\nDescribe where this rule applies.\n\n## Hard Gates\n\n- Add one non-negotiable boundary.\n\n## Check Rules\n\n- Add one machine-checkable rule.\n\n## On Failure\n\nDescribe deny / ask / degrade / suppress_tool behavior.\n\n## Guidance\n\nAdd guidance lines for model and system behavior.\n`;
    case "tool_governance_policy":
      return `---\nid: ${normalizeSourceSlug(slug)}\nname: ${title}\nsummary: Describe tool governance decisions.\ndefault_decision: allow\ndecision_model: first_match\nrules:\n  - id: redact_external_read\n    match_tool: demo_browser\n    match_scope: external_web\n    match_operation: read\n    match_risk: medium\n    decision: allow_with_redaction\n    reason: External reads may proceed with redaction.\n    redact_fields:\n      - headers.authorization\n  - id: require_sandbox_for_local_write\n    match_tool: local_shell\n    match_scope: workspace\n    match_operation: write\n    match_risk: high\n    decision: require_sandbox_ref\n    reason: Workspace writes require an explicit sandbox reference.\n    sandbox_ref: workspace-write\n---\n\n## Purpose\n\nDescribe the runtime tool request boundary.\n`;
    case "scene":
      return `---\nid: ${normalizeSourceSlug(slug)}\nname: ${title}\nsummary: Describe this scene.\n---\n\n## Purpose\n\nDescribe what this scene solves.\n\n## When It Applies\n\n- Add one triggering condition.\n\n## Primary Outcome\n\nDescribe the expected output.\n\n## Core Objects\n\n- Add one core object.\n\n## Decision Dimensions\n\n- Add one decision dimension.\n\n## Default Assets\n\n- workflow.${normalizeSourceSlug(slug)}.main\n\n## Out Of Scope\n\n- Add one out-of-scope request.\n\n## Examples\n\n- Add one example.\n`;
    case "workflow":
      return `id: ${normalizeSourceSlug(slug)}_main\nname: ${title} Workflow\nsummary: Main workflow for this scene\n\nentry:\n  allow_waiting: true\n  allow_resume: true\n  required_contracts: []\n  required_policy_rules: []\n\nstages:\n  - id: understand_request\n    title: Understand Request\n    mode: llm\n    purpose: Clarify the request and gather the minimum facts\n    uses_skills: []\n    uses_contracts: []\n    checks:\n      policy_rules: []\n    entry_if: []\n    complete_when:\n      - request_understood\n    block_when: []\n    next:\n      on_success: done\n      on_waiting: waiting_for_information\n      on_failure: failed\n`;
    case "contract":
      return `id: ${normalizeSourceSlug(slug)}\nname: ${title} Contract\nsummary: Structured contract for this scene\nkind: output\n\nrequired_fields:\n  - summary\n\nproperties:\n  summary:\n    type: string\n\nvalidation_rules:\n  - contract_rule.summary_required\n\ncompletion_rules:\n  - contract_rule.summary_present\n`;
    case "skill":
      return `---\nid: ${normalizeSourceSlug(slug)}\nname: ${title}\nsummary: Describe this skill.\ndescription: Describe when and why this skill should be used.\nscene: default\ndepends_on: []\nallowed_tools: []\n---\n\n## When to Use\n\nDescribe the trigger conditions.\n\n## Input\n\nDescribe required inputs.\n\n## Process\n\nDescribe the core steps.\n\n## Output\n\nDescribe the expected result.\n\n## Red Flags\n\n- Add one misuse warning.\n\n## References\n\nDescribe when to read references/.\n`;
    case "user_profile":
      return `# ${title}\n\n## Identity\n\nDescribe the user profile baseline.\n`;
    case "memory_view":
      return `# ${title}\n\n## Memory\n\nDescribe the memory view baseline.\n`;
    default:
      return `# ${title}\n\n`;
  }
}

function buildSystemValidationChecks(data: BootstrapPayload, items: SystemResourceDetail[]): SystemValidationCheck[] {
  const bootstrapResources = data.system_resources?.length ?? 0;
  const loadedResources = items.length;
  const enabledScenes = data.scenes.filter((item) => item.enabled).length;
  const enabledSkills = data.skills.filter((item) => item.enabled).length;
  const referencedTools = new Set(
    data.skills.flatMap((item) => item.tool_names ?? []).map((item) => item.trim()).filter(Boolean)
  );
  const visibleTools = new Set(data.tools.map((item) => item.name));
  const missingReferencedTools = [...referencedTools].filter((item) => !visibleTools.has(item));
  const unexpectedVisibleTools = data.tools.filter((item) => !referencedTools.has(item.name));
  const resourceProblems = items.filter((item) => hasProblemStatus(item.status) || hasProblemStatus(item.parse_result?.status) || hasProblemStatus(item.compile_result?.status) || hasProblemStatus(item.pipeline?.status));
  const warningCount = items.reduce((total, item) => total + (item.parse_result?.warnings?.length ?? 0) + (item.pipeline?.warnings?.length ?? 0), 0);
  const errorCount = items.reduce((total, item) => total + (item.parse_result?.errors?.length ?? 0) + (item.pipeline?.errors?.length ?? 0), 0);

  return [
    {
      title: "Truth Resources",
      status: loadedResources >= bootstrapResources ? "ok" : "warning",
      detail: `${loadedResources} loaded / ${bootstrapResources} bootstrap summaries`
    },
    {
      title: "Scenes",
      status: enabledScenes > 0 ? "ok" : "error",
      detail: `${enabledScenes}/${data.scenes.length} enabled scenes`
    },
    {
      title: "Skills",
      status: enabledSkills > 0 ? "ok" : "error",
      detail: `${enabledSkills}/${data.skills.length} enabled skills`
    },
    {
      title: "Tools",
      status: missingReferencedTools.length > 0 || unexpectedVisibleTools.length > 0 ? "warning" : "ok",
      detail:
        referencedTools.size === 0
          ? `${data.tools.length} visible tools; current truth skills reference no tools`
          : `${visibleTools.size} visible / ${referencedTools.size} referenced tools`
    },
    {
      title: "Pipeline Health",
      status: errorCount > 0 || resourceProblems.length > 0 ? "error" : warningCount > 0 ? "warning" : "ok",
      detail: `${resourceProblems.length} problem resources · ${warningCount} warnings · ${errorCount} errors`
    }
  ];
}
function buildP0ValidationChecks(data: BootstrapPayload, items: SystemResourceDetail[], endpointKeys: Set<string>, debugPayload: SystemResourceDebugPayload | null): SystemValidationCheck[] {
  const resourceProblems = items.filter((item) => hasProblemStatus(item.status) || hasProblemStatus(item.parse_result?.status) || hasProblemStatus(item.compile_result?.status) || hasProblemStatus(item.pipeline?.status));
  const debugBody = debugPayload?.payload ?? null;
  const debugEndpoint = normalizeDebugEndpoint(debugPayload?.endpoint ?? "");
  return [
    {
      title: "Swagger Contract",
      status: endpointKeys.has("POST /api/runtime/respond") && endpointKeys.has("POST /api/runtime/scenario/respond") ? "ok" : "error",
      detail: "runtime/respond 与 runtime/scenario/respond 均需出现在 OpenAPI"
    },
    {
      title: "System Resources Payload",
      status: debugBody && Object.prototype.hasOwnProperty.call(debugBody, "query") ? "ok" : "warning",
      detail: debugEndpoint ? `${debugEndpoint} actual request body is generated separately` : "select a resource to generate debug payload"
    },
    {
      title: "Runtime Direct Path",
      status: endpointKeys.has("POST /api/runtime/respond") ? "ok" : "error",
      detail: "P0 验收应走通用 direct respond adapter"
    },
    {
      title: "Scenario Compatibility",
      status: endpointKeys.has("POST /api/runtime/scenario/respond") ? "ok" : "warning",
      detail: "legacy judgment 被隔离在显式兼容路由"
    },
    {
      title: "Truth Pipeline",
      status: resourceProblems.length === 0 && data.scenes.length > 0 ? "ok" : resourceProblems.length > 0 ? "error" : "warning",
      detail: `${items.length} resources · ${resourceProblems.length} problem resources · ${data.scenes.length} scenes`
    }
  ];
}


function buildRuntimeRespondPayload(query: string, assetID: string, truthDirVersion?: string) {
  return {
    query,
    task_type: "runtime_validation",
    task_subtype: "direct_response",
    global_context: {
      validation_mode: "p0_runtime_respond",
      selected_asset_id: assetID || undefined,
      truth_dir_version: truthDirVersion || undefined
    },
    timeout_after_seconds: 45
  };
}

function defaultRuntimeScenarioPayload() {
  return {
    host_type: "openclaw",
    hook_name: "before_tool_call",
    event_type: "runtime_event",
    raw_payload: {
      params: {
        command: "bash tools/deploy.sh"
      }
    }
  };
}

function buildTextComparison(before: string, after: string): TextComparison {
  const beforeTrimmed = before.trim();
  const afterTrimmed = after.trim();
  return {
    hasBoth: beforeTrimmed !== "" && afterTrimmed !== "",
    same: beforeTrimmed !== "" && beforeTrimmed === afterTrimmed,
    beforeCharacters: before.length,
    afterCharacters: after.length,
    beforeLines: countTextLines(before),
    afterLines: countTextLines(after)
  };
}

function countTextLines(input: string) {
  const trimmed = input.trim();
  if (!trimmed) {
    return 0;
  }
  return trimmed.split(/\r?\n/).length;
}

function runtimeStatusClass(status?: string) {
  const normalized = (status || "").toLowerCase();
  if (normalized === "completed" || normalized === "success") {
    return "info-card success";
  }
  if (normalized === "failed" || normalized === "cancelled") {
    return "info-card error";
  }
  return "info-card";
}

function toolGovernanceDecisionClass(decision?: string) {
  const normalized = (decision || "").toLowerCase();
  if (normalized === "allow" || normalized === "allow_with_redaction") {
    return "info-card success";
  }
  if (normalized === "deny") {
    return "info-card error";
  }
  return "info-card";
}

function formatRuntimeTime(input?: string) {
  if (!input) {
    return "-";
  }
  const date = new Date(input);
  if (Number.isNaN(date.getTime())) {
    return input;
  }
  return date.toLocaleString();
}

function runtimeStepLifecycleCount(events: RuntimeLifecycleEvent[], stepID: string) {
  return events.filter((item) => item.step_id === stepID || item.subject_id === stepID).length;
}

function summarizeRuntimeUsage(items: RuntimeUsage[]) {
  if (items.length === 0) {
    return "no usage records";
  }
  const byUnit = new Map<string, number>();
  for (const item of items) {
    const unit = item.unit || item.resource_type || "unit";
    byUnit.set(unit, (byUnit.get(unit) ?? 0) + item.amount);
  }
  return [...byUnit.entries()].map(([unit, amount]) => `${formatRuntimeAmount(amount)} ${unit}`).join(" · ");
}

function formatRuntimeAmount(value: number) {
  if (!Number.isFinite(value)) {
    return "0";
  }
  if (Number.isInteger(value)) {
    return String(value);
  }
  return value.toFixed(3).replace(/0+$/g, "").replace(/\.$/g, "");
}

function runtimeTraceLabel(trace: RuntimeTrace) {
  const labels = trace.safe_labels ? Object.entries(trace.safe_labels).map(([key, value]) => `${key}:${value}`) : [];
  return labels.length > 0 ? labels.slice(0, 3).join(" · ") : trace.step_id || trace.id;
}

function runtimeProjectionLabel(projection?: RuntimeProjectionCandidate | null) {
  if (!projection) {
    return "no projection";
  }
  return [projection.schema_version, projection.summary, projection.id].filter(Boolean).slice(0, 2).join(" · ") || projection.id;
}

function summarizeRuntimeCheckpoints(items: RuntimeCheckpointReadout[]) {
  if (items.length === 0) {
    return "no checkpoint refs";
  }
  const snapshots = items.filter((item) => item.snapshot_available).length;
  const latest = items[0];
  return `${snapshots}/${items.length} snapshots · ${latest.stage || "checkpoint"} · payload hidden`;
}

function shortHash(value?: string) {
  if (!value) {
    return "";
  }
  return value.length > 12 ? `${value.slice(0, 12)}…` : value;
}

function hasProblemStatus(status?: string) {
  const normalized = (status || "").toLowerCase();
  return normalized.includes("error") || normalized.includes("fail");
}

function validationSourceFileLabel(item: SystemResourceDetail | null) {
  return item ? sourceFileLabel(item) : "-";
}

function toProviderDraft(provider: ProviderDefinition): ProviderDraft {
  return {
    id: provider.id,
    name: provider.name,
    base_url: provider.base_url ?? "",
    protocol: provider.protocol,
    api_key: "",
    request_timeout_seconds: provider.request_timeout_seconds ?? 45,
    headers_text: stringifyHeaders(provider.headers),
    enabled: provider.enabled
  };
}

function toModelDraft(model: ProviderModelRecord): ModelDraft {
  return {
    id: model.id,
    model_id: model.model_id,
    display_name: model.display_name,
    enabled: model.enabled,
    is_default: model.is_default,
    is_fallback: model.is_fallback
  };
}

function stringifyHeaders(input?: Record<string, string>) {
  if (!input) {
    return "";
  }
  return Object.entries(input)
    .map(([key, value]) => `${key}: ${value}`)
    .join("\n");
}

function parseObjectText(input: string): Record<string, unknown> {
  const trimmed = input.trim();
  if (trimmed === "") {
    return {};
  }
  try {
    const parsed = JSON.parse(trimmed) as Record<string, unknown>;
    return typeof parsed === "object" && parsed ? parsed : {};
  } catch {
    return {};
  }
}

function parseAPIErrorMessage(message: string) {
  try {
    const parsed = JSON.parse(message) as { error?: unknown };
    if (typeof parsed.error === "string") {
      return parsed.error;
    }
  } catch {
    return message;
  }
  return message;
}

function formatMaybeJSON(value: unknown) {
  if (value === null || value === undefined) {
    return "";
  }
  return JSON.stringify(value, null, 2);
}

function parseRuntimeFoundationJSON<T>(input: string, label: string): T {
  const trimmed = input.trim();
  if (!trimmed) {
    throw new Error(`${label} JSON is required`);
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch (cause) {
    throw new Error(`${label} JSON 无法解析: ${cause instanceof Error ? cause.message : String(cause)}`);
  }
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error(`${label} JSON 必须是 object`);
  }
  return parsed as T;
}

function defaultRuntimeContractDraft(): RuntimeContractUpsertInput {
  return {
    name: "Runtime Validation Contract",
    version: "v1",
    status: "active",
    task_type: "runtime_validation",
    metadata: {
      editor_surface: "system_validation"
    }
  };
}

function defaultRuntimeTaskTypeDraft(): RuntimeTaskTypeUpsertInput {
  return {
    display_name: "Runtime Validation",
    status: "active",
    default_contract_id: "runtime_validation_contract",
    metadata: {
      editor_surface: "system_validation"
    }
  };
}

function defaultRuntimeHookBindingDraft(contractID?: string): RuntimeHookBindingUpsertInput {
  return {
    contract_id: contractID || "runtime_validation_contract",
    hook_point: "before_run",
    binding_kind: "eino_middleware",
    binding_ref: "runtime_contract_guard",
    order_index: 10,
    enabled: true,
    failure_policy: "fail_closed",
    metadata: {
      editor_surface: "system_validation"
    }
  };
}

function downloadJSON(value: unknown, filename: string) {
  const blob = new Blob([formatMaybeJSON(value)], { type: "application/json;charset=utf-8" });
  downloadBlob(blob, filename);
}

function downloadBlob(blob: Blob, filename: string) {
  const href = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = href;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(href);
}

async function copyTextToClipboard(text: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.style.position = "fixed";
  textarea.style.opacity = "0";
  document.body.appendChild(textarea);
  textarea.focus();
  textarea.select();
  document.execCommand("copy");
  document.body.removeChild(textarea);
}

function parseHeadersText(input: string): Record<string, string> {
  return input
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean)
    .reduce<Record<string, string>>((acc, line) => {
      const index = line.indexOf(":");
      if (index === -1) {
        return acc;
      }
      const key = line.slice(0, index).trim();
      const value = line.slice(index + 1).trim();
      if (!key) {
        return acc;
      }
      acc[key] = value;
      return acc;
    }, {});
}

function safeParseJSON(input: string): Record<string, string> {
  const trimmed = input.trim();
  if (trimmed === "") {
    return {};
  }
  try {
    const parsed = JSON.parse(trimmed) as Record<string, string>;
    return typeof parsed === "object" && parsed ? parsed : {};
  } catch {
    return {};
  }
}

function buildDefaultParamsJSON(path: string) {
  const matches = [...path.matchAll(/\{([^}]+)\}/g)].map((item) => item[1]);
  if (matches.length === 0) {
    return "{}";
  }
  const seed = matches.reduce<Record<string, string>>((acc, key) => {
    acc[key] = "";
    return acc;
  }, {});
  return JSON.stringify(seed, null, 2);
}

function applyPathParams(path: string, params: Record<string, string>) {
  return path.replace(/\{([^}]+)\}/g, (_, key: string) => encodeURIComponent(params[key] ?? ""));
}

function appendQueryString(path: string, query: string) {
  const trimmed = query.trim();
  if (trimmed === "") {
    return path;
  }
  return `${path}?${trimmed}`;
}

function defaultRequestBody(endpoint: OpenAPIEndpoint) {
  const editableMethods = endpoint.method === "POST" || endpoint.method === "PUT" || endpoint.method === "PATCH";
  if (!editableMethods) {
    return "";
  }
  if (endpoint.key === "POST /api/runtime/respond" || endpoint.key === "POST /api/chat/respond" || endpoint.key === "POST /api/chat/stream") {
    return formatMaybeJSON(buildRuntimeRespondPayload("请用一句话说明当前 runtime respond 是否可用。", ""));
  }
  if (endpoint.key === "POST /api/runtime/scenario/respond") {
    return formatMaybeJSON(defaultRuntimeScenarioPayload());
  }
  return "{}";
}

function buildP0APIDebugPresets(items: OpenAPIEndpoint[]): APIDebugQuickPreset[] {
  const endpointKeys = new Set(items.map((item) => item.key));
  const runtimeBody = formatMaybeJSON(buildRuntimeRespondPayload("请用一句话说明当前 runtime respond 是否可用。", ""));
  return [
    {
      label: "Runtime Respond",
      detail: "通用 direct respond adapter",
      preset: { endpointKey: "POST /api/runtime/respond", bodyText: runtimeBody },
      available: endpointKeys.has("POST /api/runtime/respond")
    },
    {
      label: "Scenario Respond",
      detail: "legacy judgment compatibility route",
      preset: { endpointKey: "POST /api/runtime/scenario/respond", bodyText: formatMaybeJSON(defaultRuntimeScenarioPayload()) },
      available: endpointKeys.has("POST /api/runtime/scenario/respond")
    },
    {
      label: "Chat Respond",
      detail: "普通 chat direct respond baseline",
      preset: { endpointKey: "POST /api/chat/respond", bodyText: runtimeBody },
      available: endpointKeys.has("POST /api/chat/respond")
    },
    {
      label: "Chat Stream",
      detail: "SSE progress and message preview",
      preset: { endpointKey: "POST /api/chat/stream", bodyText: runtimeBody },
      available: endpointKeys.has("POST /api/chat/stream")
    }
  ];
}

function formatResponseBody(body: string) {
  const trimmed = body.trim();
  if (trimmed === "") {
    return "";
  }
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2);
  } catch {
    return body;
  }
}

function normalizeDebugEndpoint(endpoint: string) {
  switch (endpoint) {
    case "chat_respond":
      return "/api/chat/respond";
    case "chat_stream":
      return "/api/chat/stream";
    case "runtime_respond":
      return "/api/runtime/respond";
    default:
      return endpoint || "/api/chat/respond";
  }
}

function endpointKeyForDebugPayload(endpoint: string) {
  const normalized = normalizeDebugEndpoint(endpoint);
  if (normalized === "/api/chat/stream") {
    return "POST /api/chat/stream";
  }
  if (normalized === "/api/runtime/respond") {
    return "POST /api/runtime/respond";
  }
  if (normalized === "/api/runtime/scenario/respond") {
    return "POST /api/runtime/scenario/respond";
  }
  return "POST /api/chat/respond";
}

function isChatStreamEndpoint(endpoint: OpenAPIEndpoint | null) {
  return !!endpoint && endpoint.method === "POST" && endpoint.path === "/api/chat/stream";
}

function buildStreamTranscript(body: string): StreamTranscriptItem[] {
  const events = parseSSEPayloads(body);
  if (events.length === 0) {
    return [];
  }

  const items: StreamTranscriptItem[] = [];
  const assistantParts: string[] = [];

  function flushAssistant() {
    const content = assistantParts.join("").trim();
    if (!content) {
      assistantParts.length = 0;
      return;
    }
    items.push({
      kind: "assistant",
      title: "Athena",
      text: content
    });
    assistantParts.length = 0;
  }

  for (const event of events) {
    const payload = asRecord(event.payload);
    const type = textValue(payload?.type) || event.event || "message";
    if (type === "stream_chunk" || type === "message") {
      const content = textValue(payload?.content);
      if (content) {
        assistantParts.push(content);
      }
      continue;
    }

    flushAssistant();

    if (type === "progress_step") {
      const step = asRecord(payload?.progress_step);
      items.push({
        kind: "system",
        title: textValue(step?.label) || textValue(step?.step_id) || "步骤更新",
        text: textValue(step?.summary) || "Athena 更新了当前处理步骤。",
        status: textValue(payload?.status)
      });
      continue;
    }

    if (type === "done") {
      const detail = asRecord(payload?.detail);
      const stepFlow = asRecord(detail?.step_flow);
      const status = textValue(payload?.status);
      items.push({
        kind: status === "completed" ? "system" : "error",
        title: status === "completed" ? "完成" : "结束状态",
        text: textValue(stepFlow?.summary) || defaultDoneSummary(status),
        status
      });
      continue;
    }

    if (type === "action") {
      const action = asRecord(payload?.action);
      items.push({
        kind: "system",
        title: textValue(payload?.action_type) || textValue(action?.code) || "等待动作",
        text:
          textValue(action?.reason) ||
          textValue(action?.instruction) ||
          textValue(action?.message) ||
          "Athena 需要客户端执行一个动作后才能继续。"
      });
      continue;
    }

    if (type === "error") {
      items.push({
        kind: "error",
        title: "错误",
        text: textValue(payload?.error) || event.data || "请求失败。"
      });
      continue;
    }

    if (event.data.trim()) {
      items.push({
        kind: "system",
        title: type,
        text: event.data.trim()
      });
    }
  }

  flushAssistant();
  return items;
}

function parseSSEPayloads(body: string) {
  const trimmed = body.trim();
  if (!trimmed) {
    return [];
  }

  const blocks = trimmed.split(/\r?\n\r?\n+/);
  const items = blocks.flatMap((block) => {
    const lines = block.split(/\r?\n/);
    let event = "";
    const dataLines: string[] = [];
    for (const line of lines) {
      if (line.startsWith("event:")) {
        event = line.slice("event:".length).trim();
        continue;
      }
      if (line.startsWith("data:")) {
        dataLines.push(line.slice("data:".length).trimStart());
      }
    }
    const data = dataLines.join("\n").trim();
    if (!event && !data) {
      return [];
    }
    return [
      {
        event,
        data,
        payload: tryParseJSON(data)
      }
    ];
  });

  if (items.length > 0) {
    return items;
  }

  return [
    {
      event: "",
      data: trimmed,
      payload: tryParseJSON(trimmed)
    }
  ];
}

function tryParseJSON(input: string) {
  const trimmed = input.trim();
  if (!trimmed) {
    return null;
  }
  try {
    return JSON.parse(trimmed) as unknown;
  } catch {
    return null;
  }
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
}

function textValue(value: unknown) {
  if (typeof value === "string") {
    return value.trim();
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return "";
}

function defaultDoneSummary(status: string) {
  switch (status) {
    case "completed":
      return "Athena 已完成本轮流式响应。";
    case "waiting_for_information":
    case "pending_human":
      return "Athena 需要更多信息后才能继续。";
    case "timed_out":
      return "本轮流式请求超时结束。";
    default:
      return "本轮流式请求已结束。";
  }
}
