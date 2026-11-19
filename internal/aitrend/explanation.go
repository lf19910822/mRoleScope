package aitrend

import "context"

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

// IntentExplanationUsageProvider is an optional model boundary used after
// Serving tools have produced a deterministic draft. Implementations return
// bounded user-facing text and provider-reported token usage.
type IntentExplanationUsageProvider interface {
	ExplainDraftUsage(context.Context, string, DraftExplanationContext) (string, int, int, error)
}
