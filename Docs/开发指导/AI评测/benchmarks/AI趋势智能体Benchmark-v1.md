# AI 趋势智能体 Benchmark v1

本文件是首期评测的人工审阅源。每个案例的 JSON 只描述可判定的意图约束和结果边界，不保存模型思维链、凭据或 JD 正文。审阅通过后由评测工具生成机器执行数据。

# 题目格式

每道题由“用户问题”和“评测标准”组成。可以把它理解成一张考试题卡：`query` 是考生看到的题目，其余字段是阅卷规则。`category` 是评测分类，不是 AI 要查询的技术类别。

## 题目与查询范围

| 字段 | 通俗解释 | 例子 |
| --- | --- | --- |
| `id` | 题目编号 | `E01` |
| `query` | 用户原话 | `比较 Go 和 Python 的岗位数量` |
| `category` | 这道题测试哪种能力 | `explicit-values` 表示明确点名具体值 |
| `dimension` | 单子问题去哪个统计维度查询；多子问题题留空，改用 `expected_dimensions` | `technology` 表示技术关键词 |
| `expected_category` | 技术关键词下必须确认的子类别 | `programming-languages` 表示编程语言 |
| `expressions` | AI 计划查询的正式值、类别或 `*` | `['go','python']` |
| `exclusions` | 用户明确要求排除的正式值 | `['redis']` |
| `time_range` | 用户指定的时间范围 | `month`、`all`；冲突时为 `conflict` |
| `expected_time_range` | 评测预期 AI 解析出的时间范围；用户未明确时间时用于检查默认值 | `month` |
| `include_secondary` | 岗位族是否包含副岗位族 | `false` 表示仅主岗位族 |

`expressions` 是“准备查什么”，`expected_values` 是“最终草稿允许留下什么”。明确比较 Go 和 Python 时，两者通常相同；类别查询时，`expected_category` 表示查询边界，`expected_values: []` 表示具体候选由快照排名决定。

## 排名与数量

| 字段 | 通俗解释 | 例子 |
| --- | --- | --- |
| `ranking_metric` | 用什么指标比较 | 见下方指标表 |
| `ranking_order` | 是否要求排序 | `desc` 从高到低；`none` 表示只比较、不要求顺序 |
| `limit` | 用户要求的候选数量 | `2` 表示最多取 2 项 |
| `max_curves` | 看板容量上限 | 固定为 `8`，不是用户要求的数量 |
| `expected_total_at_least` | 至少应交付多少条候选 | `8` |

`limit` 是“用户想看多少”，`max_curves` 是“看板最多放多少”。例如用户要求全部项目，仍然最多展示 8 条，但应说明总候选数量和未展示数量。

### `ranking_metric` 指标

当前 Serving 趋势摘要可以直接返回并用于排名的指标：

| 值 | 含义 |
| --- | --- |
| `avg_daily_active_jobs` | 期间每日活跃岗位平均数量 |
| `period_end_active_jobs` | 期间最后一个观测日的活跃岗位数量 |
| `peak_daily_active_jobs` | 期间单日活跃岗位峰值 |
| `period_unique_jobs` | 期间去重后的岗位数量 |
| `entered_jobs` | 期间进入的岗位数量 |
| `modified_jobs` | 期间发生修改的岗位数量 |
| `left_jobs` | 期间下线的岗位数量 |
| `reopened_jobs` | 期间重新上线的岗位数量 |

以下两个值可以被 Schema 识别，但当前不能直接用于排名：

| 值 | 当前处理方式 |
| --- | --- |
| `absolute_change` | 回退到岗位数量曲线，并提示暂不支持直接比较变化量 |
| `relative_change` | 回退到岗位数量曲线，并提示暂不支持直接比较变化率 |

AI 读取的是 Serving 返回的聚合结果，不读取每日原始数据点，也不自行计算 SQL 或变化公式。

## 结果判定

| 字段 | 通俗解释 | 例子 |
| --- | --- | --- |
| `expected_values` | 最终草稿允许出现的正式值；类别查询的具体候选随快照变化时填写 `[]` | `['go','python']`，不能额外加入 Java |
| `required_values` | 最终草稿必须包含的正式值；用于“类别范围内还要保证某个点名值出现” | `['mysql']` 表示结果可以列出其他数据库，但必须包含 MySQL |
| `expected_scope` | 允许的查询范围 | `explicit` 表示只能查用户点名的值；`category` 表示只能查指定类别；`category-with-required-value` 表示查指定类别且必须包含点名值；`global` 表示该维度下所有子类别 |
| `expected_dimensions` | 多子问题应涉及的维度 | `['project','technology']` |
| `expected_categories` | 多子问题中指定维度必须确认的技术子类 | `{'technology':'programming-languages'}` |
| `expected_subquery_count` | 预期有几个独立子问题 | `2` |
| `expected_status` | 预期结果状态 | `complete`、`partial`、`failure` |
| `duplicates_must_merge` | 重复曲线是否必须合并 | `true` |

## Agent 交互字段（趋势题异常场景）

这些字段用来检查“该不该生成曲线”和用户回复，不改变原有趋势题的范围、排名和 Draft 判定：

| 字段 | 通俗解释 | 例子 |
| --- | --- | --- |
| `expected_response_modes` | 预期 Agent 模式；`trend` 表示生成曲线，`guidance` 表示解释/引导；两者都出现表示混合问题 | `["trend","guidance"]` |
| `expected_service_tags` | 首个分类节点应返回的服务 TAG；`trend` 表示趋势，`guidance` 表示指导；这是独立硬判分项 | `["trend"]` |
| `expected_terminal_status` | 最后 `done` 事件应显示的状态；纯指导正常完成也是 `success` | `partial` |
| `expected_no_trend_tools` | 是否禁止趋势工具；纯指导题设为 `true` | `true` |
| `expected_draft` | 是否必须有 Draft；指导-only 为 `false` | `true` |
| `reply_required` | 是否必须有 `assistant_message` 回复 | `true` |
| `reply_language` | 回复语言的检查口径 | `zh-CN` |
| `reply_max_runes` | 回复最多允许的 Unicode 字符数 | `1000` |

模式、工具边界、回复存在性和 Draft 是否存在是硬判定；中文、长度、完整性和限制说明是流结束后的 advisory 质量记录，不会回滚已经展示的合法 Draft。

正式值是系统内部稳定 ID，例如 `go`、`game.genshin-impact`；用户界面仍显示“Go”“原神”等正式名称。

## 异常与解释

| 字段 | 通俗解释 | 例子 |
| --- | --- | --- |
| `expected_errors` | 预期出现的错误、限制或修正 | `['unavailable_value']` |
| `fault_injection` | 评测器是否主动注入一次可控故障 | `invalid_tool_params_once`；仅用于评测，不是用户输入 |
| `explanation_required` | 是否必须给出选择理由 | `true` |
| `explanation_language` | 解释使用的语言 | `zh-CN` |
| `explanation_max_runes` | 解释最多允许多少字符 | `100` |

例如未知值题可以要求 `partial`：找不到的候选要说明，但仍保留找到的有效曲线。解释只做辅助检查，不要求固定句式。


## 具体值（5题）

### E01 比较 Go 和 Python

```json
{"id":"E01","category":"explicit-values","query":"比较 Go 和 Python 的岗位数量","dimension":"technology","expressions":["go","python"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"none","limit":2,"expected_values":["go","python"],"expected_scope":"explicit","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

### E02 比较 Redis 和 MySQL

```json
{"id":"E02","category":"explicit-values","query":"比较 Redis 和 MySQL 的岗位趋势","dimension":"technology","expressions":["redis","mysql"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"none","limit":2,"expected_values":["redis","mysql"],"expected_scope":"explicit","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

### E03 比较两个项目

```json
{"id":"E03","category":"explicit-values","query":"比较原神和绝区零的岗位数量","dimension":"project","expressions":["game.genshin-impact","game.zenless-zone-zero"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"none","limit":2,"expected_values":["game.genshin-impact","game.zenless-zone-zero"],"expected_scope":"explicit","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

### E04 比较前端和后端岗位族

```json
{"id":"E04","category":"explicit-values","query":"比较前端和后端岗位族的岗位数量","dimension":"job_family","expressions":["frontend","backend"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"none","limit":2,"expected_values":["frontend","backend"],"expected_scope":"explicit","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

### E05 比较两个地点

```json
{"id":"E05","category":"explicit-values","query":"比较上海和北京的岗位趋势","dimension":"location","expressions":["上海","北京"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"none","limit":2,"expected_values":["上海","北京"],"expected_scope":"explicit","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

## 类别与子类别范围（5题）

### C01 编程语言类别

```json
{"id":"C01","category":"category-scope","query":"编程语言中哪些岗位最多？","dimension":"technology","expressions":["programming-languages"],"expected_category":"programming-languages","ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":8,"expected_values":[],"expected_scope":"category","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

### C02 数据库类别

```json
{"id":"C02","category":"category-scope","query":"数据库技术有哪些？","dimension":"technology","expressions":["databases-and-storage"],"expected_category":"databases-and-storage","ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":8,"expected_values":[],"expected_scope":"category","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

### C03 游戏引擎类别

```json
{"id":"C03","category":"category-scope","query":"游戏引擎里哪些技术岗位最多？","dimension":"technology","expressions":["game-engines"],"expected_category":"game-engines","ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":8,"expected_values":[],"expected_scope":"category","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

### C04 云原生类别

```json
{"id":"C04","category":"category-scope","query":"容器和云原生组件的岗位趋势有哪些？","dimension":"technology","expressions":["cloud-native"],"expected_category":"cloud-native","ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":8,"expected_values":[],"expected_scope":"category","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

### C05 编程语言中前两名

```json
{"id":"C05","category":"category-scope","query":"编程语言里岗位最多的前两个是什么？","dimension":"technology","expressions":["programming-languages"],"expected_category":"programming-languages","ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":2,"expected_values":[],"expected_scope":"category","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

## 全范围与排名（5题）

### R01 项目岗位最多

```json
{"id":"R01","category":"global-ranking","query":"哪些项目的岗位最多？","dimension":"project","expressions":["*"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":8,"expected_values":["*"],"expected_scope":"global","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

### R02 使用最多的技术关键词

```json
{"id":"R02","category":"global-ranking","query":"使用最多的技术关键词是什么？","dimension":"technology","expressions":["*"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":1,"expected_values":["*"],"expected_scope":"global","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

### R03 前五个岗位族

```json
{"id":"R03","category":"global-ranking","query":"岗位族中岗位数量最多的前5个是什么？","dimension":"job_family","expressions":["*"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":5,"expected_values":["*"],"expected_scope":"global","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

### R04 最新观测日最高项目

```json
{"id":"R04","category":"global-ranking","query":"截至最新观测日，哪个项目的活跃岗位数最高？","dimension":"project","expressions":["*"],"ranking_metric":"period_end_active_jobs","ranking_order":"desc","limit":1,"expected_values":["*"],"expected_scope":"global","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

### R05 期间峰值最高地点

```json
{"id":"R05","category":"global-ranking","query":"这段时间单日岗位峰值最高的地点是哪里？","dimension":"location","expressions":["*"],"expected_time_range":"month","ranking_metric":"peak_daily_active_jobs","ranking_order":"desc","limit":1,"expected_values":["*"],"expected_scope":"global","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

## 多条件、包含排除与时间（5题）

### M01 排除一个数据库

```json
{"id":"M01","category":"compound-conditions","query":"数据库技术中排除 Redis 后，哪些岗位最多？","dimension":"technology","expressions":["databases-and-storage"],"expected_category":"databases-and-storage","exclusions":["redis"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":8,"expected_values":[],"expected_scope":"category-with-exclusion","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

### M02 项目和技术的组合问题

```json
{"id":"M02","category":"compound-conditions","query":"原神项目使用哪些技术？","dimension":"technology","expressions":["*"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":8,"expected_values":["*"],"expected_scope":"global-technology-with-project-note","max_curves":8,"expected_status":"partial","expected_errors":["unsupported_cross_dimension_filter"],"expected_service_tags":["trend"],"explanation_required":true}
```

### M03 两个独立子问题

```json
{"id":"M03","category":"compound-conditions","query":"哪些项目岗位最多？另外哪些编程语言岗位最多？","expected_dimensions":["project","technology"],"expected_categories":{"technology":"programming-languages"},"expected_subquery_count":2,"ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":8,"expected_values":["*"],"expected_scope":"multiple-subqueries","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

### M04 本月项目排名

```json
{"id":"M04","category":"compound-conditions","query":"本月岗位最多的项目有哪些？","dimension":"project","expressions":["*"],"time_range":"month","ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":8,"expected_values":["*"],"expected_scope":"global","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

### M05 仅主岗位族

```json
{"id":"M05","category":"compound-conditions","query":"只看主岗位族，哪些岗位族的岗位最多？","dimension":"job_family","expressions":["*"],"include_secondary":false,"ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":8,"expected_values":["*"],"expected_scope":"global-primary-only","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

## 未知、暂不支持与错误恢复（5题）

### U01 未知技术值

```json
{"id":"U01","category":"unsupported-recovery","query":"比较量子数据库和 Redis 的岗位数量","dimension":"technology","expressions":["量子数据库","redis"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"none","limit":2,"expected_values":["redis"],"expected_scope":"partial-unknown","max_curves":8,"expected_status":"partial","expected_errors":["unavailable_value"],"expected_response_modes":["trend","guidance"],"expected_service_tags":["trend","guidance"],"expected_terminal_status":"partial","expected_draft":true,"reply_required":true,"reply_language":"zh-CN","reply_max_runes":1000,"explanation_required":true}
```

### U02 不支持占比纵轴

```json
{"id":"U02","category":"unsupported-recovery","query":"哪些项目的岗位占比最高？","dimension":"project","expressions":["*"],"ranking_metric":"unsupported_share","ranking_order":"desc","limit":8,"expected_values":["*"],"expected_scope":"unsupported-metric-fallback","max_curves":8,"expected_status":"partial","expected_errors":["unsupported_metric"],"expected_response_modes":["trend","guidance"],"expected_service_tags":["trend","guidance"],"expected_terminal_status":"partial","expected_draft":true,"reply_required":true,"reply_language":"zh-CN","reply_max_runes":1000,"explanation_required":true}
```

### U03 不支持柱状图

```json
{"id":"U03","category":"unsupported-recovery","query":"用柱状图展示岗位最多的项目","dimension":"project","expressions":["*"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":8,"expected_values":["*"],"expected_scope":"global-line-chart-fallback","max_curves":8,"expected_status":"partial","expected_errors":["unsupported_chart_type"],"expected_response_modes":["trend","guidance"],"expected_service_tags":["trend","guidance"],"expected_terminal_status":"partial","expected_draft":true,"reply_required":true,"reply_language":"zh-CN","reply_max_runes":1000,"explanation_required":true}
```

### U04 冲突时间范围

```json
{"id":"U04","category":"unsupported-recovery","query":"本月和去年哪个项目岗位更多？","time_range":"conflict","expected_response_modes":["guidance"],"expected_service_tags":["guidance"],"expected_terminal_status":"success","expected_no_trend_tools":true,"expected_draft":false,"reply_required":true,"reply_language":"zh-CN","reply_max_runes":1000}
```

### U05 非法工具参数修正

```json
{"id":"U05","category":"unsupported-recovery","query":"比较 Go 和 Python 的岗位数量，并校验草稿","dimension":"technology","expressions":["go","python"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"none","limit":2,"expected_values":["go","python"],"expected_scope":"explicit-after-correction","max_curves":8,"expected_status":"complete","expected_errors":["invalid_tool_params_then_corrected"],"fault_injection":"invalid_tool_params_once","expected_response_modes":["trend","guidance"],"expected_service_tags":["trend","guidance"],"expected_terminal_status":"success","expected_draft":true,"reply_required":true,"reply_language":"zh-CN","reply_max_runes":1000,"explanation_required":true}
```

## 草稿、容量与解释校验（5题）

### D01 超过八条候选

```json
{"id":"D01","category":"draft-validation","query":"列出所有项目的岗位趋势","dimension":"project","expressions":["*"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":20,"expected_values":["*"],"expected_scope":"global-capacity","max_curves":8,"expected_status":"complete","expected_total_at_least":8,"expected_service_tags":["trend"],"explanation_required":true}
```

### D02 全零候选也按固定顺序

```json
{"id":"D02","category":"draft-validation","query":"列出数据库技术岗位趋势，结果中允许出现零岗位值","dimension":"technology","expressions":["databases-and-storage"],"expected_category":"databases-and-storage","ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":8,"expected_values":[],"expected_scope":"category-zero-values","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

### D03 别名归一化

```json
{"id":"D03","category":"draft-validation","query":"比较 Golang 和 Python","dimension":"technology","expressions":["go","python"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"none","limit":2,"expected_values":["go","python"],"expected_scope":"alias-normalized","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```

### D04 重复候选合并

```json
{"id":"D04","category":"draft-validation","query":"项目岗位最多和哪些项目岗位最多？","dimension":"project","expressions":["*"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":8,"expected_values":["*"],"expected_scope":"duplicate-subqueries-merged","max_curves":8,"expected_status":"complete","duplicates_must_merge":true,"expected_service_tags":["trend"],"explanation_required":true}
```

### D05 类别结果包含点名值

```json
{"id":"D05","category":"draft-validation","query":"哪些数据库正在使用？尤其是 MySQL？","dimension":"technology","expressions":["databases-and-storage"],"expected_category":"databases-and-storage","required_values":["mysql"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":8,"expected_values":[],"expected_scope":"category-with-required-value","max_curves":8,"expected_status":"complete","expected_service_tags":["trend"],"explanation_required":true}
```
