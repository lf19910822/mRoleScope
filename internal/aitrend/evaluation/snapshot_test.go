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
	snapshot, err := LoadSnapshot(filepath.Join("testdata", "serving-snapshot-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	cases, err := LoadFile(filepath.Join("..", "..", "..", "Docs", "开发指导", "AI评测", "AI趋势智能体Benchmark-v1.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := snapshot.ValidateCases(cases); err != nil {
		t.Fatal(err)
	}
}
