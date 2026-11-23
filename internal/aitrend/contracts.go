package aitrend

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	ContractVersionV1      = "ai-trend-draft-v1"
	AgentContractVersionV1 = "ai-trend-agent-v1"

	MaxToolCalls            = 20
	MaxToolCandidates       = 100
	MaxBatchCandidates      = 8
	MaxCurves               = 8
	MaxAIAttempts           = 3
	MaxExpressionCandidates = 3
	MaxExplanationRunes     = 100
	MaxReplyRunes           = 1000
)

type Dimension string

const (
	DimensionTechnology Dimension = "technology"
	DimensionProject    Dimension = "project"
	DimensionJobFamily  Dimension = "job_family"
	DimensionLocation   Dimension = "location"
)

type Metric string

const (
	MetricAverageDailyActiveJobs Metric = "avg_daily_active_jobs"
	MetricPeriodEndActiveJobs    Metric = "period_end_active_jobs"
	MetricPeakDailyActiveJobs    Metric = "peak_daily_active_jobs"
	MetricPeriodUniqueJobs       Metric = "period_unique_jobs"
	MetricEnteredJobs            Metric = "entered_jobs"
	MetricModifiedJobs           Metric = "modified_jobs"
	MetricLeftJobs               Metric = "left_jobs"
	MetricReopenedJobs           Metric = "reopened_jobs"
	MetricAbsoluteChange         Metric = "absolute_change"
	MetricRelativeChange         Metric = "relative_change"
)

type SeriesMetric string

const SeriesMetricDailyActiveJobs SeriesMetric = "daily_active_jobs"

type Order string

const (
	OrderAscending  Order = "asc"
	OrderDescending Order = "desc"
)

type ChartType string

const ChartTypeLine ChartType = "line"

type RangeKind string

const (
	RangeWeek    RangeKind = "week"
	RangeMonth   RangeKind = "month"
	RangeQuarter RangeKind = "quarter"
	RangeYear    RangeKind = "year"
	RangeAll     RangeKind = "all"
	RangeCustom  RangeKind = "custom"
)

type Comparison string

const (
	ComparisonEqual          Comparison = "eq"
	ComparisonGreaterThan    Comparison = "gt"
	ComparisonGreaterOrEqual Comparison = "gte"
	ComparisonLessThan       Comparison = "lt"
	ComparisonLessOrEqual    Comparison = "lte"
)

type MatchStatus string

const (
	MatchExact     MatchStatus = "exact"
	MatchAlias     MatchStatus = "alias"
	MatchPossible  MatchStatus = "possible"
	MatchExpansion MatchStatus = "expansion"
)

type DraftStatus string

const (
	DraftComplete DraftStatus = "complete"
	DraftPartial  DraftStatus = "partial"
	DraftStopped  DraftStatus = "stopped"
)

type StreamEventType string

const (
	StreamEventOperation  StreamEventType = "operation"
	StreamEventValidation StreamEventType = "validation"
	StreamEventDraft      StreamEventType = "draft"
	StreamEventAssistant  StreamEventType = "assistant_message"
	StreamEventDone       StreamEventType = "done"
)

type StreamStatus string

const (
	StreamRunning   StreamStatus = "running"
	StreamCompleted StreamStatus = "completed"
	StreamFailed    StreamStatus = "failed"
	StreamSuccess   StreamStatus = "success"
	StreamPartial   StreamStatus = "partial"
	StreamFailure   StreamStatus = "failure"
	StreamStopped   StreamStatus = "user_stopped"
)

type ResponseMode string

const (
	ResponseModeTrend    ResponseMode = "trend"
	ResponseModeGuidance ResponseMode = "guidance"
)

type SafeErrorCode string

const (
	ErrorInvalidJSON          SafeErrorCode = "invalid_json"
	ErrorMissingField         SafeErrorCode = "missing_field"
	ErrorInvalidEnum          SafeErrorCode = "invalid_enum"
	ErrorInvalidCombination   SafeErrorCode = "invalid_combination"
	ErrorLimitExceeded        SafeErrorCode = "limit_exceeded"
	ErrorUnavailableValue     SafeErrorCode = "unavailable_value"
	ErrorBackendFailure       SafeErrorCode = "backend_failure"
	ErrorProviderFailure      SafeErrorCode = "provider_failure"
	ErrorRequestStopped       SafeErrorCode = "request_stopped"
	ErrorConfigurationFailure SafeErrorCode = "configuration_failure"
)

type Intent struct {
	ContractVersion string       `json:"contract_version"`
	SubQueries      []SubQuery   `json:"sub_queries"`
	TimeRange       TimeRange    `json:"time_range"`
	Output          OutputIntent `json:"output"`
}

// AgentResponse is the outer interaction contract. TrendIntent deliberately
// remains the existing ai-trend-draft-v1 contract.
type AgentResponse struct {
	ContractVersion string         `json:"contract_version"`
	ResponseModes   []ResponseMode `json:"response_modes"`
	TrendIntent     *Intent        `json:"trend_intent,omitempty"`
}

type SubQuery struct {
	ID               string        `json:"id"`
	Dimension        Dimension     `json:"dimension"`
	Expressions      []string      `json:"expressions"`
	Exclusions       []string      `json:"exclusions,omitempty"`
	IncludeSecondary *bool         `json:"include_secondary,omitempty"`
	Operator         *Comparison   `json:"operator,omitempty"`
	Ranking          RankingIntent `json:"ranking"`
}

type RankingIntent struct {
	Metric Metric `json:"metric"`
	Order  Order  `json:"order"`
	Limit  int    `json:"limit"`
}

type TimeRange struct {
	Kind      RangeKind `json:"kind"`
	StartDate string    `json:"start_date,omitempty"`
	EndDate   string    `json:"end_date,omitempty"`
}

type OutputIntent struct {
	SeriesMetric SeriesMetric `json:"series_metric"`
	ChartType    ChartType    `json:"chart_type"`
}

type CurveCondition struct {
	Dimension        Dimension   `json:"dimension"`
	Value            string      `json:"value"`
	Operator         *Comparison `json:"operator,omitempty"`
	IncludeSecondary bool        `json:"include_secondary,omitempty"`
}

type DraftCurve struct {
	Condition   CurveCondition `json:"condition"`
	Label       string         `json:"label"`
	Selected    bool           `json:"selected"`
	MatchStatus MatchStatus    `json:"match_status"`
	Scope       string         `json:"scope,omitempty"`
	Reason      string         `json:"reason"`
	SubQueryIDs []string       `json:"sub_query_ids"`
}

type Draft struct {
	ContractVersion string       `json:"contract_version"`
	Status          DraftStatus  `json:"status"`
	Curves          []DraftCurve `json:"curves"`
	Warnings        []string     `json:"warnings,omitempty"`
	Explanation     string       `json:"explanation,omitempty"`
}

type SafeError struct {
	Code    SafeErrorCode `json:"code"`
	Field   string        `json:"field,omitempty"`
	Message string        `json:"message"`
}

type StreamEvent struct {
	Type   StreamEventType `json:"type"`
	Status StreamStatus    `json:"status"`
	Tool   string          `json:"tool,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Delta  string          `json:"delta,omitempty"`
	Draft  *Draft          `json:"draft,omitempty"`
	Error  *SafeError      `json:"error,omitempty"`
}

func DecodeIntent(data []byte) (Intent, error) {
	var intent Intent
	if err := decodeStrict(data, &intent); err != nil {
		return Intent{}, &ContractError{Code: ErrorInvalidJSON, Message: err.Error()}
	}
	if err := ValidateIntent(intent); err != nil {
		return Intent{}, err
	}
	return intent, nil
}

// DecodeAgentResponse accepts the new outer Agent contract and legacy trend
// Intent payloads so existing providers can migrate without changing the
// curve contract in one step.
func DecodeAgentResponse(data []byte) (AgentResponse, error) {
	var version struct {
		ContractVersion string `json:"contract_version"`
	}
	if err := json.Unmarshal(data, &version); err != nil {
		return AgentResponse{}, &ContractError{Code: ErrorInvalidJSON, Message: err.Error()}
	}
	if version.ContractVersion == ContractVersionV1 {
		intent, err := DecodeIntent(data)
		if err != nil {
			return AgentResponse{}, err
		}
		return AgentResponse{ContractVersion: AgentContractVersionV1, ResponseModes: []ResponseMode{ResponseModeTrend}, TrendIntent: &intent}, nil
	}
	var response AgentResponse
	if err := decodeStrict(data, &response); err != nil {
		return AgentResponse{}, &ContractError{Code: ErrorInvalidJSON, Message: err.Error()}
	}
	if err := ValidateAgentResponse(response); err != nil {
		return AgentResponse{}, err
	}
	return response, nil
}

func ValidateAgentResponse(response AgentResponse) error {
	if response.ContractVersion != AgentContractVersionV1 {
		return contractError(ErrorInvalidEnum, "contract_version", "unsupported Agent contract version")
	}
	if len(response.ResponseModes) == 0 || len(response.ResponseModes) > 2 {
		return contractError(ErrorMissingField, "response_modes", "one or two response modes are required")
	}
	seen := make(map[ResponseMode]struct{}, len(response.ResponseModes))
	hasTrend := false
	for _, mode := range response.ResponseModes {
		if mode != ResponseModeTrend && mode != ResponseModeGuidance {
			return contractError(ErrorInvalidEnum, "response_modes", "unsupported response mode")
		}
		if _, exists := seen[mode]; exists {
			return contractError(ErrorInvalidCombination, "response_modes", "duplicate response mode")
		}
		seen[mode] = struct{}{}
		if mode == ResponseModeTrend {
			hasTrend = true
		}
	}
	if hasTrend {
		if response.TrendIntent == nil {
			return contractError(ErrorMissingField, "trend_intent", "trend mode requires a trend Intent")
		}
		if err := ValidateIntent(*response.TrendIntent); err != nil {
			return err
		}
	} else if response.TrendIntent != nil {
		return contractError(ErrorInvalidCombination, "trend_intent", "guidance-only response cannot carry trend Intent")
	}
	return nil
}

func DecodeStreamEvent(data []byte) (StreamEvent, error) {
	var event StreamEvent
	if err := decodeStrict(data, &event); err != nil {
		return StreamEvent{}, &ContractError{Code: ErrorInvalidJSON, Message: err.Error()}
	}
	if err := ValidateStreamEvent(event); err != nil {
		return StreamEvent{}, err
	}
	return event, nil
}

func ValidateIntent(intent Intent) error {
	if intent.ContractVersion != ContractVersionV1 {
		return contractError(ErrorInvalidEnum, "contract_version", "unsupported contract version")
	}
	if len(intent.SubQueries) == 0 {
		return contractError(ErrorMissingField, "sub_queries", "at least one sub-query is required")
	}
	if err := validateTimeRange(intent.TimeRange); err != nil {
		return err
	}
	if intent.Output.SeriesMetric != SeriesMetricDailyActiveJobs || intent.Output.ChartType != ChartTypeLine {
		return contractError(ErrorInvalidCombination, "output", "only daily active-job line curves are supported")
	}
	for _, query := range intent.SubQueries {
		if err := validateSubQuery(query); err != nil {
			return err
		}
	}
	return nil
}

func ValidateDraft(draft Draft) error {
	if draft.ContractVersion != ContractVersionV1 {
		return contractError(ErrorInvalidEnum, "contract_version", "unsupported contract version")
	}
	if draft.Status != DraftComplete && draft.Status != DraftPartial && draft.Status != DraftStopped {
		return contractError(ErrorInvalidEnum, "status", "unsupported draft status")
	}
	if len(draft.Curves) > MaxCurves {
		return contractError(ErrorLimitExceeded, "curves", "curve limit exceeded")
	}
	if utf8.RuneCountInString(draft.Explanation) > MaxExplanationRunes {
		return contractError(ErrorLimitExceeded, "explanation", "draft explanation is too long")
	}
	for _, curve := range draft.Curves {
		if err := validateCurve(curve); err != nil {
			return err
		}
	}
	return nil
}

func ValidateStreamEvent(event StreamEvent) error {
	if event.Type != StreamEventOperation && event.Type != StreamEventValidation && event.Type != StreamEventDraft && event.Type != StreamEventAssistant && event.Type != StreamEventDone {
		return contractError(ErrorInvalidEnum, "type", "unsupported stream event type")
	}
	if event.Status == "" {
		return contractError(ErrorMissingField, "status", "stream event status is required")
	}
	if event.Type == StreamEventOperation && strings.TrimSpace(event.Tool) == "" {
		return contractError(ErrorMissingField, "tool", "operation tool is required")
	}
	if event.Type == StreamEventAssistant && event.Status == StreamRunning && event.Delta == "" {
		return contractError(ErrorMissingField, "delta", "running assistant message requires delta")
	}
	if event.Type == StreamEventAssistant && utf8.RuneCountInString(event.Delta) > MaxReplyRunes {
		return contractError(ErrorLimitExceeded, "delta", "assistant message delta is too long")
	}
	if event.Type != StreamEventAssistant && event.Delta != "" {
		return contractError(ErrorInvalidCombination, "delta", "delta is only valid for assistant messages")
	}
	if event.Type == StreamEventDraft && event.Draft == nil {
		return contractError(ErrorMissingField, "draft", "draft event requires a draft")
	}
	if event.Type == StreamEventDraft && event.Draft != nil {
		return ValidateDraft(*event.Draft)
	}
	if event.Type == StreamEventDone && event.Status != StreamSuccess && event.Status != StreamPartial && event.Status != StreamFailure && event.Status != StreamStopped {
		return contractError(ErrorInvalidEnum, "status", "invalid terminal status")
	}
	return nil
}

type ContractError struct {
	Code    SafeErrorCode
	Field   string
	Message string
}

func (err *ContractError) Error() string {
	if err == nil {
		return ""
	}
	if err.Field == "" {
		return err.Message
	}
	return fmt.Sprintf("%s: %s", err.Field, err.Message)
}

func contractError(code SafeErrorCode, field, message string) error {
	return &ContractError{Code: code, Field: field, Message: message}
}

func decodeStrict(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func validateTimeRange(timeRange TimeRange) error {
	switch timeRange.Kind {
	case RangeWeek, RangeMonth, RangeQuarter, RangeYear, RangeAll:
		if timeRange.StartDate != "" || timeRange.EndDate != "" {
			return contractError(ErrorInvalidCombination, "time_range", "preset ranges cannot include custom dates")
		}
	case RangeCustom:
		if timeRange.StartDate == "" || timeRange.EndDate == "" || len(timeRange.StartDate) != len("2006-01-02") || len(timeRange.EndDate) != len("2006-01-02") || timeRange.StartDate > timeRange.EndDate {
			return contractError(ErrorInvalidEnum, "time_range", "invalid custom range")
		}
		if _, err := time.Parse("2006-01-02", timeRange.StartDate); err != nil {
			return contractError(ErrorInvalidEnum, "time_range.start_date", "invalid custom range")
		}
		if _, err := time.Parse("2006-01-02", timeRange.EndDate); err != nil {
			return contractError(ErrorInvalidEnum, "time_range.end_date", "invalid custom range")
		}
	default:
		return contractError(ErrorInvalidEnum, "time_range.kind", "unsupported time range")
	}
	return nil
}

func validateSubQuery(query SubQuery) error {
	if strings.TrimSpace(query.ID) == "" || len(query.Expressions) == 0 {
		return contractError(ErrorMissingField, "sub_queries", "sub-query id and expressions are required")
	}
	if query.Dimension != DimensionTechnology && query.Dimension != DimensionProject && query.Dimension != DimensionJobFamily && query.Dimension != DimensionLocation {
		return contractError(ErrorInvalidEnum, "dimension", "unsupported dimension")
	}
	if query.Dimension != DimensionJobFamily && query.IncludeSecondary != nil {
		return contractError(ErrorInvalidCombination, "include_secondary", "secondary-family option is job-family only")
	}
	if query.Dimension != DimensionJobFamily && query.Operator != nil {
		return contractError(ErrorInvalidCombination, "operator", "operator is not supported for this dimension")
	}
	if query.Ranking.Limit < 1 || query.Ranking.Limit > MaxCurves {
		return contractError(ErrorLimitExceeded, "ranking.limit", "ranking limit must be between one and eight")
	}
	if query.Ranking.Order != OrderAscending && query.Ranking.Order != OrderDescending {
		return contractError(ErrorInvalidEnum, "ranking.order", "unsupported ranking order")
	}
	if !validMetric(query.Ranking.Metric) {
		return contractError(ErrorInvalidEnum, "ranking.metric", "unsupported ranking metric")
	}
	return nil
}

func validateCurve(curve DraftCurve) error {
	if strings.TrimSpace(curve.Label) == "" || strings.TrimSpace(curve.Reason) == "" || len(curve.SubQueryIDs) == 0 {
		return contractError(ErrorMissingField, "curves", "curve label, reason, and sub-query ids are required")
	}
	if curve.Condition.Dimension != DimensionTechnology && curve.Condition.Dimension != DimensionProject && curve.Condition.Dimension != DimensionJobFamily && curve.Condition.Dimension != DimensionLocation {
		return contractError(ErrorInvalidEnum, "curves.condition.dimension", "unsupported curve dimension")
	}
	if curve.Condition.Dimension != DimensionJobFamily && curve.Condition.Operator != nil {
		return contractError(ErrorInvalidCombination, "curves.condition.operator", "operator is not supported for this dimension")
	}
	if curve.MatchStatus != MatchExact && curve.MatchStatus != MatchAlias && curve.MatchStatus != MatchPossible && curve.MatchStatus != MatchExpansion {
		return contractError(ErrorInvalidEnum, "curves.match_status", "unsupported match status")
	}
	return nil
}

func validMetric(metric Metric) bool {
	switch metric {
	case MetricAverageDailyActiveJobs, MetricPeriodEndActiveJobs, MetricPeakDailyActiveJobs, MetricPeriodUniqueJobs, MetricEnteredJobs, MetricModifiedJobs, MetricLeftJobs, MetricReopenedJobs, MetricAbsoluteChange, MetricRelativeChange:
		return true
	default:
		return false
	}
}
