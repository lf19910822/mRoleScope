package aitrend

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"mrolescope/internal/jdserving"
)

type fakeFilterOptions struct{}

func (fakeFilterOptions) ListFilterOptions(context.Context) (jdserving.FilterOptionsResponse, error) {
	return jdserving.FilterOptionsResponse{
		TechnologyKeywords: []jdserving.TechnologyKeywordFilterOption{
			{Category: jdserving.FilterOption{Value: "databases-and-storage", Label: "数据库与存储"}, Value: "redis", Label: "Redis"},
			{Category: jdserving.FilterOption{Value: "databases-and-storage", Label: "数据库与存储"}, Value: "mysql", Label: "MySQL"},
		},
		Projects:  []jdserving.FilterOption{{Value: "alpha", Label: "Alpha"}},
		Locations: []jdserving.FilterOption{{Value: "上海", Label: "上海"}},
	}, nil
}

type countingFilterOptions struct {
	options jdserving.FilterOptionsResponse
	calls   int
}

func (reader *countingFilterOptions) ListFilterOptions(context.Context) (jdserving.FilterOptionsResponse, error) {
	reader.calls++
	return reader.options, nil
}

func TestServingToolSnapshotReusesOneVocabularyReadAndAcceptsCategoryLabel(t *testing.T) {
	reader := &countingFilterOptions{
		options: jdserving.FilterOptionsResponse{
			TechnologyKeywords: []jdserving.TechnologyKeywordFilterOption{
				{Category: jdserving.FilterOption{Value: "programming-languages", Label: "编程语言"}, Value: "cpp", Label: "C++"},
				{Category: jdserving.FilterOption{Value: "programming-languages", Label: "编程语言"}, Value: "python", Label: "Python"},
			},
		},
	}
	service := ServingToolService{Options: reader}
	snapshot, err := service.snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	service.Snapshot = snapshot
	registry, err := service.Registry()
	if err != nil {
		t.Fatal(err)
	}
	for _, params := range []string{`{}`, `{"category":"编程语言"}`, `{"category":"programming-languages"}`} {
		call := ToolListTechnologyCategories
		if params != `{}` {
			call = ToolListCategoryValues
		}
		result, callErr := registry.Call(context.Background(), ToolCall{Name: call, Params: json.RawMessage(params)})
		if callErr != nil || !result.Available {
			t.Fatalf("tool %s params=%s err=%v result=%#v", call, params, callErr, result)
		}
	}
	if reader.calls != 1 {
		t.Fatalf("filter options reads = %d, want one snapshot read", reader.calls)
	}
}

func TestServingToolCategoryRejectsUnknownWithoutWildcard(t *testing.T) {
	service := ServingToolService{Options: fakeFilterOptions{}}
	registry, err := service.Registry()
	if err != nil {
		t.Fatal(err)
	}
	result, err := registry.Call(context.Background(), ToolCall{Name: ToolListCategoryValues, Params: json.RawMessage(`{"category":"量子数据库"}`)})
	if err == nil || result.Available {
		t.Fatalf("unknown category result = %#v, err=%v", result, err)
	}
	contractErr, ok := err.(*ContractError)
	if !ok || contractErr.Code != ErrorInvalidValue {
		t.Fatalf("unknown category error = %#v", err)
	}
}

func TestServingToolCategoryErrorIsDistinctFromUnavailableFormalValue(t *testing.T) {
	service := ServingToolService{Options: fakeFilterOptions{}}
	registry, err := service.Registry()
	if err != nil {
		t.Fatal(err)
	}
	result, err := registry.Call(context.Background(), ToolCall{Name: ToolListCategoryValues, Params: json.RawMessage(`{"category":"database"}`)})
	if err == nil || result.Available {
		t.Fatalf("invalid category result = %#v, err=%v", result, err)
	}
	contractErr, ok := err.(*ContractError)
	if !ok || contractErr.Code != ErrorInvalidValue || contractErr.Field != "category" {
		t.Fatalf("invalid category error = %#v, want invalid_value/category", err)
	}
}

func TestToolRegistryDescriptorsExposeOnlyRegisteredTools(t *testing.T) {
	service := ServingToolService{Options: fakeFilterOptions{}}
	registry, err := service.Registry()
	if err != nil {
		t.Fatal(err)
	}
	descriptors := registry.Descriptors()
	if len(descriptors) != 11 {
		t.Fatalf("descriptor count = %d, want 11", len(descriptors))
	}
	for _, descriptor := range descriptors {
		if descriptor.Name == ToolAnalyzeIntent || descriptor.Name == ToolExplainDraft || len(descriptor.Parameters) == 0 {
			t.Fatalf("invalid model descriptor = %#v", descriptor)
		}
	}
}

func TestServingToolRegistryExposesOnlyBoundedFormalValues(t *testing.T) {
	service := ServingToolService{Options: fakeFilterOptions{}, Resolver: Resolver{Values: map[Dimension][]Candidate{
		DimensionTechnology: {{Dimension: DimensionTechnology, Value: "redis", Label: "Redis", MatchStatus: MatchExact}},
	}}}
	registry, err := service.Registry()
	if err != nil {
		t.Fatal(err)
	}
	result, err := registry.Call(context.Background(), ToolCall{Name: ToolListCategoryValues, Params: json.RawMessage(`{"category":"databases-and-storage"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Available || string(result.Payload) == "" || string(result.Payload) == "null" {
		t.Fatalf("unexpected tool result: %#v", result)
	}
	if _, err := registry.Call(context.Background(), ToolCall{Name: ToolListAvailableValues, Params: json.RawMessage(`{"dimension":"technology","raw_sql":"select 1"}`)}); err == nil {
		t.Fatal("unknown tool parameter was accepted")
	}
}

func TestServingToolDraftRejectsUnknownFormalValueAsInvalidValue(t *testing.T) {
	service := ServingToolService{Resolver: Resolver{Values: map[Dimension][]Candidate{
		DimensionTechnology: {{Dimension: DimensionTechnology, Value: "mysql", Label: "MySQL"}},
	}}}
	registry, err := service.Registry()
	if err != nil {
		t.Fatal(err)
	}
	result, err := registry.Call(context.Background(), ToolCall{Name: ToolValidateCurveDraft, Params: json.RawMessage(`{"contract_version":"ai-trend-draft-v1","status":"complete","curves":[{"condition":{"dimension":"technology","value":"database"},"label":"database","selected":true,"match_status":"exact","reason":"test","sub_query_ids":["q1"]}]}`)})
	if err == nil || result.Available {
		t.Fatalf("unknown formal value result=%#v err=%v", result, err)
	}
	contractErr, ok := err.(*ContractError)
	if !ok || contractErr.Code != ErrorInvalidValue {
		t.Fatalf("unknown formal value error=%#v, want invalid_value", err)
	}
}

func TestServingToolBatchRejectsMoreThanEight(t *testing.T) {
	service := ServingToolService{Options: fakeFilterOptions{}}
	registry, err := service.Registry()
	if err != nil {
		t.Fatal(err)
	}
	requests := make([]map[string]any, MaxBatchCandidates+1)
	for index := range requests {
		requests[index] = map[string]any{"dimension": "technology"}
	}
	data, _ := json.Marshal(requests)
	if _, err := registry.Call(context.Background(), ToolCall{Name: ToolBatchTrendSummary, Params: data}); err == nil {
		t.Fatal("batch tool accepted more than eight requests")
	}
}

func TestServingToolProjectValuesExcludeUnrecognizedTrendPseudoValue(t *testing.T) {
	service := ServingToolService{Options: runnerOptionsReader{options: jdserving.FilterOptionsResponse{
		Projects: []jdserving.FilterOption{
			{Value: "alpha", Label: "Alpha"},
			{Value: "unrecognized", Label: "未识别项目"},
		},
	}}}
	registry, err := service.Registry()
	if err != nil {
		t.Fatal(err)
	}
	result, err := registry.Call(context.Background(), ToolCall{Name: ToolListAvailableValues, Params: json.RawMessage(`{"dimension":"project"}`)})
	if err != nil {
		t.Fatal(err)
	}
	var values []FilterOption
	if err := json.Unmarshal(result.Payload, &values); err != nil {
		t.Fatal(err)
	}
	for _, value := range values {
		if value.Value == "unrecognized" {
			t.Fatalf("trend tool returned filter-only pseudo value: %#v", values)
		}
	}
}

type partialBatchSummaryReader struct{}

func (partialBatchSummaryReader) ReadTrendSummary(_ context.Context, query jdserving.TrendQuery) (jdserving.TrendSummary, error) {
	if query.Condition.Value == "bad" {
		return jdserving.TrendSummary{}, errors.New("summary unavailable")
	}
	value := 12.0
	return jdserving.TrendSummary{
		Curve:                  jdserving.TrendCurve{Dimension: jdserving.TrendDimensionProject, Value: query.Condition.Value, Label: query.Condition.Value},
		AverageDailyActiveJobs: &value,
	}, nil
}

func TestServingToolBatchTrendSummaryPreservesSuccessfulItems(t *testing.T) {
	service := ServingToolService{Summary: partialBatchSummaryReader{}}
	registry, err := service.Registry()
	if err != nil {
		t.Fatal(err)
	}
	result, err := registry.Call(context.Background(), ToolCall{Name: ToolBatchTrendSummary, Params: json.RawMessage(`[{"dimension":"project","value":"good","range":"all"},{"dimension":"project","value":"bad","range":"all"}]`)})
	if err != nil || !result.Available {
		t.Fatalf("batch result err=%v result=%#v", err, result)
	}
	var summaries []summaryToolResponse
	if err := json.Unmarshal(result.Payload, &summaries); err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 2 || summaries[0].Error != nil || summaries[1].Error == nil {
		t.Fatalf("summaries = %#v", summaries)
	}
}
