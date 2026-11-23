# Proposal

## Why

真实 Agent 评测显示纯指导题和混合题全部未通过：模型的初始自然语言输出被当成结构化 Intent，工具调用同时带文本时又被直接拒绝。需要把这些可恢复的模型行为转换为明确的 guidance 或继续工具流程，避免单次格式偏差让整题失败。

## What Changes

- 初始 Agent 轮无法形成结构化响应且没有工具调用时，安全降级为 `guidance`，继续正常流式回复。
- 工具调用消息同时带有文本时，忽略该阶段文本并保留工具调用，最终由 Agent 响应统一表达 `trend`/`guidance`。
- 对不支持的初始响应模式保留可审计错误，同时采用 guidance fallback，避免空回复。
- 为上述路径增加 provider 和 orchestrator 回归测试。

## Capabilities

### New Capabilities

- `ai-agent-contract-recovery`: Agent 结构化响应失败时的安全降级和工具流程恢复。

### Modified Capabilities

无。

## Impact

- 影响 `internal/aitrend/qwen_provider.go`、`internal/aitrend/orchestrator.go` 和 Agent 评测。
- 不保存模型原始回复，不改变凭据、Serving 或 Draft Schema。
