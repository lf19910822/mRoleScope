# Proposal

## Why

真实评测已经确认两个质量问题：批量趋势摘要中的单项 Serving 失败会被错误升级为整批失败，丢失仍可用的曲线；评测报告又把基础设施失败的资源消耗排除在成本和延迟统计之外。两者都会让最大努力交付和成本审计失真。

## What Changes

- 保留批量摘要中成功的结果，只有整批没有可用结果时才将工具调用标记为失败。
- 对含有单项失败的批量结果生成部分草稿，同时保留失败项的安全错误信息。
- 将所有已执行 trace 的 token、工具调用、成本和总耗时纳入报告资源统计，功能通过率仍按有效语义结果计算。
- 将首事件延迟定义为初始 `analyze_intent:running` 之后的第一个模型可观察事件，并在没有该事件时保留缺失值语义。

## Capabilities

### New Capabilities

- `ai-evaluation-quality-accounting`: 评测运行的部分结果交付、资源统计和首模型事件延迟口径。

### Modified Capabilities

无。

## Impact

- 影响 `internal/aitrend` 的批量摘要编排和评测 Runner。
- 影响 `internal/aitrend/evaluation` 的 trace/report 聚合。
- 不改变生产 Serving 数据、评测快照格式或模型凭据边界。
