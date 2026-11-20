# Spec Delta

## Purpose

让明确的技术类别问题只返回该类别的正式候选，并让 AI 说明在长度限制内完整结束，避免类别污染和句中截断。

## ADDED Requirements

### Requirement: Programming-language questions stay in the language category

当用户询问编程语言时，系统 SHALL 使用编程语言类别正式值匹配，不得把游戏引擎、容器、数据库或其他技术作为该问题的候选。

#### Scenario: Most-used programming languages

- **WHEN** 用户询问“使用最多的编程语言是什么”
- **THEN** 草稿候选只来自编程语言类别，并继续按岗位数量排序

### Requirement: Explanation leaves a completion margin

说明模型 SHALL 被要求在 80 个 Unicode 字符以内完成中文句子；服务端仍执行 100 字上限和中文校验，避免说明在句中被截断。

#### Scenario: Explanation is complete

- **WHEN** 模型解释候选和排序原因
- **THEN** 展示文本在句子边界结束，长度不超过 100 个 Unicode 字符
