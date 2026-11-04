package jobsync

import (
	"context"
	"os"
	"testing"
	"time"

	"mrolescope/internal/jdanalysis"
	"mrolescope/internal/jdserving"
)

func TestTrendReaderUsesFinalPublishedRunPerShanghaiDay(t *testing.T) {
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
		integrationServingFilterObservation("trend-alpha", "Alpha", "trend-alpha-body", "上海", false, firstPublication),
	}, nil)
	publishFixtureRun(t, repository, scope, finalPublication, 1, []Observation{
		integrationServingFilterObservation("trend-alpha", "Alpha", "trend-alpha-body", "上海", false, finalPublication),
		integrationServingFilterObservation("trend-beta", "Beta", "trend-beta-body", "上海", false, finalPublication),
	}, nil)

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile published inventory: %v", err)
	}

	series, err := serving.ReadTrend(ctx, jdserving.TrendQuery{
		Range: jdserving.TrendRangeAll,
		Condition: jdserving.TrendCondition{
			Dimension: jdserving.TrendDimensionLocation,
			Value:     "上海",
		},
	})
	if err != nil {
		t.Fatalf("read trend: %v", err)
	}

	point, found := trendPointOn(series.Points, today.Format("2006-01-02"))
	if !found {
		t.Fatalf("trend has no point for today: %#v", series.Points)
	}
	if point.Count == nil || *point.Count != 2 {
		t.Fatalf("today count = %#v, want final same-day publication count 2", point.Count)
	}
}

func TestTrendReaderDistinguishesObservedZeroFromUnavailableHistory(t *testing.T) {
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
	completeDay := time.Date(today.Year(), today.Month(), today.Day()-4, 1, 0, 0, 0, shanghai)
	incompleteDay := completeDay.AddDate(0, 0, 2)
	publishFixtureRun(t, repository, scope, completeDay, 1, []Observation{
		integrationServingFilterObservation("trend-zero", "Zero", "trend-zero-body", "上海", false, completeDay),
	}, nil)
	incompleteRun := publishFixtureRun(t, repository, scope, incompleteDay, 1, []Observation{
		integrationServingFilterObservation("trend-incomplete", "Incomplete", "trend-incomplete-body", "上海", false, incompleteDay),
	}, nil)
	if _, err := repository.pool.Exec(ctx, `
WITH missing_job AS (
    INSERT INTO jobs (id, source_company, source_job_id)
    VALUES (gen_random_uuid(), 'mihoyo', 'trend-unrecoverable-reference')
    RETURNING id
)
INSERT INTO job_presence_periods (
    observation_scope_id, job_id, opened_run_id, opened_at, last_seen_run_id, last_seen_at
)
SELECT $1, missing_job.id, $2, $3, $2, $3
FROM missing_job`, scope.ID, incompleteRun.ID, incompleteDay); err != nil {
		t.Fatalf("insert unrecoverable source member: %v", err)
	}

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile published inventory: %v", err)
	}

	series, err := serving.ReadTrend(ctx, jdserving.TrendQuery{
		Range: jdserving.TrendRangeAll,
		Condition: jdserving.TrendCondition{
			Dimension: jdserving.TrendDimensionLocation,
			Value:     "北京",
		},
	})
	if err != nil {
		t.Fatalf("read trend: %v", err)
	}

	assertTrendPointCount(t, series.Points, completeDay.Format("2006-01-02"), 0)
	assertTrendPointUnavailable(t, series.Points, completeDay.AddDate(0, 0, 1).Format("2006-01-02"))
	assertTrendPointUnavailable(t, series.Points, incompleteDay.Format("2006-01-02"))
}

func TestTrendReaderCountsOnlyEffectiveJobsInTheSelectedHistoricalRun(t *testing.T) {
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
		integrationServingFilterObservation("trend-closed", "Closed", "trend-closed-body", "上海", false, firstPublication),
	}, nil)
	publishFixtureRun(t, repository, scope, finalPublication, 1, []Observation{
		integrationServingFilterObservation("trend-effective", "Effective", "trend-effective-body", "上海", false, finalPublication),
	}, map[string]RecheckResult{
		"trend-closed": {Outcome: RecheckDefinitelyUnavailable, Code: "trend-fixture-closed"},
	})

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile published inventory: %v", err)
	}

	series, err := serving.ReadTrend(ctx, jdserving.TrendQuery{
		Range: jdserving.TrendRangeAll,
		Condition: jdserving.TrendCondition{
			Dimension: jdserving.TrendDimensionLocation,
			Value:     "上海",
		},
	})
	if err != nil {
		t.Fatalf("read trend: %v", err)
	}
	assertTrendPointCount(t, series.Points, today.Format("2006-01-02"), 1)
}

func TestTrendReaderCountsEducationUsingThePublishedFormalRelease(t *testing.T) {
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
	publishedAt := time.Date(time.Now().In(shanghai).Year(), time.Now().In(shanghai).Month(), time.Now().In(shanghai).Day(), 1, 0, 0, 0, shanghai)
	run := publishFixtureRun(t, repository, scope, publishedAt, 1, []Observation{
		integrationServingDetailObservation("trend-education", "服务端工程师", "岗位职责", "本科及以上", "", publishedAt),
	}, nil)
	_, bodyID := servingFilterReference(t, repository, run.ID, "trend-education")
	configurationID := insertPublishedAnalysisConfiguration(t, repository)
	insertServingTechnologyResult(t, repository, bodyID, configurationID)

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile published inventory: %v", err)
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
		t.Fatalf("prepare trend formal release: %v", err)
	}
	if _, err := analysis.PublishTrendFormalRelease(ctx, candidate.ID); err != nil {
		t.Fatalf("publish trend formal release: %v", err)
	}

	series, err := serving.ReadTrend(ctx, jdserving.TrendQuery{
		Range: jdserving.TrendRangeWeek,
		Condition: jdserving.TrendCondition{
			Dimension: jdserving.TrendDimensionEducation,
			Operator:  jdserving.TrendComparisonGreaterThanOrEqual,
			Value:     "bachelor",
		},
	})
	if err != nil {
		t.Fatalf("read education trend: %v", err)
	}
	assertTrendPointCount(t, series.Points, publishedAt.Format("2006-01-02"), 1)
}

func TestTrendReaderCountsExperienceUsingThePublishedFormalRelease(t *testing.T) {
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
	publishedAt := time.Date(time.Now().In(shanghai).Year(), time.Now().In(shanghai).Month(), time.Now().In(shanghai).Day(), 2, 0, 0, 0, shanghai)
	run := publishFixtureRun(t, repository, scope, publishedAt, 1, []Observation{
		integrationServingDetailObservation("trend-experience", "服务端工程师", "岗位职责", "本科及以上，3年以上工作经验", "", publishedAt),
	}, nil)
	_, bodyID := servingFilterReference(t, repository, run.ID, "trend-experience")
	configurationID := insertPublishedAnalysisConfiguration(t, repository)
	insertServingTechnologyResult(t, repository, bodyID, configurationID)

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile published inventory: %v", err)
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
		t.Fatalf("prepare trend formal release: %v", err)
	}
	if _, err := analysis.PublishTrendFormalRelease(ctx, candidate.ID); err != nil {
		t.Fatalf("publish trend formal release: %v", err)
	}

	series, err := serving.ReadTrend(ctx, jdserving.TrendQuery{
		Range: jdserving.TrendRangeAll,
		Condition: jdserving.TrendCondition{
			Dimension: jdserving.TrendDimensionExperience,
			Operator:  jdserving.TrendComparisonGreaterThanOrEqual,
			Value:     "3",
		},
	})
	if err != nil {
		t.Fatalf("read experience trend: %v", err)
	}
	assertTrendPointCount(t, series.Points, publishedAt.Format("2006-01-02"), 1)
}

func TestTrendReaderCountsCanonicalTechnologyUsingThePublishedFormalRelease(t *testing.T) {
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
	publishedAt := time.Date(time.Now().In(shanghai).Year(), time.Now().In(shanghai).Month(), time.Now().In(shanghai).Day(), 3, 0, 0, 0, shanghai)
	run := publishFixtureRun(t, repository, scope, publishedAt, 1, []Observation{
		integrationServingDetailObservation("trend-go", "服务端工程师", "使用 Golang 建设服务", "本科及以上，3年以上工作经验", "", publishedAt),
	}, nil)
	_, bodyID := servingFilterReference(t, repository, run.ID, "trend-go")
	configurationID := insertPublishedAnalysisConfiguration(t, repository)
	technologyResultID := insertServingTechnologyResult(t, repository, bodyID, configurationID)
	insertServingTechnologyMention(t, repository, technologyResultID, "go", "Go")

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile published inventory: %v", err)
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
		t.Fatalf("prepare trend formal release: %v", err)
	}
	if _, err := analysis.PublishTrendFormalRelease(ctx, candidate.ID); err != nil {
		t.Fatalf("publish trend formal release: %v", err)
	}

	series, err := serving.ReadTrend(ctx, jdserving.TrendQuery{
		Range: jdserving.TrendRangeAll,
		Condition: jdserving.TrendCondition{
			Dimension: jdserving.TrendDimensionTechnology,
			Value:     "go",
		},
	})
	if err != nil {
		t.Fatalf("read technology trend: %v", err)
	}
	assertTrendPointCount(t, series.Points, publishedAt.Format("2006-01-02"), 1)
}

func TestTrendReaderCountsProjectUsingThePublishedFormalRelease(t *testing.T) {
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
	publishedAt := time.Date(time.Now().In(shanghai).Year(), time.Now().In(shanghai).Month(), time.Now().In(shanghai).Day(), 4, 0, 0, 0, shanghai)
	run := publishFixtureRun(t, repository, scope, publishedAt, 1, []Observation{
		integrationServingDetailObservation("trend-project", "后端开发工程师-原神", "岗位职责", "本科及以上，3年以上工作经验", "", publishedAt),
	}, nil)
	_, bodyID := servingFilterReference(t, repository, run.ID, "trend-project")
	configurationID := insertPublishedAnalysisConfiguration(t, repository)
	insertServingTechnologyResult(t, repository, bodyID, configurationID)

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile published inventory: %v", err)
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
		t.Fatalf("prepare trend formal release: %v", err)
	}
	if _, err := analysis.PublishTrendFormalRelease(ctx, candidate.ID); err != nil {
		t.Fatalf("publish trend formal release: %v", err)
	}

	series, err := serving.ReadTrend(ctx, jdserving.TrendQuery{
		Range: jdserving.TrendRangeAll,
		Condition: jdserving.TrendCondition{
			Dimension: jdserving.TrendDimensionProject,
			Value:     "game.genshin-impact",
		},
	})
	if err != nil {
		t.Fatalf("read project trend: %v", err)
	}
	assertTrendPointCount(t, series.Points, publishedAt.Format("2006-01-02"), 1)
}

func TestTrendReaderCountsJobFamilyPrimaryAndSecondaryUsingThePublishedFormalRelease(t *testing.T) {
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
	publishedAt := time.Date(time.Now().In(shanghai).Year(), time.Now().In(shanghai).Month(), time.Now().In(shanghai).Day(), 5, 0, 0, 0, shanghai)
	run := publishFixtureRun(t, repository, scope, publishedAt, 1, []Observation{
		integrationTrendInScopeDetailObservation("trend-family", "客户端安全开发", "岗位职责", "本科及以上，3年以上工作经验", "", publishedAt),
	}, nil)
	_, bodyID := servingFilterReference(t, repository, run.ID, "trend-family")
	configurationID := insertPublishedAnalysisConfiguration(t, repository)
	insertServingTechnologyResult(t, repository, bodyID, configurationID)

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile published inventory: %v", err)
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
		t.Fatalf("prepare trend formal release: %v", err)
	}
	if _, err := analysis.PublishTrendFormalRelease(ctx, candidate.ID); err != nil {
		t.Fatalf("publish trend formal release: %v", err)
	}

	primaryOnly, err := serving.ReadTrend(ctx, jdserving.TrendQuery{
		Range: jdserving.TrendRangeAll,
		Condition: jdserving.TrendCondition{
			Dimension: jdserving.TrendDimensionJobFamily,
			Value:     "client",
		},
	})
	if err != nil {
		t.Fatalf("read primary-only job-family trend: %v", err)
	}
	assertTrendPointCount(t, primaryOnly.Points, publishedAt.Format("2006-01-02"), 0)

	includingSecondary, err := serving.ReadTrend(ctx, jdserving.TrendQuery{
		Range: jdserving.TrendRangeAll,
		Condition: jdserving.TrendCondition{
			Dimension:        jdserving.TrendDimensionJobFamily,
			IncludeSecondary: true,
			Value:            "client",
		},
	})
	if err != nil {
		t.Fatalf("read primary-and-secondary job-family trend: %v", err)
	}
	assertTrendPointCount(t, includingSecondary.Points, publishedAt.Format("2006-01-02"), 1)
}

func TestTrendReaderLeavesDayUnavailableWhenPublishedFormalEducationIsMissing(t *testing.T) {
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
	firstPublication := time.Date(today.Year(), today.Month(), today.Day()-1, 1, 0, 0, 0, shanghai)
	firstRun := publishFixtureRun(t, repository, scope, firstPublication, 1, []Observation{
		integrationServingDetailObservation("trend-gap", "服务端工程师", "第一版职责", "本科及以上", "", firstPublication),
	}, nil)
	_, firstBodyID := servingFilterReference(t, repository, firstRun.ID, "trend-gap")
	configurationID := insertPublishedAnalysisConfiguration(t, repository)
	insertServingTechnologyResult(t, repository, firstBodyID, configurationID)

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile first published inventory: %v", err)
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
		t.Fatalf("prepare trend formal release: %v", err)
	}
	if _, err := analysis.PublishTrendFormalRelease(ctx, candidate.ID); err != nil {
		t.Fatalf("publish trend formal release: %v", err)
	}

	secondPublication := firstPublication.AddDate(0, 0, 1)
	publishFixtureRun(t, repository, scope, secondPublication, 1, []Observation{
		integrationServingDetailObservation("trend-gap", "服务端工程师", "第二版职责", "本科及以上", "", secondPublication),
	}, nil)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile second published inventory: %v", err)
	}

	series, err := serving.ReadTrend(ctx, jdserving.TrendQuery{
		Range: jdserving.TrendRangeAll,
		Condition: jdserving.TrendCondition{
			Dimension: jdserving.TrendDimensionEducation,
			Operator:  jdserving.TrendComparisonGreaterThanOrEqual,
			Value:     "bachelor",
		},
	})
	if err != nil {
		t.Fatalf("read education trend: %v", err)
	}
	assertTrendPointCount(t, series.Points, firstPublication.Format("2006-01-02"), 1)
	assertTrendPointUnavailable(t, series.Points, secondPublication.Format("2006-01-02"))
}

func TestTrendReaderUsesFixedWeekCalendarWindow(t *testing.T) {
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
	publishedAt := time.Date(today.Year(), today.Month(), today.Day(), 6, 0, 0, 0, shanghai)
	publishFixtureRun(t, repository, scope, publishedAt, 1, []Observation{
		integrationServingFilterObservation("trend-week", "本周岗位", "trend-week-body", "上海", false, publishedAt),
	}, nil)

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile published inventory: %v", err)
	}

	series, err := serving.ReadTrend(ctx, jdserving.TrendQuery{
		Range: jdserving.TrendRangeWeek,
		Condition: jdserving.TrendCondition{
			Dimension: jdserving.TrendDimensionLocation,
			Value:     "上海",
		},
	})
	if err != nil {
		t.Fatalf("read week trend: %v", err)
	}
	weekStart := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7)).Format("2006-01-02")
	if len(series.Points) == 0 || series.Points[0].Date != weekStart {
		t.Fatalf("week trend first point = %#v, want Monday %s", series.Points, weekStart)
	}
	if series.Points[len(series.Points)-1].Date != today.Format("2006-01-02") {
		t.Fatalf("week trend last point = %#v, want today %s", series.Points, today.Format("2006-01-02"))
	}
	assertTrendPointCount(t, series.Points, today.Format("2006-01-02"), 1)
}

func trendPointOn(points []jdserving.TrendPoint, date string) (jdserving.TrendPoint, bool) {
	for _, point := range points {
		if point.Date == date {
			return point, true
		}
	}
	return jdserving.TrendPoint{}, false
}

func assertTrendPointCount(t *testing.T, points []jdserving.TrendPoint, date string, want int) {
	t.Helper()
	point, found := trendPointOn(points, date)
	if !found || point.Count == nil || *point.Count != want {
		t.Fatalf("trend count on %s = %#v, want %d", date, point.Count, want)
	}
}

func assertTrendPointUnavailable(t *testing.T, points []jdserving.TrendPoint, date string) {
	t.Helper()
	point, found := trendPointOn(points, date)
	if !found || point.Count != nil {
		t.Fatalf("trend point on %s = %#v, want unavailable", date, point.Count)
	}
}

func integrationTrendInScopeDetailObservation(sourceJobID, title, description, requirement, addition string, at time.Time) Observation {
	observation, err := NormalizeObservation(SourceCompanyMihoyo,
		map[string]any{
			"id":                sourceJobID,
			"addressDetailList": []any{map[string]any{"addressId": "1", "addressDetail": "上海"}},
		},
		map[string]any{
			"id": sourceJobID, "title": title, "competencyTypeId": "1",
			"description": description, "jobRequire": requirement, "addition": addition,
		}, at)
	if err != nil {
		panic(err)
	}
	return observation
}
