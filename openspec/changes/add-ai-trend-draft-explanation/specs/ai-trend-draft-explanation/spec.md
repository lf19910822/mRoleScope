# Spec Delta

## Purpose

让用户在工具调用记录之后看到一段基于本次问题、候选和排序结果的 AI 说明，帮助理解草稿选择，同时保持输出长度和隐私边界可控。

## ADDED Requirements

### Requirement: Generate a bounded explanation after tool work

工具调用和草稿校验完成后，系统 SHALL 请求模型生成一段不超过 100 个 Unicode 字符的说明；输入只能包含用户问题、规范化意图、正式候选和有限的 Serving 摘要，不得包含 SQL、JD 正文、凭据或逐字推理。

#### Scenario: Successful explanation

- **WHEN** 候选曲线和工具结果可用
- **THEN** 流式操作记录之后显示模型说明，说明长度不超过 100 个字符并写入草稿 explanation

#### Scenario: No usable curves

- **WHEN** 工具没有返回可用曲线
- **THEN** 模型仍可根据安全的失败摘要说明原因和下一步建议，长度仍不超过 100 个字符

### Requirement: Explanation failures do not discard the draft

说明模型超时、返回无效内容或调用失败时，系统 SHALL 标记说明操作失败，使用确定性短说明继续发送已有草稿，不得因此把可用曲线变成失败请求。

#### Scenario: Provider failure during explanation

- **WHEN** 说明请求失败但草稿已经构建并校验
- **THEN** 用户仍能看到并应用草稿，说明区域显示确定性回退文本

### Requirement: Explanation operation and usage are observable safely

说明阶段 SHALL 作为独立的内部操作事件显示在工具结果之后；该操作不计入 Serving 工具调用上限或工具审计数量，但其输入和输出 token 必须计入现有 AI 审计统计。

#### Scenario: Stream ordering

- **WHEN** 工具调用完成后进入说明阶段
- **THEN** 流先发送说明操作 `running`，再发送 `completed` 或 `failed`，随后发送草稿和最终状态事件
