package aitrend

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"mrolescope/internal/jdserving"
)

// FilterOptionsReader is the only vocabulary boundary exposed to the trend
// assistant. It returns formal, display-safe values and never raw JD text.
type FilterOptionsReader interface {
	ListFilterOptions(context.Context) (jdserving.FilterOptionsResponse, error)
}

type TrendEventReader interface {
	ReadTrendEventStats(context.Context, jdserving.TrendEventStatsQuery) (jdserving.TrendEventStatsResponse, error)
}

// ServingToolService adapts the existing Serving seams into the small,
// registered tool vocabulary available to the model.
type ServingToolService struct {
	Options  FilterOptionsReader
	Summary  jdserving.TrendSummaryReader
	Events   TrendEventReader
	Resolver Resolver
}

func (service ServingToolService) Registry() (*ToolRegistry, error) {
	return NewToolRegistry([]ToolDefinition{
		{Name: ToolListTechnologyCategories, Handler: service.listTechnologyCategories},
		{Name: ToolListCategoryValues, Handler: service.listCategoryValues},
		{Name: ToolListAvailableValues, Handler: service.listAvailableValues},
		{Name: ToolRankValues, Handler: service.rankValues},
		{Name: ToolReadTrendSummary, Handler: service.readTrendSummary},
		{Name: ToolReadEventSummary, Handler: service.readEventSummary},
		{Name: ToolValidateCurveDraft, Handler: service.validateCurveDraft},
		{Name: ToolBatchTrendSummary, Handler: service.batchTrendSummary},
		{Name: ToolBatchEventSummary, Handler: service.batchEventSummary},
		{Name: ToolBatchValidateDraft, Handler: service.batchValidateDraft},
	})
}

type valueRequest struct {
	Dimension Dimension `json:"dimension"`
}

type categoryRequest struct {
	Category string `json:"category"`
}

type rankRequest struct {
	Dimension  Dimension `json:"dimension"`
	Expression string    `json:"expression"`
	SubQueryID string    `json:"sub_query_id"`
}

type trendToolRequest struct {
	Dimension        Dimension   `json:"dimension"`
	Value            string      `json:"value"`
	Range            RangeKind   `json:"range"`
	StartDate        string      `json:"start_date,omitempty"`
	EndDate          string      `json:"end_date,omitempty"`
	IncludeSecondary bool        `json:"include_secondary,omitempty"`
	Operator         *Comparison `json:"operator,omitempty"`
}

type summaryToolResponse struct {
	Curve                  jdserving.TrendCurve             `json:"curve"`
	AverageDailyActiveJobs *float64                         `json:"average_daily_active_jobs,omitempty"`
	PeriodEndActiveJobs    *int                             `json:"period_end_active_jobs,omitempty"`
	PeakDailyActiveJobs    *jdserving.TrendSummaryPeak      `json:"peak_daily_active_jobs,omitempty"`
	PeriodUniqueJobs       *int                             `json:"period_unique_jobs,omitempty"`
	ValidObservationDays   int                              `json:"valid_observation_days"`
	MissingObservationDays int                              `json:"missing_observation_days"`
	Events                 map[jdserving.TrendEventType]int `json:"events,omitempty"`
}

func (service ServingToolService) listTechnologyCategories(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	if len(params) != 0 && string(params) != "null" && string(params) != "{}" {
		return nil, contractError(ErrorInvalidCombination, "params", "this tool does not accept parameters")
	}
	options, err := service.readOptions(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]FilterOption{}
	for _, option := range options.TechnologyKeywords {
		seen[option.Category.Value] = FilterOption{Dimension: DimensionTechnology, Value: option.Category.Value, Label: option.Category.Label}
	}
	result := make([]FilterOption, 0, len(seen))
	for _, option := range seen {
		result = append(result, option)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Value < result[j].Value })
	return json.Marshal(result)
}

type FilterOption struct {
	Dimension Dimension `json:"dimension"`
	Value     string    `json:"value"`
	Label     string    `json:"label"`
}

func (service ServingToolService) listCategoryValues(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var request categoryRequest
	if err := decodeToolParams(params, &request); err != nil {
		return nil, err
	}
	options, err := service.readOptions(ctx)
	if err != nil {
		return nil, err
	}
	values := make([]FilterOption, 0)
	for _, option := range options.TechnologyKeywords {
		if option.Category.Value == request.Category {
			values = append(values, FilterOption{Dimension: DimensionTechnology, Value: option.Value, Label: option.Label})
		}
	}
	return marshalBounded(values, MaxToolCandidates)
}

func (service ServingToolService) listAvailableValues(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var request valueRequest
	if err := decodeToolParams(params, &request); err != nil {
		return nil, err
	}
	options, err := service.readOptions(ctx)
	if err != nil {
		return nil, err
	}
	values := service.optionsForDimension(options, request.Dimension)
	return marshalBounded(values, MaxToolCandidates)
}

func (service ServingToolService) rankValues(_ context.Context, params json.RawMessage) (json.RawMessage, error) {
	var request rankRequest
	if err := decodeToolParams(params, &request); err != nil {
		return nil, err
	}
	return json.Marshal(service.Resolver.Resolve(request.Dimension, request.Expression, request.SubQueryID))
}

func (service ServingToolService) readTrendSummary(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var request trendToolRequest
	if err := decodeToolParams(params, &request); err != nil {
		return nil, err
	}
	if service.Summary == nil {
		return nil, fmt.Errorf("trend summary reader is unavailable")
	}
	query, err := request.toTrendQuery()
	if err != nil {
		return nil, err
	}
	summary, err := service.Summary.ReadTrendSummary(ctx, query)
	if err != nil {
		return nil, err
	}
	return json.Marshal(summaryToolResponse{Curve: summary.Curve, AverageDailyActiveJobs: summary.AverageDailyActiveJobs, PeriodEndActiveJobs: summary.PeriodEndActiveJobs, PeakDailyActiveJobs: summary.PeakDailyActiveJobs, PeriodUniqueJobs: summary.PeriodUniqueJobs, ValidObservationDays: summary.ValidObservationDays, MissingObservationDays: summary.MissingObservationDays, Events: summary.Events})
}

func (service ServingToolService) readEventSummary(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var request trendToolRequest
	if err := decodeToolParams(params, &request); err != nil {
		return nil, err
	}
	if service.Events == nil {
		return nil, fmt.Errorf("trend event reader is unavailable")
	}
	query, err := request.toEventQuery()
	if err != nil {
		return nil, err
	}
	response, err := service.Events.ReadTrendEventStats(ctx, query)
	if err != nil {
		return nil, err
	}
	return json.Marshal(response.Rows)
}

func (service ServingToolService) validateCurveDraft(_ context.Context, params json.RawMessage) (json.RawMessage, error) {
	var draft Draft
	if err := decodeToolParams(params, &draft); err != nil {
		return nil, err
	}
	if err := ValidateDraft(draft); err != nil {
		return nil, err
	}
	for _, curve := range draft.Curves {
		if !service.hasFormalValue(curve.Condition.Dimension, curve.Condition.Value) {
			return nil, contractError(ErrorUnavailableValue, "curves", "draft contains an unavailable formal value")
		}
	}
	return json.Marshal(draft)
}

func (service ServingToolService) batchTrendSummary(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var requests []trendToolRequest
	if err := decodeToolParams(params, &requests); err != nil {
		return nil, err
	}
	if err := ValidateBatchSize(len(requests)); err != nil {
		return nil, err
	}
	result := make([]json.RawMessage, 0, len(requests))
	for _, request := range requests {
		payload, err := service.readTrendSummary(ctx, mustMarshal(request))
		if err != nil {
			return nil, err
		}
		result = append(result, payload)
	}
	return json.Marshal(result)
}

func (service ServingToolService) batchEventSummary(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var requests []trendToolRequest
	if err := decodeToolParams(params, &requests); err != nil {
		return nil, err
	}
	if err := ValidateBatchSize(len(requests)); err != nil {
		return nil, err
	}
	result := make([]json.RawMessage, 0, len(requests))
	for _, request := range requests {
		payload, err := service.readEventSummary(ctx, mustMarshal(request))
		if err != nil {
			return nil, err
		}
		result = append(result, payload)
	}
	return json.Marshal(result)
}

func (service ServingToolService) batchValidateDraft(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var drafts []Draft
	if err := decodeToolParams(params, &drafts); err != nil {
		return nil, err
	}
	if err := ValidateBatchSize(len(drafts)); err != nil {
		return nil, err
	}
	result := make([]json.RawMessage, 0, len(drafts))
	for _, draft := range drafts {
		payload, err := service.validateCurveDraft(ctx, mustMarshal(draft))
		if err != nil {
			return nil, err
		}
		result = append(result, payload)
	}
	return json.Marshal(result)
}

func (service ServingToolService) readOptions(ctx context.Context) (jdserving.FilterOptionsResponse, error) {
	if service.Options == nil {
		return jdserving.FilterOptionsResponse{}, fmt.Errorf("filter options reader is unavailable")
	}
	return service.Options.ListFilterOptions(ctx)
}

func (service ServingToolService) optionsForDimension(options jdserving.FilterOptionsResponse, dimension Dimension) []FilterOption {
	result := make([]FilterOption, 0)
	appendOptions := func(values []jdserving.FilterOption) {
		for _, option := range values {
			result = append(result, FilterOption{Dimension: dimension, Value: option.Value, Label: option.Label})
		}
	}
	switch dimension {
	case DimensionTechnology:
		for _, option := range options.TechnologyKeywords {
			result = append(result, FilterOption{Dimension: dimension, Value: option.Value, Label: option.Label})
		}
	case DimensionProject:
		appendOptions(options.Projects)
	case DimensionJobFamily:
		appendOptions(options.JobFamilies)
	case DimensionLocation:
		appendOptions(options.Locations)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Label != result[j].Label {
			return result[i].Label < result[j].Label
		}
		return result[i].Value < result[j].Value
	})
	return result
}

func (service ServingToolService) hasFormalValue(dimension Dimension, value string) bool {
	for _, candidate := range service.Resolver.Values[dimension] {
		if strings.EqualFold(candidate.Value, value) {
			return true
		}
	}
	return false
}

func decodeToolParams(data json.RawMessage, destination any) error {
	if len(data) == 0 {
		return contractError(ErrorMissingField, "params", "tool parameters are required")
	}
	if err := decodeStrict(data, destination); err != nil {
		return contractError(ErrorInvalidJSON, "params", err.Error())
	}
	return nil
}

func marshalBounded(value any, limit int) (json.RawMessage, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(data) > limit*1024 {
		return nil, contractError(ErrorLimitExceeded, "result", "tool result exceeds bounded output")
	}
	return data, nil
}

func mustMarshal(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

func (request trendToolRequest) toTrendQuery() (jdserving.TrendQuery, error) {
	rangeValue, start, end, err := request.rangeValues()
	if err != nil {
		return jdserving.TrendQuery{}, err
	}
	return jdserving.TrendQuery{Condition: jdserving.TrendCondition{Dimension: jdserving.TrendDimension(request.Dimension), Value: strings.TrimSpace(request.Value), IncludeSecondary: request.IncludeSecondary, Operator: jdserving.TrendComparison(pointerValue(request.Operator))}, Range: jdserving.TrendRange(rangeValue), StartDate: start, EndDate: end}, nil
}

func (request trendToolRequest) toEventQuery() (jdserving.TrendEventStatsQuery, error) {
	query, err := request.toTrendQuery()
	if err != nil {
		return jdserving.TrendEventStatsQuery{}, err
	}
	return jdserving.TrendEventStatsQuery{Condition: query.Condition, Range: query.Range, StartDate: query.StartDate, EndDate: query.EndDate}, nil
}

func pointerValue(value *Comparison) Comparison {
	if value == nil {
		return ""
	}
	return *value
}

func (request trendToolRequest) rangeValues() (RangeKind, time.Time, time.Time, error) {
	if request.Range == "" {
		return "", time.Time{}, time.Time{}, contractError(ErrorMissingField, "range", "range is required")
	}
	if request.Range != RangeWeek && request.Range != RangeMonth && request.Range != RangeQuarter && request.Range != RangeYear && request.Range != RangeAll && request.Range != RangeCustom {
		return "", time.Time{}, time.Time{}, contractError(ErrorInvalidEnum, "range", "unsupported range")
	}
	if request.Range != RangeCustom {
		if request.StartDate != "" || request.EndDate != "" {
			return "", time.Time{}, time.Time{}, contractError(ErrorInvalidCombination, "range", "preset range cannot include dates")
		}
		return request.Range, time.Time{}, time.Time{}, nil
	}
	location, _ := time.LoadLocation("Asia/Shanghai")
	start, startErr := time.ParseInLocation("2006-01-02", request.StartDate, location)
	end, endErr := time.ParseInLocation("2006-01-02", request.EndDate, location)
	if startErr != nil || endErr != nil || start.After(end) {
		return "", time.Time{}, time.Time{}, contractError(ErrorInvalidEnum, "range", "invalid custom range")
	}
	return request.Range, start, end, nil
}
