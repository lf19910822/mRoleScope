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
