# Design

## Context

当前 Qwen 调用只生成结构化意图，`draftExplanation` 在 Go 服务端根据 ranking.metric 固定拼接说明；前端已经在 AI 对话窗口中展示 `draft.explanation`，因此可以复用展示位置。参见 proposal.md 和 `ai-trend-draft-explanation` 规格。

## Goals / Non-Goals

**Goals:**

- 在所有 Serving 工具完成后调用一次受控说明模型。
- 向模型提供有界、展示安全的候选和摘要上下文。
- 保留确定性回退、token 审计和流式状态。

**Non-Goals:**

- 不让模型重新决定曲线、排序或正式值。
- 不展示模型逐字推理，不把原始工具返回或 JD 内容发送给模型。
- 不阻断草稿应用，不改变既有曲线和应用 API。

## Decisions

### 1. 新增说明阶段而不是扩展首次意图 JSON

意图模型在工具调用前无法看到候选摘要；单独的说明调用能使用本次工具结果，并让流式顺序明确。说明阶段使用固定 JSON schema，只返回 `explanation` 字段。

### 2. 发送结构化、有限的说明上下文

Runner 构造包含用户问题、维度、排序指标/顺序、候选正式标签、匹配状态和有限指标值的 JSON。上下文最多覆盖 8 条最终候选，工具名称和状态只保留摘要，不传递原始 payload。

### 3. 说明失败回退而不改变业务状态

说明是增强信息，不参与候选或草稿正确性判断。模型调用失败时发出失败操作事件，使用现有确定性 `draftExplanation`，草稿保持成功或部分成功状态。

### 4. 复用模型操作审计边界

新增 `explain_draft` 显示操作，和 `analyze_intent` 一样不计入 Serving 工具调用审计；Qwen 返回的 prompt/completion token 累加到现有 Runner 审计字段。

## Risks / Trade-offs

- [额外模型调用增加延迟和费用] → 只调用一次，限制输出 100 字和请求上下文大小，失败立即回退。
- [模型说明与数据排序不一致] → 模型只能解释 Runner 已确定的候选和指标，曲线选择与排序仍由服务端决定。
- [说明阶段请求被取消] → 沿用现有取消路径，已构建草稿仍按停止语义保留。

## Migration Plan

1. 扩展 Qwen 适配器、Runner 事件和审计 token 累加。
2. 添加 provider、runner、HTTP 和前端状态回归测试。
3. 通过受保护 `main` 发布，真实请求检查说明位于工具记录之后且不超过 100 字。
