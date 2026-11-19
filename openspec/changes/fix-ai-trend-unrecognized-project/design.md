# Design

## Context

`FilterOptionsResponse.Projects` 会额外加入 `unrecognized`，供岗位列表筛选没有项目标签的岗位；`validateProjectTrendQuery` 只验证非空项目值，后续趋势发布组件无法解析该伪值并返回 400。AI 趋势 Runner 直接复用同一项目列表，因此宽泛查询会把它送入 `batch_trend_summary`。

## Goals / Non-Goals

**Goals:**

- 在 AI 趋势工具边界过滤趋势不支持的筛选伪值。
- 继续保留正式项目候选和岗位筛选的原有行为。

**Non-Goals:**

- 不为未识别岗位创建新的趋势聚合口径。
- 不修改 Serving 趋势 API 或数据库。
- 不改变其他后端批量错误的处理策略。

## Decisions

在 AI 趋势的 `resolverFromOptions` 和 `listAvailableValues` 项目投影处过滤 `unrecognized`。这样模型和确定性候选解析都看不到不可应用值，避免仅在批量读取阶段失败；岗位筛选继续使用完整的 `FilterOptionsResponse`。

批量趋势工具同时改为逐项封装摘要或安全错误。编排器识别带错误项的批量结果，保留可用 payload、标记操作失败并让草稿进入 partial 状态。

## Risks / Trade-offs

- [用户无法通过 AI 趋势查看未识别项目曲线] → 当前 Serving 没有该曲线的正式数据口径，继续生成会导致不可应用结果；岗位筛选仍可查看这些岗位。
- [其他未知伪值出现] → 保持过滤逻辑集中，并为当前已知伪值添加回归测试。

## Migration Plan

1. 更新 AI 工具项目值投影和 resolver 测试。
2. 通过受保护 `main` 发布。
3. 真实验证“哪些项目的岗位最多？”不再发送 `unrecognized`，草稿仍可应用。
