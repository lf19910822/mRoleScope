package evaluation

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"mrolescope/internal/aitrend"
)

type delayedIntentProvider struct {
	delay time.Duration
	intent aitrend.Intent
}

func (provider delayedIntentProvider) AnalyzeIntent(context.Context, string, *aitrend.ContractError) ([]byte, error) {
	time.Sleep(provider.delay)
	return json.Marshal(provider.intent)
}

func TestRunnerMeasuresFirstModelEventAfterInitialOperation(t *testing.T) {
	delay := 15 * time.Millisecond
	provider := delayedIntentProvider{delay: delay, intent: aitrend.Intent{
		ContractVersion: aitrend.ContractVersionV1,
		SubQueries: []aitrend.SubQuery{{ID: "go", Dimension: aitrend.DimensionTechnology, Expressions: []string{"go"}, Ranking: aitrend.RankingIntent{Metric: aitrend.MetricAverageDailyActiveJobs, Order: aitrend.OrderDescending, Limit: 1}}},
		TimeRange: aitrend.TimeRange{Kind: aitrend.RangeAll},
		Output: aitrend.OutputIntent{SeriesMetric: aitrend.SeriesMetricDailyActiveJobs, ChartType: aitrend.ChartTypeLine},
	}}
	snapshot, err := LoadSnapshot(filepath.Join("testdata", "serving-snapshot-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	runner, err := NewRunner(provider, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.Run(context.Background(), "Go 的岗位趋势")
	if err != nil {
		t.Fatal(err)
	}
	if result.FirstEventLatency < delay {
		t.Fatalf("first model event latency = %s, want at least %s", result.FirstEventLatency, delay)
	}
}

type intentOnlyProvider struct{ intent aitrend.Intent }

func (provider intentOnlyProvider) AnalyzeIntent(context.Context, string, *aitrend.ContractError) ([]byte, error) {
	return json.Marshal(provider.intent)
}

func TestRunnerReusesProductionTrendPathAndSnapshotReaders(t *testing.T) {
	snapshot, err := LoadSnapshot(filepath.Join("testdata", "serving-snapshot-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	intent := aitrend.Intent{
		ContractVersion: aitrend.ContractVersionV1,
		SubQueries: []aitrend.SubQuery{{
			ID: "compare", Dimension: aitrend.DimensionTechnology, Expressions: []string{"go", "python"},
			Ranking: aitrend.RankingIntent{Metric: aitrend.MetricAverageDailyActiveJobs, Order: aitrend.OrderDescending, Limit: 2},
		}},
		TimeRange: aitrend.TimeRange{Kind: aitrend.RangeAll},
		Output:    aitrend.OutputIntent{SeriesMetric: aitrend.SeriesMetricDailyActiveJobs, ChartType: aitrend.ChartTypeLine},
	}
	runner, err := NewRunner(intentOnlyProvider{intent: intent}, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.Run(context.Background(), "比较 Go 和 Python 的岗位数量")
	if err != nil {
		t.Fatal(err)
	}
	if result.Draft == nil || len(result.Draft.Curves) != 2 {
		t.Fatalf("draft = %#v", result.Draft)
	}
	if result.Stats.ToolCalls == 0 || result.Stats.DraftCurveCount != 2 {
		t.Fatalf("stats = %#v", result.Stats)
	}
	if len(result.Events) == 0 {
		t.Fatal("production path emitted no events")
	}
}

func TestRunnerDoesNotMutateEvaluationSnapshotState(t *testing.T) {
	snapshot, err := LoadSnapshot(filepath.Join("testdata", "serving-snapshot-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	before, err := snapshot.ListFilterOptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	intent := aitrend.Intent{
		ContractVersion: aitrend.ContractVersionV1,
		SubQueries:      []aitrend.SubQuery{{ID: "project", Dimension: aitrend.DimensionProject, Expressions: []string{"game.genshin-impact"}, Ranking: aitrend.RankingIntent{Metric: aitrend.MetricAverageDailyActiveJobs, Order: aitrend.OrderDescending, Limit: 1}}},
		TimeRange:       aitrend.TimeRange{Kind: aitrend.RangeAll},
		Output:          aitrend.OutputIntent{SeriesMetric: aitrend.SeriesMetricDailyActiveJobs, ChartType: aitrend.ChartTypeLine},
	}
	runner, err := NewRunner(intentOnlyProvider{intent: intent}, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Run(context.Background(), "原神项目的岗位趋势"); err != nil {
		t.Fatal(err)
	}
	after, err := snapshot.ListFilterOptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("snapshot options changed after evaluation: before=%#v after=%#v", before, after)
	}
}

func TestRunnerMarksProjectTechnologyQueryAsGlobalPartialResult(t *testing.T) {
	snapshot, err := LoadSnapshot(filepath.Join("testdata", "serving-snapshot-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	intent := aitrend.Intent{
		ContractVersion: aitrend.ContractVersionV1,
		SubQueries:      []aitrend.SubQuery{{ID: "technology", Dimension: aitrend.DimensionTechnology, Expressions: []string{"*"}, Ranking: aitrend.RankingIntent{Metric: aitrend.MetricAverageDailyActiveJobs, Order: aitrend.OrderDescending, Limit: 8}}},
		TimeRange:       aitrend.TimeRange{Kind: aitrend.RangeAll},
		Output:          aitrend.OutputIntent{SeriesMetric: aitrend.SeriesMetricDailyActiveJobs, ChartType: aitrend.ChartTypeLine},
	}
	runner, err := NewRunner(intentOnlyProvider{intent: intent}, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.Run(context.Background(), "原神项目使用哪些技术？")
	if err != nil {
		t.Fatal(err)
	}
	if result.Draft == nil || result.Draft.Status != aitrend.DraftPartial || len(result.Draft.Warnings) == 0 {
		t.Fatalf("cross-dimension result = %#v", result.Draft)
	}
}
