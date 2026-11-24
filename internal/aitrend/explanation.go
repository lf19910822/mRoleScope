package aitrend

import (
	"context"
	"strings"
	"unicode/utf8"
)

// DraftExplanationCandidate is the bounded, display-safe context sent to the
// explanation model. It contains formal labels and aggregate ranking values,
// never raw tool payloads or source content.
type DraftExplanationCandidate struct {
	Label       string      `json:"label"`
	MatchStatus MatchStatus `json:"match_status"`
	Scope       string      `json:"scope,omitempty"`
	MetricValue *float64    `json:"metric_value,omitempty"`
}

// DraftExplanationContext is the complete bounded context for one explanation
// request. Curves are already selected and ordered by the deterministic runner.
type DraftExplanationContext struct {
	Dimension      Dimension                   `json:"dimension,omitempty"`
	Metric         Metric                      `json:"metric,omitempty"`
	Order          Order                       `json:"order,omitempty"`
	TimeRange      RangeKind                   `json:"time_range,omitempty"`
	Candidates     []DraftExplanationCandidate `json:"candidates"`
	ToolOperations []string                    `json:"tool_operations,omitempty"`
}

// AssistantReplyContext is the bounded input for a user-facing streamed
// reply. Draft is optional for guidance-only requests.
type AssistantReplyContext struct {
	Query               string                   `json:"query"`
	ResponseModes       []ResponseMode           `json:"response_modes"`
	Draft               *DraftExplanationContext `json:"draft,omitempty"`
	ServiceModeFeedback *ServiceModeFeedback     `json:"service_feedback,omitempty"`
}

// SafeForModel removes operation names from the user-facing reply context;
// operations remain available to the separate stream/audit surfaces.
func (context AssistantReplyContext) SafeForModel() AssistantReplyContext {
	safe := context
	if context.Draft != nil {
		draft := *context.Draft
		draft.ToolOperations = nil
		safe.Draft = &draft
	}
	safe.ServiceModeFeedback = safeServiceModeFeedback(context.ServiceModeFeedback)
	return safe
}

func AssistantReplyQualityWarnings(reply string) []string {
	warnings := make([]string, 0, 3)
	if strings.TrimSpace(reply) == "" {
		warnings = append(warnings, "empty")
	}
	if utf8.RuneCountInString(reply) > MaxReplyRunes {
		warnings = append(warnings, "too_long")
	}
	if strings.TrimSpace(reply) != "" && !containsChinese(reply) {
		warnings = append(warnings, "not_chinese")
	}
	return warnings
}

// IntentExplanationUsageProvider is an optional model boundary used after
// Serving tools have produced a deterministic draft. Implementations return
// bounded user-facing text and provider-reported token usage.
type IntentExplanationUsageProvider interface {
	ExplainDraftUsage(context.Context, string, DraftExplanationContext) (string, int, int, error)
}

// AssistantReplyStreamProvider emits plain-text deltas for guidance or trend
// explanations. The provider returns usage after the upstream stream closes.
type AssistantReplyStreamProvider interface {
	StreamAssistantMessage(context.Context, AssistantReplyContext, func(string) error) (int, int, error)
}
