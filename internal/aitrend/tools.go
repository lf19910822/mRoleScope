package aitrend

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

type ToolName string

const (
	// ToolAnalyzeIntent is a display-only operation emitted while the model
	// prepares the structured intent. It is not registered as a Serving tool.
	ToolAnalyzeIntent            ToolName = "analyze_intent"
	ToolListTechnologyCategories ToolName = "list_available_categories"
	ToolListCategoryValues       ToolName = "list_category_values"
	ToolListAvailableValues      ToolName = "list_available_values"
	ToolRankValues               ToolName = "rank_values"
	ToolReadTrendSummary         ToolName = "read_trend_summary"
	ToolReadEventSummary         ToolName = "read_event_summary"
	ToolValidateCurveDraft       ToolName = "validate_curve_draft"
	ToolBatchTrendSummary        ToolName = "batch_trend_summary"
	ToolBatchEventSummary        ToolName = "batch_event_summary"
	ToolBatchValidateDraft       ToolName = "batch_validate_curve_draft"
	MaxToolOutputBytes                    = 256 * 1024
)

type ToolHandler func(context.Context, json.RawMessage) (json.RawMessage, error)

type ToolDefinition struct {
	Name    ToolName
	Handler ToolHandler
}

type ToolCall struct {
	Name   ToolName
	Params json.RawMessage
}

type ToolCallResult struct {
	Name       ToolName
	Payload    json.RawMessage
	SafeError  *SafeError
	CallNumber int
	Available  bool
}

type ToolRegistry struct {
	mu       sync.Mutex
	maxCalls int
	calls    int
	tools    map[ToolName]ToolHandler
}

func NewToolRegistry(definitions []ToolDefinition) (*ToolRegistry, error) {
	registry := &ToolRegistry{maxCalls: MaxToolCalls, tools: make(map[ToolName]ToolHandler, len(definitions))}
	for _, definition := range definitions {
		if definition.Name == "" || definition.Handler == nil {
			return nil, fmt.Errorf("invalid tool definition")
		}
		if _, exists := registry.tools[definition.Name]; exists {
			return nil, fmt.Errorf("duplicate tool definition: %s", definition.Name)
		}
		registry.tools[definition.Name] = definition.Handler
	}
	return registry, nil
}

func (registry *ToolRegistry) Call(ctx context.Context, call ToolCall) (ToolCallResult, error) {
	if registry == nil {
		return ToolCallResult{}, fmt.Errorf("tool registry is unavailable")
	}
	registry.mu.Lock()
	handler, ok := registry.tools[call.Name]
	if !ok {
		registry.mu.Unlock()
		return ToolCallResult{}, fmt.Errorf("tool is not registered: %s", call.Name)
	}
	if registry.calls >= registry.maxCalls {
		registry.mu.Unlock()
		return ToolCallResult{}, &ContractError{Code: ErrorLimitExceeded, Field: "tool_calls", Message: "tool call limit exceeded"}
	}
	registry.calls++
	callNumber := registry.calls
	registry.mu.Unlock()

	payload, err := handler(ctx, call.Params)
	if err != nil {
		return ToolCallResult{Name: call.Name, CallNumber: callNumber, Available: false}, err
	}
	if len(payload) > MaxToolOutputBytes {
		return ToolCallResult{Name: call.Name, CallNumber: callNumber, Available: false}, &ContractError{Code: ErrorLimitExceeded, Field: "tool_output", Message: "tool output exceeds bounded result size"}
	}
	return ToolCallResult{Name: call.Name, Payload: payload, CallNumber: callNumber, Available: true}, nil
}

func (registry *ToolRegistry) Calls() int {
	if registry == nil {
		return 0
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	return registry.calls
}

func ValidateBatchSize(count int) error {
	if count < 1 || count > MaxBatchCandidates {
		return &ContractError{Code: ErrorLimitExceeded, Field: "batch_size", Message: "batch size must be between one and eight"}
	}
	return nil
}
