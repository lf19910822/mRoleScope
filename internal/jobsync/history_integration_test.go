package jobsync

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"mrolescope/internal/jdserving"
)

func TestJobHistoryUsesDailyFinalRunsAndConfirmedLifecycleBoundaries(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, time.November, 1, 1, 0, 0, 0, location)
	publishFixtureRun(t, repository, scope, base, 1, []Observation{
		integrationServingDetailObservation("history-timeline", "历史岗位", "旧职责", "旧要求", "旧补充", base),
	}, nil)
	// Same business day: this later successful publication is the only row
	// eligible for the day and intentionally has the same source content.
	publishFixtureRun(t, repository, scope, base.Add(2*time.Hour), 2, []Observation{
		integrationServingDetailObservation("history-timeline", "历史岗位", "旧职责", "旧要求", "旧补充", base.Add(2*time.Hour)),
	}, nil)
	changedAt := base.AddDate(0, 0, 2)
	publishFixtureRun(t, repository, scope, changedAt, 1, []Observation{
		integrationServingDetailObservation("history-timeline", "新岗位", "新职责", "旧要求", "新补充", changedAt),
	}, nil)
	closedAt := base.AddDate(0, 0, 3)
	publishFixtureRun(t, repository, scope, closedAt, 1, nil, map[string]RecheckResult{
		"history-timeline": {Outcome: RecheckDefinitelyUnavailable, Code: "history-fixture-closed"},
	})
	reopenedAt := base.AddDate(0, 0, 5)
	publishFixtureRun(t, repository, scope, reopenedAt, 1, []Observation{
		integrationServingDetailObservation("history-timeline", "重新上线岗位", "新职责", "旧要求", "新补充", reopenedAt),
	}, nil)

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("create serving repository: %v", err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatalf("reconcile serving history fixture: %v", err)
	}
	current, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{Title: "重新上线岗位"})
	if err != nil || len(current.Items) != 1 {
		t.Fatalf("read current history fixture: %#v err:%v", current, err)
	}
	history, err := serving.GetJobHistory(ctx, current.Items[0].ID)
	if err != nil {
		t.Fatalf("read job history: %v", err)
	}
	if len(history.Nodes) != 4 {
		t.Fatalf("history nodes = %#v, want discovered, modified, closed, reopened/latest", history.Nodes)
	}
	if current.Items[0].LastModifiedDate == nil || *current.Items[0].LastModifiedDate != history.Nodes[3].Date {
		t.Fatalf("list last modified date = %v, want reopened history date %s", current.Items[0].LastModifiedDate, history.Nodes[3].Date)
	}
	if history.Nodes[0].Type != jdserving.HistoryNodeDiscovered || history.Nodes[0].Date != "2026-11-01" ||
		history.Nodes[1].Type != jdserving.HistoryNodeModified || history.Nodes[1].Date != "2026-11-03" ||
		history.Nodes[2].Type != jdserving.HistoryNodeConfirmedClosed || history.Nodes[2].Date != "2026-11-04" ||
		history.Nodes[3].Type != jdserving.HistoryNodeReopened || history.Nodes[3].Date != "2026-11-06" || history.Nodes[3].ConnectPrevious {
		t.Fatalf("history lifecycle nodes = %#v", history.Nodes)
	}
	if _, err := serving.GetJobHistoryDetail(ctx, current.Items[0].ID, time.Date(2026, time.November, 3, 0, 0, 0, 0, location), jdserving.HistoryNodeModified); err != nil {
		t.Fatalf("read adjacent modified detail: %v", err)
	}
}

func TestListLastModifiedUsesHistorySelectedScope(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	firstScope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	otherScopeDefinition := MihoyoTechnicalScope()
	otherScopeDefinition.Filter = map[string]any{"fixture": "other-scope"}
	otherScopeDefinition.FilterSHA256 = strings.Repeat("a", 64)
	secondScope, err := repository.EnsureScope(ctx, otherScopeDefinition)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, time.November, 1, 1, 0, 0, 0, time.UTC)
	observe := func(body string, at time.Time) []Observation {
		return []Observation{integrationServingDetailObservation("scope-shared", "范围测试岗位", body, "要求", "", at)}
	}
	publishFixtureRun(t, repository, firstScope, base, 1, observe("旧", base), nil)
	changedAt := base.AddDate(0, 0, 1)
	publishFixtureRun(t, repository, firstScope, changedAt, 1, observe("新", changedAt), nil)
	otherAt := base.AddDate(0, 0, 2)
	publishFixtureRun(t, repository, secondScope, otherAt, 1, observe("新", otherAt), nil)
	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatal(err)
	}
	result, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{Title: "范围测试岗位"})
	if err != nil || len(result.Items) != 2 {
		t.Fatalf("scope list = %#v, err=%v", result, err)
	}
	for _, item := range result.Items {
		if item.LastModifiedDate != nil {
			t.Fatalf("non-selected scope leaked modification: %#v", item)
		}
		history, err := serving.GetJobHistory(ctx, item.ID)
		if err != nil || len(history.Nodes) != 1 || history.Nodes[0].Type != jdserving.HistoryNodeDiscovered {
			t.Fatalf("selected scope history = %#v, err=%v", history, err)
		}
	}
}

func TestListLastModifiedMatchesHistoryAndSortsBeforePagination(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, time.November, 1, 1, 0, 0, 0, location)
	observe := func(id, title, body string, at time.Time) Observation {
		return integrationServingDetailObservation(id, title, body, "要求", "", at)
	}
	publish := func(at time.Time, attempt int, bodyA, bodyB string, includeA bool, rechecks map[string]RecheckResult) {
		observations := []Observation{observe("last-b", "排序乙", bodyB, at), observe("last-c", "排序丙", "未变", at)}
		if includeA {
			observations = append(observations, observe("last-a", "排序甲", bodyA, at))
		}
		publishFixtureRun(t, repository, scope, at, attempt, observations, rechecks)
	}
	publish(base, 1, "旧", "旧", true, nil)
	secondDay := base.AddDate(0, 0, 2)
	publish(secondDay, 1, "新", "临时", true, nil)
	publish(secondDay.Add(time.Hour), 2, "新", "旧", true, nil)
	publish(base.AddDate(0, 0, 3), 1, "新", "新", true, nil)
	publish(base.AddDate(0, 0, 4), 1, "新", "新", false, map[string]RecheckResult{
		"last-a": {Outcome: RecheckDefinitelyUnavailable, Code: "test-closed"},
	})
	publish(base.AddDate(0, 0, 6), 1, "新", "新", true, nil)

	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		sort  string
		want  []string
		dates []string
	}{
		{"last_modified_desc", []string{"排序甲", "排序乙", "排序丙"}, []string{"2026-11-07", "2026-11-04", ""}},
		{"last_modified_asc", []string{"排序乙", "排序甲", "排序丙"}, []string{"2026-11-04", "2026-11-07", ""}},
	} {
		rules, _, err := jdserving.ParseListSortRules([]string{test.sort})
		if err != nil {
			t.Fatal(err)
		}
		for index, wantTitle := range test.want {
			result, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{Title: "排序", SortRules: rules, PageNumber: index + 1, PageSize: 1})
			if err != nil || len(result.Items) != 1 {
				t.Fatalf("sort %s page %d: %#v %v", test.sort, index+1, result, err)
			}
			item := result.Items[0]
			if item.Title != wantTitle || (test.dates[index] == "" && item.LastModifiedDate != nil) ||
				(test.dates[index] != "" && (item.LastModifiedDate == nil || *item.LastModifiedDate != test.dates[index])) {
				t.Fatalf("sort %s page %d: %#v", test.sort, index+1, item)
			}
			history, err := serving.GetJobHistory(ctx, item.ID)
			if err != nil {
				t.Fatal(err)
			}
			lastDate := ""
			for _, node := range history.Nodes {
				if node.Type == jdserving.HistoryNodeModified || node.Type == jdserving.HistoryNodeReopened {
					lastDate = node.Date
				}
			}
			if lastDate != test.dates[index] {
				t.Fatalf("history/list mismatch for %s: history=%q list=%q", item.Title, lastDate, test.dates[index])
			}
		}
	}
	rules, _, err := jdserving.ParseListSortRules([]string{"last_modified_desc", "title_desc"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{Title: "排序", SortRules: rules, PageNumber: 1, PageSize: 3})
	if err != nil || len(result.Items) != 3 || result.Items[0].Title != "排序甲" || result.Items[1].Title != "排序乙" || result.Items[2].Title != "排序丙" {
		t.Fatalf("multi-rule sort = %#v, err=%v", result, err)
	}
	rules, _, err = jdserving.ParseListSortRules([]string{"title_asc", "last_modified_desc"})
	if err != nil {
		t.Fatal(err)
	}
	result, err = serving.ListJobs(ctx, jdserving.ListJobsQuery{Title: "排序", SortRules: rules, PageNumber: 1, PageSize: 3})
	if err != nil || len(result.Items) != 3 || result.Items[0].Title != "排序丙" || result.Items[1].Title != "排序乙" || result.Items[2].Title != "排序甲" {
		t.Fatalf("primary title sort = %#v, err=%v", result, err)
	}
	repairSource := time.Date(2026, time.November, 7, 0, 0, 0, 0, location)
	repairTarget := repairSource.AddDate(0, 0, -1)
	if _, err := repository.RepairPublishedObservation(ctx, scope, repairSource, repairTarget); err != nil {
		t.Fatalf("repair historical observation: %v", err)
	}
	result, err = serving.ListJobs(ctx, jdserving.ListJobsQuery{Title: "排序甲"})
	if err != nil || len(result.Items) == 0 || result.Items[0].Title != "排序甲" || result.Items[0].LastModifiedDate == nil || *result.Items[0].LastModifiedDate != "2026-11-06" {
		t.Fatalf("repaired list date = %#v, err=%v", result, err)
	}
	history, err := serving.GetJobHistory(ctx, result.Items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	lastDate := ""
	for _, node := range history.Nodes {
		if node.Type == jdserving.HistoryNodeModified || node.Type == jdserving.HistoryNodeReopened {
			lastDate = node.Date
		}
	}
	if lastDate != *result.Items[0].LastModifiedDate {
		t.Fatalf("repaired history/list mismatch: history=%s list=%s", lastDate, *result.Items[0].LastModifiedDate)
	}
}
