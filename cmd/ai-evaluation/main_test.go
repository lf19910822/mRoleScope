package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"mrolescope/internal/aitrend"
	"mrolescope/internal/aitrend/evaluation"
)

func TestSelectCasesByIDPreservesFixedSampleOrder(t *testing.T) {
	cases := []evaluation.Case{
		{ID: "E01"}, {ID: "C01"}, {ID: "R01"}, {ID: "M01"}, {ID: "U01"}, {ID: "D01"},
	}
	selected, err := selectCasesByID(cases, "E01,C01,R01,M01,U01,D01")
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 6 {
		t.Fatalf("selected %d cases, want 6", len(selected))
	}
	for index, want := range []string{"E01", "C01", "R01", "M01", "U01", "D01"} {
		if selected[index].ID != want {
			t.Fatalf("selected[%d] = %q, want %q", index, selected[index].ID, want)
		}
	}
}

func TestSelectCasesByIDRejectsUnknownAndDuplicateIDs(t *testing.T) {
	cases := []evaluation.Case{{ID: "E01"}, {ID: "C01"}}
	for _, raw := range []string{"E01,E01", "E01,missing"} {
		if _, err := selectCasesByID(cases, raw); err == nil {
			t.Fatalf("selectCasesByID(%q) succeeded, want error", raw)
		}
	}
}

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

func TestFixtureProviderReturnsReviewedServiceTags(t *testing.T) {
	provider := fixtureProvider{item: evaluation.Case{ID: "M01", ExpectedServiceTags: []string{"trend", "guidance"}}}
	classification, input, output, err := provider.ClassifyServiceModes(context.Background(), "哪些项目？并说明怎么修改")
	if err != nil || input != 3 || output != 1 || len(classification.ServiceTags) != 2 || classification.ServiceTags[0] != aitrend.ServiceTagTrend || classification.ServiceTags[1] != aitrend.ServiceTagGuidance {
		t.Fatalf("fixture classification=%#v usage=%d/%d err=%v", classification, input, output, err)
	}
}

func TestEvaluationTraceContextGroupsCaseAttempts(t *testing.T) {
	version, err := evaluation.NewEvaluationVersion(evaluation.VersionInput{
		RunID: "run-1", Model: "fixture", Provider: "fixture", PromptVersion: "p", ToolSchemaVersion: "t",
		SnapshotVersion: "s", CodeRevision: "c", CasesVersion: "cases", ScorerVersion: "score",
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	first := evaluationTraceContext("run-1", "agent", evaluation.Case{ID: "E01"}, version, 1, 0)
	second := evaluationTraceContext("run-1", "agent", evaluation.Case{ID: "E01"}, version, 2, 1)
	other := evaluationTraceContext("run-2", "agent", evaluation.Case{ID: "E01"}, version, 1, 0)
	if first.SessionID != "eval:run-1:agent:E01" || second.SessionID != first.SessionID || other.SessionID == first.SessionID {
		t.Fatalf("session IDs = %q/%q/%q", first.SessionID, second.SessionID, other.SessionID)
	}
	if first.Metadata["evaluation_repetition"] != "1" || second.Metadata["evaluation_retry"] != "1" {
		t.Fatalf("attempt metadata = %#v/%#v", first.Metadata, second.Metadata)
	}
}
