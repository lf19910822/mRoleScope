package aitrend

import (
	"strings"
	"testing"
)

func TestMetricFromPhraseUsesDeterministicMappings(t *testing.T) {
	tests := []struct {
		phrase string
		metric Metric
	}{
		{phrase: "本月岗位最多的项目", metric: MetricAverageDailyActiveJobs},
		{phrase: "截至目前岗位最多", metric: MetricPeriodEndActiveJobs},
		{phrase: "最高一天", metric: MetricPeakDailyActiveJobs},
		{phrase: "进入岗位最多", metric: MetricEnteredJobs},
		{phrase: "环比增长率", metric: MetricRelativeChange},
	}
	for _, test := range tests {
		got, ok := MetricFromPhrase(test.phrase)
		if !ok || got != test.metric {
			t.Fatalf("MetricFromPhrase(%q) = %q, %t, want %q", test.phrase, got, ok, test.metric)
		}
	}
}

func TestHasConflictingRanges(t *testing.T) {
	if HasConflictingRanges([]RangeKind{RangeMonth, RangeMonth}) {
		t.Fatal("same ranges reported as conflicting")
	}
	if !HasConflictingRanges([]RangeKind{RangeMonth, RangeYear}) {
		t.Fatal("different ranges not reported as conflicting")
	}
}

func TestIntentPromptDefinesGenericScopeAndRankingRules(t *testing.T) {
	prompt := IntentPromptV1()
	for _, phrase := range []string{"具体正式值或已确认别名", "已登记类别", "[\"*\"]", "ranking.limit", "产品默认范围 month", "日期表达只决定时间范围", "avg_daily_active_jobs", "response_modes", "ai-trend-agent-v1", "guidance", "只能返回工具调用", "不得同时输出解释文字", "assistant_message"} {
		if !strings.Contains(prompt, phrase) {
			t.Fatalf("intent prompt missing %q", phrase)
		}
	}
}

func TestAgentContextV1IsStableAndStateless(t *testing.T) {
	first := AgentContextV1()
	second := AgentContextV1()
	if first == "" || first != second {
		t.Fatalf("Agent context is not stable: %q/%q", first, second)
	}
	for _, phrase := range []string{"趋势看板助手", "每日活跃岗位数量", "不记住其他请求", "guidance"} {
		if !strings.Contains(first, phrase) {
			t.Fatalf("Agent context missing %q: %s", phrase, first)
		}
	}
}

func TestAssistantReplyPromptGuidanceProvidesCopyableExamples(t *testing.T) {
	guidancePrompt := assistantReplyPromptForModes([]ResponseMode{ResponseModeGuidance})
	for _, phrase := range []string{"仅包含 guidance", "比较 Go 和 Python 的岗位数量", "哪些项目的岗位最多？", "哪些数据库正在使用？", "不要自动执行示例"} {
		if !strings.Contains(guidancePrompt, phrase) {
			t.Fatalf("assistant reply prompt missing %q", phrase)
		}
	}
	trendPrompt := assistantReplyPromptForModes([]ResponseMode{ResponseModeTrend})
	if strings.Contains(trendPrompt, "比较 Go 和 Python 的岗位数量") || !strings.Contains(trendPrompt, "严禁列出提问示例") {
		t.Fatalf("trend-only reply prompt exposes guidance examples: %s", trendPrompt)
	}
}
