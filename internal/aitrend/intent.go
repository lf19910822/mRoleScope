package aitrend

import (
	_ "embed"
	"strings"
)

//go:embed assets/intent_schema_v1.json
var intentSchemaV1 string

const intentPromptV1 = `You translate a user's trend-dashboard question into the strict ai-trend-draft-v1 JSON contract.
The output selects ordinary daily active-job curves. Do not output SQL, database fields, JD text, credentials, daily points, or arbitrary formulas.
Use avg_daily_active_jobs when a ranking has no qualifier. Use period_end_active_jobs for current/end wording, peak_daily_active_jobs for peak wording, and period_unique_jobs only for distinct jobs observed during a period.
Use only technology, project, job_family, and location dimensions. Keep each sub-question independent and preserve explicit exclusions and time ranges.
Expressions must be the user's requested value or category phrase, such as “数据库”, “Redis”, or “后端”; never put a metric name, schema field, or tool name in expressions. For broad category questions such as “哪些项目的岗位最多” or “全部项目”, set the relevant expressions to exactly ["*"]; never use an empty string for that case. For “哪些数据库正在被使用”, use technology with a database-related expression and keep avg_daily_active_jobs only in ranking.metric.`

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
