package aitrend

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestTrendOrchestrationEndToEndWithCorrectionAndPartialToolFailure(t *testing.T) {
	valid, _ := json.Marshal(validIntent())
	registry, err := NewToolRegistry([]ToolDefinition{
		{Name: ToolListTechnologyCategories, Handler: func(context.Context, json.RawMessage) (json.RawMessage, error) {
			return []byte(`[{"value":"databases-and-storage"}]`), nil
		}},
		{Name: ToolRankValues, Handler: func(context.Context, json.RawMessage) (json.RawMessage, error) {
			return nil, errors.New("serving unavailable")
		}},
		{Name: ToolBatchTrendSummary, Handler: func(context.Context, json.RawMessage) (json.RawMessage, error) {
			return []byte(`[{"period_end_active_jobs":2}]`), nil
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	attempt := 0
	events := make([]StreamEvent, 0)
	orchestrator := Orchestrator{
		Analyzer: func(context.Context, *ContractError) ([]byte, error) {
			attempt++
			if attempt == 1 {
				return []byte(`{"contract_version":"wrong"}`), nil
			}
			return valid, nil
		},
		Planner: func(context.Context, Intent, []ToolCallResult) ([]ToolCall, error) {
			return []ToolCall{{Name: ToolListTechnologyCategories}, {Name: ToolRankValues}, {Name: ToolBatchTrendSummary, Params: []byte(`[]`)}}, nil
		},
		Registry: registry,
		Emit:     func(event StreamEvent) error { events = append(events, event); return nil },
	}
	result, err := orchestrator.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Partial || result.AIAttempts != 2 || len(result.ToolResults) != 3 {
		t.Fatalf("result = %#v", result)
	}
	failureSeen := false
	for _, event := range events {
		if event.Type == StreamEventOperation && event.Status == StreamFailed {
			failureSeen = true
		}
	}
	if !failureSeen {
		t.Fatalf("events did not retain backend failure: %#v", events)
	}

	oldDraft, err := BuildDraft([]CandidateGroup{{SubQueryID: "old", Candidates: []Candidate{{Dimension: DimensionTechnology, Value: "mysql", Label: "MySQL", MatchStatus: MatchExact}}}}, DraftBuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Partial && oldDraft.Curves[0].Label != "MySQL" {
		t.Fatal("partial request should not replace old draft")
	}
	newDraft, err := BuildDraft([]CandidateGroup{{SubQueryID: "new", Candidates: []Candidate{{Dimension: DimensionTechnology, Value: "redis", Label: "Redis", MatchStatus: MatchExact}}}}, DraftBuildOptions{})
	if err != nil || newDraft.Curves[0].Label != "Redis" {
		t.Fatalf("new draft = %#v err=%v", newDraft, err)
	}
}

func TestTrendOrchestrationStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	registry, _ := NewToolRegistry([]ToolDefinition{{Name: ToolRankValues, Handler: func(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
		cancel()
		<-ctx.Done()
		return nil, ctx.Err()
	}}})
	orchestrator := Orchestrator{Analyzer: func(context.Context, *ContractError) ([]byte, error) { return json.Marshal(validIntent()) }, Planner: func(context.Context, Intent, []ToolCallResult) ([]ToolCall, error) {
		return []ToolCall{{Name: ToolRankValues}}, nil
	}, Registry: registry, MaxDuration: time.Second}
	result, err := orchestrator.Run(ctx)
	if err == nil || !result.Stopped || !result.Partial {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}
