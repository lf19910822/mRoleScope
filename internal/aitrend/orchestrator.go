package aitrend

import (
	"context"
	"encoding/json"
	"time"
)

type IntentAnalyzer func(context.Context, *ContractError) ([]byte, error)
type ToolPlanner func(context.Context, Intent, []ToolCallResult) ([]ToolCall, error)

type Orchestrator struct {
	Analyzer        IntentAnalyzer
	Planner         ToolPlanner
	Registry        *ToolRegistry
	MaxDuration     time.Duration
	MaxInputTokens  int
	MaxOutputTokens int
	TokenCounter    func([]byte) int
	Emit            func(StreamEvent) error
}

type OrchestrationResult struct {
	Intent      Intent
	AIAttempts  int
	ToolResults []ToolCallResult
	Partial     bool
	Stopped     bool
}

func (orchestrator Orchestrator) Run(ctx context.Context) (OrchestrationResult, error) {
	return orchestrator.run(ctx, "")
}

// RunWithInput applies the request token budget before any model or tool call.
// The input is counted transiently and is never persisted or emitted.
func (orchestrator Orchestrator) RunWithInput(ctx context.Context, input string) (OrchestrationResult, error) {
	if orchestrator.MaxInputTokens <= 0 {
		orchestrator.MaxInputTokens = 12_000
	}
	counter := orchestrator.TokenCounter
	if counter == nil {
		counter = func(data []byte) int { return (len(data) + 3) / 4 }
	}
	if counter([]byte(input)) > orchestrator.MaxInputTokens {
		return OrchestrationResult{}, &ContractError{Code: ErrorLimitExceeded, Field: "input_tokens", Message: "request token budget exceeded"}
	}
	return orchestrator.run(ctx, input)
}

func (orchestrator Orchestrator) run(ctx context.Context, _ string) (OrchestrationResult, error) {
	if orchestrator.Analyzer == nil || orchestrator.Planner == nil || orchestrator.Registry == nil {
		return OrchestrationResult{}, &ContractError{Code: ErrorConfigurationFailure, Message: "orchestrator is not configured"}
	}
	if orchestrator.MaxDuration <= 0 {
		orchestrator.MaxDuration = 2 * time.Minute
	}
	if orchestrator.MaxInputTokens <= 0 {
		orchestrator.MaxInputTokens = 12_000
	}
	if orchestrator.MaxOutputTokens <= 0 {
		orchestrator.MaxOutputTokens = 16_000
	}
	if orchestrator.TokenCounter == nil {
		orchestrator.TokenCounter = func(data []byte) int { return (len(data) + 3) / 4 }
	}
	ctx, cancel := context.WithTimeout(ctx, orchestrator.MaxDuration)
	defer cancel()
	var correction *ContractError
	var intent Intent
	attemptsUsed := 0
	for attempt := 1; attempt <= MaxAIAttempts; attempt++ {
		attemptsUsed = attempt
		encoded, err := orchestrator.Analyzer(ctx, correction)
		if err != nil {
			if ctx.Err() != nil {
				return OrchestrationResult{AIAttempts: attempt, Stopped: true}, ctx.Err()
			}
			return OrchestrationResult{AIAttempts: attempt}, err
		}
		if orchestrator.TokenCounter(encoded) > orchestrator.MaxOutputTokens {
			return OrchestrationResult{AIAttempts: attempt}, &ContractError{Code: ErrorLimitExceeded, Field: "ai_output_tokens", Message: "AI output token budget exceeded"}
		}
		intent, err = DecodeIntent(encoded)
		if err == nil {
			break
		}
		contractErr, ok := err.(*ContractError)
		if !ok {
			return OrchestrationResult{AIAttempts: attempt}, err
		}
		_ = orchestrator.emit(StreamEvent{Type: StreamEventValidation, Status: StreamRunning, Error: safeErrorFrom(contractErr)})
		correction = contractErr
		if attempt == MaxAIAttempts {
			return OrchestrationResult{AIAttempts: attempt}, contractErr
		}
	}

	calls, err := orchestrator.Planner(ctx, intent, nil)
	if err != nil {
		return OrchestrationResult{Intent: intent, AIAttempts: attemptsUsed}, err
	}
	if len(calls) > MaxToolCalls {
		return OrchestrationResult{Intent: intent, AIAttempts: attemptsUsed}, &ContractError{Code: ErrorLimitExceeded, Field: "tool_calls", Message: "tool call limit exceeded"}
	}
	results := make([]ToolCallResult, 0, len(calls))
	partial := false
	for _, call := range calls {
		if ctx.Err() != nil {
			return OrchestrationResult{Intent: intent, AIAttempts: attemptsUsed, ToolResults: results, Partial: true, Stopped: true}, ctx.Err()
		}
		if err := orchestrator.emit(StreamEvent{Type: StreamEventOperation, Status: StreamRunning, Tool: string(call.Name), Params: call.Params}); err != nil {
			return OrchestrationResult{Intent: intent, AIAttempts: attemptsUsed, ToolResults: results, Partial: true}, err
		}
		result, callErr := orchestrator.Registry.Call(ctx, call)
		if callErr != nil {
			partial = true
			result.SafeError = safeErrorFrom(callErr)
			result.Available = false
			_ = orchestrator.emit(StreamEvent{Type: StreamEventOperation, Status: StreamFailed, Tool: string(call.Name), Params: call.Params, Error: result.SafeError})
		} else {
			if orchestrator.TokenCounter(result.Payload) > orchestrator.MaxOutputTokens {
				partial = true
				result.SafeError = &SafeError{Code: ErrorLimitExceeded, Field: "tool_output_tokens", Message: "tool output exceeded budget"}
				result.Available = false
			}
			status := StreamCompleted
			if result.SafeError != nil {
				status = StreamFailed
			}
			_ = orchestrator.emit(StreamEvent{Type: StreamEventOperation, Status: status, Tool: string(call.Name), Params: call.Params, Error: result.SafeError})
		}
		results = append(results, result)
		if ctx.Err() != nil {
			return OrchestrationResult{Intent: intent, AIAttempts: attemptsUsed, ToolResults: results, Partial: true, Stopped: true}, ctx.Err()
		}
	}
	return OrchestrationResult{Intent: intent, AIAttempts: attemptsUsed, ToolResults: results, Partial: partial}, nil
}

func (orchestrator Orchestrator) emit(event StreamEvent) error {
	if orchestrator.Emit == nil {
		return nil
	}
	return orchestrator.Emit(event)
}

func safeErrorFrom(err error) *SafeError {
	if contractErr, ok := err.(*ContractError); ok {
		return &SafeError{Code: contractErr.Code, Field: contractErr.Field, Message: contractErr.Message}
	}
	return &SafeError{Code: ErrorBackendFailure, Message: "tool execution failed"}
}

func MarshalIntent(intent Intent) ([]byte, error) { return json.Marshal(intent) }
