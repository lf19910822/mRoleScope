package jdserving

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"

	"mrolescope/internal/jdanalysis"
)

// CrossCondition is one same-dimension group. The groups in Conditions are
// ANDed; each group in Exclusions is applied as AND NOT. An "any" group may
// name technology categories; expansion uses the same snapshot as the count.
type CrossCondition struct {
	Dimension  TrendDimension `json:"dimension"`
	Operator   string         `json:"operator"`
	Values     []string       `json:"values,omitempty"`
	Categories []string       `json:"categories,omitempty"`
}

type CrossStatisticsQuery struct {
	Conditions        []CrossCondition
	Exclusions        []CrossCondition
	GroupBy           TrendDimension
	PrimaryFamilyOnly bool
	// CategorySnapshot records the exact category members the planner saw.
	// An absent or changed category is rejected rather than silently widened.
	CategorySnapshot map[string][]string
}

type CrossStatisticsGroup struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// CrossStatisticsResult contains aggregate public values only; no membership
// or job identities leave the repository. Groups are sorted over the entire
// eligible scope, before the caller applies any display limit or pagination.
type CrossStatisticsResult struct {
	Total  int                    `json:"total"`
	Groups []CrossStatisticsGroup `json:"groups,omitempty"`
}

type CrossStatisticsReader interface {
	ReadCrossStatistics(context.Context, CrossStatisticsQuery) (CrossStatisticsResult, error)
}

type CrossStatisticsBatchReader interface {
	ReadCrossStatisticsBatch(context.Context, []CrossStatisticsQuery) ([]CrossStatisticsResult, error)
}

var ErrInvalidCrossStatisticsQuery = errors.New("invalid cross-statistics query")

type crossVocabulary struct {
	labels     map[TrendDimension]map[string]string
	categories map[string][]string
}

// ReadCrossStatistics resolves the current formal vocabulary and computes
// job-identity-deduplicated aggregates in one read-only repeatable snapshot.
func (repository *Repository) ReadCrossStatistics(ctx context.Context, query CrossStatisticsQuery) (CrossStatisticsResult, error) {
	results, err := repository.ReadCrossStatisticsBatch(ctx, []CrossStatisticsQuery{query})
	if err != nil {
		return CrossStatisticsResult{}, err
	}
	return results[0], nil
}

// ReadCrossStatisticsBatch holds one snapshot for related sub-questions such
// as the individual, union and intersection counts of an ambiguous "and".
func (repository *Repository) ReadCrossStatisticsBatch(ctx context.Context, queries []CrossStatisticsQuery) ([]CrossStatisticsResult, error) {
	if repository == nil || repository.pool == nil {
		return nil, fmt.Errorf("serving repository is unavailable")
	}
	if len(queries) == 0 || len(queries) > 16 {
		return nil, fmt.Errorf("%w: invalid batch size", ErrInvalidCrossStatisticsQuery)
	}
	for _, query := range queries {
		if err := validateCrossQuery(query); err != nil {
			return nil, err
		}
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin cross-statistics read: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	vocabulary, err := readCrossVocabulary(ctx, tx)
	if err != nil {
		return nil, err
	}
	results := make([]CrossStatisticsResult, 0, len(queries))
	for _, query := range queries {
		result, err := readCrossStatisticsFrom(ctx, tx, query, vocabulary)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("finish cross-statistics read: %w", err)
	}
	return results, nil
}

func readCrossStatisticsFrom(ctx context.Context, tx pgx.Tx, query CrossStatisticsQuery, vocabulary crossVocabulary) (CrossStatisticsResult, error) {
	conditions, err := resolveCrossConditions(query.Conditions, vocabulary, query.CategorySnapshot)
	if err != nil {
		return CrossStatisticsResult{}, err
	}
	exclusions, err := resolveCrossConditions(query.Exclusions, vocabulary, query.CategorySnapshot)
	if err != nil {
		return CrossStatisticsResult{}, err
	}
	conditionsJSON, err := json.Marshal(conditions)
	if err != nil {
		return CrossStatisticsResult{}, fmt.Errorf("encode cross conditions: %w", err)
	}
	exclusionsJSON, err := json.Marshal(exclusions)
	if err != nil {
		return CrossStatisticsResult{}, fmt.Errorf("encode cross exclusions: %w", err)
	}
	rows, err := tx.Query(ctx, crossStatisticsSQL, conditionsJSON, exclusionsJSON, string(query.GroupBy), query.PrimaryFamilyOnly)
	if err != nil {
		return CrossStatisticsResult{}, fmt.Errorf("read cross statistics: %w", err)
	}
	result := CrossStatisticsResult{Groups: make([]CrossStatisticsGroup, 0)}
	for rows.Next() {
		var value *string
		var count int
		if err := rows.Scan(&value, &count); err != nil {
			rows.Close()
			return CrossStatisticsResult{}, fmt.Errorf("scan cross statistics: %w", err)
		}
		if value == nil {
			result.Total = count
			continue
		}
		label, ok := vocabulary.labels[query.GroupBy][*value]
		if !ok || (query.GroupBy == TrendDimensionProject && *value == "unrecognized") {
			rows.Close()
			return CrossStatisticsResult{}, fmt.Errorf("%w: grouped value is not in the current formal vocabulary", ErrInvalidCrossStatisticsQuery)
		}
		result.Groups = append(result.Groups, CrossStatisticsGroup{Value: *value, Label: label, Count: count})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return CrossStatisticsResult{}, fmt.Errorf("iterate cross statistics: %w", err)
	}
	rows.Close()
	sort.Slice(result.Groups, func(i, j int) bool {
		if result.Groups[i].Count != result.Groups[j].Count {
			return result.Groups[i].Count > result.Groups[j].Count
		}
		if result.Groups[i].Label != result.Groups[j].Label {
			return result.Groups[i].Label < result.Groups[j].Label
		}
		return result.Groups[i].Value < result.Groups[j].Value
	})
	return result, nil
}

func validateCrossQuery(query CrossStatisticsQuery) error {
	if query.GroupBy != "" && !supportedTrendEventDimension(query.GroupBy) {
		return fmt.Errorf("%w: unsupported grouping dimension", ErrInvalidCrossStatisticsQuery)
	}
	if len(query.Conditions)+len(query.Exclusions) == 0 && query.GroupBy == "" {
		return fmt.Errorf("%w: a condition or grouping is required", ErrInvalidCrossStatisticsQuery)
	}
	if len(query.Conditions)+len(query.Exclusions) > 16 {
		return fmt.Errorf("%w: too many condition groups", ErrInvalidCrossStatisticsQuery)
	}
	for _, groups := range [][]CrossCondition{query.Conditions, query.Exclusions} {
		for _, group := range groups {
			if !supportedTrendEventDimension(group.Dimension) || (group.Operator != "any" && group.Operator != "all") {
				return fmt.Errorf("%w: unsupported dimension or operator", ErrInvalidCrossStatisticsQuery)
			}
			if len(group.Values)+len(group.Categories) == 0 || len(group.Values)+len(group.Categories) > 100 {
				return fmt.Errorf("%w: empty or unbounded condition", ErrInvalidCrossStatisticsQuery)
			}
			if len(group.Categories) > 0 && (group.Dimension != TrendDimensionTechnology || group.Operator != "any") {
				return fmt.Errorf("%w: invalid category condition", ErrInvalidCrossStatisticsQuery)
			}
			for _, value := range append(append([]string(nil), group.Values...), group.Categories...) {
				if value == "" || value != strings.TrimSpace(value) || len([]rune(value)) > 128 {
					return fmt.Errorf("%w: invalid condition value", ErrInvalidCrossStatisticsQuery)
				}
			}
		}
	}
	return nil
}

func readCrossVocabulary(ctx context.Context, executor queryExecutor) (crossVocabulary, error) {
	vocabulary := crossVocabulary{
		labels: map[TrendDimension]map[string]string{
			TrendDimensionTechnology: {}, TrendDimensionProject: {}, TrendDimensionJobFamily: {}, TrendDimensionLocation: {},
		},
		categories: make(map[string][]string),
	}
	rows, err := executor.Query(ctx, technologyOptionsSQL)
	if err != nil {
		return crossVocabulary{}, fmt.Errorf("read cross technology vocabulary: %w", err)
	}
	for rows.Next() {
		var key, label, kind string
		if err := rows.Scan(&key, &label, &kind); err != nil {
			rows.Close()
			return crossVocabulary{}, fmt.Errorf("scan cross technology vocabulary: %w", err)
		}
		vocabulary.labels[TrendDimensionTechnology][key] = label
		category := technologyCategoryForKind(kind).category.Value
		vocabulary.categories[category] = append(vocabulary.categories[category], key)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return crossVocabulary{}, fmt.Errorf("iterate cross technology vocabulary: %w", err)
	}
	rows.Close()
	for _, item := range []struct {
		dimension TrendDimension
		query     string
		family    bool
	}{
		{TrendDimensionProject, projectOptionsSQL, false},
		{TrendDimensionLocation, locationOptionsSQL, false},
		{TrendDimensionJobFamily, jobFamilyOptionsSQL, true},
	} {
		rows, err := executor.Query(ctx, item.query)
		if err != nil {
			return crossVocabulary{}, fmt.Errorf("read cross formal vocabulary: %w", err)
		}
		for rows.Next() {
			var value, label string
			if item.family {
				err = rows.Scan(&value)
				label = jdanalysis.JobFamilyDisplayName(value)
			} else {
				err = rows.Scan(&value, &label)
			}
			if err != nil {
				rows.Close()
				return crossVocabulary{}, fmt.Errorf("scan cross formal vocabulary: %w", err)
			}
			if item.dimension != TrendDimensionProject || value != "unrecognized" {
				vocabulary.labels[item.dimension][value] = label
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return crossVocabulary{}, fmt.Errorf("iterate cross formal vocabulary: %w", err)
		}
		rows.Close()
	}
	return vocabulary, nil
}

func resolveCrossConditions(groups []CrossCondition, vocabulary crossVocabulary, expected map[string][]string) ([]CrossCondition, error) {
	resolved := make([]CrossCondition, 0, len(groups))
	for _, group := range groups {
		values := make(map[string]struct{})
		for _, value := range group.Values {
			if _, ok := vocabulary.labels[group.Dimension][value]; !ok {
				return nil, fmt.Errorf("%w: condition value is not current and formal", ErrInvalidCrossStatisticsQuery)
			}
			values[value] = struct{}{}
		}
		for _, category := range group.Categories {
			members := vocabulary.categories[category]
			prior, ok := expected[category]
			if !ok || len(members) == 0 || !equalCrossValues(members, prior) {
				return nil, fmt.Errorf("%w: category is unavailable or changed", ErrInvalidCrossStatisticsQuery)
			}
			for _, value := range members {
				values[value] = struct{}{}
			}
		}
		if len(values) == 0 {
			return nil, fmt.Errorf("%w: category has no current formal values", ErrInvalidCrossStatisticsQuery)
		}
		flat := CrossCondition{Dimension: group.Dimension, Operator: group.Operator, Values: make([]string, 0, len(values))}
		for value := range values {
			flat.Values = append(flat.Values, value)
		}
		sort.Strings(flat.Values)
		resolved = append(resolved, flat)
	}
	return resolved, nil
}

func equalCrossValues(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[string]int, len(left))
	for _, value := range left {
		counts[value]++
	}
	for _, value := range right {
		counts[value]--
		if counts[value] < 0 {
			return false
		}
	}
	return true
}
