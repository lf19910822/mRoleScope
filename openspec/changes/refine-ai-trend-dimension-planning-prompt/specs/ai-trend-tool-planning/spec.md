# Spec Delta

## Purpose

让趋势规划请求向模型清楚说明可比较的对象、岗位数量指标以及目录和排名工具的范围，使模型有依据保留用户提出的条件，同时继续由服务端合同控制可用能力和结果安全边界。

## ADDED Requirements

### Requirement: Planning guidance explains dimension meaning and scope

趋势规划请求 SHALL 向模型说明 `technology`、`project`、`job_family`、`location` 各自代表的比较对象，并区分岗位数量指标与岗位族分类。指导 SHALL 要求保留用户指定的类别、具体值和限制，并以当前工具返回的正式目录为准。

#### Scenario: Game-engine technology question

- **WHEN** 用户询问游戏引擎技术对应的岗位数量
- **THEN** 规划指导 SHALL 将其作为技术类别范围的示例，并说明不能仅因为问题包含“岗位”而选择岗位族

#### Scenario: Explicit job families are compared

- **WHEN** 用户明确比较前端和后端岗位类别
- **THEN** 规划指导 SHALL 允许选择岗位族，同时说明具体职位名称不能未经确认直接等同于整个岗位族

### Requirement: Planning guidance distinguishes global ranking from scoped evidence

趋势规划请求 SHALL 说明维度全局排名工具不支持按技术类别、排除值或其他维度过滤，并要求按类别候选读取趋势摘要；无法覆盖全部候选或表达跨维度交集时 SHALL 指导模型保留限制和不完整性。

#### Scenario: Category-only evidence

- **WHEN** 用户要求某技术类别内部排名
- **THEN** 规划指导 SHALL 禁止将全局排名前几名冒充完整的类别内部排名

#### Scenario: Cross-dimension condition

- **WHEN** 用户比较使用某项技术的不同岗位族
- **THEN** 规划指导 SHALL 要求保留技术过滤条件，并在工具不支持该交集时说明限制或请求澄清

### Requirement: Continuation guidance reflects available evidence

工具续轮请求 SHALL 指导模型根据用户问题和实际工具结果判断候选范围，类别目录结果本身 SHALL NOT 被描述为已确认具体候选。

#### Scenario: Category directory returned before category values

- **WHEN** 已返回技术类别目录但尚未返回所选类别的正式值
- **THEN** 续轮提示 SHALL 允许继续查询类别值，并且不得声称候选范围已经确认

#### Scenario: Formal candidates are available

- **WHEN** 模型已经取得适用的正式候选但尚未取得趋势摘要
- **THEN** 续轮提示 SHALL 指导模型按用户问题获取所需摘要，而不将未经选择的全局目录当作用户限定范围
