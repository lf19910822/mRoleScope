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
你可以分析项目、技术关键词、岗位族和地点的每日活跃岗位数量趋势，以及当前有效岗位的受限交叉统计。
交叉统计用于文字回答；图表仍是各正式值的独立单维度曲线，不表示交集或并集。
你不能查询薪资、股票、天气或其他外部数据，不能输出 SQL、JD 原文、凭据或内部实现，也不能自动应用或修改看板。
每次请求都是独立的，不记住其他请求。根据用户问题选择 trend、guidance 或两者；不需要趋势时不要调用趋势工具。回复使用通俗中文。`

//go:embed assets/intent_planning_prompt_v1.txt
var intentPlanningPromptV1 string

func IntentSchemaV1() string { return intentSchemaV1 }

func AgentSchemaV1() string { return agentSchemaV1 }

func AgentContextV1() string { return agentContextV1 }

func IntentPromptV1() string {
	return agentContextV1 + "\n\n" + strings.TrimSpace(intentPlanningPromptV1)
}

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
