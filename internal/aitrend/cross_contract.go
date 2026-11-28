package aitrend

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// CrossConditionGroup contains values from exactly one dimension. Groups in
// Conditions are ANDed; each group in Exclusions is applied as AND NOT.
// Keeping the dimension on the group makes cross-dimension OR unrepresentable.
type CrossConditionGroup struct {
	Dimension  Dimension     `json:"dimension"`
	Operator   CrossOperator `json:"operator"`
	Values     []string      `json:"values,omitempty"`
	Categories []string      `json:"categories,omitempty"`
}

type CrossOperator string

const (
	CrossAny CrossOperator = "any"
	CrossAll CrossOperator = "all"

	MaxCrossSubQueries      = 16
	MaxCrossConditionGroups = 16
	MaxCrossExplicitValues  = 100
	MaxCrossValueRunes      = 128
	MaxCrossPageSize        = 100
)

// CrossTime describes the requested interpretation, not a historical
// database query. Both values compute current jobs; historical_reference tells
// the renderer to explain that the original historical request is unavailable.
type CrossTime string

const (
	CrossTimeCurrent             CrossTime = "current"
	CrossTimeHistoricalReference CrossTime = "historical_reference"
)

// CrossStatisticsQuery is separate from the existing single-curve Intent.
// RankingLimit nil means the default eight results; zero means all results.
// IncludeSecondary nil defaults to including both primary and secondary family.
type CrossStatisticsQuery struct {
	ID               string                `json:"id"`
	Conditions       []CrossConditionGroup `json:"conditions,omitempty"`
	Exclusions       []CrossConditionGroup `json:"exclusions,omitempty"`
	GroupBy          Dimension             `json:"group_by,omitempty"`
	IncludeSecondary *bool                 `json:"include_secondary,omitempty"`
	Time             CrossTime             `json:"time"`
	RankingLimit     *int                  `json:"ranking_limit,omitempty"`
	PageSize         int                   `json:"page_size,omitempty"`
}

type CrossStatisticsPlan struct {
	SubQueries []CrossStatisticsQuery `json:"sub_queries"`
}

// DecodeCrossStatisticsPlan refuses unknown fields: in particular a model
// cannot send an OR over dimension-bearing groups and have it silently ignored.
func DecodeCrossStatisticsPlan(data []byte) (CrossStatisticsPlan, error) {
	var plan CrossStatisticsPlan
	if err := decodeStrict(data, &plan); err != nil {
		return CrossStatisticsPlan{}, contractError(ErrorInvalidJSON, "cross_statistics", "invalid cross-statistics contract")
	}
	if err := ValidateCrossStatisticsPlan(plan); err != nil {
		return CrossStatisticsPlan{}, err
	}
	return plan, nil
}

func ValidateCrossStatisticsPlan(plan CrossStatisticsPlan) error {
	if len(plan.SubQueries) == 0 || len(plan.SubQueries) > MaxCrossSubQueries {
		return contractError(ErrorLimitExceeded, "sub_queries", "cross statistics requires between one and sixteen sub-queries")
	}
	seen := make(map[string]struct{}, len(plan.SubQueries))
	for _, query := range plan.SubQueries {
		if err := ValidateCrossStatisticsQuery(query); err != nil {
			return err
		}
		if _, ok := seen[query.ID]; ok {
			return contractError(ErrorInvalidValue, "id", "sub-query ids must be unique")
		}
		seen[query.ID] = struct{}{}
	}
	return nil
}

func ValidateCrossStatisticsQuery(query CrossStatisticsQuery) error {
	if !validCrossSubQueryID(query.ID) {
		return contractError(ErrorInvalidValue, "id", "sub-query id is required and must be bounded")
	}
	if query.Time != CrossTimeCurrent && query.Time != CrossTimeHistoricalReference {
		return contractError(ErrorInvalidEnum, "time", "cross statistics only supports current results or a current historical reference")
	}
	if query.GroupBy != "" && !isCrossDimension(query.GroupBy) {
		return contractError(ErrorInvalidEnum, "group_by", "unsupported grouping dimension")
	}
	if len(query.Conditions)+len(query.Exclusions) > MaxCrossConditionGroups {
		return contractError(ErrorLimitExceeded, "conditions", "too many condition groups")
	}
	if len(query.Conditions) == 0 && len(query.Exclusions) == 0 && query.GroupBy == "" {
		return contractError(ErrorMissingField, "conditions", "a confirmed condition or grouping is required")
	}
	for _, groups := range [][]CrossConditionGroup{query.Conditions, query.Exclusions} {
		for _, group := range groups {
			if err := validateCrossConditionGroup(group); err != nil {
				return err
			}
		}
	}
	if query.RankingLimit != nil && (*query.RankingLimit < 0 || *query.RankingLimit > 1_000_000) {
		return contractError(ErrorLimitExceeded, "ranking_limit", "ranking limit must be zero for all or a bounded positive number")
	}
	if query.RankingLimit != nil && query.GroupBy == "" {
		return contractError(ErrorInvalidCombination, "ranking_limit", "a ranking limit requires a grouping dimension")
	}
	if query.PageSize < 0 || query.PageSize > MaxCrossPageSize {
		return contractError(ErrorLimitExceeded, "page_size", "page size exceeds the bounded result size")
	}
	return nil
}

func validCrossSubQueryID(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func validateCrossConditionGroup(group CrossConditionGroup) error {
	if !isCrossDimension(group.Dimension) {
		return contractError(ErrorInvalidEnum, "dimension", "unsupported condition dimension")
	}
	if group.Operator != CrossAny && group.Operator != CrossAll {
		return contractError(ErrorInvalidEnum, "operator", "condition operator must be any or all")
	}
	if len(group.Values)+len(group.Categories) == 0 {
		return contractError(ErrorMissingField, "values", "an empty group cannot be treated as an unrestricted filter")
	}
	if len(group.Values)+len(group.Categories) > MaxCrossExplicitValues {
		return contractError(ErrorLimitExceeded, "values", "too many explicit condition values")
	}
	if len(group.Categories) > 0 && (group.Dimension != DimensionTechnology || group.Operator != CrossAny) {
		return contractError(ErrorInvalidCombination, "categories", "technology categories must be expanded as an any group")
	}
	for _, values := range [][]string{group.Values, group.Categories} {
		seen := make(map[string]struct{}, len(values))
		for _, value := range values {
			if value == "" || strings.TrimSpace(value) != value || utf8.RuneCountInString(value) > MaxCrossValueRunes {
				return contractError(ErrorInvalidValue, "values", "condition values must be non-empty and bounded")
			}
			if _, ok := seen[value]; ok {
				return contractError(ErrorInvalidValue, "values", fmt.Sprintf("duplicate condition value in %s", group.Dimension))
			}
			seen[value] = struct{}{}
		}
	}
	return nil
}

func isCrossDimension(dimension Dimension) bool {
	switch dimension {
	case DimensionTechnology, DimensionProject, DimensionJobFamily, DimensionLocation:
		return true
	default:
		return false
	}
}
