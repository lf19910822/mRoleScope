# Design

## Context

生产 Web Edge 与 API 使用同源 `/api/`。本地预览只有 Vite 静态开发服务器，因此必须通过绑定在开发机回环地址的 SSH 隧道访问 CVM 内部 API。过去代理配置只读取 shell 环境变量，未配置时 `/api/` 会被 Vite SPA fallback 接管。

## Decisions

### 1. 开发模式默认代理

- 使用 Vite `loadEnv` 读取 `VITE_API_PROXY_TARGET`，同时保留进程环境变量覆盖能力。
- 开发模式未提供覆盖值时，默认目标为 `http://127.0.0.1:20080`。
- 生产构建不设置本地默认代理目标。

### 2. 保持回环边界

- 代理目标只允许配置为本机回环地址或经操作者明确提供的本地目标；本次不新增公网 API 暴露。
- CVM API 仍通过本地 SSH 隧道访问，浏览器入口仍固定为 `http://127.0.0.1:5175/`。

### 3. 失败表现

- 隧道不可用时由 Vite 代理返回连接错误，而不是将 `/api/` 静默返回 `index.html`。
- 未登录 API 返回后端正常的 `401`，由现有访问流程处理；不把鉴权失败伪装成 404。

## Verification

- 启动不带 `VITE_API_PROXY_TARGET` 的 5175，确认 `/api/v1/jobs` 经过代理并返回 API 响应类型。
- 确认根页面仍返回前端 HTML。
- 运行前端类型检查、构建、单元测试和 `git diff --check`。
- 运行严格 OpenSpec 校验。
