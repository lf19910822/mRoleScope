# Proposal

## Why

“使用最多的编程语言”当前可能被模型输出为通配表达式，导致技术词典中的游戏引擎、容器和数据库也进入草稿；说明文本还可能刚好达到 100 字而在句中截断。

## What Changes

- 将中文类别问题写入意图提示词，并由服务端对“编程语言”等明确类别做确定性兜底匹配。
- 将 AI 说明目标长度收紧到 80 字，保留 100 字协议上限，减少句中截断。
- 保持其他宽泛问题的 `*` 规则和现有排序逻辑。

## Capabilities

### New Capabilities

- `ai-trend-category-precision`: 确保明确技术类别只产生该类别的正式候选。

### Modified Capabilities

<!-- No existing repository specs are present; this change introduces the capability above. -->

## Impact

- 影响意图提示词、Runner 的类别归一化和说明模型提示词。
- 不改变 API、草稿格式或看板应用接口。
