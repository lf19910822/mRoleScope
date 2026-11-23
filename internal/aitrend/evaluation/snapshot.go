package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"mrolescope/internal/aitrend"
	"mrolescope/internal/jdserving"
)

// Snapshot is a read-only Serving fixture. It exposes only the same public
// reader seams consumed by the production trend runner.
type Snapshot struct {
	version   string
	options   jdserving.FilterOptionsResponse
	summaries map[snapshotSummaryKey]jdserving.TrendSummary
}

type snapshotSummaryKey struct {
	dimension        jdserving.TrendDimension
	value            string
	rangeKind        jdserving.TrendRange
	includeSecondary bool
}

type snapshotFile struct {
	Version   string                          `json:"version"`
	Options   jdserving.FilterOptionsResponse `json:"filter_options"`
	Summaries []snapshotSummary               `json:"summaries"`
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
	snapshot := &Snapshot{version: file.Version, options: file.Options, summaries: make(map[snapshotSummaryKey]jdserving.TrendSummary, len(file.Summaries))}
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
	key := snapshotSummaryKey{dimension: query.Condition.Dimension, value: query.Condition.Value, rangeKind: query.Range, includeSecondary: query.Condition.IncludeSecondary}
	if summary, ok := snapshot.summaries[key]; ok {
		return summary, nil
	}
	if !snapshot.hasFormalValue(query.Condition.Dimension, query.Condition.Value) {
		return jdserving.TrendSummary{}, &aitrend.ContractError{Code: aitrend.ErrorUnavailableValue, Field: "value", Message: "evaluation snapshot value is unavailable"}
	}
	return jdserving.TrendSummary{}, fmt.Errorf("evaluation snapshot has no summary for %s/%s/%s", query.Condition.Dimension, query.Condition.Value, query.Range)
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
