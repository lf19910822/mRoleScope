# Proposal

## Why

部分“哪些项目/哪些岗位族”问题会被模型表示为包含空字符串的表达式数组。现有规划器把空字符串当作可排序条件，导致正式值查询完成后仍没有候选曲线，用户看到 0/8 条草稿。

## What Changes

- 将空表达式、通配符和明确的宽泛表达式解释为当前统计维度的全部正式值，并继续按趋势摘要排序和限制最多 8 条曲线。
- 对空表达式或宽泛维度表达式跳过无意义的 `rank_values` 调用。
- 在意图提示词中明确要求宽泛类别使用 `expressions: ["*"]`，避免模型继续产生空字符串。
- 保持排除条件、正式值校验、扩展推荐标记和工具调用上限不变。

## Capabilities

### New Capabilities

- `ai-trend-broad-value-fallback`: 对未提供具体值的类别型趋势问题提供有界的正式值候选结果。

### Modified Capabilities

<!-- No existing repository specs are present; this change introduces the capability above. -->

## Impact

- 影响趋势意图到 Serving 候选的确定性解析和工具规划。
- 不改变 API 路径、草稿格式或前端交互；会让原本 0 条的宽泛项目/岗位族问题生成可应用草稿。
- 增加 resolver、planner、runner 的回归测试，并重新执行真实业务验收。
