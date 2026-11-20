package aitrend

import (
	_ "embed"
	"strings"
)

//go:embed assets/intent_schema_v1.json
var intentSchemaV1 string

const intentPromptV1 = `将用户趋势问题转换为严格的 ai-trend-draft-v1 JSON 合同。
输出只能选择普通的每日活跃岗位数量折线曲线。不得输出 SQL、数据库字段、JD 正文、凭据、每日数据点或自定义公式。
没有特殊限定时，ranking.metric 使用 avg_daily_active_jobs；“当前/截至目前”使用 period_end_active_jobs；“峰值/最高一天”使用 peak_daily_active_jobs；只有用户明确询问不同岗位时才使用 period_unique_jobs。
只能使用 technology、project、job_family、location 四种维度。每个子问题必须独立，并保留明确的排除条件和时间范围。
expressions 必须填写用户要求匹配的正式值或类别词，例如“数据库”“Redis”“后端”。不得把指标名、数据库字段或工具名放进 expressions。
如果用户询问“哪些项目”“全部项目”等没有点名具体值的宽泛类别，相关 expressions 必须严格写成 ["*"]，不得写空字符串。
如果用户询问“使用最多的编程语言”“哪些编程语言”等编程语言类别，必须使用 technology 维度和“编程语言”类别表达式，只匹配编程语言正式值（如 C++、Python、Go），不得使用 ["*"]，不得返回游戏引擎、容器、数据库或其他非编程语言技术。
如果用户询问“哪些数据库正在被使用”，使用 technology 维度和数据库类别表达式，并且只把 avg_daily_active_jobs 放在 ranking.metric 中。`

func IntentSchemaV1() string { return intentSchemaV1 }

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
