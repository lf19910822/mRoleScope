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
	for _, phrase := range []string{"具体正式值或已确认别名", "已登记类别", "[\"*\"]", "ranking.limit", "产品默认范围 month", "日期表达只决定时间范围", "avg_daily_active_jobs"} {
		if !strings.Contains(prompt, phrase) {
			t.Fatalf("intent prompt missing %q", phrase)
		}
	}
}
