package jobsync

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"mrolescope/internal/jdserving"
)

func TestServingReconciliationIsBestEffortAndIdempotent(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC)
	run := publishFixtureRun(t, repository, scope, at, 1, []Observation{
		integrationObservationWithTitleAndBody("服务端工程师", "serving-reconcile-v1", at),
	}, nil)

	if _, err := repository.pool.Exec(ctx, `
WITH missing_job AS (
    INSERT INTO jobs (id, source_company, source_job_id)
    VALUES (gen_random_uuid(), 'mihoyo', 'serving-missing-reference')
    RETURNING id
)
INSERT INTO job_presence_periods (
    observation_scope_id, job_id, opened_run_id, opened_at, last_seen_run_id, last_seen_at
)
SELECT $1, missing_job.id, $2, $3, $2, $3
FROM missing_job`, scope.ID, run.ID, at); err != nil {
		t.Fatalf("insert source fixture without version reference: %v", err)
	}

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)

	first, err := serving.ReconcilePublishedInventory(ctx)
	if err != nil {
		t.Fatalf("best-effort serving reconciliation returned an item failure: %v", err)
	}
	if first.ReconciledRuns != 1 || first.MembershipsCreated != 1 || first.MembershipsReused != 0 || first.RetryableFailures != 1 {
		t.Fatalf("first serving reconciliation = %#v", first)
	}

	var title, activityState, location string
	var urgent bool
	if err := repository.pool.QueryRow(ctx, `
SELECT title, activity_state::text, locations[1], is_urgent
FROM serving_jd_current_inventory_memberships
WHERE observation_scope_id = $1`, scope.ID).Scan(&title, &activityState, &location, &urgent); err != nil {
		t.Fatalf("read current serving membership: %v", err)
	}
	if title != "服务端工程师" || activityState != "effective" || location != "Synthetic city" || urgent {
		t.Fatalf("current serving membership = title:%q activity:%q location:%q urgent:%t", title, activityState, location, urgent)
	}

	var published bool
	if err := repository.pool.QueryRow(ctx, `SELECT status = 'published' FROM collection_runs WHERE id = $1`, run.ID).Scan(&published); err != nil {
		t.Fatal(err)
	}
	if !published {
		t.Fatal("serving reconciliation changed the source publication state")
	}
	var initialAudits int
	if err := repository.pool.QueryRow(ctx, `
SELECT count(*)
FROM serving_jd_reconciliation_audits
WHERE published_run_id = $1
  AND severity = 'warning'
  AND code = 'source_version_reference_missing'`, run.ID).Scan(&initialAudits); err != nil {
		t.Fatal(err)
	}
	if initialAudits != 1 {
		t.Fatalf("initial missing-reference audits = %d, want 1", initialAudits)
	}

	second, err := serving.ReconcilePublishedInventory(ctx)
	if err != nil {
		t.Fatalf("repeat best-effort serving reconciliation returned an item failure: %v", err)
	}
	if second.ReconciledRuns != 1 || second.MembershipsCreated != 0 || second.MembershipsReused != 1 || second.RetryableFailures != 1 {
		t.Fatalf("repeat serving reconciliation = %#v", second)
	}
	var memberships, retryAudits int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM serving_jd_inventory_memberships WHERE published_run_id = $1),
  (SELECT count(*) FROM serving_jd_reconciliation_audits
   WHERE published_run_id = $1 AND stage = 'retry' AND code = 'source_version_reference_missing')`, run.ID).Scan(&memberships, &retryAudits); err != nil {
		t.Fatal(err)
	}
	if memberships != 1 || retryAudits != 1 {
		t.Fatalf("repeat serving state = memberships:%d retry_audits:%d, want 1:1", memberships, retryAudits)
	}
}

func TestServingReconciliationResolvesOnlyDeterministicFormalDimensions(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 30, 4, 0, 0, 0, time.UTC)
	olderRun := publishFixtureRun(t, repository, scope, at, 1, []Observation{
		integrationObservationWithTitleAndBody("旧标题", "serving-resolution-v1", at),
	}, nil)
	newerRun := publishFixtureRun(t, repository, scope, at.Add(time.Minute), 1, []Observation{
		integrationObservationWithTitleAndBody("新标题", "serving-resolution-v2", at.Add(time.Minute)),
	}, nil)
	olderMetadataID, olderBodyID := canonicalReferenceForRun(t, repository, olderRun.ID)
	newerMetadataID, newerBodyID := canonicalReferenceForRun(t, repository, newerRun.ID)

	configurationID := insertPublishedAnalysisConfiguration(t, repository)
	aiConfigurationID := insertPublishedAISemanticConfiguration(t, repository, configurationID)
	educationID := insertServingEducationResult(t, repository, newerBodyID, "serving-current-education", "bachelor")
	experienceID := insertServingExperienceResult(t, repository, olderBodyID, "serving-fallback-experience", 3)
	technologyID := insertServingTechnologyResult(t, repository, newerBodyID, configurationID)
	projectID := insertServingProjectResult(t, repository, olderMetadataID, "serving-fallback-project")
	familyID := insertServingJobFamilyResult(t, repository, newerMetadataID, "serving-current-family")
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_semantic_results (
    body_content_version_id, ai_analysis_configuration_id,
    execution_status, semantic_status, completed_at
) VALUES ($1, $2, 'complete', 'configured', now())`, newerBodyID, aiConfigurationID); err != nil {
		t.Fatalf("insert AI result that serving must ignore: %v", err)
	}

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile deterministic serving resolutions: %v", err)
	}

	var currentMembershipID, currentBodyID string
	if err := repository.pool.QueryRow(ctx, `
SELECT id::text, body_content_version_id::text
FROM serving_jd_current_inventory_memberships
WHERE observation_scope_id = $1`, scope.ID).Scan(&currentMembershipID, &currentBodyID); err != nil {
		t.Fatalf("load current serving membership: %v", err)
	}
	if currentBodyID != newerBodyID {
		t.Fatalf("current serving membership body = %q, want %q", currentBodyID, newerBodyID)
	}

	assertServingCurrentDimension(t, repository, currentMembershipID, "education", 0, educationID)
	assertServingCurrentDimension(t, repository, currentMembershipID, "experience", 1, experienceID)
	assertServingCurrentDimension(t, repository, currentMembershipID, "technology", 0, technologyID)
	assertServingCurrentDimension(t, repository, currentMembershipID, "project", 1, projectID)
	assertServingCurrentDimension(t, repository, currentMembershipID, "job_family", 0, familyID)

	var memberships, historicalOlderBodies, aiProjectionColumns int
	if err := repository.pool.QueryRow(ctx, `
SELECT
    (SELECT count(*) FROM serving_jd_inventory_memberships WHERE observation_scope_id = $1),
    (SELECT count(*) FROM serving_jd_inventory_memberships
     WHERE published_run_id = $2 AND body_content_version_id = $3),
    (SELECT count(*) FROM information_schema.columns
     WHERE table_schema = current_schema()
       AND table_name = 'serving_jd_dimension_resolutions'
       AND column_name LIKE '%ai_semantic%')`,
		scope.ID, olderRun.ID, olderBodyID,
	).Scan(&memberships, &historicalOlderBodies, &aiProjectionColumns); err != nil {
		t.Fatalf("read serving history and AI boundary: %v", err)
	}
	if memberships != 2 || historicalOlderBodies != 1 {
		t.Fatalf("serving historical memberships = total:%d older:%d, want 2:1", memberships, historicalOlderBodies)
	}
	if aiProjectionColumns != 0 {
		t.Fatalf("serving resolution schema exposes %d AI projection columns", aiProjectionColumns)
	}
}

func TestServingListJobsComposesStructuredFilters(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	firstDay := time.Date(2026, 10, 1, 2, 0, 0, 0, time.UTC)
	publishFixtureRun(t, repository, scope, firstDay, 1, []Observation{
		integrationServingFilterObservation("filter-alpha", "Alpha 后端", "filter-alpha-body", "上海", true, firstDay),
	}, nil)
	publishFixtureRun(t, repository, scope, firstDay.AddDate(0, 0, 1), 1, []Observation{
		integrationServingFilterObservation("filter-alpha", "Alpha 后端", "filter-alpha-body", "上海", true, firstDay.AddDate(0, 0, 1)),
		integrationServingFilterObservation("filter-beta", "Beta 前端", "filter-beta-body", "北京", false, firstDay.AddDate(0, 0, 1)),
	}, nil)
	publishFixtureRun(t, repository, scope, firstDay.AddDate(0, 0, 2), 1, []Observation{
		integrationServingFilterObservation("filter-alpha", "Alpha 后端", "filter-alpha-body", "上海", true, firstDay.AddDate(0, 0, 2)),
		integrationServingFilterObservation("filter-beta", "Beta 前端", "filter-beta-body", "北京", false, firstDay.AddDate(0, 0, 2)),
		integrationServingFilterObservation("filter-gamma", "Gamma 客户端", "filter-gamma-body", "上海", true, firstDay.AddDate(0, 0, 2)),
	}, nil)
	latestRun := publishFixtureRun(t, repository, scope, firstDay.AddDate(0, 0, 3), 1, []Observation{
		integrationServingFilterObservation("filter-alpha", "Alpha 后端", "filter-alpha-body", "上海", true, firstDay.AddDate(0, 0, 3)),
		integrationServingFilterObservation("filter-beta", "Beta 前端", "filter-beta-body", "北京", false, firstDay.AddDate(0, 0, 3)),
	}, map[string]RecheckResult{
		"filter-gamma": {Outcome: RecheckDefinitelyUnavailable, Code: "serving-filter-fixture-closed"},
	})

	alphaMetadataID, alphaBodyID := servingFilterReference(t, repository, latestRun.ID, "filter-alpha")
	betaMetadataID, betaBodyID := servingFilterReference(t, repository, latestRun.ID, "filter-beta")
	gammaMetadataID, gammaBodyID := servingFilterReference(t, repository, latestRun.ID, "filter-gamma")
	configurationID := insertPublishedAnalysisConfiguration(t, repository)
	insertServingFilterFormalFacts(t, repository, alphaMetadataID, alphaBodyID, configurationID, "alpha", "backend", "security", "go", "Go", "genshin_impact", "原神", "bachelor", 3)
	insertServingFilterFormalFacts(t, repository, betaMetadataID, betaBodyID, configurationID, "beta", "frontend", "operations_sre", "python", "Python", "star_rail", "崩坏：星穹铁道", "master", 5)
	insertServingFilterFormalFacts(t, repository, gammaMetadataID, gammaBodyID, configurationID, "gamma", "client", "backend", "rust", "Rust", "", "", "doctor", 8)

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile serving filter fixture: %v", err)
	}
	options, err := serving.ListFilterOptions(ctx)
	if err != nil {
		t.Fatalf("list serving filter options: %v", err)
	}
	for name, options := range map[string][]jdserving.FilterOption{
		"job families":   options.JobFamilies,
		"main families":  options.MainJobFamilies,
		"other families": options.OtherJobFamilies,
		"projects":       options.Projects,
		"locations":      options.Locations,
		"experience":     options.ExperienceYears,
	} {
		if len(options) == 0 {
			t.Fatalf("serving %s options are unexpectedly empty", name)
		}
		for _, option := range options {
			if option.Value == "" || option.Label == "" {
				t.Fatalf("serving %s contains an incomplete option: %#v", name, option)
			}
		}
	}
	if len(options.TechnologyKeywords) == 0 {
		t.Fatal("serving technology options are unexpectedly empty")
	}
	for _, option := range options.TechnologyKeywords {
		if option.Value == "" || option.Label == "" || option.Category.Value == "" || option.Category.Label == "" {
			t.Fatalf("serving technology option is incomplete: %#v", option)
		}
	}
	if !containsServingFilterOption(options.Projects, "unrecognized") ||
		!containsServingTechnologyFilterOption(options.TechnologyKeywords, "go") ||
		!containsServingFilterOption(options.MainJobFamilies, "backend") ||
		!containsServingFilterOption(options.JobFamilies, "backend") ||
		!containsServingFilterOption(options.JobFamilies, "security") {
		t.Fatalf("serving filter options missed expected current values: %#v", options)
	}
	for _, option := range options.TechnologyKeywords {
		if option.Value == "go" && (option.Category.Value != "programming-languages" || option.Category.Label != "编程语言") {
			t.Fatalf("Go category = %#v", option.Category)
		}
	}

	trueValue := true
	secondDay := firstDay.AddDate(0, 0, 1)
	tests := []struct {
		name  string
		query jdserving.ListJobsQuery
		want  []string
	}{
		{
			name: "all dimensions intersect while same-dimension values union",
			query: jdserving.ListJobsQuery{
				ActivityStates:       []jdserving.ActivityState{jdserving.ActivityStateEffective},
				EducationLevels:      []jdserving.EducationLevel{jdserving.EducationLevelBachelor, jdserving.EducationLevelMaster},
				ExperienceYears:      []int{3, 5},
				FirstDiscoveredFrom:  &firstDay,
				FirstDiscoveredUntil: &secondDay,
				IsUrgent:             &trueValue,
				Locations:            []string{"上海", "北京"},
				MainJobFamilies:      []string{"backend", "frontend"},
				OtherJobFamilies:     []string{"security", "operations_sre"},
				Projects:             []string{"genshin_impact", "star_rail"},
				TechnologyKeywords:   []string{"go", "python"},
				Title:                "alpha",
			},
			want: []string{"Alpha 后端"},
		},
		{name: "education", query: jdserving.ListJobsQuery{EducationLevels: []jdserving.EducationLevel{jdserving.EducationLevelMaster}}, want: []string{"Beta 前端"}},
		{name: "experience", query: jdserving.ListJobsQuery{ExperienceYears: []int{5}}, want: []string{"Beta 前端"}},
		{name: "first discovered date", query: jdserving.ListJobsQuery{FirstDiscoveredFrom: &secondDay, FirstDiscoveredUntil: &secondDay}, want: []string{"Beta 前端"}},
		{name: "location", query: jdserving.ListJobsQuery{Locations: []string{"北京"}}, want: []string{"Beta 前端"}},
		{name: "main family", query: jdserving.ListJobsQuery{MainJobFamilies: []string{"frontend"}}, want: []string{"Beta 前端"}},
		{name: "other family", query: jdserving.ListJobsQuery{OtherJobFamilies: []string{"security"}}, want: []string{"Alpha 后端"}},
		{name: "unified family matches primary and other hits", query: jdserving.ListJobsQuery{IncludeAllActivityStates: true, JobFamilies: []string{"backend"}}, want: []string{"Alpha 后端", "Gamma 客户端"}},
		{name: "project", query: jdserving.ListJobsQuery{Projects: []string{"star_rail"}}, want: []string{"Beta 前端"}},
		{name: "technology", query: jdserving.ListJobsQuery{TechnologyKeywords: []string{"go"}}, want: []string{"Alpha 后端"}},
		{name: "title", query: jdserving.ListJobsQuery{Title: "beta"}, want: []string{"Beta 前端"}},
		{name: "urgent", query: jdserving.ListJobsQuery{IsUrgent: &trueValue}, want: []string{"Alpha 后端"}},
		{name: "confirmed closed", query: jdserving.ListJobsQuery{ActivityStates: []jdserving.ActivityState{jdserving.ActivityStateConfirmedClosed}}, want: []string{"Gamma 客户端"}},
		{name: "all activity states", query: jdserving.ListJobsQuery{IncludeAllActivityStates: true}, want: []string{"Alpha 后端", "Beta 前端", "Gamma 客户端"}},
		{name: "unrecognized project", query: jdserving.ListJobsQuery{ActivityStates: []jdserving.ActivityState{jdserving.ActivityStateConfirmedClosed}, Projects: []string{"unrecognized"}}, want: []string{"Gamma 客户端"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := serving.ListJobs(ctx, test.query)
			if err != nil {
				t.Fatalf("list filtered serving jobs: %v", err)
			}
			if result.Page.Number != 1 || result.Page.PageSize != 20 {
				t.Fatalf("default bounded page = %#v, want number 1 size 20", result.Page)
			}
			got := make([]string, 0, len(result.Items))
			for _, item := range result.Items {
				if item.ID == "" || item.ID == "filter-alpha" || item.ID == "filter-beta" || item.ID == "filter-gamma" {
					t.Fatalf("list returned a non-opaque ID %q", item.ID)
				}
				got = append(got, item.Title)
			}
			if strings.Join(got, "|") != strings.Join(test.want, "|") {
				t.Fatalf("filtered list titles = %#v, want %#v", got, test.want)
			}
		})
	}
}

func containsServingFilterOption(options []jdserving.FilterOption, value string) bool {
	for _, option := range options {
		if option.Value == value {
			return true
		}
	}
	return false
}

func containsServingTechnologyFilterOption(options []jdserving.TechnologyKeywordFilterOption, value string) bool {
	for _, option := range options {
		if option.Value == value {
			return true
		}
	}
	return false
}

func TestServingListJobsSearchesOnlyTitlesWithDeterministicFuzzyRanking(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 5, 2, 0, 0, 0, time.UTC)
	publishFixtureRun(t, repository, scope, at, 1, []Observation{
		integrationServingFilterObservation("title-exact", "数据开发工程师", "ordinary-body", "上海", false, at),
		integrationServingFilterObservation("title-substring", "高级数据开发工程师", "ordinary-body", "上海", false, at),
		integrationServingFilterObservation("title-fuzzy", "数聚开发工程师", "ordinary-body", "上海", false, at),
		integrationServingFilterObservation("title-body-only", "通用工程师", "body-hidden-keyword", "上海", false, at),
		integrationServingFilterObservation("title-no-synonym", "后端工程师", "ordinary-body", "上海", false, at),
	}, nil)
	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile serving title-search fixture: %v", err)
	}

	exact, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{Title: "数据开发工程师"})
	if err != nil {
		t.Fatalf("search exact and substring titles: %v", err)
	}
	exactTitles := servingTitles(exact.Items)
	if len(exactTitles) < 3 || exactTitles[0] != "数据开发工程师" || exactTitles[1] != "高级数据开发工程师" || exactTitles[2] != "数聚开发工程师" {
		t.Fatalf("exact/substr/fuzzy title rank = %#v", exactTitles)
	}

	fuzzy, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{Title: "数据开収工程师"})
	if err != nil {
		t.Fatalf("search typo title: %v", err)
	}
	if !containsServingTitle(servingTitles(fuzzy.Items), "数据开发工程师") {
		t.Fatalf("fixed-threshold fuzzy result omitted the intended title: %#v", servingTitles(fuzzy.Items))
	}

	bodyOnly, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{Title: "body-hidden-keyword"})
	if err != nil {
		t.Fatalf("search body-only text: %v", err)
	}
	if len(bodyOnly.Items) != 0 {
		t.Fatalf("title search matched a JD body: %#v", servingTitles(bodyOnly.Items))
	}

	noSynonym, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{Title: "backend"})
	if err != nil {
		t.Fatalf("search unexpanded synonym: %v", err)
	}
	if len(noSynonym.Items) != 0 {
		t.Fatalf("title search expanded a synonym: %#v", servingTitles(noSynonym.Items))
	}
}

func TestServingListJobsSortsConfiguredRequirementsBeforeUnavailableValues(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	firstDay := time.Date(2026, 10, 2, 2, 0, 0, 0, time.UTC)
	publishFixtureRun(t, repository, scope, firstDay, 1, []Observation{
		integrationServingFilterObservation("sort-unavailable", "Sort Unavailable", "sort-unavailable-body", "上海", false, firstDay),
	}, nil)
	upperRun := publishFixtureRun(t, repository, scope, firstDay.AddDate(0, 0, 1), 1, []Observation{
		integrationServingFilterObservation("sort-upper", "Sort Upper", "sort-upper-body", "上海", false, firstDay.AddDate(0, 0, 1)),
	}, nil)
	peerMasterRun := publishFixtureRun(t, repository, scope, firstDay.AddDate(0, 0, 2), 1, []Observation{
		integrationServingFilterObservation("sort-peer-master", "Sort Peer Master", "sort-peer-master-body", "上海", true, firstDay.AddDate(0, 0, 2)),
	}, nil)
	bachelorRun := publishFixtureRun(t, repository, scope, firstDay.AddDate(0, 0, 3), 1, []Observation{
		integrationServingFilterObservation("sort-bachelor", "Sort Bachelor", "sort-bachelor-body", "上海", false, firstDay.AddDate(0, 0, 3)),
	}, nil)
	masterRun := publishFixtureRun(t, repository, scope, firstDay.AddDate(0, 0, 4), 1, []Observation{
		integrationServingFilterObservation("sort-master", "Sort Master", "sort-master-body", "上海", true, firstDay.AddDate(0, 0, 4)),
		integrationServingFilterObservation("sort-bachelor", "Sort Bachelor", "sort-bachelor-body", "上海", false, firstDay.AddDate(0, 0, 4)),
		integrationServingFilterObservation("sort-unavailable", "Sort Unavailable", "sort-unavailable-body", "上海", false, firstDay.AddDate(0, 0, 4)),
	}, map[string]RecheckResult{
		"sort-peer-master": {Outcome: RecheckDefinitelyUnavailable, Code: "serving-sort-fixture-closed"},
		"sort-upper":       {Outcome: RecheckDefinitelyUnavailable, Code: "serving-sort-fixture-closed"},
	})
	_, upperBodyID := servingFilterReference(t, repository, upperRun.ID, "sort-upper")
	_, bachelorBodyID := servingFilterReference(t, repository, bachelorRun.ID, "sort-bachelor")
	_, masterBodyID := servingFilterReference(t, repository, masterRun.ID, "sort-master")
	_, peerMasterBodyID := servingFilterReference(t, repository, peerMasterRun.ID, "sort-peer-master")
	insertServingEducationUpperResult(t, repository, upperBodyID, "serving-sort-upper", "associate_or_lower")
	insertServingEducationResult(t, repository, bachelorBodyID, "serving-sort-bachelor", "bachelor")
	insertServingEducationResult(t, repository, masterBodyID, "serving-sort-master", "master")
	insertServingEducationResult(t, repository, peerMasterBodyID, "serving-sort-peer-master", "master")
	insertServingExperienceResult(t, repository, bachelorBodyID, "serving-sort-bachelor", 3)
	insertServingExperienceResult(t, repository, masterBodyID, "serving-sort-master", 5)
	insertServingExperienceResult(t, repository, peerMasterBodyID, "serving-sort-peer-master", 3)

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile serving sort fixture: %v", err)
	}

	assertServingSortRuleTitlesWithQuery(t, serving, ctx, []jdserving.SortRule{
		{Field: jdserving.SortFieldEducation, Direction: jdserving.SortDirectionAscending},
	}, []string{"Sort Upper", "Sort Bachelor", "Sort Master", "Sort Peer Master", "Sort Unavailable"})
	assertServingSortRuleTitlesWithQuery(t, serving, ctx, []jdserving.SortRule{
		{Field: jdserving.SortFieldEducation, Direction: jdserving.SortDirectionDescending},
	}, []string{"Sort Master", "Sort Peer Master", "Sort Bachelor", "Sort Upper", "Sort Unavailable"})
	assertServingSortRuleTitlesWithQuery(t, serving, ctx, []jdserving.SortRule{
		{Field: jdserving.SortFieldExperience, Direction: jdserving.SortDirectionAscending},
	}, []string{"Sort Bachelor", "Sort Peer Master", "Sort Master", "Sort Unavailable", "Sort Upper"})
	assertServingSortRuleTitlesWithQuery(t, serving, ctx, []jdserving.SortRule{
		{Field: jdserving.SortFieldExperience, Direction: jdserving.SortDirectionDescending},
	}, []string{"Sort Master", "Sort Bachelor", "Sort Peer Master", "Sort Unavailable", "Sort Upper"})
	assertServingSortRuleTitlesWithQuery(t, serving, ctx, []jdserving.SortRule{
		{Field: jdserving.SortFieldExperience, Direction: jdserving.SortDirectionDescending},
		{Field: jdserving.SortFieldEducation, Direction: jdserving.SortDirectionDescending},
	}, []string{"Sort Master", "Sort Peer Master", "Sort Bachelor", "Sort Upper", "Sort Unavailable"})
	minimumExperienceYears := 3
	thresholdResult, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{IncludeAllActivityStates: true, MinimumExperienceYears: &minimumExperienceYears})
	if err != nil {
		t.Fatalf("list configured experience threshold: %v", err)
	}
	if got := servingTitles(thresholdResult.Items); strings.Join(got, "|") != "Sort Bachelor|Sort Master|Sort Peer Master" {
		t.Fatalf("configured experience threshold titles = %#v", got)
	}
	minimumExperienceYears = 5
	thresholdResult, err = serving.ListJobs(ctx, jdserving.ListJobsQuery{IncludeAllActivityStates: true, MinimumExperienceYears: &minimumExperienceYears})
	if err != nil {
		t.Fatalf("list five-year experience threshold: %v", err)
	}
	if got := servingTitles(thresholdResult.Items); strings.Join(got, "|") != "Sort Master" {
		t.Fatalf("five-year experience threshold titles = %#v", got)
	}
	minimumExperienceYears = 0
	configuredResult, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{IncludeAllActivityStates: true, MinimumExperienceYears: &minimumExperienceYears})
	if err != nil {
		t.Fatalf("list zero experience threshold: %v", err)
	}
	if got := servingTitles(configuredResult.Items); strings.Join(got, "|") != "Sort Bachelor|Sort Master|Sort Peer Master" {
		t.Fatalf("zero experience threshold incorrectly treated unavailable values as zero: %#v", got)
	}

	ascending, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{IncludeAllActivityStates: true, Sort: jdserving.ListSortEducationAscending})
	if err != nil {
		t.Fatalf("load education display shape: %v", err)
	}
	if ascending.Items[0].Education == nil || ascending.Items[0].Education.LowerBound != "" || ascending.Items[0].Education.UpperBound != jdserving.EducationLevelAssociateOrLower {
		t.Fatalf("upper-bound education display = %#v", ascending.Items[0].Education)
	}

	firstPage, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{IncludeAllActivityStates: true, PageNumber: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("load stable first page: %v", err)
	}
	secondPage, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{IncludeAllActivityStates: true, PageNumber: 2, PageSize: 2})
	if err != nil {
		t.Fatalf("load stable second page: %v", err)
	}
	repeatedFirstPage, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{IncludeAllActivityStates: true, PageNumber: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("repeat stable first page: %v", err)
	}
	if firstPage.Page.Total != 5 || secondPage.Page.Total != 5 || strings.Join(servingTitles(firstPage.Items), "|") != strings.Join(servingTitles(repeatedFirstPage.Items), "|") {
		t.Fatalf("stable page metadata = first:%#v second:%#v repeated:%#v", firstPage.Page, secondPage.Page, repeatedFirstPage.Page)
	}
	for _, firstItem := range firstPage.Items {
		for _, secondItem := range secondPage.Items {
			if firstItem.ID == secondItem.ID {
				t.Fatalf("stable page boundary repeated opaque ID %q", firstItem.ID)
			}
		}
	}
	assertServingSortRuleTitlesWithQuery(t, serving, ctx, []jdserving.SortRule{
		{Field: jdserving.SortFieldFirstDiscovered, Direction: jdserving.SortDirectionDescending},
	}, []string{"Sort Master", "Sort Bachelor", "Sort Peer Master", "Sort Upper", "Sort Unavailable"})
	assertServingSortRuleTitlesWithQuery(t, serving, ctx, []jdserving.SortRule{
		{Field: jdserving.SortFieldUrgent, Direction: jdserving.SortDirectionDescending},
	}, []string{"Sort Master", "Sort Peer Master", "Sort Bachelor", "Sort Unavailable", "Sort Upper"})
	assertServingSortRuleTitlesWithQuery(t, serving, ctx, []jdserving.SortRule{
		{Field: jdserving.SortFieldActivityState, Direction: jdserving.SortDirectionAscending},
	}, []string{"Sort Bachelor", "Sort Master", "Sort Unavailable", "Sort Peer Master", "Sort Upper"})
	assertServingSortRuleTitlesWithQuery(t, serving, ctx, []jdserving.SortRule{
		{Field: jdserving.SortFieldActivityState, Direction: jdserving.SortDirectionDescending},
	}, []string{"Sort Peer Master", "Sort Upper", "Sort Bachelor", "Sort Master", "Sort Unavailable"})
	assertServingSortRuleTitlesWithQuery(t, serving, ctx, []jdserving.SortRule{
		{Field: jdserving.SortFieldTitle, Direction: jdserving.SortDirectionDescending},
	}, []string{"Sort Upper", "Sort Unavailable", "Sort Peer Master", "Sort Master", "Sort Bachelor"})
}

func TestServingJDDetailReadsCurrentBodyForEffectiveAndConfirmedClosedJobs(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 7, 2, 0, 0, 0, time.UTC)
	publishFixtureRun(t, repository, scope, at, 1, []Observation{
		integrationServingDetailObservation("detail-active", "详情活动岗位", "活动职责", "活动要求", "", at),
		integrationServingDetailObservation("detail-closed", "详情下线岗位", "下线职责", "下线要求", "下线补充", at),
	}, nil)
	publishFixtureRun(t, repository, scope, at.AddDate(0, 0, 1), 1, []Observation{
		integrationServingDetailObservation("detail-active", "详情活动岗位", "活动职责", "活动要求", "", at.AddDate(0, 0, 1)),
	}, map[string]RecheckResult{
		"detail-closed": {Outcome: RecheckDefinitelyUnavailable, Code: "serving-detail-fixture-closed"},
	})
	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile serving detail fixture: %v", err)
	}

	effective, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{Title: "详情活动岗位"})
	if err != nil || len(effective.Items) != 1 {
		t.Fatalf("list effective detail fixture = %#v err:%v", effective, err)
	}
	activeDetail, err := serving.GetJobDetail(ctx, effective.Items[0].ID)
	if err != nil {
		t.Fatalf("read effective detail: %v", err)
	}
	if activeDetail.ActivityState != jdserving.ActivityStateEffective || activeDetail.Body.Responsibilities != "活动职责" || activeDetail.Body.Requirements != "活动要求" || activeDetail.Body.Supplemental != nil {
		t.Fatalf("effective detail = %#v", activeDetail)
	}

	closed, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{ActivityStates: []jdserving.ActivityState{jdserving.ActivityStateConfirmedClosed}})
	if err != nil || len(closed.Items) != 1 {
		t.Fatalf("list closed detail fixture = %#v err:%v", closed, err)
	}
	closedDetail, err := serving.GetJobDetail(ctx, closed.Items[0].ID)
	if err != nil {
		t.Fatalf("read confirmed-closed detail: %v", err)
	}
	if closedDetail.ActivityState != jdserving.ActivityStateConfirmedClosed || closedDetail.Body.Responsibilities != "下线职责" || closedDetail.Body.Requirements != "下线要求" || closedDetail.Body.Supplemental == nil || *closedDetail.Body.Supplemental != "下线补充" {
		t.Fatalf("confirmed-closed detail = %#v", closedDetail)
	}
}

func TestServingListQueryPlansStayInsideServingBoundaryAtDeployedScale(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 8, 2, 0, 0, 0, time.UTC)
	observations := make([]Observation, 0, 200)
	for index := 0; index < 200; index++ {
		observations = append(observations, integrationServingFilterObservation(
			fmt.Sprintf("plan-%03d", index),
			fmt.Sprintf("Scale Query %03d", index),
			fmt.Sprintf("scale-body-%03d", index),
			"上海", false, at,
		))
	}
	run := publishFixtureRun(t, repository, scope, at, 1, observations, nil)
	metadataID, bodyID := servingFilterReference(t, repository, run.ID, "plan-010")
	configurationID := insertPublishedAnalysisConfiguration(t, repository)
	insertServingFilterFormalFacts(t, repository, metadataID, bodyID, configurationID, "plan-010", "backend", "security", "go", "Go", "", "", "master", 5)
	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile serving query-plan fixture: %v", err)
	}

	plans := []struct {
		name              string
		query             string
		args              []any
		requiredRelations []string
	}{
		{
			name: "default active location page",
			query: `
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF)
SELECT membership.title
FROM serving_jd_current_inventory_memberships AS membership
JOIN serving_jd_public_identifiers AS identifier ON identifier.job_id = membership.job_id
WHERE membership.activity_state = 'effective'
  AND membership.locations && ARRAY['上海']::text[]
ORDER BY lower(membership.title), identifier.public_id
LIMIT 20`,
		},
		{
			name: "fixed-threshold title page",
			query: `
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF)
SELECT membership.title
FROM serving_jd_current_inventory_memberships AS membership
JOIN serving_jd_public_identifiers AS identifier ON identifier.job_id = membership.job_id
WHERE membership.activity_state = 'effective'
  AND (
      lower(membership.title) LIKE '%' || lower($1::text) || '%'
      OR similarity(lower(membership.title), lower($1::text)) >= 0.30
  )
ORDER BY lower(membership.title), identifier.public_id
LIMIT 20`,
			args: []any{"Scale Query 10"},
		},
		{
			name: "unified family with two requirement sort rules",
			query: `
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF)
SELECT membership.title
FROM serving_jd_current_inventory_memberships AS membership
JOIN serving_jd_public_identifiers AS identifier ON identifier.job_id = membership.job_id
LEFT JOIN LATERAL (
    SELECT CASE
               WHEN result.status <> 'configured' THEN NULL
               WHEN COALESCE(result.lower_bound_level::text, result.upper_bound_level::text) = 'associate_or_lower' THEN 1
               WHEN COALESCE(result.lower_bound_level::text, result.upper_bound_level::text) = 'bachelor' THEN 2
               WHEN COALESCE(result.lower_bound_level::text, result.upper_bound_level::text) = 'master' THEN 3
               WHEN COALESCE(result.lower_bound_level::text, result.upper_bound_level::text) = 'doctor' THEN 4
           END AS rank
    FROM serving_jd_current_dimension_resolutions AS resolution
    JOIN education_requirement_results AS result ON result.id = resolution.education_requirement_result_id
    WHERE resolution.serving_jd_inventory_membership_id = membership.id
      AND resolution.dimension = 'education'
) AS education_sort ON true
LEFT JOIN LATERAL (
    SELECT CASE WHEN result.status = 'configured' THEN result.base_year END AS rank
    FROM serving_jd_current_dimension_resolutions AS resolution
    JOIN experience_requirement_results AS result ON result.id = resolution.experience_requirement_result_id
    WHERE resolution.serving_jd_inventory_membership_id = membership.id
      AND resolution.dimension = 'experience'
) AS experience_sort ON true
WHERE membership.activity_state = 'effective'
  AND EXISTS (
      SELECT 1
      FROM serving_jd_current_dimension_resolutions AS resolution
      JOIN job_family_results AS result ON result.id = resolution.job_family_result_id
      JOIN job_family_matches AS match ON match.job_family_result_id = result.id
      WHERE resolution.serving_jd_inventory_membership_id = membership.id
        AND resolution.dimension = 'job_family'
        AND match.family_key = ANY($1::text[])
  )
ORDER BY CASE WHEN experience_sort.rank IS NULL THEN 1 ELSE 0 END,
         experience_sort.rank DESC,
         CASE WHEN education_sort.rank IS NULL THEN 1 ELSE 0 END,
         education_sort.rank DESC,
         lower(membership.title), identifier.public_id
LIMIT 20`,
			args:              []any{[]string{"backend"}},
			requiredRelations: []string{"job_family_matches"},
		},
	}
	for _, plan := range plans {
		t.Run(plan.name, func(t *testing.T) {
			rows, err := repository.pool.Query(ctx, plan.query, plan.args...)
			if err != nil {
				t.Fatalf("run serving EXPLAIN ANALYZE: %v", err)
			}
			defer rows.Close()
			var lines []string
			for rows.Next() {
				var line string
				if err := rows.Scan(&line); err != nil {
					t.Fatalf("scan serving query plan: %v", err)
				}
				lines = append(lines, line)
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("iterate serving query plan: %v", err)
			}
			planText := strings.Join(lines, "\n")
			if !strings.Contains(planText, "Limit") {
				t.Fatalf("serving query plan lacks bounded page limit")
			}
			for _, forbiddenRelation := range []string{
				"job_source_metadata_versions", "job_body_content_versions", "jd_versions", "ai_semantic_results", "raw_fields",
			} {
				if strings.Contains(planText, forbiddenRelation) {
					t.Fatalf("serving query plan crossed into %q", forbiddenRelation)
				}
			}
			for _, requiredRelation := range plan.requiredRelations {
				if !strings.Contains(planText, requiredRelation) {
					t.Fatalf("serving query plan omitted required relation %q:\n%s", requiredRelation, planText)
				}
			}
		})
	}

	var indexCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT count(*)
FROM pg_indexes
WHERE schemaname = current_schema()
  AND indexname IN (
      'serving_jd_inventory_memberships_current_lookup_idx',
      'serving_jd_inventory_memberships_title_trgm_idx',
      'serving_jd_inventory_memberships_locations_idx',
      'serving_jd_public_identifiers_public_id_idx'
  )`).Scan(&indexCount); err != nil {
		t.Fatalf("inspect serving indexes: %v", err)
	}
	if indexCount != 4 {
		t.Fatalf("serving query-plan indexes = %d, want 4", indexCount)
	}
}

func assertServingSortRuleTitles(t *testing.T, serving *jdserving.Repository, ctx context.Context, sortRules []jdserving.SortRule, want []string) {
	t.Helper()
	result, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{SortRules: sortRules})
	if err != nil {
		t.Fatalf("list ordered rules %#v: %v", sortRules, err)
	}
	if got := servingTitles(result.Items); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("ordered rules %#v titles = %#v, want %#v", sortRules, got, want)
	}
}

func assertServingSortRuleTitlesWithQuery(t *testing.T, serving *jdserving.Repository, ctx context.Context, sortRules []jdserving.SortRule, want []string) {
	t.Helper()
	result, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{
		IncludeAllActivityStates: true,
		SortRules:                sortRules,
	})
	if err != nil {
		t.Fatalf("list ordered rules %#v across states: %v", sortRules, err)
	}
	if got := servingTitles(result.Items); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("ordered rules %#v across states titles = %#v, want %#v", sortRules, got, want)
	}
}

func containsServingTitle(titles []string, want string) bool {
	for _, title := range titles {
		if title == want {
			return true
		}
	}
	return false
}

func servingTitles(items []jdserving.JobListItem) []string {
	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.Title)
	}
	return titles
}

func integrationServingFilterObservation(sourceJobID, title, bodyVersion, location string, urgent bool, at time.Time) Observation {
	observation, err := NormalizeObservation(SourceCompanyMihoyo,
		map[string]any{"id": sourceJobID, "addressDetailList": []any{map[string]any{"addressId": "1", "addressDetail": location}}},
		map[string]any{
			"id": sourceJobID, "title": title, "hurry": urgent,
			"description": "Synthetic description " + bodyVersion,
			"jobRequire":  "Synthetic requirement",
			"addition":    "Synthetic addition",
		}, at)
	if err != nil {
		panic(err)
	}
	return observation
}

func integrationServingDetailObservation(sourceJobID, title, description, requirement, addition string, at time.Time) Observation {
	observation, err := NormalizeObservation(SourceCompanyMihoyo,
		map[string]any{"id": sourceJobID, "addressDetailList": []any{map[string]any{"addressId": "1", "addressDetail": "上海"}}},
		map[string]any{
			"id": sourceJobID, "title": title,
			"description": description,
			"jobRequire":  requirement,
			"addition":    addition,
		}, at)
	if err != nil {
		panic(err)
	}
	return observation
}

func servingFilterReference(t *testing.T, repository *Repository, runID, sourceJobID string) (metadataID, bodyID string) {
	t.Helper()
	if err := repository.pool.QueryRow(context.Background(), `
SELECT reference.source_metadata_version_id::text, reference.body_content_version_id::text
FROM run_job_version_references AS reference
JOIN jobs AS job ON job.id = reference.job_id
JOIN collection_runs AS observed_run ON observed_run.id = reference.run_id
WHERE observed_run.observation_scope_id = (
          SELECT observation_scope_id FROM collection_runs WHERE id = $1
      )
  AND job.source_job_id = $2
ORDER BY reference.observed_at DESC, reference.run_id DESC
LIMIT 1`, runID, sourceJobID).Scan(&metadataID, &bodyID); err != nil {
		t.Fatalf("load serving filter source reference for %q: %v", sourceJobID, err)
	}
	return metadataID, bodyID
}

func insertServingFilterFormalFacts(
	t *testing.T,
	repository *Repository,
	metadataID, bodyID, configurationID, suffix, primaryFamily, otherFamily, technologyKey, technologyName, projectKey, projectName, educationLevel string,
	experienceYears int,
) {
	t.Helper()
	insertServingEducationResult(t, repository, bodyID, "serving-filter-education-"+suffix, educationLevel)
	insertServingExperienceResult(t, repository, bodyID, "serving-filter-experience-"+suffix, experienceYears)
	technologyResultID := insertServingTechnologyResult(t, repository, bodyID, configurationID)
	if _, err := repository.pool.Exec(context.Background(), `
INSERT INTO technology_identities (technology_key, canonical_name, technology_kind)
VALUES ($1, $2, 'programming-language')
	ON CONFLICT (technology_key) DO NOTHING`, technologyKey, technologyName); err != nil {
		t.Fatalf("insert serving filter technology identity: %v", err)
	}
	var technologyID string
	if err := repository.pool.QueryRow(context.Background(), `
SELECT id::text FROM technology_identities WHERE technology_key = $1`, technologyKey).Scan(&technologyID); err != nil {
		t.Fatalf("load serving filter technology identity: %v", err)
	}
	if _, err := repository.pool.Exec(context.Background(), `
INSERT INTO technology_mentions (analysis_result_id, technology_identity_id)
VALUES ($1, $2)`, technologyResultID, technologyID); err != nil {
		t.Fatalf("insert serving filter technology mention: %v", err)
	}
	if projectKey != "" {
		projectResultID := insertServingProjectResult(t, repository, metadataID, "serving-filter-project-"+suffix)
		if _, err := repository.pool.Exec(context.Background(), `
INSERT INTO project_tag_assignments (project_tag_result_id, project_key, display_name, classification)
VALUES ($1, $2, $3, 'named_game')`, projectResultID, projectKey, projectName); err != nil {
			t.Fatalf("insert serving filter project assignment: %v", err)
		}
	}
	familyResultID := insertServingFilterJobFamilyResult(t, repository, metadataID, "serving-filter-family-"+suffix, primaryFamily)
	for _, family := range []string{primaryFamily, otherFamily} {
		if _, err := repository.pool.Exec(context.Background(), `
INSERT INTO job_family_matches (job_family_result_id, family_key, match_count)
VALUES ($1, $2, 1)`, familyResultID, family); err != nil {
			t.Fatalf("insert serving filter family match: %v", err)
		}
	}
}

func insertServingFilterJobFamilyResult(t *testing.T, repository *Repository, metadataID, normalizedTitle, primaryFamily string) string {
	t.Helper()
	ctx := context.Background()
	var titleIdentityID, resultID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO job_family_title_identities (title_sha256, normalization_version, normalized_title)
VALUES (encode(digest('serving-filter-family-' || $1, 'sha256'), 'hex'), 'serving-filter-v1', $2)
RETURNING id::text`, metadataID, normalizedTitle).Scan(&titleIdentityID); err != nil {
		t.Fatalf("insert serving filter job-family title identity: %v", err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO job_family_observations (
    source_metadata_version_id, job_family_title_identity_id, status
) VALUES ($1, $2, 'in_scope')`, metadataID, titleIdentityID); err != nil {
		t.Fatalf("insert serving filter job-family observation: %v", err)
	}
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO job_family_results (
    job_family_title_identity_id,
    parser_version, dictionary_version, dictionary_sha256,
    project_mask_parser_version, project_mask_dictionary_version, project_mask_dictionary_sha256,
    status, primary_family_key
) VALUES (
    $1, 'serving-filter-family', 'serving-filter-dictionary', $2,
    'serving-filter-project-mask', 'serving-filter-project-dictionary', $3,
    'classified', $4
)
RETURNING id::text`, titleIdentityID, strings.Repeat("f", 64), strings.Repeat("0", 64), primaryFamily).Scan(&resultID); err != nil {
		t.Fatalf("insert serving filter job-family result: %v", err)
	}
	return resultID
}

func assertServingCurrentDimension(t *testing.T, repository *Repository, membershipID, dimension string, wantPriority int16, wantResultID string) {
	t.Helper()
	var priority int16
	var resultID string
	if err := repository.pool.QueryRow(context.Background(), `
SELECT selection_priority,
       COALESCE(
           education_requirement_result_id::text,
           experience_requirement_result_id::text,
           technology_analysis_result_id::text,
           project_tag_result_id::text,
           job_family_result_id::text
       )
FROM serving_jd_current_dimension_resolutions
WHERE serving_jd_inventory_membership_id = $1
  AND dimension = $2::serving_jd_formal_dimension`, membershipID, dimension).Scan(&priority, &resultID); err != nil {
		t.Fatalf("load current %s resolution: %v", dimension, err)
	}
	if priority != wantPriority || resultID != wantResultID {
		t.Fatalf("current %s resolution = priority:%d result:%q, want priority:%d result:%q", dimension, priority, resultID, wantPriority, wantResultID)
	}
}

func insertServingEducationResult(t *testing.T, repository *Repository, bodyID, parserVersion, level string) string {
	t.Helper()
	var resultID string
	if err := repository.pool.QueryRow(context.Background(), `
INSERT INTO education_requirement_results (
    body_content_version_id, parser_version, lexicon_version, lexicon_sha256,
    status, lower_bound_level
) VALUES ($1, $2, 'serving-resolution-lexicon', $3, 'configured', $4::deterministic_education_level)
RETURNING id::text`, bodyID, parserVersion, strings.Repeat("a", 64), level).Scan(&resultID); err != nil {
		t.Fatalf("insert deterministic education result: %v", err)
	}
	return resultID
}

func insertServingEducationUpperResult(t *testing.T, repository *Repository, bodyID, parserVersion, level string) string {
	t.Helper()
	var resultID string
	if err := repository.pool.QueryRow(context.Background(), `
INSERT INTO education_requirement_results (
    body_content_version_id, parser_version, lexicon_version, lexicon_sha256,
    status, upper_bound_level
) VALUES ($1, $2, 'serving-resolution-lexicon', $3, 'configured', $4::deterministic_education_level)
RETURNING id::text`, bodyID, parserVersion, strings.Repeat("a", 64), level).Scan(&resultID); err != nil {
		t.Fatalf("insert deterministic upper-bound education result: %v", err)
	}
	return resultID
}

func insertServingExperienceResult(t *testing.T, repository *Repository, bodyID, parserVersion string, years int) string {
	t.Helper()
	var resultID string
	if err := repository.pool.QueryRow(context.Background(), `
INSERT INTO experience_requirement_results (
    body_content_version_id, parser_version, lexicon_version, lexicon_sha256,
    status, base_year
) VALUES ($1, $2, 'serving-resolution-lexicon', $3, 'configured', $4)
RETURNING id::text`, bodyID, parserVersion, strings.Repeat("b", 64), years).Scan(&resultID); err != nil {
		t.Fatalf("insert deterministic experience result: %v", err)
	}
	return resultID
}

func insertServingTechnologyResult(t *testing.T, repository *Repository, bodyID, configurationID string) string {
	t.Helper()
	var resultID string
	if err := repository.pool.QueryRow(context.Background(), `
INSERT INTO technology_analysis_results (
    body_content_version_id, parser_version, dictionary_version_id,
    configuration_id, status, completed_at
)
SELECT $1, configuration.parser_version, configuration.dictionary_version_id,
       configuration.id, 'complete', now()
FROM technology_analysis_configurations AS configuration
WHERE configuration.id = $2
RETURNING id::text`, bodyID, configurationID).Scan(&resultID); err != nil {
		t.Fatalf("insert deterministic technology result: %v", err)
	}
	return resultID
}

func insertServingProjectResult(t *testing.T, repository *Repository, metadataID, normalizedTitle string) string {
	t.Helper()
	ctx := context.Background()
	var titleIdentityID, resultID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO project_title_identities (title_sha256, normalization_version, normalized_title)
VALUES (encode(digest('serving-project-resolution-' || $1, 'sha256'), 'hex'), 'serving-resolution-v1', $2)
RETURNING id::text`, metadataID, normalizedTitle).Scan(&titleIdentityID); err != nil {
		t.Fatalf("insert project title identity: %v", err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO project_title_observations (source_metadata_version_id, project_title_identity_id)
VALUES ($1, $2)`, metadataID, titleIdentityID); err != nil {
		t.Fatalf("insert project observation: %v", err)
	}
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO project_tag_results (
    project_title_identity_id, parser_version, dictionary_version, dictionary_sha256, status
) VALUES ($1, 'serving-resolution-project', 'serving-resolution-dictionary', $2, 'complete')
RETURNING id::text`, titleIdentityID, strings.Repeat("c", 64)).Scan(&resultID); err != nil {
		t.Fatalf("insert deterministic project result: %v", err)
	}
	return resultID
}

func insertServingJobFamilyResult(t *testing.T, repository *Repository, metadataID, normalizedTitle string) string {
	t.Helper()
	ctx := context.Background()
	var titleIdentityID, resultID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO job_family_title_identities (title_sha256, normalization_version, normalized_title)
VALUES (encode(digest('serving-family-resolution-' || $1, 'sha256'), 'hex'), 'serving-resolution-v1', $2)
RETURNING id::text`, metadataID, normalizedTitle).Scan(&titleIdentityID); err != nil {
		t.Fatalf("insert job-family title identity: %v", err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO job_family_observations (
    source_metadata_version_id, job_family_title_identity_id, status
) VALUES ($1, $2, 'in_scope')`, metadataID, titleIdentityID); err != nil {
		t.Fatalf("insert job-family observation: %v", err)
	}
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO job_family_results (
    job_family_title_identity_id,
    parser_version, dictionary_version, dictionary_sha256,
    project_mask_parser_version, project_mask_dictionary_version, project_mask_dictionary_sha256,
    status, primary_family_key
) VALUES (
    $1, 'serving-resolution-family', 'serving-resolution-dictionary', $2,
    'serving-resolution-project-mask', 'serving-resolution-project-dictionary', $3,
    'classified', 'backend'
)
RETURNING id::text`, titleIdentityID, strings.Repeat("d", 64), strings.Repeat("e", 64)).Scan(&resultID); err != nil {
		t.Fatalf("insert deterministic job-family result: %v", err)
	}
	return resultID
}
