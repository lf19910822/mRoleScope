# Design

## Decision

`response_modes` 继续保持数组，数量和枚举范围仍由 Schema 约束；删除 `uniqueItems`，因为 Qwen 的 JSON Schema 校验器拒绝该数组关键字。Go 的 `DecodeAgentResponse` 已独立检查空数组、非法枚举和重复模式，因此重复模式仍会在本地被拒绝。

## TDD seam

在 Qwen provider 的 HTTP 请求边界检查最终 JSON Schema：请求必须使用 Agent Schema，且不得包含 Qwen 不接受的数组关键字。该测试覆盖真实失败的协议边界，不依赖外部模型响应。
