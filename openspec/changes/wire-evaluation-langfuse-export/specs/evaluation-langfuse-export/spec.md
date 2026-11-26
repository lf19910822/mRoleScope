# Spec Delta

## Purpose

为受控远程 AI 评测提供显式、可验证的 Langfuse 导出边界，使评测结果能够按题目 Session 在 Langfuse 中回放，同时避免未授权或未完整配置时外发评测数据。

## ADDED Requirements

### Requirement: Evaluation export is explicitly opt-in
远程评测服务 MUST 仅在运行时显式启用评测 Langfuse 导出时发送 Trace；未启用时评测仍可生成本地报告，但不得创建 Langfuse 导出请求。

#### Scenario: Export disabled by default
- **WHEN** 评测运行时未设置评测导出开关为 true
- **THEN** 评测完成本地执行和报告写入，且不向 Langfuse 发起请求

#### Scenario: Export enabled explicitly
- **WHEN** 操作者将评测导出开关设置为 true
- **THEN** 评测服务使用受控运行时配置尝试上传带题目级 Session 的 Trace

### Requirement: Complete Langfuse configuration is passed only to evaluation services
启用评测导出时，评测服务 MUST 接收 Langfuse endpoint、project、public key 和 secret key；这些值 MUST 来自受保护的运行时配置，不得出现在命令行参数、公开端口或评测报告明文中。

#### Scenario: Complete configuration is available
- **WHEN** 受控运行时配置包含完整 Langfuse 配置且导出开关为 true
- **THEN** Trend 和 Agent 评测服务均可使用该配置发送其自身评测 Trace

#### Scenario: Configuration is incomplete
- **WHEN** 导出开关为 true 但任一 Langfuse 配置项缺失
- **THEN** 评测在发送前报告配置错误或保持导出失败状态，并且不得以不完整配置发送请求

### Requirement: Evaluation Session identity remains case-scoped
评测导出的每个 Trace MUST 使用 `eval:<run_id>:<suite>:<case_id>` 题目级 Session；同一题目的 repetition 和 retry MUST 共享该 Session，并通过结构化 metadata 区分。

#### Scenario: Repetitions share one case Session
- **WHEN** 同一运行执行同一道题的多个 repetition 或 retry
- **THEN** Langfuse 中这些 Trace 使用相同题目 Session，并保留 repetition、retry 和评测版本 metadata

#### Scenario: Different cases remain isolated
- **WHEN** 同一运行执行两道不同题目
- **THEN** 两道题的 Trace 使用不同 Session
