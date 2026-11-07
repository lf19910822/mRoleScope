package jobsync

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRepairPublishedObservationCopiesFormalMembershipAndIsIdempotent(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	sourceAt := time.Date(2026, 11, 7, 13, 20, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	sourceDate := time.Date(2026, 11, 7, 0, 0, 0, 0, sourceAt.Location())
	source := publishFixtureRun(t, repository, scope, sourceAt, 1, []Observation{integrationObservation("repair-job", sourceAt)}, nil)
	insertRepairServingMembership(t, repository, scope, source)

	targetAt := time.Date(2026, 11, 6, 0, 0, 0, 0, sourceAt.Location())
	first, err := repository.RepairPublishedObservation(ctx, scope, sourceDate, targetAt)
	if err != nil {
		t.Fatalf("repair published observation: %v", err)
	}
	if first.AlreadyRepaired || first.MembershipsCopied != 1 || first.SourceDate != "2026-11-07" || first.TargetDate != "2026-11-06" {
		t.Fatalf("first repair summary = %#v", first)
	}

	var repairedRunID string
	var trigger string
	var members, references int
	if err := repository.pool.QueryRow(ctx, `
SELECT run.id::text, run.trigger_kind::text,
       (SELECT count(*) FROM serving_jd_inventory_memberships WHERE published_run_id = run.id),
       (SELECT count(*) FROM run_job_version_references WHERE run_id = run.id)
FROM collection_runs AS run
WHERE run.observation_scope_id = $1
  AND run.data_date = DATE '2026-11-06'
  AND run.trigger_kind = 'repair'`, scope.ID).Scan(&repairedRunID, &trigger, &members, &references); err != nil {
		t.Fatalf("read repaired publication: %v", err)
	}
	if trigger != string(TriggerRepair) || members != 1 || references != 1 || repairedRunID == source.ID {
		t.Fatalf("repaired publication = id:%s trigger:%s members:%d refs:%d", repairedRunID, trigger, members, references)
	}

	second, err := repository.RepairPublishedObservation(ctx, scope, sourceDate, targetAt)
	if err != nil {
		t.Fatalf("repeat repair: %v", err)
	}
	if !second.AlreadyRepaired || second.MembershipsCopied != 1 {
		t.Fatalf("repeat repair summary = %#v", second)
	}
	var repairRuns, repairAudits int
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM collection_runs WHERE observation_scope_id = $1 AND trigger_kind = 'repair'`, scope.ID).Scan(&repairRuns); err != nil {
		t.Fatal(err)
	}
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM collection_run_audit_events WHERE run_id = $1 AND code = 'historical_observation_repaired'`, repairedRunID).Scan(&repairAudits); err != nil {
		t.Fatal(err)
	}
	if repairRuns != 1 || repairAudits != 1 {
		t.Fatalf("repair was duplicated: runs=%d audits=%d", repairRuns, repairAudits)
	}
}

func TestRepairPublishedObservationRejectsExistingPublishedTarget(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	sourceAt := time.Date(2026, 11, 7, 13, 20, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	sourceDate := time.Date(2026, 11, 7, 0, 0, 0, 0, sourceAt.Location())
	targetAt := time.Date(2026, 11, 6, 13, 20, 0, 0, sourceAt.Location())
	publishFixtureRun(t, repository, scope, targetAt, 1, []Observation{integrationObservation("existing-target", targetAt)}, nil)
	publishFixtureRun(t, repository, scope, sourceAt, 1, []Observation{integrationObservation("repair-source", sourceAt)}, nil)
	_, err = repository.RepairPublishedObservation(ctx, scope, sourceDate, time.Date(2026, 11, 6, 0, 0, 0, 0, sourceAt.Location()))
	if !errors.Is(err, ErrRepairTargetPublished) {
		t.Fatalf("existing target error = %v, want %v", err, ErrRepairTargetPublished)
	}
}

func insertRepairServingMembership(t *testing.T, repository *Repository, scope StoredScope, run Run) {
	t.Helper()
	ctx := context.Background()
	var jobID, presenceID, metadataID, bodyID string
	if err := repository.pool.QueryRow(ctx, `
SELECT reference.job_id::text, presence.id::text,
       reference.source_metadata_version_id::text, reference.body_content_version_id::text
FROM run_job_version_references AS reference
JOIN job_presence_periods AS presence
  ON presence.observation_scope_id = $2 AND presence.job_id = reference.job_id
WHERE reference.run_id = $1`, run.ID, scope.ID).Scan(&jobID, &presenceID, &metadataID, &bodyID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO serving_jd_inventory_memberships (
    published_run_id, observation_scope_id, job_id, presence_period_id,
    source_metadata_version_id, body_content_version_id, title, locations
) VALUES ($1, $2, $3, $4, $5, $6, 'repair fixture', ARRAY['上海'])`, run.ID, scope.ID, jobID, presenceID, metadataID, bodyID); err != nil {
		t.Fatal(err)
	}
}
