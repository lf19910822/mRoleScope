package aitrend

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mrolescope/internal/jdserving"
)

type runnerOptionsReader struct {
	options jdserving.FilterOptionsResponse
}

type failingRunnerOptionsReader struct{}

func (failingRunnerOptionsReader) ListFilterOptions(context.Context) (jdserving.FilterOptionsResponse, error) {
	return jdserving.FilterOptionsResponse{}, fmt.Errorf("options backend unavailable")
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

func TestTrendRunnerGuidanceOnlyDoesNotBuildDraft(t *testing.T) {
	provider := &scriptedToolProvider{turns: []IntentToolTurn{{Intent: []byte(`{"contract_version":"ai-trend-agent-v1","response_modes":["guidance"]}`)}}}
	runner, err := NewTrendRunner(provider, runnerOptionsReader{}, runnerSummaryReader{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var events []StreamEvent
	if err := runner.Run(context.Background(), "你好", func(event StreamEvent) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.Type == StreamEventDraft {
			t.Fatalf("guidance-only request emitted Draft: %#v", events)
		}
	}
	foundMessage, foundSuccess := false, false
	for _, event := range events {
		if event.Type == StreamEventAssistant && event.Status == StreamRunning && event.Delta != "" {
			foundMessage = true
		}
		if event.Type == StreamEventDone && event.Status == StreamSuccess {
			foundSuccess = true
		}
	}
	if !foundMessage || !foundSuccess {
		t.Fatalf("guidance events = %#v", events)
	}
}

func TestTrendRunnerGuidanceIgnoresAccidentalTrendToolResult(t *testing.T) {
	provider := &scriptedToolProvider{turns: []IntentToolTurn{
		{ToolCalls: []ToolCall{{ID: "values", Name: ToolListAvailableValues, Params: []byte(`{"dimension":"technology"}`)}}, InputTokens: 1, OutputTokens: 1},
		{Intent: []byte(`{"contract_version":"ai-trend-agent-v1","response_modes":["guidance"]}`), InputTokens: 1, OutputTokens: 1},
	}}
	runner, err := NewTrendRunner(provider, runnerOptionsReader{}, runnerSummaryReader{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var events []StreamEvent
	if err := runner.Run(context.Background(), "你好", func(event StreamEvent) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	foundTool, foundDraft, foundSuccess := false, false, false
	for _, event := range events {
		if event.Type == StreamEventOperation && event.Tool == string(ToolListAvailableValues) {
			foundTool = true
		}
		if event.Type == StreamEventDraft {
			foundDraft = true
		}
		if event.Type == StreamEventDone && event.Status == StreamSuccess {
			foundSuccess = true
		}
	}
	if !foundTool || foundDraft || !foundSuccess {
		t.Fatalf("guidance accidental-tool events = %#v", events)
	}
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

type languageSummaryReader struct{}

func (languageSummaryReader) ReadTrendSummary(_ context.Context, query jdserving.TrendQuery) (jdserving.TrendSummary, error) {
	counts := map[string]float64{"cpp": 30, "python": 20, "go": 10, "unreal-engine": 40}
	average := counts[query.Condition.Value]
	return jdserving.TrendSummary{
		Curve:                  jdserving.TrendCurve{Dimension: jdserving.TrendDimensionTechnology, Value: query.Condition.Value, Label: query.Condition.Value},
		AverageDailyActiveJobs: &average,
		ValidObservationDays:   1,
	}, nil
}

type databaseCategorySummaryReader struct{}

func (databaseCategorySummaryReader) ReadTrendSummary(_ context.Context, query jdserving.TrendQuery) (jdserving.TrendSummary, error) {
	values := map[string]float64{
		"alluxio": 2, "clickhouse": 1, "milvus": 0.64, "mongodb": 6,
		"mysql": 9.12, "postgresql": 2.64, "redis": 10.12, "starrocks": 1.44,
		"tidb": 8.5, "tikv": 7.5,
	}
	average := values[query.Condition.Value]
	return jdserving.TrendSummary{
		Curve:                  jdserving.TrendCurve{Dimension: jdserving.TrendDimensionTechnology, Value: query.Condition.Value, Label: query.Condition.Value},
		AverageDailyActiveJobs: &average,
		ValidObservationDays:   1,
	}, nil
}

func TestTrendRunnerCompletesCategorySummaryBatchesBeforeRanking(t *testing.T) {
	values := []jdserving.TechnologyKeywordFilterOption{
		{Category: jdserving.FilterOption{Value: "databases-and-storage", Label: "数据库与存储"}, Value: "alluxio", Label: "Alluxio"},
		{Category: jdserving.FilterOption{Value: "databases-and-storage", Label: "数据库与存储"}, Value: "clickhouse", Label: "ClickHouse"},
		{Category: jdserving.FilterOption{Value: "databases-and-storage", Label: "数据库与存储"}, Value: "milvus", Label: "Milvus"},
		{Category: jdserving.FilterOption{Value: "databases-and-storage", Label: "数据库与存储"}, Value: "mongodb", Label: "MongoDB"},
		{Category: jdserving.FilterOption{Value: "databases-and-storage", Label: "数据库与存储"}, Value: "mysql", Label: "MySQL"},
		{Category: jdserving.FilterOption{Value: "databases-and-storage", Label: "数据库与存储"}, Value: "postgresql", Label: "PostgreSQL"},
		{Category: jdserving.FilterOption{Value: "databases-and-storage", Label: "数据库与存储"}, Value: "redis", Label: "Redis"},
		{Category: jdserving.FilterOption{Value: "databases-and-storage", Label: "数据库与存储"}, Value: "starrocks", Label: "StarRocks"},
		{Category: jdserving.FilterOption{Value: "databases-and-storage", Label: "数据库与存储"}, Value: "tidb", Label: "TiDB"},
		{Category: jdserving.FilterOption{Value: "databases-and-storage", Label: "数据库与存储"}, Value: "tikv", Label: "TiKV"},
	}
	options := jdserving.FilterOptionsResponse{TechnologyKeywords: values}
	allValues := []string{"alluxio", "clickhouse", "milvus", "mongodb", "mysql", "postgresql", "redis", "starrocks", "tidb", "tikv"}
	firstBatch, _ := json.Marshal(func() []map[string]any {
		result := make([]map[string]any, 0, 8)
		for _, value := range allValues[:8] {
			result = append(result, map[string]any{"dimension": "technology", "value": value, "range": "month"})
		}
		return result
	}())
	// Simulate the production failure: the model returned all formal values but
	// incorrectly treated the plural question as top-one.
	intent := toolIntentJSON("databases", DimensionTechnology, allValues, 1)
	provider := &scriptedToolProvider{turns: []IntentToolTurn{
		{ToolCalls: []ToolCall{{ID: "summary", Name: ToolBatchTrendSummary, Params: firstBatch}}},
		{Intent: intent},
	}}
	runner, err := NewTrendRunner(provider, runnerOptionsReader{options: options}, databaseCategorySummaryReader{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var draft *Draft
	var summaryCalls int
	if err := runner.Run(context.Background(), "哪些数据库正在被使用？", func(event StreamEvent) error {
		if event.Type == StreamEventDraft {
			draft = event.Draft
		}
		if event.Type == StreamEventOperation && event.Status == StreamRunning && event.Tool == string(ToolBatchTrendSummary) {
			summaryCalls++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if draft == nil || len(draft.Curves) != 8 || summaryCalls != 2 {
		t.Fatalf("draft=%#v summaryCalls=%d", draft, summaryCalls)
	}
	want := []string{"redis", "mysql", "tidb", "tikv", "mongodb", "postgresql", "alluxio", "starrocks"}
	for index, value := range want {
		if draft.Curves[index].Condition.Value != value {
			t.Fatalf("curve[%d]=%q want %q; draft=%#v", index, draft.Curves[index].Condition.Value, value, draft)
		}
	}
}

func TestTrendRunnerUsesModelCategoryDiscoveryAndRankingLimit(t *testing.T) {
	options := jdserving.FilterOptionsResponse{TechnologyKeywords: []jdserving.TechnologyKeywordFilterOption{
		{Category: jdserving.FilterOption{Value: "programming-languages", Label: "编程语言"}, Value: "cpp", Label: "C++"},
		{Category: jdserving.FilterOption{Value: "programming-languages", Label: "编程语言"}, Value: "python", Label: "Python"},
		{Category: jdserving.FilterOption{Value: "programming-languages", Label: "编程语言"}, Value: "go", Label: "Go"},
		{Category: jdserving.FilterOption{Value: "game-engines", Label: "游戏引擎"}, Value: "unreal-engine", Label: "Unreal Engine"},
	}}
	finalIntent := []byte(`{"contract_version":"ai-trend-draft-v1","sub_queries":[{"id":"languages","dimension":"technology","expressions":["cpp","python","go"],"ranking":{"metric":"avg_daily_active_jobs","order":"desc","limit":1}}],"time_range":{"kind":"month"},"output":{"series_metric":"daily_active_jobs","chart_type":"line"}}`)
	provider := &scriptedToolProvider{turns: []IntentToolTurn{
		{ToolCalls: []ToolCall{{ID: "categories", Name: ToolListTechnologyCategories, Params: []byte(`{}`)}}, InputTokens: 2, OutputTokens: 2},
		{ToolCalls: []ToolCall{{ID: "language-values", Name: ToolListCategoryValues, Params: []byte(`{"category":"编程语言"}`)}}, InputTokens: 2, OutputTokens: 2},
		{ToolCalls: []ToolCall{{ID: "language-summary", Name: ToolBatchTrendSummary, Params: []byte(`[{"dimension":"technology","value":"cpp","range":"month"},{"dimension":"technology","value":"python","range":"month"},{"dimension":"technology","value":"go","range":"month"}]`)}}, InputTokens: 2, OutputTokens: 2},
		{Intent: finalIntent, InputTokens: 2, OutputTokens: 2},
	}}
	runner, err := NewTrendRunner(provider, runnerOptionsReader{options: options}, languageSummaryReader{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var draft *Draft
	if err := runner.Run(context.Background(), "使用最多的编程语言是什么", func(event StreamEvent) error {
		if event.Type == StreamEventDraft {
			draft = event.Draft
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if draft == nil || len(draft.Curves) != 1 || draft.Curves[0].Condition.Value != "cpp" {
		t.Fatalf("draft = %#v", draft)
	}
}

type wideGlobalSummaryReader struct{}

func (wideGlobalSummaryReader) ReadTrendSummary(_ context.Context, query jdserving.TrendQuery) (jdserving.TrendSummary, error) {
	var index int
	if len(query.Condition.Value) >= len("tech-") {
		_, _ = fmt.Sscanf(query.Condition.Value, "tech-%d", &index)
	}
	score := float64(index)
	return jdserving.TrendSummary{
		Curve:                  jdserving.TrendCurve{Dimension: jdserving.TrendDimensionTechnology, Value: query.Condition.Value, Label: query.Condition.Value},
		AverageDailyActiveJobs: &score,
		ValidObservationDays:   1,
	}, nil
}

func TestTrendRunnerGlobalRankingIncludesValuesBeyondCandidateBound(t *testing.T) {
	options := jdserving.FilterOptionsResponse{TechnologyKeywords: make([]jdserving.TechnologyKeywordFilterOption, 101)}
	for index := range options.TechnologyKeywords {
		value := fmt.Sprintf("tech-%03d", index)
		options.TechnologyKeywords[index] = jdserving.TechnologyKeywordFilterOption{Category: jdserving.FilterOption{Value: "synthetic", Label: "合成类别"}, Value: value, Label: value}
	}
	intent := Intent{ContractVersion: ContractVersionV1, SubQueries: []SubQuery{{ID: "all-tech", Dimension: DimensionTechnology, Expressions: []string{"*"}, Ranking: RankingIntent{Metric: MetricAverageDailyActiveJobs, Order: OrderDescending, Limit: 1}}}, TimeRange: TimeRange{Kind: RangeAll}, Output: OutputIntent{SeriesMetric: SeriesMetricDailyActiveJobs, ChartType: ChartTypeLine}}
	runner, err := NewTrendRunner(runnerIntentProvider{intent: intent}, runnerOptionsReader{options: options}, wideGlobalSummaryReader{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var draft *Draft
	var events []StreamEvent
	if err := runner.Run(context.Background(), "使用最多的技术关键词是什么？", func(event StreamEvent) error {
		events = append(events, event)
		if event.Type == StreamEventDraft {
			draft = event.Draft
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if draft == nil || len(draft.Curves) != 1 || draft.Curves[0].Condition.Value != "tech-100" {
		t.Fatalf("global ranking draft = %#v, events=%#v, want tech-100", draft, events)
	}
}

type runnerIntentProvider struct {
	intent Intent
}

func (provider runnerIntentProvider) AnalyzeIntent(context.Context, string, *ContractError) ([]byte, error) {
	return json.Marshal(provider.intent)
}

type runnerClassifierProvider struct {
	tags              []ServiceTag
	classificationErr error
	classifyCalls     int
	analyzeCalls      int
	turns             []IntentToolTurn
	turnCalls         int
}

func (provider *runnerClassifierProvider) AnalyzeIntent(context.Context, string, *ContractError) ([]byte, error) {
	provider.analyzeCalls++
	return []byte(`{"contract_version":"ai-trend-agent-v1","response_modes":["trend"]}`), nil
}

func (provider *runnerClassifierProvider) ClassifyServiceModes(context.Context, string) (ServiceModeClassification, int, int, error) {
	provider.classifyCalls++
	if provider.classificationErr != nil {
		return ServiceModeClassification{}, 7, 3, provider.classificationErr
	}
	return ServiceModeClassification{ContractVersion: ServiceModeContractVersionV1, ServiceTags: append([]ServiceTag(nil), provider.tags...)}, 7, 3, nil
}

func (provider *runnerClassifierProvider) AnalyzeIntentTurn(_ context.Context, _ string, _ *ContractError, _ []ToolConversation, _ bool) (IntentToolTurn, error) {
	provider.turnCalls++
	if len(provider.turns) == 0 {
		return IntentToolTurn{}, fmt.Errorf("classifier provider exhausted")
	}
	turn := provider.turns[0]
	provider.turns = provider.turns[1:]
	return turn, nil
}

func TestTrendRunnerClassificationFailuresFallbackToGuidanceWithoutTools(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "timeout", err: context.DeadlineExceeded},
		{name: "provider", err: &ContractError{Code: ErrorProviderFailure, Message: "provider unavailable"}},
		{name: "invalid json", err: &ContractError{Code: ErrorInvalidJSON, Message: "invalid classification"}},
		{name: "unknown tag", err: &ContractError{Code: ErrorInvalidEnum, Field: "service_tags", Message: "unknown tag"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := &runnerClassifierProvider{classificationErr: test.err}
			runner, err := NewTrendRunner(provider, runnerOptionsReader{}, runnerSummaryReader{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			var events []StreamEvent
			var stats TrendRunStats
			runner.StatsSink = func(value TrendRunStats) { stats = value }
			if err := runner.Run(context.Background(), "你好", func(event StreamEvent) error {
				events = append(events, event)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if provider.classifyCalls != 1 || provider.analyzeCalls != 0 {
				t.Fatalf("provider calls = classify:%d analyze:%d", provider.classifyCalls, provider.analyzeCalls)
			}
			for _, event := range events {
				if event.Type == StreamEventOperation && event.Tool != string(ToolClassifyServiceModes) {
					t.Fatalf("classification fallback entered downstream operation: %#v", events)
				}
				if event.Type == StreamEventDraft {
					t.Fatalf("classification fallback emitted draft: %#v", events)
				}
			}
			if stats.ClassificationStatus != "failed" || len(stats.ServiceTags) != 1 || stats.ServiceTags[0] != ServiceTagGuidance || stats.ClassificationInput != 7 || stats.ClassificationOutput != 3 {
				t.Fatalf("classification stats = %#v", stats)
			}
			if len(events) == 0 || events[len(events)-1].Type != StreamEventDone || events[len(events)-1].Status != StreamSuccess {
				t.Fatalf("classification fallback events = %#v", events)
			}
		})
	}
}

func TestTrendRunnerGuidanceClassificationSkipsTrendFlow(t *testing.T) {
	provider := &runnerClassifierProvider{tags: []ServiceTag{ServiceTagGuidance}}
	runner, err := NewTrendRunner(provider, runnerOptionsReader{}, runnerSummaryReader{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var events []StreamEvent
	if err := runner.Run(context.Background(), "怎么使用趋势看板？", func(event StreamEvent) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if provider.classifyCalls != 1 || provider.analyzeCalls != 0 {
		t.Fatalf("provider calls = classify:%d analyze:%d", provider.classifyCalls, provider.analyzeCalls)
	}
	for _, event := range events {
		if event.Type == StreamEventOperation && event.Tool != string(ToolClassifyServiceModes) && event.Tool != string(ToolExplainDraft) {
			t.Fatalf("guidance classification entered trend flow: %#v", events)
		}
	}
}

func TestTrendRunnerClassifiesBeforeServingOptions(t *testing.T) {
	provider := &runnerClassifierProvider{tags: []ServiceTag{ServiceTagGuidance}}
	runner, err := NewTrendRunner(provider, failingRunnerOptionsReader{}, runnerSummaryReader{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var events []StreamEvent
	if err := runner.Run(context.Background(), "你好", func(event StreamEvent) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if provider.classifyCalls != 1 || len(events) < 2 || events[0].Tool != string(ToolClassifyServiceModes) || events[0].Status != StreamRunning {
		t.Fatalf("classification did not precede options lookup: calls=%d events=%#v", provider.classifyCalls, events)
	}
}

func TestTrendRunnerRunsClassificationOnceBeforeTrendTools(t *testing.T) {
	intent := Intent{ContractVersion: ContractVersionV1, SubQueries: []SubQuery{{ID: "projects", Dimension: DimensionTechnology, Expressions: []string{"redis"}, Ranking: RankingIntent{Metric: MetricAverageDailyActiveJobs, Order: OrderDescending, Limit: 1}}}, TimeRange: TimeRange{Kind: RangeMonth}, Output: OutputIntent{SeriesMetric: SeriesMetricDailyActiveJobs, ChartType: ChartTypeLine}}
	intentJSON, err := json.Marshal(intent)
	if err != nil {
		t.Fatal(err)
	}
	provider := &runnerClassifierProvider{
		tags: []ServiceTag{ServiceTagTrend},
		turns: []IntentToolTurn{
			{ToolCalls: []ToolCall{{ID: "summary", Name: ToolBatchTrendSummary, Params: []byte(`[{"dimension":"technology","value":"redis","range":"month"}]`)}}, InputTokens: 2, OutputTokens: 1},
			{Intent: intentJSON, InputTokens: 2, OutputTokens: 1},
		},
	}
	runner, err := NewTrendRunner(provider, runnerOptionsReader{}, runnerSummaryReader{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var stats TrendRunStats
	runner.StatsSink = func(value TrendRunStats) { stats = value }
	var events []StreamEvent
	if err := runner.Run(context.Background(), "哪些技术的岗位最多？", func(event StreamEvent) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if provider.classifyCalls != 1 || provider.turnCalls != 2 {
		t.Fatalf("provider calls = classify:%d turns:%d", provider.classifyCalls, provider.turnCalls)
	}
	if stats.ToolCalls != 1 || stats.ClassificationStatus != "completed" || stats.ClassificationInput != 7 || stats.ClassificationOutput != 3 || len(stats.ServiceTags) != 1 || stats.ServiceTags[0] != ServiceTagTrend {
		t.Fatalf("run stats = %#v", stats)
	}
	if len(events) < 2 || events[0].Type != StreamEventOperation || events[0].Tool != string(ToolClassifyServiceModes) || events[0].Status != StreamRunning || events[1].Tool != string(ToolClassifyServiceModes) || events[1].Status != StreamCompleted {
		t.Fatalf("classification events = %#v", events)
	}
}

type runnerExplanationProvider struct {
	intent  Intent
	context DraftExplanationContext
}

type runnerStreamingProvider struct {
	intent Intent
}

type runnerStoppingProvider struct {
	intent Intent
}

type runnerLongReplyProvider struct{ intent Intent }

func (provider runnerLongReplyProvider) AnalyzeIntent(context.Context, string, *ContractError) ([]byte, error) {
	return json.Marshal(provider.intent)
}

func (runnerLongReplyProvider) StreamAssistantMessage(_ context.Context, _ AssistantReplyContext, emit func(string) error) (int, int, error) {
	return 0, 0, emit(strings.Repeat("中", MaxReplyRunes+1))
}

func (provider runnerStoppingProvider) AnalyzeIntent(context.Context, string, *ContractError) ([]byte, error) {
	return json.Marshal(provider.intent)
}

func (runnerStoppingProvider) StreamAssistantMessage(_ context.Context, _ AssistantReplyContext, emit func(string) error) (int, int, error) {
	if err := emit("已开始说明"); err != nil {
		return 0, 0, err
	}
	return 0, 0, context.Canceled
}

func (provider runnerStreamingProvider) AnalyzeIntent(context.Context, string, *ContractError) ([]byte, error) {
	return json.Marshal(provider.intent)
}

func (runnerStreamingProvider) StreamAssistantMessage(_ context.Context, _ AssistantReplyContext, emit func(string) error) (int, int, error) {
	if err := emit("第一段说明"); err != nil {
		return 3, 5, err
	}
	if err := emit("第二段指导"); err != nil {
		return 3, 5, err
	}
	return 3, 5, nil
}

func TestTrendRunnerEmitsDraftBeforeStreamingAssistantMessage(t *testing.T) {
	options := jdserving.FilterOptionsResponse{Projects: []jdserving.FilterOption{{Value: "game.genshin-impact", Label: "原神"}}}
	intent := Intent{ContractVersion: ContractVersionV1, SubQueries: []SubQuery{{ID: "projects", Dimension: DimensionProject, Expressions: []string{"*"}, Ranking: RankingIntent{Metric: MetricAverageDailyActiveJobs, Order: OrderDescending, Limit: 1}}}, TimeRange: TimeRange{Kind: RangeMonth}, Output: OutputIntent{SeriesMetric: SeriesMetricDailyActiveJobs, ChartType: ChartTypeLine}}
	runner, err := NewTrendRunner(runnerStreamingProvider{intent: intent}, runnerOptionsReader{options: options}, runnerProjectSummaryReader{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var events []StreamEvent
	if err := runner.Run(context.Background(), "哪些项目岗位最多？", func(event StreamEvent) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	draftIndex, assistantIndex, doneIndex := -1, -1, -1
	for index, event := range events {
		if event.Type == StreamEventDraft && draftIndex < 0 {
			draftIndex = index
		}
		if event.Type == StreamEventAssistant && assistantIndex < 0 {
			assistantIndex = index
		}
		if event.Type == StreamEventDone {
			doneIndex = index
		}
	}
	if draftIndex < 0 || assistantIndex <= draftIndex || doneIndex <= assistantIndex {
		t.Fatalf("event order = %#v", events)
	}
}

func TestTrendRunnerPreservesDraftWhenAssistantStreamStops(t *testing.T) {
	options := jdserving.FilterOptionsResponse{Projects: []jdserving.FilterOption{{Value: "game.genshin-impact", Label: "原神"}}}
	intent := Intent{ContractVersion: ContractVersionV1, SubQueries: []SubQuery{{ID: "projects", Dimension: DimensionProject, Expressions: []string{"*"}, Ranking: RankingIntent{Metric: MetricAverageDailyActiveJobs, Order: OrderDescending, Limit: 1}}}, TimeRange: TimeRange{Kind: RangeMonth}, Output: OutputIntent{SeriesMetric: SeriesMetricDailyActiveJobs, ChartType: ChartTypeLine}}
	runner, err := NewTrendRunner(runnerStoppingProvider{intent: intent}, runnerOptionsReader{options: options}, runnerProjectSummaryReader{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var events []StreamEvent
	if err := runner.Run(context.Background(), "哪些项目岗位最多？", func(event StreamEvent) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var stoppedDraft bool
	var terminal StreamStatus
	for _, event := range events {
		if event.Type == StreamEventDraft && event.Status == StreamStopped && event.Draft != nil && len(event.Draft.Curves) > 0 {
			stoppedDraft = true
		}
		if event.Type == StreamEventDone {
			terminal = event.Status
		}
	}
	if !stoppedDraft || terminal != StreamStopped {
		t.Fatalf("stop events = %#v", events)
	}
}

func TestTrendRunnerRecordsReplyQualityWarningWithoutRollingBackDraft(t *testing.T) {
	options := jdserving.FilterOptionsResponse{Projects: []jdserving.FilterOption{{Value: "game.genshin-impact", Label: "原神"}}}
	intent := Intent{ContractVersion: ContractVersionV1, SubQueries: []SubQuery{{ID: "projects", Dimension: DimensionProject, Expressions: []string{"*"}, Ranking: RankingIntent{Metric: MetricAverageDailyActiveJobs, Order: OrderDescending, Limit: 1}}}, TimeRange: TimeRange{Kind: RangeMonth}, Output: OutputIntent{SeriesMetric: SeriesMetricDailyActiveJobs, ChartType: ChartTypeLine}}
	runner, err := NewTrendRunner(runnerLongReplyProvider{intent: intent}, runnerOptionsReader{options: options}, runnerProjectSummaryReader{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var draft *Draft
	var stats TrendRunStats
	runner.StatsSink = func(value TrendRunStats) { stats = value }
	if err := runner.Run(context.Background(), "哪些项目岗位最多？", func(event StreamEvent) error {
		if event.Type == StreamEventDraft && event.Draft != nil {
			draft = event.Draft
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if draft == nil || draft.Status != DraftComplete || len(stats.ReplyQualityWarnings) != 1 || stats.ReplyQualityWarnings[0] != "too_long" {
		t.Fatalf("draft=%#v stats=%#v", draft, stats)
	}
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

func TestTrendRunnerLimitsProgrammingLanguageCategory(t *testing.T) {
	options := jdserving.FilterOptionsResponse{TechnologyKeywords: []jdserving.TechnologyKeywordFilterOption{
		{Category: jdserving.FilterOption{Value: "programming-languages", Label: "编程语言"}, Value: "cpp", Label: "C++"},
		{Category: jdserving.FilterOption{Value: "programming-languages", Label: "编程语言"}, Value: "python", Label: "Python"},
		{Category: jdserving.FilterOption{Value: "game-engines", Label: "游戏引擎"}, Value: "unreal-engine", Label: "Unreal Engine"},
	}}
	intent := Intent{
		ContractVersion: ContractVersionV1,
		SubQueries: []SubQuery{{
			ID: "languages", Dimension: DimensionTechnology, Expressions: []string{"*"},
			Ranking: RankingIntent{Metric: MetricAverageDailyActiveJobs, Order: OrderDescending, Limit: MaxCurves},
		}},
		TimeRange: TimeRange{Kind: RangeAll},
		Output:    OutputIntent{SeriesMetric: SeriesMetricDailyActiveJobs, ChartType: ChartTypeLine},
	}
	runner, err := NewTrendRunner(runnerIntentProvider{intent: intent}, runnerOptionsReader{options: options}, languageSummaryReader{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var draft *Draft
	if err := runner.Run(context.Background(), "使用最多的编程语言是什么", func(event StreamEvent) error {
		if event.Type == StreamEventDraft {
			draft = event.Draft
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if draft == nil || len(draft.Curves) != 2 {
		t.Fatalf("draft = %#v, want two programming-language curves", draft)
	}
	for _, curve := range draft.Curves {
		if curve.Condition.Value != "cpp" && curve.Condition.Value != "python" {
			t.Fatalf("non-language curve returned: %#v", curve)
		}
	}
}

func TestTrendRunnerResolvesUnlistedCategoryFromServingCatalog(t *testing.T) {
	options := jdserving.FilterOptionsResponse{TechnologyKeywords: []jdserving.TechnologyKeywordFilterOption{
		{Category: jdserving.FilterOption{Value: "container-orchestration", Label: "容器编排"}, Value: "kubernetes", Label: "Kubernetes"},
		{Category: jdserving.FilterOption{Value: "container-orchestration", Label: "容器编排"}, Value: "docker-swarm", Label: "Docker Swarm"},
		{Category: jdserving.FilterOption{Value: "databases-and-storage", Label: "数据库与存储"}, Value: "mysql", Label: "MySQL"},
	}}
	intent := Intent{
		ContractVersion: ContractVersionV1,
		SubQueries: []SubQuery{{
			ID: "containers", Dimension: DimensionTechnology, Expressions: []string{"*"},
			Ranking: RankingIntent{Metric: MetricAverageDailyActiveJobs, Order: OrderDescending, Limit: MaxCurves},
		}},
		TimeRange: TimeRange{Kind: RangeMonth},
		Output:    OutputIntent{SeriesMetric: SeriesMetricDailyActiveJobs, ChartType: ChartTypeLine},
	}
	runner, err := NewTrendRunner(runnerIntentProvider{intent: intent}, runnerOptionsReader{options: options}, runnerSummaryReader{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var draft *Draft
	if err := runner.Run(context.Background(), "哪些容器编排技术的岗位最多？", func(event StreamEvent) error {
		if event.Type == StreamEventDraft {
			draft = event.Draft
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if draft == nil || len(draft.Curves) != 2 {
		t.Fatalf("draft = %#v", draft)
	}
	for _, curve := range draft.Curves {
		if curve.Condition.Value == "mysql" {
			t.Fatalf("adjacent category leaked into draft: %#v", draft.Curves)
		}
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

func TestTrendRunnerRejectsConflictingTimeRanges(t *testing.T) {
	intent := Intent{ContractVersion: ContractVersionV1, SubQueries: []SubQuery{{ID: "projects", Dimension: DimensionProject, Expressions: []string{"*"}, Ranking: RankingIntent{Metric: MetricAverageDailyActiveJobs, Order: OrderDescending, Limit: 1}}}, TimeRange: TimeRange{Kind: RangeMonth}, Output: OutputIntent{SeriesMetric: SeriesMetricDailyActiveJobs, ChartType: ChartTypeLine}}
	runner, err := NewTrendRunner(runnerIntentProvider{intent: intent}, runnerOptionsReader{options: jdserving.FilterOptionsResponse{Projects: []jdserving.FilterOption{{Value: "alpha", Label: "Alpha"}}}}, runnerProjectSummaryReader{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = runner.Run(context.Background(), "本月和去年哪个项目岗位更多？", func(StreamEvent) error { return nil })
	contractErr, ok := err.(*ContractError)
	if !ok || contractErr.Code != ErrorInvalidCombination || contractErr.Field != "time_range" {
		t.Fatalf("conflicting range error = %#v", err)
	}
}

func TestTrendRunnerMarksUnsupportedMetricAndChartFallbacksPartial(t *testing.T) {
	options := jdserving.FilterOptionsResponse{Projects: []jdserving.FilterOption{{Value: "alpha", Label: "Alpha"}}}
	intent := Intent{ContractVersion: ContractVersionV1, SubQueries: []SubQuery{{ID: "projects", Dimension: DimensionProject, Expressions: []string{"*"}, Ranking: RankingIntent{Metric: MetricAverageDailyActiveJobs, Order: OrderDescending, Limit: 1}}}, TimeRange: TimeRange{Kind: RangeMonth}, Output: OutputIntent{SeriesMetric: SeriesMetricDailyActiveJobs, ChartType: ChartTypeLine}}
	for _, query := range []string{"哪些项目的岗位占比最高？", "用柱状图展示岗位最多的项目"} {
		runner, err := NewTrendRunner(runnerIntentProvider{intent: intent}, runnerOptionsReader{options: options}, runnerProjectSummaryReader{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		var draft *Draft
		if err := runner.Run(context.Background(), query, func(event StreamEvent) error {
			if event.Type == StreamEventDraft {
				draft = event.Draft
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if draft == nil || draft.Status != DraftPartial || len(draft.Warnings) == 0 {
			t.Fatalf("fallback query %q draft = %#v", query, draft)
		}
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
		prompt := ""
		if ok && len(messages) > 0 {
			prompt = messages[0].(map[string]any)["content"].(string)
		}
		if !ok || len(messages) == 0 || !strings.Contains(prompt, "将用户趋势问题转换") || !strings.Contains(prompt, "编程语言") || !strings.Contains(prompt, "[\"*\"]") {
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
		if body["max_tokens"] != float64(200) {
			t.Fatalf("max_tokens = %#v", body["max_tokens"])
		}
		messages, ok := body["messages"].([]any)
		prompt := ""
		if ok && len(messages) > 0 {
			prompt = messages[0].(map[string]any)["content"].(string)
		}
		if !ok || len(messages) == 0 || !strings.Contains(prompt, "简体中文") || !strings.Contains(prompt, "80") {
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
