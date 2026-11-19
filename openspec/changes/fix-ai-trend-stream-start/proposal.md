# Proposal

## Why

真实请求在模型首次返回前可能等待二十多秒，浏览器只能显示静态的“分析中”，用户无法判断请求是否已经开始、是否仍在等待模型。流式接口应在进入模型调用时立即发出可展示的操作状态，并在模型返回或失败时闭合该状态。

## What Changes

- 在每次结构化意图分析开始前立即发送模型调用操作事件。
- 在模型返回、协议校验失败重试或提供方错误时发送对应的完成/失败事件。
- 保持现有工具调用事件、终止事件、token 审计和错误职责边界不变；模型操作不计入后端工具调用配额和工具审计数量。
- 前端将模型等待阶段显示在同一个 AI 操作窗口中，避免首个工具事件前出现无反馈空档。

## Capabilities

### New Capabilities

- `ai-trend-stream-progress`: 为 AI 趋势草稿请求提供从模型调用开始到工具执行前的可观察流式进度。

### Modified Capabilities

<!-- No existing repository specs are present; this change introduces the capability above. -->

## Impact

- 影响 `internal/aitrend` 的编排事件和审计过滤、HTTP NDJSON 事件白名单、前端 AI 操作状态类型与渲染。
- 扩展现有流式事件协议的操作名称集合，不改变请求路径、终止状态或草稿格式。
- 增加编排器、HTTP 传输和前端状态测试。
