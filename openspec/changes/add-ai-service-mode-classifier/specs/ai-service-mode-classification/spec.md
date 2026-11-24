# Spec Delta

## Purpose

为趋势助手提供一个独立、轻量且可审计的服务类别判断能力，使用户问题先被映射为结构化服务 TAG，再进入后续趋势或指导流程。

## ADDED Requirements

### Requirement: Classify service tags before downstream work

系统 SHALL 在每次请求的后续工作流开始前，根据本次用户原始问题生成结构化 `service_tags` 列表；分类节点不得调用趋势工具、读取 Serving 数据或生成 Draft。

#### Scenario: Trend request classification

- **WHEN** 用户询问“哪些项目的岗位最多？”
- **THEN** 分类节点返回包含 `trend` 的合法 `service_tags`，且在分类完成前不调用趋势工具

#### Scenario: Guidance request classification

- **WHEN** 用户询问“怎么使用这个趋势看板？”
- **THEN** 分类节点返回 `guidance`，且分类阶段不生成 Draft

### Requirement: Validate and normalize service tags

系统 SHALL 拒绝未知 TAG、重复 TAG、空列表和非法结构；对合法 TAG 列表去重，并按固定顺序规范化，当前将 `trend` 排在 `guidance` 前。

#### Scenario: Mixed tags are normalized

- **WHEN** 分类模型返回 `{"service_tags":["guidance","trend","trend"]}`
- **THEN** 服务端保存并传播 `['trend','guidance']` 语义上的去重、规范化结果

#### Scenario: Unknown tag is rejected

- **WHEN** 分类模型返回未登记的 TAG
- **THEN** 分类结果被判定为失败并进入 guidance 降级流程

### Requirement: Degrade classification failures to guidance

分类模型服务失败、超时、非法 JSON、空 TAG 或 TAG 校验失败时，系统 SHALL 将本次请求降级为仅 `guidance`，不调用趋势工具；指导回复成功时请求 SHALL 以成功终态结束。

#### Scenario: Provider failure fallback

- **WHEN** 分类模型请求超时
- **THEN** 系统记录分类失败，跳过趋势工具并尝试生成 guidance 回复

#### Scenario: Guidance fallback completes

- **WHEN** 分类失败后的 guidance 回复成功生成
- **THEN** 系统发送成功终态且不生成空 Draft

### Requirement: Expose bounded classification progress

系统 SHALL 在 AI 操作记录中显示分类节点的进行中、完成或失败状态，但不得展示分类模型原始内容、内部判断过程或完整请求内容；分类节点不计入趋势工具调用上限。

#### Scenario: Classification progress is visible

- **WHEN** 分类节点开始并完成
- **THEN** 流式操作记录依次显示 `classify_service_modes` 的进行中和完成状态

#### Scenario: Classification failure is visible

- **WHEN** 分类节点失败并触发 guidance 降级
- **THEN** 流式操作记录显示分类失败，且不泄露模型原始响应
