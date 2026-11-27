# Spec Delta

## Purpose

让趋势 AI 的交叉统计和单维度草稿保留用户在各子问题中表达的范围，并在可验证条件不一致时阻断不合格结果。

## ADDED Requirements

### Requirement: Scope is established from user intent and formal values

系统 SHALL 分别从用户原话、用户明确选择及同次请求的正式目录建立可验证的子问题范围；模型调用工具或读取类别 SHALL 不单独构成用户确认。

#### Scenario: A model merely inspects a category
- **WHEN** 模型查询了游戏引擎类别但用户没有要求限制为游戏引擎
- **THEN** 系统 SHALL 不得仅因该工具调用将用户问题锁定为该类别

#### Scenario: User explicitly names a category
- **WHEN** 用户明确询问游戏引擎相关岗位且正式目录能解析该类别
- **THEN** 系统 SHALL 将该类别纳入对应子问题的条件，不能更换为全局岗位族或其他技术类别

#### Scenario: Model output conflicts with user wording
- **WHEN** 用户明确询问游戏引擎技术排行，而模型只给出全局岗位族排名
- **THEN** 系统 SHALL 依据独立记录的用户原话与正式目录识别冲突，不能将模型输出或其工具调用作为确认范围的唯一证据

#### Scenario: A category appears in a negated phrase
- **WHEN** 用户明确说“不看游戏引擎”
- **THEN** 系统 SHALL 保留否定关系，不得仅因原话包含类别名称就将游戏引擎确认成正向范围

### Requirement: Planning guidance distinguishes statistics from trend charts

模型实际接收的上下文、规划提示词和工具说明 SHALL 区分当前交叉统计与单维度趋势曲线，并保留已审核的四维语义、范围及排除规则。

#### Scenario: A current cross-statistics question is planned
- **WHEN** 用户询问上海的 Java 岗位数量
- **THEN** 模型请求 SHALL 获得可用交叉统计能力和当前统计口径，不得仅要求生成折线或将图表默认月份和日均指标自动当作文字统计口径

#### Scenario: An explicit request for all ranking items
- **WHEN** 用户要求显示全部符合交叉条件的项目排名
- **THEN** 规划指导 SHALL 区分可超过 8 项的文字结果与最多 8 条的曲线，并保留旧有单维度图表规则

### Requirement: Explicit conditions and exclusions remain attached to their sub-question

系统 SHALL 对每个子问题校验维度、类别、正式值、组合关系与排除条件，确保条件不丢失、不跨子问题借用，也不因模型自由发挥而扩大。

#### Scenario: Technology category and family intersection
- **WHEN** 用户询问游戏引擎相关的后端岗位数
- **THEN** 交叉统计 SHALL 保留游戏引擎和后端两项条件，不得仅统计全局后端岗位

#### Scenario: Excluded technology stays excluded
- **WHEN** 用户要求游戏引擎后端岗位排除 Unity
- **THEN** 统计 SHALL 保留 Unity 排除条件，不得返回包含 Unity 的结果冒充所求范围

#### Scenario: Separate sub-questions
- **WHEN** 一个请求同时包含上海 Java 岗位数和北京 Python 岗位数
- **THEN** 系统 SHALL 保持各自的地点与技术条件，不得互换或合并两个子问题的条件

### Requirement: Category membership constrains ranking and draft candidates

类别问题 SHALL 先确定正式类别的完整可用候选集合，并以此约束对应筛选维度和曲线候选；分组维度 SHALL 按用户要求独立确认，排名结果 SHALL 来自类别范围内的岗位。

#### Scenario: A global ranking returns an unrelated value
- **WHEN** 用户请求游戏引擎技术排行，排序结果混入全局岗位族或非游戏引擎技术
- **THEN** 系统 SHALL 拒绝或移除范围外候选，不得将其加入该子问题的统计或草稿

#### Scenario: A different dimension is the requested grouping
- **WHEN** 用户请求游戏引擎相关岗位的岗位族排名
- **THEN** 系统 SHALL 允许后端等正式岗位族作为分组值，但排名数量必须来自游戏引擎范围内的岗位，不得使用全局岗位族排名冒充

#### Scenario: Category has more candidates than chart capacity
- **WHEN** 正式类别的可用候选超过图表 8 条曲线容量
- **THEN** 系统 SHALL 在类别完整可用范围内统计并排名，之后才按展示容量选择独立曲线

### Requirement: Scope validation actually blocks invalid delivery

系统 SHALL 在交叉统计执行前及草稿发送前校验可确认的范围；校验失败 SHALL 真正阻止不合格查询或草稿交付，而不只是发送一个失败事件。

#### Scenario: Draft validation fails after the failure event is emitted
- **WHEN** 草稿校验失败且失败事件成功发送
- **THEN** 系统 SHALL 不再发送该不合格草稿；失败事件成功发送 SHALL 不得被当成校验成功

#### Scenario: Some independent sub-questions remain valid
- **WHEN** 一个子问题超出已确认范围而另一个子问题的结果已通过校验
- **THEN** 系统可以仅交付已验证部分，并说明由于范围问题缺少的部分，不得交付未通过校验部分

### Requirement: Independent curves retain their own scope

单维度曲线 SHALL 按其自身正式维度和值校验，且 SHALL 明确其为独立趋势；系统 SHALL 不得因曲线未实现交叉过滤而把它伪装成交叉结果。

#### Scenario: Shanghai and Java curves accompany a cross count
- **WHEN** 用户询问上海的 Java 岗位且草稿包含上海、Java 两条独立曲线
- **THEN** 系统 SHALL 可分别验证两条曲线并说明它们是独立趋势，不能要求任何一条曲线具有本期不支持的交叉过滤能力

### Requirement: Unknown conditions do not broaden the query

无法解析、冲突或无法验证的限制 SHALL 不被静默删除、映射为通配符或视为没有过滤条件；系统 SHALL 拒绝相关查询，或只交付独立且已验证的部分。

#### Scenario: Unknown category or empty expansion
- **WHEN** 用户明确要求某技术类别但该类别无法解析，或系统不能确定其正式候选范围
- **THEN** 系统 SHALL 不得将空候选传作“无技术限制”并返回全局岗位数量

#### Scenario: Cross-dimension union is outside supported scope
- **WHEN** 用户明确要求“上海岗位或 Java 岗位”的并集
- **THEN** 系统 SHALL 不得将该表达式改写成交集或全局统计；如可生成独立曲线，须说明其与所求并集不同

### Requirement: Ambiguity is not treated as verified truth

系统 SHALL 区分可由用户原话和正式目录确定的约束与模型推测；无法可靠确定的范围 SHALL 保留不确定性，必要时说明限制或请求澄清。

#### Scenario: A title cannot be mapped confidently
- **WHEN** 用户提到一个不能可靠映射到正式岗位族或技术值的具体职位名称
- **THEN** 系统 SHALL 不得仅凭模型猜测把该职位扩展为整个岗位族并称其为已确认范围

#### Scenario: Same-dimension conjunction has multiple meanings
- **WHEN** 同维度“和”的含义不明但分别、并集和交集都可以计算
- **THEN** 系统 SHALL 按交叉统计规格一次展示三种口径，而不是将其中一种模型猜测当成用户已确认的唯一意图
