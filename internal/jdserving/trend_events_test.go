package jdserving

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func trendEventDate(year int, month time.Month, day int) time.Time {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return time.Date(year, month, day, 0, 0, 0, 0, location)
}

func TestResolveTrendEventPeriodsUsesFinalObservationCutoff(t *testing.T) {
	query := TrendEventStatsQuery{Range: TrendRangeMonth}
	periods, err := resolveTrendEventPeriods(query, trendEventDate(2026, time.November, 12), trendEventDate(2026, time.November, 11))
	if err != nil {
		t.Fatalf("resolve event periods: %v", err)
	}
	assertTrendEventWindow(t, periods.Current, "2026-11-01", "2026-11-11")
	assertTrendEventWindow(t, periods.Previous, "2026-10-01", "2026-10-11")
	assertTrendEventWindow(t, periods.YearOverYear, "2025-11-01", "2025-11-11")
}

func TestResolveTrendEventPeriodsSupportsNaturalAndCustomWindows(t *testing.T) {
	tests := []struct {
		name     string
		query    TrendEventStatsQuery
		current  [2]string
		previous [2]string
		yoy      [2]string
	}{
		{
			name:     "week",
			query:    TrendEventStatsQuery{Range: TrendRangeWeek},
			current:  [2]string{"2026-11-09", "2026-11-12"},
			previous: [2]string{"2026-11-05", "2026-11-08"},
			yoy:      [2]string{"2025-11-10", "2025-11-13"},
		},
		{
			name:     "quarter",
			query:    TrendEventStatsQuery{Range: TrendRangeQuarter},
			current:  [2]string{"2026-10-01", "2026-11-12"},
			previous: [2]string{"2026-08-19", "2026-09-30"},
			yoy:      [2]string{"2025-10-01", "2025-11-12"},
		},
		{
			name: "custom",
			query: TrendEventStatsQuery{
				Range:     TrendRangeCustom,
				StartDate: trendEventDate(2026, time.October, 1),
				EndDate:   trendEventDate(2026, time.October, 15),
			},
			current:  [2]string{"2026-10-01", "2026-10-15"},
			previous: [2]string{"2026-09-16", "2026-09-30"},
			yoy:      [2]string{"2025-10-01", "2025-10-15"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			periods, err := resolveTrendEventPeriods(test.query, trendEventDate(2026, time.November, 12), trendEventDate(2026, time.November, 12))
			if err != nil {
				t.Fatalf("resolve event periods: %v", err)
			}
			assertTrendEventWindow(t, periods.Current, test.current[0], test.current[1])
			assertTrendEventWindow(t, periods.Previous, test.previous[0], test.previous[1])
			assertTrendEventWindow(t, periods.YearOverYear, test.yoy[0], test.yoy[1])
		})
	}
}

func TestResolveTrendEventPeriodsUsesCompletePreviousNaturalPeriods(t *testing.T) {
	periods, err := resolveTrendEventPeriods(
		TrendEventStatsQuery{Range: TrendRangeMonth},
		trendEventDate(2026, time.November, 30),
		trendEventDate(2026, time.November, 30),
	)
	if err != nil {
		t.Fatalf("resolve complete month: %v", err)
	}
	assertTrendEventWindow(t, periods.Previous, "2026-10-01", "2026-10-31")

	periods, err = resolveTrendEventPeriods(
		TrendEventStatsQuery{Range: TrendRangeQuarter},
		trendEventDate(2026, time.December, 31),
		trendEventDate(2026, time.December, 31),
	)
	if err != nil {
		t.Fatalf("resolve complete quarter: %v", err)
	}
	assertTrendEventWindow(t, periods.Previous, "2026-07-01", "2026-09-30")

	periods, err = resolveTrendEventPeriods(
		TrendEventStatsQuery{Range: TrendRangeQuarter},
		trendEventDate(2026, time.November, 30),
		trendEventDate(2026, time.November, 30),
	)
	if err != nil {
		t.Fatalf("resolve partial quarter: %v", err)
	}
	assertTrendEventWindow(t, periods.Current, "2026-10-01", "2026-11-30")
	assertTrendEventWindow(t, periods.Previous, "2026-08-01", "2026-09-30")
}

func TestResolveTrendEventPeriodsRejectsAllHistoryAndInvalidCustomRange(t *testing.T) {
	tests := []TrendEventStatsQuery{
		{Range: TrendRangeAll},
		{Range: TrendRangeCustom, StartDate: trendEventDate(2026, time.October, 15), EndDate: trendEventDate(2026, time.October, 1)},
	}
	for _, query := range tests {
		if _, err := resolveTrendEventPeriods(query, trendEventDate(2026, time.November, 12), trendEventDate(2026, time.November, 12)); err != ErrInvalidTrendQuery {
			t.Fatalf("query %#v error = %v, want ErrInvalidTrendQuery", query, err)
		}
	}
}

func TestResolveTrendEventPeriodsAllowsCustomRangeThroughTodayWhenFinalObservationLags(t *testing.T) {
	periods, err := resolveTrendEventPeriods(
		TrendEventStatsQuery{
			Range:     TrendRangeCustom,
			StartDate: trendEventDate(2026, time.November, 1),
			EndDate:   trendEventDate(2026, time.November, 12),
		},
		trendEventDate(2026, time.November, 12),
		trendEventDate(2026, time.November, 11),
	)
	if err != nil {
		t.Fatalf("custom range should tolerate a missing final observation: %v", err)
	}
	assertTrendEventWindow(t, periods.Current, "2026-11-01", "2026-11-12")
	if periods.CurrentAsOf != "" {
		t.Fatalf("custom period as-of = %q, want empty", periods.CurrentAsOf)
	}
}

func TestTrendEventObservationWindowCoversCurrentPreviousAndYearOverYear(t *testing.T) {
	periods := trendEventPeriods{
		Current:      trendEventWindow{Start: trendEventDate(2026, time.November, 1), End: trendEventDate(2026, time.November, 12)},
		Previous:     trendEventWindow{Start: trendEventDate(2026, time.October, 1), End: trendEventDate(2026, time.October, 12)},
		YearOverYear: trendEventWindow{Start: trendEventDate(2025, time.November, 1), End: trendEventDate(2025, time.November, 12)},
	}

	assertTrendEventWindow(t, trendEventObservationWindow(periods), "2025-11-01", "2026-11-12")
}

func TestTrendEventDateAvailabilityTreatsBaselineAndMissingDatesAsLegalZeroes(t *testing.T) {
	baseline := trendEventDate(2026, time.October, 22)
	period := trendEventWindow{Start: trendEventDate(2026, time.October, 12), End: trendEventDate(2026, time.October, 21)}
	if got := trendEventWindowObservationDays(period, baseline, nil); got != 0 {
		t.Fatalf("pre-baseline observation days = %d, want legal zero", got)
	}

	period = trendEventWindow{Start: trendEventDate(2026, time.October, 22), End: trendEventDate(2026, time.October, 24)}
	observed := map[string]struct{}{"2026-10-22": {}, "2026-10-24": {}}
	if got := trendEventWindowObservationDays(period, baseline, observed); got != 2 {
		t.Fatalf("trusted observation days = %d, want 2 with missing date skipped", got)
	}

	period = trendEventWindow{Start: trendEventDate(2026, time.October, 23), End: trendEventDate(2026, time.October, 31)}
	if got := trendEventWindowObservationDays(period, baseline, nil); got != 0 {
		t.Fatalf("fully missing observation days = %d, want legal zero", got)
	}
}

func TestClassifyTrendEventUsesLifecyclePriorityAndExistingModificationSignal(t *testing.T) {
	date := trendEventDate(2026, time.October, 23)
	tests := []struct {
		name       string
		previous   trendEventObservation
		current    trendEventObservation
		first      bool
		wantType   TrendEventType
		wantReason TrendEventReason
		want       bool
	}{
		{
			name:       "baseline entered",
			current:    trendEventObservation{Date: trendEventDate(2026, time.October, 22), ActivityState: ActivityStateEffective, Matches: true},
			first:      true,
			wantType:   TrendEventEntered,
			wantReason: TrendEventReasonBaseline,
			want:       true,
		},
		{
			name:       "first observed after baseline",
			current:    trendEventObservation{Date: date, ActivityState: ActivityStateEffective, Matches: true},
			first:      true,
			wantType:   TrendEventEntered,
			wantReason: TrendEventReasonFirstObserved,
			want:       true,
		},
		{
			name:       "modified entered",
			previous:   trendEventObservation{ActivityState: ActivityStateEffective, Matches: false},
			current:    trendEventObservation{ActivityState: ActivityStateEffective, Matches: true, Modified: true},
			wantType:   TrendEventEntered,
			wantReason: TrendEventReasonModifiedEntered,
			want:       true,
		},
		{
			name:       "modified inside",
			previous:   trendEventObservation{ActivityState: ActivityStateEffective, Matches: true},
			current:    trendEventObservation{ActivityState: ActivityStateEffective, Matches: true, Modified: true},
			wantType:   TrendEventModified,
			wantReason: TrendEventReasonInsideModified,
			want:       true,
		},
		{
			name:       "modified left",
			previous:   trendEventObservation{ActivityState: ActivityStateEffective, Matches: true},
			current:    trendEventObservation{ActivityState: ActivityStateEffective, Matches: false, Modified: true},
			wantType:   TrendEventLeft,
			wantReason: TrendEventReasonModifiedLeft,
			want:       true,
		},
		{
			name:       "closed takes precedence over modification",
			previous:   trendEventObservation{ActivityState: ActivityStateEffective, Matches: true},
			current:    trendEventObservation{ActivityState: ActivityStateConfirmedClosed, Matches: false, Modified: true},
			wantType:   TrendEventLeft,
			wantReason: TrendEventReasonConfirmedClosed,
			want:       true,
		},
		{
			name:       "reopened is independent",
			previous:   trendEventObservation{ActivityState: ActivityStateConfirmedClosed, Matches: false},
			current:    trendEventObservation{ActivityState: ActivityStateEffective, Matches: true, Modified: true},
			wantType:   TrendEventReopened,
			wantReason: TrendEventReasonReopened,
			want:       true,
		},
		{
			name:     "unchanged is not an event",
			previous: trendEventObservation{ActivityState: ActivityStateEffective, Matches: true},
			current:  trendEventObservation{ActivityState: ActivityStateEffective, Matches: true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotType, gotReason, got := classifyTrendEvent(test.previous, test.current, test.first, trendEventDate(2026, time.October, 22))
			if got != test.want || gotType != test.wantType || gotReason != test.wantReason {
				t.Fatalf("classify = (%q, %q, %t), want (%q, %q, %t)", gotType, gotReason, got, test.wantType, test.wantReason, test.want)
			}
		})
	}
}

func TestTrendEventDrilldownMarksInternalModificationAsChanged(t *testing.T) {
	observation := trendEventObservation{
		SourceMetadataVersionID: "metadata-v2",
		BodyContentVersionID:    "body-v2",
	}
	if got := trendEventDrilldownItemStatus(TrendEventModified, string(ActivityStateEffective), "metadata-v2", "body-v2", observation); got != TrendDrilldownStatusChanged {
		t.Fatalf("modified event status = %q, want changed", got)
	}
	if got := trendEventDrilldownItemStatus(TrendEventModified, string(ActivityStateConfirmedClosed), "metadata-v2", "body-v2", observation); got != TrendDrilldownStatusClosed {
		t.Fatalf("closed modified event status = %q, want confirmed_closed", got)
	}
}

func TestAggregateTrendEventsDeduplicatesPerJobAndType(t *testing.T) {
	window := trendEventWindow{Start: trendEventDate(2026, time.October, 22), End: trendEventDate(2026, time.October, 31)}
	events := []trendEvent{
		{Date: trendEventDate(2026, time.October, 23), JobID: "job-a", Type: TrendEventEntered},
		{Date: trendEventDate(2026, time.October, 24), JobID: "job-a", Type: TrendEventEntered},
		{Date: trendEventDate(2026, time.October, 25), JobID: "job-a", Type: TrendEventModified},
		{Date: trendEventDate(2026, time.October, 26), JobID: "job-b", Type: TrendEventLeft},
	}
	counts := aggregateTrendEvents(events, window)
	if counts[TrendEventEntered] != 1 || counts[TrendEventModified] != 1 || counts[TrendEventLeft] != 1 {
		t.Fatalf("counts = %#v, want one per job/type", counts)
	}
}

func BenchmarkAggregateTrendEvents(b *testing.B) {
	window := trendEventWindow{Start: trendEventDate(2026, time.October, 22), End: trendEventDate(2026, time.October, 31)}
	types := []TrendEventType{TrendEventEntered, TrendEventModified, TrendEventLeft, TrendEventReopened}
	events := make([]trendEvent, 0, 20000)
	for index := 0; index < cap(events); index++ {
		events = append(events, trendEvent{
			Date:  trendEventDate(2026, time.October, 22+(index%10)),
			JobID: "job-" + strconv.Itoa(index/4),
			Type:  types[index%len(types)],
		})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		counts := aggregateTrendEvents(events, window)
		if len(counts) != len(types) {
			b.Fatalf("aggregated event types = %d, want %d", len(counts), len(types))
		}
	}
}

func TestClassifyTrendEventObservationsSkipsPreBaselineHistory(t *testing.T) {
	baseline := trendEventDate(2026, time.October, 22)
	events := classifyTrendEventObservations([]trendEventObservation{
		{Date: trendEventDate(2026, time.October, 21), JobID: "job-a", ActivityState: ActivityStateEffective, Matches: true},
		{Date: baseline, JobID: "job-a", ActivityState: ActivityStateEffective, Matches: true},
	}, baseline)
	if len(events) != 1 || events[0].Type != TrendEventEntered || events[0].Reason != TrendEventReasonBaseline {
		t.Fatalf("events = %#v, want one baseline entry", events)
	}
}

func TestClassifyTrendEventObservationsAssignsGapTransitionToLaterTrustedDate(t *testing.T) {
	baseline := trendEventDate(2026, time.October, 22)
	later := trendEventDate(2026, time.October, 24)
	events := classifyTrendEventObservations([]trendEventObservation{
		{Date: baseline, JobID: "job-a", ActivityState: ActivityStateEffective, BodyContentVersionID: "body-1", Matches: false},
		{Date: later, JobID: "job-a", ActivityState: ActivityStateEffective, BodyContentVersionID: "body-2", Matches: true},
	}, baseline)
	if len(events) != 1 || !events[0].Date.Equal(later) || events[0].Type != TrendEventEntered {
		t.Fatalf("events = %#v, want one entered event on later trusted date", events)
	}

	if counts := aggregateTrendEvents(nil, trendEventWindow{Start: baseline, End: later}); len(counts) != 0 {
		t.Fatalf("empty trusted history counts = %#v, want legal zero counts", counts)
	}
}

func TestClassifyTrendEventObservationsUsesAdjacentSourceVersions(t *testing.T) {
	baseline := trendEventDate(2026, time.October, 22)
	changeDate := trendEventDate(2026, time.October, 23)
	events := classifyTrendEventObservations([]trendEventObservation{
		{Date: baseline, JobID: "job-entered", ActivityState: ActivityStateEffective, BodyContentVersionID: "body-1", SourceMetadataVersionID: "metadata-1"},
		{Date: changeDate, JobID: "job-entered", ActivityState: ActivityStateEffective, BodyContentVersionID: "body-2", SourceMetadataVersionID: "metadata-2", Matches: true},
		{Date: baseline, JobID: "job-modified", ActivityState: ActivityStateEffective, BodyContentVersionID: "body-1", SourceMetadataVersionID: "metadata-1", Matches: true},
		{Date: changeDate, JobID: "job-modified", ActivityState: ActivityStateEffective, BodyContentVersionID: "body-2", SourceMetadataVersionID: "metadata-2", Matches: true},
		{Date: baseline, JobID: "job-left", ActivityState: ActivityStateEffective, BodyContentVersionID: "body-1", SourceMetadataVersionID: "metadata-1", Matches: true},
		{Date: changeDate, JobID: "job-left", ActivityState: ActivityStateEffective, BodyContentVersionID: "body-2", SourceMetadataVersionID: "metadata-2"},
	}, baseline)

	got := make(map[string]TrendEventType)
	for _, event := range events {
		got[event.JobID] = event.Type
	}
	for jobID, want := range map[string]TrendEventType{
		"job-entered":  TrendEventEntered,
		"job-modified": TrendEventModified,
		"job-left":     TrendEventLeft,
	} {
		if got[jobID] != want {
			t.Fatalf("job %q event = %q, want %q; events = %#v", jobID, got[jobID], want, events)
		}
	}
}

func TestTrendEventRateKeepsLegalZeroAndUnknownDenominatorDistinct(t *testing.T) {
	if got := trendEventRate(4, 0); got != nil {
		t.Fatalf("zero denominator rate = %v, want nil", *got)
	}
	got := trendEventRate(0, 4)
	if got == nil || *got != -100 {
		t.Fatalf("zero current rate = %v, want -100", got)
	}
	got = trendEventRate(9, 7)
	if got == nil || *got != 28.6 {
		t.Fatalf("rate = %v, want 28.6", got)
	}
}

func TestTrendEventObservationSQLUsesFormalCompleteRunsAndNullableLowerBound(t *testing.T) {
	statement := trendEventObservationSQL(trendEventFilter{})
	if strings.Contains(statement, "__TREND_EVENT_RUN_SOURCE__") || strings.Contains(statement, "%s") {
		t.Fatalf("observation SQL contains unresolved source placeholder: %s", statement)
	}
	if !strings.Contains(statement, "FROM trusted_runs AS selected") {
		t.Fatalf("observation SQL does not use trusted runs: %s", statement)
	}

	formal := trendEventObservationSQL(trendEventFilter{
		completenessJoins: "LEFT JOIN example_results AS result ON result.id = membership.id",
		formalIncomplete:  "result.id IS NULL",
	})
	if !strings.Contains(formal, "FROM trusted_formal_runs AS selected") || strings.Contains(formal, "FROM trusted_runs AS selected\n    JOIN serving_jd_inventory_memberships AS membership\n      ON membership.published_run_id = selected.id\n    ORDER BY") {
		t.Fatalf("formal observation SQL does not select complete runs: %s", formal)
	}
	if !strings.Contains(formal, "($1::date IS NULL OR") {
		t.Fatalf("observation SQL does not support a dynamic lower bound: %s", formal)
	}
}

func TestTrendEventObservationBoundsUseTrustedRunsInsteadOfMembershipRows(t *testing.T) {
	if got := trendEventRunSource(trendEventFilter{}); got != "trusted_runs" {
		t.Fatalf("run source = %q, want trusted_runs", got)
	}
	if got := trendEventRunSource(trendEventFilter{
		completenessJoins: "LEFT JOIN example_results AS result ON result.id = membership.id",
		formalIncomplete:  "result.id IS NULL",
	}); got != "trusted_formal_runs" {
		t.Fatalf("formal run source = %q, want trusted_formal_runs", got)
	}
}

func TestTrendEventFreshnessIncludesPeriodsAndFormalInputsWithoutExposingThem(t *testing.T) {
	query := TrendEventStatsQuery{
		Condition: TrendCondition{Dimension: TrendDimensionTechnology, Value: "c++"},
		Range:     TrendRangeMonth,
	}
	periods := trendEventPeriods{
		Current:      trendEventWindow{Start: trendEventDate(2026, time.November, 1), End: trendEventDate(2026, time.November, 12)},
		Previous:     trendEventWindow{Start: trendEventDate(2026, time.October, 1), End: trendEventDate(2026, time.October, 12)},
		YearOverYear: trendEventWindow{Start: trendEventDate(2025, time.November, 1), End: trendEventDate(2025, time.November, 12)},
	}
	first := trendEventFreshness(query, periods, trendEventDate(2026, time.November, 12), trendEventFilter{args: []any{"formal-a"}}, "runs-a")
	second := trendEventFreshness(query, periods, trendEventDate(2026, time.November, 12), trendEventFilter{args: []any{"formal-b"}}, "runs-b")
	if first == second || strings.Contains(first, "formal-a") || strings.Contains(second, "formal-b") {
		t.Fatalf("event freshness did not remain opaque or distinguish formal inputs: %q %q", first, second)
	}
}

func TestTrendEventWindowSQLCoversAllComparisonPeriods(t *testing.T) {
	periods := trendEventPeriods{
		Current:      trendEventWindow{Start: trendEventDate(2026, time.November, 1), End: trendEventDate(2026, time.November, 12)},
		Previous:     trendEventWindow{Start: trendEventDate(2026, time.October, 1), End: trendEventDate(2026, time.October, 12)},
		YearOverYear: trendEventWindow{Start: trendEventDate(2025, time.November, 1), End: trendEventDate(2025, time.November, 12)},
	}
	sql := trendEventWindowSQL("source.calendar_date", periods)
	for _, date := range []string{"2026-11-01", "2026-10-12", "2025-11-12"} {
		if !strings.Contains(sql, date) {
			t.Fatalf("formal coverage SQL omitted %s: %s", date, sql)
		}
	}
}

func TestTrendEventFormalCoverageSQLPlacesBaselineAfterFormalArguments(t *testing.T) {
	periods := trendEventPeriods{
		Current:      trendEventWindow{Start: trendEventDate(2026, time.November, 1), End: trendEventDate(2026, time.November, 12)},
		Previous:     trendEventWindow{Start: trendEventDate(2026, time.October, 1), End: trendEventDate(2026, time.October, 12)},
		YearOverYear: trendEventWindow{Start: trendEventDate(2025, time.November, 1), End: trendEventDate(2025, time.November, 12)},
	}
	statement := trendEventFormalCoverageSQL(trendEventFilter{
		completenessArgs:  []any{"parser", "lexicon", "sha"},
		completenessJoins: "LEFT JOIN example_results AS result ON result.id = membership.id AND result.parser_version = $3",
		formalIncomplete:  "result.id IS NULL",
	}, periods, 6)
	if !strings.Contains(statement, "source.calendar_date >= $6::date") {
		t.Fatalf("formal coverage SQL does not place baseline after formal arguments: %s", statement)
	}
	if strings.Contains(statement, "source.calendar_date >= $3::date") {
		t.Fatalf("formal coverage SQL reused a formal argument placeholder for baseline: %s", statement)
	}
}

func assertTrendEventWindow(t *testing.T, got trendEventWindow, start, end string) {
	t.Helper()
	if got.Start.Format("2006-01-02") != start || got.End.Format("2006-01-02") != end {
		t.Fatalf("window = %s..%s, want %s..%s", got.Start.Format("2006-01-02"), got.End.Format("2006-01-02"), start, end)
	}
}
