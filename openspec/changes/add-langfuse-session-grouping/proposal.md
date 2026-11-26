# Proposal

## Why

Langfuse 当前每次受控 Debug 运行都只有独立 trace，没有 Session 归组。Langfuse 官方将 Session 定义为把多个 trace/observations 归入同一会话回放的应用侧标识；评测场景需要按同一道题查看多个 repetition 和 retry，而不同评测运行必须保持隔离。

## What Changes

- 为 Langfuse trace 增加可选的 session ID、trace name 和结构化 metadata 属性。
- `/internal/ai-debug` 支持受控调用者显式传入 session ID；未传入时保持独立 trace。
- 评测运行在显式启用 Langfuse 导出时，按 `eval:<run_id>:<suite>:<case_id>` 归组同一道题的 traces，并记录评测版本、重复次数和重试次数等 metadata。
- 对 session ID 和 metadata 做 ASCII、长度和键值边界校验，避免产生不可查询或被 Langfuse 丢弃的属性。
- 增加 OTLP 映射、Debug 请求和评测 session 回归测试及操作文档。

## Capabilities

### New Capabilities

- `langfuse-session-grouping`: 受控 AI trace 的 Session 归组和评测上下文元数据。

### Modified Capabilities

无。

## Impact

影响 `internal/aitrend` Langfuse exporter、AI Debug loopback endpoint、离线评测 runner/CLI 和 AI 调试文档。普通生产请求、现有独立 Debug trace 和业务数据库不变。
