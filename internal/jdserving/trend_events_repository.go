package jdserving

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const trendEventObservationCTE = `
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
), trusted_runs AS (
    SELECT selected.*
    FROM source_runs AS selected
    WHERE selected.source_complete
      AND NOT EXISTS (
          SELECT 1
          FROM source_runs AS incomplete
          WHERE incomplete.calendar_date = selected.calendar_date
            AND NOT incomplete.source_complete
      )
), snapshot AS (
    SELECT DISTINCT ON (membership.job_id, selected.calendar_date)
        membership.job_id::text AS job_id,
        selected.calendar_date,
        selected.id AS published_run_id,
        membership.activity_state::text AS activity_state,
        membership.source_metadata_version_id::text AS source_metadata_version_id,
        membership.body_content_version_id::text AS body_content_version_id,
        membership.title,
        selected.published_at,
        selected.id AS run_id
    FROM __TREND_EVENT_RUN_SOURCE__ AS selected
    JOIN serving_jd_inventory_memberships AS membership
      ON membership.published_run_id = selected.id
    ORDER BY membership.job_id, selected.calendar_date, selected.published_at DESC, selected.id DESC
)`

func (repository *Repository) ReadTrendEventStats(ctx context.Context, query TrendEventStatsQuery) (TrendEventStatsResponse, error) {
	if repository == nil || repository.pool == nil {
		return TrendEventStatsResponse{}, fmt.Errorf("serving repository is unavailable")
	}
	if query.Range == TrendRangeAll || !supportedTrendEventDimension(query.Condition.Dimension) {
		return TrendEventStatsResponse{}, ErrInvalidTrendQuery
	}
	if err := validateTrendCondition(query.Condition); err != nil {
		return TrendEventStatsResponse{}, err
	}

	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return TrendEventStatsResponse{}, fmt.Errorf("begin trend event read: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	today := normalizeTrendEventDate(time.Now().In(shanghaiLocation()))
	filter, curve, err := repository.buildTrendEventFilter(ctx, tx, query.Condition)
	if err != nil {
		return TrendEventStatsResponse{}, err
	}
	bounds, err := readTrendEventObservationBounds(ctx, tx, filter, today)
	if err != nil {
		return TrendEventStatsResponse{}, err
	}
	latest := bounds.Latest
	if latest.IsZero() {
		latest = today
	}
	periods, err := resolveTrendEventPeriods(query, today, latest)
	if err != nil {
		return TrendEventStatsResponse{}, err
	}
	freshnessInput, err := readTrendEventFreshnessInput(ctx, tx, filter, periods)
	if err != nil {
		return TrendEventStatsResponse{}, err
	}
	formalCoverage, err := readTrendEventFormalCoverage(ctx, tx, filter, periods, bounds.First)
	if err != nil {
		return TrendEventStatsResponse{}, err
	}
	baseline := bounds.First
	observationWindow := trendEventObservationWindow(periods)
	var observations []trendEventObservation
	if !baseline.IsZero() && !observationWindow.End.Before(baseline) {
		observationStart := observationWindow.Start
		if observationStart.Before(baseline) {
			observationStart = baseline
		}
		observations, err = readTrendEventObservations(ctx, tx, observationStart, observationWindow.End, filter)
	}
	if err != nil {
		return TrendEventStatsResponse{}, err
	}
	events := classifyTrendEventObservations(observations, baseline)
	current := aggregateTrendEvents(events, periods.Current)
	previous := aggregateTrendEvents(events, periods.Previous)
	yearOverYear := aggregateTrendEvents(events, periods.YearOverYear)

	response := TrendEventStatsResponse{
		AsOf:         periods.CurrentAsOf,
		Current:      trendEventPeriodFromWindow(periods.Current),
		Curve:        curve,
		Freshness:    trendEventFreshness(query, periods, latest, filter, freshnessInput),
		Previous:     trendEventPeriodFromWindow(periods.Previous),
		Rows:         trendEventRows(current, previous, yearOverYear),
		Status:       "available",
		YearOverYear: trendEventPeriodFromWindow(periods.YearOverYear),
	}
	if formalCoverage.Incomplete {
		response.Status = "unavailable"
	}
	if err := tx.Commit(ctx); err != nil {
		return TrendEventStatsResponse{}, fmt.Errorf("commit trend event read: %w", err)
	}
	return response, nil
}

func trendEventFreshness(query TrendEventStatsQuery, periods trendEventPeriods, latest time.Time, filter trendEventFilter, observationInput string) string {
	parts := []string{
		"event-v1",
		observationInput,
		formatTrendEventDate(latest),
		string(query.Range),
		string(query.Condition.Dimension),
		query.Condition.Value,
		string(query.Condition.Operator),
		fmt.Sprintf("%t", query.Condition.IncludeSecondary),
		formatTrendEventDate(periods.Current.Start),
		formatTrendEventDate(periods.Current.End),
		formatTrendEventDate(periods.Previous.Start),
		formatTrendEventDate(periods.Previous.End),
		formatTrendEventDate(periods.YearOverYear.Start),
		formatTrendEventDate(periods.YearOverYear.End),
	}
	for _, argument := range append(filter.args, filter.completenessArgs...) {
		parts = append(parts, fmt.Sprintf("%T:%v", argument, argument))
	}
	return trendFreshness(strings.Join(parts, "\x00"))
}

func readTrendEventFreshnessInput(ctx context.Context, tx pgx.Tx, filter trendEventFilter, periods trendEventPeriods) (string, error) {
	start := periods.Current.Start
	if periods.Previous.Start.Before(start) {
		start = periods.Previous.Start
	}
	if periods.YearOverYear.Start.Before(start) {
		start = periods.YearOverYear.Start
	}
	end := periods.Current.End
	if periods.Previous.End.After(end) {
		end = periods.Previous.End
	}
	if periods.YearOverYear.End.After(end) {
		end = periods.YearOverYear.End
	}
	args := []any{start.Format("2006-01-02"), end.Format("2006-01-02")}
	args = append(args, filter.completenessArgs...)
	query := trendEventObservationSQL(filter) + `
SELECT COALESCE(
    string_agg(
        observation_scope_id::text || ':' || calendar_date::text || ':' || id::text,
        ',' ORDER BY observation_scope_id, calendar_date, id
    ),
    ''
)
FROM ` + trendEventRunSource(filter)
	query += `
WHERE (` + trendEventWindowSQL("calendar_date", periods) + `)`
	var input string
	if err := tx.QueryRow(ctx, query, args...).Scan(&input); err != nil {
		return "", fmt.Errorf("read trend event freshness: %w", err)
	}
	return input, nil
}

func supportedTrendEventDimension(dimension TrendDimension) bool {
	switch dimension {
	case TrendDimensionTechnology, TrendDimensionJobFamily, TrendDimensionProject, TrendDimensionLocation:
		return true
	default:
		return false
	}
}

func validateTrendCondition(condition TrendCondition) error {
	switch condition.Dimension {
	case TrendDimensionTechnology:
		_, err := validateTechnologyTrendQuery(TrendQuery{Condition: condition})
		return err
	case TrendDimensionJobFamily:
		_, _, err := validateJobFamilyTrendQuery(TrendQuery{Condition: condition})
		return err
	case TrendDimensionProject:
		_, err := validateProjectTrendQuery(TrendQuery{Condition: condition})
		return err
	case TrendDimensionLocation:
		return validateLocationTrendQuery(TrendQuery{Condition: condition})
	default:
		return ErrInvalidTrendQuery
	}
}

type trendEventFilter struct {
	args              []any
	joins             string
	predicate         string
	completenessArgs  []any
	completenessJoins string
	formalIncomplete  string
}

func (repository *Repository) buildTrendEventFilter(ctx context.Context, tx pgx.Tx, condition TrendCondition) (trendEventFilter, TrendCurve, error) {
	filter, err := buildTrendDrilldownFilter(ctx, tx, condition)
	if err != nil {
		return trendEventFilter{}, TrendCurve{}, err
	}
	curve := TrendCurve{Dimension: condition.Dimension, Value: condition.Value, IncludeSecondary: condition.IncludeSecondary, Operator: condition.Operator}
	switch condition.Dimension {
	case TrendDimensionTechnology:
		value, label, _, err := readTrendTechnologyIdentity(ctx, tx, TrendQuery{Condition: condition})
		if err != nil {
			return trendEventFilter{}, TrendCurve{}, err
		}
		curve.Value, curve.Label = value, label
	case TrendDimensionJobFamily:
		value, label, err := validateJobFamilyTrendQuery(TrendQuery{Condition: condition})
		if err != nil {
			return trendEventFilter{}, TrendCurve{}, err
		}
		curve.Value, curve.Label = value, label
	case TrendDimensionProject:
		value, err := validateProjectTrendQuery(TrendQuery{Condition: condition})
		if err != nil {
			return trendEventFilter{}, TrendCurve{}, err
		}
		component, err := readPublishedTrendProjectComponent(ctx, tx)
		if err != nil {
			return trendEventFilter{}, TrendCurve{}, err
		}
		label, err := readTrendProjectLabel(ctx, tx, component, value)
		if err != nil {
			return trendEventFilter{}, TrendCurve{}, err
		}
		curve.Value, curve.Label = value, label
	case TrendDimensionLocation:
		curve.Label = condition.Value
	}
	return normalizeTrendEventFilter(filter), curve, nil
}

func normalizeTrendEventFilter(filter trendDrilldownFilter) trendEventFilter {
	valueOffset := 1 + len(filter.completenessArgs)
	return trendEventFilter{
		args:              filter.args,
		joins:             shiftSQLPlaceholders(filter.joins, valueOffset),
		predicate:         shiftSQLPlaceholders(filter.predicate, valueOffset),
		completenessArgs:  filter.completenessArgs,
		completenessJoins: shiftSQLPlaceholders(filter.completenessJoins, 1),
		formalIncomplete:  shiftSQLPlaceholders(filter.formalIncompleteSQL, 1),
	}
}

func trendEventObservationSQL(filter trendEventFilter) string {
	runSource := "trusted_runs"
	if strings.TrimSpace(filter.completenessJoins) == "" || strings.TrimSpace(filter.formalIncomplete) == "" {
		return strings.Replace(trendEventObservationCTE, "__TREND_EVENT_RUN_SOURCE__", runSource, 1)
	}
	statement := strings.Replace(trendEventObservationCTE, `), snapshot AS (`, `), formal_runs AS (
    SELECT selected.*,
           NOT EXISTS (
               SELECT 1
               FROM serving_jd_inventory_memberships AS membership
               `+filter.completenessJoins+`
               WHERE membership.published_run_id = selected.id
                 AND `+filter.formalIncomplete+`
           ) AS formal_complete
    FROM trusted_runs AS selected
), trusted_formal_runs AS (
    SELECT selected.*
    FROM formal_runs AS selected
    WHERE selected.formal_complete
      AND NOT EXISTS (
          SELECT 1
          FROM formal_runs AS incomplete
          WHERE incomplete.calendar_date = selected.calendar_date
            AND NOT incomplete.formal_complete
      )
), snapshot AS (`, 1)
	runSource = "trusted_formal_runs"
	return strings.Replace(statement, "__TREND_EVENT_RUN_SOURCE__", runSource, 1)
}

func trendEventRunSource(filter trendEventFilter) string {
	if strings.TrimSpace(filter.completenessJoins) != "" && strings.TrimSpace(filter.formalIncomplete) != "" {
		return "trusted_formal_runs"
	}
	return "trusted_runs"
}

type trendEventObservationBounds struct {
	First  time.Time
	Latest time.Time
}

func readTrendEventObservationBounds(ctx context.Context, tx pgx.Tx, filter trendEventFilter, end time.Time) (trendEventObservationBounds, error) {
	var first, latest *time.Time
	args := []any{nil, end.Format("2006-01-02")}
	args = append(args, filter.completenessArgs...)
	err := tx.QueryRow(ctx, trendEventObservationSQL(filter)+`
SELECT min(calendar_date), max(calendar_date) FROM `+trendEventRunSource(filter), args...).Scan(&first, &latest)
	if err != nil {
		return trendEventObservationBounds{}, fmt.Errorf("read trend event observation bounds: %w", err)
	}
	result := trendEventObservationBounds{}
	if first != nil {
		result.First = normalizeTrendEventDate(*first)
	}
	if latest != nil {
		result.Latest = normalizeTrendEventDate(*latest)
	}
	return result, nil
}

type trendEventFormalCoverage struct {
	Incomplete bool
}

func readTrendEventFormalCoverage(ctx context.Context, tx pgx.Tx, filter trendEventFilter, periods trendEventPeriods, baseline time.Time) (trendEventFormalCoverage, error) {
	if strings.TrimSpace(filter.completenessJoins) == "" || strings.TrimSpace(filter.formalIncomplete) == "" {
		return trendEventFormalCoverage{}, nil
	}
	baselineParameter := 3 + len(filter.completenessArgs)
	query := trendEventFormalCoverageSQL(filter, periods, baselineParameter)
	baselineDate := baseline
	if baselineDate.IsZero() {
		baselineDate = periods.Current.End
	}
	args := []any{nil, periods.Current.End.Format("2006-01-02")}
	args = append(args, filter.completenessArgs...)
	args = append(args, baselineDate.Format("2006-01-02"))
	var coverage trendEventFormalCoverage
	if err := tx.QueryRow(ctx, query, args...).Scan(&coverage.Incomplete); err != nil {
		return trendEventFormalCoverage{}, fmt.Errorf("read trend event formal completeness: %w", err)
	}
	return coverage, nil
}

func trendEventFormalCoverageSQL(filter trendEventFilter, periods trendEventPeriods, baselineParameter int) string {
	return trendEventObservationSQL(filter) + `
SELECT EXISTS (
    SELECT 1
    FROM trusted_runs AS source
    WHERE source.calendar_date >= $` + strconv.Itoa(baselineParameter) + `::date
      AND (` + trendEventWindowSQL("source.calendar_date", periods) + `)
      AND NOT EXISTS (
          SELECT 1
          FROM trusted_formal_runs AS formal
          WHERE formal.calendar_date = source.calendar_date
      )
)`
}

func trendEventWindowSQL(column string, periods trendEventPeriods) string {
	return fmt.Sprintf(
		"%s BETWEEN DATE '%s' AND DATE '%s' OR %s BETWEEN DATE '%s' AND DATE '%s' OR %s BETWEEN DATE '%s' AND DATE '%s'",
		column,
		formatTrendEventDate(periods.Current.Start), formatTrendEventDate(periods.Current.End),
		column,
		formatTrendEventDate(periods.Previous.Start), formatTrendEventDate(periods.Previous.End),
		column,
		formatTrendEventDate(periods.YearOverYear.Start), formatTrendEventDate(periods.YearOverYear.End),
	)
}

func readTrendEventObservations(ctx context.Context, tx pgx.Tx, start, end time.Time, filter trendEventFilter) ([]trendEventObservation, error) {
	observationSQL := trendEventObservationSQL(filter)
	args := []any{start.Format("2006-01-02"), end.Format("2006-01-02")}
	args = append(args, filter.completenessArgs...)
	rows, err := tx.Query(ctx, observationSQL+`
SELECT job_id, calendar_date, activity_state, source_metadata_version_id,
       body_content_version_id, title
FROM snapshot
ORDER BY job_id, calendar_date`, args...)
	if err != nil {
		return nil, fmt.Errorf("read trend event observations: %w", err)
	}
	defer rows.Close()
	type row struct {
		jobID, state, metadata, body, title string
		date                                time.Time
	}
	all := make([]row, 0)
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.jobID, &item.date, &item.state, &item.metadata, &item.body, &item.title); err != nil {
			return nil, fmt.Errorf("scan trend event observation: %w", err)
		}
		all = append(all, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trend event observations: %w", err)
	}

	matchQuery := observationSQL + `
SELECT DISTINCT snapshot.job_id, snapshot.calendar_date
FROM snapshot
JOIN serving_jd_inventory_memberships AS membership
  ON membership.published_run_id = snapshot.published_run_id
 AND membership.job_id::text = snapshot.job_id
` + filter.joins + `
WHERE membership.activity_state = 'effective' AND (` + filter.predicate + `)`
	args = []any{start.Format("2006-01-02"), end.Format("2006-01-02")}
	args = append(args, filter.completenessArgs...)
	args = append(args, filter.args...)
	matchRows, err := tx.Query(ctx, matchQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("read trend event curve matches: %w", err)
	}
	defer matchRows.Close()
	matches := make(map[string]struct{})
	for matchRows.Next() {
		var jobID string
		var date time.Time
		if err := matchRows.Scan(&jobID, &date); err != nil {
			return nil, fmt.Errorf("scan trend event curve match: %w", err)
		}
		matches[jobID+"\x00"+formatTrendEventDate(date)] = struct{}{}
	}
	if err := matchRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trend event curve matches: %w", err)
	}
	observations := make([]trendEventObservation, 0, len(all))
	for _, item := range all {
		observations = append(observations, trendEventObservation{
			ActivityState:           ActivityState(item.state),
			BodyContentVersionID:    item.body,
			SourceMetadataVersionID: item.metadata,
			Date:                    normalizeTrendEventDate(item.date),
			JobID:                   item.jobID,
			Title:                   item.title,
			Matches: func() bool {
				_, ok := matches[item.jobID+"\x00"+formatTrendEventDate(item.date)]
				return ok
			}(),
			Modified: false,
		})
	}
	return observations, nil
}

func classifyTrendEventObservations(observations []trendEventObservation, baseline time.Time) []trendEvent {
	byJob := make(map[string][]trendEventObservation)
	for _, observation := range observations {
		if !baseline.IsZero() && observation.Date.Before(baseline) {
			continue
		}
		byJob[observation.JobID] = append(byJob[observation.JobID], observation)
	}
	events := make([]trendEvent, 0)
	for jobID, history := range byJob {
		sort.SliceStable(history, func(left, right int) bool { return history[left].Date.Before(history[right].Date) })
		for index := range history {
			if index > 0 {
				history[index].Modified = historySourceChanged(historyObservation{
					BodyContentVersionID:    history[index-1].BodyContentVersionID,
					SourceMetadataVersionID: history[index-1].SourceMetadataVersionID,
				}, historyObservation{
					BodyContentVersionID:    history[index].BodyContentVersionID,
					SourceMetadataVersionID: history[index].SourceMetadataVersionID,
				})
			}
			var previous trendEventObservation
			if index > 0 {
				previous = history[index-1]
			}
			eventType, reason, ok := classifyTrendEvent(previous, history[index], index == 0, baseline)
			if ok {
				events = append(events, trendEvent{Date: history[index].Date, JobID: jobID, Reason: reason, Type: eventType})
			}
		}
	}
	sortTrendEvents(events)
	return events
}

func (repository *Repository) readTrendEventDrilldownPage(ctx context.Context, tx pgx.Tx, query TrendDrilldownQuery, filter trendDrilldownFilter) (TrendDrilldownResponse, error) {
	if query.EventStartDate.After(query.EventEndDate) {
		return TrendDrilldownResponse{}, ErrInvalidTrendQuery
	}
	eventFilter := normalizeTrendEventFilter(filter)
	bounds, err := readTrendEventObservationBounds(ctx, tx, eventFilter, query.EventEndDate)
	if err != nil {
		return TrendDrilldownResponse{}, err
	}
	var observations []trendEventObservation
	if !bounds.First.IsZero() && !query.EventEndDate.Before(bounds.First) {
		observations, err = readTrendEventObservations(ctx, tx, bounds.First, query.EventEndDate, eventFilter)
	}
	if err != nil {
		return TrendDrilldownResponse{}, err
	}
	events := classifyTrendEventObservations(observations, bounds.First)
	selected := make(map[string]trendEvent)
	for _, event := range events {
		if event.Type != query.EventType || event.Date.Before(query.EventStartDate) || event.Date.After(query.EventEndDate) {
			continue
		}
		if previous, ok := selected[event.JobID]; !ok || event.Date.Before(previous.Date) {
			selected[event.JobID] = event
		}
	}

	byID := make(map[string]trendEventObservation)
	for _, observation := range observations {
		if event, ok := selected[observation.JobID]; ok && observation.Date.Equal(event.Date) {
			byID[observation.JobID] = observation
		}
	}
	jobIDs := make([]string, 0, len(selected))
	for jobID := range selected {
		jobIDs = append(jobIDs, jobID)
	}
	sort.Strings(jobIDs)
	currentRows := make(map[string]struct {
		publicID string
		state    string
		metadata string
		body     string
	})
	if len(jobIDs) > 0 {
		rows, err := tx.Query(ctx, `
SELECT identifier.job_id::text,
       identifier.public_id::text,
       current.activity_state::text,
       current.source_metadata_version_id::text,
       current.body_content_version_id::text
FROM serving_jd_public_identifiers AS identifier
LEFT JOIN LATERAL (
    SELECT current.*
    FROM serving_jd_current_inventory_memberships AS current
    JOIN collection_runs AS current_run ON current_run.id = current.published_run_id
    WHERE current.job_id = identifier.job_id
    ORDER BY current_run.published_at DESC, current.published_run_id DESC
    LIMIT 1
) AS current ON true
WHERE identifier.job_id::text = ANY($1::text[])`, jobIDs)
		if err != nil {
			return TrendDrilldownResponse{}, fmt.Errorf("read trend event current status: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var jobID, publicID string
			var state, metadata, body *string
			if err := rows.Scan(&jobID, &publicID, &state, &metadata, &body); err != nil {
				return TrendDrilldownResponse{}, fmt.Errorf("scan trend event current status: %w", err)
			}
			currentRows[jobID] = struct {
				publicID string
				state    string
				metadata string
				body     string
			}{publicID: publicID, state: dereferenceString(state), metadata: dereferenceString(metadata), body: dereferenceString(body)}
		}
		if err := rows.Err(); err != nil {
			return TrendDrilldownResponse{}, fmt.Errorf("iterate trend event current status: %w", err)
		}
	}

	items := make([]TrendDrilldownItem, 0, len(selected))
	reasonCounts := make(map[string]int)
	for _, jobID := range jobIDs {
		event := selected[jobID]
		observation := byID[jobID]
		if query.Title != "" && !trendEventTitleMatches(observation.Title, query.Title) {
			continue
		}
		current := currentRows[jobID]
		status := trendEventDrilldownItemStatus(
			event.Type,
			current.state,
			current.metadata,
			current.body,
			observation,
		)
		canViewLatest := current.publicID != "" && current.state != "" && current.state != string(ActivityStateConfirmedClosed)
		items = append(items, TrendDrilldownItem{
			CanViewHistorical: true,
			CanViewLatest:     canViewLatest,
			EventDate:         formatTrendEventDate(event.Date),
			ID:                current.publicID,
			Reason:            publicTrendEventReason(event.Reason),
			Status:            status,
			Title:             observation.Title,
		})
		reasonCounts[publicTrendEventReason(event.Reason)]++
	}
	sort.SliceStable(items, func(left, right int) bool {
		if strings.ToLower(items[left].Title) != strings.ToLower(items[right].Title) {
			return strings.ToLower(items[left].Title) < strings.ToLower(items[right].Title)
		}
		return items[left].ID < items[right].ID
	})
	matchedTotal := len(items)
	start := (query.PageNumber - 1) * query.PageSize
	if start > matchedTotal {
		start = matchedTotal
	}
	end := start + query.PageSize
	if end > matchedTotal {
		end = matchedTotal
	}
	pageItems := items[start:end]
	response := TrendDrilldownResponse{
		CountMatchesExpected: nil,
		EventType:            query.EventType,
		Items:                pageItems,
		MatchedTotal:         matchedTotal,
		Mode:                 "event",
		Page:                 PageInfo{Number: query.PageNumber, PageSize: query.PageSize, Total: matchedTotal},
		PeriodEnd:            formatTrendEventDate(query.EventEndDate),
		PeriodStart:          formatTrendEventDate(query.EventStartDate),
		ReasonCounts:         reasonCounts,
	}
	if query.Title == "" {
		matchesExpected := matchedTotal == query.ExpectedCount
		response.CountMatchesExpected = &matchesExpected
	}
	return response, nil
}

func trendEventDrilldownItemStatus(
	eventType TrendEventType,
	currentState string,
	currentMetadata string,
	currentBody string,
	observation trendEventObservation,
) TrendDrilldownStatus {
	if currentState == string(ActivityStateConfirmedClosed) || currentState == "" {
		return TrendDrilldownStatusClosed
	}
	if eventType == TrendEventModified ||
		currentMetadata != observation.SourceMetadataVersionID || currentBody != observation.BodyContentVersionID {
		return TrendDrilldownStatusChanged
	}
	return TrendDrilldownStatusCurrent
}

func publicTrendEventReason(reason TrendEventReason) string {
	switch reason {
	case TrendEventReasonBaseline:
		return "历史基线进入"
	case TrendEventReasonFirstObserved:
		return "首次观察进入"
	case TrendEventReasonModifiedEntered:
		return "修改后进入"
	case TrendEventReasonInsideModified:
		return "曲线内修改"
	case TrendEventReasonModifiedLeft:
		return "修改后移出条件"
	case TrendEventReasonConfirmedClosed:
		return "确认下线"
	case TrendEventReasonReopened:
		return "重新上线"
	default:
		return "事件"
	}
}

func trendEventTitleMatches(title, query string) bool {
	title = strings.ToLower(strings.TrimSpace(title))
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" || strings.Contains(title, query) {
		return true
	}
	return trendEventTrigramSimilarity(title, query) >= fuzzyTitleSimilarityThreshold
}

func trendEventTrigramSimilarity(left, right string) float64 {
	leftTrigrams := trendEventTrigrams(left)
	rightTrigrams := trendEventTrigrams(right)
	if len(leftTrigrams) == 0 || len(rightTrigrams) == 0 {
		return 0
	}
	common := 0
	for trigram := range leftTrigrams {
		if _, ok := rightTrigrams[trigram]; ok {
			common++
		}
	}
	return float64(common) / float64(len(leftTrigrams)+len(rightTrigrams)-common)
}

func trendEventTrigrams(value string) map[string]struct{} {
	trigrams := make(map[string]struct{})
	runes := []rune("  " + value + " ")
	for index := 0; index+3 <= len(runes); index++ {
		trigrams[string(runes[index:index+3])] = struct{}{}
	}
	return trigrams
}

func dereferenceString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
