package jobsync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	// ErrRepairSourceUnavailable is intentionally stable so the worker can
	// report a bounded failure without leaking database details.
	ErrRepairSourceUnavailable = errors.New("repair source publication unavailable")
	// ErrRepairTargetPublished prevents a repair from rewriting an existing
	// historical fact. A repeated identical repair is handled idempotently.
	ErrRepairTargetPublished = errors.New("repair target publication already exists")
	ErrRepairInvalidDates    = errors.New("repair dates are invalid")
)

// RepairSummary is safe for a controlled worker log. It contains dates and
// aggregate counts only; run and version identifiers remain internal.
type RepairSummary struct {
	SourceDate        string
	TargetDate        string
	MembershipsCopied int
	AlreadyRepaired   bool
}

// RepairPublishedObservation creates a formal historical observation for a
// missing calendar date by copying an existing successful publication in the
// same observation scope. It never moves the current publication pointer.
// The operation is transactionally idempotent per scope and target date.
func (repository *Repository) RepairPublishedObservation(ctx context.Context, scope StoredScope, sourceDate, targetDate time.Time) (RepairSummary, error) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return RepairSummary{}, fmt.Errorf("load repair calendar location: %w", err)
	}
	source := calendarDate(sourceDate, shanghai)
	target := calendarDate(targetDate, shanghai)
	if sourceDate.IsZero() || targetDate.IsZero() || !isMidnight(sourceDate, shanghai) || !isMidnight(targetDate, shanghai) || source.Equal(target) {
		return RepairSummary{}, ErrRepairInvalidDates
	}
	if repository == nil || repository.pool == nil || scope.ID == "" {
		return RepairSummary{}, fmt.Errorf("repair repository is unavailable")
	}

	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return RepairSummary{}, fmt.Errorf("begin historical observation repair: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	lockKey := "historical-repair:" + scope.ID + ":" + target.Format("2006-01-02")
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return RepairSummary{}, fmt.Errorf("lock historical observation repair: %w", err)
	}

	var existingRunID, existingTrigger, existingSourceDate string
	var existingMemberships int
	err = tx.QueryRow(ctx, `
SELECT run.id::text,
       run.trigger_kind::text,
       COALESCE((
           SELECT audit.diagnostics->>'source_date'
           FROM collection_run_audit_events AS audit
           WHERE audit.run_id = run.id
             AND audit.code = 'historical_observation_repaired'
           ORDER BY audit.occurred_at DESC
           LIMIT 1
       ), ''),
       (SELECT count(*)::int FROM serving_jd_inventory_memberships AS membership WHERE membership.published_run_id = run.id)
FROM collection_runs AS run
WHERE run.observation_scope_id = $1
  AND run.status = 'published'
  AND (run.published_at AT TIME ZONE 'Asia/Shanghai')::date = $2::date
ORDER BY run.published_at DESC, run.id DESC
LIMIT 1`, scope.ID, target.Format("2006-01-02")).Scan(&existingRunID, &existingTrigger, &existingSourceDate, &existingMemberships)
	if err == nil {
		if existingTrigger != string(TriggerRepair) || existingSourceDate != source.Format("2006-01-02") {
			return RepairSummary{}, ErrRepairTargetPublished
		}
		if err := tx.Commit(ctx); err != nil {
			return RepairSummary{}, fmt.Errorf("commit idempotent historical observation repair: %w", err)
		}
		return RepairSummary{
			SourceDate: source.Format("2006-01-02"), TargetDate: target.Format("2006-01-02"),
			MembershipsCopied: existingMemberships, AlreadyRepaired: true,
		}, nil
	}
	if err != pgx.ErrNoRows {
		return RepairSummary{}, fmt.Errorf("check historical repair target: %w", err)
	}

	type sourceRun struct {
		id, archiveURI, archiveSHA, manifestSHA          string
		startedAt, finishedAt, publishedAt               time.Time
		listReportedTotal, listItems, uniqueSourceJobIDs *int
		detailSucceeded, detailFailed                    *int
	}
	var sourceRunRow sourceRun
	err = tx.QueryRow(ctx, `
SELECT run.id::text, run.started_at, run.finished_at, run.published_at,
       run.list_reported_total, run.list_items, run.unique_source_job_ids,
       run.detail_succeeded, run.detail_failed,
       COALESCE(run.archive_uri, ''), COALESCE(run.archive_sha256, ''),
       COALESCE(run.archive_manifest_sha256, '')
FROM collection_runs AS run
WHERE run.observation_scope_id = $1
  AND run.status = 'published'
  AND run.published_at IS NOT NULL
  AND (run.published_at AT TIME ZONE 'Asia/Shanghai')::date = $2::date
ORDER BY run.published_at DESC, run.id DESC
LIMIT 1`, scope.ID, source.Format("2006-01-02")).Scan(
		&sourceRunRow.id, &sourceRunRow.startedAt, &sourceRunRow.finishedAt, &sourceRunRow.publishedAt,
		&sourceRunRow.listReportedTotal, &sourceRunRow.listItems, &sourceRunRow.uniqueSourceJobIDs,
		&sourceRunRow.detailSucceeded, &sourceRunRow.detailFailed,
		&sourceRunRow.archiveURI, &sourceRunRow.archiveSHA, &sourceRunRow.manifestSHA,
	)
	if err == pgx.ErrNoRows {
		return RepairSummary{}, ErrRepairSourceUnavailable
	}
	if err != nil {
		return RepairSummary{}, fmt.Errorf("select historical repair source: %w", err)
	}
	deltaDays := int(target.Sub(source).Hours() / 24)
	targetStartedAt := sourceRunRow.startedAt.AddDate(0, 0, deltaDays)
	targetFinishedAt := sourceRunRow.finishedAt.AddDate(0, 0, deltaDays)
	targetPublishedAt := sourceRunRow.publishedAt.AddDate(0, 0, deltaDays)
	targetRunID := newUUID()
	scheduleKey := fmt.Sprintf("%s:%s:%s:1", scope.ID, target.Format("2006-01-02"), TriggerRepair)
	if _, err := tx.Exec(ctx, `
INSERT INTO collection_runs (
    id, schedule_key, source_company, filter_config, filter_sha256,
    observation_scope_id, data_date, trigger_kind, attempt_number, status,
    started_at, finished_at, published_at,
    list_reported_total, list_items, unique_source_job_ids,
    detail_succeeded, detail_failed, archive_uri, archive_sha256,
    archive_manifest_sha256
) SELECT $1, $2, source_company, filter_config, filter_sha256,
         observation_scope_id, $3::date, $4::collection_trigger_kind, 1, 'published',
         $5, $6, $7,
         list_reported_total, list_items, unique_source_job_ids,
         detail_succeeded, detail_failed, NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, '')
FROM collection_runs
WHERE id = $11`, targetRunID, scheduleKey, target.Format("2006-01-02"), string(TriggerRepair),
		targetStartedAt, targetFinishedAt, targetPublishedAt,
		sourceRunRow.archiveURI, sourceRunRow.archiveSHA, sourceRunRow.manifestSHA, sourceRunRow.id); err != nil {
		return RepairSummary{}, fmt.Errorf("create historical repair publication: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO run_jobs (run_id, job_id, jd_version_id, observed_at)
SELECT $1, job_id, jd_version_id, observed_at
FROM run_jobs WHERE run_id = $2
ON CONFLICT (run_id, job_id) DO NOTHING`, targetRunID, sourceRunRow.id); err != nil {
		return RepairSummary{}, fmt.Errorf("copy historical repair run observations: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO run_job_version_references (run_id, job_id, source_metadata_version_id, body_content_version_id, observed_at)
SELECT $1, job_id, source_metadata_version_id, body_content_version_id, observed_at
FROM run_job_version_references WHERE run_id = $2
ON CONFLICT (run_id, job_id) DO NOTHING`, targetRunID, sourceRunRow.id); err != nil {
		return RepairSummary{}, fmt.Errorf("copy historical repair version references: %w", err)
	}
	var membershipsCopied int
	if err := tx.QueryRow(ctx, `
WITH source_memberships AS (
    SELECT * FROM serving_jd_inventory_memberships WHERE published_run_id = $2
), inserted AS (
    INSERT INTO serving_jd_inventory_memberships (
        published_run_id, observation_scope_id, job_id, presence_period_id,
        source_metadata_version_id, body_content_version_id, activity_state,
        title, locations, is_urgent, first_discovered_at
    )
    SELECT $1, observation_scope_id, job_id, presence_period_id,
           source_metadata_version_id, body_content_version_id, activity_state,
           title, locations, is_urgent, first_discovered_at
    FROM source_memberships
    RETURNING id, job_id
), copied_resolutions AS (
    INSERT INTO serving_jd_dimension_resolutions (
        serving_jd_inventory_membership_id, dimension, selection_priority,
        resolution_key, education_requirement_result_id,
        experience_requirement_result_id, technology_analysis_result_id,
        project_tag_result_id, job_family_result_id
    )
    SELECT inserted.id, resolution.dimension, resolution.selection_priority,
           resolution.resolution_key, resolution.education_requirement_result_id,
           resolution.experience_requirement_result_id, resolution.technology_analysis_result_id,
           resolution.project_tag_result_id, resolution.job_family_result_id
    FROM inserted
    JOIN source_memberships ON source_memberships.job_id = inserted.job_id
    JOIN serving_jd_dimension_resolutions AS resolution
      ON resolution.serving_jd_inventory_membership_id = source_memberships.id
    RETURNING serving_jd_inventory_membership_id
)
SELECT count(*) FROM inserted`, targetRunID, sourceRunRow.id).Scan(&membershipsCopied); err != nil {
		return RepairSummary{}, fmt.Errorf("copy historical repair Serving membership: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO collection_run_audit_events (run_id, stage, severity, code, attempt_number, message, diagnostics)
VALUES ($1, 'persisting', 'info', 'historical_observation_repaired', 1,
        'historical observation was copied from a successful publication',
        jsonb_build_object('source_date', $2::text, 'target_date', $3::text, 'source_run_id', $4::text, 'memberships', $5::int))`,
		targetRunID, source.Format("2006-01-02"), target.Format("2006-01-02"), sourceRunRow.id, membershipsCopied); err != nil {
		return RepairSummary{}, fmt.Errorf("audit historical repair publication: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return RepairSummary{}, fmt.Errorf("commit historical repair publication: %w", err)
	}
	return RepairSummary{SourceDate: source.Format("2006-01-02"), TargetDate: target.Format("2006-01-02"), MembershipsCopied: membershipsCopied}, nil
}

func calendarDate(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

func isMidnight(value time.Time, location *time.Location) bool {
	local := value.In(location)
	return local.Hour() == 0 && local.Minute() == 0 && local.Second() == 0 && local.Nanosecond() == 0
}
