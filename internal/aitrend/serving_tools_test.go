package aitrend

import (
	"context"
	"encoding/json"
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
