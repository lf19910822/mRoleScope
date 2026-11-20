# Spec Delta

## Purpose

让趋势 AI 根据用户语义自主发现正式类别和查询值，并通过受限工具完成曲线候选规划，同时保证范围精确、调用可审计、结果数量可控和局部失败可交付。

## ADDED Requirements

### Requirement: Model may autonomously plan safe trend tools

趋势分析 SHALL 向模型提供受限的趋势工具定义，并允许模型按顺序调用类别目录、类别值、正式值、排序和趋势摘要工具。模型最终 SHALL 返回符合 `ai-trend-draft-v1` 的 Intent；服务端 SHALL 在每次工具调用和最终 Intent 上执行现有合同校验。

#### Scenario: Model discovers a technology category
- **WHEN** 用户询问“使用最多的编程语言是什么”且没有点名具体语言
- **THEN** 模型可以先调用技术类别目录，再调用编程语言类别值工具，并最终只规划该类别内的候选

#### Scenario: Model skips category discovery for explicit values
- **WHEN** 用户询问“比较 Go 和 Python”
- **THEN** 模型 SHALL 保留 Go 和 Python 两个明确值，可以跳过技术类别目录，不得添加其他技术值

### Requirement: Category scope SHALL constrain candidate discovery

技术类别工具 SHALL 返回当前 Serving 目录中的正式类别值和显示名称。类别值工具 SHALL 接受正式类别值或对应显示名称，并在内部统一为正式类别值；类别查询产生的候选 SHALL 只来自所选类别，不得扩展到相邻类别。

#### Scenario: Display name resolves to a formal category
- **WHEN** 工具收到 `编程语言` 作为类别参数
- **THEN** 工具 SHALL 将其解析为当前目录对应的正式类别，并返回该类别的正式技术值

#### Scenario: Unknown category does not become a wildcard
- **WHEN** 用户给出目录中不存在的技术类别
- **THEN** 系统 SHALL 保留该限制并返回无可用候选或可行动提示，不得将表达式替换为 `*` 或查询整个 technology 维度

### Requirement: Wildcard SHALL represent only an unrestricted dimension

Intent 中的 `expressions: ["*"]` SHALL 仅用于用户确实要求整个维度、且没有类别、具体值或排除限制的子问题。模型和服务端 SHALL 不得使用通配符替代无法解析的词或类别。

#### Scenario: Unrestricted technology question uses wildcard
- **WHEN** 用户询问“使用了哪些技术关键词”且没有类别或具体值限制
- **THEN** 系统可以使用 `expressions: ["*"]` 并读取整个 technology 维度的正式值

#### Scenario: Category question never expands to all technology
- **WHEN** 用户询问“哪些数据库”
- **THEN** 系统 SHALL 只读取数据库类别值，即使模型初始输出了空表达式或通配符，也不得查询其他技术类别

### Requirement: Ranking limits SHALL determine draft capacity per sub-question

系统 SHALL 执行每个子问题的 `ranking.limit`。单项问题 SHALL 只保留一项；“前 N 个” SHALL 最多保留 N 项；未指定数量的类别问题 SHALL 使用产品默认数量；多个子问题共享总曲线容量，并继续遵守现有最多 8 条曲线限制。

#### Scenario: Top-one question returns one curve
- **WHEN** 用户询问“使用最多的编程语言是什么”
- **THEN** 最终草稿中该子问题 SHALL 至多包含 1 条曲线

#### Scenario: Requested count exceeds available candidates
- **WHEN** 用户要求前 8 个但类别只有 3 个有效候选
- **THEN** 系统 SHALL 返回实际可用的 3 条曲线，不得填充其他类别或虚构候选

### Requirement: Tool budgets and partial failures remain enforced

模型工具循环 SHALL 遵守单次请求最多 20 次调用、单次批量最多 8 个候选和既有超时。工具参数非法时 SHALL 将安全校验错误反馈给模型并允许有限重试；参数合法但 Serving 或数据库失败时 SHALL 标记失败、继续其他可行步骤，并保留成功候选。

#### Scenario: Invalid tool arguments are corrected
- **WHEN** 模型提交不符合工具合同的类别或趋势参数
- **THEN** 系统 SHALL 返回不含敏感信息的校验错误，模型可以在既定尝试上限内修正后重试

#### Scenario: One trend item fails inside a batch
- **WHEN** 批量趋势摘要中的一个正式值读取失败而其他值成功
- **THEN** 系统 SHALL 保留成功摘要、标记失败项并生成部分草稿，不得中止整批结果

#### Scenario: Tool budget is exhausted
- **WHEN** 工具调用达到 20 次或请求超时
- **THEN** 系统 SHALL 停止继续调用，使用已获得的有效结果生成完整或部分草稿，并明确结果可能不完整

### Requirement: Intent semantics SHALL keep scope, time, and ranking independent

趋势 Intent 提示词 SHALL 要求模型优先使用具体正式值，其次使用已登记类别，最后才使用不受限维度；时间范围未指定时 SHALL 使用产品默认范围；“本月”“截至今天”等日期表达 SHALL 不自动改变排名指标；未指定比较口径时 SHALL 使用 `avg_daily_active_jobs`。

#### Scenario: Date phrase does not change ranking metric
- **WHEN** 用户询问“本月使用最多的数据库”且未指定比较口径
- **THEN** 系统 SHALL 使用本月作为时间范围，并使用 `avg_daily_active_jobs` 排名

#### Scenario: Explicit peak metric is preserved
- **WHEN** 用户询问“期间单日峰值最高的数据库”
- **THEN** 系统 SHALL 使用 `peak_daily_active_jobs`，不得改写为平均岗位数量
