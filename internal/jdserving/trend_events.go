package jdserving

import (
	"errors"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var errTrendEventNoFinalObservation = errors.New("no final trend observation")

// TrendEventType is the four mutually named event rows shown by the trend
// dashboard. Counts are unique jobs per period and event type.
type TrendEventType string

const (
	TrendEventEntered  TrendEventType = "entered"
	TrendEventModified TrendEventType = "modified"
	TrendEventLeft     TrendEventType = "left"
	TrendEventReopened TrendEventType = "reopened"
)

// TrendEventReason is kept internal to the calculation and is only exposed by
// the event drill-down as a safe display label.
type TrendEventReason string

const (
	TrendEventReasonBaseline        TrendEventReason = "baseline"
	TrendEventReasonFirstObserved   TrendEventReason = "first_observed"
	TrendEventReasonModifiedEntered TrendEventReason = "modified_entered"
	TrendEventReasonInsideModified  TrendEventReason = "inside_modified"
	TrendEventReasonModifiedLeft    TrendEventReason = "modified_left"
	TrendEventReasonConfirmedClosed TrendEventReason = "confirmed_closed"
	TrendEventReasonReopened        TrendEventReason = "reopened"
)

// TrendEventStatsQuery requests one supported curve and one comparison range.
// It deliberately mirrors TrendQuery instead of embedding a precomputed line
// response, so line and event reads remain separate service boundaries.
type TrendEventStatsQuery struct {
	Condition TrendCondition
	StartDate time.Time
	EndDate   time.Time
	Range     TrendRange
}

type trendEventWindow struct {
	Start time.Time
	End   time.Time
}

type trendEventPeriods struct {
	Current      trendEventWindow
	Previous     trendEventWindow
	YearOverYear trendEventWindow
	Cutoff       time.Time
	CurrentAsOf  string
}

func trendEventObservationWindow(periods trendEventPeriods) trendEventWindow {
	window := trendEventWindow{Start: periods.Current.Start, End: periods.Current.End}
	for _, candidate := range []trendEventWindow{periods.Previous, periods.YearOverYear} {
		if candidate.Start.Before(window.Start) {
			window.Start = candidate.Start
		}
		if candidate.End.After(window.End) {
			window.End = candidate.End
		}
	}
	return window
}

// TrendEventPeriod is the public date envelope for one event comparison
// window. Dates are Shanghai calendar dates, never instants or run IDs.
type TrendEventPeriod struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type TrendEventComparison struct {
	Count *int     `json:"count"`
	Rate  *float64 `json:"rate"`
}

type TrendEventRow struct {
	Type         TrendEventType       `json:"type"`
	Current      int                  `json:"current"`
	Previous     TrendEventComparison `json:"previous"`
	YearOverYear TrendEventComparison `json:"year_over_year"`
}

type TrendEventStatsResponse struct {
	Curve        TrendCurve       `json:"curve"`
	Freshness    string           `json:"freshness"`
	Status       string           `json:"status"`
	Current      TrendEventPeriod `json:"current"`
	Previous     TrendEventPeriod `json:"previous"`
	YearOverYear TrendEventPeriod `json:"year_over_year"`
	AsOf         string           `json:"as_of,omitempty"`
	Rows         []TrendEventRow  `json:"rows"`
}

type trendEventObservation struct {
	Date                    time.Time
	JobID                   string
	Title                   string
	ActivityState           ActivityState
	SourceMetadataVersionID string
	BodyContentVersionID    string
	Matches                 bool
	Modified                bool
}

type trendEvent struct {
	Date   time.Time
	JobID  string
	Type   TrendEventType
	Reason TrendEventReason
}

func shanghaiLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return location
}

func normalizeTrendEventDate(value time.Time) time.Time {
	location := shanghaiLocation()
	value = value.In(location)
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, location)
}

func formatTrendEventDate(value time.Time) string {
	return normalizeTrendEventDate(value).Format("2006-01-02")
}

func parseTrendEventDate(value string) time.Time {
	date, err := time.ParseInLocation("2006-01-02", value, shanghaiLocation())
	if err != nil {
		return time.Time{}
	}
	return date
}

func resolveTrendEventPeriods(query TrendEventStatsQuery, today, latestObservation time.Time) (trendEventPeriods, error) {
	location := shanghaiLocation()
	if latestObservation.IsZero() {
		return trendEventPeriods{}, errTrendEventNoFinalObservation
	}
	today = normalizeTrendEventDate(today)
	latestObservation = normalizeTrendEventDate(latestObservation)
	if latestObservation.IsZero() || latestObservation.After(today) {
		latestObservation = today
	}
	cutoff := latestObservation
	if cutoff.IsZero() {
		return trendEventPeriods{}, errTrendEventNoFinalObservation
	}
	if query.Range == TrendRangeAll {
		return trendEventPeriods{}, ErrInvalidTrendQuery
	}

	var current trendEventWindow
	switch query.Range {
	case TrendRangeCustom:
		current.Start = normalizeTrendEventDate(query.StartDate)
		current.End = normalizeTrendEventDate(query.EndDate)
		if query.StartDate.IsZero() || query.EndDate.IsZero() ||
			current.Start.IsZero() || current.End.IsZero() || current.Start.After(current.End) ||
			current.Start.Year() < 1 || current.End.Year() < 1 || current.End.After(today) {
			return trendEventPeriods{}, ErrInvalidTrendQuery
		}
	default:
		periodStart := cutoff
		switch query.Range {
		case TrendRangeWeek:
			periodStart = cutoff.AddDate(0, 0, -((int(cutoff.Weekday()) + 6) % 7))
		case TrendRangeMonth:
			periodStart = time.Date(cutoff.Year(), cutoff.Month(), 1, 0, 0, 0, 0, location)
		case TrendRangeQuarter:
			month := time.Month(((int(cutoff.Month())-1)/3)*3 + 1)
			periodStart = time.Date(cutoff.Year(), month, 1, 0, 0, 0, 0, location)
		case TrendRangeYear:
			periodStart = time.Date(cutoff.Year(), time.January, 1, 0, 0, 0, 0, location)
		default:
			return trendEventPeriods{}, ErrInvalidTrendQuery
		}
		current = trendEventWindow{Start: periodStart, End: cutoff}
	}

	length := daysInclusive(current)
	previous := trendEventWindow{}
	fullNaturalRange := completeTrendEventNaturalWindow(query.Range, current)
	if fullNaturalRange {
		switch query.Range {
		case TrendRangeMonth:
			previous.Start = current.Start.AddDate(0, -1, 0)
			previous.End = current.Start.AddDate(0, 0, -1)
		case TrendRangeQuarter:
			previous.Start = current.Start.AddDate(0, -3, 0)
			previous.End = current.Start.AddDate(0, 0, -1)
		case TrendRangeYear:
			previous.Start = current.Start.AddDate(-1, 0, 0)
			previous.End = current.Start.AddDate(0, 0, -1)
		default:
			previous.Start = current.Start.AddDate(0, 0, -length)
			previous.End = current.Start.AddDate(0, 0, -1)
		}
	} else if query.Range == TrendRangeMonth {
		previous.Start = current.Start.AddDate(0, -1, 0)
		previous.End = previous.Start.AddDate(0, 0, length-1)
	} else {
		previous.Start = current.Start.AddDate(0, 0, -length)
		previous.End = current.Start.AddDate(0, 0, -1)
	}
	yearOverYear := trendEventWindow{
		Start: current.Start.AddDate(-1, 0, 0),
		End:   current.End.AddDate(-1, 0, 0),
	}
	if query.Range == TrendRangeWeek {
		// Keep the comparison on the same weekday in the prior calendar year.
		// A calendar-year shift can move a Monday to Sunday around a leap day.
		for yearOverYear.Start.Weekday() != current.Start.Weekday() {
			yearOverYear.Start = yearOverYear.Start.AddDate(0, 0, 1)
			yearOverYear.End = yearOverYear.End.AddDate(0, 0, 1)
		}
	}
	return trendEventPeriods{
		Current:      current,
		Previous:     previous,
		YearOverYear: yearOverYear,
		Cutoff:       cutoff,
		CurrentAsOf: func() string {
			if query.Range == TrendRangeCustom {
				return ""
			}
			return formatTrendEventDate(cutoff)
		}(),
	}, nil
}

func completeTrendEventNaturalWindow(rng TrendRange, window trendEventWindow) bool {
	if window.Start.IsZero() || window.End.IsZero() || window.Start.After(window.End) {
		return false
	}
	switch rng {
	case TrendRangeWeek:
		return daysInclusive(window) == 7
	case TrendRangeMonth:
		return window.Start.Day() == 1 && window.End.Equal(lastDayOfMonth(window.End))
	case TrendRangeQuarter:
		quarterEndMonth := time.Month(((int(window.Start.Month())-1)/3)*3 + 3)
		quarterEnd := lastDayOfMonth(time.Date(window.Start.Year(), quarterEndMonth, 1, 0, 0, 0, 0, window.Start.Location()))
		return window.End.Equal(quarterEnd)
	case TrendRangeYear:
		return window.Start.Month() == time.January && window.Start.Day() == 1 &&
			window.End.Month() == time.December && window.End.Equal(lastDayOfMonth(window.End))
	default:
		return false
	}
}

func lastDayOfMonth(value time.Time) time.Time {
	value = normalizeTrendEventDate(value)
	return time.Date(value.Year(), value.Month()+1, 0, 0, 0, 0, 0, value.Location())
}

func daysInclusive(window trendEventWindow) int {
	if window.Start.IsZero() || window.End.IsZero() || window.Start.After(window.End) {
		return 0
	}
	return int(window.End.Sub(window.Start).Hours()/24) + 1
}

func trendEventWindowObservationDays(window trendEventWindow, baseline time.Time, observed map[string]struct{}) int {
	window.Start = normalizeTrendEventDate(window.Start)
	window.End = normalizeTrendEventDate(window.End)
	baseline = normalizeTrendEventDate(baseline)
	if window.Start.After(window.End) || (!baseline.IsZero() && window.End.Before(baseline)) {
		return 0
	}
	if !baseline.IsZero() && window.Start.Before(baseline) {
		window.Start = baseline
	}
	if observed == nil {
		return 0
	}
	count := 0
	for date := window.Start; !date.After(window.End); date = date.AddDate(0, 0, 1) {
		if _, ok := observed[formatTrendEventDate(date)]; ok {
			count++
		}
	}
	return count
}

func trendEventPeriodFromWindow(window trendEventWindow) TrendEventPeriod {
	return TrendEventPeriod{Start: formatTrendEventDate(window.Start), End: formatTrendEventDate(window.End)}
}

func trendEventRate(current, comparison int) *float64 {
	if comparison == 0 {
		return nil
	}
	rate := (float64(current) - float64(comparison)) / float64(comparison) * 100
	rate = math.Round(rate*10) / 10
	return &rate
}

func trendEventComparison(current, comparison int) TrendEventComparison {
	count := comparison
	return TrendEventComparison{Count: &count, Rate: trendEventRate(current, comparison)}
}

func trendEventRows(current, previous, yearOverYear map[TrendEventType]int) []TrendEventRow {
	rows := make([]TrendEventRow, 0, 4)
	for _, eventType := range []TrendEventType{TrendEventEntered, TrendEventModified, TrendEventLeft, TrendEventReopened} {
		rows = append(rows, TrendEventRow{
			Type:         eventType,
			Current:      current[eventType],
			Previous:     trendEventComparison(current[eventType], previous[eventType]),
			YearOverYear: trendEventComparison(current[eventType], yearOverYear[eventType]),
		})
	}
	return rows
}

func classifyTrendEvent(previous, current trendEventObservation, first bool, baseline time.Time) (TrendEventType, TrendEventReason, bool) {
	if first {
		if current.Matches && current.ActivityState == ActivityStateEffective {
			if !baseline.IsZero() && normalizeTrendEventDate(current.Date).Equal(normalizeTrendEventDate(baseline)) {
				return TrendEventEntered, TrendEventReasonBaseline, true
			}
			return TrendEventEntered, TrendEventReasonFirstObserved, true
		}
		return "", "", false
	}
	if previous.ActivityState == ActivityStateConfirmedClosed && current.ActivityState == ActivityStateEffective && current.Matches {
		return TrendEventReopened, TrendEventReasonReopened, true
	}
	if previous.ActivityState == ActivityStateEffective && current.ActivityState == ActivityStateConfirmedClosed && previous.Matches {
		return TrendEventLeft, TrendEventReasonConfirmedClosed, true
	}
	if !current.Modified {
		return "", "", false
	}
	if !previous.Matches && current.Matches {
		return TrendEventEntered, TrendEventReasonModifiedEntered, true
	}
	if previous.Matches && !current.Matches {
		return TrendEventLeft, TrendEventReasonModifiedLeft, true
	}
	if previous.Matches && current.Matches {
		return TrendEventModified, TrendEventReasonInsideModified, true
	}
	return "", "", false
}

var sqlParameterPattern = regexp.MustCompile(`\$(\d+)`)

func shiftSQLPlaceholders(statement string, offset int) string {
	return sqlParameterPattern.ReplaceAllStringFunc(statement, func(match string) string {
		value := strings.TrimPrefix(match, "$")
		index := 0
		for _, character := range value {
			index = index*10 + int(character-'0')
		}
		return "$" + strconv.Itoa(index+offset)
	})
}

func aggregateTrendEvents(events []trendEvent, window trendEventWindow) map[TrendEventType]int {
	seen := make(map[string]struct{})
	counts := make(map[TrendEventType]int)
	for _, event := range events {
		if event.Date.Before(window.Start) || event.Date.After(window.End) {
			continue
		}
		key := event.JobID + "\x00" + string(event.Type)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		counts[event.Type]++
	}
	return counts
}

func sortTrendEvents(events []trendEvent) {
	sort.SliceStable(events, func(left, right int) bool {
		if !events[left].Date.Equal(events[right].Date) {
			return events[left].Date.Before(events[right].Date)
		}
		if events[left].JobID != events[right].JobID {
			return events[left].JobID < events[right].JobID
		}
		return strings.Compare(string(events[left].Type), string(events[right].Type)) < 0
	})
}
