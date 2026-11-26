# Proposal

## Why

AI 趋势 Benchmark 当前有题目字段超出生产 Intent 合同、故障注入无法形成可观察的校验修正，以及 guidance-only 被工具评分器误报等问题。评测专用 Serving 快照也没有足够候选覆盖跨批次、容量截断和零值排序，因此本地通过率不能代表真实边界覆盖。

## What Changes

- 对齐 U02、D01、D04 题卡与生产合同和真实判定目标。
- 修正 U05 fixture 故障注入，使非法参数修正事件真实进入评测记录。
- 让 guidance-only 且明确禁止趋势工具的题目在无工具调用时通过工具子评分。
- 让带趋势 Draft 的 Agent 题纳入范围和排名硬判定。
- 扩充 v3 评测快照，增加超过 8 个项目、超过 8 个数据库候选和零岗位数据库值。
- 增加题卡、评分器、fixture 和快照回归测试，并更新评测文档。

## Capabilities

### New Capabilities

- `benchmark-contract-alignment`: 评测题卡、fixture、评分器和快照与生产趋势合同保持一致。

### Modified Capabilities

无。

## Impact

影响 `Docs/开发指导/AI评测/benchmarks`、评测 scorer、fixture provider、evaluation snapshot 和相关测试；不改变生产 Serving 或用户 API 合同。
