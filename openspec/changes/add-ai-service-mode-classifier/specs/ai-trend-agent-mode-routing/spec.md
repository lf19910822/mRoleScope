# Spec Delta

## Purpose

让服务端根据前置分类结果稳定选择趋势、指导或混合工作流，避免后续趋势规划模型重新判断服务类别并造成模式漂移。

## ADDED Requirements

### Requirement: Derive response modes from service tags

系统 SHALL 根据规范化后的 `service_tags` 生成 Agent 外层 `response_modes`；后续趋势规划模型不得重新决定或覆盖该模式结果。

#### Scenario: Trend-only routing

- **WHEN** `service_tags` 仅包含 `trend`
- **THEN** 服务端生成 `response_modes: ["trend"]` 并进入趋势工具流程

#### Scenario: Mixed routing

- **WHEN** `service_tags` 包含 `trend` 和 `guidance`
- **THEN** 服务端生成 `response_modes: ["trend","guidance"]`，保留趋势流程并启用混合回复

### Requirement: Route guidance-only requests without Draft

当 `response_modes` 仅包含 `guidance` 时，系统 SHALL 跳过趋势工具和 Draft 生成，使用后续回复阶段生成指导内容。

#### Scenario: Guidance-only request

- **WHEN** 用户询问功能或无法提供可执行趋势问题
- **THEN** 系统不调用趋势工具、不发送 Draft，并通过 `assistant_message` 返回指导

### Requirement: Scope example prompts to guidance-only mode

系统 SHALL 仅在 `response_modes` 恰好为 `guidance` 时向回复模型注入可复制的样例问题；包含 `trend` 的模式不得接收样例问题提示词。

#### Scenario: Trend reply has no examples

- **WHEN** `response_modes` 为 `["trend"]` 或 `["trend","guidance"]`
- **THEN** 回复模型只回答当前趋势或用户明确提出的指导内容，不列出无关样例问题

#### Scenario: Guidance reply has examples

- **WHEN** `response_modes` 为 `["guidance"]`
- **THEN** 回复模型可以列出当前支持范围内的可复制问题示例

### Requirement: Return structured feedback for unsupported combinations

当服务端发现已识别 TAG 组合无法由当前工作流处理时，系统 SHALL 生成结构化问题反馈供 AI 回复阶段使用，并保留可交付的服务结果。

#### Scenario: Unsupported combination feedback

- **WHEN** 服务端发现某个 TAG 组合缺少对应处理路径
- **THEN** 系统将安全的结构化限制反馈给 AI，最终回复说明受限部分，不泄露内部错误或工具细节
