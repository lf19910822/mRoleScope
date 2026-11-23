package evaluation

import (
	"testing"
	"time"
)

func TestBuildReportAggregatesCasesAndExcludesInfrastructureFailures(t *testing.T) {
	version, err := NewEvaluationVersion(completeVersionInput(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	makeTrace := func(caseID string, category string, pass bool, status TraceStatus, first, total int64) Trace {
		return Trace{CaseID: caseID, EvaluationVersionID: version.ID(), Status: status, Repetition: 1, Version: version.Metadata(), FirstEventLatencyMS: first, TotalDurationMS: total, InputTokens: 10, OutputTokens: 2, ToolCalls: 2, EstimatedCostNanoRMB: 12, Score: Score{Pass: pass, ToolCalls: passed(), Scope: passed(), Draft: SubScore{Pass: pass}}}
	}
	cases := []Case{{ID: "E01", Category: "explicit-values"}, {ID: "C01", Category: "category-scope"}}
	traces := []Trace{
		makeTrace("E01", "explicit-values", true, TraceSuccess, 10, 100),
		makeTrace("E01", "explicit-values", false, TraceSemanticFailure, 20, 200),
		makeTrace("E01", "explicit-values", true, TraceSuccess, 30, 300),
		makeTrace("C01", "category-scope", false, TraceInfrastructure, 0, 0),
	}
	report, err := BuildReport(cases, traces, version)
	if err != nil {
		t.Fatal(err)
	}
	if report.OverallPassRate != 0.6666666666666666 || report.IncompleteCases != 1 {
		t.Fatalf("report = %#v", report)
	}
	if report.Latency.P50TotalMS != 200 || report.Latency.P95TotalMS != 200 {
		t.Fatalf("latency = %#v", report.Latency)
	}
	if report.TotalToolCalls != 6 || report.TotalInputTokens != 30 {
		t.Fatalf("totals = %#v", report)
	}
}

func TestBuildAgentReportKeepsAgentCategoriesSeparate(t *testing.T) {
	version, err := NewEvaluationVersion(completeVersionInput(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	cases := []Case{
		{ID: "G01", Category: "pure-guidance"},
		{ID: "M01", Category: "mixed-trend-guidance"},
	}
	traces := []Trace{
		{CaseID: "G01", EvaluationVersionID: version.ID(), Status: TraceSuccess, Repetition: 1, Version: version.Metadata(), Score: Score{Pass: true, Agent: passed(), ReplyQuality: passed()}},
		{CaseID: "M01", EvaluationVersionID: version.ID(), Status: TraceSuccess, Repetition: 1, Version: version.Metadata(), Score: Score{Pass: false, Agent: failed("wrong mode"), ReplyQuality: passed()}},
	}
	report, err := BuildAgentReport(cases, traces, version)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.CategoryReports) != len(AgentCategories()) || report.OverallPassRate != 0.5 {
		t.Fatalf("Agent report = %#v", report)
	}
	if len(report.CaseReports) != 2 || report.CaseReports[0].SubScoreRates.Agent != 1 {
		t.Fatalf("Agent sub-scores = %#v", report.CaseReports)
	}
}
