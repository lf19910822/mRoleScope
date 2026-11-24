package evaluation

import (
	"context"
	"path/filepath"
	"testing"

	"mrolescope/internal/jdserving"
)

func TestEvaluationSnapshotIsDeterministicAndReadOnly(t *testing.T) {
	path := filepath.Join("testdata", "serving-snapshot-v1.json")
	first, err := LoadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if first.Version() != second.Version() || first.Version() == "" {
		t.Fatalf("snapshot versions differ or are empty: %q %q", first.Version(), second.Version())
	}
	options, err := first.ListFilterOptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(options.Projects) != 2 || len(options.TechnologyKeywords) != 6 {
		t.Fatalf("snapshot options = %#v", options)
	}
	summary, err := first.ReadTrendSummary(context.Background(), jdserving.TrendQuery{Range: jdserving.TrendRangeAll, Condition: jdserving.TrendCondition{Dimension: jdserving.TrendDimensionTechnology, Value: "go"}})
	if err != nil {
		t.Fatal(err)
	}
	if summary.AverageDailyActiveJobs == nil || *summary.AverageDailyActiveJobs != 64 {
		t.Fatalf("snapshot summary = %#v", summary)
	}
	if _, err := first.ReadTrendSummary(context.Background(), jdserving.TrendQuery{Range: jdserving.TrendRangeAll, Condition: jdserving.TrendCondition{Dimension: jdserving.TrendDimensionTechnology, Value: "missing"}}); err == nil {
		t.Fatal("missing fixture summary unexpectedly succeeded")
	}
}

func TestEvaluationSnapshotValidatesBenchmarkFormalIDs(t *testing.T) {
	snapshot, err := LoadSnapshot(filepath.Join("testdata", "serving-snapshot-v3.json"))
	if err != nil {
		t.Fatal(err)
	}
	cases, err := LoadFile(filepath.Join("..", "..", "..", "Docs", "开发指导", "AI评测", "benchmarks", "AI趋势智能体Benchmark-v1.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := snapshot.ValidateCases(cases); err != nil {
		t.Fatal(err)
	}
}

func TestEvaluationSnapshotVersionsRemainIsolated(t *testing.T) {
	v1, err := LoadSnapshot(filepath.Join("testdata", "serving-snapshot-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	v3, err := LoadSnapshot(filepath.Join("testdata", "serving-snapshot-v3.json"))
	if err != nil {
		t.Fatal(err)
	}
	if v1.Version() == v3.Version() || v1.Version() != "serving-evaluation-snapshot-v1" || v3.Version() != "serving-evaluation-snapshot-v3" {
		t.Fatalf("snapshot versions = %q/%q", v1.Version(), v3.Version())
	}
	v1Options, _ := v1.ListFilterOptions(context.Background())
	v3Options, _ := v3.ListFilterOptions(context.Background())
	if len(v1Options.Projects) >= len(v3Options.Projects) || len(v3Options.Projects) < 8 {
		t.Fatalf("snapshot candidate isolation = %d/%d", len(v1Options.Projects), len(v3Options.Projects))
	}
}

func TestEvaluationSnapshotV2SupportsDefaultMonthRange(t *testing.T) {
	snapshot, err := LoadSnapshot(filepath.Join("testdata", "serving-snapshot-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	summary, err := snapshot.ReadTrendSummary(context.Background(), jdserving.TrendQuery{
		Range:     jdserving.TrendRangeMonth,
		Condition: jdserving.TrendCondition{Dimension: jdserving.TrendDimensionTechnology, Value: "go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary.AverageDailyActiveJobs == nil {
		t.Fatal("month snapshot summary has no average")
	}
	allSummary, err := snapshot.ReadTrendSummary(context.Background(), jdserving.TrendQuery{
		Range:     jdserving.TrendRangeAll,
		Condition: jdserving.TrendCondition{Dimension: jdserving.TrendDimensionTechnology, Value: "go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if allSummary.ValidObservationDays <= summary.ValidObservationDays {
		t.Fatalf("daily range derivation = all %d days, month %d days", allSummary.ValidObservationDays, summary.ValidObservationDays)
	}
}

func TestEvaluationSnapshotV3ProvidesCapacityCandidates(t *testing.T) {
	snapshot, err := LoadSnapshot(filepath.Join("testdata", "serving-snapshot-v3.json"))
	if err != nil {
		t.Fatal(err)
	}
	options, err := snapshot.ListFilterOptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(options.Projects) < 8 {
		t.Fatalf("snapshot project candidates = %d, want at least 8", len(options.Projects))
	}
}
