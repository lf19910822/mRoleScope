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
	ServiceTags     []ServiceTag
}

type OrchestrationResult struct {
	Intent               Intent
	AgentResponse        AgentResponse
	AIAttempts           int
	ToolResults          []ToolCallResult
	Partial              bool
	Stopped              bool
	InputTokens          int
	OutputTokens         int
	ReplyQualityWarnings []string
	ServiceTags          []ServiceTag
	ClassificationStatus string
	ClassificationInput  int
	ClassificationOutput int
	ClassificationTime   time.Duration
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
	if len(orchestrator.ServiceTags) > 0 {
		normalized, err := NormalizeServiceTags(orchestrator.ServiceTags)
		if err != nil {
			return OrchestrationResult{}, err
		}
		orchestrator.ServiceTags = normalized
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
	ctx = withServiceTags(ctx, orchestrator.ServiceTags)
	if orchestrator.ToolAnalyzer != nil {
		return orchestrator.runToolSession(ctx, input)
	}
	var correction *ContractError
	var intent Intent
	var agentResponse AgentResponse
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
		agentResponse, err = DecodeAgentResponse(encoded)
		if err == nil {
			if agentResponse.TrendIntent == nil {
				return OrchestrationResult{AgentResponse: agentResponse, AIAttempts: attemptsUsed}, nil
			}
			intent = *agentResponse.TrendIntent
			if debugErr := recordDebug(ctx, DebugTraceEvent{Type: DebugEventFinalIntent, Intent: &intent}); debugErr != nil {
				return OrchestrationResult{Intent: intent, AgentResponse: agentResponse, AIAttempts: attemptsUsed}, debugErr
			}
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
		return OrchestrationResult{Intent: intent, AgentResponse: agentResponse, AIAttempts: attemptsUsed}, err
	}
	if len(calls) > MaxToolCalls {
		return OrchestrationResult{Intent: intent, AgentResponse: agentResponse, AIAttempts: attemptsUsed}, &ContractError{Code: ErrorLimitExceeded, Field: "tool_calls", Message: "tool call limit exceeded"}
	}
	results := make([]ToolCallResult, 0, len(calls))
	partial := false
	for _, call := range calls {
		if ctx.Err() != nil {
			return OrchestrationResult{Intent: intent, AgentResponse: agentResponse, AIAttempts: attemptsUsed, ToolResults: results, Partial: true, Stopped: true}, ctx.Err()
		}
		if err := orchestrator.emit(StreamEvent{Type: StreamEventOperation, Status: StreamRunning, Tool: string(call.Name), Params: call.Params}); err != nil {
			return OrchestrationResult{Intent: intent, AgentResponse: agentResponse, AIAttempts: attemptsUsed, ToolResults: results, Partial: true}, err
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
				if result.SafeError == nil {
					result.SafeError = batchTrendPayloadPartialFailure(result.Payload)
				}
				if result.SafeError == nil && batchTrendPayloadHasItemErrors(result.Payload) {
					partial = true
				}
			}
			if result.Name == ToolRankTrendValues {
				result.SafeError = rankedTrendPayloadFailure(result.Payload)
			}
			if orchestrator.TokenCounter(result.Payload) > orchestrator.MaxOutputTokens {
				partial = true
				result.SafeError = &SafeError{Code: ErrorLimitExceeded, Field: "tool_output_tokens", Message: "tool output exceeded budget"}
				result.Available = false
			}
			if result.SafeError != nil {
				partial = true
			}
			status := StreamCompleted
			if result.SafeError != nil {
				status = StreamFailed
			}
			_ = orchestrator.emit(StreamEvent{Type: StreamEventOperation, Status: status, Tool: string(call.Name), Params: call.Params, Error: result.SafeError})
		}
		results = append(results, result)
		if ctx.Err() != nil {
			return OrchestrationResult{Intent: intent, AgentResponse: agentResponse, AIAttempts: attemptsUsed, ToolResults: results, Partial: true, Stopped: true}, ctx.Err()
		}
	}
	return OrchestrationResult{Intent: intent, AgentResponse: agentResponse, AIAttempts: attemptsUsed, ToolResults: results, Partial: partial}, nil
}

func (orchestrator Orchestrator) runToolSession(ctx context.Context, query string) (OrchestrationResult, error) {
	if orchestrator.ToolAnalyzer == nil || orchestrator.Registry == nil {
		return OrchestrationResult{}, &ContractError{Code: ErrorConfigurationFailure, Message: "tool analyzer is not configured"}
	}
	result := OrchestrationResult{ServiceTags: append([]ServiceTag(nil), orchestrator.ServiceTags...)}
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
		if result.AIAttempts == 0 {
			result.AIAttempts = 1
		}
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
			for callIndex, call := range turn.ToolCalls {
				contractErr := validateToolCallContract(call)
				eventParams := call.Params
				if call.Name == ToolCrossStatistics || contractErr != nil {
					eventParams = nil
				}
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
					exchange.Calls[callIndex].Params = json.RawMessage(`{}`)
					_ = recordDebug(ctx, DebugTraceEvent{Type: DebugEventToolResult, Tool: string(call.Name), Status: string(StreamFailed), Params: debugJSON(eventParams), Error: toolResult.SafeError})
					continue
				}
				if debugErr := recordDebug(ctx, DebugTraceEvent{Type: DebugEventToolCall, Tool: string(call.Name), Params: debugJSON(eventParams)}); debugErr != nil {
					return result, debugErr
				}
				var toolResult ToolCallResult
				var callErr error
				if contractErr == nil {
					if err := orchestrator.emit(StreamEvent{Type: StreamEventOperation, Status: StreamRunning, Tool: string(call.Name), Params: eventParams}); err != nil {
						return result, err
					}
				}
				if contractErr != nil {
					toolResult = ToolCallResult{Name: call.Name, CallNumber: toolCallsUsed, Available: false, SafeError: safeErrorFrom(contractErr)}
					callErr = contractErr
					_ = orchestrator.emit(StreamEvent{Type: StreamEventValidation, Status: StreamRunning, Tool: string(call.Name), Error: toolResult.SafeError})
				} else {
					toolResult, callErr = orchestrator.Registry.Call(ctx, call)
				}
				if callErr != nil {
					if call.Name == ToolCrossStatistics || contractErr != nil {
						exchange.Calls[callIndex].Params = json.RawMessage(`{}`)
					}
					if !isModelToolContractError(callErr) {
						result.Partial = true
					}
					toolResult.SafeError = safeErrorFrom(callErr)
					toolResult.Available = false
					if isModelToolContractError(callErr) {
						_ = orchestrator.emit(StreamEvent{Type: StreamEventValidation, Status: StreamRunning, Tool: string(call.Name), Error: toolResult.SafeError})
					}
					if contractErr == nil {
						_ = orchestrator.emit(StreamEvent{Type: StreamEventOperation, Status: StreamFailed, Tool: string(call.Name), Params: eventParams, Error: toolResult.SafeError})
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
						if toolResult.SafeError == nil {
							toolResult.SafeError = batchTrendPayloadPartialFailure(toolResult.Payload)
						}
						if toolResult.SafeError != nil {
							result.Partial = true
						} else if batchTrendPayloadHasItemErrors(toolResult.Payload) {
							result.Partial = true
						}
					}
					if toolResult.Name == ToolRankTrendValues {
						if toolResult.SafeError == nil {
							toolResult.SafeError = rankedTrendPayloadFailure(toolResult.Payload)
						}
						if toolResult.SafeError != nil {
							result.Partial = true
						}
					}
					status := StreamCompleted
					if toolResult.SafeError != nil {
						status = StreamFailed
					}
					_ = orchestrator.emit(StreamEvent{Type: StreamEventOperation, Status: status, Tool: string(call.Name), Params: eventParams, Error: toolResult.SafeError})
				}
				exchange.Results = append(exchange.Results, toolResult)
				result.ToolResults = append(result.ToolResults, toolResult)
				debugResult := DebugTraceEvent{Type: DebugEventToolResult, Tool: string(call.Name), Params: debugJSON(eventParams), Result: debugJSON(toolResult.Payload), Status: string(StreamCompleted), Error: toolResult.SafeError}
				if callErr != nil {
					debugResult.Status = string(StreamFailed)
				}
				if debugErr := recordDebug(ctx, debugResult); debugErr != nil {
					return result, debugErr
				}
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
		agentResponse, decodeErr := DecodeAgentResponse(turn.Intent)
		if decodeErr != nil {
			contractErr, ok := decodeErr.(*ContractError)
			if !ok {
				return result, decodeErr
			}
			if len(history) == 0 && recoverInitialAgentContract(contractErr) {
				result.AgentResponse = AgentResponse{ContractVersion: AgentContractVersionV1, ResponseModes: []ResponseMode{ResponseModeGuidance}}
				result.AIAttempts = modelTurns + 1
				return result, nil
			}
			correctionAttempts++
			_ = orchestrator.emit(StreamEvent{Type: StreamEventValidation, Status: StreamRunning, Error: safeErrorFrom(contractErr)})
			if correctionAttempts >= MaxAIAttempts {
				result.AIAttempts = MaxAIAttempts
				return result, contractErr
			}
			result.AIAttempts = correctionAttempts + 1
			correction = contractErr
			continue
		}
		if crossRequiredFromContext(ctx) && !historyContainsTool(history, ToolCrossStatistics) && !toolBudgetExhausted {
			contractErr := &ContractError{Code: ErrorMissingField, Field: "cross_statistics", Message: "用户要求交叉统计，请先调用当前岗位统计工具"}
			correctionAttempts++
			_ = orchestrator.emit(StreamEvent{Type: StreamEventValidation, Status: StreamRunning, Error: safeErrorFrom(contractErr)})
			if correctionAttempts >= MaxAIAttempts {
				result.AIAttempts = MaxAIAttempts
				return result, contractErr
			}
			result.AIAttempts = correctionAttempts + 1
			correction = contractErr
			continue
		}
		if len(orchestrator.ServiceTags) > 0 {
			expectedModes, modeErr := ResponseModesFromServiceTags(orchestrator.ServiceTags)
			if modeErr != nil {
				return result, modeErr
			}
			if serviceTagsContain(orchestrator.ServiceTags, ServiceTagTrend) && agentResponse.TrendIntent == nil {
				contractErr := &ContractError{Code: ErrorMissingField, Field: "trend_intent", Message: "trend service tag requires a trend Intent"}
				correctionAttempts++
				_ = orchestrator.emit(StreamEvent{Type: StreamEventValidation, Status: StreamRunning, Error: safeErrorFrom(contractErr)})
				if correctionAttempts >= MaxAIAttempts {
					result.AIAttempts = MaxAIAttempts
					return result, contractErr
				}
				result.AIAttempts = correctionAttempts + 1
				correction = contractErr
				continue
			}
			if !serviceTagsContain(orchestrator.ServiceTags, ServiceTagTrend) {
				agentResponse.TrendIntent = nil
			}
			agentResponse.ResponseModes = expectedModes
		}
		if agentResponse.TrendIntent == nil {
			result.AgentResponse = agentResponse
			return result, nil
		}
		if !historyHasTrendSummaryCall(history) && !toolBudgetExhausted && !historyHasToolFailure(history) {
			contractErr := &ContractError{Code: ErrorInvalidCombination, Field: "model_response", Message: "AI must call a trend summary tool before final Intent"}
			correctionAttempts++
			_ = orchestrator.emit(StreamEvent{Type: StreamEventValidation, Status: StreamRunning, Error: safeErrorFrom(contractErr)})
			if correctionAttempts >= MaxAIAttempts {
				result.AIAttempts = MaxAIAttempts
				return result, contractErr
			}
			result.AIAttempts = correctionAttempts + 1
			correction = contractErr
			continue
		}
		result.AgentResponse = agentResponse
		result.Intent = *agentResponse.TrendIntent
		if debugErr := recordDebug(ctx, DebugTraceEvent{Type: DebugEventFinalIntent, Intent: &result.Intent}); debugErr != nil {
			return result, debugErr
		}
		return result, nil
	}
	return result, &ContractError{Code: ErrorLimitExceeded, Field: "model_turns", Message: "AI model turn limit exceeded"}
}

func recoverInitialAgentContract(err *ContractError) bool {
	if err == nil {
		return false
	}
	switch err.Code {
	case ErrorInvalidJSON, ErrorInvalidEnum, ErrorInvalidCombination, ErrorMissingField:
		return true
	default:
		return false
	}
}

func historyHasTrendSummaryCall(history []ToolConversation) bool {
	for _, exchange := range history {
		for _, call := range exchange.Calls {
			if call.Name == ToolBatchTrendSummary || call.Name == ToolReadTrendSummary {
				return true
			}
		}
	}
	return false
}

func historyHasToolFailure(history []ToolConversation) bool {
	for _, exchange := range history {
		for _, result := range exchange.Results {
			if result.SafeError != nil {
				return true
			}
		}
	}
	return false
}

func historyHasModelToolFailure(history []ToolConversation) bool {
	if len(history) == 0 {
		return false
	}
	last := history[len(history)-1]
	for _, result := range last.Results {
		if result.SafeError != nil && isModelToolContractError(&ContractError{Code: result.SafeError.Code}) {
			return true
		}
	}
	return false
}

func isModelToolContractError(err error) bool {
	contractErr, ok := err.(*ContractError)
	if !ok {
		return false
	}
	switch contractErr.Code {
	case ErrorInvalidJSON, ErrorMissingField, ErrorInvalidEnum, ErrorInvalidCombination, ErrorInvalidValue, ErrorUnavailableValue:
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
	var firstError *SafeError
	for _, summary := range summaries {
		if summary.Error == nil {
			// A batch may contain unavailable values alongside usable summaries.
			// Keep the usable rows available to the planner and let it filter the
			// individual failed candidates from the draft.
			return nil
		}
		if firstError == nil {
			firstError = summary.Error
		}
	}
	if firstError != nil {
		return firstError
	}
	return &SafeError{Code: ErrorBackendFailure, Field: "summaries", Message: "trend summary batch returned no usable result"}
}

func batchTrendPayloadHasItemErrors(payload json.RawMessage) bool {
	var summaries []summaryToolResponse
	if err := json.Unmarshal(payload, &summaries); err != nil {
		return false
	}
	for _, summary := range summaries {
		if summary.Error != nil {
			return true
		}
	}
	return false
}

func batchTrendPayloadPartialFailure(payload json.RawMessage) *SafeError {
	var summaries []summaryToolResponse
	if err := json.Unmarshal(payload, &summaries); err != nil {
		return nil
	}
	var firstError *SafeError
	hasUsable := false
	for _, summary := range summaries {
		if summary.Error == nil {
			hasUsable = true
			continue
		}
		if firstError == nil {
			firstError = summary.Error
		}
	}
	if hasUsable {
		return firstError
	}
	return nil
}

func rankedTrendPayloadFailure(payload json.RawMessage) *SafeError {
	var response rankedTrendValuesResponse
	if err := json.Unmarshal(payload, &response); err != nil || len(response.FailedValues) == 0 {
		return nil
	}
	return &SafeError{Code: ErrorBackendFailure, Field: "ranked_values", Message: "部分趋势候选读取失败"}
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
