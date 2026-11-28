package aitrend

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"mrolescope/internal/jdserving"
)

type crossStatisticsRecord struct {
	query  CrossStatisticsQuery
	result jdserving.CrossStatisticsResult
}

type crossRequiredContextKey struct{}

func withCrossRequired(ctx context.Context, required bool) context.Context {
	return context.WithValue(ctx, crossRequiredContextKey{}, required)
}

func crossRequiredFromContext(ctx context.Context) bool {
	required, _ := ctx.Value(crossRequiredContextKey{}).(bool)
	return required
}

// A session retains the full aggregate snapshot for deterministic delivery;
// the model receives only a bounded summary of the same facts.
type crossStatisticsSession struct {
	mu      sync.Mutex
	scope   crossScopeEvidence
	plan    string
	records []crossStatisticsRecord
}

func newCrossStatisticsSession(query string, snapshot *ServingSnapshot) *crossStatisticsSession {
	return &crossStatisticsSession{scope: buildCrossScopeEvidence(query, snapshot)}
}

type crossModelResult struct {
	SubQueryID  string                           `json:"sub_query_id"`
	Time        CrossTime                        `json:"time"`
	Conditions  []CrossConditionGroup            `json:"conditions,omitempty"`
	Exclusions  []CrossConditionGroup            `json:"exclusions,omitempty"`
	TotalJobs   int                              `json:"total_jobs"`
	GroupBy     Dimension                        `json:"group_by,omitempty"`
	Groups      []jdserving.CrossStatisticsGroup `json:"top_groups,omitempty"`
	TotalGroups int                              `json:"total_groups"`
	TopComplete bool                             `json:"top_groups_complete"`
	TopCount    *int                             `json:"top_count,omitempty"`
	TopTies     int                              `json:"top_ties,omitempty"`
}

func (service ServingToolService) queryCrossStatistics(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	if service.Cross == nil || service.CrossRun == nil || service.Snapshot == nil {
		return nil, contractError(ErrorConfigurationFailure, "statistics", "交叉统计暂不可用")
	}
	plan, err := DecodeCrossStatisticsPlan(params)
	if err != nil {
		return nil, err
	}
	if err := validateCrossScopePlan(plan, service.CrossRun.scope); err != nil {
		return nil, err
	}
	plan = normalizeCrossRanking(plan, service.CrossRun.scope)
	canonical, err := json.Marshal(plan)
	if err != nil {
		return nil, contractError(ErrorInvalidJSON, "statistics", "交叉统计参数无效")
	}
	service.CrossRun.mu.Lock()
	defer service.CrossRun.mu.Unlock()
	if service.CrossRun.plan != "" && service.CrossRun.plan != string(canonical) {
		return nil, contractError(ErrorInvalidCombination, "statistics", "已确认的统计范围不能在同一次请求中更换")
	}
	if service.CrossRun.plan == "" {
		queries := make([]jdserving.CrossStatisticsQuery, 0, len(plan.SubQueries))
		for _, query := range plan.SubQueries {
			conditions, categories, err := crossServingConditions(query.Conditions, service.Snapshot)
			if err != nil {
				return nil, err
			}
			exclusions, excludedCategories, err := crossServingConditions(query.Exclusions, service.Snapshot)
			if err != nil {
				return nil, err
			}
			for category, values := range excludedCategories {
				categories[category] = values
			}
			queries = append(queries, jdserving.CrossStatisticsQuery{
				Conditions: conditions, Exclusions: exclusions, GroupBy: jdserving.TrendDimension(query.GroupBy),
				PrimaryFamilyOnly: query.IncludeSecondary != nil && !*query.IncludeSecondary,
				CategorySnapshot:  categories,
			})
		}
		results, err := service.Cross.ReadCrossStatisticsBatch(ctx, queries)
		if err != nil {
			if errors.Is(err, jdserving.ErrInvalidCrossStatisticsQuery) {
				return nil, contractError(ErrorUnavailableValue, "scope", "正式统计范围已变化或无法确认")
			}
			return nil, contractError(ErrorBackendFailure, "statistics", "当前交叉统计暂时无法计算")
		}
		if len(results) != len(plan.SubQueries) {
			return nil, contractError(ErrorBackendFailure, "statistics", "当前交叉统计结果不完整")
		}
		records := make([]crossStatisticsRecord, 0, len(results))
		for index, result := range results {
			if err := service.validateCrossResult(plan.SubQueries[index], result); err != nil {
				return nil, err
			}
			records = append(records, crossStatisticsRecord{query: plan.SubQueries[index], result: result})
		}
		service.CrossRun.records = records
		service.CrossRun.plan = string(canonical)
	}
	return crossModelPayload(service.CrossRun.records)
}

func (service ServingToolService) validateCrossResult(query CrossStatisticsQuery, result jdserving.CrossStatisticsResult) error {
	if result.Total < 0 || query.GroupBy == "" && len(result.Groups) > 0 {
		return contractError(ErrorBackendFailure, "statistics", "当前交叉统计结果无效")
	}
	formal := make(map[string]string)
	for _, option := range service.optionsForDimension(service.Snapshot.options, query.GroupBy) {
		formal[option.Value] = option.Label
	}
	seen := make(map[string]struct{}, len(result.Groups))
	previousCount := result.Total + 1
	for _, group := range result.Groups {
		label, ok := formal[group.Value]
		if !ok || label != group.Label || group.Count < 0 || group.Count > result.Total || group.Count > previousCount {
			return contractError(ErrorUnavailableValue, "scope", "统计分组不属于已确认的正式目录")
		}
		if _, duplicate := seen[group.Value]; duplicate {
			return contractError(ErrorBackendFailure, "statistics", "当前交叉统计结果无效")
		}
		seen[group.Value] = struct{}{}
		previousCount = group.Count
	}
	return nil
}

func crossServingConditions(groups []CrossConditionGroup, snapshot *ServingSnapshot) ([]jdserving.CrossCondition, map[string][]string, error) {
	converted := make([]jdserving.CrossCondition, 0, len(groups))
	categories := make(map[string][]string)
	for _, group := range groups {
		converted = append(converted, jdserving.CrossCondition{
			Dimension: jdserving.TrendDimension(group.Dimension), Operator: string(group.Operator),
			Values: append([]string(nil), group.Values...), Categories: append([]string(nil), group.Categories...),
		})
		for _, category := range group.Categories {
			options, formal, ok := snapshot.categoryValues(category)
			if !ok || formal != category || len(options) == 0 {
				return nil, nil, contractError(ErrorInvalidValue, "category", "无法确认正式类别及其候选")
			}
			values := make([]string, 0, len(options))
			for _, option := range options {
				values = append(values, option.Value)
			}
			categories[category] = values
		}
	}
	return converted, categories, nil
}

func crossModelPayload(records []crossStatisticsRecord) (json.RawMessage, error) {
	results := make([]crossModelResult, 0, len(records))
	for _, record := range records {
		limit := len(record.result.Groups)
		if limit > DefaultRankingLimit {
			limit = DefaultRankingLimit
		}
		modelResult := crossModelResult{
			SubQueryID: record.query.ID, Time: record.query.Time, TotalJobs: record.result.Total,
			Conditions: record.query.Conditions, Exclusions: record.query.Exclusions,
			GroupBy: record.query.GroupBy, Groups: append([]jdserving.CrossStatisticsGroup(nil), record.result.Groups[:limit]...),
			TotalGroups: len(record.result.Groups), TopComplete: limit == len(record.result.Groups),
		}
		if len(record.result.Groups) > 0 {
			count := record.result.Groups[0].Count
			modelResult.TopCount = &count
			for _, group := range record.result.Groups {
				if group.Count != count {
					break
				}
				modelResult.TopTies++
			}
		}
		results = append(results, modelResult)
	}
	return marshalBounded(struct {
		Results []crossModelResult `json:"results"`
	}{Results: results}, MaxToolOutputBytes/1024)
}
