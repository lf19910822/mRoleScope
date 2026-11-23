package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"mrolescope/internal/aitrend"
	"mrolescope/internal/jdserving"
)

// Snapshot is a read-only Serving fixture. It exposes only the same public
// reader seams consumed by the production trend runner.
type Snapshot struct {
	version   string
	asOfDate  time.Time
	options   jdserving.FilterOptionsResponse
	summaries map[snapshotSummaryKey]jdserving.TrendSummary
	series    map[snapshotSeriesKey]snapshotSeries
}

type snapshotSummaryKey struct {
	dimension        jdserving.TrendDimension
	value            string
	rangeKind        jdserving.TrendRange
	includeSecondary bool
}

type snapshotSeriesKey struct {
	dimension        jdserving.TrendDimension
	value            string
	includeSecondary bool
}

type snapshotSeries struct {
	curve            jdserving.TrendCurve
	freshness        string
	points           []jdserving.TrendPoint
	periodUniqueJobs *int
}

type snapshotFile struct {
	Version     string                          `json:"version"`
	AsOfDate    string                          `json:"as_of_date,omitempty"`
	Options     jdserving.FilterOptionsResponse `json:"filter_options"`
	Summaries   []snapshotSummary               `json:"summaries"`
	DailySeries []snapshotDailySeries           `json:"daily_series"`
}

type snapshotSummary struct {
	Dimension          jdserving.TrendDimension `json:"dimension"`
	Value              string                   `json:"value"`
	Range              jdserving.TrendRange     `json:"range"`
	IncludeSecondary   bool                     `json:"include_secondary,omitempty"`
	Label              string                   `json:"label"`
	Freshness          string                   `json:"freshness,omitempty"`
	AverageDailyActive *float64                 `json:"average_daily_active_jobs,omitempty"`
	PeriodEndActive    *int                     `json:"period_end_active_jobs,omitempty"`
	PeriodUnique       *int                     `json:"period_unique_jobs,omitempty"`
	PeakDate           string                   `json:"peak_date,omitempty"`
	PeakCount          int                      `json:"peak_count,omitempty"`
	ValidDays          int                      `json:"valid_observation_days"`
	MissingDays        int                      `json:"missing_observation_days"`
}

// snapshotDailySeries stores frozen daily counts. The evaluator derives the
// requested natural range from these points instead of storing one aggregate
// row per range. A null count preserves an unavailable observation day.
type snapshotDailySeries struct {
	Dimension        jdserving.TrendDimension `json:"dimension"`
	Value            string                   `json:"value"`
	IncludeSecondary bool                     `json:"include_secondary,omitempty"`
	Label            string                   `json:"label"`
	Freshness        string                   `json:"freshness,omitempty"`
	StartDate        string                   `json:"start_date"`
	Counts           []*int                   `json:"counts"`
	PeriodUniqueJobs *int                     `json:"period_unique_jobs,omitempty"`
}

// LoadSnapshot reads and validates a versioned evaluation-only fixture.
func LoadSnapshot(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read evaluation snapshot: %w", err)
	}
	var file snapshotFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("decode evaluation snapshot: %w", err)
	}
	if strings.TrimSpace(file.Version) == "" {
		return nil, fmt.Errorf("evaluation snapshot version is empty")
	}
	snapshot := &Snapshot{
		version: file.Version, options: file.Options,
		summaries: make(map[snapshotSummaryKey]jdserving.TrendSummary, len(file.Summaries)),
		series:    make(map[snapshotSeriesKey]snapshotSeries, len(file.DailySeries)),
	}
	if file.AsOfDate != "" {
		asOfDate, err := parseSnapshotDate(file.AsOfDate)
		if err != nil {
			return nil, fmt.Errorf("evaluation snapshot as_of_date: %w", err)
		}
		snapshot.asOfDate = asOfDate
	}
	for index, item := range file.Summaries {
		if item.Dimension == "" || strings.TrimSpace(item.Value) == "" || item.Range == "" {
			return nil, fmt.Errorf("evaluation snapshot summary %d is incomplete", index+1)
		}
		curve := jdserving.TrendCurve{Dimension: item.Dimension, Value: item.Value, Label: item.Label, IncludeSecondary: item.IncludeSecondary}
		if curve.Label == "" {
			curve.Label = item.Value
		}
		summary := jdserving.TrendSummary{
			Curve: curve, Freshness: item.Freshness,
			AverageDailyActiveJobs: item.AverageDailyActive,
			PeriodEndActiveJobs:    item.PeriodEndActive,
			PeriodUniqueJobs:       item.PeriodUnique,
			ValidObservationDays:   item.ValidDays,
			MissingObservationDays: item.MissingDays,
		}
		if item.PeakDate != "" || item.PeakCount != 0 {
			summary.PeakDailyActiveJobs = &jdserving.TrendSummaryPeak{Date: item.PeakDate, Count: item.PeakCount}
		}
		key := snapshotSummaryKey{dimension: item.Dimension, value: item.Value, rangeKind: item.Range, includeSecondary: item.IncludeSecondary}
		if _, exists := snapshot.summaries[key]; exists {
			return nil, fmt.Errorf("duplicate evaluation snapshot summary for %s/%s/%s", item.Dimension, item.Value, item.Range)
		}
		snapshot.summaries[key] = summary
	}
	for index, item := range file.DailySeries {
		if item.Dimension == "" || strings.TrimSpace(item.Value) == "" || item.StartDate == "" || len(item.Counts) == 0 {
			return nil, fmt.Errorf("evaluation snapshot daily series %d is incomplete", index+1)
		}
		startDate, err := parseSnapshotDate(item.StartDate)
		if err != nil {
			return nil, fmt.Errorf("evaluation snapshot daily series %d start_date: %w", index+1, err)
		}
		if snapshot.asOfDate.IsZero() {
			snapshot.asOfDate = startDate.AddDate(0, 0, len(item.Counts)-1)
		}
		curve := jdserving.TrendCurve{Dimension: item.Dimension, Value: item.Value, Label: item.Label, IncludeSecondary: item.IncludeSecondary}
		if curve.Label == "" {
			curve.Label = item.Value
		}
		points := make([]jdserving.TrendPoint, len(item.Counts))
		for pointIndex, count := range item.Counts {
			date := startDate.AddDate(0, 0, pointIndex)
			if date.After(snapshot.asOfDate) {
				return nil, fmt.Errorf("evaluation snapshot daily series %s/%s exceeds as_of_date", item.Dimension, item.Value)
			}
			value := count
			points[pointIndex] = jdserving.TrendPoint{Date: date.Format("2006-01-02"), Count: value}
		}
		key := snapshotSeriesKey{dimension: item.Dimension, value: item.Value, includeSecondary: item.IncludeSecondary}
		if _, exists := snapshot.series[key]; exists {
			return nil, fmt.Errorf("duplicate evaluation snapshot daily series for %s/%s", item.Dimension, item.Value)
		}
		snapshot.series[key] = snapshotSeries{curve: curve, freshness: item.Freshness, points: points, periodUniqueJobs: item.PeriodUniqueJobs}
	}
	return snapshot, nil
}

func (snapshot *Snapshot) Version() string {
	if snapshot == nil {
		return ""
	}
	return snapshot.version
}

// ListFilterOptions implements aitrend.FilterOptionsReader without importing
// the production AI package, keeping the fixture reusable by tests and tools.
func (snapshot *Snapshot) ListFilterOptions(context.Context) (jdserving.FilterOptionsResponse, error) {
	if snapshot == nil {
		return jdserving.FilterOptionsResponse{}, fmt.Errorf("evaluation snapshot is nil")
	}
	return snapshot.options, nil
}

// ReadTrendSummary implements jdserving.TrendSummaryReader with a stable,
// read-only lookup. Missing fixture rows are explicit backend failures so the
// evaluator can exercise partial-result handling.
func (snapshot *Snapshot) ReadTrendSummary(_ context.Context, query jdserving.TrendQuery) (jdserving.TrendSummary, error) {
	if snapshot == nil {
		return jdserving.TrendSummary{}, fmt.Errorf("evaluation snapshot is nil")
	}
	seriesKey := snapshotSeriesKey{dimension: query.Condition.Dimension, value: query.Condition.Value, includeSecondary: query.Condition.IncludeSecondary}
	if series, ok := snapshot.series[seriesKey]; ok {
		start, end, err := snapshot.rangeDates(query, series.points)
		if err != nil {
			return jdserving.TrendSummary{}, err
		}
		filtered := jdserving.TrendSeries{Curve: series.curve, Freshness: series.freshness, Points: make([]jdserving.TrendPoint, 0, len(series.points))}
		for _, point := range series.points {
			date, parseErr := parseSnapshotDate(point.Date)
			if parseErr != nil {
				return jdserving.TrendSummary{}, parseErr
			}
			if (start.IsZero() || !date.Before(start)) && !date.After(end) {
				filtered.Points = append(filtered.Points, point)
			}
		}
		summary := jdserving.SummarizeTrendSeries(filtered)
		summary.PeriodUniqueJobs = series.periodUniqueJobs
		return summary, nil
	}
	key := snapshotSummaryKey{dimension: query.Condition.Dimension, value: query.Condition.Value, rangeKind: query.Range, includeSecondary: query.Condition.IncludeSecondary}
	if summary, ok := snapshot.summaries[key]; ok {
		return summary, nil
	}
	if !snapshot.hasFormalValue(query.Condition.Dimension, query.Condition.Value) {
		return jdserving.TrendSummary{}, &aitrend.ContractError{Code: aitrend.ErrorUnavailableValue, Field: "value", Message: "evaluation snapshot value is unavailable"}
	}
	return jdserving.TrendSummary{}, fmt.Errorf("evaluation snapshot has no summary for %s/%s/%s", query.Condition.Dimension, query.Condition.Value, query.Range)
}

func parseSnapshotDate(value string) (time.Time, error) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Time{}, err
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, location)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q", value)
	}
	return parsed, nil
}

func (snapshot *Snapshot) rangeDates(query jdserving.TrendQuery, points []jdserving.TrendPoint) (time.Time, time.Time, error) {
	if snapshot.asOfDate.IsZero() || len(points) == 0 {
		return time.Time{}, time.Time{}, fmt.Errorf("evaluation snapshot has no daily range")
	}
	end := snapshot.asOfDate
	var start time.Time
	switch query.Range {
	case jdserving.TrendRangeAll:
		start = time.Time{}
	case jdserving.TrendRangeWeek:
		start = end.AddDate(0, 0, -((int(end.Weekday()) + 6) % 7))
	case jdserving.TrendRangeMonth:
		start = time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, end.Location())
	case jdserving.TrendRangeQuarter:
		month := time.Month(((int(end.Month())-1)/3)*3 + 1)
		start = time.Date(end.Year(), month, 1, 0, 0, 0, 0, end.Location())
	case jdserving.TrendRangeYear:
		start = time.Date(end.Year(), time.January, 1, 0, 0, 0, 0, end.Location())
	case jdserving.TrendRangeCustom:
		start = query.StartDate.In(end.Location())
		end = query.EndDate.In(end.Location())
		if query.StartDate.IsZero() || query.EndDate.IsZero() || start.After(end) {
			return time.Time{}, time.Time{}, &aitrend.ContractError{Code: aitrend.ErrorInvalidEnum, Field: "range", Message: "invalid custom range"}
		}
	default:
		return time.Time{}, time.Time{}, &aitrend.ContractError{Code: aitrend.ErrorInvalidEnum, Field: "range", Message: "unsupported range"}
	}
	return start, end, nil
}

func (snapshot *Snapshot) hasFormalValue(dimension jdserving.TrendDimension, value string) bool {
	switch dimension {
	case jdserving.TrendDimensionTechnology:
		for _, option := range snapshot.options.TechnologyKeywords {
			if option.Value == value {
				return true
			}
		}
	case jdserving.TrendDimensionProject:
		for _, option := range snapshot.options.Projects {
			if option.Value == value {
				return true
			}
		}
	case jdserving.TrendDimensionJobFamily:
		for _, option := range snapshot.options.JobFamilies {
			if option.Value == value {
				return true
			}
		}
	case jdserving.TrendDimensionLocation:
		for _, option := range snapshot.options.Locations {
			if option.Value == value {
				return true
			}
		}
	}
	return false
}

// ValidateCases checks technical formal IDs against the evaluation snapshot.
// Cases that intentionally exercise an unavailable value declare that error
// expectation and are allowed to reference that one unknown expression.
func (snapshot *Snapshot) ValidateCases(cases []Case) error {
	if snapshot == nil {
		return fmt.Errorf("evaluation snapshot is nil")
	}
	known := map[string]map[string]struct{}{
		string(aitrendDimensionTechnology): {},
		string(aitrendDimensionProject):    {},
		string(aitrendDimensionJobFamily):  {},
		string(aitrendDimensionLocation):   {},
	}
	for _, option := range snapshot.options.TechnologyKeywords {
		known[string(aitrendDimensionTechnology)][option.Value] = struct{}{}
		known[string(aitrendDimensionTechnology)][option.Category.Value] = struct{}{}
	}
	for _, option := range snapshot.options.Projects {
		known[string(aitrendDimensionProject)][option.Value] = struct{}{}
	}
	for _, option := range snapshot.options.JobFamilies {
		known[string(aitrendDimensionJobFamily)][option.Value] = struct{}{}
	}
	for _, option := range snapshot.options.Locations {
		known[string(aitrendDimensionLocation)][option.Value] = struct{}{}
	}
	for _, item := range cases {
		if item.ExpectedCategory != "" {
			if _, ok := known[aitrendDimensionTechnology][item.ExpectedCategory]; !ok {
				return fmt.Errorf("case %s references unknown technology category %q", item.ID, item.ExpectedCategory)
			}
		}
		for _, expression := range item.Expressions {
			if expression == "*" {
				continue
			}
			if _, ok := known[item.Dimension][expression]; ok {
				continue
			}
			if containsExpectedError(item.ExpectedErrors, "unavailable_value") {
				continue
			}
			return fmt.Errorf("case %s references unknown %s value %q", item.ID, item.Dimension, expression)
		}
		for _, required := range item.RequiredValues {
			if required == "*" {
				return fmt.Errorf("case %s cannot require wildcard value", item.ID)
			}
			if _, ok := known[item.Dimension][required]; ok {
				continue
			}
			if containsExpectedError(item.ExpectedErrors, "unavailable_value") {
				continue
			}
			return fmt.Errorf("case %s references unknown required %s value %q", item.ID, item.Dimension, required)
		}
	}
	return nil
}

func containsExpectedError(errors []string, expected string) bool {
	for _, value := range errors {
		if value == expected {
			return true
		}
	}
	return false
}

// Keep these aliases local so snapshot validation does not import the AI
// package and create a production dependency cycle.
const (
	aitrendDimensionTechnology = "technology"
	aitrendDimensionProject    = "project"
	aitrendDimensionJobFamily  = "job_family"
	aitrendDimensionLocation   = "location"
)
