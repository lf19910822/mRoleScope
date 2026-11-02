// Package jdserving owns the DWD-to-Serving read projection. It never calls a
// source system or an AI provider; it reconciles only durable PostgreSQL facts
// after source publication has already committed.
package jdserving

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is deliberately separate from source synchronization. A serving
// projection failure must not share the source publication transaction.
type Repository struct {
	closePool bool
	pool      *pgxpool.Pool
}

func NewRepository(ctx context.Context, databaseURL string) (*Repository, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create serving database pool failed")
	}
	return &Repository{closePool: true, pool: pool}, nil
}

// NewRepositoryFromPool creates a read repository over a caller-owned pool.
// Close is intentionally a no-op for this form, so the API process retains
// ownership of its shared readiness pool.
func NewRepositoryFromPool(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (repository *Repository) Close() {
	if repository != nil && repository.closePool && repository.pool != nil {
		repository.pool.Close()
	}
}

// ReconciliationSummary contains aggregate counts only. It intentionally
// excludes JD text, source identifiers, database values, and credentials so a
// worker may log it safely.
type ReconciliationSummary struct {
	ReconciledRuns     int
	MembershipsCreated int
	MembershipsReused  int
	RetryableFailures  int
}

type publishedRun struct {
	ID                 string
	ObservationScopeID string
	PublishedAt        time.Time
}

type membershipInput struct {
	JobID                   string
	PresencePeriodID        string
	SourceMetadataVersionID *string
	BodyContentVersionID    *string
	ActivityState           string
	Title                   string
	Locations               []string
	Urgent                  bool
	FirstDiscoveredAt       time.Time
}

type formalResolutionCandidate struct {
	Dimension string
	ResultID  string
	Priority  int16
}

// ReconcilePublishedInventory builds every missing source snapshot from the
// durable published-run history. Individual rows with no canonical version
// reference are audited and retried on the next invocation; they cannot
// invalidate source publication or prevent unrelated rows from being served.
func (repository *Repository) ReconcilePublishedInventory(ctx context.Context) (ReconciliationSummary, error) {
	if repository == nil || repository.pool == nil {
		return ReconciliationSummary{}, fmt.Errorf("serving repository is unavailable")
	}
	runs, err := repository.publishedRuns(ctx)
	if err != nil {
		return ReconciliationSummary{}, err
	}

	summary := ReconciliationSummary{}
	for _, run := range runs {
		runSummary, err := repository.reconcilePublishedRun(ctx, run)
		if err != nil {
			return summary, err
		}
		summary.ReconciledRuns++
		summary.MembershipsCreated += runSummary.MembershipsCreated
		summary.MembershipsReused += runSummary.MembershipsReused
		summary.RetryableFailures += runSummary.RetryableFailures
	}
	return summary, nil
}

func (repository *Repository) publishedRuns(ctx context.Context) ([]publishedRun, error) {
	rows, err := repository.pool.Query(ctx, `
SELECT run.id::text, run.observation_scope_id::text, run.published_at
FROM collection_runs AS run
WHERE run.status = 'published'
  AND run.published_at IS NOT NULL
ORDER BY run.published_at, run.id`)
	if err != nil {
		return nil, fmt.Errorf("select published source runs for serving reconciliation: %w", err)
	}
	defer rows.Close()
	runs := make([]publishedRun, 0)
	for rows.Next() {
		var run publishedRun
		if err := rows.Scan(&run.ID, &run.ObservationScopeID, &run.PublishedAt); err != nil {
			return nil, fmt.Errorf("scan published source run for serving reconciliation: %w", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate published source runs for serving reconciliation: %w", err)
	}
	return runs, nil
}

func (repository *Repository) reconcilePublishedRun(ctx context.Context, run publishedRun) (ReconciliationSummary, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ReconciliationSummary{}, fmt.Errorf("begin serving reconciliation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "serving-inventory:"+run.ID); err != nil {
		return ReconciliationSummary{}, fmt.Errorf("lock serving reconciliation run: %w", err)
	}

	inputs, err := loadMembershipInputs(ctx, tx, run)
	if err != nil {
		return ReconciliationSummary{}, err
	}
	summary := ReconciliationSummary{}
	for _, input := range inputs {
		if input.SourceMetadataVersionID == nil || input.BodyContentVersionID == nil {
			if err := appendMissingReferenceAudit(ctx, tx, run, input.JobID); err != nil {
				return ReconciliationSummary{}, err
			}
			summary.RetryableFailures++
			continue
		}
		if err := ensurePublicIdentifier(ctx, tx, input.JobID); err != nil {
			return ReconciliationSummary{}, err
		}
		membershipID, created, err := insertMembership(ctx, tx, run, input)
		if err != nil {
			return ReconciliationSummary{}, err
		}
		if err := reconcileFormalDimensions(ctx, tx, membershipID, input); err != nil {
			return ReconciliationSummary{}, err
		}
		if created {
			summary.MembershipsCreated++
		} else {
			summary.MembershipsReused++
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return ReconciliationSummary{}, fmt.Errorf("commit serving reconciliation: %w", err)
	}
	return summary, nil
}

func ensurePublicIdentifier(ctx context.Context, tx pgx.Tx, jobID string) error {
	if _, err := tx.Exec(ctx, `
INSERT INTO serving_jd_public_identifiers (job_id)
VALUES ($1)
ON CONFLICT (job_id) DO NOTHING`, jobID); err != nil {
		return fmt.Errorf("ensure opaque serving JD identifier: %w", err)
	}
	return nil
}

func loadMembershipInputs(ctx context.Context, tx pgx.Tx, run publishedRun) ([]membershipInput, error) {
	rows, err := tx.Query(ctx, `
WITH target AS (
    SELECT $1::uuid AS run_id, $2::uuid AS observation_scope_id, $3::timestamptz AS published_at
), latest_presence AS (
    SELECT DISTINCT ON (presence.job_id)
           presence.id,
           presence.job_id,
           presence.closed_at,
           presence.opened_at
    FROM job_presence_periods AS presence
    JOIN target ON target.observation_scope_id = presence.observation_scope_id
    WHERE presence.opened_at <= target.published_at
    ORDER BY presence.job_id, presence.opened_at DESC, presence.id DESC
)
SELECT presence.job_id::text,
       presence.id::text,
       reference.source_metadata_version_id::text,
       reference.body_content_version_id::text,
       CASE WHEN presence.closed_at IS NULL OR presence.closed_at > target.published_at
            THEN 'effective' ELSE 'confirmed_closed' END,
       CASE WHEN metadata.metadata_payload #>> '{title,state}' = 'string'
            THEN metadata.metadata_payload #>> '{title,value}'
            ELSE '' END,
       COALESCE(
           ARRAY(
               SELECT location.item #>> '{name,value}'
               FROM jsonb_array_elements(
                   CASE WHEN metadata.metadata_payload #>> '{locations,state}' = 'array'
                        THEN metadata.metadata_payload #> '{locations,items}'
                        ELSE '[]'::jsonb END
               ) AS location(item)
               WHERE location.item #>> '{name,state}' = 'string'
           ),
           ARRAY[]::text[]
       ),
       CASE WHEN metadata.metadata_payload #>> '{hurry,state}' = 'boolean'
            THEN COALESCE((metadata.metadata_payload #>> '{hurry,value}')::boolean, false)
            ELSE false END,
       first_presence.first_opened_at
FROM latest_presence AS presence
JOIN target ON true
JOIN jobs AS job ON job.id = presence.job_id
JOIN LATERAL (
    SELECT min(period.opened_at) AS first_opened_at
    FROM job_presence_periods AS period
    WHERE period.observation_scope_id = target.observation_scope_id
      AND period.job_id = presence.job_id
      AND period.opened_at <= target.published_at
) AS first_presence ON true
LEFT JOIN LATERAL (
    SELECT version_reference.source_metadata_version_id,
           version_reference.body_content_version_id
    FROM run_job_version_references AS version_reference
    JOIN collection_runs AS observed_run ON observed_run.id = version_reference.run_id
    WHERE version_reference.job_id = presence.job_id
      AND observed_run.observation_scope_id = target.observation_scope_id
      AND version_reference.observed_at <= target.published_at
    ORDER BY version_reference.observed_at DESC, version_reference.run_id DESC
    LIMIT 1
) AS reference ON true
LEFT JOIN job_source_metadata_versions AS metadata ON metadata.id = reference.source_metadata_version_id
ORDER BY presence.job_id`, run.ID, run.ObservationScopeID, run.PublishedAt)
	if err != nil {
		return nil, fmt.Errorf("select serving membership source facts: %w", err)
	}
	defer rows.Close()
	inputs := make([]membershipInput, 0)
	for rows.Next() {
		var input membershipInput
		if err := rows.Scan(
			&input.JobID,
			&input.PresencePeriodID,
			&input.SourceMetadataVersionID,
			&input.BodyContentVersionID,
			&input.ActivityState,
			&input.Title,
			&input.Locations,
			&input.Urgent,
			&input.FirstDiscoveredAt,
		); err != nil {
			return nil, fmt.Errorf("scan serving membership source facts: %w", err)
		}
		inputs = append(inputs, input)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate serving membership source facts: %w", err)
	}
	return inputs, nil
}

func insertMembership(ctx context.Context, tx pgx.Tx, run publishedRun, input membershipInput) (string, bool, error) {
	var membershipID string
	var created bool
	err := tx.QueryRow(ctx, `
WITH inserted AS (
    INSERT INTO serving_jd_inventory_memberships (
    published_run_id, observation_scope_id, job_id, presence_period_id,
    source_metadata_version_id, body_content_version_id, activity_state,
    title, locations, is_urgent, first_discovered_at
) VALUES ($1, $2, $3, $4, $5, $6, $7::serving_jd_activity_state, $8, $9, $10, $11)
	ON CONFLICT (published_run_id, job_id) DO NOTHING
    RETURNING id
)
SELECT id::text, true FROM inserted
UNION ALL
SELECT existing.id::text, false
FROM serving_jd_inventory_memberships AS existing
WHERE existing.published_run_id = $1
  AND existing.job_id = $3
  AND NOT EXISTS (SELECT 1 FROM inserted)`,
		run.ID, run.ObservationScopeID, input.JobID, input.PresencePeriodID,
		*input.SourceMetadataVersionID, *input.BodyContentVersionID, input.ActivityState,
		input.Title, input.Locations, input.Urgent, input.FirstDiscoveredAt,
	).Scan(&membershipID, &created)
	if err != nil {
		return "", false, fmt.Errorf("insert serving membership: %w", err)
	}
	return membershipID, created, nil
}

// reconcileFormalDimensions records the best currently known deterministic
// result for each dimension without modifying the immutable source snapshot.
// A current-version result outranks an earlier result for the same logical
// job. Each new choice is appended, allowing the serving view to select the
// newest reconciliation while preserving historical provenance for charts.
func reconcileFormalDimensions(ctx context.Context, tx pgx.Tx, membershipID string, input membershipInput) error {
	candidates, err := loadFormalResolutionCandidates(ctx, tx, input)
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
		if err := appendFormalResolution(ctx, tx, membershipID, candidate); err != nil {
			return err
		}
	}
	return nil
}

func loadFormalResolutionCandidates(ctx context.Context, tx pgx.Tx, input membershipInput) ([]formalResolutionCandidate, error) {
	queries := []struct {
		dimension string
		query     string
		versionID string
	}{
		{
			dimension: "education",
			query: `
SELECT result.id::text,
       (CASE WHEN result.body_content_version_id = $2::uuid THEN 0 ELSE 1 END)::smallint
FROM education_requirement_results AS result
JOIN job_body_content_versions AS body ON body.id = result.body_content_version_id
WHERE body.job_id = $1::uuid
  AND result.completed_at IS NOT NULL
ORDER BY CASE WHEN result.body_content_version_id = $2::uuid THEN 0 ELSE 1 END,
         result.completed_at DESC, result.id DESC
LIMIT 1`,
			versionID: *input.BodyContentVersionID,
		},
		{
			dimension: "experience",
			query: `
SELECT result.id::text,
       (CASE WHEN result.body_content_version_id = $2::uuid THEN 0 ELSE 1 END)::smallint
FROM experience_requirement_results AS result
JOIN job_body_content_versions AS body ON body.id = result.body_content_version_id
WHERE body.job_id = $1::uuid
  AND result.completed_at IS NOT NULL
ORDER BY CASE WHEN result.body_content_version_id = $2::uuid THEN 0 ELSE 1 END,
         result.completed_at DESC, result.id DESC
LIMIT 1`,
			versionID: *input.BodyContentVersionID,
		},
		{
			dimension: "technology",
			query: `
SELECT result.id::text,
       (CASE WHEN result.body_content_version_id = $2::uuid THEN 0 ELSE 1 END)::smallint
FROM technology_analysis_results AS result
JOIN job_body_content_versions AS body ON body.id = result.body_content_version_id
WHERE body.job_id = $1::uuid
  AND result.status = 'complete'
  AND result.completed_at IS NOT NULL
ORDER BY CASE WHEN result.body_content_version_id = $2::uuid THEN 0 ELSE 1 END,
         result.completed_at DESC, result.id DESC
LIMIT 1`,
			versionID: *input.BodyContentVersionID,
		},
		{
			dimension: "project",
			query: `
SELECT result.id::text,
       (CASE WHEN observation.source_metadata_version_id = $2::uuid THEN 0 ELSE 1 END)::smallint
FROM project_tag_results AS result
JOIN project_title_observations AS observation
  ON observation.project_title_identity_id = result.project_title_identity_id
JOIN job_source_metadata_versions AS metadata
  ON metadata.id = observation.source_metadata_version_id
WHERE metadata.job_id = $1::uuid
  AND result.status = 'complete'
  AND result.completed_at IS NOT NULL
ORDER BY CASE WHEN observation.source_metadata_version_id = $2::uuid THEN 0 ELSE 1 END,
         result.completed_at DESC, result.id DESC
LIMIT 1`,
			versionID: *input.SourceMetadataVersionID,
		},
		{
			dimension: "job_family",
			query: `
SELECT result.id::text,
       (CASE WHEN observation.source_metadata_version_id = $2::uuid THEN 0 ELSE 1 END)::smallint
FROM job_family_results AS result
JOIN job_family_observations AS observation
  ON observation.job_family_title_identity_id = result.job_family_title_identity_id
JOIN job_source_metadata_versions AS metadata
  ON metadata.id = observation.source_metadata_version_id
WHERE metadata.job_id = $1::uuid
  AND result.completed_at IS NOT NULL
ORDER BY CASE WHEN observation.source_metadata_version_id = $2::uuid THEN 0 ELSE 1 END,
         result.completed_at DESC, result.id DESC
LIMIT 1`,
			versionID: *input.SourceMetadataVersionID,
		},
	}

	candidates := make([]formalResolutionCandidate, 0, len(queries))
	for _, selection := range queries {
		candidate := formalResolutionCandidate{Dimension: selection.dimension}
		err := tx.QueryRow(ctx, selection.query, input.JobID, selection.versionID).Scan(&candidate.ResultID, &candidate.Priority)
		if err == pgx.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("select %s serving resolution candidate: %w", selection.dimension, err)
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func appendFormalResolution(ctx context.Context, tx pgx.Tx, membershipID string, candidate formalResolutionCandidate) error {
	resolutionKey := fmt.Sprintf("%d:%s", candidate.Priority, candidate.ResultID)
	queries := map[string]string{
		"education": `INSERT INTO serving_jd_dimension_resolutions (
    serving_jd_inventory_membership_id, dimension, selection_priority, resolution_key,
    education_requirement_result_id
) VALUES ($1, 'education', $2, $3, $4)
ON CONFLICT (serving_jd_inventory_membership_id, dimension, resolution_key) DO NOTHING`,
		"experience": `INSERT INTO serving_jd_dimension_resolutions (
    serving_jd_inventory_membership_id, dimension, selection_priority, resolution_key,
    experience_requirement_result_id
) VALUES ($1, 'experience', $2, $3, $4)
ON CONFLICT (serving_jd_inventory_membership_id, dimension, resolution_key) DO NOTHING`,
		"technology": `INSERT INTO serving_jd_dimension_resolutions (
    serving_jd_inventory_membership_id, dimension, selection_priority, resolution_key,
    technology_analysis_result_id
) VALUES ($1, 'technology', $2, $3, $4)
ON CONFLICT (serving_jd_inventory_membership_id, dimension, resolution_key) DO NOTHING`,
		"project": `INSERT INTO serving_jd_dimension_resolutions (
    serving_jd_inventory_membership_id, dimension, selection_priority, resolution_key,
    project_tag_result_id
) VALUES ($1, 'project', $2, $3, $4)
ON CONFLICT (serving_jd_inventory_membership_id, dimension, resolution_key) DO NOTHING`,
		"job_family": `INSERT INTO serving_jd_dimension_resolutions (
    serving_jd_inventory_membership_id, dimension, selection_priority, resolution_key,
    job_family_result_id
) VALUES ($1, 'job_family', $2, $3, $4)
ON CONFLICT (serving_jd_inventory_membership_id, dimension, resolution_key) DO NOTHING`,
	}
	query, found := queries[candidate.Dimension]
	if !found {
		return fmt.Errorf("unsupported serving resolution dimension")
	}
	if _, err := tx.Exec(ctx, query, membershipID, candidate.Priority, resolutionKey, candidate.ResultID); err != nil {
		return fmt.Errorf("append %s serving resolution: %w", candidate.Dimension, err)
	}
	return nil
}

func appendMissingReferenceAudit(ctx context.Context, tx pgx.Tx, run publishedRun, jobID string) error {
	_, err := tx.Exec(ctx, `
INSERT INTO serving_jd_reconciliation_audits (
    published_run_id, observation_scope_id, job_id, stage, severity, code
)
SELECT $1, $2, $3,
       CASE WHEN EXISTS (
           SELECT 1
           FROM serving_jd_reconciliation_audits
           WHERE published_run_id = $1
             AND observation_scope_id = $2
             AND job_id = $3
             AND code = 'source_version_reference_missing'
       ) THEN 'retry' ELSE 'reconcile' END,
       'warning', 'source_version_reference_missing'`, run.ID, run.ObservationScopeID, jobID)
	if err != nil {
		return fmt.Errorf("append serving source-reference retry audit: %w", err)
	}
	return nil
}
