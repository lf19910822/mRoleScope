package evaluation

import (
	"fmt"
	"sort"
)

type SubScoreRates struct {
	Agent        float64 `json:"agent"`
	ToolCalls    float64 `json:"tool_calls"`
	Scope        float64 `json:"scope"`
	Draft        float64 `json:"draft"`
	ReplyQuality float64 `json:"reply_quality"`
}

type CaseReport struct {
	CaseID               string        `json:"case_id"`
	Category             string        `json:"category"`
	PassCount            int           `json:"pass_count"`
	ValidRuns            int           `json:"valid_runs"`
	IncompleteRuns       int           `json:"incomplete_runs"`
	PassRate             float64       `json:"pass_rate"`
	SubScoreRates        SubScoreRates `json:"sub_score_rates"`
	AverageToolCalls     float64       `json:"average_tool_calls"`
	AverageInputToken    float64       `json:"average_input_tokens"`
	AverageOutputToken   float64       `json:"average_output_tokens"`
	AverageCostNanoRMB   float64       `json:"average_cost_nano_rmb"`
	AverageReplyWarnings float64       `json:"average_reply_warning_count"`
}

type CategoryReport struct {
	Category  string  `json:"category"`
	CaseCount int     `json:"case_count"`
	PassRate  float64 `json:"pass_rate"`
}

type LatencyReport struct {
	AverageFirstEventMS float64 `json:"average_first_event_ms"`
	P50FirstEventMS     float64 `json:"p50_first_event_ms"`
	P95FirstEventMS     float64 `json:"p95_first_event_ms"`
	AverageTotalMS      float64 `json:"average_total_ms"`
	P50TotalMS          float64 `json:"p50_total_ms"`
	P95TotalMS          float64 `json:"p95_total_ms"`
}

type Report struct {
	EvaluationVersionID string           `json:"evaluation_version_id"`
	CaseReports         []CaseReport     `json:"case_reports"`
	CategoryReports     []CategoryReport `json:"category_reports"`
	OverallPassRate     float64          `json:"overall_pass_rate"`
	IncompleteCases     int              `json:"incomplete_cases"`
	Latency             LatencyReport    `json:"latency"`
	TotalToolCalls      int              `json:"total_tool_calls"`
	TotalInputTokens    int              `json:"total_input_tokens"`
	TotalOutputTokens   int              `json:"total_output_tokens"`
	TotalCostNanoRMB    int64            `json:"total_cost_nano_rmb"`
	TotalReplyWarnings  int              `json:"total_reply_warnings"`
}

func BuildReport(cases []Case, traces []Trace, version EvaluationVersion) (Report, error) {
	return buildReport(cases, traces, version, Categories())
}

func BuildAgentReport(cases []Case, traces []Trace, version EvaluationVersion) (Report, error) {
	return buildReport(cases, traces, version, AgentCategories())
}

func buildReport(cases []Case, traces []Trace, version EvaluationVersion, categories []string) (Report, error) {
	if version.ID() == "" {
		return Report{}, fmt.Errorf("report evaluation version is empty")
	}
	caseByID := make(map[string]Case, len(cases))
	for _, item := range cases {
		if item.ID == "" {
			return Report{}, fmt.Errorf("report contains a case with empty ID")
		}
		caseByID[item.ID] = item
	}
	byCase := make(map[string][]Trace, len(cases))
	for _, trace := range traces {
		if err := trace.Validate(); err != nil {
			return Report{}, err
		}
		if trace.EvaluationVersionID != version.ID() {
			return Report{}, fmt.Errorf("trace %s belongs to %s, want %s", trace.CaseID, trace.EvaluationVersionID, version.ID())
		}
		if _, ok := caseByID[trace.CaseID]; !ok {
			return Report{}, fmt.Errorf("trace references unknown case %s", trace.CaseID)
		}
		byCase[trace.CaseID] = append(byCase[trace.CaseID], trace)
	}
	report := Report{EvaluationVersionID: version.ID(), CaseReports: make([]CaseReport, 0, len(cases)), CategoryReports: make([]CategoryReport, 0, len(categories))}
	categoryCases := make(map[string][]CaseReport, len(categories))
	casePassRates := make([]float64, 0, len(cases))
	totals := latencyTotals{}
	for _, item := range cases {
		caseReport := summarizeCase(item, byCase[item.ID])
		report.CaseReports = append(report.CaseReports, caseReport)
		categoryCases[item.Category] = append(categoryCases[item.Category], caseReport)
		if caseReport.ValidRuns > 0 {
			casePassRates = append(casePassRates, caseReport.PassRate)
		} else if caseReport.IncompleteRuns > 0 {
			report.IncompleteCases++
		}
	}
	for _, category := range categories {
		entries := categoryCases[category]
		categoryReport := CategoryReport{Category: category, CaseCount: len(entries)}
		for _, entry := range entries {
			if entry.ValidRuns > 0 {
				categoryReport.PassRate += entry.PassRate
			}
		}
		if categoryReport.CaseCount > 0 {
			categoryReport.PassRate /= float64(categoryReport.CaseCount)
		}
		report.CategoryReports = append(report.CategoryReports, categoryReport)
	}
	for _, trace := range traces {
		if trace.Status == TraceIncomplete || trace.Status == TraceInfrastructure {
			continue
		}
		totals.first = append(totals.first, float64(trace.FirstEventLatencyMS))
		totals.total = append(totals.total, float64(trace.TotalDurationMS))
		report.TotalToolCalls += trace.ToolCalls
		report.TotalInputTokens += trace.InputTokens
		report.TotalOutputTokens += trace.OutputTokens
		report.TotalCostNanoRMB += trace.EstimatedCostNanoRMB
		report.TotalReplyWarnings += len(trace.ReplyQualityWarnings)
	}
	if len(casePassRates) > 0 {
		for _, rate := range casePassRates {
			report.OverallPassRate += rate
		}
		report.OverallPassRate /= float64(len(casePassRates))
	}
	report.Latency = summarizeLatency(totals)
	return report, nil
}

func summarizeCase(item Case, traces []Trace) CaseReport {
	report := CaseReport{CaseID: item.ID, Category: item.Category}
	for _, trace := range traces {
		if trace.Status == TraceInfrastructure || trace.Status == TraceIncomplete {
			report.IncompleteRuns++
			continue
		}
		report.ValidRuns++
		if trace.Score.Pass {
			report.PassCount++
		}
		if trace.Score.ToolCalls.Pass {
			report.SubScoreRates.ToolCalls++
		}
		if trace.Score.Agent.Pass {
			report.SubScoreRates.Agent++
		}
		if trace.Score.Scope.Pass {
			report.SubScoreRates.Scope++
		}
		if trace.Score.Draft.Pass {
			report.SubScoreRates.Draft++
		}
		if trace.Score.ReplyQuality.Pass {
			report.SubScoreRates.ReplyQuality++
		}
		report.AverageToolCalls += float64(trace.ToolCalls)
		report.AverageInputToken += float64(trace.InputTokens)
		report.AverageOutputToken += float64(trace.OutputTokens)
		report.AverageCostNanoRMB += float64(trace.EstimatedCostNanoRMB)
		report.AverageReplyWarnings += float64(len(trace.ReplyQualityWarnings))
	}
	if report.ValidRuns > 0 {
		denominator := float64(report.ValidRuns)
		report.PassRate = float64(report.PassCount) / denominator
		report.SubScoreRates.ToolCalls /= denominator
		report.SubScoreRates.Agent /= denominator
		report.SubScoreRates.Scope /= denominator
		report.SubScoreRates.Draft /= denominator
		report.SubScoreRates.ReplyQuality /= denominator
		report.AverageToolCalls /= denominator
		report.AverageInputToken /= denominator
		report.AverageOutputToken /= denominator
		report.AverageCostNanoRMB /= denominator
		report.AverageReplyWarnings /= denominator
	}
	return report
}

type latencyTotals struct {
	first []float64
	total []float64
}

func summarizeLatency(totals latencyTotals) LatencyReport {
	sort.Float64s(totals.first)
	sort.Float64s(totals.total)
	return LatencyReport{
		AverageFirstEventMS: average(totals.first), P50FirstEventMS: percentile(totals.first, 0.50), P95FirstEventMS: percentile(totals.first, 0.95),
		AverageTotalMS: average(totals.total), P50TotalMS: percentile(totals.total, 0.50), P95TotalMS: percentile(totals.total, 0.95),
	}
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func percentile(values []float64, fraction float64) float64 {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * fraction)
	return values[index]
}
