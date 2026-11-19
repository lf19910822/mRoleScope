# Proposal

## Why

“未识别项目”是岗位筛选专用的伪值，当前趋势接口没有对应的正式项目曲线。AI 宽泛项目问题把它送入批量趋势工具后会触发 400，并把整批标记为失败。

## What Changes

- 趋势 AI 的项目正式值列表不再包含筛选专用的 `unrecognized` 伪值。
- 趋势工具规划不会为该伪值生成曲线或批量摘要请求。
- 保持岗位筛选接口继续支持“未识别项目”，并继续展示其他正式项目的最大努力结果。

## Capabilities

### New Capabilities

- `ai-trend-formal-project-values`: 限制 AI 趋势候选为可读取和可应用的正式项目值。

### Modified Capabilities

<!-- No existing repository specs are present; this change introduces the capability above. -->

## Impact

- 影响 AI 趋势 Serving 工具的项目值投影和候选解析。
- 不改变岗位筛选、趋势 API 正式项目读取或前端交互。
- 宽泛项目问题不再因筛选伪值导致批量摘要失败。
