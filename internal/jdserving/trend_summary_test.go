package jdserving

import "testing"

func TestSummarizeTrendSeriesDistinguishesMissingAndZero(t *testing.T) {
	zero := 0
	two := 2
	summary := SummarizeTrendSeries(TrendSeries{Points: []TrendPoint{
		{Date: "2026-11-01", Count: nil},
		{Date: "2026-11-02", Count: &zero},
		{Date: "2026-11-03", Count: &two},
	}})
	if summary.ValidObservationDays != 2 || summary.MissingObservationDays != 1 {
		t.Fatalf("days = valid:%d missing:%d", summary.ValidObservationDays, summary.MissingObservationDays)
	}
	if summary.AverageDailyActiveJobs == nil || *summary.AverageDailyActiveJobs != 1 {
		t.Fatalf("average = %v, want 1", summary.AverageDailyActiveJobs)
	}
	if summary.PeriodEndActiveJobs == nil || *summary.PeriodEndActiveJobs != 2 {
		t.Fatalf("period end = %v, want 2", summary.PeriodEndActiveJobs)
	}
	if summary.PeakDailyActiveJobs == nil || summary.PeakDailyActiveJobs.Date != "2026-11-03" || summary.PeakDailyActiveJobs.Count != 2 {
		t.Fatalf("peak = %#v", summary.PeakDailyActiveJobs)
	}
}

func TestSummarizeTrendSeriesUsesStableEarliestPeakDate(t *testing.T) {
	twoA, twoB := 2, 2
	summary := SummarizeTrendSeries(TrendSeries{Points: []TrendPoint{
		{Date: "2026-11-03", Count: &twoA},
		{Date: "2026-11-02", Count: &twoB},
	}})
	if summary.PeakDailyActiveJobs == nil || summary.PeakDailyActiveJobs.Date != "2026-11-02" {
		t.Fatalf("peak = %#v, want earliest tied date", summary.PeakDailyActiveJobs)
	}
}

func TestTrendSummaryMetricValueReadsEventsAndKeepsUniqueUnavailable(t *testing.T) {
	two := 2
	summary := TrendSummary{AverageDailyActiveJobs: &[]float64{3.5}[0], PeriodUniqueJobs: nil, Events: map[TrendEventType]int{TrendEventEntered: 4}}
	if got, ok := summary.SummaryMetricValue("entered_jobs"); !ok || got != 4 {
		t.Fatalf("entered metric = %v, %t", got, ok)
	}
	if _, ok := summary.SummaryMetricValue("period_unique_jobs"); ok {
		t.Fatal("unique metric unexpectedly available without distinct-job aggregation")
	}
	if _, ok := SummarizeTrendSeries(TrendSeries{Points: []TrendPoint{{Date: "2026-11-01", Count: &two}}}).SummaryMetricValue("avg_daily_active_jobs"); !ok {
		t.Fatal("average metric unavailable")
	}
}
