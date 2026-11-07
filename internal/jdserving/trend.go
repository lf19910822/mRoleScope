package jdserving

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"mrolescope/internal/jdanalysis"
)

// TrendRange is the viewer-selectable natural calendar range for one curve.
type TrendRange string

const (
	TrendRangeAll     TrendRange = "all"
	TrendRangeMonth   TrendRange = "month"
	TrendRangeQuarter TrendRange = "quarter"
	TrendRangeWeek    TrendRange = "week"
	TrendRangeYear    TrendRange = "year"
	TrendRangeCustom  TrendRange = "custom"
)

// TrendDimension identifies the one formal condition counted by a curve.
type TrendDimension string

const (
	TrendDimensionEducation  TrendDimension = "education"
	TrendDimensionExperience TrendDimension = "experience"
	TrendDimensionJobFamily  TrendDimension = "job_family"
	TrendDimensionLocation   TrendDimension = "location"
	TrendDimensionProject    TrendDimension = "project"
	TrendDimensionTechnology TrendDimension = "technology"
)

// TrendComparison is meaningful only for education and experience curves.
type TrendComparison string

const (
	TrendComparisonEqual              TrendComparison = "eq"
	TrendComparisonGreaterThan        TrendComparison = "gt"
	TrendComparisonGreaterThanOrEqual TrendComparison = "gte"
	TrendComparisonLessThan           TrendComparison = "lt"
	TrendComparisonLessThanOrEqual    TrendComparison = "lte"
)

// TrendCondition is one normalized condition. Composite AND/OR conditions
// deliberately do not belong in a single curve.
type TrendCondition struct {
	Dimension        TrendDimension
	IncludeSecondary bool
	Operator         TrendComparison
	Value            string
}

// TrendQuery asks for exactly one named range and one curve condition.
type TrendQuery struct {
	Condition TrendCondition
	// StartDate and EndDate are used only with TrendRangeCustom. They are
	// midnight Asia/Shanghai dates so the reader never has to infer a client
	// timezone from an instant.
	StartDate time.Time
	EndDate   time.Time
	Range     TrendRange
}

// TrendCurve is the display-safe description echoed with a trend result.
type TrendCurve struct {
	Dimension        TrendDimension  `json:"dimension"`
	IncludeSecondary bool            `json:"include_secondary,omitempty"`
	Label            string          `json:"label"`
	Operator         TrendComparison `json:"operator,omitempty"`
	Value            string          `json:"value"`
}

// TrendPoint is one calendar date. A nil Count is unavailable history, which
// is intentionally different from an observed complete count of zero.
type TrendPoint struct {
	Count *int   `json:"count"`
	Date  string `json:"date"`
}

// TrendCoverage is the display-safe envelope of successful published history.
// It describes only the outer bounds; intermediate unavailable dates remain
// represented by null points and are intentionally not enumerated here.
type TrendCoverage struct {
	FirstDate string `json:"first_date"`
	LastDate  string `json:"last_date"`
}

// TrendSeries is the complete display-safe result for one requested curve.
// Freshness is opaque and is never a source/run/parser/dictionary identifier.
type TrendSeries struct {
	Curve     TrendCurve    `json:"curve"`
	Coverage  TrendCoverage `json:"coverage"`
	Freshness string        `json:"freshness"`
	Points    []TrendPoint  `json:"points"`
}

// TrendReader is the public Serving seam for a single historical curve.
// Implementations hide snapshot selection and formal-result reconstruction.
type TrendReader interface {
	ReadTrend(context.Context, TrendQuery) (TrendSeries, error)
}

// TrendDrilldownQuery is one bounded read of the jobs represented by a
// numeric historical node. ExpectedCount is compared with the unsearched
// result but never selects or filters memberships.
type TrendDrilldownQuery struct {
	Condition     TrendCondition
	ExpectedCount int
	NodeDate      time.Time
	PageNumber    int
	PageSize      int
	Title         string
}

// TrendDrilldownReader hides final-run selection, formal predicate evaluation,
// and current-status comparison behind the Serving boundary.
type TrendDrilldownReader interface {
	ReadTrendDrilldown(context.Context, TrendDrilldownQuery) (TrendDrilldownResponse, error)
}

// ErrTrendNodeUnavailable means the selected date cannot be reconstructed as
// a trustworthy historical node. It carries no source/run detail.
var ErrTrendNodeUnavailable = errors.New("trend node unavailable")

// ErrInvalidTrendQuery denotes an incompatible or unbounded public condition.
// HTTP maps it to a safe client error without exposing a storage detail.
var ErrInvalidTrendQuery = errors.New("invalid trend query")

// ReadTrend establishes historical source selection and evaluates one bounded
// curve under the condition's applicable immutable facts. Formal dimensions
// always use one published, complete trend interpretation release rather than
// current-list resolutions.
func (repository *Repository) ReadTrend(ctx context.Context, query TrendQuery) (TrendSeries, error) {
	if repository == nil || repository.pool == nil {
		return TrendSeries{}, fmt.Errorf("serving repository is unavailable")
	}
	calendarRange, err := resolveTrendCalendarRange(query)
	if err != nil {
		return TrendSeries{}, err
	}

	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return TrendSeries{}, fmt.Errorf("begin trend read: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var series TrendSeries
	switch query.Condition.Dimension {
	case TrendDimensionLocation:
		if err := validateLocationTrendQuery(query); err != nil {
			return TrendSeries{}, err
		}
		points, err := readAllLocationTrendPoints(ctx, tx, query.Condition.Value, calendarRange)
		if err != nil {
			return TrendSeries{}, err
		}
		series = TrendSeries{
			Curve: TrendCurve{
				Dimension: TrendDimensionLocation,
				Label:     query.Condition.Value,
				Value:     query.Condition.Value,
			},
			Points: points,
		}
	case TrendDimensionEducation:
		value, label, err := validateEducationTrendQuery(query)
		if err != nil {
			return TrendSeries{}, err
		}
		component, err := readPublishedTrendEducationComponent(ctx, tx)
		if err != nil {
			return TrendSeries{}, err
		}
		points, err := readAllEducationTrendPoints(ctx, tx, component, query.Condition.Operator, value, calendarRange)
		if err != nil {
			return TrendSeries{}, err
		}
		series = TrendSeries{
			Curve: TrendCurve{
				Dimension: TrendDimensionEducation,
				Label:     label,
				Operator:  query.Condition.Operator,
				Value:     value,
			},
			Points: points,
		}
		series.Freshness = component.IdentitySHA256
	case TrendDimensionExperience:
		value, label, err := validateExperienceTrendQuery(query)
		if err != nil {
			return TrendSeries{}, err
		}
		component, err := readPublishedTrendExperienceComponent(ctx, tx)
		if err != nil {
			return TrendSeries{}, err
		}
		points, err := readAllExperienceTrendPoints(ctx, tx, component, query.Condition.Operator, value, calendarRange)
		if err != nil {
			return TrendSeries{}, err
		}
		series = TrendSeries{
			Curve: TrendCurve{
				Dimension: TrendDimensionExperience,
				Label:     label,
				Operator:  query.Condition.Operator,
				Value:     strconv.Itoa(value),
			},
			Points: points,
		}
		series.Freshness = component.IdentitySHA256
	case TrendDimensionTechnology:
		value, label, technologyID, err := readTrendTechnologyIdentity(ctx, tx, query)
		if err != nil {
			return TrendSeries{}, err
		}
		component, err := readPublishedTrendTechnologyComponent(ctx, tx)
		if err != nil {
			return TrendSeries{}, err
		}
		points, err := readAllTechnologyTrendPoints(ctx, tx, component, technologyID, calendarRange)
		if err != nil {
			return TrendSeries{}, err
		}
		series = TrendSeries{
			Curve: TrendCurve{
				Dimension: TrendDimensionTechnology,
				Label:     label,
				Value:     value,
			},
			Points: points,
		}
		series.Freshness = component.IdentitySHA256
	case TrendDimensionProject:
		value, err := validateProjectTrendQuery(query)
		if err != nil {
			return TrendSeries{}, err
		}
		component, err := readPublishedTrendProjectComponent(ctx, tx)
		if err != nil {
			return TrendSeries{}, err
		}
		label, err := readTrendProjectLabel(ctx, tx, component, value)
		if err != nil {
			return TrendSeries{}, err
		}
		points, err := readAllProjectTrendPoints(ctx, tx, component, value, calendarRange)
		if err != nil {
			return TrendSeries{}, err
		}
		series = TrendSeries{
			Curve: TrendCurve{
				Dimension: TrendDimensionProject,
				Label:     label,
				Value:     value,
			},
			Points: points,
		}
		series.Freshness = component.IdentitySHA256
	case TrendDimensionJobFamily:
		value, label, err := validateJobFamilyTrendQuery(query)
		if err != nil {
			return TrendSeries{}, err
		}
		component, err := readPublishedTrendJobFamilyComponent(ctx, tx)
		if err != nil {
			return TrendSeries{}, err
		}
		points, err := readAllJobFamilyTrendPoints(ctx, tx, component, query.Condition.IncludeSecondary, value, calendarRange)
		if err != nil {
			return TrendSeries{}, err
		}
		series = TrendSeries{
			Curve: TrendCurve{
				Dimension:        TrendDimensionJobFamily,
				IncludeSecondary: query.Condition.IncludeSecondary,
				Label:            label,
				Value:            value,
			},
			Points: points,
		}
		series.Freshness = component.IdentitySHA256
	default:
		return TrendSeries{}, ErrInvalidTrendQuery
	}
	freshnessInput, err := readTrendFreshnessInput(ctx, tx, calendarRange)
	if err != nil {
		return TrendSeries{}, err
	}
	if series.Freshness != "" {
		freshnessInput += "\nformal=" + series.Freshness
	}
	coverage, err := readTrendCoverage(ctx, tx)
	if err != nil {
		return TrendSeries{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TrendSeries{}, fmt.Errorf("commit trend read: %w", err)
	}
	series.Freshness = trendFreshness(freshnessInput)
	series.Coverage = coverage
	return series, nil
}

func validateLocationTrendQuery(query TrendQuery) error {
	if query.Condition.Dimension != TrendDimensionLocation ||
		strings.TrimSpace(query.Condition.Value) == "" || query.Condition.Operator != "" || query.Condition.IncludeSecondary {
		return ErrInvalidTrendQuery
	}
	return nil
}

type trendCalendarRange struct {
	End   string
	Start *string
}

func resolveTrendCalendarRange(query TrendQuery) (trendCalendarRange, error) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return trendCalendarRange{}, fmt.Errorf("load trend calendar location: %w", err)
	}
	today := time.Now().In(shanghai)
	format := func(date time.Time) string { return date.Format("2006-01-02") }
	rangeValue := trendCalendarRange{End: format(today)}
	var start time.Time
	switch query.Range {
	case TrendRangeAll:
		return rangeValue, nil
	case TrendRangeCustom:
		start := query.StartDate.In(shanghai)
		end := query.EndDate.In(shanghai)
		if query.StartDate.IsZero() || query.EndDate.IsZero() ||
			start.Hour() != 0 || start.Minute() != 0 || start.Second() != 0 || start.Nanosecond() != 0 ||
			end.Hour() != 0 || end.Minute() != 0 || end.Second() != 0 || end.Nanosecond() != 0 ||
			start.After(end) {
			return trendCalendarRange{}, ErrInvalidTrendQuery
		}
		formattedStart := format(start)
		formattedEnd := format(end)
		return trendCalendarRange{Start: &formattedStart, End: formattedEnd}, nil
	case TrendRangeWeek:
		start = today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
	case TrendRangeMonth:
		start = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, shanghai)
	case TrendRangeQuarter:
		month := time.Month(((int(today.Month())-1)/3)*3 + 1)
		start = time.Date(today.Year(), month, 1, 0, 0, 0, 0, shanghai)
	case TrendRangeYear:
		start = time.Date(today.Year(), time.January, 1, 0, 0, 0, 0, shanghai)
	default:
		return trendCalendarRange{}, ErrInvalidTrendQuery
	}
	formattedStart := format(start)
	rangeValue.Start = &formattedStart
	return rangeValue, nil
}

func validateEducationTrendQuery(query TrendQuery) (string, string, error) {
	if query.Condition.Dimension != TrendDimensionEducation || query.Condition.IncludeSecondary {
		return "", "", ErrInvalidTrendQuery
	}
	if query.Condition.Operator != TrendComparisonEqual &&
		query.Condition.Operator != TrendComparisonGreaterThan &&
		query.Condition.Operator != TrendComparisonGreaterThanOrEqual &&
		query.Condition.Operator != TrendComparisonLessThan &&
		query.Condition.Operator != TrendComparisonLessThanOrEqual {
		return "", "", ErrInvalidTrendQuery
	}
	value := strings.TrimSpace(strings.ToLower(query.Condition.Value))
	switch value {
	case string(EducationLevelAssociateOrLower):
		return value, "大专及以下", nil
	case string(EducationLevelBachelor):
		return value, "本科", nil
	case string(EducationLevelMaster):
		return value, "硕士", nil
	case string(EducationLevelDoctor):
		return value, "博士", nil
	default:
		return "", "", ErrInvalidTrendQuery
	}
}

func validateExperienceTrendQuery(query TrendQuery) (int, string, error) {
	if query.Condition.Dimension != TrendDimensionExperience || query.Condition.IncludeSecondary {
		return 0, "", ErrInvalidTrendQuery
	}
	if query.Condition.Operator != TrendComparisonEqual &&
		query.Condition.Operator != TrendComparisonGreaterThan &&
		query.Condition.Operator != TrendComparisonGreaterThanOrEqual &&
		query.Condition.Operator != TrendComparisonLessThan &&
		query.Condition.Operator != TrendComparisonLessThanOrEqual {
		return 0, "", ErrInvalidTrendQuery
	}
	value, err := strconv.Atoi(strings.TrimSpace(query.Condition.Value))
	if err != nil || value < 0 || value > 9 {
		return 0, "", ErrInvalidTrendQuery
	}
	return value, strconv.Itoa(value) + " 年", nil
}

func validateTechnologyTrendQuery(query TrendQuery) (string, error) {
	if query.Condition.Dimension != TrendDimensionTechnology ||
		query.Condition.IncludeSecondary || query.Condition.Operator != "" {
		return "", ErrInvalidTrendQuery
	}
	value := strings.TrimSpace(strings.ToLower(query.Condition.Value))
	if value == "" {
		return "", ErrInvalidTrendQuery
	}
	return value, nil
}

func validateProjectTrendQuery(query TrendQuery) (string, error) {
	if query.Condition.Dimension != TrendDimensionProject ||
		query.Condition.IncludeSecondary || query.Condition.Operator != "" {
		return "", ErrInvalidTrendQuery
	}
	value := strings.TrimSpace(query.Condition.Value)
	if value == "" {
		return "", ErrInvalidTrendQuery
	}
	return value, nil
}

func validateJobFamilyTrendQuery(query TrendQuery) (string, string, error) {
	if query.Condition.Dimension != TrendDimensionJobFamily || query.Condition.Operator != "" {
		return "", "", ErrInvalidTrendQuery
	}
	value := strings.TrimSpace(query.Condition.Value)
	if value == "" {
		return "", "", ErrInvalidTrendQuery
	}
	label := jdanalysis.JobFamilyDisplayName(value)
	if label == value {
		return "", "", ErrInvalidTrendQuery
	}
	return value, label, nil
}

type trendFormalEducationComponent struct {
	IdentitySHA256 string
	LexiconSHA256  string
	LexiconVersion string
	ParserVersion  string
}

type trendFormalExperienceComponent struct {
	IdentitySHA256 string
	LexiconSHA256  string
	LexiconVersion string
	ParserVersion  string
}

type trendFormalTechnologyComponent struct {
	ConfigurationID string
	IdentitySHA256  string
}

type trendFormalProjectComponent struct {
	DictionarySHA256  string
	DictionaryVersion string
	IdentitySHA256    string
	ParserVersion     string
}

type trendFormalJobFamilyComponent struct {
	DictionarySHA256             string
	DictionaryVersion            string
	IdentitySHA256               string
	ParserVersion                string
	ProjectMaskDictionarySHA256  string
	ProjectMaskDictionaryVersion string
	ProjectMaskParserVersion     string
}

func readPublishedTrendEducationComponent(ctx context.Context, tx pgx.Tx) (trendFormalEducationComponent, error) {
	component := trendFormalEducationComponent{}
	err := tx.QueryRow(ctx, `
SELECT release.identity_sha256,
       component.parser_version,
       component.lexicon_version,
       component.lexicon_sha256
FROM trend_formal_releases AS release
JOIN trend_formal_release_components AS component
  ON component.trend_formal_release_id = release.id
 AND component.dimension = 'education'
WHERE release.status = 'published'`).Scan(
		&component.IdentitySHA256,
		&component.ParserVersion,
		&component.LexiconVersion,
		&component.LexiconSHA256,
	)
	if err == pgx.ErrNoRows {
		return trendFormalEducationComponent{}, fmt.Errorf("published formal trend release is unavailable")
	}
	if err != nil {
		return trendFormalEducationComponent{}, fmt.Errorf("read published education trend release: %w", err)
	}
	return component, nil
}

func readPublishedTrendExperienceComponent(ctx context.Context, tx pgx.Tx) (trendFormalExperienceComponent, error) {
	component := trendFormalExperienceComponent{}
	err := tx.QueryRow(ctx, `
SELECT release.identity_sha256,
       component.parser_version,
       component.lexicon_version,
       component.lexicon_sha256
FROM trend_formal_releases AS release
JOIN trend_formal_release_components AS component
  ON component.trend_formal_release_id = release.id
 AND component.dimension = 'experience'
WHERE release.status = 'published'`).Scan(
		&component.IdentitySHA256,
		&component.ParserVersion,
		&component.LexiconVersion,
		&component.LexiconSHA256,
	)
	if err == pgx.ErrNoRows {
		return trendFormalExperienceComponent{}, fmt.Errorf("published formal trend release is unavailable")
	}
	if err != nil {
		return trendFormalExperienceComponent{}, fmt.Errorf("read published experience trend release: %w", err)
	}
	return component, nil
}

func readTrendTechnologyIdentity(ctx context.Context, tx pgx.Tx, query TrendQuery) (string, string, string, error) {
	value, err := validateTechnologyTrendQuery(query)
	if err != nil {
		return "", "", "", err
	}
	var technologyID, label string
	err = tx.QueryRow(ctx, `
SELECT id::text, canonical_name
FROM technology_identities
WHERE technology_key = $1`, value).Scan(&technologyID, &label)
	if err == pgx.ErrNoRows {
		return "", "", "", ErrInvalidTrendQuery
	}
	if err != nil {
		return "", "", "", fmt.Errorf("read trend technology identity: %w", err)
	}
	return value, label, technologyID, nil
}

func readPublishedTrendTechnologyComponent(ctx context.Context, tx pgx.Tx) (trendFormalTechnologyComponent, error) {
	component := trendFormalTechnologyComponent{}
	err := tx.QueryRow(ctx, `
SELECT release.identity_sha256,
       component.technology_analysis_configuration_id::text
FROM trend_formal_releases AS release
JOIN trend_formal_release_components AS component
  ON component.trend_formal_release_id = release.id
 AND component.dimension = 'technology'
WHERE release.status = 'published'`).Scan(
		&component.IdentitySHA256,
		&component.ConfigurationID,
	)
	if err == pgx.ErrNoRows {
		return trendFormalTechnologyComponent{}, fmt.Errorf("published formal trend release is unavailable")
	}
	if err != nil {
		return trendFormalTechnologyComponent{}, fmt.Errorf("read published technology trend release: %w", err)
	}
	return component, nil
}

func readPublishedTrendProjectComponent(ctx context.Context, tx pgx.Tx) (trendFormalProjectComponent, error) {
	component := trendFormalProjectComponent{}
	err := tx.QueryRow(ctx, `
SELECT release.identity_sha256,
       component.parser_version,
       component.dictionary_version,
       component.dictionary_sha256
FROM trend_formal_releases AS release
JOIN trend_formal_release_components AS component
  ON component.trend_formal_release_id = release.id
 AND component.dimension = 'project'
WHERE release.status = 'published'`).Scan(
		&component.IdentitySHA256,
		&component.ParserVersion,
		&component.DictionaryVersion,
		&component.DictionarySHA256,
	)
	if err == pgx.ErrNoRows {
		return trendFormalProjectComponent{}, fmt.Errorf("published formal trend release is unavailable")
	}
	if err != nil {
		return trendFormalProjectComponent{}, fmt.Errorf("read published project trend release: %w", err)
	}
	return component, nil
}

func readPublishedTrendJobFamilyComponent(ctx context.Context, tx pgx.Tx) (trendFormalJobFamilyComponent, error) {
	component := trendFormalJobFamilyComponent{}
	err := tx.QueryRow(ctx, `
SELECT release.identity_sha256,
       component.parser_version,
       component.dictionary_version,
       component.dictionary_sha256,
       component.project_mask_parser_version,
       component.project_mask_dictionary_version,
       component.project_mask_dictionary_sha256
FROM trend_formal_releases AS release
JOIN trend_formal_release_components AS component
  ON component.trend_formal_release_id = release.id
 AND component.dimension = 'job_family'
WHERE release.status = 'published'`).Scan(
		&component.IdentitySHA256,
		&component.ParserVersion,
		&component.DictionaryVersion,
		&component.DictionarySHA256,
		&component.ProjectMaskParserVersion,
		&component.ProjectMaskDictionaryVersion,
		&component.ProjectMaskDictionarySHA256,
	)
	if err == pgx.ErrNoRows {
		return trendFormalJobFamilyComponent{}, fmt.Errorf("published formal trend release is unavailable")
	}
	if err != nil {
		return trendFormalJobFamilyComponent{}, fmt.Errorf("read published job-family trend release: %w", err)
	}
	return component, nil
}

func readTrendProjectLabel(ctx context.Context, tx pgx.Tx, component trendFormalProjectComponent, value string) (string, error) {
	var label string
	err := tx.QueryRow(ctx, `
SELECT assignment.display_name
FROM project_tag_results AS result
JOIN project_tag_assignments AS assignment ON assignment.project_tag_result_id = result.id
WHERE result.parser_version = $1
  AND result.dictionary_version = $2
  AND result.dictionary_sha256 = $3
  AND result.completed_at IS NOT NULL
  AND assignment.project_key = $4
ORDER BY assignment.display_name
LIMIT 1`, component.ParserVersion, component.DictionaryVersion, component.DictionarySHA256, value).Scan(&label)
	if err == pgx.ErrNoRows {
		return "", ErrInvalidTrendQuery
	}
	if err != nil {
		return "", fmt.Errorf("read trend project label: %w", err)
	}
	return label, nil
}

func readAllLocationTrendPoints(ctx context.Context, tx pgx.Tx, location string, calendarRange trendCalendarRange) ([]TrendPoint, error) {
	rows, err := tx.Query(ctx, `
WITH selected_runs AS (
    SELECT DISTINCT ON (
        run.observation_scope_id,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date
    )
        run.id,
        run.observation_scope_id,
        run.published_at,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date AS calendar_date
    FROM collection_runs AS run
    WHERE run.status = 'published'
      AND run.published_at IS NOT NULL
	  AND ($2::date IS NULL OR (run.published_at AT TIME ZONE 'Asia/Shanghai')::date >= $2::date)
	  AND (run.published_at AT TIME ZONE 'Asia/Shanghai')::date <= $3::date
    ORDER BY
        run.observation_scope_id,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date,
        run.published_at DESC,
        run.id DESC
), selected_runs_with_completeness AS (
    SELECT selected.*,
           NOT EXISTS (
               SELECT 1
               FROM (
                   SELECT DISTINCT ON (presence.job_id) presence.job_id
                   FROM job_presence_periods AS presence
                   WHERE presence.observation_scope_id = selected.observation_scope_id
                     AND presence.opened_at <= selected.published_at
                   ORDER BY presence.job_id, presence.opened_at DESC, presence.id DESC
               ) AS observed_presence
               LEFT JOIN serving_jd_inventory_memberships AS membership
                 ON membership.published_run_id = selected.id
                AND membership.job_id = observed_presence.job_id
               WHERE membership.id IS NULL
           ) AS is_complete
    FROM selected_runs AS selected
), bounds AS (
    SELECT COALESCE($2::date, min(calendar_date)) AS first_date,
           $3::date AS last_date
    FROM selected_runs_with_completeness
), days AS (
    SELECT generated.day::date AS calendar_date
    FROM bounds
    CROSS JOIN LATERAL generate_series(
        bounds.first_date,
        bounds.last_date,
        interval '1 day'
    ) AS generated(day)
)
SELECT to_char(days.calendar_date, 'YYYY-MM-DD') AS calendar_date,
       CASE
           WHEN count(selected.id) = 0 OR NOT bool_and(selected.is_complete) THEN NULL
           ELSE count(DISTINCT membership.job_id) FILTER (
               WHERE membership.activity_state = 'effective'
                 AND membership.locations @> ARRAY[$1]::text[]
           )
       END AS matching_jobs
FROM days
LEFT JOIN selected_runs_with_completeness AS selected
  ON selected.calendar_date = days.calendar_date
LEFT JOIN serving_jd_inventory_memberships AS membership
  ON membership.published_run_id = selected.id
GROUP BY days.calendar_date
ORDER BY days.calendar_date`, location, calendarRange.Start, calendarRange.End)
	if err != nil {
		return nil, fmt.Errorf("select all-history location trend: %w", err)
	}
	defer rows.Close()

	points := make([]TrendPoint, 0)
	for rows.Next() {
		var point TrendPoint
		var count *int64
		if err := rows.Scan(&point.Date, &count); err != nil {
			return nil, fmt.Errorf("scan all-history location trend: %w", err)
		}
		if count != nil {
			if *count > int64(^uint(0)>>1) {
				return nil, fmt.Errorf("location trend count exceeds public integer range")
			}
			value := int(*count)
			point.Count = &value
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate all-history location trend: %w", err)
	}
	return points, nil
}

func readAllEducationTrendPoints(
	ctx context.Context,
	tx pgx.Tx,
	component trendFormalEducationComponent,
	operator TrendComparison,
	value string,
	calendarRange trendCalendarRange,
) ([]TrendPoint, error) {
	rows, err := tx.Query(ctx, `
WITH selected_runs AS (
    SELECT DISTINCT ON (
        run.observation_scope_id,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date
    )
        run.id,
        run.observation_scope_id,
        run.published_at,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date AS calendar_date
    FROM collection_runs AS run
    WHERE run.status = 'published'
      AND run.published_at IS NOT NULL
	  AND ($1::date IS NULL OR (run.published_at AT TIME ZONE 'Asia/Shanghai')::date >= $1::date)
	  AND (run.published_at AT TIME ZONE 'Asia/Shanghai')::date <= $2::date
    ORDER BY
        run.observation_scope_id,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date,
        run.published_at DESC,
        run.id DESC
), selected_runs_with_completeness AS (
    SELECT selected.*,
           NOT EXISTS (
               SELECT 1
               FROM (
                   SELECT DISTINCT ON (presence.job_id) presence.job_id
                   FROM job_presence_periods AS presence
                   WHERE presence.observation_scope_id = selected.observation_scope_id
                     AND presence.opened_at <= selected.published_at
                   ORDER BY presence.job_id, presence.opened_at DESC, presence.id DESC
               ) AS observed_presence
               LEFT JOIN serving_jd_inventory_memberships AS membership
                 ON membership.published_run_id = selected.id
                AND membership.job_id = observed_presence.job_id
               WHERE membership.id IS NULL
           )
           AND NOT EXISTS (
               SELECT 1
               FROM serving_jd_inventory_memberships AS membership
               LEFT JOIN education_requirement_results AS result
                 ON result.body_content_version_id = membership.body_content_version_id
                AND result.parser_version = $3
                AND result.lexicon_version = $4
                AND result.lexicon_sha256 = $5
                AND result.completed_at IS NOT NULL
               WHERE membership.published_run_id = selected.id
                 AND result.id IS NULL
           ) AS is_complete
    FROM selected_runs AS selected
), bounds AS (
    SELECT COALESCE($1::date, min(calendar_date)) AS first_date,
           $2::date AS last_date
    FROM selected_runs_with_completeness
), days AS (
    SELECT generated.day::date AS calendar_date
    FROM bounds
    CROSS JOIN LATERAL generate_series(
        bounds.first_date,
        bounds.last_date,
        interval '1 day'
    ) AS generated(day)
)
SELECT to_char(days.calendar_date, 'YYYY-MM-DD') AS calendar_date,
       CASE
           WHEN count(selected.id) = 0 OR NOT bool_and(selected.is_complete) THEN NULL
           ELSE count(DISTINCT membership.job_id) FILTER (
               WHERE membership.activity_state = 'effective'
                 AND result.status = 'configured'
                 AND result.lower_bound_level IS NOT NULL
                 AND CASE $6::text
                     WHEN 'eq' THEN (
                         CASE result.lower_bound_level::text
                             WHEN 'associate_or_lower' THEN 1
                             WHEN 'bachelor' THEN 2
                             WHEN 'master' THEN 3
                             WHEN 'doctor' THEN 4
                         END
                         = CASE $7::text
                             WHEN 'associate_or_lower' THEN 1
                             WHEN 'bachelor' THEN 2
                             WHEN 'master' THEN 3
                             WHEN 'doctor' THEN 4
                         END
                     )
                     WHEN 'gt' THEN (
                         CASE result.lower_bound_level::text
                             WHEN 'associate_or_lower' THEN 1
                             WHEN 'bachelor' THEN 2
                             WHEN 'master' THEN 3
                             WHEN 'doctor' THEN 4
                         END
                         > CASE $7::text
                             WHEN 'associate_or_lower' THEN 1
                             WHEN 'bachelor' THEN 2
                             WHEN 'master' THEN 3
                             WHEN 'doctor' THEN 4
                         END
                     )
                     WHEN 'gte' THEN (
                         CASE result.lower_bound_level::text
                             WHEN 'associate_or_lower' THEN 1
                             WHEN 'bachelor' THEN 2
                             WHEN 'master' THEN 3
                             WHEN 'doctor' THEN 4
                         END
                         >= CASE $7::text
                             WHEN 'associate_or_lower' THEN 1
                             WHEN 'bachelor' THEN 2
                             WHEN 'master' THEN 3
                             WHEN 'doctor' THEN 4
                         END
                     )
                     WHEN 'lt' THEN (
                         CASE result.lower_bound_level::text
                             WHEN 'associate_or_lower' THEN 1
                             WHEN 'bachelor' THEN 2
                             WHEN 'master' THEN 3
                             WHEN 'doctor' THEN 4
                         END
                         < CASE $7::text
                             WHEN 'associate_or_lower' THEN 1
                             WHEN 'bachelor' THEN 2
                             WHEN 'master' THEN 3
                             WHEN 'doctor' THEN 4
                         END
                     )
                     WHEN 'lte' THEN (
                         CASE result.lower_bound_level::text
                             WHEN 'associate_or_lower' THEN 1
                             WHEN 'bachelor' THEN 2
                             WHEN 'master' THEN 3
                             WHEN 'doctor' THEN 4
                         END
                         <= CASE $7::text
                             WHEN 'associate_or_lower' THEN 1
                             WHEN 'bachelor' THEN 2
                             WHEN 'master' THEN 3
                             WHEN 'doctor' THEN 4
                         END
                     )
                     ELSE false
                 END
           )
       END AS matching_jobs
FROM days
LEFT JOIN selected_runs_with_completeness AS selected
  ON selected.calendar_date = days.calendar_date
LEFT JOIN serving_jd_inventory_memberships AS membership
  ON membership.published_run_id = selected.id
LEFT JOIN education_requirement_results AS result
  ON result.body_content_version_id = membership.body_content_version_id
 AND result.parser_version = $3
 AND result.lexicon_version = $4
 AND result.lexicon_sha256 = $5
 AND result.completed_at IS NOT NULL
GROUP BY days.calendar_date
ORDER BY days.calendar_date`,
		calendarRange.Start,
		calendarRange.End,
		component.ParserVersion,
		component.LexiconVersion,
		component.LexiconSHA256,
		string(operator),
		value,
	)
	if err != nil {
		return nil, fmt.Errorf("select all-history education trend: %w", err)
	}
	defer rows.Close()

	return scanTrendPoints(rows, "education")
}

func readAllExperienceTrendPoints(
	ctx context.Context,
	tx pgx.Tx,
	component trendFormalExperienceComponent,
	operator TrendComparison,
	value int,
	calendarRange trendCalendarRange,
) ([]TrendPoint, error) {
	rows, err := tx.Query(ctx, `
WITH selected_runs AS (
    SELECT DISTINCT ON (
        run.observation_scope_id,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date
    )
        run.id,
        run.observation_scope_id,
        run.published_at,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date AS calendar_date
    FROM collection_runs AS run
    WHERE run.status = 'published'
      AND run.published_at IS NOT NULL
	  AND ($1::date IS NULL OR (run.published_at AT TIME ZONE 'Asia/Shanghai')::date >= $1::date)
	  AND (run.published_at AT TIME ZONE 'Asia/Shanghai')::date <= $2::date
    ORDER BY
        run.observation_scope_id,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date,
        run.published_at DESC,
        run.id DESC
), selected_runs_with_completeness AS (
    SELECT selected.*,
           NOT EXISTS (
               SELECT 1
               FROM (
                   SELECT DISTINCT ON (presence.job_id) presence.job_id
                   FROM job_presence_periods AS presence
                   WHERE presence.observation_scope_id = selected.observation_scope_id
                     AND presence.opened_at <= selected.published_at
                   ORDER BY presence.job_id, presence.opened_at DESC, presence.id DESC
               ) AS observed_presence
               LEFT JOIN serving_jd_inventory_memberships AS membership
                 ON membership.published_run_id = selected.id
                AND membership.job_id = observed_presence.job_id
               WHERE membership.id IS NULL
           )
           AND NOT EXISTS (
               SELECT 1
               FROM serving_jd_inventory_memberships AS membership
               LEFT JOIN experience_requirement_results AS result
                 ON result.body_content_version_id = membership.body_content_version_id
                AND result.parser_version = $3
                AND result.lexicon_version = $4
                AND result.lexicon_sha256 = $5
                AND result.completed_at IS NOT NULL
               WHERE membership.published_run_id = selected.id
                 AND result.id IS NULL
           ) AS is_complete
    FROM selected_runs AS selected
), bounds AS (
    SELECT COALESCE($1::date, min(calendar_date)) AS first_date,
           $2::date AS last_date
    FROM selected_runs_with_completeness
), days AS (
    SELECT generated.day::date AS calendar_date
    FROM bounds
    CROSS JOIN LATERAL generate_series(
        bounds.first_date,
        bounds.last_date,
        interval '1 day'
    ) AS generated(day)
)
SELECT to_char(days.calendar_date, 'YYYY-MM-DD') AS calendar_date,
       CASE
           WHEN count(selected.id) = 0 OR NOT bool_and(selected.is_complete) THEN NULL
           ELSE count(DISTINCT membership.job_id) FILTER (
               WHERE membership.activity_state = 'effective'
                 AND result.status = 'configured'
                 AND result.base_year IS NOT NULL
                 AND CASE $6::text
                     WHEN 'eq' THEN result.base_year = $7::integer
                     WHEN 'gt' THEN result.base_year > $7::integer
                     WHEN 'gte' THEN result.base_year >= $7::integer
                     WHEN 'lt' THEN result.base_year < $7::integer
                     WHEN 'lte' THEN result.base_year <= $7::integer
                     ELSE false
                 END
           )
       END AS matching_jobs
FROM days
LEFT JOIN selected_runs_with_completeness AS selected
  ON selected.calendar_date = days.calendar_date
LEFT JOIN serving_jd_inventory_memberships AS membership
  ON membership.published_run_id = selected.id
LEFT JOIN experience_requirement_results AS result
  ON result.body_content_version_id = membership.body_content_version_id
 AND result.parser_version = $3
 AND result.lexicon_version = $4
 AND result.lexicon_sha256 = $5
 AND result.completed_at IS NOT NULL
GROUP BY days.calendar_date
ORDER BY days.calendar_date`,
		calendarRange.Start,
		calendarRange.End,
		component.ParserVersion,
		component.LexiconVersion,
		component.LexiconSHA256,
		string(operator),
		value,
	)
	if err != nil {
		return nil, fmt.Errorf("select all-history experience trend: %w", err)
	}
	defer rows.Close()

	return scanTrendPoints(rows, "experience")
}

func readAllTechnologyTrendPoints(
	ctx context.Context,
	tx pgx.Tx,
	component trendFormalTechnologyComponent,
	technologyID string,
	calendarRange trendCalendarRange,
) ([]TrendPoint, error) {
	rows, err := tx.Query(ctx, `
WITH selected_runs AS (
    SELECT DISTINCT ON (
        run.observation_scope_id,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date
    )
        run.id,
        run.observation_scope_id,
        run.published_at,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date AS calendar_date
    FROM collection_runs AS run
    WHERE run.status = 'published'
      AND run.published_at IS NOT NULL
	  AND ($1::date IS NULL OR (run.published_at AT TIME ZONE 'Asia/Shanghai')::date >= $1::date)
	  AND (run.published_at AT TIME ZONE 'Asia/Shanghai')::date <= $2::date
    ORDER BY
        run.observation_scope_id,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date,
        run.published_at DESC,
        run.id DESC
), selected_runs_with_completeness AS (
    SELECT selected.*,
           NOT EXISTS (
               SELECT 1
               FROM (
                   SELECT DISTINCT ON (presence.job_id) presence.job_id
                   FROM job_presence_periods AS presence
                   WHERE presence.observation_scope_id = selected.observation_scope_id
                     AND presence.opened_at <= selected.published_at
                   ORDER BY presence.job_id, presence.opened_at DESC, presence.id DESC
               ) AS observed_presence
               LEFT JOIN serving_jd_inventory_memberships AS membership
                 ON membership.published_run_id = selected.id
                AND membership.job_id = observed_presence.job_id
               WHERE membership.id IS NULL
           )
           AND NOT EXISTS (
               SELECT 1
               FROM serving_jd_inventory_memberships AS membership
               LEFT JOIN technology_analysis_results AS result
                 ON result.body_content_version_id = membership.body_content_version_id
                AND result.configuration_id = $3
                AND result.status = 'complete'
                AND result.completed_at IS NOT NULL
               WHERE membership.published_run_id = selected.id
                 AND result.id IS NULL
           ) AS is_complete
    FROM selected_runs AS selected
), bounds AS (
    SELECT COALESCE($1::date, min(calendar_date)) AS first_date,
           $2::date AS last_date
    FROM selected_runs_with_completeness
), days AS (
    SELECT generated.day::date AS calendar_date
    FROM bounds
    CROSS JOIN LATERAL generate_series(
        bounds.first_date,
        bounds.last_date,
        interval '1 day'
    ) AS generated(day)
)
SELECT to_char(days.calendar_date, 'YYYY-MM-DD') AS calendar_date,
       CASE
           WHEN count(selected.id) = 0 OR NOT bool_and(selected.is_complete) THEN NULL
           ELSE count(DISTINCT membership.job_id) FILTER (
               WHERE membership.activity_state = 'effective'
                 AND mention.technology_identity_id = $4
           )
       END AS matching_jobs
FROM days
LEFT JOIN selected_runs_with_completeness AS selected
  ON selected.calendar_date = days.calendar_date
LEFT JOIN serving_jd_inventory_memberships AS membership
  ON membership.published_run_id = selected.id
LEFT JOIN technology_analysis_results AS result
  ON result.body_content_version_id = membership.body_content_version_id
 AND result.configuration_id = $3
 AND result.status = 'complete'
 AND result.completed_at IS NOT NULL
LEFT JOIN technology_mentions AS mention
  ON mention.analysis_result_id = result.id
GROUP BY days.calendar_date
ORDER BY days.calendar_date`, calendarRange.Start, calendarRange.End, component.ConfigurationID, technologyID)
	if err != nil {
		return nil, fmt.Errorf("select all-history technology trend: %w", err)
	}
	defer rows.Close()

	return scanTrendPoints(rows, "technology")
}

func readAllProjectTrendPoints(
	ctx context.Context,
	tx pgx.Tx,
	component trendFormalProjectComponent,
	projectKey string,
	calendarRange trendCalendarRange,
) ([]TrendPoint, error) {
	rows, err := tx.Query(ctx, `
WITH selected_runs AS (
    SELECT DISTINCT ON (
        run.observation_scope_id,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date
    )
        run.id,
        run.observation_scope_id,
        run.published_at,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date AS calendar_date
    FROM collection_runs AS run
    WHERE run.status = 'published'
      AND run.published_at IS NOT NULL
	  AND ($1::date IS NULL OR (run.published_at AT TIME ZONE 'Asia/Shanghai')::date >= $1::date)
	  AND (run.published_at AT TIME ZONE 'Asia/Shanghai')::date <= $2::date
    ORDER BY
        run.observation_scope_id,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date,
        run.published_at DESC,
        run.id DESC
), selected_runs_with_completeness AS (
    SELECT selected.*,
           NOT EXISTS (
               SELECT 1
               FROM (
                   SELECT DISTINCT ON (presence.job_id) presence.job_id
                   FROM job_presence_periods AS presence
                   WHERE presence.observation_scope_id = selected.observation_scope_id
                     AND presence.opened_at <= selected.published_at
                   ORDER BY presence.job_id, presence.opened_at DESC, presence.id DESC
               ) AS observed_presence
               LEFT JOIN serving_jd_inventory_memberships AS membership
                 ON membership.published_run_id = selected.id
                AND membership.job_id = observed_presence.job_id
               WHERE membership.id IS NULL
           )
           AND NOT EXISTS (
               SELECT 1
               FROM serving_jd_inventory_memberships AS membership
               JOIN job_source_metadata_versions AS metadata
                 ON metadata.id = membership.source_metadata_version_id
               LEFT JOIN project_title_observations AS observation
                 ON observation.source_metadata_version_id = metadata.id
               LEFT JOIN project_tag_results AS result
                 ON result.project_title_identity_id = observation.project_title_identity_id
                AND result.parser_version = $3
                AND result.dictionary_version = $4
                AND result.dictionary_sha256 = $5
                AND result.completed_at IS NOT NULL
               WHERE membership.published_run_id = selected.id
                 AND metadata.metadata_payload #>> '{title,state}' = 'string'
                 AND result.id IS NULL
           ) AS is_complete
    FROM selected_runs AS selected
), bounds AS (
    SELECT COALESCE($1::date, min(calendar_date)) AS first_date,
           $2::date AS last_date
    FROM selected_runs_with_completeness
), days AS (
    SELECT generated.day::date AS calendar_date
    FROM bounds
    CROSS JOIN LATERAL generate_series(
        bounds.first_date,
        bounds.last_date,
        interval '1 day'
    ) AS generated(day)
)
SELECT to_char(days.calendar_date, 'YYYY-MM-DD') AS calendar_date,
       CASE
           WHEN count(selected.id) = 0 OR NOT bool_and(selected.is_complete) THEN NULL
           ELSE count(DISTINCT membership.job_id) FILTER (
               WHERE membership.activity_state = 'effective'
                 AND assignment.project_key = $6
           )
       END AS matching_jobs
FROM days
LEFT JOIN selected_runs_with_completeness AS selected
  ON selected.calendar_date = days.calendar_date
LEFT JOIN serving_jd_inventory_memberships AS membership
  ON membership.published_run_id = selected.id
LEFT JOIN job_source_metadata_versions AS metadata
  ON metadata.id = membership.source_metadata_version_id
LEFT JOIN project_title_observations AS observation
  ON observation.source_metadata_version_id = metadata.id
LEFT JOIN project_tag_results AS result
  ON result.project_title_identity_id = observation.project_title_identity_id
 AND result.parser_version = $3
 AND result.dictionary_version = $4
 AND result.dictionary_sha256 = $5
 AND result.completed_at IS NOT NULL
LEFT JOIN project_tag_assignments AS assignment
  ON assignment.project_tag_result_id = result.id
GROUP BY days.calendar_date
ORDER BY days.calendar_date`,
		calendarRange.Start,
		calendarRange.End,
		component.ParserVersion,
		component.DictionaryVersion,
		component.DictionarySHA256,
		projectKey,
	)
	if err != nil {
		return nil, fmt.Errorf("select all-history project trend: %w", err)
	}
	defer rows.Close()

	return scanTrendPoints(rows, "project")
}

func readAllJobFamilyTrendPoints(
	ctx context.Context,
	tx pgx.Tx,
	component trendFormalJobFamilyComponent,
	includeSecondary bool,
	familyKey string,
	calendarRange trendCalendarRange,
) ([]TrendPoint, error) {
	rows, err := tx.Query(ctx, `
WITH selected_runs AS (
    SELECT DISTINCT ON (
        run.observation_scope_id,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date
    )
        run.id,
        run.observation_scope_id,
        run.published_at,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date AS calendar_date
    FROM collection_runs AS run
    WHERE run.status = 'published'
      AND run.published_at IS NOT NULL
	  AND ($1::date IS NULL OR (run.published_at AT TIME ZONE 'Asia/Shanghai')::date >= $1::date)
	  AND (run.published_at AT TIME ZONE 'Asia/Shanghai')::date <= $2::date
    ORDER BY
        run.observation_scope_id,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date,
        run.published_at DESC,
        run.id DESC
), selected_runs_with_completeness AS (
    SELECT selected.*,
           NOT EXISTS (
               SELECT 1
               FROM (
                   SELECT DISTINCT ON (presence.job_id) presence.job_id
                   FROM job_presence_periods AS presence
                   WHERE presence.observation_scope_id = selected.observation_scope_id
                     AND presence.opened_at <= selected.published_at
                   ORDER BY presence.job_id, presence.opened_at DESC, presence.id DESC
               ) AS observed_presence
               LEFT JOIN serving_jd_inventory_memberships AS membership
                 ON membership.published_run_id = selected.id
                AND membership.job_id = observed_presence.job_id
               WHERE membership.id IS NULL
           )
           AND NOT EXISTS (
               SELECT 1
               FROM serving_jd_inventory_memberships AS membership
               JOIN job_source_metadata_versions AS metadata
                 ON metadata.id = membership.source_metadata_version_id
               LEFT JOIN job_family_observations AS observation
                 ON observation.source_metadata_version_id = metadata.id
               LEFT JOIN job_family_results AS result
                 ON result.job_family_title_identity_id = observation.job_family_title_identity_id
                AND result.parser_version = $3
                AND result.dictionary_version = $4
                AND result.dictionary_sha256 = $5
                AND result.project_mask_parser_version = $6
                AND result.project_mask_dictionary_version = $7
                AND result.project_mask_dictionary_sha256 = $8
                AND result.completed_at IS NOT NULL
               WHERE membership.published_run_id = selected.id
                 AND metadata.metadata_payload #>> '{title,state}' = 'string'
                 AND (observation.status IS NULL OR observation.status = 'in_scope')
                 AND result.id IS NULL
           ) AS is_complete
    FROM selected_runs AS selected
), bounds AS (
    SELECT COALESCE($1::date, min(calendar_date)) AS first_date,
           $2::date AS last_date
    FROM selected_runs_with_completeness
), days AS (
    SELECT generated.day::date AS calendar_date
    FROM bounds
    CROSS JOIN LATERAL generate_series(
        bounds.first_date,
        bounds.last_date,
        interval '1 day'
    ) AS generated(day)
)
SELECT to_char(days.calendar_date, 'YYYY-MM-DD') AS calendar_date,
       CASE
           WHEN count(selected.id) = 0 OR NOT bool_and(selected.is_complete) THEN NULL
           ELSE count(DISTINCT membership.job_id) FILTER (
               WHERE membership.activity_state = 'effective'
                 AND (
                     result.primary_family_key = $10
                     OR ($9::boolean AND match.family_key = $10)
                 )
           )
       END AS matching_jobs
FROM days
LEFT JOIN selected_runs_with_completeness AS selected
  ON selected.calendar_date = days.calendar_date
LEFT JOIN serving_jd_inventory_memberships AS membership
  ON membership.published_run_id = selected.id
LEFT JOIN job_source_metadata_versions AS metadata
  ON metadata.id = membership.source_metadata_version_id
LEFT JOIN job_family_observations AS observation
  ON observation.source_metadata_version_id = metadata.id
LEFT JOIN job_family_results AS result
  ON result.job_family_title_identity_id = observation.job_family_title_identity_id
 AND result.parser_version = $3
 AND result.dictionary_version = $4
 AND result.dictionary_sha256 = $5
 AND result.project_mask_parser_version = $6
 AND result.project_mask_dictionary_version = $7
 AND result.project_mask_dictionary_sha256 = $8
 AND result.completed_at IS NOT NULL
LEFT JOIN job_family_matches AS match
  ON match.job_family_result_id = result.id
GROUP BY days.calendar_date
ORDER BY days.calendar_date`,
		calendarRange.Start,
		calendarRange.End,
		component.ParserVersion,
		component.DictionaryVersion,
		component.DictionarySHA256,
		component.ProjectMaskParserVersion,
		component.ProjectMaskDictionaryVersion,
		component.ProjectMaskDictionarySHA256,
		includeSecondary,
		familyKey,
	)
	if err != nil {
		return nil, fmt.Errorf("select all-history job-family trend: %w", err)
	}
	defer rows.Close()

	return scanTrendPoints(rows, "job-family")
}

func scanTrendPoints(rows pgx.Rows, dimension string) ([]TrendPoint, error) {
	points := make([]TrendPoint, 0)
	for rows.Next() {
		var point TrendPoint
		var count *int64
		if err := rows.Scan(&point.Date, &count); err != nil {
			return nil, fmt.Errorf("scan all-history %s trend: %w", dimension, err)
		}
		if count != nil {
			if *count > int64(^uint(0)>>1) {
				return nil, fmt.Errorf("%s trend count exceeds public integer range", dimension)
			}
			value := int(*count)
			point.Count = &value
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate all-history %s trend: %w", dimension, err)
	}
	return points, nil
}

func readTrendFreshnessInput(ctx context.Context, tx pgx.Tx, calendarRange trendCalendarRange) (string, error) {
	var input string
	err := tx.QueryRow(ctx, `
WITH selected_runs AS (
    SELECT DISTINCT ON (
        run.observation_scope_id,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date
    )
        run.id,
        run.observation_scope_id,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date AS calendar_date
    FROM collection_runs AS run
    WHERE run.status = 'published'
      AND run.published_at IS NOT NULL
	  AND ($1::date IS NULL OR (run.published_at AT TIME ZONE 'Asia/Shanghai')::date >= $1::date)
	  AND (run.published_at AT TIME ZONE 'Asia/Shanghai')::date <= $2::date
    ORDER BY
        run.observation_scope_id,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date,
        run.published_at DESC,
        run.id DESC
)
SELECT COALESCE(
    string_agg(
        observation_scope_id::text || ':' || calendar_date::text || ':' || id::text,
        ',' ORDER BY observation_scope_id, calendar_date, id
    ),
    ''
)
FROM selected_runs`, calendarRange.Start, calendarRange.End).Scan(&input)
	if err != nil {
		return "", fmt.Errorf("read trend freshness input: %w", err)
	}
	return input, nil
}

func readTrendCoverage(ctx context.Context, tx pgx.Tx) (TrendCoverage, error) {
	var coverage TrendCoverage
	err := tx.QueryRow(ctx, `
SELECT COALESCE(min((published_at AT TIME ZONE 'Asia/Shanghai')::date)::text, ''),
       COALESCE(max((published_at AT TIME ZONE 'Asia/Shanghai')::date)::text, '')
FROM collection_runs
WHERE status = 'published'
  AND published_at IS NOT NULL`).Scan(&coverage.FirstDate, &coverage.LastDate)
	if err != nil {
		return TrendCoverage{}, fmt.Errorf("read trend coverage: %w", err)
	}
	return coverage, nil
}

func trendFreshness(input string) string {
	digest := sha256.Sum256([]byte(input))
	return hex.EncodeToString(digest[:])
}
