package main

import (
	"context"
	"encoding/json"
	"testing"

	"mrolescope/internal/aitrend"
	"mrolescope/internal/aitrend/evaluation"
)

func TestFixtureProviderFollowsTrendSummaryHandshakeForCategorySubset(t *testing.T) {
	provider := &fixtureProvider{item: evaluation.Case{
		ID: "D05", Dimension: "technology", ExpectedCategory: "databases-and-storage",
		Expressions: []string{"databases-and-storage"}, ExpectedValues: []string{}, ExpectedStatus: "complete",
	}}
	categoryTurn, err := provider.AnalyzeIntentTurn(context.Background(), provider.item.Query, nil, nil, true)
	if err != nil || len(categoryTurn.ToolCalls) != 1 || categoryTurn.ToolCalls[0].Name != aitrend.ToolListCategoryValues {
		t.Fatalf("category discovery turn = %#v, err=%v", categoryTurn, err)
	}
	summaryTurn, err := provider.AnalyzeIntentTurn(context.Background(), provider.item.Query, nil, nil, true)
	if err != nil || len(summaryTurn.ToolCalls) != 1 || summaryTurn.ToolCalls[0].Name != aitrend.ToolBatchTrendSummary {
		t.Fatalf("summary turn = %#v, err=%v", summaryTurn, err)
	}
	var requests []struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(summaryTurn.ToolCalls[0].Params, &requests); err != nil {
		t.Fatal(err)
	}
	seenMySQL := false
	for _, request := range requests {
		if request.Value == "mysql" {
			seenMySQL = true
		}
	}
	if !seenMySQL {
		t.Fatalf("summary requests did not retain required mysql value: %#v", requests)
	}
	finalTurn, err := provider.AnalyzeIntentTurn(context.Background(), provider.item.Query, nil, nil, true)
	if err != nil || len(finalTurn.Intent) == 0 {
		t.Fatalf("final intent turn = %#v, err=%v", finalTurn, err)
	}
}
