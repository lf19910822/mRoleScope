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
	ToolAnalyzer    IntentToolProvider
	Planner         ToolPlanner
	Registry        *ToolRegistry
	MaxDuration     time.Duration
	MaxInputTokens  int
	MaxOutputTokens int
	TokenCounter    func([]byte) int
	Emit            func(StreamEvent) error
}

type OrchestrationResult struct {
	Intent       Intent
	AIAttempts   int
	ToolResults  []ToolCallResult
	Partial      bool
	Stopped      bool
	InputTokens  int
	OutputTokens int
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

func (orchestrator Orchestrator) run(ctx context.Context, input string) (OrchestrationResult, error) {
	if orchestrator.Registry == nil || (orchestrator.ToolAnalyzer == nil && (orchestrator.Analyzer == nil || orchestrator.Planner == nil)) {
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
	if orchestrator.ToolAnalyzer != nil {
		return orchestrator.runToolSession(ctx, input)
	}
	var correction *ContractError
	var intent Intent
	attemptsUsed := 0
	for attempt := 1; attempt <= MaxAIAttempts; attempt++ {
		attemptsUsed = attempt
		modelOperation := string(ToolAnalyzeIntent)
		if err := orchestrator.emit(StreamEvent{Type: StreamEventOperation, Status: StreamRunning, Tool: modelOperation}); err != nil {
			return OrchestrationResult{AIAttempts: attempt}, err
		}
		encoded, err := orchestrator.Analyzer(ctx, correction)
		if err != nil {
			_ = orchestrator.emit(StreamEvent{
				Type:   StreamEventOperation,
				Status: StreamFailed,
				Tool:   modelOperation,
				Error:  safeErrorFrom(err),
			})
			if ctx.Err() != nil {
				return OrchestrationResult{AIAttempts: attempt, Stopped: true}, ctx.Err()
			}
			return OrchestrationResult{AIAttempts: attempt}, err
		}
		if orchestrator.TokenCounter(encoded) > orchestrator.MaxOutputTokens {
			limitErr := &ContractError{Code: ErrorLimitExceeded, Field: "ai_output_tokens", Message: "AI output token budget exceeded"}
			_ = orchestrator.emit(StreamEvent{
				Type:   StreamEventOperation,
				Status: StreamFailed,
				Tool:   modelOperation,
				Error:  safeErrorFrom(limitErr),
			})
			return OrchestrationResult{AIAttempts: attempt}, limitErr
		}
		if err := orchestrator.emit(StreamEvent{Type: StreamEventOperation, Status: StreamCompleted, Tool: modelOperation}); err != nil {
			return OrchestrationResult{AIAttempts: attempt}, err
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
			if result.Name == ToolBatchTrendSummary {
				result.SafeError = batchTrendPayloadFailure(result.Payload)
			}
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

func (orchestrator Orchestrator) runToolSession(ctx context.Context, query string) (OrchestrationResult, error) {
	if orchestrator.ToolAnalyzer == nil || orchestrator.Registry == nil {
		return OrchestrationResult{}, &ContractError{Code: ErrorConfigurationFailure, Message: "tool analyzer is not configured"}
	}
	result := OrchestrationResult{}
	history := make([]ToolConversation, 0)
	var correction *ContractError
	correctionAttempts := 0
	toolCallsUsed := 0
	toolBudgetExhausted := false
	modelToolLimit := MaxToolCalls - 1 // reserve the final registry call for draft validation
	// A model may need several turns to discover a category, read values,
	// summarize candidates, and finally emit Intent. The tool budget remains
	// the hard bound; this turn bound prevents a provider loop from running
	// forever when it never emits a final Intent.
	for modelTurns := 0; modelTurns < MaxToolCalls+MaxAIAttempts+5; modelTurns++ {
		if ctx.Err() != nil {
			result.Partial = true
			result.Stopped = true
			return result, ctx.Err()
		}
		result.AIAttempts++
		modelOperation := string(ToolAnalyzeIntent)
		if err := orchestrator.emit(StreamEvent{Type: StreamEventOperation, Status: StreamRunning, Tool: modelOperation}); err != nil {
			return result, err
		}
		turn, err := orchestrator.ToolAnalyzer.AnalyzeIntentTurn(ctx, query, correction, history, !toolBudgetExhausted)
		result.InputTokens += turn.InputTokens
		result.OutputTokens += turn.OutputTokens
		if err != nil {
			_ = orchestrator.emit(StreamEvent{Type: StreamEventOperation, Status: StreamFailed, Tool: modelOperation, Error: safeErrorFrom(err)})
			if ctx.Err() != nil {
				result.Partial = true
				result.Stopped = true
				return result, ctx.Err()
			}
			return result, err
		}
		if err := orchestrator.emit(StreamEvent{Type: StreamEventOperation, Status: StreamCompleted, Tool: modelOperation}); err != nil {
			return result, err
		}
		if len(turn.Intent) > 0 && len(turn.ToolCalls) > 0 {
			return result, &ContractError{Code: ErrorInvalidCombination, Field: "model_response", Message: "AI 同时返回了工具调用和最终意图"}
		}
		if len(turn.Intent) > 0 && orchestrator.TokenCounter(turn.Intent) > orchestrator.MaxOutputTokens {
			return result, &ContractError{Code: ErrorLimitExceeded, Field: "ai_output_tokens", Message: "AI output token budget exceeded"}
		}
		if len(turn.ToolCalls) > 0 {
			if toolBudgetExhausted {
				return result, &ContractError{Code: ErrorLimitExceeded, Field: "tool_calls", Message: "tool call limit exceeded before final Intent"}
			}
			exchange := ToolConversation{Calls: append([]ToolCall(nil), turn.ToolCalls...)}
			for _, call := range turn.ToolCalls {
				toolCallsUsed++
				if toolCallsUsed > modelToolLimit {
					toolBudgetExhausted = true
					result.Partial = true
					toolResult := ToolCallResult{
						Name: call.Name, CallNumber: toolCallsUsed, Available: false,
						SafeError: &SafeError{Code: ErrorLimitExceeded, Field: "tool_calls", Message: "tool call limit exceeded"},
					}
					exchange.Results = append(exchange.Results, toolResult)
					result.ToolResults = append(result.ToolResults, toolResult)
					continue
				}
				var toolResult ToolCallResult
				var callErr error
				contractErr := validateToolCallContract(call)
				if contractErr == nil {
					if err := orchestrator.emit(StreamEvent{Type: StreamEventOperation, Status: StreamRunning, Tool: string(call.Name), Params: call.Params}); err != nil {
						return result, err
					}
				}
				if contractErr != nil {
					toolResult = ToolCallResult{Name: call.Name, CallNumber: toolCallsUsed, Available: false, SafeError: safeErrorFrom(contractErr)}
					callErr = contractErr
				} else {
					toolResult, callErr = orchestrator.Registry.Call(ctx, call)
				}
				if callErr != nil {
					if !isModelToolContractError(callErr) {
						result.Partial = true
					}
					toolResult.SafeError = safeErrorFrom(callErr)
					toolResult.Available = false
					if contractErr == nil {
						_ = orchestrator.emit(StreamEvent{Type: StreamEventOperation, Status: StreamFailed, Tool: string(call.Name), Params: call.Params, Error: toolResult.SafeError})
					}
				} else {
					if orchestrator.TokenCounter(toolResult.Payload) > orchestrator.MaxOutputTokens {
						toolResult.SafeError = &SafeError{Code: ErrorLimitExceeded, Field: "tool_output_tokens", Message: "tool output exceeded budget"}
						toolResult.Available = false
						result.Partial = true
					}
					if toolResult.Name == ToolBatchTrendSummary {
						if toolResult.SafeError == nil {
							toolResult.SafeError = batchTrendPayloadFailure(toolResult.Payload)
						}
						if toolResult.SafeError != nil {
							result.Partial = true
						}
					}
					status := StreamCompleted
					if toolResult.SafeError != nil {
						status = StreamFailed
					}
					_ = orchestrator.emit(StreamEvent{Type: StreamEventOperation, Status: status, Tool: string(call.Name), Params: call.Params, Error: toolResult.SafeError})
				}
				exchange.Results = append(exchange.Results, toolResult)
				result.ToolResults = append(result.ToolResults, toolResult)
				if toolCallsUsed >= modelToolLimit {
					toolBudgetExhausted = true
				}
				if ctx.Err() != nil {
					result.Partial = true
					result.Stopped = true
					return result, ctx.Err()
				}
			}
			history = append(history, exchange)
			correction = nil
			continue
		}
		if len(turn.Intent) == 0 {
			return result, &ContractError{Code: ErrorMissingField, Field: "intent", Message: "AI returned neither tool calls nor final Intent"}
		}
		intent, decodeErr := DecodeIntent(turn.Intent)
		if decodeErr == nil {
			result.Intent = intent
			return result, nil
		}
		contractErr, ok := decodeErr.(*ContractError)
		if !ok {
			return result, decodeErr
		}
		correctionAttempts++
		_ = orchestrator.emit(StreamEvent{Type: StreamEventValidation, Status: StreamRunning, Error: safeErrorFrom(contractErr)})
		if correctionAttempts >= MaxAIAttempts {
			return result, contractErr
		}
		correction = contractErr
	}
	return result, &ContractError{Code: ErrorLimitExceeded, Field: "model_turns", Message: "AI model turn limit exceeded"}
}

func isModelToolContractError(err error) bool {
	contractErr, ok := err.(*ContractError)
	if !ok {
		return false
	}
	switch contractErr.Code {
	case ErrorInvalidJSON, ErrorMissingField, ErrorInvalidEnum, ErrorInvalidCombination, ErrorUnavailableValue:
		return true
	default:
		return false
	}
}

func batchTrendPayloadFailure(payload json.RawMessage) *SafeError {
	var summaries []summaryToolResponse
	if err := json.Unmarshal(payload, &summaries); err != nil {
		return nil
	}
	for _, summary := range summaries {
		if summary.Error != nil {
			return summary.Error
		}
	}
	return nil
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
