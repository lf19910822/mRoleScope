package jobsync

import (
	"context"
	"os"
	"testing"
	"time"

	"mrolescope/internal/jdanalysis"
	"mrolescope/internal/jdserving"
)

func TestTrendEventStatsUsesBaselineMissingDaysAndEventDrilldown(t *testing.T) {
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
	day := func(day int) time.Time {
		return time.Date(2026, time.October, day, 0, 0, 0, 0, shanghai)
	}
	publishAt := func(dayNumber int) time.Time { return day(dayNumber).Add(2 * time.Hour) }

	publishFixtureRun(t, repository, scope, publishAt(22), 1, []Observation{
		integrationServingFilterObservation("event-alpha", "Alpha", "alpha-v1", "上海", false, publishAt(22)),
	}, nil)
	publishFixtureRun(t, repository, scope, publishAt(23), 1, []Observation{
		integrationServingFilterObservation("event-alpha", "Alpha", "alpha-v2", "上海", false, publishAt(23)),
	}, nil)
	// 10 月 24 日故意没有发布观察。跨越该缺失日的状态转移只能归属到
	// 10 月 25 日这个后续可信观察日，不得伪造 10 月 24 日事件。
	publishFixtureRun(t, repository, scope, publishAt(25), 1, []Observation{
		integrationServingFilterObservation("event-alpha", "Alpha", "alpha-v2", "北京", false, publishAt(25)),
		integrationServingFilterObservation("event-beta", "Beta", "beta-v1", "上海", false, publishAt(25)),
	}, nil)
	publishFixtureRun(t, repository, scope, publishAt(26), 1, nil, map[string]RecheckResult{
		"event-alpha": {Outcome: RecheckDefinitelyUnavailable, Code: "event-fixture-closed"},
	})
	publishFixtureRun(t, repository, scope, publishAt(27), 1, []Observation{
		integrationServingFilterObservation("event-alpha", "Alpha", "alpha-v3", "上海", false, publishAt(27)),
		integrationServingFilterObservation("event-beta", "Beta", "beta-v2", "上海", false, publishAt(27)),
	}, nil)

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile event fixture: %v", err)
	}

	query := jdserving.TrendEventStatsQuery{
		Condition: jdserving.TrendCondition{
			Dimension: jdserving.TrendDimensionLocation,
			Value:     "上海",
		},
		EndDate:   day(27),
		Range:     jdserving.TrendRangeCustom,
		StartDate: day(22),
	}
	stats, err := serving.ReadTrendEventStats(ctx, query)
	if err != nil {
		t.Fatalf("read trend event stats: %v", err)
	}
	if stats.Status != "available" || stats.Current.Start != "2026-10-22" || stats.Current.End != "2026-10-27" {
		t.Fatalf("event stats envelope = %#v", stats)
	}
	if stats.Previous.Start != "2026-10-16" || stats.Previous.End != "2026-10-21" {
		t.Fatalf("event stats previous period = %#v", stats.Previous)
	}

	wantCounts := map[jdserving.TrendEventType]int{
		jdserving.TrendEventEntered:  2,
		jdserving.TrendEventModified: 2,
		jdserving.TrendEventLeft:     1,
		jdserving.TrendEventReopened: 1,
	}
	for _, row := range stats.Rows {
		if row.Current != wantCounts[row.Type] {
			t.Errorf("event %s current count = %d, want %d", row.Type, row.Current, wantCounts[row.Type])
		}
		if row.Previous.Count == nil || *row.Previous.Count != 0 || row.Previous.Rate != nil {
			t.Errorf("event %s previous comparison = %#v, want zero with no rate", row.Type, row.Previous)
		}
		if row.YearOverYear.Count == nil || *row.YearOverYear.Count != 0 || row.YearOverYear.Rate != nil {
			t.Errorf("event %s year-over-year comparison = %#v, want zero with no rate", row.Type, row.YearOverYear)
		}
	}

	entered, err := serving.ReadTrendDrilldown(ctx, jdserving.TrendDrilldownQuery{
		Condition:      query.Condition,
		EventEndDate:   day(27),
		EventStartDate: day(22),
		EventType:      jdserving.TrendEventEntered,
		ExpectedCount:  2,
		PageNumber:     1,
		PageSize:       1,
	})
	if err != nil {
		t.Fatalf("read entered event drilldown: %v", err)
	}
	if entered.Mode != "event" || entered.MatchedTotal != 2 || entered.Page.Total != 2 || len(entered.Items) != 1 {
		t.Fatalf("entered drilldown page = %#v", entered)
	}
	if entered.Items[0].EventDate != "2026-10-22" || entered.Items[0].Reason != "历史基线进入" {
		t.Fatalf("entered baseline item = %#v", entered.Items[0])
	}
	if entered.CountMatchesExpected == nil || !*entered.CountMatchesExpected {
		t.Fatalf("entered count consistency = %#v", entered.CountMatchesExpected)
	}

	left, err := serving.ReadTrendDrilldown(ctx, jdserving.TrendDrilldownQuery{
		Condition:      query.Condition,
		EventEndDate:   day(27),
		EventStartDate: day(22),
		EventType:      jdserving.TrendEventLeft,
		ExpectedCount:  1,
		PageNumber:     1,
		PageSize:       20,
	})
	if err != nil {
		t.Fatalf("read left event drilldown: %v", err)
	}
	if left.MatchedTotal != 1 || len(left.Items) != 1 || left.Items[0].EventDate != "2026-10-25" || left.Items[0].Reason != "修改后移出条件" {
		t.Fatalf("left event drilldown = %#v", left)
	}

	modified, err := serving.ReadTrendDrilldown(ctx, jdserving.TrendDrilldownQuery{
		Condition:      query.Condition,
		EventEndDate:   day(27),
		EventStartDate: day(22),
		EventType:      jdserving.TrendEventModified,
		ExpectedCount:  2,
		PageNumber:     1,
		PageSize:       20,
		Title:          "Beta",
	})
	if err != nil {
		t.Fatalf("read modified event drilldown: %v", err)
	}
	if modified.MatchedTotal != 1 || len(modified.Items) != 1 || modified.Items[0].Reason != "曲线内修改" || modified.CountMatchesExpected != nil {
		t.Fatalf("modified event search = %#v", modified)
	}

	reopened, err := serving.ReadTrendDrilldown(ctx, jdserving.TrendDrilldownQuery{
		Condition:      query.Condition,
		EventEndDate:   day(27),
		EventStartDate: day(22),
		EventType:      jdserving.TrendEventReopened,
		ExpectedCount:  1,
		PageNumber:     1,
		PageSize:       20,
	})
	if err != nil {
		t.Fatalf("read reopened event drilldown: %v", err)
	}
	if reopened.MatchedTotal != 1 || len(reopened.Items) != 1 || reopened.Items[0].EventDate != "2026-10-27" || reopened.Items[0].Reason != "重新上线" {
		t.Fatalf("reopened event drilldown = %#v", reopened)
	}
}

func TestTrendEventStatsUsesPublishedFormalDimensionsAndSecondaryFamilies(t *testing.T) {
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
	day := func(dayNumber int) time.Time {
		return time.Date(2026, time.October, dayNumber, 0, 0, 0, 0, shanghai)
	}
	publishAt := func(dayNumber int) time.Time { return day(dayNumber).Add(2 * time.Hour) }

	firstRun := publishFixtureRun(t, repository, scope, publishAt(22), 1, []Observation{
		integrationServingFormalFilterObservation("formal-event", "原神客户端后端工程师", "formal-event-v1", "上海", publishAt(22)),
	}, nil)
	secondRun := publishFixtureRun(t, repository, scope, publishAt(23), 1, []Observation{
		integrationServingFormalFilterObservation("formal-event", "原神客户端后端工程师更新", "formal-event-v2", "北京", publishAt(23)),
	}, nil)
	_, firstBody := trendEventReferenceAtRun(t, repository, firstRun.ID, "formal-event")
	_, secondBody := trendEventReferenceAtRun(t, repository, secondRun.ID, "formal-event")
	configurationID := insertPublishedAnalysisConfiguration(t, repository)
	firstTechnologyResult := insertServingTechnologyResult(t, repository, firstBody, configurationID)
	secondTechnologyResult := insertServingTechnologyResult(t, repository, secondBody, configurationID)
	insertServingTechnologyMention(t, repository, firstTechnologyResult, "go", "Go")
	insertServingTechnologyMention(t, repository, secondTechnologyResult, "go", "Go")

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile formal event fixture: %v", err)
	}

	analysis, err := jdanalysis.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create analysis repository: %v", err)
	}
	t.Cleanup(analysis.Close)
	if _, err := jdanalysis.ProcessPendingEducationRequirements(ctx, analysis); err != nil {
		t.Fatalf("process education requirements: %v", err)
	}
	if _, err := jdanalysis.ProcessPendingExperienceRequirements(ctx, analysis); err != nil {
		t.Fatalf("process experience requirements: %v", err)
	}
	if _, err := jdanalysis.ProcessPendingProjectTags(ctx, analysis); err != nil {
		t.Fatalf("process project tags: %v", err)
	}
	if _, err := jdanalysis.ProcessPendingJobFamilies(ctx, analysis); err != nil {
		t.Fatalf("process job families: %v", err)
	}
	candidate, err := analysis.PrepareTrendFormalRelease(ctx)
	if err != nil {
		t.Fatalf("prepare formal event release: %v", err)
	}
	if _, err := analysis.PublishTrendFormalRelease(ctx, candidate.ID); err != nil {
		t.Fatalf("publish formal event release: %v", err)
	}

	tests := []struct {
		name      string
		condition jdserving.TrendCondition
		entered   int
		modified  int
		left      int
	}{
		{name: "technology", condition: jdserving.TrendCondition{Dimension: jdserving.TrendDimensionTechnology, Value: "go"}, entered: 1, modified: 1},
		{name: "project", condition: jdserving.TrendCondition{Dimension: jdserving.TrendDimensionProject, Value: "game.genshin-impact"}, entered: 1, modified: 1},
		{name: "location", condition: jdserving.TrendCondition{Dimension: jdserving.TrendDimensionLocation, Value: "上海"}, entered: 1, left: 1},
		{name: "location entered", condition: jdserving.TrendCondition{Dimension: jdserving.TrendDimensionLocation, Value: "北京"}, entered: 1},
		{name: "job family primary", condition: jdserving.TrendCondition{Dimension: jdserving.TrendDimensionJobFamily, Value: "backend"}, entered: 1, modified: 1},
		{name: "job family secondary", condition: jdserving.TrendCondition{Dimension: jdserving.TrendDimensionJobFamily, IncludeSecondary: true, Value: "client"}, entered: 1, modified: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stats, err := serving.ReadTrendEventStats(ctx, jdserving.TrendEventStatsQuery{
				Condition: test.condition,
				EndDate:   day(23),
				Range:     jdserving.TrendRangeCustom,
				StartDate: day(22),
			})
			if err != nil {
				t.Fatalf("read %s event stats: %v", test.name, err)
			}
			if stats.Status != "available" {
				t.Fatalf("%s event stats status = %q", test.name, stats.Status)
			}
			counts := make(map[jdserving.TrendEventType]int, len(stats.Rows))
			for _, row := range stats.Rows {
				counts[row.Type] = row.Current
			}
			if counts[jdserving.TrendEventEntered] != test.entered ||
				counts[jdserving.TrendEventModified] != test.modified ||
				counts[jdserving.TrendEventLeft] != test.left ||
				counts[jdserving.TrendEventReopened] != 0 {
				t.Fatalf("%s event counts = %#v", test.name, counts)
			}
		})
	}
}

func trendEventReferenceAtRun(t *testing.T, repository *Repository, runID, sourceJobID string) (metadataID, bodyID string) {
	t.Helper()
	if err := repository.pool.QueryRow(context.Background(), `
SELECT reference.source_metadata_version_id::text, reference.body_content_version_id::text
FROM run_job_version_references AS reference
JOIN jobs AS job ON job.id = reference.job_id
WHERE reference.run_id = $1
  AND job.source_job_id = $2`, runID, sourceJobID).Scan(&metadataID, &bodyID); err != nil {
		t.Fatalf("load trend event fixture reference for %q: %v", sourceJobID, err)
	}
	return metadataID, bodyID
}

func integrationServingFormalFilterObservation(sourceJobID, title, bodyVersion, location string, at time.Time) Observation {
	observation, err := NormalizeObservation(SourceCompanyMihoyo,
		map[string]any{"id": sourceJobID, "addressDetailList": []any{map[string]any{"addressId": "1", "addressDetail": location}}},
		map[string]any{
			"id": sourceJobID, "title": title, "competencyTypeId": "1", "competencyType": "程序&技术类",
			"description": "Synthetic description " + bodyVersion,
			"jobRequire":  "Synthetic requirement",
			"addition":    "Synthetic addition",
		}, at)
	if err != nil {
		panic(err)
	}
	return observation
}
