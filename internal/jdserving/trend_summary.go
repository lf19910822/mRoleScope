package jdserving

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
)

// TrendSummary is the bounded, analysis-safe view of a trend curve. It is
// intentionally smaller than TrendSeries and never carries daily points.
type TrendSummary struct {
	Curve                  TrendCurve
	Freshness              string
	AverageDailyActiveJobs *float64
	PeriodEndActiveJobs    *int
	PeakDailyActiveJobs    *TrendSummaryPeak
	PeriodUniqueJobs       *int
	ValidObservationDays   int
	MissingObservationDays int
	Events                 map[TrendEventType]int
}

type TrendSummaryPeak struct {
	Date  string
	Count int
}

// TrendSummaryReader is the safe Serving boundary for AI analysis tools.
type TrendSummaryReader interface {
	ReadTrendSummary(context.Context, TrendQuery) (TrendSummary, error)
}

// SummarizeTrendSeries calculates bounded values from a complete Serving
// series. It deliberately ignores missing points and never sums daily stock.
func SummarizeTrendSeries(series TrendSeries) TrendSummary {
	summary := TrendSummary{
		Curve:     series.Curve,
		Freshness: series.Freshness,
		Events:    make(map[TrendEventType]int),
	}
	var total int64
	var peak *TrendSummaryPeak
	for _, point := range series.Points {
		if point.Count == nil {
			summary.MissingObservationDays++
			continue
		}
		summary.ValidObservationDays++
		total += int64(*point.Count)
		if peak == nil || *point.Count > peak.Count || (*point.Count == peak.Count && point.Date < peak.Date) {
			peak = &TrendSummaryPeak{Date: point.Date, Count: *point.Count}
		}
		summary.PeriodEndActiveJobs = intPointer(*point.Count)
	}
	if summary.ValidObservationDays > 0 {
		average := float64(total) / float64(summary.ValidObservationDays)
		summary.AverageDailyActiveJobs = &average
	}
	if peak != nil {
		summary.PeakDailyActiveJobs = peak
	}
	return summary
}

// ReadTrendSummary reuses the existing historical trend and event seams while
// keeping period-wide distinct-job aggregation behind the Serving boundary.
func (repository *Repository) ReadTrendSummary(ctx context.Context, query TrendQuery) (TrendSummary, error) {
	series, err := repository.ReadTrend(ctx, query)
	if err != nil {
		return TrendSummary{}, err
	}
	summary := SummarizeTrendSeries(series)
	unique, err := repository.readTrendPeriodUniqueJobs(ctx, query)
	if err != nil {
		return TrendSummary{}, err
	}
	summary.PeriodUniqueJobs = unique
	if query.Range != TrendRangeAll {
		events, err := repository.ReadTrendEventStats(ctx, TrendEventStatsQuery{
			Condition: query.Condition,
			StartDate: query.StartDate,
			EndDate:   query.EndDate,
			Range:     query.Range,
		})
		if err != nil {
			return TrendSummary{}, err
		}
		for _, row := range events.Rows {
			summary.Events[row.Type] = row.Current
		}
	}
	return summary, nil
}

// readTrendPeriodUniqueJobs counts distinct effective jobs across the same
// complete published calendar dates used by ReadTrend. It stays behind the
// Serving seam and uses the existing immutable condition filter.
func (repository *Repository) readTrendPeriodUniqueJobs(ctx context.Context, query TrendQuery) (*int, error) {
	calendarRange, err := resolveTrendCalendarRange(query)
	if err != nil {
		return nil, err
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin trend unique read: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	filter, err := buildTrendDrilldownFilter(ctx, tx, query.Condition)
	if err != nil {
		return nil, err
	}
	formalJoin := strings.ReplaceAll(shiftSQLPlaceholders(filter.completenessJoins, 1), "membership.", "completeness_membership.")
	formalIncomplete := strings.ReplaceAll(shiftSQLPlaceholders(filter.formalIncompleteSQL, 1), "membership.", "completeness_membership.")
	filterOffset := 1 + len(filter.completenessArgs)
	joins := shiftSQLPlaceholders(filter.joins, filterOffset)
	predicate := shiftSQLPlaceholders(filter.predicate, filterOffset)
	queryText := `
WITH selected_runs AS (
    SELECT DISTINCT ON (run.observation_scope_id, (run.published_at AT TIME ZONE 'Asia/Shanghai')::date)
        run.id, run.observation_scope_id, run.published_at,
        (run.published_at AT TIME ZONE 'Asia/Shanghai')::date AS calendar_date
    FROM collection_runs AS run
    WHERE run.status = 'published' AND run.published_at IS NOT NULL
      AND ($1::date IS NULL OR (run.published_at AT TIME ZONE 'Asia/Shanghai')::date >= $1::date)
      AND (run.published_at AT TIME ZONE 'Asia/Shanghai')::date <= $2::date
    ORDER BY run.observation_scope_id, (run.published_at AT TIME ZONE 'Asia/Shanghai')::date,
             run.published_at DESC, run.id DESC
), source_runs AS (
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
               LEFT JOIN serving_jd_inventory_memberships AS observed_membership
                 ON observed_membership.published_run_id = selected.id
                AND observed_membership.job_id = observed_presence.job_id
               WHERE observed_membership.id IS NULL
           ) AS source_complete
    FROM selected_runs AS selected
), formal_runs AS (
    SELECT selected.*, NOT EXISTS (
        SELECT 1 FROM serving_jd_inventory_memberships AS completeness_membership
        ` + formalJoin + `
        WHERE completeness_membership.published_run_id = selected.id
          AND ` + falseWhenBlank(formalIncomplete) + `
    ) AS formal_complete
    FROM source_runs AS selected
), trusted_runs AS (
    SELECT * FROM formal_runs WHERE source_complete AND formal_complete
)
SELECT count(DISTINCT membership.job_id)
FROM trusted_runs AS selected
JOIN serving_jd_inventory_memberships AS membership ON membership.published_run_id = selected.id
` + strings.ReplaceAll(joins, "membership.", "membership.") + `
WHERE membership.activity_state = 'effective' AND (` + falseWhenBlank(predicate) + `)`
	args := []any{nil, calendarRange.End}
	if calendarRange.Start != nil {
		args[0] = *calendarRange.Start
	}
	args = append(args, filter.completenessArgs...)
	args = append(args, filter.args...)
	var count int
	if err := tx.QueryRow(ctx, queryText, args...).Scan(&count); err != nil {
		return nil, fmt.Errorf("read trend unique jobs: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit trend unique read: %w", err)
	}
	return &count, nil
}

func intPointer(value int) *int {
	return &value
}

// SummaryMetricValue returns a finite metric value for ranking. Metrics that
// require a period-wide distinct-job aggregation remain unavailable until the
// dedicated Serving aggregator supplies them.
func (summary TrendSummary) SummaryMetricValue(metric string) (float64, bool) {
	switch metric {
	case "avg_daily_active_jobs":
		if summary.AverageDailyActiveJobs == nil {
			return 0, false
		}
		return *summary.AverageDailyActiveJobs, true
	case "period_end_active_jobs":
		if summary.PeriodEndActiveJobs == nil {
			return 0, false
		}
		return float64(*summary.PeriodEndActiveJobs), true
	case "peak_daily_active_jobs":
		if summary.PeakDailyActiveJobs == nil {
			return 0, false
		}
		return float64(summary.PeakDailyActiveJobs.Count), true
	case "period_unique_jobs":
		if summary.PeriodUniqueJobs == nil {
			return 0, false
		}
		return float64(*summary.PeriodUniqueJobs), true
	case "entered_jobs", "modified_jobs", "left_jobs", "reopened_jobs":
		eventType := TrendEventType(metric[:len(metric)-len("_jobs")])
		value, ok := summary.Events[eventType]
		return float64(value), ok
	default:
		return math.NaN(), false
	}
}
