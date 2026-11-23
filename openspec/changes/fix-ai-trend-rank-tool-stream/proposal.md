# Proposal

## Why

生产模型已开始使用 `rank_trend_values`，但 HTTP 流安全白名单遗漏该工具。模型完成首轮分析后，发送排名工具的进行中事件会被 HTTP 层拒绝，导致流立即失败，草稿无法生成。

## What Changes

- 将 `rank_trend_values` 加入安全工具白名单。
- 保留并过滤该工具允许的安全参数，使前端可以展示完整工具调用。
- 增加 HTTP 流回归测试。

## Impact

影响 AI 趋势 NDJSON 流的工具事件转发；不改变工具执行、Serving 查询或 Draft 结构。
