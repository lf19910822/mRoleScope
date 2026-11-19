# Spec Delta

## Purpose

让“哪些项目”“哪些岗位族”等没有点名具体值的问题仍能从当前 Serving 正式值中生成有界、可排序和可应用的曲线草稿，而不是因模型使用空表达式而返回空结果。

## ADDED Requirements

### Requirement: Empty or wildcard expressions use the current dimension vocabulary

当有效意图的某个子问题表达式为空、只包含空白或使用 `*`/`all` 等通配表示时，系统 SHALL 将其解释为该子问题维度的全部当前正式值，并应用现有排除条件、趋势摘要排序和最多 8 条曲线限制。

#### Scenario: Project ranking without a named project

- **WHEN** 用户询问“哪些项目的岗位最多？”且模型表达式为空
- **THEN** 系统从当前正式项目值中生成候选，按岗位数量排序并返回最多 8 条草稿曲线

#### Scenario: Empty expression with exclusions

- **WHEN** 用户询问全部项目但排除一个正式项目
- **THEN** 被排除的项目不进入候选，其余正式项目仍可生成草稿

#### Scenario: Wildcard project ranking

- **WHEN** 模型将“哪些项目”表示为 `*`
- **THEN** 系统按全部当前正式项目生成候选，不把 `*` 当作一个项目值

### Requirement: Broad value queries avoid invalid ranking calls

系统 SHALL 对空表达式、通配表达式和明确表示整个维度的宽泛表达式跳过 `rank_values` 调用，避免向工具发送无效条件；候选的最终排序仍由正式趋势摘要完成。

#### Scenario: No empty rank parameter

- **WHEN** 宽泛项目或岗位族问题进入工具规划
- **THEN** 工具事件中不出现空的 `rank_values.expression`，并继续执行正式值读取和趋势摘要

#### Scenario: No wildcard rank parameter

- **WHEN** 宽泛问题使用 `*` 表达式进入工具规划
- **THEN** 工具事件中不出现 `rank_values.expression = "*"`

### Requirement: Existing specific matching remains unchanged

系统 SHALL 保持具体项目、技术关键词、岗位族或地点表达式的精确/别名/可能匹配行为，以及现有 20 次工具调用和 8 条曲线硬上限。

#### Scenario: Named project remains exact

- **WHEN** 用户询问“原神项目的岗位趋势”
- **THEN** 系统继续优先使用该正式项目值，不将其替换为全部项目扩展候选
