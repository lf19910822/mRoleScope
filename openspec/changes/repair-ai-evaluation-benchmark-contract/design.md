# Design

## Decisions

1. U02 的 `ranking_metric` 使用 `avg_daily_active_jobs`，新增 `requested_metric` 字段记录 `share`；现有 query 文字仍触发 `unsupported_metric` 回退。
2. D01 的 `limit` 对齐为 8；超过 8 的候选由快照数量和 `expected_total_at_least` 验证容量，而不是向 Intent Schema 写入 20。
3. D04 明确两个重复子问题，验证服务端合并后的唯一曲线和 `sub_query_ids`。
4. Orchestrator 将 Serving 返回的模型工具合同错误也作为 validation 事件反馈；评分器据此识别注入修正。
5. Agent 题只有在包含 trend 和 Draft 时才额外要求 Scope、Ranking、ToolCalls；guidance-only 保持无趋势工具即可通过。
6. v3 快照新增正式值和 daily series，使用确定性零值与递增值覆盖排序、补批和容量。
