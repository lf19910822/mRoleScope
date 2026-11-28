package aitrend

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

type ToolName string

const (
	// ToolClassifyServiceModes is a display-only operation emitted by the
	// lightweight service classification call. It is not a Serving tool.
	ToolClassifyServiceModes ToolName = "classify_service_modes"
	// ToolAnalyzeIntent is a display-only operation emitted while the model
	// prepares the structured intent. It is not registered as a Serving tool.
	ToolAnalyzeIntent ToolName = "analyze_intent"
	// ToolExplainDraft is a display-only operation emitted while the model
	// explains a completed draft. It is not registered as a Serving tool.
	ToolExplainDraft             ToolName = "explain_draft"
	ToolListTechnologyCategories ToolName = "list_available_categories"
	ToolListCategoryValues       ToolName = "list_category_values"
	ToolListAvailableValues      ToolName = "list_available_values"
	ToolRankValues               ToolName = "rank_values"
	ToolRankTrendValues          ToolName = "rank_trend_values"
	ToolReadTrendSummary         ToolName = "read_trend_summary"
	ToolReadEventSummary         ToolName = "read_event_summary"
	ToolValidateCurveDraft       ToolName = "validate_curve_draft"
	ToolBatchTrendSummary        ToolName = "batch_trend_summary"
	ToolBatchEventSummary        ToolName = "batch_event_summary"
	ToolBatchValidateDraft       ToolName = "batch_validate_curve_draft"
	ToolCrossStatistics          ToolName = "query_current_job_statistics"
	MaxToolOutputBytes                    = 256 * 1024
)

type ToolHandler func(context.Context, json.RawMessage) (json.RawMessage, error)

type ToolDefinition struct {
	Name        ToolName
	Description string
	Parameters  json.RawMessage
	Handler     ToolHandler
}

type ToolCall struct {
	ID     string
	Name   ToolName
	Params json.RawMessage
}

// ToolConversation is the bounded history exchanged between the model and the
// serving tool registry. It contains only safe tool payloads, never provider
// messages or raw source data.
type ToolConversation struct {
	Calls   []ToolCall
	Results []ToolCallResult
}

// IntentToolTurn is one model turn. Exactly one of Intent or ToolCalls is
// expected after provider response validation.
type IntentToolTurn struct {
	Intent       json.RawMessage
	ToolCalls    []ToolCall
	InputTokens  int
	OutputTokens int
}

// IntentToolProvider is implemented by providers that can continue a model
// conversation after receiving bounded serving-tool results.
type IntentToolProvider interface {
	AnalyzeIntentTurn(context.Context, string, *ContractError, []ToolConversation, bool) (IntentToolTurn, error)
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

// ToolDescriptor is the model-facing contract without exposing handler
// functions or other server internals.
type ToolDescriptor struct {
	Name        ToolName
	Description string
	Parameters  json.RawMessage
}

// Descriptors returns only tools registered by this registry in stable order.
func (registry *ToolRegistry) Descriptors() []ToolDescriptor {
	if registry == nil {
		return nil
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	result := make([]ToolDescriptor, 0, len(registry.tools))
	for _, descriptor := range trendToolDescriptors() {
		if _, ok := registry.tools[descriptor.Name]; ok {
			result = append(result, descriptor)
		}
	}
	return result
}

func trendToolDescriptors() []ToolDescriptor {
	return []ToolDescriptor{
		{Name: ToolListTechnologyCategories, Description: "List the current formal technology categories.", Parameters: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`)},
		{Name: ToolListCategoryValues, Description: "List formal technology values in one technology category.", Parameters: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["category"],"properties":{"category":{"type":"string","minLength":1}}}`)},
		{Name: ToolListAvailableValues, Description: "List bounded formal values for one supported dimension.", Parameters: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["dimension"],"properties":{"dimension":{"enum":["technology","project","job_family","location"]}}}`)},
		{Name: ToolRankValues, Description: "Resolve a bounded expression against formal values.", Parameters: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["dimension","expression","sub_query_id"],"properties":{"dimension":{"enum":["technology","project","job_family","location"]},"expression":{"type":"string","minLength":1},"sub_query_id":{"type":"string","minLength":1}}}`)},
		{Name: ToolRankTrendValues, Description: "Rank all formal values in one dimension by a trend metric and return the top bounded candidates.", Parameters: trendRankToolParameters()},
		{Name: ToolReadTrendSummary, Description: "Read a safe aggregate trend summary for one formal curve.", Parameters: trendToolParameters()},
		{Name: ToolReadEventSummary, Description: "Read safe aggregate event counts for one formal curve.", Parameters: trendToolParameters()},
		{Name: ToolValidateCurveDraft, Description: "Validate one curve draft against the active formal vocabulary.", Parameters: json.RawMessage(`{"type":"object"}`)},
		{Name: ToolBatchTrendSummary, Description: "Read up to eight safe aggregate trend summaries.", Parameters: batchTrendToolParameters()},
		{Name: ToolBatchEventSummary, Description: "Read up to eight safe aggregate event summaries.", Parameters: batchTrendToolParameters()},
		{Name: ToolBatchValidateDraft, Description: "Validate up to eight curve drafts.", Parameters: json.RawMessage(`{"type":"array","maxItems":8}`)},
		{Name: ToolCrossStatistics, Description: "Count or rank current effective jobs using confirmed formal conditions; groups are ANDed, values within a group are any/all, and exclusions apply to the same jobs. Historical cross statistics and cross-dimension OR are unavailable. Results are aggregates only; charts remain independent single-dimension trends.", Parameters: crossStatisticsToolParameters()},
	}
}

func crossStatisticsToolParameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","additionalProperties":false,"required":["sub_queries"],"properties":{"sub_queries":{"type":"array","minItems":1,"maxItems":16,"items":{"type":"object","additionalProperties":false,"required":["id","time"],"properties":{"id":{"type":"string","minLength":1,"maxLength":128},"time":{"enum":["current","historical_reference"]},"group_by":{"enum":["technology","project","job_family","location"]},"include_secondary":{"type":"boolean"},"ranking_limit":{"type":"integer","minimum":0,"maximum":1000000},"page_size":{"type":"integer","minimum":1,"maximum":100},"conditions":{"type":"array","maxItems":16,"items":{"type":"object","additionalProperties":false,"required":["dimension","operator"],"properties":{"dimension":{"enum":["technology","project","job_family","location"]},"operator":{"enum":["any","all"]},"values":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":128}},"categories":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":128}}}}},"exclusions":{"type":"array","maxItems":16,"items":{"type":"object","additionalProperties":false,"required":["dimension","operator"],"properties":{"dimension":{"enum":["technology","project","job_family","location"]},"operator":{"enum":["any","all"]},"values":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":128}},"categories":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":128}}}}}}}}}}`)
}

func validateToolCallContract(call ToolCall) error {
	if call.Name == "" {
		return contractError(ErrorMissingField, "tool", "tool name is required")
	}
	known := false
	for _, descriptor := range trendToolDescriptors() {
		if descriptor.Name == call.Name {
			known = true
			break
		}
	}
	if !known {
		return contractError(ErrorInvalidEnum, "tool", "tool is not allowed")
	}
	if len(call.Params) > MaxToolOutputBytes {
		return contractError(ErrorLimitExceeded, "params", "tool parameters exceed bounded size")
	}
	if len(call.Params) > 0 && !json.Valid(call.Params) {
		return contractError(ErrorInvalidJSON, "params", "tool parameters are invalid JSON")
	}
	return nil
}

func trendToolParameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","additionalProperties":false,"required":["dimension","value","range"],"properties":{"dimension":{"enum":["technology","project","job_family","location"]},"value":{"type":"string","minLength":1},"range":{"enum":["week","month","quarter","year","all","custom"]},"start_date":{"type":"string"},"end_date":{"type":"string"},"include_secondary":{"type":"boolean"},"operator":{"enum":["eq","gt","gte","lt","lte"]}}}`)
}

func trendRankToolParameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","additionalProperties":false,"required":["dimension","range","metric","limit","sub_query_id"],"properties":{"dimension":{"enum":["technology","project","job_family","location"]},"range":{"enum":["week","month","quarter","year","all","custom"]},"start_date":{"type":"string"},"end_date":{"type":"string"},"metric":{"enum":["avg_daily_active_jobs","period_end_active_jobs","peak_daily_active_jobs","period_unique_jobs","entered_jobs","modified_jobs","left_jobs","reopened_jobs"]},"limit":{"type":"integer","minimum":1,"maximum":8},"sub_query_id":{"type":"string","minLength":1},"include_secondary":{"type":"boolean"}}}`)
}

func batchTrendToolParameters() json.RawMessage {
	return json.RawMessage(`{"type":"array","minItems":1,"maxItems":8,"items":{"type":"object","additionalProperties":false,"required":["dimension","value","range"],"properties":{"dimension":{"enum":["technology","project","job_family","location"]},"value":{"type":"string","minLength":1},"range":{"enum":["week","month","quarter","year","all","custom"]},"start_date":{"type":"string"},"end_date":{"type":"string"},"include_secondary":{"type":"boolean"},"operator":{"enum":["eq","gt","gte","lt","lte"]}}}}`)
}

func ValidateBatchSize(count int) error {
	if count < 1 || count > MaxBatchCandidates {
		return &ContractError{Code: ErrorLimitExceeded, Field: "batch_size", Message: "batch size must be between one and eight"}
	}
	return nil
}
