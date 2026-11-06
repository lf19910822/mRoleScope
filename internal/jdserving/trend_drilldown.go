package jdserving

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ReadTrendDrilldown rebuilds one trusted historical trend node from immutable
// Serving memberships. The caller supplies a condition and a date, never a
// collection of job IDs, so current inventory cannot replace the node fact.
func (repository *Repository) ReadTrendDrilldown(ctx context.Context, query TrendDrilldownQuery) (TrendDrilldownResponse, error) {
	if repository == nil || repository.pool == nil {
		return TrendDrilldownResponse{}, fmt.Errorf("serving repository is unavailable")
	}
	if err := validateTrendDrilldownQuery(query); err != nil {
		return TrendDrilldownResponse{}, err
	}
	query.Title = strings.TrimSpace(query.Title)

	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return TrendDrilldownResponse{}, fmt.Errorf("begin trend drilldown read: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	filter, err := buildTrendDrilldownFilter(ctx, tx, query.Condition)
	if err != nil {
		return TrendDrilldownResponse{}, err
	}
	complete, err := trendNodeIsComplete(ctx, tx, query.NodeDate, filter)
	if err != nil {
		return TrendDrilldownResponse{}, err
	}
	if !complete {
		return TrendDrilldownResponse{}, ErrTrendNodeUnavailable
	}
	return readTrendDrilldownPage(ctx, tx, query, filter)
}

func validateTrendDrilldownQuery(query TrendDrilldownQuery) error {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return fmt.Errorf("load trend calendar location: %w", err)
	}
	date := query.NodeDate.In(shanghai)
	today := time.Now().In(shanghai)
	todayStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, shanghai)
	if query.NodeDate.IsZero() || date.Hour() != 0 || date.Minute() != 0 || date.Second() != 0 || date.Nanosecond() != 0 ||
		date.After(todayStart) || query.ExpectedCount < 0 || query.PageNumber < 1 || query.PageSize < 1 || query.PageSize > MaximumPageSize {
		return ErrInvalidTrendQuery
	}
	return nil
}

const trendNodeRunsCTE = `
WITH selected_runs AS (
    SELECT DISTINCT ON (run.observation_scope_id)
        run.id, run.observation_scope_id, run.published_at
    FROM collection_runs AS run
    WHERE run.status = 'published'
      AND run.published_at IS NOT NULL
      AND (run.published_at AT TIME ZONE 'Asia/Shanghai')::date = $1::date
    ORDER BY run.observation_scope_id, run.published_at DESC, run.id DESC
)
`

type trendDrilldownFilter struct {
	args                []any
	completenessArgs    []any
	completenessJoins   string
	formalIncompleteSQL string
	joins               string
	predicate           string
}

func buildTrendDrilldownFilter(ctx context.Context, tx pgx.Tx, condition TrendCondition) (trendDrilldownFilter, error) {
	switch condition.Dimension {
	case TrendDimensionLocation:
		if err := validateLocationTrendQuery(TrendQuery{Condition: condition}); err != nil {
			return trendDrilldownFilter{}, err
		}
		return trendDrilldownFilter{args: []any{condition.Value}, predicate: "membership.locations @> ARRAY[$2]::text[]"}, nil
	case TrendDimensionEducation:
		value, _, err := validateEducationTrendQuery(TrendQuery{Condition: condition})
		if err != nil {
			return trendDrilldownFilter{}, err
		}
		component, err := readPublishedTrendEducationComponent(ctx, tx)
		if err != nil {
			return trendDrilldownFilter{}, err
		}
		resultJoin := `
LEFT JOIN education_requirement_results AS result
  ON result.body_content_version_id = membership.body_content_version_id
 AND result.parser_version = $2
 AND result.lexicon_version = $3
 AND result.lexicon_sha256 = $4
 AND result.completed_at IS NOT NULL`
		return trendDrilldownFilter{
			args:                []any{component.ParserVersion, component.LexiconVersion, component.LexiconSHA256, string(condition.Operator), value},
			completenessArgs:    []any{component.ParserVersion, component.LexiconVersion, component.LexiconSHA256},
			completenessJoins:   resultJoin,
			formalIncompleteSQL: "result.id IS NULL",
			joins:               strings.Replace(resultJoin, "LEFT JOIN", "JOIN", 1),
			predicate:           "result.status = 'configured' AND result.lower_bound_level IS NOT NULL AND " + educationComparisonSQL("result.lower_bound_level", "$5", "$6"),
		}, nil
	case TrendDimensionExperience:
		value, _, err := validateExperienceTrendQuery(TrendQuery{Condition: condition})
		if err != nil {
			return trendDrilldownFilter{}, err
		}
		component, err := readPublishedTrendExperienceComponent(ctx, tx)
		if err != nil {
			return trendDrilldownFilter{}, err
		}
		resultJoin := `
LEFT JOIN experience_requirement_results AS result
  ON result.body_content_version_id = membership.body_content_version_id
 AND result.parser_version = $2
 AND result.lexicon_version = $3
 AND result.lexicon_sha256 = $4
 AND result.completed_at IS NOT NULL`
		return trendDrilldownFilter{
			args:                []any{component.ParserVersion, component.LexiconVersion, component.LexiconSHA256, string(condition.Operator), value},
			completenessArgs:    []any{component.ParserVersion, component.LexiconVersion, component.LexiconSHA256},
			completenessJoins:   resultJoin,
			formalIncompleteSQL: "result.id IS NULL",
			joins:               strings.Replace(resultJoin, "LEFT JOIN", "JOIN", 1),
			predicate: `result.status = 'configured' AND result.base_year IS NOT NULL
  AND CASE $5::text
      WHEN 'eq' THEN result.base_year = $6::integer
      WHEN 'gt' THEN result.base_year > $6::integer
      WHEN 'gte' THEN result.base_year >= $6::integer
      WHEN 'lt' THEN result.base_year < $6::integer
      WHEN 'lte' THEN result.base_year <= $6::integer
      ELSE false
  END`,
		}, nil
	case TrendDimensionTechnology:
		_, _, technologyID, err := readTrendTechnologyIdentity(ctx, tx, TrendQuery{Condition: condition})
		if err != nil {
			return trendDrilldownFilter{}, err
		}
		component, err := readPublishedTrendTechnologyComponent(ctx, tx)
		if err != nil {
			return trendDrilldownFilter{}, err
		}
		resultJoin := `
LEFT JOIN technology_analysis_results AS result
  ON result.body_content_version_id = membership.body_content_version_id
 AND result.configuration_id = $2::uuid
 AND result.status = 'complete'
 AND result.completed_at IS NOT NULL`
		return trendDrilldownFilter{
			args:                []any{component.ConfigurationID, technologyID},
			completenessArgs:    []any{component.ConfigurationID},
			completenessJoins:   resultJoin,
			formalIncompleteSQL: "result.id IS NULL",
			joins: strings.Replace(resultJoin, "LEFT JOIN", "JOIN", 1) + `
JOIN technology_mentions AS mention ON mention.analysis_result_id = result.id`,
			predicate: "mention.technology_identity_id = $3::uuid",
		}, nil
	case TrendDimensionProject:
		value, err := validateProjectTrendQuery(TrendQuery{Condition: condition})
		if err != nil {
			return trendDrilldownFilter{}, err
		}
		component, err := readPublishedTrendProjectComponent(ctx, tx)
		if err != nil {
			return trendDrilldownFilter{}, err
		}
		if _, err := readTrendProjectLabel(ctx, tx, component, value); err != nil {
			return trendDrilldownFilter{}, err
		}
		metadataJoin := `
JOIN job_source_metadata_versions AS metadata ON metadata.id = membership.source_metadata_version_id
LEFT JOIN project_title_observations AS observation ON observation.source_metadata_version_id = metadata.id
LEFT JOIN project_tag_results AS result
  ON result.project_title_identity_id = observation.project_title_identity_id
 AND result.parser_version = $2
 AND result.dictionary_version = $3
 AND result.dictionary_sha256 = $4
 AND result.completed_at IS NOT NULL`
		return trendDrilldownFilter{
			args:                []any{component.ParserVersion, component.DictionaryVersion, component.DictionarySHA256, value},
			completenessArgs:    []any{component.ParserVersion, component.DictionaryVersion, component.DictionarySHA256},
			completenessJoins:   metadataJoin,
			formalIncompleteSQL: `metadata.metadata_payload #>> '{title,state}' = 'string' AND result.id IS NULL`,
			joins: strings.ReplaceAll(metadataJoin, "LEFT JOIN", "JOIN") + `
JOIN project_tag_assignments AS assignment ON assignment.project_tag_result_id = result.id`,
			predicate: "assignment.project_key = $5",
		}, nil
	case TrendDimensionJobFamily:
		value, _, err := validateJobFamilyTrendQuery(TrendQuery{Condition: condition})
		if err != nil {
			return trendDrilldownFilter{}, err
		}
		component, err := readPublishedTrendJobFamilyComponent(ctx, tx)
		if err != nil {
			return trendDrilldownFilter{}, err
		}
		metadataJoin := `
JOIN job_source_metadata_versions AS metadata ON metadata.id = membership.source_metadata_version_id
LEFT JOIN job_family_observations AS observation ON observation.source_metadata_version_id = metadata.id
LEFT JOIN job_family_results AS result
  ON result.job_family_title_identity_id = observation.job_family_title_identity_id
 AND result.parser_version = $2
 AND result.dictionary_version = $3
 AND result.dictionary_sha256 = $4
 AND result.project_mask_parser_version = $5
 AND result.project_mask_dictionary_version = $6
 AND result.project_mask_dictionary_sha256 = $7
 AND result.completed_at IS NOT NULL`
		return trendDrilldownFilter{
			args: []any{component.ParserVersion, component.DictionaryVersion, component.DictionarySHA256,
				component.ProjectMaskParserVersion, component.ProjectMaskDictionaryVersion, component.ProjectMaskDictionarySHA256,
				condition.IncludeSecondary, value},
			completenessArgs: []any{component.ParserVersion, component.DictionaryVersion, component.DictionarySHA256,
				component.ProjectMaskParserVersion, component.ProjectMaskDictionaryVersion, component.ProjectMaskDictionarySHA256},
			completenessJoins: metadataJoin,
			formalIncompleteSQL: `metadata.metadata_payload #>> '{title,state}' = 'string'
  AND (observation.status IS NULL OR observation.status = 'in_scope') AND result.id IS NULL`,
			joins: strings.ReplaceAll(metadataJoin, "LEFT JOIN", "JOIN") + `
JOIN job_family_matches AS match ON match.job_family_result_id = result.id`,
			predicate: `result.primary_family_key = $9 OR ($8::boolean AND match.family_key = $9)`,
		}, nil
	default:
		return trendDrilldownFilter{}, ErrInvalidTrendQuery
	}
}

func educationComparisonSQL(level, operator, value string) string {
	rank := func(field string) string {
		return "CASE " + field + "::text WHEN 'associate_or_lower' THEN 1 WHEN 'bachelor' THEN 2 WHEN 'master' THEN 3 WHEN 'doctor' THEN 4 END"
	}
	return `CASE ` + operator + `::text
    WHEN 'eq' THEN (` + rank(level) + ` = ` + rank(value) + `)
    WHEN 'gt' THEN (` + rank(level) + ` > ` + rank(value) + `)
    WHEN 'gte' THEN (` + rank(level) + ` >= ` + rank(value) + `)
    WHEN 'lt' THEN (` + rank(level) + ` < ` + rank(value) + `)
    WHEN 'lte' THEN (` + rank(level) + ` <= ` + rank(value) + `)
    ELSE false END`
}

func trendNodeIsComplete(ctx context.Context, tx pgx.Tx, nodeDate time.Time, filter trendDrilldownFilter) (bool, error) {
	query := trendNodeRunsCTE + `,
source_incomplete AS (
    SELECT 1
    FROM selected_runs AS selected
    JOIN LATERAL (
        SELECT DISTINCT ON (presence.job_id) presence.job_id
        FROM job_presence_periods AS presence
        WHERE presence.observation_scope_id = selected.observation_scope_id
          AND presence.opened_at <= selected.published_at
        ORDER BY presence.job_id, presence.opened_at DESC, presence.id DESC
    ) AS observed_presence ON true
    LEFT JOIN serving_jd_inventory_memberships AS membership
      ON membership.published_run_id = selected.id AND membership.job_id = observed_presence.job_id
    WHERE membership.id IS NULL
), formal_incomplete AS (
    SELECT 1
    FROM selected_runs AS selected
    JOIN serving_jd_inventory_memberships AS membership ON membership.published_run_id = selected.id
    ` + filter.completenessJoins + `
    WHERE ` + falseWhenBlank(filter.formalIncompleteSQL) + `
)
SELECT (SELECT count(*) FROM selected_runs) > 0
   AND NOT EXISTS (SELECT 1 FROM source_incomplete)
   AND NOT EXISTS (SELECT 1 FROM formal_incomplete)`
	args := append([]any{nodeDate.Format("2006-01-02")}, filter.completenessArgs...)
	var complete bool
	if err := tx.QueryRow(ctx, query, args...).Scan(&complete); err != nil {
		return false, fmt.Errorf("check trend node completeness: %w", err)
	}
	return complete, nil
}

func falseWhenBlank(statement string) string {
	if strings.TrimSpace(statement) == "" {
		return "false"
	}
	return statement
}

type trendDrilldownRow struct {
	BodyContentVersionID string
	CurrentActivityState *string
	CurrentBodyVersionID *string
	CurrentMetadataID    *string
	HistoricalMetadataID string
	ID                   string
	Title                string
}

func readTrendDrilldownPage(ctx context.Context, tx pgx.Tx, query TrendDrilldownQuery, filter trendDrilldownFilter) (TrendDrilldownResponse, error) {
	titleParameter := 2 + len(filter.args)
	similarityParameter := titleParameter + 1
	limitParameter := similarityParameter + 1
	offsetParameter := limitParameter + 1
	nodeMemberships := fmt.Sprintf(trendNodeRunsCTE+`, node_memberships AS (
    SELECT DISTINCT ON (membership.job_id)
        membership.id, membership.job_id, membership.source_metadata_version_id,
        membership.body_content_version_id, membership.title, selected.published_at
    FROM selected_runs AS selected
    JOIN serving_jd_inventory_memberships AS membership ON membership.published_run_id = selected.id
    %s
    WHERE membership.activity_state = 'effective' AND (%s)
    ORDER BY membership.job_id, selected.published_at DESC, membership.id DESC
), filtered AS (
    SELECT history.* FROM node_memberships AS history
    WHERE $%d::text = ''
       OR lower(history.title) LIKE '%%%%' || lower($%d::text) || '%%%%'
       OR similarity(lower(history.title), lower($%d::text)) >= $%d::real
)`, filter.joins, filter.predicate, titleParameter, titleParameter, titleParameter, similarityParameter)
	args := append([]any{query.NodeDate.Format("2006-01-02")}, filter.args...)
	args = append(args, query.Title, fuzzyTitleSimilarityThreshold)
	var matchedTotal int
	if err := tx.QueryRow(ctx, nodeMemberships+`SELECT count(*) FROM filtered`, args...).Scan(&matchedTotal); err != nil {
		return TrendDrilldownResponse{}, fmt.Errorf("count trend drilldown jobs: %w", err)
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(nodeMemberships+`
SELECT identifier.public_id::text, filtered.title, filtered.source_metadata_version_id::text,
       filtered.body_content_version_id::text, current_membership.activity_state::text,
       current_membership.source_metadata_version_id::text, current_membership.body_content_version_id::text
FROM filtered
JOIN serving_jd_public_identifiers AS identifier ON identifier.job_id = filtered.job_id
LEFT JOIN LATERAL (
    SELECT current.*
    FROM serving_jd_current_inventory_memberships AS current
    JOIN collection_runs AS current_run ON current_run.id = current.published_run_id
    WHERE current.job_id = filtered.job_id
    ORDER BY current_run.published_at DESC, current.published_run_id DESC
    LIMIT 1
) AS current_membership ON true
ORDER BY lower(filtered.title), identifier.public_id
LIMIT $%d OFFSET $%d`, limitParameter, offsetParameter), append(args, query.PageSize, (query.PageNumber-1)*query.PageSize)...)
	if err != nil {
		return TrendDrilldownResponse{}, fmt.Errorf("select trend drilldown jobs: %w", err)
	}
	defer rows.Close()
	response := TrendDrilldownResponse{
		Items: make([]TrendDrilldownItem, 0), MatchedTotal: matchedTotal,
		NodeDate: query.NodeDate.Format("2006-01-02"),
		Page:     PageInfo{Number: query.PageNumber, PageSize: query.PageSize, Total: matchedTotal},
	}
	if query.Title == "" {
		matchesExpected := matchedTotal == query.ExpectedCount
		response.CountMatchesExpected = &matchesExpected
	}
	for rows.Next() {
		var row trendDrilldownRow
		if err := rows.Scan(&row.ID, &row.Title, &row.HistoricalMetadataID, &row.BodyContentVersionID,
			&row.CurrentActivityState, &row.CurrentMetadataID, &row.CurrentBodyVersionID); err != nil {
			return TrendDrilldownResponse{}, fmt.Errorf("scan trend drilldown job: %w", err)
		}
		response.Items = append(response.Items, trendDrilldownItem(row))
	}
	if err := rows.Err(); err != nil {
		return TrendDrilldownResponse{}, fmt.Errorf("iterate trend drilldown jobs: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return TrendDrilldownResponse{}, fmt.Errorf("commit trend drilldown read: %w", err)
	}
	return response, nil
}

func trendDrilldownItem(row trendDrilldownRow) TrendDrilldownItem {
	item := TrendDrilldownItem{ID: row.ID, Title: row.Title}
	if row.CurrentActivityState == nil || *row.CurrentActivityState == string(ActivityStateConfirmedClosed) {
		item.Status, item.CanViewHistorical = TrendDrilldownStatusClosed, true
		return item
	}
	if row.CurrentMetadataID != nil && row.CurrentBodyVersionID != nil &&
		*row.CurrentMetadataID == row.HistoricalMetadataID && *row.CurrentBodyVersionID == row.BodyContentVersionID {
		item.Status, item.CanViewLatest = TrendDrilldownStatusCurrent, true
		return item
	}
	item.Status, item.CanViewHistorical, item.CanViewLatest = TrendDrilldownStatusChanged, true, true
	return item
}
