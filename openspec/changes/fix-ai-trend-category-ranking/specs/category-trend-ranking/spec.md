# Category trend ranking

## ADDED Requirements

### Requirement: Default result count is explicit

对于未明确数量的“哪些”“有哪些”“正在使用哪些”类别或不受限维度查询，系统 SHALL 使用产品默认数量 8；明确单项（“哪个”“哪一个”“第一”“使用最多的 X 是什么”）仍 SHALL 使用 1；明确“前 N 个” SHALL 使用 N，并受最多 8 条曲线限制。

#### Scenario: Plural database query uses the default count

- **WHEN** 用户询问“哪些数据库正在被使用？”且没有指定数量
- **THEN** Intent 的有效数量 SHALL 为 8，Draft SHALL 按类别内有效候选排序后最多保留 8 条

#### Scenario: Explicit top one remains top one

- **WHEN** 用户询问“使用最多的数据库是什么？”
- **THEN** 系统 SHALL 只保留排名第一的数据库

### Requirement: Ranking covers the complete formal candidate set

类别型或宽范围排名 SHALL 在服务端为候选正式值读取可用趋势摘要，跨越多个最多 8 项的批次；模型未查询的候选不得因为缺少模型批次而被静默排除。服务端 SHALL 按 Intent 指定指标和顺序排序，再执行有效数量与总曲线容量。

#### Scenario: Category values beyond the first batch are considered

- **WHEN** 一个类别有 10 个正式值而第一批只返回 8 个摘要
- **THEN** 服务端 SHALL 补齐其余 2 个值并将 10 个值一起排名

#### Scenario: One batch item fails

- **WHEN** 补齐摘要时单个候选读取失败
- **THEN** 其他成功候选 SHALL 继续参与排名，Draft SHALL 标记为部分结果并保留失败边界

### Requirement: Candidate provenance identifiers are unique

Draft 曲线的 `sub_query_ids` SHALL 保持稳定输入顺序且不包含重复标识，即使候选经过多次解析、合并或摘要补齐。

#### Scenario: Duplicate candidate merge

- **WHEN** 同一正式值由同一子问题多次解析
- **THEN** 合并后的 Draft 曲线 SHALL 只包含一次该子问题 ID

### Requirement: Exact formal values do not expand to fuzzy neighbors

当 Intent 表达式精确命中当前正式值或正式显示名称时，候选解析 SHALL 只保留该正式值，不得因为子串关系加入其他正式值。类别表达式没有精确值命中时，仍可按类别目录解析候选。

#### Scenario: MongoDB does not add Go

- **WHEN** Intent 包含正式值 `mongodb`
- **THEN** 候选集合 SHALL 包含 MongoDB，且不得因为 `mongodb` 包含 `go` 而加入 Go
