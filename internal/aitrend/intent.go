package aitrend

import (
	_ "embed"
	"strings"
)

//go:embed assets/intent_schema_v1.json
var intentSchemaV1 string

//go:embed assets/agent_schema_v1.json
var agentSchemaV1 string

const agentContextV1 = `你是 mRoleScope 趋势看板助手。
你只能帮助用户分析项目、技术关键词、岗位族和地点的每日活跃岗位数量趋势。
你不能查询薪资、股票、天气或其他外部数据，不能输出 SQL、JD 原文、凭据或内部实现，也不能自动应用或修改看板。
每次请求都是独立的，不记住其他请求。根据用户问题选择 trend、guidance 或两者；不需要趋势时不要调用趋势工具。回复使用通俗中文。`

const intentPromptV1 = agentContextV1 + `

你是趋势查询规划器。将用户趋势问题转换为符合 ai-trend-agent-v1 的 Agent 响应，并在需要时自主调用已提供的安全工具。

response_modes 可以是 trend、guidance 或同时包含两者。纯 guidance 问题不要调用趋势工具，不要生成空 Draft；部分可支持的问题保留有效趋势并同时 guidance。趋势模式的 trend_intent 必须符合 ai-trend-draft-v1；未知词、未知类别或冲突条件不得改写成 ["*"]。

规划阶段必须遵守交互格式：需要调用工具时，本轮只能返回工具调用，不得同时输出解释文字、Markdown 或普通文本；工具调用完成后，最后一轮只能返回符合 ai-trend-agent-v1 Schema 的 JSON。纯 guidance 问题直接返回 {"contract_version":"ai-trend-agent-v1","response_modes":["guidance"]}，不要调用趋势工具。混合问题先完成工具调用，最后返回 response_modes 为 ["trend","guidance"] 的 Agent JSON，指导说明由后续 assistant_message 阶段生成。任何规划轮都不得把用户可读回复混入工具调用或 Intent JSON。

最终只能规划每日活跃岗位数量折线曲线。不得输出 SQL、数据库字段、JD 正文、凭据、每日数据点或自定义公式。只允许 technology、project、job_family、location 四种维度；每个独立子问题分别确定范围、排名口径、数量、时间、包含和排除条件。

查询范围必须遵守以下优先级：具体正式值或已确认别名 > 已登记类别 > 整个维度。点名具体值时只查询这些值；限定类别但未点名值时，先使用类别目录工具，再使用类别值工具，只查询该类别，例如“编程语言”或“数据库”；只有确实覆盖整个维度且没有类别、具体值或排除限制时，expressions 才能使用 ["*"]。未知词、未知类别或冲突条件不得改写成 ["*"]，应保留限制并交给应用处理。

如果工具目录返回正式值和显示名称，最终 Intent 使用正式值。expressions 只承载匹配对象，不放指标名、数据库字段或工具名。需要比较多个明确值时不得添加相邻或相关值。

未指定时间时使用产品默认范围 month；用户明确给出的时间范围必须保留。日期表达只决定时间范围，不自动改变排名指标。time_range.kind 为 week、month、quarter、year 或 all 时，禁止填写 start_date 和 end_date；只有 kind 为 custom 时才允许同时填写这两个日期。未指定时间时只能输出 {"kind":"month"}，不得从工具结果或其他上下文自行补充日期。未指定比较口径时使用 avg_daily_active_jobs；明确比较最新观测日使用 period_end_active_jobs；明确比较期间单日峰值使用 peak_daily_active_jobs；明确询问期间去重岗位数使用 period_unique_jobs；明确询问进入、修改、下线或重上线数量时使用对应事件指标。

“哪一个”“谁第一”“使用最多的 X 是什么”将 ranking.limit 设为 1；“前 N 个”最多取 N 项；“有哪些”“看看趋势”且未指定数量时使用产品默认数量；用户明确要求全部时保留全量意图，但最终仍受系统曲线容量限制。排名顺序必须与用户问题一致。

在返回最终 Agent 响应前，若趋势候选范围需要类别或正式值，应先完成必要的工具调用并用趋势摘要支持排名；纯 guidance 可以直接返回外层响应。最终必须直接返回符合 ai-trend-agent-v1 Schema 的 JSON，不返回解释文字或模型思维过程。`

func IntentSchemaV1() string { return intentSchemaV1 }

func AgentSchemaV1() string { return agentSchemaV1 }

func AgentContextV1() string { return agentContextV1 }

func IntentPromptV1() string { return intentPromptV1 }

func MetricFromPhrase(phrase string) (Metric, bool) {
	value := strings.ToLower(strings.TrimSpace(phrase))
	switch {
	case strings.Contains(value, "peak"), strings.Contains(value, "峰值"), strings.Contains(value, "最高一天"):
		return MetricPeakDailyActiveJobs, true
	case strings.Contains(value, "end"), strings.Contains(value, "截至目前"), strings.Contains(value, "现在"):
		return MetricPeriodEndActiveJobs, true
	case strings.Contains(value, "unique"), strings.Contains(value, "distinct"), strings.Contains(value, "不同岗位"), strings.Contains(value, "出现过"):
		return MetricPeriodUniqueJobs, true
	case strings.Contains(value, "entered"), strings.Contains(value, "新增"), strings.Contains(value, "进入"):
		return MetricEnteredJobs, true
	case strings.Contains(value, "modified"), strings.Contains(value, "修改"):
		return MetricModifiedJobs, true
	case strings.Contains(value, "left"), strings.Contains(value, "下线"):
		return MetricLeftJobs, true
	case strings.Contains(value, "reopened"), strings.Contains(value, "重上线"), strings.Contains(value, "重新上线"):
		return MetricReopenedJobs, true
	case strings.Contains(value, "relative"), strings.Contains(value, "同比"), strings.Contains(value, "环比"), strings.Contains(value, "增长率"):
		return MetricRelativeChange, true
	case strings.Contains(value, "absolute"), strings.Contains(value, "绝对变化"), strings.Contains(value, "变化量"):
		return MetricAbsoluteChange, true
	case strings.Contains(value, "average"), strings.Contains(value, "平均"):
		return MetricAverageDailyActiveJobs, true
	case strings.Contains(value, "most"), strings.Contains(value, "最多"), strings.Contains(value, "最高"), value == "":
		return MetricAverageDailyActiveJobs, true
	default:
		return "", false
	}
}

func HasConflictingRanges(ranges []RangeKind) bool {
	seen := make(map[RangeKind]struct{}, len(ranges))
	for _, value := range ranges {
		if value == "" {
			continue
		}
		seen[value] = struct{}{}
	}
	return len(seen) > 1
}
