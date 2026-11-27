# Spec Delta

## Purpose

让趋势 AI 在当前正式岗位数据中计算可追溯的多条件交叉数量和分组排名，并以可信的文字结果补充现有单维度趋势图。

## ADDED Requirements

### Requirement: Cross statistics use current effective jobs

系统 SHALL 基于当前已发布的有效岗位及其正式结构化结果计算交叉统计，并按岗位身份去重；正向交叉条件 SHALL 在同一条当前有效服务记录上同时满足，不能从不同记录拼凑匹配。用户排除项 SHALL 不被同一岗位其他记录绕过。

#### Scenario: Two dimensions match the same job
- **WHEN** 用户询问“上海的 Java 岗位有多少”，某岗位的地点包含上海且技术结果命中 Java
- **THEN** 该岗位 SHALL 计入当前交集；只有上海或只有 Java 的岗位 SHALL 不计入

#### Scenario: Duplicate current observations
- **WHEN** 同一岗位在多个当前观察范围都有满足全部条件的记录
- **THEN** 该岗位在总数中 SHALL 只计一次；处于待复查但仍有效的岗位 SHALL 依照现有有效状态参与统计

#### Scenario: Conditions are split across records
- **WHEN** 同一岗位有两条不同的当前服务记录，一条只满足上海而另一条只满足 Java
- **THEN** 系统 SHALL 不得把两条记录拼成一条同时满足上海和 Java 的记录

#### Scenario: An excluded value occurs in another current record
- **WHEN** 同一岗位的两条当前有效记录都命中 Java，一条标为上海、另一条标为北京，用户要求 Java 岗位排除上海
- **THEN** 系统 SHALL 不得借北京记录将该岗位作为已确认符合排除条件的岗位计入

### Requirement: Supported boolean combinations retain user conditions

系统 SHALL 支持同维度值的并集和交集、不同维度条件的交集及用户明确提出的排除条件。跨维度并集 SHALL 不作为本期交叉统计执行。

#### Scenario: Same-dimension union within a cross-dimension intersection
- **WHEN** 用户查询“上海的 Java 或 Python 岗位”
- **THEN** 系统 SHALL 筛选地点为上海且技术命中 Java 或 Python 的岗位，同一岗位同时命中两种技术时只计一次

#### Scenario: Same-dimension intersection and exclusion
- **WHEN** 用户查询“同时命中 Unity 和 Unreal 的后端岗位，排除上海”
- **THEN** 系统 SHALL 在同一有效岗位上同时应用两个技术条件、后端岗位族条件及排除上海条件

#### Scenario: An excluded dimension is unknown
- **WHEN** 用户要求排除上海，而某岗位没有可判断的地点信息
- **THEN** 系统 SHALL 不得仅因没有上海记录而将该岗位视为已确认符合排除条件；技术分析不可用时仍 SHALL 按既定规则静默跳过

#### Scenario: Cross-dimension union is requested
- **WHEN** 用户查询“上海岗位或 Java 岗位”的合并数量
- **THEN** 系统 SHALL 说明该组合暂不支持，不得将其当成交集或某一条件的全局数量

### Requirement: Ambiguous conjunction exposes all relevant counts

当同维度值之间的“和”无法从上下文确定为分别统计、并集或交集时，系统 SHALL 在同一次回答中提供这三种口径；明确指定关系时 SHALL 按指定关系回答。

#### Scenario: Ambiguous pair of technology values
- **WHEN** 三个后端岗位分别只命中 Unity、只命中 Unreal、同时命中两者，用户问“Unity 和 Unreal 的后端岗位有多少”且上下文未明确关系
- **THEN** 回答 SHALL 给出 Unity 2 个、Unreal 2 个、去重并集 3 个、交集 1 个，并清楚标明各自口径

### Requirement: Job family statistics include primary and secondary matches by default

岗位族筛选和分组 SHALL 默认包含正式主岗位族与副岗位族，并支持用户明确要求仅统计主岗位族。同一岗位在每个分组及总数中最多计一次。

#### Scenario: One job belongs to multiple families
- **WHEN** 某岗位的主岗位族为后端、副岗位族包含运维，且该岗位符合其余筛选条件
- **THEN** 后端和运维排名各可计入该岗位一次，去重后的岗位总数只计一次

### Requirement: Technology filters use formal recognized keywords

技术条件 SHALL 使用正式技术关键词识别结果，文字 SHALL 表述为岗位内容中识别到或命中该技术，不得将关键词命中保证为实际使用或必备技能。

#### Scenario: A keyword appears as a preferred skill
- **WHEN** 有效岗位的正式技术结果命中 Unity，且其原文仅将 Unity 列为优先条件
- **THEN** 该岗位 SHALL 在现有技术关键词口径下参与 Unity 统计，回答 SHALL 不得宣称该岗位必须使用 Unity

### Requirement: Missing technology results are skipped silently

需要技术条件或技术分组时，暂时没有可用技术分析结果的岗位 SHALL 跳过；系统 SHALL 不因此向用户提示或触发告警，并继续交付其他可计算结果。

#### Scenario: Some jobs lack technology analysis
- **WHEN** 系统能够正常统计已识别岗位，且另一些岗位没有可用技术分析结果
- **THEN** 系统 SHALL 使用可用结果完成统计，不因这些岗位缺少结果而提示或告警

#### Scenario: An unrelated query does not require a technology result
- **WHEN** 用户仅按地点和岗位族统计或排名，且某岗位缺少技术分析结果
- **THEN** 该岗位 SHALL 不得仅因缺少技术结果而被排除

### Requirement: Ranking is computed within the complete eligible scope

系统 SHALL 在已确认的完整可用范围内先筛选和分组计数，再按计数排名；排名对象 SHALL 支持技术关键词、岗位族、项目和地点四个维度。

#### Scenario: Rank families within a technology category
- **WHEN** 用户询问“游戏引擎相关岗位中哪些岗位族最多”
- **THEN** 排名 SHALL 依据正式游戏引擎类别内全部可用技术值筛选后的岗位，不能先取全局岗位族前几名再过滤

#### Scenario: Multi-valued groups overlap
- **WHEN** 一个筛选后的岗位同时命中两个技术关键词或两个项目
- **THEN** 两个对应分组可各计该岗位一次，回答 SHALL 不得将各分组数量之和冒充去重岗位总数

#### Scenario: A group exists only on an unmatched record
- **WHEN** 同一岗位的上海 Java 记录属于项目 A、另一条未满足上海条件的记录属于项目 B，用户按项目排名上海 Java 岗位
- **THEN** 该岗位 SHALL 可在项目 A 中计一次，不得因去重后重新关联未匹配记录而计入项目 B

#### Scenario: A filter-only project placeholder is available
- **WHEN** 正式项目排行中出现岗位筛选专用的“未识别项目”伪值
- **THEN** 系统 SHALL 不得将其当作正式项目曲线或默认正式项目排名值

### Requirement: Text ranking limits are independent of chart capacity

文字排名 SHALL 默认展示前 8 项并标明展示范围；用户明确要求更多或全部时 SHALL 按要求提供，不受趋势图最多 8 条曲线的限制。

#### Scenario: Default and explicit larger requests
- **WHEN** 筛选后有 20 个项目，用户分别请求“哪些项目最多”和“显示全部项目”
- **THEN** 前者 SHALL 从全部 20 个项目排名后展示前 8 项，后者 SHALL 提供全部 20 项

#### Scenario: An interrupted result cannot claim completeness
- **WHEN** 用户要求全部排名但查询、传输或预算中断导致无法完整提供
- **THEN** 系统 SHALL 保留可验证的部分结果并明确实际展示范围，不得将部分结果声称为全部或完整排名

### Requirement: User-facing statistics are grounded in verified aggregate results

系统 SHALL 将正式交叉聚合结果提供给 AI，并由程序直接生成数量和排名。AI 的统计结论 SHALL 仅在可确认与同一结果一致时展示；未通过确认的统计内容 SHALL 不得先流式发送。

#### Scenario: Model changes an aggregate number
- **WHEN** 可信统计结果为 12 个而模型输出 21 个或与排名相矛盾的结论
- **THEN** 用户 SHALL 不会收到该未经验证的结论，程序 SHALL 仍可展示可信的 12 个及已验证结果

#### Scenario: Model cannot see every result page
- **WHEN** 完整排名需要多个有界结果页而 AI 只读取其中部分页面
- **THEN** AI SHALL 不得在没有覆盖完整范围的可信汇总时对未读取内容作完整排名结论；程序仍 SHALL 按用户要求提供可交付的结果页

#### Scenario: The AI explains a complete ranking
- **WHEN** 用户要求分析完整排名且结果超过单页模型输入容量
- **THEN** AI SHALL 通过分页结果或由完整交叉结果计算的可信汇总获得足以支持总体结论的证据，不得将局部样本冒充完整数据

### Requirement: Historical questions receive labeled current reference

本期交叉统计 SHALL 仅计算当前有效岗位。用户要求历史交叉数量时，系统 SHALL 说明历史交叉统计不可用，并在当前查询成功时附带明确标为当前的数据作为参考。

#### Scenario: Last-month intersection is requested
- **WHEN** 用户询问上个月同时命中 Unity 和 Unreal 的岗位数
- **THEN** 回答 SHALL 不得将当前交集冒充上个月结果；当前数据可用时 SHALL 标明其为当前参考，历史图若可生成 SHALL 仍为各值的独立趋势

### Requirement: Statistics and independent chart results are delivered separately

交叉统计与单维度图表 SHALL 按各自结果独立交付。交叉统计的局部技术结果缺失与整个交叉查询故障 SHALL 分别处理。

#### Scenario: Statistic succeeds and chart fails
- **WHEN** 交叉统计成功但图表无法生成
- **THEN** 系统 SHALL 仍交付已验证的文字统计结果

#### Scenario: Statistic fails and chart succeeds
- **WHEN** 交叉查询因系统故障整体失败但独立曲线可生成
- **THEN** 系统 SHALL 仍交付曲线并说明交叉数量本次无法计算，不能声称结果为零或编造数量

### Requirement: Charts remain independent single-dimension trends

本期生成的图表 SHALL 沿用现有单维度曲线和最多 8 条曲线的能力；回答 SHALL 说明独立曲线不表示交集或并集，且不得将当前统计等同于历史趋势节点。

#### Scenario: A cross-dimension question produces two curves
- **WHEN** 用户询问“上海的 Java 岗位有多少”且上海与 Java 曲线可生成
- **THEN** 图表可展示上海和 Java 的独立曲线，文字 SHALL 单独给出当前交集并说明曲线不是交集

### Requirement: The model receives only bounded safe aggregate data

交叉工具 SHALL 仅接受白名单内、经过校验的结构化条件，并向模型提供有界的正式标签和聚合结果；JD 正文、岗位标识、SQL 和凭据 SHALL 不进入模型或用户流式工具事件。

#### Scenario: A model requests raw data or arbitrary query text
- **WHEN** 模型提交原始 SQL、岗位标识读取或未注册的维度、值与操作
- **THEN** 系统 SHALL 拒绝该请求并且不向模型或流式事件暴露原始岗位数据
