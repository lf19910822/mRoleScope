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

func TestOrchestratorFallsBackToGuidanceAfterInitialInvalidAgentResponse(t *testing.T) {
	provider := &scriptedToolProvider{turns: []IntentToolTurn{{Intent: []byte(`not-json`)}}}
	registry, err := NewToolRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	orchestrator := Orchestrator{ToolAnalyzer: provider, Registry: registry}
	result, err := orchestrator.RunWithInput(context.Background(), "你好")
	if err != nil || result.AgentResponse.ContractVersion != AgentContractVersionV1 || len(result.AgentResponse.ResponseModes) != 1 || result.AgentResponse.ResponseModes[0] != ResponseModeGuidance {
		t.Fatalf("result=%#v err=%v", result, err)
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

func TestBatchTrendPayloadFailureAllowsUsablePartialResults(t *testing.T) {
	payload := json.RawMessage(`[
    {"curve":{"dimension":"technology","value":"unknown"},"error":{"code":"backend_failure","message":"summary unavailable"}},
    {"curve":{"dimension":"technology","value":"go"},"average_daily_active_jobs":12}
  ]`)
	if err := batchTrendPayloadFailure(payload); err != nil {
		t.Fatalf("partial batch was treated as failed: %#v", err)
	}
}

func TestBatchTrendPayloadPartialFailureExposesItemError(t *testing.T) {
	payload := json.RawMessage(`[
    {"curve":{"dimension":"technology","value":"unknown"},"error":{"code":"unavailable_value","message":"value unavailable"}},
    {"curve":{"dimension":"technology","value":"go"},"average_daily_active_jobs":12}
  ]`)
	err := batchTrendPayloadPartialFailure(payload)
	if err == nil || err.Code != ErrorUnavailableValue {
		t.Fatalf("partial batch error = %#v, want unavailable_value", err)
	}
}

func TestBatchTrendPayloadFailureReportsAllFailedBatch(t *testing.T) {
	payload := json.RawMessage(`[
    {"curve":{"dimension":"technology","value":"unknown"},"error":{"code":"backend_failure","message":"summary unavailable"}},
    {"curve":{"dimension":"technology","value":"missing"},"error":{"code":"unavailable_value","message":"value unavailable"}}
  ]`)
	err := batchTrendPayloadFailure(payload)
	if err == nil || err.Code != ErrorBackendFailure {
		t.Fatalf("all-failed batch error = %#v, want backend_failure", err)
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
	if len(steps) != 7 {
		t.Fatalf("events = %#v", steps)
	}
	if steps[0].Tool != string(ToolAnalyzeIntent) || steps[0].Status != StreamRunning ||
		steps[1].Tool != string(ToolAnalyzeIntent) || steps[1].Status != StreamCompleted ||
		steps[2].Type != StreamEventValidation ||
		steps[3].Tool != string(ToolAnalyzeIntent) || steps[3].Status != StreamRunning ||
		steps[4].Tool != string(ToolAnalyzeIntent) || steps[4].Status != StreamCompleted ||
		steps[5].Tool != string(ToolListAvailableValues) || steps[5].Status != StreamRunning ||
		steps[6].Tool != string(ToolListAvailableValues) || steps[6].Status != StreamCompleted {
		t.Fatalf("events = %#v", steps)
	}
}

func TestOrchestratorEmitsModelOperationBeforeSlowAnalyzer(t *testing.T) {
	valid, err := json.Marshal(validIntent())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewToolRegistry([]ToolDefinition{{Name: ToolListAvailableValues, Handler: func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return []byte(`[]`), nil
	}}})
	if err != nil {
		t.Fatal(err)
	}
	steps := make([]StreamEvent, 0)
	orchestrator := Orchestrator{
		Analyzer: func(context.Context, *ContractError) ([]byte, error) {
			time.Sleep(10 * time.Millisecond)
			return valid, nil
		},
		Planner: func(context.Context, Intent, []ToolCallResult) ([]ToolCall, error) {
			return []ToolCall{{Name: ToolListAvailableValues}}, nil
		},
		Registry: registry,
		Emit:     func(event StreamEvent) error { steps = append(steps, event); return nil },
	}
	if _, err := orchestrator.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(steps) < 2 || steps[0].Type != StreamEventOperation || steps[0].Tool != string(ToolAnalyzeIntent) || steps[0].Status != StreamRunning {
		t.Fatalf("first events = %#v", steps)
	}
	if steps[1].Type != StreamEventOperation || steps[1].Tool != string(ToolAnalyzeIntent) || steps[1].Status != StreamCompleted {
		t.Fatalf("model completion event = %#v", steps[1])
	}
}

type scriptedToolProvider struct {
	turns   []IntentToolTurn
	history [][]ToolConversation
}

func (provider *scriptedToolProvider) AnalyzeIntent(context.Context, string, *ContractError) ([]byte, error) {
	return nil, errors.New("scripted provider uses tool turns")
}

func (provider *scriptedToolProvider) AnalyzeIntentTurn(_ context.Context, _ string, _ *ContractError, history []ToolConversation, _ bool) (IntentToolTurn, error) {
	provider.history = append(provider.history, history)
	if len(provider.turns) == 0 {
		return IntentToolTurn{}, errors.New("scripted provider exhausted")
	}
	turn := provider.turns[0]
	provider.turns = provider.turns[1:]
	return turn, nil
}

func TestOrchestratorRunsBoundedModelToolLoop(t *testing.T) {
	valid, err := json.Marshal(validIntent())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewToolRegistry([]ToolDefinition{
		{Name: ToolListTechnologyCategories, Handler: func(context.Context, json.RawMessage) (json.RawMessage, error) {
			return []byte(`[{"value":"programming-languages","label":"编程语言"}]`), nil
		}},
		{Name: ToolBatchTrendSummary, Handler: func(context.Context, json.RawMessage) (json.RawMessage, error) {
			return []byte(`[]`), nil
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	provider := &scriptedToolProvider{turns: []IntentToolTurn{
		{ToolCalls: []ToolCall{{ID: "categories", Name: ToolListTechnologyCategories, Params: []byte(`{}`)}}, InputTokens: 3, OutputTokens: 2},
		{ToolCalls: []ToolCall{{ID: "summary", Name: ToolBatchTrendSummary, Params: []byte(`[{"dimension":"technology","value":"cpp","range":"month"}]`)}}, InputTokens: 5, OutputTokens: 4},
		{Intent: valid, InputTokens: 7, OutputTokens: 6},
	}}
	steps := make([]StreamEvent, 0)
	orchestrator := Orchestrator{
		ToolAnalyzer: provider,
		Registry:     registry,
		MaxDuration:  time.Second,
		Emit:         func(event StreamEvent) error { steps = append(steps, event); return nil },
	}
	result, err := orchestrator.RunWithInput(context.Background(), "使用最多的编程语言是什么")
	if err != nil {
		t.Fatal(err)
	}
	if result.Intent.ContractVersion != ContractVersionV1 || result.AIAttempts != 1 || len(result.ToolResults) != 2 || result.InputTokens != 15 || result.OutputTokens != 12 {
		t.Fatalf("result = %#v", result)
	}
	if len(provider.history) != 3 || len(provider.history[2]) != 2 {
		t.Fatalf("provider history = %#v", provider.history)
	}
	if len(steps) < 8 {
		t.Fatalf("steps = %#v", steps)
	}
}

func TestOrchestratorAcceptsMixedAgentResponseWithTrendIntent(t *testing.T) {
	intent, err := json.Marshal(validIntent())
	if err != nil {
		t.Fatal(err)
	}
	wrapped := append([]byte(`{"contract_version":"ai-trend-agent-v1","response_modes":["trend","guidance"],"trend_intent":`), intent...)
	wrapped = append(wrapped, '}')
	provider := &scriptedToolProvider{turns: []IntentToolTurn{
		{ToolCalls: []ToolCall{{ID: "summary", Name: ToolBatchTrendSummary, Params: []byte(`[{}]`)}}, InputTokens: 1, OutputTokens: 1},
		{Intent: wrapped, InputTokens: 1, OutputTokens: 1},
	}}
	registry, err := NewToolRegistry([]ToolDefinition{{Name: ToolBatchTrendSummary, Handler: func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return []byte(`[]`), nil
	}}})
	if err != nil {
		t.Fatal(err)
	}
	orchestrator := Orchestrator{ToolAnalyzer: provider, Registry: registry}
	result, err := orchestrator.RunWithInput(context.Background(), "比较 Go 和 Python，并告诉我怎么修改曲线")
	if err != nil || result.Intent.ContractVersion != ContractVersionV1 || result.AgentResponse.ContractVersion != AgentContractVersionV1 || len(result.AgentResponse.ResponseModes) != 2 {
		t.Fatalf("mixed result=%#v err=%v", result, err)
	}
}

func TestOrchestratorDerivesResponseModesFromServiceTags(t *testing.T) {
	intent, err := json.Marshal(validIntent())
	if err != nil {
		t.Fatal(err)
	}
	// The planner model claims trend-only, but the service classifier has
	// already determined that this is a mixed request. The server must own the
	// outer response mode and normalize it to trend+guidance.
	wrapped := append([]byte(`{"contract_version":"ai-trend-agent-v1","response_modes":["trend"],"trend_intent":`), intent...)
	wrapped = append(wrapped, '}')
	provider := &scriptedToolProvider{turns: []IntentToolTurn{
		{ToolCalls: []ToolCall{{ID: "summary", Name: ToolBatchTrendSummary, Params: []byte(`[{"dimension":"technology","value":"go","range":"month"}]`)}}, InputTokens: 1, OutputTokens: 1},
		{Intent: wrapped, InputTokens: 1, OutputTokens: 1},
	}}
	registry, err := NewToolRegistry([]ToolDefinition{{Name: ToolBatchTrendSummary, Handler: func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return []byte(`[{"curve":{"dimension":"technology","value":"go"},"average_daily_active_jobs":12}]`), nil
	}}})
	if err != nil {
		t.Fatal(err)
	}
	orchestrator := Orchestrator{
		ToolAnalyzer: provider,
		Registry:     registry,
		ServiceTags:  []ServiceTag{ServiceTagGuidance, ServiceTagTrend},
	}
	result, err := orchestrator.RunWithInput(context.Background(), "比较 Go，并说明如何修改")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ServiceTags) != 2 || result.ServiceTags[0] != ServiceTagTrend || result.ServiceTags[1] != ServiceTagGuidance {
		t.Fatalf("service tags = %#v", result.ServiceTags)
	}
	if len(result.AgentResponse.ResponseModes) != 2 || result.AgentResponse.ResponseModes[0] != ResponseModeTrend || result.AgentResponse.ResponseModes[1] != ResponseModeGuidance {
		t.Fatalf("response modes = %#v", result.AgentResponse)
	}
}

func TestOrchestratorRequiresTrendSummaryBeforeFinalIntent(t *testing.T) {
	final, err := json.Marshal(validIntent())
	if err != nil {
		t.Fatal(err)
	}
	provider := &scriptedToolProvider{turns: []IntentToolTurn{
		{Intent: final},
		{ToolCalls: []ToolCall{{ID: "summary", Name: ToolBatchTrendSummary, Params: []byte(`[{"dimension":"technology","value":"go","range":"month"}]`)}}, InputTokens: 2, OutputTokens: 1},
		{Intent: final},
	}}
	registry, err := NewToolRegistry([]ToolDefinition{{Name: ToolBatchTrendSummary, Handler: func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return []byte(`[{"curve":{"dimension":"technology","value":"go"},"average_daily_active_jobs":12}]`), nil
	}}})
	if err != nil {
		t.Fatal(err)
	}
	orchestrator := Orchestrator{ToolAnalyzer: provider, Registry: registry}
	result, err := orchestrator.RunWithInput(context.Background(), "比较 Go 和 Python 的岗位数量")
	if err != nil {
		t.Fatal(err)
	}
	if result.Intent.ContractVersion != ContractVersionV1 || len(result.ToolResults) != 1 || result.ToolResults[0].Name != ToolBatchTrendSummary {
		t.Fatalf("result = %#v", result)
	}
}

type budgetToolProvider struct {
	final       []byte
	toolTurns   int
	toolsClosed bool
}

func (provider *budgetToolProvider) AnalyzeIntent(context.Context, string, *ContractError) ([]byte, error) {
	return nil, errors.New("budget provider uses tool turns")
}

func (provider *budgetToolProvider) AnalyzeIntentTurn(_ context.Context, _ string, _ *ContractError, _ []ToolConversation, allowTools bool) (IntentToolTurn, error) {
	if !allowTools {
		provider.toolsClosed = true
		return IntentToolTurn{Intent: provider.final}, nil
	}
	provider.toolTurns++
	return IntentToolTurn{ToolCalls: []ToolCall{{ID: "budget", Name: ToolListTechnologyCategories, Params: []byte(`{}`)}}, InputTokens: 1, OutputTokens: 1}, nil
}

func TestOrchestratorClosesToolsAtBudgetBeforeFinalIntent(t *testing.T) {
	final, _ := json.Marshal(validIntent())
	provider := &budgetToolProvider{final: final}
	registry, err := NewToolRegistry([]ToolDefinition{{Name: ToolListTechnologyCategories, Handler: func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return []byte(`[]`), nil
	}}})
	if err != nil {
		t.Fatal(err)
	}
	orchestrator := Orchestrator{ToolAnalyzer: provider, Registry: registry}
	result, err := orchestrator.RunWithInput(context.Background(), "哪些技术？")
	if err != nil || result.Intent.ContractVersion != ContractVersionV1 || len(result.ToolResults) != MaxToolCalls-1 || !provider.toolsClosed {
		t.Fatalf("result=%#v err=%v provider=%#v", result, err, provider)
	}
}

func TestOrchestratorToolLoopStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := &scriptedToolProvider{turns: []IntentToolTurn{{ToolCalls: []ToolCall{{ID: "cancel", Name: ToolRankValues, Params: []byte(`{"dimension":"technology","expression":"Go","sub_query_id":"q1"}`)}}}}}
	registry, err := NewToolRegistry([]ToolDefinition{{Name: ToolRankValues, Handler: func(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
		cancel()
		<-ctx.Done()
		return nil, ctx.Err()
	}}})
	if err != nil {
		t.Fatal(err)
	}
	orchestrator := Orchestrator{ToolAnalyzer: provider, Registry: registry, MaxDuration: time.Second}
	result, err := orchestrator.RunWithInput(ctx, "比较 Go")
	if err == nil || !result.Stopped || !result.Partial {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestOrchestratorFeedsInvalidToolArgumentsBackToModel(t *testing.T) {
	final, _ := json.Marshal(validIntent())
	provider := &scriptedToolProvider{turns: []IntentToolTurn{
		{ToolCalls: []ToolCall{{ID: "invalid", Name: "read_sql", Params: []byte(`{}`)}}},
		{Intent: final},
	}}
	registry, err := NewToolRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	orchestrator := Orchestrator{ToolAnalyzer: provider, Registry: registry}
	result, err := orchestrator.RunWithInput(context.Background(), "哪些技术？")
	if err != nil || result.Intent.ContractVersion != ContractVersionV1 || result.Partial || len(result.ToolResults) != 1 || result.ToolResults[0].SafeError == nil || result.ToolResults[0].SafeError.Code != ErrorInvalidEnum {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}
