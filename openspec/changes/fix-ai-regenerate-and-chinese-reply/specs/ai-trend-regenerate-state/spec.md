# Spec Delta

## Purpose

让 AI 重新生成过程只展示当前请求的待确认草稿，并让面向用户的 AI 说明保持简体中文，避免旧结果和语言混杂造成误解。

## ADDED Requirements

### Requirement: Regeneration clears the pending draft

用户提交新的 AI 草稿请求时，前端 SHALL 立即清空上一轮未应用草稿；请求完成后只显示本轮返回的草稿，已应用到看板的曲线不受影响。

#### Scenario: Regenerate after eight curves

- **WHEN** 用户点击“重新生成草稿”
- **THEN** 旧的 8 条待确认曲线立即消失，分析期间不显示旧草稿，成功后显示新结果

#### Scenario: Regeneration fails or stops

- **WHEN** 新请求失败或用户停止分析
- **THEN** 待确认区域保持为空并显示当前请求状态，不恢复上一轮未应用草稿

### Requirement: AI explanation is Chinese and bounded

说明模型 SHALL 使用简体中文生成不超过 100 个 Unicode 字符的说明；返回不含中文字符时系统 SHALL 使用中文确定性回退文本。

#### Scenario: Chinese explanation

- **WHEN** 说明模型返回有效结果
- **THEN** 对话窗口显示简体中文说明

#### Scenario: Non-Chinese model response

- **WHEN** 模型返回英文或其他不含中文的说明
- **THEN** 系统不展示该原文，改用中文回退说明并保留草稿
