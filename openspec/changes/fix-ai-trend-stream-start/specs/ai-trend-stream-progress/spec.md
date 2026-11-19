# Spec Delta

## Purpose

让趋势草稿请求从模型调用开始就持续反馈可观察的操作状态，避免模型首次响应前用户无法判断请求是否已启动或仍在正常等待。

## ADDED Requirements

### Requirement: Model analysis emits an immediate operation event

趋势草稿流 SHALL 在每次结构化意图模型调用开始前发送一个操作事件，事件包含固定的模型操作名称和 `running` 状态，不包含模型原文、逐字推理或凭据。

#### Scenario: First model call is slow

- **WHEN** 用户提交有效趋势问题且模型尚未返回内容
- **THEN** HTTP NDJSON 流先返回模型操作的 `running` 事件，浏览器可以在首个工具调用前显示该操作

#### Scenario: Intent validation requires retry

- **WHEN** 模型返回内容但内容未通过结构化意图校验
- **THEN** 当前模型操作先返回 `completed`，随后返回校验事件并为下一次模型调用重新发送一个 `running` 操作事件

### Requirement: Model operation closes with a safe status

模型调用完成后，流 SHALL 发送同一模型操作的 `completed` 或 `failed` 事件；失败事件只能包含安全错误代码和面向用户的有限消息，且模型操作不得被计入 Serving 工具调用上限或工具调用审计数量。

#### Scenario: Model returns a valid intent

- **WHEN** 模型返回并通过结构化意图校验
- **THEN** 模型操作以 `completed` 结束，随后继续发送已有的 Serving 工具操作事件

#### Scenario: Provider fails before returning intent

- **WHEN** 模型请求超时、提供方返回错误或响应无效
- **THEN** 模型操作以 `failed` 结束，并按现有错误流程发送最终失败事件，不伪造工具调用完成事件

### Requirement: Existing stream consumers remain compatible

已有工具、草稿和终止事件的 JSON 结构及状态语义 SHALL 保持兼容；前端 SHALL 在同一 AI 操作窗口中展示模型操作，并继续支持折叠、停止和最终草稿应用。

#### Scenario: Existing tool stream succeeds

- **WHEN** 模型操作完成后 Serving 工具和草稿流程成功
- **THEN** 浏览器同时看到模型操作、工具操作和最终草稿，最终状态仍为 `success`

#### Scenario: User stops while waiting for model

- **WHEN** 用户在模型操作仍为 `running` 时停止分析
- **THEN** 请求可取消，旧草稿保持不变，服务端不把未执行的模型操作记作成功工具调用
