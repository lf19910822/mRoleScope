# Proposal

## Why

待确认草稿当前显示的说明由服务端固定拼接，无法解释模型为什么选择这些曲线，也没有利用工具返回的候选和排序信息。用户需要在工具调用记录之后看到一段简短、基于本次分析的 AI 回复。

## What Changes

- 在工具调用和草稿校验完成后增加一次受控的 AI 说明生成。
- 将用户问题、规范化意图、候选曲线和有限的 Serving 摘要交给模型，要求生成不超过 100 字的说明或回答。
- 将模型说明放入现有草稿 explanation 区域，显示在操作记录之后；模型失败时使用确定性回退说明并保留草稿。
- 记录说明请求的 token 用量，并将说明阶段作为独立的非工具操作展示，不占用 Serving 工具调用配额。

## Capabilities

### New Capabilities

- `ai-trend-draft-explanation`: 为 AI 趋势草稿提供基于工具结果的短说明。

### Modified Capabilities

<!-- No existing repository specs are present; this change introduces the capability above. -->

## Impact

- 影响 Qwen 适配器、趋势 Runner、流式操作协议和草稿解释字段。
- 不改变曲线候选、排序、应用接口或前端布局；前端继续显示现有 explanation 字段。
- 额外增加一次受控模型请求及 token 消耗，模型失败不阻断草稿。
