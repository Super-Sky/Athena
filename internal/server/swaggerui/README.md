# swaggerui

## 模块职责

- 保存嵌入式 Swagger UI 静态资源。

## 不负责什么

- 不负责 OpenAPI 生成逻辑

## 子目录索引

- 当前无子目录

## 文件索引

- `swagger-ui-bundle.js`
  - Swagger UI 的前端脚本资源。
- `swagger-ui.css`
  - Swagger UI 的样式资源。

## 对外入口

- 被 `swagger_assets.go` 暴露

## 维护提示

- 升级 Swagger UI 资源时，注意与页面加载路径保持兼容。
