package evaluation

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"mrolescope/internal/aitrend"
)

func TestScoreCaseEnforcesExplicitValuesAndRankingMetric(t *testing.T) {
	intent := aitrend.Intent{SubQueries: []aitrend.SubQuery{{Dimension: aitrend.DimensionTechnology, Expressions: []string{"go", "python"}, Ranking: aitrend.RankingIntent{Metric: aitrend.MetricAverageDailyActiveJobs, Order: aitrend.OrderDescending}}}}
	draft := &aitrend.Draft{Status: aitrend.DraftComplete, Explanation: "Go 和 Python 的岗位数量用于比较。", Curves: []aitrend.DraftCurve{{Condition: aitrend.CurveCondition{Dimension: aitrend.DimensionTechnology, Value: "go"}}, {Condition: aitrend.CurveCondition{Dimension: aitrend.DimensionTechnology, Value: "python"}}}}
	result := Result{Draft: draft, Stats: aitrend.TrendRunStats{ToolCalls: 3, Intent: &intent}, Events: []aitrend.StreamEvent{{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolBatchTrendSummary), Params: json.RawMessage(`[ {"dimension":"technology","value":"go"}, {"dimension":"technology","value":"python"} ]`)}}}
	item := Case{ID: "E01", ExpectedScope: "explicit", ExpectedValues: []string{"go", "python"}, RankingMetric: "avg_daily_active_jobs", RankingOrder: "desc", ExpectedStatus: "complete", ExplanationRequired: true}
	score := ScoreCase(item, result, nil)
	if !score.Pass || !score.ToolCalls.Pass || !score.Scope.Pass || !score.Ranking.Pass || !score.Draft.Pass || !score.Explanation.Pass {
		t.Fatalf("score = %#v", score)
	}
	intent.SubQueries[0].Expressions = []string{"*"}
	result.Stats.Intent = &intent
	result.Events = []aitrend.StreamEvent{{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolBatchTrendSummary), Params: json.RawMessage(`[{"dimension":"technology","value":"*"}]`)}}
	score = ScoreCase(item, result, nil)
	if score.Pass || score.ToolCalls.Pass {
		t.Fatalf("wildcard escaped explicit-value scope: %#v", score)
	}
}

func TestScoreCaseTreatsExplanationAsAdvisory(t *testing.T) {
	intent := aitrend.Intent{SubQueries: []aitrend.SubQuery{{Dimension: aitrend.DimensionTechnology, Expressions: []string{"go"}, Ranking: aitrend.RankingIntent{Metric: aitrend.MetricAverageDailyActiveJobs, Order: aitrend.OrderDescending}}}}
	result := Result{Draft: &aitrend.Draft{Status: aitrend.DraftComplete, Curves: []aitrend.DraftCurve{{Condition: aitrend.CurveCondition{Dimension: aitrend.DimensionTechnology, Value: "go"}}}}, Stats: aitrend.TrendRunStats{ToolCalls: 1, Intent: &intent}, Events: []aitrend.StreamEvent{{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolBatchTrendSummary), Params: json.RawMessage(`[{"dimension":"technology","value":"go"}]`)}}}
	score := ScoreCase(Case{ID: "E01", ExpectedScope: "explicit", ExpectedValues: []string{"go"}, RankingMetric: "avg_daily_active_jobs", RankingOrder: "desc", ExpectedStatus: "complete", ExplanationRequired: true, ExplanationLanguage: "zh-CN"}, result, nil)
	if !score.Pass || score.Explanation.Pass {
		t.Fatalf("advisory explanation changed functional result: %#v", score)
	}
}

func TestScoreCaseRequiresTechnologySubcategoryLookupWhenDeclared(t *testing.T) {
	intent := aitrend.Intent{SubQueries: []aitrend.SubQuery{{Dimension: aitrend.DimensionTechnology, Expressions: []string{"go", "python"}, Ranking: aitrend.RankingIntent{Metric: aitrend.MetricAverageDailyActiveJobs, Order: aitrend.OrderDescending, Limit: 2}}}}
	result := Result{Draft: &aitrend.Draft{Status: aitrend.DraftComplete, Explanation: "Go 和 Python 的岗位数量用于比较。", Curves: []aitrend.DraftCurve{{Condition: aitrend.CurveCondition{Dimension: aitrend.DimensionTechnology, Value: "go"}}, {Condition: aitrend.CurveCondition{Dimension: aitrend.DimensionTechnology, Value: "python"}}}}, Stats: aitrend.TrendRunStats{ToolCalls: 2, Intent: &intent}, Events: []aitrend.StreamEvent{{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolBatchTrendSummary), Params: json.RawMessage(`[{"dimension":"technology","value":"go"}]`)}}}
	item := Case{ID: "E01", ExpectedCategory: "programming-languages", ExpectedScope: "explicit", ExpectedValues: []string{"go", "python"}, RankingMetric: "avg_daily_active_jobs", RankingOrder: "desc", Limit: 2, ExpectedStatus: "complete", ExplanationRequired: true}
	if score := ScoreCase(item, result, nil); score.Scope.Pass {
		t.Fatalf("missing category lookup was accepted: %#v", score)
	}
	result.Events = append(result.Events, aitrend.StreamEvent{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolListCategoryValues), Params: json.RawMessage(`{"category":"programming-languages"}`)})
	if score := ScoreCase(item, result, nil); !score.Scope.Pass {
		t.Fatalf("category lookup was not accepted: %#v", score)
	}
}

func TestScoreCaseAcceptsFormalValuesAfterCategoryLookupAndCandidateShortage(t *testing.T) {
	intent := aitrend.Intent{SubQueries: []aitrend.SubQuery{{Dimension: aitrend.DimensionTechnology, Expressions: []string{"go", "python"}, Ranking: aitrend.RankingIntent{Metric: aitrend.MetricAverageDailyActiveJobs, Order: aitrend.OrderDescending, Limit: 2}}}}
	result := Result{
		Draft: &aitrend.Draft{Status: aitrend.DraftComplete, Explanation: "按编程语言类别整理。", Curves: []aitrend.DraftCurve{
			{Condition: aitrend.CurveCondition{Dimension: aitrend.DimensionTechnology, Value: "go"}},
			{Condition: aitrend.CurveCondition{Dimension: aitrend.DimensionTechnology, Value: "python"}},
		}},
		Stats: aitrend.TrendRunStats{ToolCalls: 3, Intent: &intent},
		Events: []aitrend.StreamEvent{
			{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolListCategoryValues), Params: json.RawMessage(`{"category":"programming-languages"}`)},
			{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolBatchTrendSummary), Params: json.RawMessage(`[{"dimension":"technology","value":"go"},{"dimension":"technology","value":"python"}]`)},
		},
	}
	item := Case{ID: "C01", Dimension: "technology", ExpectedCategory: "programming-languages", ExpectedScope: "category", RankingMetric: "avg_daily_active_jobs", RankingOrder: "desc", Limit: 8, ExpectedStatus: "complete", ExplanationRequired: true}
	if score := ScoreCase(item, result, nil); !score.Scope.Pass || !score.Ranking.Pass || !score.Pass {
		t.Fatalf("formal category result was rejected: %#v", score)
	}
}

func TestScoreCaseAcceptsFixedCapacityForZeroValues(t *testing.T) {
	intent := aitrend.Intent{SubQueries: []aitrend.SubQuery{{Dimension: aitrend.DimensionProject, Expressions: []string{"*"}, Ranking: aitrend.RankingIntent{Metric: aitrend.MetricAverageDailyActiveJobs, Order: aitrend.OrderDescending, Limit: 8}}}}
	curves := make([]aitrend.DraftCurve, 8)
	for index := range curves {
		curves[index] = aitrend.DraftCurve{Condition: aitrend.CurveCondition{Dimension: aitrend.DimensionProject, Value: string(rune('a' + index))}}
	}
	result := Result{Draft: &aitrend.Draft{Status: aitrend.DraftComplete, Curves: curves, Explanation: "按固定顺序保留候选。"}, Stats: aitrend.TrendRunStats{ToolCalls: 1, Intent: &intent}, Events: []aitrend.StreamEvent{{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolBatchTrendSummary), Params: json.RawMessage(`[ {"value":"*"} ]`)}}}
	score := ScoreCase(Case{ID: "D02", ExpectedScope: "global-capacity", ExpectedValues: []string{"*"}, RankingMetric: "avg_daily_active_jobs", RankingOrder: "desc", ExpectedStatus: "complete", ExpectedTotalAtLeast: 8, ExplanationRequired: true}, result, nil)
	if !score.Pass {
		t.Fatalf("zero-valued capacity score = %#v", score)
	}
}

func TestScoreCaseRequiresNamedValueInsideCategoryResult(t *testing.T) {
	intent := aitrend.Intent{SubQueries: []aitrend.SubQuery{{Dimension: aitrend.DimensionTechnology, Expressions: []string{"databases-and-storage"}, Ranking: aitrend.RankingIntent{Metric: aitrend.MetricAverageDailyActiveJobs, Order: aitrend.OrderDescending, Limit: 8}}}}
	result := Result{
		Draft: &aitrend.Draft{Status: aitrend.DraftComplete, Explanation: "列出数据库类别，并确保包含 MySQL。", Curves: []aitrend.DraftCurve{
			{Condition: aitrend.CurveCondition{Dimension: aitrend.DimensionTechnology, Value: "mysql"}},
			{Condition: aitrend.CurveCondition{Dimension: aitrend.DimensionTechnology, Value: "redis"}},
		}},
		Stats: aitrend.TrendRunStats{ToolCalls: 2, Intent: &intent},
		Events: []aitrend.StreamEvent{
			{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolListCategoryValues), Params: json.RawMessage(`{"category":"databases-and-storage"}`)},
			{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolBatchTrendSummary), Params: json.RawMessage(`[{"dimension":"technology","value":"mysql"},{"dimension":"technology","value":"redis"}]`)},
		},
	}
	item := Case{ID: "D05", Dimension: "technology", Expressions: []string{"databases-and-storage"}, ExpectedCategory: "databases-and-storage", RequiredValues: []string{"mysql"}, ExpectedScope: "category-with-required-value", ExpectedValues: []string{}, RankingMetric: "avg_daily_active_jobs", RankingOrder: "desc", Limit: 8, ExpectedStatus: "complete", ExplanationRequired: true}
	if score := ScoreCase(item, result, nil); !score.Pass {
		t.Fatalf("required category value was rejected: %#v", score)
	}
	result.Draft.Curves[0].Condition.Value = "postgresql"
	if score := ScoreCase(item, result, nil); score.Scope.Pass {
		t.Fatalf("missing required category value was accepted: %#v", score)
	}
	result.Draft.Curves[0].Condition.Value = "mysql"
	result.Draft.Curves[0].Condition.Dimension = aitrend.DimensionProject
	if score := ScoreCase(item, result, nil); score.Scope.Pass {
		t.Fatalf("required value from the wrong dimension was accepted: %#v", score)
	}
}

func TestScoreCaseAllowsExpectedClarificationWithoutDraft(t *testing.T) {
	score := ScoreCase(Case{ID: "U04", ExpectedStatus: "failure", ExplanationRequired: true}, Result{}, &aitrend.ContractError{Code: aitrend.ErrorInvalidCombination, Message: "conflicting range"})
	if !score.Pass || score.Explanation.Pass {
		t.Fatalf("expected failure should be functional pass with advisory explanation failure: %#v", score)
	}
}

func TestScoreCaseAcceptsGuidanceOnlyAgentResultWithoutDraft(t *testing.T) {
	noDraft := false
	intent := &aitrend.Intent{}
	result := Result{
		Stats: aitrend.TrendRunStats{ResponseModes: []aitrend.ResponseMode{aitrend.ResponseModeGuidance}, Intent: intent},
		Events: []aitrend.StreamEvent{
			{Type: aitrend.StreamEventAssistant, Status: aitrend.StreamRunning, Delta: "你好，我可以帮助你分析岗位趋势。"},
			{Type: aitrend.StreamEventAssistant, Status: aitrend.StreamCompleted},
			{Type: aitrend.StreamEventDone, Status: aitrend.StreamSuccess},
		},
	}
	item := Case{ID: "G01", Category: "agent", Query: "你好", ExpectedResponseModes: []string{"guidance"}, ExpectedTerminalStatus: "success", ExpectedNoTrendTools: true, ExpectedDraft: &noDraft, ReplyRequired: true}
	score := ScoreCase(item, result, nil)
	if !score.Pass || !score.Agent.Pass || !score.Draft.Pass {
		t.Fatalf("guidance score = %#v", score)
	}
}

func TestScoreCaseGuidanceOnlyWithoutToolsPassesToolCalls(t *testing.T) {
	noDraft := false
	result := Result{Stats: aitrend.TrendRunStats{ResponseModes: []aitrend.ResponseMode{aitrend.ResponseModeGuidance}}, Events: []aitrend.StreamEvent{
		{Type: aitrend.StreamEventAssistant, Status: aitrend.StreamRunning, Delta: "可以帮助你分析趋势。"},
		{Type: aitrend.StreamEventDone, Status: aitrend.StreamSuccess},
	}}
	score := ScoreCase(Case{ID: "G01", ExpectedResponseModes: []string{"guidance"}, ExpectedNoTrendTools: true, ExpectedTerminalStatus: "success", ExpectedDraft: &noDraft, ReplyRequired: true}, result, nil)
	if !score.Pass || !score.ToolCalls.Pass {
		t.Fatalf("guidance-only score = %#v", score)
	}
}

func TestScoreCaseAgentTrendRequiresScopeAndRanking(t *testing.T) {
	result := Result{Draft: &aitrend.Draft{Status: aitrend.DraftComplete, Curves: []aitrend.DraftCurve{{Condition: aitrend.CurveCondition{Dimension: aitrend.DimensionTechnology, Value: "go"}}}}, Stats: aitrend.TrendRunStats{ResponseModes: []aitrend.ResponseMode{aitrend.ResponseModeTrend, aitrend.ResponseModeGuidance}, Intent: &aitrend.Intent{SubQueries: []aitrend.SubQuery{{Dimension: aitrend.DimensionTechnology, Expressions: []string{"go"}}}}}, Events: []aitrend.StreamEvent{{Type: aitrend.StreamEventDone, Status: aitrend.StreamSuccess}}}
	score := ScoreCase(Case{ID: "M02", ExpectedResponseModes: []string{"trend", "guidance"}, ExpectedDraft: func() *bool { value := true; return &value }(), ReplyRequired: true, ExpectedScope: "explicit", ExpectedValues: []string{"go"}, RankingMetric: "avg_daily_active_jobs", RankingOrder: "desc", Limit: 1}, result, nil)
	if score.Pass {
		t.Fatalf("Agent trend accepted missing scope/ranking: %#v", score)
	}
}

func TestScoreCaseSeparatesAgentBoundaryFromAdvisoryReplyQuality(t *testing.T) {
	noDraft := false
	item := Case{ID: "G01", Category: "pure-guidance", Query: "你好", ExpectedResponseModes: []string{"guidance"}, ExpectedTerminalStatus: "success", ExpectedNoTrendTools: true, ExpectedDraft: &noDraft, ReplyRequired: true, ReplyLanguage: "zh-CN", ReplyMaxRunes: 1000}
	result := Result{Stats: aitrend.TrendRunStats{ResponseModes: []aitrend.ResponseMode{aitrend.ResponseModeGuidance}}, Events: []aitrend.StreamEvent{
		{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolBatchTrendSummary)},
		{Type: aitrend.StreamEventAssistant, Status: aitrend.StreamRunning, Delta: strings.Repeat("中", 1001)},
		{Type: aitrend.StreamEventAssistant, Status: aitrend.StreamCompleted},
		{Type: aitrend.StreamEventDone, Status: aitrend.StreamSuccess},
	}}
	score := ScoreCase(item, result, nil)
	if score.Pass || score.Agent.Pass || score.ReplyQuality.Pass {
		t.Fatalf("Agent boundary/advisory failure was not separated: %#v", score)
	}
	if !score.Draft.Pass {
		t.Fatalf("quality warning changed Draft hard score: %#v", score)
	}
}

func TestScorerCoversEveryReviewedBenchmarkCase(t *testing.T) {
	cases, err := LoadFile(filepath.Join("..", "..", "..", "Docs", "开发指导", "AI评测", "benchmarks", "AI趋势智能体Benchmark-v1.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range cases {
		score := ScoreCase(item, Result{}, fmt.Errorf("fixture result unavailable"))
		if score.Agent.Reasons == nil && score.ToolCalls.Reasons == nil && score.Scope.Reasons == nil && score.Ranking.Reasons == nil && score.Draft.Reasons == nil && score.Explanation.Reasons == nil && score.Errors.Reasons == nil {
			t.Fatalf("case %s produced no deterministic score details", item.ID)
		}
	}
}

func TestScoreCaseTreatsServiceTagsAsIndependentHardConstraint(t *testing.T) {
	expectedDraft := false
	item := Case{ID: "G01", Category: "pure-guidance", Query: "你好", ExpectedServiceTags: []string{"guidance"}, ExpectedResponseModes: []string{"guidance"}, ExpectedDraft: &expectedDraft, ReplyRequired: true, ExpectedTerminalStatus: "success"}
	result := Result{Stats: aitrend.TrendRunStats{ServiceTags: []aitrend.ServiceTag{aitrend.ServiceTagTrend}, ResponseModes: []aitrend.ResponseMode{aitrend.ResponseModeGuidance}}, Events: []aitrend.StreamEvent{{Type: aitrend.StreamEventAssistant, Status: aitrend.StreamRunning, Delta: "可以帮助你。"}, {Type: aitrend.StreamEventDone, Status: aitrend.StreamSuccess}}}
	score := ScoreCase(item, result, nil)
	if score.ServiceTags.Pass || score.Pass {
		t.Fatalf("wrong service tags passed: %#v", score)
	}
	result.Stats.ServiceTags = []aitrend.ServiceTag{aitrend.ServiceTagGuidance}
	score = ScoreCase(item, result, nil)
	if !score.ServiceTags.Pass || !score.Pass {
		t.Fatalf("correct service tags failed: %#v", score)
	}
}
