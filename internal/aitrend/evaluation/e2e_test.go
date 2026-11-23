package evaluation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mrolescope/internal/aitrend"
)

type scriptedToolProvider struct {
	turns []aitrend.IntentToolTurn
	index int
}

func (provider *scriptedToolProvider) AnalyzeIntent(context.Context, string, *aitrend.ContractError) ([]byte, error) {
	return provider.turns[len(provider.turns)-1].Intent, nil
}

func (provider *scriptedToolProvider) AnalyzeIntentTurn(context.Context, string, *aitrend.ContractError, []aitrend.ToolConversation, bool) (aitrend.IntentToolTurn, error) {
	if provider.index >= len(provider.turns) {
		return aitrend.IntentToolTurn{}, &aitrend.ContractError{Code: aitrend.ErrorMissingField, Message: "fixture provider exhausted"}
	}
	turn := provider.turns[provider.index]
	provider.index++
	return turn, nil
}

func finalGoPythonIntent() []byte {
	intent := aitrend.Intent{ContractVersion: aitrend.ContractVersionV1, SubQueries: []aitrend.SubQuery{{ID: "compare", Dimension: aitrend.DimensionTechnology, Expressions: []string{"go", "python"}, Ranking: aitrend.RankingIntent{Metric: aitrend.MetricAverageDailyActiveJobs, Order: aitrend.OrderDescending, Limit: 2}}}, TimeRange: aitrend.TimeRange{Kind: aitrend.RangeAll}, Output: aitrend.OutputIntent{SeriesMetric: aitrend.SeriesMetricDailyActiveJobs, ChartType: aitrend.ChartTypeLine}}
	data, _ := json.Marshal(intent)
	return data
}

func TestProductionPathExercisesToolDiscoverySummaryValidationAndUsage(t *testing.T) {
	snapshot, err := LoadSnapshot(filepath.Join("testdata", "serving-snapshot-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	provider := &scriptedToolProvider{turns: []aitrend.IntentToolTurn{
		{ToolCalls: []aitrend.ToolCall{{ID: "values", Name: aitrend.ToolListAvailableValues, Params: []byte(`{"dimension":"technology"}`)}}, InputTokens: 17, OutputTokens: 23},
		{ToolCalls: []aitrend.ToolCall{{ID: "summary", Name: aitrend.ToolBatchTrendSummary, Params: []byte(`[{"dimension":"technology","value":"go","range":"all"},{"dimension":"technology","value":"python","range":"all"}]`)}}, InputTokens: 19, OutputTokens: 11},
		{Intent: finalGoPythonIntent(), InputTokens: 13, OutputTokens: 29},
	},
	}
	runner, err := NewRunner(provider, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.Run(context.Background(), "比较 Go 和 Python 的岗位数量")
	if err != nil {
		t.Fatal(err)
	}
	if result.Draft == nil || len(result.Draft.Curves) != 2 || result.Stats.InputTokens != 49 || result.Stats.OutputTokens != 63 {
		t.Fatalf("result = %#v", result)
	}
	if result.FirstEventLatency <= 0 || result.TotalDuration <= 0 {
		t.Fatalf("timing = %s/%s", result.FirstEventLatency, result.TotalDuration)
	}
	seenSummary, seenValidation := false, false
	for _, event := range result.Events {
		if event.Tool == string(aitrend.ToolBatchTrendSummary) && event.Status == aitrend.StreamCompleted {
			seenSummary = true
		}
		if event.Type == aitrend.StreamEventOperation && event.Tool == string(aitrend.ToolBatchValidateDraft) {
			seenValidation = true
		}
	}
	if !seenSummary || !seenValidation {
		t.Fatalf("events did not include summary and draft validation: %#v", result.Events)
	}
}

func TestProductionPathFeedsInvalidToolArgumentsBackToProvider(t *testing.T) {
	snapshot, err := LoadSnapshot(filepath.Join("testdata", "serving-snapshot-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	provider := &scriptedToolProvider{turns: []aitrend.IntentToolTurn{
		{ToolCalls: []aitrend.ToolCall{{ID: "category", Name: aitrend.ToolListCategoryValues, Params: []byte(`{"category":"programming-languages"}`)}}, InputTokens: 1, OutputTokens: 1},
		{ToolCalls: []aitrend.ToolCall{{ID: "corrected", Name: aitrend.ToolListCategoryValues, Params: []byte(`{"category":"programming-languages"}`)}}, InputTokens: 1, OutputTokens: 1},
		{ToolCalls: []aitrend.ToolCall{{ID: "summary", Name: aitrend.ToolBatchTrendSummary, Params: []byte(`[{"dimension":"technology","value":"go","range":"all"},{"dimension":"technology","value":"python","range":"all"}]`)}}, InputTokens: 1, OutputTokens: 1},
		{Intent: finalGoPythonIntent(), InputTokens: 1, OutputTokens: 1},
	},
	}
	runner, err := NewRunner(provider, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	runner.FaultInjection = "invalid_tool_params_once"
	result, err := runner.Run(context.Background(), "比较 Go 和 Python 的岗位数量")
	if err != nil {
		t.Fatal(err)
	}
	if result.Draft == nil || provider.index != 4 {
		t.Fatalf("correction feedback missing: draft=%#v turns=%d", result.Draft, provider.index)
	}
}

func TestProductionPathReportsPartialSummaryFailureWithoutErasingOtherCurves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial-snapshot.json")
	data := []byte(`{
"version":"partial-v1",
"filter_options":{"activity_states":[],"education_levels":[],"experience_years":[],"job_families":[],"locations":[],"main_job_families":[],"other_job_families":[],"projects":[],"technology_keywords":[{"category":{"value":"programming-languages","label":"编程语言"},"value":"go","label":"Go"},{"category":{"value":"programming-languages","label":"编程语言"},"value":"python","label":"Python"}],"urgent_options":[]},
"summaries":[{"dimension":"technology","value":"go","range":"all","label":"Go","average_daily_active_jobs":64,"valid_observation_days":1,"missing_observation_days":0}]
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := LoadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	provider := &scriptedToolProvider{}
	params := []byte(`[{"dimension":"technology","value":"go","range":"all"},{"dimension":"technology","value":"python","range":"all"}]`)
	provider.turns = append(provider.turns,
		aitrend.IntentToolTurn{ToolCalls: []aitrend.ToolCall{{ID: "summary", Name: aitrend.ToolBatchTrendSummary, Params: params}}},
		aitrend.IntentToolTurn{Intent: finalGoPythonIntent()},
	)
	runner, err := NewRunner(provider, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.Run(context.Background(), "比较 Go 和 Python 的岗位数量")
	if err != nil {
		t.Fatal(err)
	}
	if result.Draft == nil || len(result.Draft.Curves) != 1 || result.Draft.Status != aitrend.DraftPartial {
		t.Fatalf("partial result = %#v", result.Draft)
	}
}

func TestProductionPathReturnsTwoIndependentSubqueryGroups(t *testing.T) {
	snapshot, err := LoadSnapshot(filepath.Join("testdata", "serving-snapshot-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	intent := aitrend.Intent{ContractVersion: aitrend.ContractVersionV1, SubQueries: []aitrend.SubQuery{
		{ID: "projects", Dimension: aitrend.DimensionProject, Expressions: []string{"*"}, Ranking: aitrend.RankingIntent{Metric: aitrend.MetricAverageDailyActiveJobs, Order: aitrend.OrderDescending, Limit: 8}},
		{ID: "languages", Dimension: aitrend.DimensionTechnology, Expressions: []string{"programming-languages"}, Ranking: aitrend.RankingIntent{Metric: aitrend.MetricAverageDailyActiveJobs, Order: aitrend.OrderDescending, Limit: 8}},
	}, TimeRange: aitrend.TimeRange{Kind: aitrend.RangeAll}, Output: aitrend.OutputIntent{SeriesMetric: aitrend.SeriesMetricDailyActiveJobs, ChartType: aitrend.ChartTypeLine}}
	intentBytes, err := json.Marshal(intent)
	if err != nil {
		t.Fatal(err)
	}
	provider := &scriptedToolProvider{turns: []aitrend.IntentToolTurn{
		{ToolCalls: []aitrend.ToolCall{{ID: "category", Name: aitrend.ToolListCategoryValues, Params: []byte(`{"category":"programming-languages"}`)}}, InputTokens: 1, OutputTokens: 1},
		{ToolCalls: []aitrend.ToolCall{{ID: "summary", Name: aitrend.ToolBatchTrendSummary, Params: []byte(`[{"dimension":"project","value":"game.genshin-impact","range":"all"},{"dimension":"technology","value":"go","range":"all"}]`)}}, InputTokens: 1, OutputTokens: 1},
		{Intent: intentBytes, InputTokens: 1, OutputTokens: 1},
	}}
	runner, err := NewRunner(provider, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.Run(context.Background(), "哪些项目岗位最多？另外哪些编程语言岗位最多？")
	if err != nil {
		t.Fatal(err)
	}
	if result.Draft == nil || len(result.Draft.Curves) < 2 {
		t.Fatalf("multi-subquery draft = %#v", result.Draft)
	}
	dimensions := map[aitrend.Dimension]bool{}
	for _, curve := range result.Draft.Curves {
		dimensions[curve.Condition.Dimension] = true
	}
	if !dimensions[aitrend.DimensionProject] || !dimensions[aitrend.DimensionTechnology] {
		t.Fatalf("multi-subquery dimensions = %#v", dimensions)
	}
	item := Case{ID: "M03", ExpectedDimensions: []string{"project", "technology"}, ExpectedCategories: map[string]string{"technology": "programming-languages"}, ExpectedSubQueryCount: 2, ExpectedScope: "multiple-subqueries", RankingMetric: "avg_daily_active_jobs", RankingOrder: "desc", Limit: 8, ExpectedStatus: "complete", ExplanationRequired: true}
	score := ScoreCase(item, result, nil)
	if !score.Pass {
		t.Fatalf("multi-subquery score = %#v", score)
	}
}
