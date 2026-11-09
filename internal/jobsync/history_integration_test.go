package jobsync

import (
	"context"
	"os"
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
