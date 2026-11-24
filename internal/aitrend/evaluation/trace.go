package evaluation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"mrolescope/internal/aitrend"
)

const (
	DefaultInputNanoRMBPerToken  int64 = 800
	DefaultOutputNanoRMBPerToken int64 = 2000
)

type TraceStatus string

const (
	TraceSuccess         TraceStatus = "success"
	TraceSemanticFailure TraceStatus = "semantic_failure"
	TraceInfrastructure  TraceStatus = "infrastructure_failure"
	TraceIncomplete      TraceStatus = "incomplete"
	TraceLimitExceeded   TraceStatus = "limit_exceeded"
)

type UsageRates struct {
	InputNanoRMBPerToken  int64 `json:"input_nano_rmb_per_token"`
	OutputNanoRMBPerToken int64 `json:"output_nano_rmb_per_token"`
}

func DefaultUsageRates() UsageRates {
	return UsageRates{InputNanoRMBPerToken: DefaultInputNanoRMBPerToken, OutputNanoRMBPerToken: DefaultOutputNanoRMBPerToken}
}

func (rates UsageRates) Estimate(inputTokens, outputTokens int) (int64, error) {
	if rates.InputNanoRMBPerToken < 0 || rates.OutputNanoRMBPerToken < 0 || inputTokens < 0 || outputTokens < 0 {
		return 0, fmt.Errorf("usage rates and token counts must be non-negative")
	}
	return int64(inputTokens)*rates.InputNanoRMBPerToken + int64(outputTokens)*rates.OutputNanoRMBPerToken, nil
}

type Trace struct {
	CaseID                             string                 `json:"case_id"`
	Repetition                         int                    `json:"repetition"`
	Retry                              int                    `json:"retry"`
	Version                            VersionInput           `json:"version"`
	EvaluationVersionID                string                 `json:"evaluation_version_id"`
	Status                             TraceStatus            `json:"status"`
	ErrorCode                          string                 `json:"error_code,omitempty"`
	Intent                             *aitrend.Intent        `json:"intent,omitempty"`
	Operations                         []TraceOperation       `json:"operations"`
	ResponseModes                      []aitrend.ResponseMode `json:"response_modes,omitempty"`
	ServiceTags                        []aitrend.ServiceTag   `json:"service_tags,omitempty"`
	ClassificationStatus               string                 `json:"classification_status,omitempty"`
	ClassificationInputTokens          int                    `json:"classification_input_tokens"`
	ClassificationOutputTokens         int                    `json:"classification_output_tokens"`
	ClassificationDurationMS           int64                  `json:"classification_duration_ms"`
	AssistantReplyStatus               string                 `json:"assistant_reply_status,omitempty"`
	ReplyQualityWarnings               []string               `json:"reply_quality_warnings,omitempty"`
	Draft                              *aitrend.Draft         `json:"draft,omitempty"`
	Score                              Score                  `json:"score"`
	InputTokens                        int                    `json:"input_tokens"`
	OutputTokens                       int                    `json:"output_tokens"`
	ToolCalls                          int                    `json:"tool_calls"`
	ToolFailures                       int                    `json:"tool_failures"`
	EstimatedCostNanoRMB               int64                  `json:"estimated_cost_nano_rmb"`
	EstimatedClassificationCostNanoRMB int64                  `json:"estimated_classification_cost_nano_rmb"`
	FirstEventLatencyMS                int64                  `json:"first_event_latency_ms"`
	TotalDurationMS                    int64                  `json:"total_duration_ms"`
	CreatedAt                          time.Time              `json:"created_at"`
}

type TraceOperation struct {
	Tool      string               `json:"tool"`
	Status    aitrend.StreamStatus `json:"status"`
	Params    json.RawMessage      `json:"params,omitempty"`
	ErrorCode string               `json:"error_code,omitempty"`
}

func BuildTrace(item Case, version EvaluationVersion, attempt Attempt, score Score, rates UsageRates) (Trace, error) {
	if err := attempt.ResultStatsValidate(); err != nil {
		return Trace{}, err
	}
	if version.ID() == "" || item.ID == "" || attempt.CaseID != item.ID {
		return Trace{}, fmt.Errorf("trace identity is incomplete")
	}
	cost, err := rates.Estimate(attempt.Result.Stats.InputTokens, attempt.Result.Stats.OutputTokens)
	if err != nil {
		return Trace{}, err
	}
	classificationCost, err := rates.Estimate(attempt.Result.Stats.ClassificationInput, attempt.Result.Stats.ClassificationOutput)
	if err != nil {
		return Trace{}, err
	}
	status := traceStatus(attempt.Status)
	trace := Trace{
		CaseID: item.ID, Repetition: attempt.Repetition, Retry: attempt.Retry,
		Version: version.Metadata(), EvaluationVersionID: version.ID(), Status: status,
		ErrorCode: attempt.ErrorCode, Score: score,
		InputTokens: attempt.Result.Stats.InputTokens, OutputTokens: attempt.Result.Stats.OutputTokens,
		ToolCalls: attempt.Result.Stats.ToolCalls, ToolFailures: attempt.Result.Stats.ToolFailures,
		ResponseModes:              append([]aitrend.ResponseMode(nil), attempt.Result.Stats.ResponseModes...),
		ServiceTags:                append([]aitrend.ServiceTag(nil), attempt.Result.Stats.ServiceTags...),
		ClassificationStatus:       attempt.Result.Stats.ClassificationStatus,
		ClassificationInputTokens:  attempt.Result.Stats.ClassificationInput,
		ClassificationOutputTokens: attempt.Result.Stats.ClassificationOutput,
		ClassificationDurationMS:   attempt.Result.Stats.ClassificationTime.Milliseconds(),
		AssistantReplyStatus:       assistantReplyStatus(attempt.Result.Events),
		ReplyQualityWarnings:       append([]string(nil), attempt.Result.Stats.ReplyQualityWarnings...),
		EstimatedCostNanoRMB:       cost, EstimatedClassificationCostNanoRMB: classificationCost, FirstEventLatencyMS: attempt.Result.FirstEventLatency.Milliseconds(), TotalDurationMS: attempt.Result.TotalDuration.Milliseconds(),
		CreatedAt: time.Now().UTC(), Operations: traceOperations(attempt.Result.Events),
	}
	if attempt.Result.Stats.Intent != nil {
		intent := *attempt.Result.Stats.Intent
		trace.Intent = &intent
	}
	if attempt.Result.Draft != nil {
		draft := *attempt.Result.Draft
		trace.Draft = &draft
	}
	if err := trace.Validate(); err != nil {
		return Trace{}, err
	}
	return trace, nil
}

func assistantReplyStatus(events []aitrend.StreamEvent) string {
	status := ""
	for _, event := range events {
		if event.Type != aitrend.StreamEventAssistant {
			continue
		}
		switch event.Status {
		case aitrend.StreamRunning:
			status = "running"
		case aitrend.StreamCompleted:
			status = "completed"
		case aitrend.StreamStopped:
			status = "user_stopped"
		case aitrend.StreamFailed:
			status = "failed"
		}
	}
	return status
}

// ResultStatsValidate keeps Trace independent of the executor's internal
// attempt validation while enforcing the same bounded accounting contract.
func (attempt Attempt) ResultStatsValidate() error {
	if attempt.CaseID == "" || attempt.Repetition < 1 || attempt.Repetition > 3 || attempt.Retry < 0 || attempt.Retry > 2 {
		return fmt.Errorf("invalid trace attempt identity")
	}
	if attempt.Result.Stats.ToolCalls < 0 || attempt.Result.Stats.ToolCalls > MaxEvaluationToolCalls || attempt.Result.Stats.InputTokens < 0 || attempt.Result.Stats.OutputTokens < 0 || attempt.Result.Stats.ClassificationInput < 0 || attempt.Result.Stats.ClassificationOutput < 0 || attempt.Result.Stats.ClassificationTime < 0 {
		return fmt.Errorf("trace accounting exceeds limits")
	}
	return nil
}

func (trace Trace) Validate() error {
	if trace.CaseID == "" || trace.EvaluationVersionID == "" || trace.Repetition < 1 || trace.Repetition > 3 || trace.Retry < 0 || trace.Retry > 2 {
		return fmt.Errorf("invalid trace identity")
	}
	if trace.ToolCalls < 0 || trace.ToolCalls > MaxEvaluationToolCalls || trace.ToolFailures < 0 || trace.InputTokens < 0 || trace.OutputTokens < 0 || trace.EstimatedCostNanoRMB < 0 {
		return fmt.Errorf("trace counters exceed limits")
	}
	if trace.ClassificationInputTokens < 0 || trace.ClassificationOutputTokens < 0 || trace.ClassificationDurationMS < 0 || trace.EstimatedClassificationCostNanoRMB < 0 {
		return fmt.Errorf("classification trace counters exceed limits")
	}
	if trace.Status == "" {
		return fmt.Errorf("trace status is empty")
	}
	if len(trace.ResponseModes) > 2 {
		return fmt.Errorf("invalid Agent reply trace summary")
	}
	if len(trace.ServiceTags) > 0 {
		if _, err := aitrend.NormalizeServiceTags(trace.ServiceTags); err != nil {
			return fmt.Errorf("invalid service tags: %w", err)
		}
	}
	if trace.ClassificationStatus != "" && trace.ClassificationStatus != "running" && trace.ClassificationStatus != "completed" && trace.ClassificationStatus != "failed" {
		return fmt.Errorf("invalid classification status %q", trace.ClassificationStatus)
	}
	seenModes := make(map[aitrend.ResponseMode]struct{}, len(trace.ResponseModes))
	for _, mode := range trace.ResponseModes {
		if mode != aitrend.ResponseModeTrend && mode != aitrend.ResponseModeGuidance {
			return fmt.Errorf("invalid Agent response mode %q", mode)
		}
		if _, exists := seenModes[mode]; exists {
			return fmt.Errorf("duplicate Agent response mode %q", mode)
		}
		seenModes[mode] = struct{}{}
	}
	if trace.AssistantReplyStatus != "" && trace.AssistantReplyStatus != "completed" && trace.AssistantReplyStatus != "running" && trace.AssistantReplyStatus != "user_stopped" && trace.AssistantReplyStatus != "failed" {
		return fmt.Errorf("invalid Agent reply status %q", trace.AssistantReplyStatus)
	}
	return nil
}

func traceStatus(status AttemptStatus) TraceStatus {
	switch status {
	case AttemptComplete:
		return TraceSuccess
	case AttemptSemanticFailure:
		return TraceSemanticFailure
	case AttemptInfrastructure:
		return TraceInfrastructure
	case AttemptLimitExceeded:
		return TraceLimitExceeded
	default:
		return TraceIncomplete
	}
}

func traceOperations(events []aitrend.StreamEvent) []TraceOperation {
	operations := make([]TraceOperation, 0, len(events))
	for _, event := range events {
		if event.Type != aitrend.StreamEventOperation || event.Tool == string(aitrend.ToolAnalyzeIntent) || event.Tool == string(aitrend.ToolExplainDraft) {
			continue
		}
		params := safeJSON(event.Params)
		errorCode := ""
		if event.Error != nil {
			errorCode = string(event.Error.Code)
		}
		operations = append(operations, TraceOperation{Tool: event.Tool, Status: event.Status, Params: params, ErrorCode: errorCode})
	}
	return operations
}

func safeJSON(data json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return json.RawMessage(`{"redacted":"invalid_json"}`)
	}
	redactJSON(value)
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{"redacted":"unavailable"}`)
	}
	return encoded
}

func redactJSON(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "authorization") || strings.Contains(lower, "credential") || strings.Contains(lower, "jd") {
				typed[key] = "[redacted]"
				continue
			}
			redactJSON(child)
		}
	case []any:
		for _, child := range typed {
			redactJSON(child)
		}
	}
}

func TraceDigest(trace Trace) (string, error) {
	data, err := json.Marshal(trace)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}
