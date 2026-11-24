# Tasks

## Contract alignment

- [x] 1.1 审计 30 道 Trend 题与 10 道 Agent 题的字段和快照候选。
- [x] 1.2 修正显式值、类别正式值和候选不足的 scorer/题卡口径。
- [x] 1.3 新增 v3 容量快照，保留 v2 历史快照。
- [x] 1.4 暴露批量摘要部分错误并继续保留可用结果。
- [x] 1.5 修复离线 fixture 在显式值故障注入题中的空类别崩溃。

## Verification

- [x] 2.1 运行 Go 测试、评测快照验证、vet、diff 检查和 OpenSpec 校验。
- [ ] 2.2 不部署；下一轮抽样评测使用 v3 快照重新验证语义通过率。
