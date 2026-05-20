# web

## 模块职责

- `web/` 承载 Athena 独立控制面的 React + TypeScript 前端。
- 它负责展示 scene、skill、tool、模型治理、治理策略、system resources、配置版本与 Swagger 文档，不负责 Athena 主运行时本身。
- 用户可见产品名统一为“墨思”。

## 不负责什么

- 不负责后端存储
- 不负责平台业务状态机
- 不负责修改 Athena 核心治理底线

## 子目录索引

- `src/`
  - 控制面页面、API 客户端和样式资源。

## 文件索引

- `package.json`
  - 前端依赖与构建脚本定义。
- `vite.config.ts`
  - Vite 开发代理和构建配置。
- `tsconfig.json`
  - 浏览器端 TypeScript 配置。
- `tsconfig.node.json`
  - Vite 配置文件使用的 TypeScript 配置。
- `index.html`
  - Vite 入口 HTML。
- `src/main.tsx`
  - React 启动入口。
- `src/App.tsx`
  - 控制面主布局、最小登录页、Release Readiness、system resources 调试页、模型配置、版本回滚、治理策略和接口调试页面。
- `src/api.ts`
  - 控制面后端接口客户端。
- `src/types.ts`
  - 前端消费的 control-plane 类型定义。
- `src/styles.css`
  - 控制面页面样式；业务表单样式需要限制在控制台容器内，避免污染 Swagger UI 子树。
- `src/global.d.ts`
  - 第三方模块的最小声明补充。

## 对外入口

- `npm install`
- `npm run dev`
- `npm run build`
- `npm run preview`

## 关键依赖

- 依赖 Athena 后端 `control-plane` 接口和 `/swagger/openapi.json`
- 依赖配置版本、tool registry、governance、模型治理和 `system-resources` 接口

## 启动与打包

### 本地开发

```bash
cd web
npm install
npm run dev
```

默认开发地址：

- `http://127.0.0.1:5173`

默认开发代理会把：

- `/api`
- `/swagger`

转发到：

- `http://127.0.0.1:8090`

如需覆盖，可在启动前设置：

```bash
export ATHENA_WEB_PROXY_TARGET=http://127.0.0.1:8090
```

### 生产打包

```bash
cd web
npm install
npm run build
```

构建产物位于：

- `dist/`

部署要求：

- 当前前端通过相对路径调用：
  - `/api/*`
  - `/swagger/openapi.json`
- 因此前端静态部署时，需要同域访问 Athena 后端，或由网关转发 `/api`、`/swagger`

## 维护提示

- Swagger 通过动态加载独立 chunk 展示，避免首屏 bundle 被文档组件拖大。
- 控制台表单样式不得使用无作用域的 `label / input / select / textarea` 全局选择器；Swagger 依赖原生表单布局，样式污染会直接破坏文档显示。
- 新增控制面页面时，优先复用当前 `/api/control-plane/*` contract，不要再发明第二套后端配置接口。
- `runtime-config` 当前仍保留兼容语义；新增治理项优先进入 `governance` 接口和页面。
- 模型配置页当前直接复用 `/api/models/providers*` 现有治理接口，不复制一套模型控制面后端。
- 接口调试页当前直接复用 `/swagger/openapi.json` 和同域 HTTP 请求，不新增专用 debug API。
- system-resources 页当前直接复用 `/api/system-resources*` 和 active truth dir 状态，不单独发明第二套对象管理协议。
- 当前支持从 system resource 的 debug payload 一键带入接口调试，并支持下载 parse / compile 结果、查看版本快照与审计轨迹、执行单资源回滚，以及导出 truth dir 快照。
- Release Readiness 页面是 v2.0.0 成品门禁入口，可触发产品验证并展示 runtime / MCP / sandbox 结果。
- 模型页 Provider Readiness 区块可暴露 default model 缺失，并提供一键设为 default 的修复入口。
- system-resources 页的主交互已经收口为：
  - 左侧按 `sources/core/` 与 `sources/scenes/` 分组展示 source 文件
  - 选中文件后直接编辑 source 并“保存并编译”
  - scene 下直接展示 `SCENE.md / workflow.yaml / contract/*.yaml / policy_rule/*.md / skills/<skill_id>/SKILL.md`
- 不再把 system-resources 页作为“通用 resource CRUD 表单”来组织。
