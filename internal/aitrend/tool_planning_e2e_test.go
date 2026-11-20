package aitrend

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mrolescope/internal/jdserving"
)

func toolIntentJSON(id string, dimension Dimension, expressions []string, limit int) []byte {
	data, _ := json.Marshal(Intent{
		ContractVersion: ContractVersionV1,
		SubQueries: []SubQuery{{
			ID: id, Dimension: dimension, Expressions: expressions,
			Ranking: RankingIntent{Metric: MetricAverageDailyActiveJobs, Order: OrderDescending, Limit: limit},
		}},
		TimeRange: TimeRange{Kind: RangeMonth},
		Output:    OutputIntent{SeriesMetric: SeriesMetricDailyActiveJobs, ChartType: ChartTypeLine},
	})
	return data
}

func runToolScenario(t *testing.T, provider *scriptedToolProvider, options jdserving.FilterOptionsResponse, summary jdserving.TrendSummaryReader, query string) (*Draft, []string) {
	t.Helper()
	runner, err := NewTrendRunner(provider, runnerOptionsReader{options: options}, summary, nil)
	if err != nil {
		t.Fatal(err)
	}
	var draft *Draft
	tools := make([]string, 0)
	if err := runner.Run(context.Background(), query, func(event StreamEvent) error {
		if event.Type == StreamEventDraft {
			draft = event.Draft
		}
		if event.Type == StreamEventOperation && event.Status == StreamRunning && event.Tool != string(ToolAnalyzeIntent) {
			tools = append(tools, event.Tool)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return draft, tools
}

func languageOptions() jdserving.FilterOptionsResponse {
	return jdserving.FilterOptionsResponse{TechnologyKeywords: []jdserving.TechnologyKeywordFilterOption{
		{Category: jdserving.FilterOption{Value: "programming-languages", Label: "编程语言"}, Value: "cpp", Label: "C++"},
		{Category: jdserving.FilterOption{Value: "programming-languages", Label: "编程语言"}, Value: "python", Label: "Python"},
		{Category: jdserving.FilterOption{Value: "programming-languages", Label: "编程语言"}, Value: "go", Label: "Go"},
		{Category: jdserving.FilterOption{Value: "game-engines", Label: "游戏引擎"}, Value: "unreal-engine", Label: "Unreal Engine"},
	}}
}

func TestTrendToolPlanningScopeScenarios(t *testing.T) {
	t.Run("explicit values skip category discovery", func(t *testing.T) {
		provider := &scriptedToolProvider{turns: []IntentToolTurn{
			{ToolCalls: []ToolCall{
				{ID: "summary", Name: ToolBatchTrendSummary, Params: []byte(`[{"dimension":"technology","value":"go","range":"month"},{"dimension":"technology","value":"python","range":"month"}]`)},
			}},
			{Intent: toolIntentJSON("explicit", DimensionTechnology, []string{"go", "python"}, 2)},
		}}
		draft, tools := runToolScenario(t, provider, languageOptions(), languageSummaryReader{}, "比较 Go 和 Python")
		if draft == nil || len(draft.Curves) != 2 || containsTool(tools, string(ToolListTechnologyCategories)) {
			t.Fatalf("draft=%#v tools=%v", draft, tools)
		}
	})

	t.Run("unrestricted technology uses whole dimension", func(t *testing.T) {
		provider := &scriptedToolProvider{turns: []IntentToolTurn{
			{ToolCalls: []ToolCall{
				{ID: "values", Name: ToolListAvailableValues, Params: []byte(`{"dimension":"technology"}`)},
			}},
			{ToolCalls: []ToolCall{
				{ID: "summary", Name: ToolBatchTrendSummary, Params: []byte(`[{"dimension":"technology","value":"cpp","range":"month"},{"dimension":"technology","value":"python","range":"month"},{"dimension":"technology","value":"go","range":"month"},{"dimension":"technology","value":"unreal-engine","range":"month"}]`)},
			}},
			{Intent: toolIntentJSON("all-tech", DimensionTechnology, []string{"*"}, 8)},
		}}
		draft, tools := runToolScenario(t, provider, languageOptions(), languageSummaryReader{}, "哪些技术关键词？")
		if draft == nil || len(draft.Curves) != 4 || !containsTool(tools, string(ToolListAvailableValues)) {
			t.Fatalf("draft=%#v tools=%v", draft, tools)
		}
	})

	t.Run("unknown category remains unresolved", func(t *testing.T) {
		provider := &scriptedToolProvider{turns: []IntentToolTurn{
			{ToolCalls: []ToolCall{
				{ID: "unknown", Name: ToolListCategoryValues, Params: []byte(`{"category":"量子数据库"}`)},
			}},
			{Intent: toolIntentJSON("unknown", DimensionTechnology, []string{"量子数据库"}, 1)},
		}}
		draft, _ := runToolScenario(t, provider, languageOptions(), languageSummaryReader{}, "哪些量子数据库？")
		if draft == nil || len(draft.Curves) != 0 {
			t.Fatalf("unknown category draft=%#v", draft)
		}
	})

	t.Run("database category excludes languages", func(t *testing.T) {
		options := languageOptions()
		options.TechnologyKeywords = append(options.TechnologyKeywords,
			jdserving.TechnologyKeywordFilterOption{Category: jdserving.FilterOption{Value: "databases-and-storage", Label: "数据库与存储"}, Value: "mysql", Label: "MySQL"},
		)
		provider := &scriptedToolProvider{turns: []IntentToolTurn{
			{ToolCalls: []ToolCall{
				{ID: "categories", Name: ToolListTechnologyCategories, Params: []byte(`{}`)},
			}},
			{ToolCalls: []ToolCall{
				{ID: "database-values", Name: ToolListCategoryValues, Params: []byte(`{"category":"数据库与存储"}`)},
			}},
			{ToolCalls: []ToolCall{
				{ID: "summary", Name: ToolBatchTrendSummary, Params: []byte(`[{"dimension":"technology","value":"mysql","range":"month"}]`)},
			}},
			{Intent: toolIntentJSON("database", DimensionTechnology, []string{"mysql"}, 8)},
		}}
		draft, _ := runToolScenario(t, provider, options, languageSummaryReader{}, "哪些数据库正在被使用？")
		if draft == nil || len(draft.Curves) != 1 || draft.Curves[0].Condition.Value != "mysql" {
			t.Fatalf("database draft=%#v", draft)
		}
	})

	t.Run("backend batch failure keeps successful result", func(t *testing.T) {
		options := jdserving.FilterOptionsResponse{Projects: []jdserving.FilterOption{{Value: "good", Label: "Good"}, {Value: "bad", Label: "Bad"}}}
		provider := &scriptedToolProvider{turns: []IntentToolTurn{
			{ToolCalls: []ToolCall{
				{ID: "summary", Name: ToolBatchTrendSummary, Params: []byte(`[{"dimension":"project","value":"good","range":"month"},{"dimension":"project","value":"bad","range":"month"}]`)},
			}},
			{Intent: toolIntentJSON("projects", DimensionProject, []string{"good", "bad"}, 8)},
		}}
		draft, tools := runToolScenario(t, provider, options, partialBatchSummaryReader{}, "哪些项目的岗位最多？")
		if draft == nil || draft.Status != DraftPartial || len(draft.Curves) != 1 || draft.Curves[0].Condition.Value != "good" || !containsTool(tools, string(ToolBatchTrendSummary)) {
			t.Fatalf("partial draft=%#v tools=%v", draft, tools)
		}
	})
}

func containsTool(tools []string, wanted string) bool {
	for _, tool := range tools {
		if strings.TrimSpace(tool) == wanted {
			return true
		}
	}
	return false
}
