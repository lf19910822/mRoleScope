# Design

## Context

AI 状态对象在提交新请求时会清空操作记录，但保留 `draft`，因此旧曲线继续渲染；Qwen 说明提示词当前为英文，模型可能用英文回答。参见 proposal.md 和 `ai-trend-regenerate-state` 规格。

## Goals / Non-Goals

**Goals:**

- 让待确认草稿严格绑定当前请求。
- 保证用户看到的 AI 说明为简体中文。

**Non-Goals:**

- 不删除已经应用到看板的曲线。
- 不改变 AI 候选、工具排序、停止按钮或 token 审计。

## Decisions

- 在 `submit` 进入 running 状态时将 `draft` 置空；后续只接受当前 request ID 的 draft 事件。
- 在说明系统提示词中明确简体中文，并在 Provider 层检查至少一个中文字符；检查失败走现有中文回退文本。

## Risks / Trade-offs

- [新请求失败后用户看不到上一轮未应用结果] → 这是本次明确要求；已应用看板不受影响。
- [中文字符检查可能拒绝只包含项目英文名的有效说明] → 回退文本仍可说明排序和结果，不阻断草稿。

## Migration Plan

1. 更新前端状态、Qwen 提示词和 Provider 校验。
2. 运行前端/后端测试并发布。
3. 实际点击重新生成，确认旧草稿立即清空且新说明为中文。
