package aitrend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mrolescope/internal/jdserving"
)

type runnerOptionsReader struct {
	options jdserving.FilterOptionsResponse
}

func (reader runnerOptionsReader) ListFilterOptions(context.Context) (jdserving.FilterOptionsResponse, error) {
	return reader.options, nil
}

type runnerSummaryReader struct{}

func (runnerSummaryReader) ReadTrendSummary(_ context.Context, query jdserving.TrendQuery) (jdserving.TrendSummary, error) {
	counts := map[string]int{"redis": 12, "mysql": 8, "postgresql": 4}
	count := counts[query.Condition.Value]
	average := float64(count)
	return jdserving.TrendSummary{
		Curve:                  jdserving.TrendCurve{Dimension: jdserving.TrendDimensionTechnology, Value: query.Condition.Value, Label: strings.ToUpper(query.Condition.Value)},
		AverageDailyActiveJobs: &average,
		PeriodEndActiveJobs:    &count,
		ValidObservationDays:   1,
	}, nil
}

type runnerProjectSummaryReader struct{}

func (runnerProjectSummaryReader) ReadTrendSummary(_ context.Context, query jdserving.TrendQuery) (jdserving.TrendSummary, error) {
	counts := map[string]float64{"alpha": 30, "beta": 20, "gamma": 10}
	average := counts[query.Condition.Value]
	return jdserving.TrendSummary{
		Curve:                  jdserving.TrendCurve{Dimension: jdserving.TrendDimensionProject, Value: query.Condition.Value, Label: query.Condition.Value},
		AverageDailyActiveJobs: &average,
		ValidObservationDays:   1,
	}, nil
}

type runnerIntentProvider struct {
	intent Intent
}

func (provider runnerIntentProvider) AnalyzeIntent(context.Context, string, *ContractError) ([]byte, error) {
	return json.Marshal(provider.intent)
}

type runnerExplanationProvider struct {
	intent  Intent
	context DraftExplanationContext
}

func (provider *runnerExplanationProvider) AnalyzeIntent(context.Context, string, *ContractError) ([]byte, error) {
	return json.Marshal(provider.intent)
}

func (provider *runnerExplanationProvider) ExplainDraftUsage(_ context.Context, _ string, explanationContext DraftExplanationContext) (string, int, int, error) {
	provider.context = explanationContext
	return "模型说明：按岗位数量整理候选。", 7, 11, nil
}

func TestTrendRunnerUsesServingValuesAndRanksDraft(t *testing.T) {
	options := jdserving.FilterOptionsResponse{TechnologyKeywords: []jdserving.TechnologyKeywordFilterOption{
		{Category: jdserving.FilterOption{Value: "databases-and-storage", Label: "数据库与存储"}, Value: "redis", Label: "Redis"},
		{Category: jdserving.FilterOption{Value: "databases-and-storage", Label: "数据库与存储"}, Value: "mysql", Label: "MySQL"},
		{Category: jdserving.FilterOption{Value: "databases-and-storage", Label: "数据库与存储"}, Value: "postgresql", Label: "PostgreSQL"},
	}}
	intent := Intent{
		ContractVersion: ContractVersionV1,
		SubQueries: []SubQuery{{
			ID: "databases", Dimension: DimensionTechnology, Expressions: []string{"哪些数据库"},
			Ranking: RankingIntent{Metric: MetricAverageDailyActiveJobs, Order: OrderDescending, Limit: MaxCurves},
		}},
		TimeRange: TimeRange{Kind: RangeMonth},
		Output:    OutputIntent{SeriesMetric: SeriesMetricDailyActiveJobs, ChartType: ChartTypeLine},
	}
	runner, err := NewTrendRunner(runnerIntentProvider{intent: intent}, runnerOptionsReader{options: options}, runnerSummaryReader{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var events []StreamEvent
	if err := runner.Run(context.Background(), "哪些数据库正在被使用？", func(event StreamEvent) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var draft *Draft
	for _, event := range events {
		if event.Type == StreamEventDraft {
			draft = event.Draft
		}
	}
	if draft == nil || len(draft.Curves) != 3 {
		t.Fatalf("draft = %#v", draft)
	}
	if got := draft.Curves[0].Label; got != "Redis" {
		t.Fatalf("first curve = %q, want Redis", got)
	}
	if got := draft.Curves[1].Label; got != "MySQL" {
		t.Fatalf("second curve = %q, want MySQL", got)
	}
	if draft.Curves[0].Condition.Value == "" || draft.Curves[0].MatchStatus != MatchExpansion {
		t.Fatalf("curve identity = %#v", draft.Curves[0])
	}
	last := events[len(events)-1]
	if last.Type != StreamEventDone || last.Status != StreamSuccess {
		t.Fatalf("last event = %#v", last)
	}
}

func TestTrendRunnerEmitsModelExplanationAfterToolResults(t *testing.T) {
	options := jdserving.FilterOptionsResponse{TechnologyKeywords: []jdserving.TechnologyKeywordFilterOption{
		{Category: jdserving.FilterOption{Value: "databases-and-storage", Label: "数据库与存储"}, Value: "redis", Label: "Redis"},
	}}
	intent := Intent{
		ContractVersion: ContractVersionV1,
		SubQueries: []SubQuery{{
			ID: "database", Dimension: DimensionTechnology, Expressions: []string{"Redis"},
			Ranking: RankingIntent{Metric: MetricAverageDailyActiveJobs, Order: OrderDescending, Limit: MaxCurves},
		}},
		TimeRange: TimeRange{Kind: RangeMonth},
		Output:    OutputIntent{SeriesMetric: SeriesMetricDailyActiveJobs, ChartType: ChartTypeLine},
	}
	provider := &runnerExplanationProvider{intent: intent}
	runner, err := NewTrendRunner(provider, runnerOptionsReader{options: options}, runnerSummaryReader{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	events := make([]StreamEvent, 0)
	if err := runner.Run(context.Background(), "哪些数据库正在被使用？", func(event StreamEvent) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	runningIndex, completedIndex, draftIndex := -1, -1, -1
	var draft *Draft
	for index, event := range events {
		if event.Type == StreamEventOperation && event.Tool == string(ToolExplainDraft) && event.Status == StreamRunning {
			runningIndex = index
		}
		if event.Type == StreamEventOperation && event.Tool == string(ToolExplainDraft) && event.Status == StreamCompleted {
			completedIndex = index
		}
		if event.Type == StreamEventDraft {
			draftIndex = index
			draft = event.Draft
		}
	}
	if runningIndex < 0 || completedIndex <= runningIndex || draftIndex <= completedIndex || draft == nil || draft.Explanation != "模型说明：按岗位数量整理候选。" {
		t.Fatalf("events=%#v draft=%#v", events, draft)
	}
	if len(provider.context.Candidates) != 1 || provider.context.Metric != MetricAverageDailyActiveJobs {
		t.Fatalf("explanation context=%#v", provider.context)
	}
}

func TestTrendRunnerUsesAllProjectsForEmptyExpression(t *testing.T) {
	options := jdserving.FilterOptionsResponse{Projects: []jdserving.FilterOption{
		{Value: "alpha", Label: "Alpha"},
		{Value: "beta", Label: "Beta"},
		{Value: "gamma", Label: "Gamma"},
	}}
	intent := Intent{
		ContractVersion: ContractVersionV1,
		SubQueries: []SubQuery{{
			ID: "projects", Dimension: DimensionProject, Expressions: []string{""},
			Ranking: RankingIntent{Metric: MetricAverageDailyActiveJobs, Order: OrderDescending, Limit: MaxCurves},
		}},
		TimeRange: TimeRange{Kind: RangeMonth},
		Output:    OutputIntent{SeriesMetric: SeriesMetricDailyActiveJobs, ChartType: ChartTypeLine},
	}
	runner, err := NewTrendRunner(runnerIntentProvider{intent: intent}, runnerOptionsReader{options: options}, runnerProjectSummaryReader{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var events []StreamEvent
	if err := runner.Run(context.Background(), "哪些项目的岗位最多？", func(event StreamEvent) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var draft *Draft
	for _, event := range events {
		if event.Type == StreamEventDraft {
			draft = event.Draft
		}
		if event.Type == StreamEventOperation && event.Tool == string(ToolRankValues) && strings.Contains(string(event.Params), `"expression":""`) {
			t.Fatalf("empty rank operation emitted: %#v", event)
		}
	}
	if draft == nil || len(draft.Curves) != 3 || draft.Curves[0].Condition.Value != "alpha" || draft.Curves[1].Condition.Value != "beta" || draft.Curves[2].Condition.Value != "gamma" {
		t.Fatalf("draft = %#v", draft)
	}
}

func TestTrendRunnerKeepsSuccessfulCurvesWhenBatchItemFails(t *testing.T) {
	options := jdserving.FilterOptionsResponse{Projects: []jdserving.FilterOption{
		{Value: "good", Label: "Good"},
		{Value: "bad", Label: "Bad"},
	}}
	intent := Intent{
		ContractVersion: ContractVersionV1,
		SubQueries: []SubQuery{{
			ID: "projects", Dimension: DimensionProject, Expressions: []string{"*"},
			Ranking: RankingIntent{Metric: MetricAverageDailyActiveJobs, Order: OrderDescending, Limit: MaxCurves},
		}},
		TimeRange: TimeRange{Kind: RangeAll},
		Output:    OutputIntent{SeriesMetric: SeriesMetricDailyActiveJobs, ChartType: ChartTypeLine},
	}
	runner, err := NewTrendRunner(runnerIntentProvider{intent: intent}, runnerOptionsReader{options: options}, partialBatchSummaryReader{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var draft *Draft
	var terminal StreamStatus
	if err := runner.Run(context.Background(), "哪些项目的岗位最多？", func(event StreamEvent) error {
		if event.Type == StreamEventDraft {
			draft = event.Draft
		}
		if event.Type == StreamEventDone {
			terminal = event.Status
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if draft == nil || len(draft.Curves) != 1 || draft.Curves[0].Condition.Value != "good" || draft.Status != DraftPartial || terminal != StreamPartial {
		t.Fatalf("draft=%#v terminal=%s", draft, terminal)
	}
}

func TestAppendAuditOperationCapturesTerminalFailure(t *testing.T) {
	operations := appendAuditOperation(nil, StreamEvent{Type: StreamEventOperation, Status: StreamRunning, Tool: string(ToolRankValues)})
	operations = appendAuditOperation(operations, StreamEvent{
		Type:   StreamEventOperation,
		Status: StreamFailed,
		Tool:   string(ToolRankValues),
		Error:  &SafeError{Code: ErrorBackendFailure, Message: "safe failure"},
	})
	if len(operations) != 1 || operations[0].Status != StreamFailed || operations[0].SafeErrorCode == nil || *operations[0].SafeErrorCode != string(ErrorBackendFailure) {
		t.Fatalf("operations = %#v", operations)
	}
	if operations[0].EndedAt.IsZero() {
		t.Fatal("terminal operation has no end time")
	}
}

func TestAppendAuditOperationIgnoresModelProgress(t *testing.T) {
	operations := appendAuditOperation(nil, StreamEvent{Type: StreamEventOperation, Status: StreamRunning, Tool: string(ToolAnalyzeIntent)})
	operations = appendAuditOperation(operations, StreamEvent{Type: StreamEventOperation, Status: StreamCompleted, Tool: string(ToolAnalyzeIntent)})
	if len(operations) != 0 {
		t.Fatalf("model progress operations = %#v", operations)
	}
}

func TestQwenIntentProviderDecodesStructuredResponseWithoutLeakingProviderBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("authorization header missing")
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "qwen-plus" {
			t.Fatalf("model = %#v", body["model"])
		}
		if body["max_tokens"] != float64(1200) {
			t.Fatalf("max_tokens = %#v", body["max_tokens"])
		}
		messages, ok := body["messages"].([]any)
		if !ok || len(messages) == 0 || !strings.Contains(messages[0].(map[string]any)["content"].(string), "never put a metric name") || !strings.Contains(messages[0].(map[string]any)["content"].(string), "exactly [\"*\"]") {
			t.Fatalf("intent prompt did not separate expressions from metrics: %#v", body["messages"])
		}
		writer.Header().Set("Content-Type", "application/json")
		response := map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"content": `{"contract_version":"ai-trend-draft-v1","sub_queries":[],"time_range":{"kind":"month"},"output":{"series_metric":"daily_active_jobs","chart_type":"line"}}`}}},
			"usage":   map[string]int{"prompt_tokens": 17, "completion_tokens": 23, "total_tokens": 40},
		}
		_ = json.NewEncoder(writer).Encode(response)
	}))
	defer server.Close()
	provider := &QwenIntentProvider{apiKey: "test-key", endpoint: server.URL, httpClient: server.Client()}
	content, err := provider.AnalyzeIntent(context.Background(), "哪些数据库", nil)
	if err != nil {
		t.Fatalf("AnalyzeIntent() error = %v", err)
	}
	if !strings.Contains(string(content), ContractVersionV1) || strings.Contains(string(content), "test-key") {
		t.Fatalf("unexpected provider content: %s", content)
	}
	_, inputTokens, outputTokens, err := provider.AnalyzeIntentUsage(context.Background(), "哪些数据库", nil)
	if err != nil || inputTokens != 17 || outputTokens != 23 {
		t.Fatalf("usage = %d/%d err=%v", inputTokens, outputTokens, err)
	}
}

func TestQwenIntentProviderGeneratesBoundedDraftExplanation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["max_tokens"] != float64(160) {
			t.Fatalf("max_tokens = %#v", body["max_tokens"])
		}
		messages, ok := body["messages"].([]any)
		if !ok || len(messages) == 0 || !strings.Contains(messages[0].(map[string]any)["content"].(string), "简体中文") {
			t.Fatalf("explanation prompt is not Chinese: %#v", body["messages"])
		}
		writer.Header().Set("Content-Type", "application/json")
		response := map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"content": `{"explanation":"按平均岗位数排序，优先展示岗位最多的项目。"}`}}},
			"usage":   map[string]int{"prompt_tokens": 31, "completion_tokens": 14, "total_tokens": 45},
		}
		_ = json.NewEncoder(writer).Encode(response)
	}))
	defer server.Close()
	provider := &QwenIntentProvider{apiKey: "test-key", endpoint: server.URL, httpClient: server.Client()}
	explanation, inputTokens, outputTokens, err := provider.ExplainDraftUsage(context.Background(), "哪些项目的岗位最多？", DraftExplanationContext{
		Dimension: DimensionProject,
		Metric:    MetricAverageDailyActiveJobs,
		Order:     OrderDescending,
		Candidates: []DraftExplanationCandidate{{
			Label: "原神", MatchStatus: MatchExpansion,
		}},
	})
	if err != nil || explanation == "" || inputTokens != 31 || outputTokens != 14 {
		t.Fatalf("explanation=%q usage=%d/%d err=%v", explanation, inputTokens, outputTokens, err)
	}
}
