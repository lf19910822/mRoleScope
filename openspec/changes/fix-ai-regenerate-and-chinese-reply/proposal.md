# Proposal

## Why

当前 AI 说明可能返回英文，且点击“重新生成草稿”时旧草稿会继续留在右侧，用户无法分辨正在等待的新结果还是上一轮结果。

## What Changes

- 要求说明模型只返回简体中文；非中文说明回退为中文确定性说明。
- 新请求开始时立即清空旧待确认草稿，完成后再显示新草稿。
- 保持已应用看板曲线不变，重新生成只影响待确认草稿区。

## Capabilities

### New Capabilities

- `ai-trend-regenerate-state`: 重新生成期间的草稿清空和中文说明约束。

### Modified Capabilities

<!-- No existing repository specs are present; this change introduces the capability above. -->

## Impact

- 影响 AI 说明提示词与前端草稿状态管理。
- 不改变曲线候选、工具调用、草稿应用和已应用看板数据。
