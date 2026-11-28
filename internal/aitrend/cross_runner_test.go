package aitrend

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"mrolescope/internal/jdserving"
)

type crossRunnerOptions struct {
	options jdserving.FilterOptionsResponse
	reader  *fakeCrossBatchReader
}

func (options crossRunnerOptions) ListFilterOptions(context.Context) (jdserving.FilterOptionsResponse, error) {
	return options.options, nil
}

func (options crossRunnerOptions) ReadCrossStatisticsBatch(ctx context.Context, queries []jdserving.CrossStatisticsQuery) ([]jdserving.CrossStatisticsResult, error) {
	return options.reader.ReadCrossStatisticsBatch(ctx, queries)
}

type crossRunnerSummary struct{}

func (crossRunnerSummary) ReadTrendSummary(_ context.Context, query jdserving.TrendQuery) (jdserving.TrendSummary, error) {
	count := 3
	return jdserving.TrendSummary{Curve: jdserving.TrendCurve{Dimension: query.Condition.Dimension, Value: query.Condition.Value, Label: query.Condition.Value}, PeriodEndActiveJobs: &count, ValidObservationDays: 1}, nil
}

func crossRunnerIntent(dimension Dimension, value string) json.RawMessage {
	intent := Intent{ContractVersion: ContractVersionV1, SubQueries: []SubQuery{{ID: "chart", Dimension: dimension, Expressions: []string{value}, Ranking: RankingIntent{Metric: MetricPeriodEndActiveJobs, Order: OrderDescending, Limit: 1}}}, TimeRange: TimeRange{Kind: RangeMonth}, Output: OutputIntent{SeriesMetric: SeriesMetricDailyActiveJobs, ChartType: ChartTypeLine}}
	encoded, _ := json.Marshal(intent)
	return encoded
}

func TestCrossRunnerDeliversVerifiedStatisticsWithIndependentChart(t *testing.T) {
	snapshot := crossTestSnapshot()
	reader := &fakeCrossBatchReader{results: []jdserving.CrossStatisticsResult{{Total: 12}}}
	options := crossRunnerOptions{options: snapshot.options, reader: reader}
	statistics := ToolCall{ID: "cross", Name: ToolCrossStatistics, Params: json.RawMessage(`{"sub_queries":[{"id":"q","time":"current","conditions":[{"dimension":"location","operator":"any","values":["上海"]},{"dimension":"technology","operator":"any","values":["java"]}]}]}`)}
	summary := ToolCall{ID: "summary", Name: ToolBatchTrendSummary, Params: json.RawMessage(`[{"dimension":"technology","value":"java","range":"month"}]`)}
	for _, statsFirst := range []bool{true, false} {
		turns := []IntentToolTurn{{ToolCalls: []ToolCall{statistics}}, {ToolCalls: []ToolCall{summary}}, {Intent: crossRunnerIntent(DimensionTechnology, "java")}}
		if !statsFirst {
			turns[0], turns[1] = turns[1], turns[0]
		}
		provider := &scriptedToolProvider{turns: turns}
		runner, err := NewTrendRunner(provider, options, crossRunnerSummary{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		var events []StreamEvent
		if err := runner.Run(context.Background(), "上海 Java 岗位有多少", func(event StreamEvent) error { events = append(events, event); return nil }); err != nil {
			t.Fatal(err)
		}
		var reply string
		var draft *Draft
		for _, event := range events {
			if event.Type == StreamEventAssistant {
				reply += event.Delta
			}
			if event.Type == StreamEventDraft {
				draft = event.Draft
			}
		}
		if !strings.Contains(reply, "12 个岗位") || !strings.Contains(reply, "独立趋势曲线") || !strings.Contains(reply, "不表示交集或并集") {
			t.Fatalf("unsafe or incomplete cross reply: %q", reply)
		}
		if draft == nil || len(draft.Curves) != 2 || draft.Curves[0].Condition.Value != "java" || draft.Curves[1].Condition.Value != "上海" {
			t.Fatalf("independent Java and Shanghai charts missing: %#v", draft)
		}
	}
}

func TestCrossRunnerBlocksInvalidDraftButKeepsStatistics(t *testing.T) {
	snapshot := crossTestSnapshot()
	reader := &fakeCrossBatchReader{results: []jdserving.CrossStatisticsResult{{Total: 12}}}
	provider := &scriptedToolProvider{turns: []IntentToolTurn{
		{ToolCalls: []ToolCall{{ID: "cross", Name: ToolCrossStatistics, Params: json.RawMessage(`{"sub_queries":[{"id":"q","time":"current","conditions":[{"dimension":"location","operator":"any","values":["上海"]},{"dimension":"technology","operator":"any","values":["java"]}]}]}`)}}},
		{ToolCalls: []ToolCall{{ID: "summary", Name: ToolBatchTrendSummary, Params: json.RawMessage(`[{"dimension":"job_family","value":"backend","range":"month"}]`)}}},
		{Intent: crossRunnerIntent(DimensionJobFamily, "backend")},
	}}
	runner, err := NewTrendRunner(provider, crossRunnerOptions{options: snapshot.options, reader: reader}, crossRunnerSummary{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var events []StreamEvent
	if err := runner.Run(context.Background(), "上海 Java 岗位有多少", func(event StreamEvent) error { events = append(events, event); return nil }); err != nil {
		t.Fatal(err)
	}
	var reply string
	validationFailed := false
	for _, event := range events {
		if event.Type == StreamEventDraft {
			t.Fatalf("out-of-scope draft was sent: %#v", event.Draft)
		}
		if event.Type == StreamEventAssistant {
			reply += event.Delta
		}
		if event.Type == StreamEventOperation && event.Tool == string(ToolBatchValidateDraft) && event.Status == StreamFailed {
			validationFailed = true
		}
	}
	if !validationFailed || !strings.Contains(reply, "12 个岗位") || events[len(events)-1].Status != StreamPartial {
		t.Fatalf("validated statistics were lost with the draft: reply=%q events=%#v", reply, events)
	}
}

func TestCrossRunnerKeepsChartWhenStatisticsFail(t *testing.T) {
	snapshot := crossTestSnapshot()
	reader := &fakeCrossBatchReader{err: errors.New("synthetic database failure")}
	provider := &scriptedToolProvider{turns: []IntentToolTurn{
		{ToolCalls: []ToolCall{{ID: "cross", Name: ToolCrossStatistics, Params: json.RawMessage(`{"sub_queries":[{"id":"q","time":"current","conditions":[{"dimension":"location","operator":"any","values":["上海"]},{"dimension":"technology","operator":"any","values":["java"]}]}]}`)}}},
		{ToolCalls: []ToolCall{{ID: "summary", Name: ToolBatchTrendSummary, Params: json.RawMessage(`[{"dimension":"technology","value":"java","range":"month"}]`)}}},
		{Intent: crossRunnerIntent(DimensionTechnology, "java")},
	}}
	runner, err := NewTrendRunner(provider, crossRunnerOptions{options: snapshot.options, reader: reader}, crossRunnerSummary{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var events []StreamEvent
	if err := runner.Run(context.Background(), "上海 Java 岗位有多少", func(event StreamEvent) error { events = append(events, event); return nil }); err != nil {
		t.Fatal(err)
	}
	var reply string
	chart := false
	for _, event := range events {
		if event.Type == StreamEventDraft && len(event.Draft.Curves) == 2 {
			chart = true
		}
		if event.Type == StreamEventAssistant {
			reply += event.Delta
		}
	}
	if !chart || !strings.Contains(reply, "无法计算") || strings.Contains(reply, "0 个岗位") {
		t.Fatalf("statistics failure lost chart or became zero: chart=%v reply=%q", chart, reply)
	}
}

func TestCrossRunnerExplainsUnsupportedCrossDimensionUnion(t *testing.T) {
	snapshot := crossTestSnapshot()
	reader := &fakeCrossBatchReader{}
	provider := &scriptedToolProvider{turns: []IntentToolTurn{
		{ToolCalls: []ToolCall{{ID: "cross", Name: ToolCrossStatistics, Params: json.RawMessage(`{"sub_queries":[{"id":"q","time":"current","conditions":[{"dimension":"location","operator":"any","values":["上海"]},{"dimension":"technology","operator":"any","values":["java"]}]}]}`)}}},
		{ToolCalls: []ToolCall{{ID: "summary", Name: ToolBatchTrendSummary, Params: json.RawMessage(`[{"dimension":"technology","value":"java","range":"month"}]`)}}},
		{Intent: crossRunnerIntent(DimensionTechnology, "java")},
	}}
	runner, err := NewTrendRunner(provider, crossRunnerOptions{options: snapshot.options, reader: reader}, crossRunnerSummary{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var reply string
	if err := runner.Run(context.Background(), "上海岗位或 Java 岗位有多少", func(event StreamEvent) error { reply += event.Delta; return nil }); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reply, "跨维度并集统计本期暂不支持") || len(reader.queries) != 0 || strings.Contains(reply, "0 个岗位") {
		t.Fatalf("cross-dimension union was misrepresented: reply=%q queries=%#v", reply, reader.queries)
	}
}

func TestCrossRunnerDoesNotSkipStatisticsAfterGuidanceClassification(t *testing.T) {
	snapshot := crossTestSnapshot()
	reader := &fakeCrossBatchReader{results: []jdserving.CrossStatisticsResult{{Total: 12}}}
	provider := &runnerClassifierProvider{tags: []ServiceTag{ServiceTagGuidance}, turns: []IntentToolTurn{
		{ToolCalls: []ToolCall{{ID: "cross", Name: ToolCrossStatistics, Params: json.RawMessage(`{"sub_queries":[{"id":"q","time":"current","conditions":[{"dimension":"location","operator":"any","values":["上海"]},{"dimension":"technology","operator":"any","values":["java"]}]}]}`)}}},
		{ToolCalls: []ToolCall{{ID: "summary", Name: ToolBatchTrendSummary, Params: json.RawMessage(`[{"dimension":"technology","value":"java","range":"month"}]`)}}},
		{Intent: crossRunnerIntent(DimensionTechnology, "java")},
	}}
	runner, err := NewTrendRunner(provider, crossRunnerOptions{options: snapshot.options, reader: reader}, crossRunnerSummary{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var reply string
	if err := runner.Run(context.Background(), "上海 Java 岗位有多少", func(event StreamEvent) error { reply += event.Delta; return nil }); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reply, "12 个岗位") || len(reader.queries) != 1 {
		t.Fatalf("cross statistics were skipped after classification: reply=%q queries=%#v", reply, reader.queries)
	}
}

func TestCrossRunnerLabelsHistoricalReference(t *testing.T) {
	snapshot := crossTestSnapshot()
	reader := &fakeCrossBatchReader{results: []jdserving.CrossStatisticsResult{{Total: 12}}}
	provider := &scriptedToolProvider{turns: []IntentToolTurn{
		{ToolCalls: []ToolCall{{ID: "cross", Name: ToolCrossStatistics, Params: json.RawMessage(`{"sub_queries":[{"id":"q","time":"historical_reference","conditions":[{"dimension":"location","operator":"any","values":["上海"]},{"dimension":"technology","operator":"any","values":["java"]}]}]}`)}}},
		{ToolCalls: []ToolCall{{ID: "summary", Name: ToolBatchTrendSummary, Params: json.RawMessage(`[{"dimension":"technology","value":"java","range":"month"}]`)}}},
		{Intent: crossRunnerIntent(DimensionTechnology, "java")},
	}}
	runner, err := NewTrendRunner(provider, crossRunnerOptions{options: snapshot.options, reader: reader}, crossRunnerSummary{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var reply string
	if err := runner.Run(context.Background(), "上个月上海 Java 岗位有多少", func(event StreamEvent) error { reply += event.Delta; return nil }); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reply, "历史交叉数量暂不可用") || !strings.Contains(reply, "当前有效岗位的参考统计") || !strings.Contains(reply, "12 个岗位") || !strings.Contains(reply, "独立趋势曲线") {
		t.Fatalf("historical and current scopes were confused: %q", reply)
	}
}

func TestCrossRunnerHistoricalPrimaryFamilyReference(t *testing.T) {
	snapshot := crossTestSnapshot()
	reader := &fakeCrossBatchReader{results: []jdserving.CrossStatisticsResult{{Total: 5}}}
	provider := &scriptedToolProvider{turns: []IntentToolTurn{
		{ToolCalls: []ToolCall{{ID: "cross", Name: ToolCrossStatistics, Params: json.RawMessage(`{"sub_queries":[{"id":"q","time":"historical_reference","include_secondary":false,"conditions":[{"dimension":"technology","operator":"any","categories":["game-engines"]},{"dimension":"job_family","operator":"any","values":["backend"]}]}]}`)}}},
		{ToolCalls: []ToolCall{{ID: "summary", Name: ToolBatchTrendSummary, Params: json.RawMessage(`[{"dimension":"technology","value":"unity","range":"month"}]`)}}},
		{Intent: crossRunnerIntent(DimensionTechnology, "unity")},
	}}
	runner, err := NewTrendRunner(provider, crossRunnerOptions{options: snapshot.options, reader: reader}, crossRunnerSummary{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var reply string
	if err := runner.Run(context.Background(), "上个月游戏引擎只看主岗位族的后端岗位有多少", func(event StreamEvent) error { reply += event.Delta; return nil }); err != nil {
		t.Fatal(err)
	}
	if len(reader.queries) != 1 || !reader.queries[0].PrimaryFamilyOnly || !strings.Contains(reply, "历史交叉数量暂不可用") || !strings.Contains(reply, "仅主岗位族") || !strings.Contains(reply, "岗位族曲线使用曲线自身") {
		t.Fatalf("historical primary-family or chart scope was lost: queries=%#v reply=%q", reader.queries, reply)
	}
}

func TestCrossRunnerAmbiguousAndReturnsAllThreeInterpretations(t *testing.T) {
	snapshot := crossTestSnapshot()
	reader := &fakeCrossBatchReader{results: []jdserving.CrossStatisticsResult{{Total: 2}, {Total: 2}, {Total: 3}, {Total: 1}}}
	provider := &scriptedToolProvider{turns: []IntentToolTurn{
		{ToolCalls: []ToolCall{{ID: "cross", Name: ToolCrossStatistics, Params: json.RawMessage(`{"sub_queries":[
            {"id":"unity","time":"current","conditions":[{"dimension":"technology","operator":"any","values":["unity"]},{"dimension":"job_family","operator":"any","values":["backend"]}]},
            {"id":"unreal","time":"current","conditions":[{"dimension":"technology","operator":"any","values":["unreal"]},{"dimension":"job_family","operator":"any","values":["backend"]}]},
            {"id":"union","time":"current","conditions":[{"dimension":"technology","operator":"any","values":["unity","unreal"]},{"dimension":"job_family","operator":"any","values":["backend"]}]},
            {"id":"intersection","time":"current","conditions":[{"dimension":"technology","operator":"all","values":["unity","unreal"]},{"dimension":"job_family","operator":"any","values":["backend"]}]}
        ]}`)}}},
		{ToolCalls: []ToolCall{{ID: "summary", Name: ToolBatchTrendSummary, Params: json.RawMessage(`[{"dimension":"technology","value":"unity","range":"month"}]`)}}},
		{Intent: crossRunnerIntent(DimensionTechnology, "unity")},
	}}
	runner, err := NewTrendRunner(provider, crossRunnerOptions{options: snapshot.options, reader: reader}, crossRunnerSummary{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var reply string
	var draft *Draft
	if err := runner.Run(context.Background(), "Unity 和 Unreal 的后端岗位有多少", func(event StreamEvent) error {
		reply += event.Delta
		if event.Type == StreamEventDraft {
			draft = event.Draft
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"技术关键词（Unity）且岗位族（后端）：2", "技术关键词（Unreal）且岗位族（后端）：2", "技术关键词（Unity或Unreal）且岗位族（后端）：3", "技术关键词（Unity且Unreal）且岗位族（后端）：1"} {
		if !strings.Contains(reply, expected) {
			t.Fatalf("ambiguous conjunction omitted %q: %q", expected, reply)
		}
	}
	if len(reader.queries) != 4 || draft == nil || len(draft.Curves) != 3 {
		t.Fatalf("ambiguous statistics or independent curves missing: queries=%d draft=%#v", len(reader.queries), draft)
	}
}

func TestCrossRunnerRedactsRejectedModelArgumentsFromEventsAndHistory(t *testing.T) {
	snapshot := crossTestSnapshot()
	reader := &fakeCrossBatchReader{results: []jdserving.CrossStatisticsResult{{Total: 12}}}
	provider := &scriptedToolProvider{turns: []IntentToolTurn{
		{ToolCalls: []ToolCall{{ID: "bad", Name: ToolCrossStatistics, Params: json.RawMessage(`{"sub_queries":[{"id":"q","time":"current","sql":"SELECT secret","conditions":[{"dimension":"technology","operator":"any","values":["java"]}]}]}`)}}},
		{ToolCalls: []ToolCall{{ID: "good", Name: ToolCrossStatistics, Params: json.RawMessage(`{"sub_queries":[{"id":"q","time":"current","conditions":[{"dimension":"location","operator":"any","values":["上海"]},{"dimension":"technology","operator":"any","values":["java"]}]}]}`)}}},
		{ToolCalls: []ToolCall{{ID: "summary", Name: ToolBatchTrendSummary, Params: json.RawMessage(`[{"dimension":"technology","value":"java","range":"month"}]`)}}},
		{Intent: crossRunnerIntent(DimensionTechnology, "java")},
	}}
	runner, err := NewTrendRunner(provider, crossRunnerOptions{options: snapshot.options, reader: reader}, crossRunnerSummary{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var events []StreamEvent
	if err := runner.Run(context.Background(), "上海 Java 岗位有多少", func(event StreamEvent) error { events = append(events, event); return nil }); err != nil {
		t.Fatal(err)
	}
	encodedEvents, _ := json.Marshal(events)
	encodedHistory, _ := json.Marshal(provider.history)
	if strings.Contains(string(encodedEvents), "SELECT secret") || strings.Contains(string(encodedHistory), "SELECT secret") {
		t.Fatalf("rejected tool arguments escaped: events=%s history=%s", encodedEvents, encodedHistory)
	}
}
