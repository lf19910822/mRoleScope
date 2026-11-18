package aitrend

import "testing"

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
