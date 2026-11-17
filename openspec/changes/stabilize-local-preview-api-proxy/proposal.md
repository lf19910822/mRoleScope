# Proposal

## Why

本地 5175 预览依赖 `VITE_API_PROXY_TARGET` 才会转发 `/api/`。实际启动时经常直接执行 `npm run dev`，导致 Vite 将 API 路径交给 SPA fallback，页面无法读取岗位数据；过时的开发文档还指向已经不使用的 `18080` 端口，增加了重复排查成本。

## What Changes

- 开发模式下读取 Vite 的 `.env`/进程环境，并在未显式指定时默认使用 `127.0.0.1:20080` 回环 API 代理。
- 保持生产构建不注入本地代理目标。
- 更新本地预览文档，统一使用当前回环端口，并说明显式覆盖方式。
- 不改变业务 API 路由、鉴权逻辑、生产 Web Edge 或数据读取口径。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

无。

## Impact

- 影响 `web/vite.config.ts` 的本地开发服务器代理配置。
- 影响 `web/README.md` 的本地验收启动说明。
- 只改变开发机 5175 的请求转发，不改变生产运行时行为。
