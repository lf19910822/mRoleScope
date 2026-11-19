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
