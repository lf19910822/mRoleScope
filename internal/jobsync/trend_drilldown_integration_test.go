package jobsync

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"mrolescope/internal/jdserving"
)

func TestTrendDrilldownUsesFinalPublishedRunForTheNodeDay(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}

	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(shanghai)
	firstPublication := time.Date(today.Year(), today.Month(), today.Day(), 1, 0, 0, 0, shanghai)
	finalPublication := firstPublication.Add(time.Hour)
	publishFixtureRun(t, repository, scope, firstPublication, 1, []Observation{
		integrationServingFilterObservation("drilldown-first", "Earlier platform role", "drilldown-first-body", "上海", false, firstPublication),
	}, nil)
	publishFixtureRun(t, repository, scope, finalPublication, 1, []Observation{
		integrationServingFilterObservation("drilldown-final", "Final platform role", "drilldown-final-body", "上海", false, finalPublication),
	}, nil)

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile published inventory: %v", err)
	}

	page, err := serving.ReadTrendDrilldown(ctx, jdserving.TrendDrilldownQuery{
		Condition:     jdserving.TrendCondition{Dimension: jdserving.TrendDimensionLocation, Value: "上海"},
		ExpectedCount: 1,
		NodeDate:      time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, shanghai),
		PageNumber:    1,
		PageSize:      20,
	})
	if err != nil {
		t.Fatalf("read trend drilldown: %v", err)
	}
	if page.NodeDate != today.Format("2006-01-02") || page.MatchedTotal != 1 || page.Page != (jdserving.PageInfo{Number: 1, PageSize: 20, Total: 1}) {
		t.Fatalf("drilldown page = %#v", page)
	}
	if len(page.Items) != 1 || page.Items[0].Title != "Final platform role" ||
		page.Items[0].Status != jdserving.TrendDrilldownStatusCurrent || !page.Items[0].CanViewLatest || page.Items[0].CanViewHistorical {
		t.Fatalf("drilldown items = %#v", page.Items)
	}
}

func TestHistoricalJobDetailUsesTheNodeDateVersionWithoutLatestFallback(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(shanghai)
	historicalAt := time.Date(today.Year(), today.Month(), today.Day()-1, 2, 0, 0, 0, shanghai)
	latestAt := historicalAt.AddDate(0, 0, 1)
	publishFixtureRun(t, repository, scope, historicalAt, 1, []Observation{
		integrationServingDetailObservation("historical-detail", "历史岗位名称", "历史职责", "历史要求", "", historicalAt),
	}, nil)
	publishFixtureRun(t, repository, scope, latestAt, 1, []Observation{
		integrationServingDetailObservation("historical-detail", "最新岗位名称", "最新职责", "最新要求", "", latestAt),
	}, nil)

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile published inventory: %v", err)
	}
	latest, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{Title: "最新岗位名称"})
	if err != nil || len(latest.Items) != 1 {
		t.Fatalf("list current detail = %#v err:%v", latest, err)
	}
	nodeDate := time.Date(historicalAt.Year(), historicalAt.Month(), historicalAt.Day(), 0, 0, 0, 0, shanghai)
	historical, err := serving.GetHistoricalJobDetail(ctx, latest.Items[0].ID, nodeDate)
	if err != nil {
		t.Fatalf("read historical detail: %v", err)
	}
	if historical.Title != "历史岗位名称" || historical.Body.Responsibilities != "历史职责" || historical.Body.Requirements != "历史要求" {
		t.Fatalf("historical detail = %#v", historical)
	}
}

func TestTrendDrilldownReturnsATrustedEmptyPageForNumericZero(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(shanghai)
	publishedAt := time.Date(today.Year(), today.Month(), today.Day(), 3, 0, 0, 0, shanghai)
	publishFixtureRun(t, repository, scope, publishedAt, 1, []Observation{
		integrationServingFilterObservation("drilldown-zero", "Shanghai only", "drilldown-zero-body", "上海", false, publishedAt),
	}, nil)
	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile published inventory: %v", err)
	}

	page, err := serving.ReadTrendDrilldown(ctx, jdserving.TrendDrilldownQuery{
		Condition:     jdserving.TrendCondition{Dimension: jdserving.TrendDimensionLocation, Value: "北京"},
		ExpectedCount: 0,
		NodeDate:      time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, shanghai),
		PageNumber:    1,
		PageSize:      20,
	})
	if err != nil {
		t.Fatalf("read zero trend drilldown: %v", err)
	}
	if page.MatchedTotal != 0 || len(page.Items) != 0 || page.Page != (jdserving.PageInfo{Number: 1, PageSize: 20, Total: 0}) {
		t.Fatalf("trusted zero drilldown = %#v", page)
	}
}

func TestTrendDrilldownComparesCurrentStatusAndKeepsSearchSeparateFromNodeConsistency(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}

	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(shanghai)
	nodeDate := time.Date(today.Year(), today.Month(), today.Day()-1, 0, 0, 0, 0, shanghai)
	historicalAt := nodeDate.Add(2 * time.Hour)
	latestAt := time.Date(today.Year(), today.Month(), today.Day(), 2, 0, 0, 0, shanghai)
	publishFixtureRun(t, repository, scope, historicalAt, 1, []Observation{
		integrationServingFilterObservation("drilldown-stable", "Alpha 稳定岗位", "stable-body", "上海", false, historicalAt),
		integrationServingFilterObservation("drilldown-changed", "Beta 变更岗位", "changed-body-before", "上海", false, historicalAt),
		integrationServingFilterObservation("drilldown-closed", "Gamma 下线岗位", "closed-body", "上海", false, historicalAt),
	}, nil)
	publishFixtureRun(t, repository, scope, latestAt, 1, []Observation{
		integrationServingFilterObservation("drilldown-stable", "Alpha 稳定岗位", "stable-body", "上海", false, latestAt),
		integrationServingFilterObservation("drilldown-changed", "Beta 变更岗位（更新）", "changed-body-after", "上海", false, latestAt),
	}, map[string]RecheckResult{
		"drilldown-closed": {Outcome: RecheckDefinitelyUnavailable, Code: "fixture-closed"},
	})

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile published inventory: %v", err)
	}

	page, err := serving.ReadTrendDrilldown(ctx, jdserving.TrendDrilldownQuery{
		Condition:     jdserving.TrendCondition{Dimension: jdserving.TrendDimensionLocation, Value: "上海"},
		ExpectedCount: 3,
		NodeDate:      nodeDate,
		PageNumber:    1,
		PageSize:      20,
	})
	if err != nil {
		t.Fatalf("read historical node: %v", err)
	}
	if page.MatchedTotal != 3 || page.CountMatchesExpected == nil || !*page.CountMatchesExpected {
		t.Fatalf("unsearched node consistency = %#v, want matching total of 3", page)
	}
	got := append([]jdserving.TrendDrilldownItem(nil), page.Items...)
	sort.Slice(got, func(left, right int) bool { return got[left].Title < got[right].Title })
	if len(got) != 3 ||
		got[0].Status != jdserving.TrendDrilldownStatusCurrent || !got[0].CanViewLatest || got[0].CanViewHistorical ||
		got[1].Status != jdserving.TrendDrilldownStatusChanged || !got[1].CanViewLatest || !got[1].CanViewHistorical ||
		got[2].Status != jdserving.TrendDrilldownStatusClosed || got[2].CanViewLatest || !got[2].CanViewHistorical {
		t.Fatalf("status/action mapping = %#v", got)
	}

	filtered, err := serving.ReadTrendDrilldown(ctx, jdserving.TrendDrilldownQuery{
		Condition:     jdserving.TrendCondition{Dimension: jdserving.TrendDimensionLocation, Value: "上海"},
		ExpectedCount: 3,
		NodeDate:      nodeDate,
		PageNumber:    1,
		PageSize:      1,
		Title:         "  beta  ",
	})
	if err != nil {
		t.Fatalf("search historical node: %v", err)
	}
	if filtered.MatchedTotal != 1 || filtered.Page != (jdserving.PageInfo{Number: 1, PageSize: 1, Total: 1}) ||
		len(filtered.Items) != 1 || filtered.Items[0].Status != jdserving.TrendDrilldownStatusChanged ||
		filtered.CountMatchesExpected != nil {
		t.Fatalf("searched node page = %#v", filtered)
	}

	mismatched, err := serving.ReadTrendDrilldown(ctx, jdserving.TrendDrilldownQuery{
		Condition:     jdserving.TrendCondition{Dimension: jdserving.TrendDimensionLocation, Value: "上海"},
		ExpectedCount: 4,
		NodeDate:      nodeDate,
		PageNumber:    2,
		PageSize:      1,
	})
	if err != nil {
		t.Fatalf("read mismatched node: %v", err)
	}
	if mismatched.Page != (jdserving.PageInfo{Number: 2, PageSize: 1, Total: 3}) || len(mismatched.Items) != 1 ||
		mismatched.CountMatchesExpected == nil || *mismatched.CountMatchesExpected {
		t.Fatalf("mismatched node page = %#v", mismatched)
	}
}
