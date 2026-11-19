package aitrend

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestOrchestratorRetriesOnlyInvalidIntent(t *testing.T) {
	valid, err := json.Marshal(validIntent())
	if err != nil {
		t.Fatal(err)
	}
	callCount := 0
	registry, err := NewToolRegistry([]ToolDefinition{{Name: ToolRankValues, Handler: func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"ok":true}`), nil
	}}})
	if err != nil {
		t.Fatal(err)
	}
	orchestrator := Orchestrator{
		Registry: registry,
		Analyzer: func(context.Context, *ContractError) ([]byte, error) {
			callCount++
			if callCount == 1 {
				return []byte(`{"contract_version":"ai-trend-draft-v1","sub_queries":[]}`), nil
			}
			return valid, nil
		},
		Planner: func(context.Context, Intent, []ToolCallResult) ([]ToolCall, error) {
			return []ToolCall{{Name: ToolRankValues}}, nil
		},
	}
	result, err := orchestrator.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.AIAttempts != 2 || callCount != 2 || len(result.ToolResults) != 1 || !result.ToolResults[0].Available {
		t.Fatalf("result = %#v, analyzer calls = %d", result, callCount)
	}
}

func TestOrchestratorStopsAfterThreeInvalidAttempts(t *testing.T) {
	callCount := 0
	registry, _ := NewToolRegistry([]ToolDefinition{{Name: ToolRankValues, Handler: func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("must not run")
	}}})
	orchestrator := Orchestrator{
		Registry: registry,
		Analyzer: func(context.Context, *ContractError) ([]byte, error) {
			callCount++
			return []byte(`{"contract_version":"wrong"}`), nil
		},
		Planner: func(context.Context, Intent, []ToolCallResult) ([]ToolCall, error) {
			return []ToolCall{{Name: ToolRankValues}}, nil
		},
	}
	_, err := orchestrator.Run(context.Background())
	if err == nil || callCount != MaxAIAttempts || registry.Calls() != 0 {
		t.Fatalf("err = %v, analyzer calls = %d, tool calls = %d", err, callCount, registry.Calls())
	}
}

func TestOrchestratorDoesNotRetryBackendToolFailure(t *testing.T) {
	analyzerCalls := 0
	registry, _ := NewToolRegistry([]ToolDefinition{{Name: ToolRankValues, Handler: func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("backend unavailable")
	}}})
	orchestrator := Orchestrator{
		Registry: registry,
		Analyzer: func(context.Context, *ContractError) ([]byte, error) {
			analyzerCalls++
			return json.Marshal(validIntent())
		},
		Planner: func(context.Context, Intent, []ToolCallResult) ([]ToolCall, error) {
			return []ToolCall{{Name: ToolRankValues}}, nil
		},
	}
	result, err := orchestrator.Run(context.Background())
	if err != nil || analyzerCalls != 1 || !result.Partial || len(result.ToolResults) != 1 || result.ToolResults[0].SafeError == nil || result.ToolResults[0].SafeError.Code != ErrorBackendFailure {
		t.Fatalf("result = %#v, analyzer calls = %d, err = %v", result, analyzerCalls, err)
	}
}

func TestOrchestratorEmitsValidationAndOperationEvents(t *testing.T) {
	valid, err := json.Marshal(validIntent())
	if err != nil {
		t.Fatal(err)
	}
	steps := []StreamEvent{}
	registry, err := NewToolRegistry([]ToolDefinition{{Name: ToolListAvailableValues, Handler: func(context.Context, json.RawMessage) (json.RawMessage, error) { return []byte(`[]`), nil }}})
	if err != nil {
		t.Fatal(err)
	}
	attempts := 0
	orchestrator := Orchestrator{
		Analyzer: func(context.Context, *ContractError) ([]byte, error) {
			attempts++
			if attempts == 1 {
				return []byte(`{"contract_version":"bad"}`), nil
			}
			return valid, nil
		},
		Planner: func(context.Context, Intent, []ToolCallResult) ([]ToolCall, error) {
			return []ToolCall{{Name: ToolListAvailableValues, Params: []byte(`{"dimension":"technology"}`)}}, nil
		},
		Registry:    registry,
		MaxDuration: time.Second,
		Emit:        func(event StreamEvent) error { steps = append(steps, event); return nil },
	}
	if _, err := orchestrator.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(steps) != 3 || steps[0].Type != StreamEventValidation || steps[1].Type != StreamEventOperation || steps[2].Status != StreamCompleted {
		t.Fatalf("events = %#v", steps)
	}
}
