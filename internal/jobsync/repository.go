package jobsync

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mrolescope/internal/collector"
)

// Repository owns the PostgreSQL state machine.  Every mutation that affects
// presence/content history happens in Publish's single transaction.
type Repository struct {
	pool *pgxpool.Pool
}

// BackfillSummary deliberately contains counts only. It is safe to expose in
// controlled CVM operations because it has no source text, snippets, source
// identifiers, archive locations, credentials, or database error details.
type BackfillSummary struct {
	PublishedObservations    int
	VersionReferencesCreated int
	BodyPeriodsCreated       int
	SkippedObservations      int
	SkippedBodyPeriods       int
}

func NewRepository(ctx context.Context, databaseURL string) (*Repository, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	return &Repository{pool: pool}, nil
}

func (repository *Repository) Close() {
	if repository != nil && repository.pool != nil {
		repository.pool.Close()
	}
}

// BackfillVersionedContent maps the already-published legacy baseline into
// the additive metadata/body model. It never modifies legacy source facts or
// COS references. Where an old database row lacks sufficient normalized input
// to reconstruct a canonical version, it records only a safe skipped count;
// the private source archive remains the authoritative historical evidence.
func (repository *Repository) BackfillVersionedContent(ctx context.Context) (BackfillSummary, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return BackfillSummary{}, fmt.Errorf("begin versioned-content backfill: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	summary, err := backfillVersionedContent(ctx, tx)
	if err != nil {
		return BackfillSummary{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return BackfillSummary{}, fmt.Errorf("commit versioned-content backfill: %w", err)
	}
	return summary, nil
}

func backfillVersionedContent(ctx context.Context, tx pgx.Tx) (BackfillSummary, error) {
	rows, err := tx.Query(ctx, `
SELECT rj.run_id::text, rj.job_id::text, j.source_company, j.source_job_id,
       rj.jd_version_id::text, jd.raw_fields, jd.display_text, rj.observed_at
FROM run_jobs AS rj
JOIN collection_runs AS run ON run.id = rj.run_id
JOIN jobs AS j ON j.id = rj.job_id
JOIN jd_versions AS jd ON jd.id = rj.jd_version_id
WHERE run.status = 'published'
ORDER BY rj.run_id, rj.job_id`)
	if err != nil {
		return BackfillSummary{}, fmt.Errorf("query published legacy observations: %w", err)
	}
	defer rows.Close()

	type legacyObservationRow struct {
		runID, jobID, sourceCompany, sourceJobID, legacyVersionID string
		rawFields, displayFields                                  []byte
		observedAt                                                time.Time
	}
	legacyRows := make([]legacyObservationRow, 0)
	for rows.Next() {
		var row legacyObservationRow
		if err := rows.Scan(&row.runID, &row.jobID, &row.sourceCompany, &row.sourceJobID, &row.legacyVersionID, &row.rawFields, &row.displayFields, &row.observedAt); err != nil {
			return BackfillSummary{}, fmt.Errorf("scan published legacy observation: %w", err)
		}
		legacyRows = append(legacyRows, row)
	}
	if err := rows.Err(); err != nil {
		return BackfillSummary{}, fmt.Errorf("iterate published legacy observations: %w", err)
	}
	rows.Close()

	summary := BackfillSummary{}
	legacyToBody := make(map[string]string)
	for _, row := range legacyRows {
		summary.PublishedObservations++

		observation, ok := observationFromLegacyVersion(row.sourceCompany, row.sourceJobID, row.rawFields, row.displayFields, row.observedAt)
		if !ok {
			summary.SkippedObservations++
			continue
		}
		metadataID, err := upsertSourceMetadataVersion(ctx, tx, row.jobID, observation)
		if err != nil {
			return BackfillSummary{}, err
		}
		bodyID, err := upsertBodyContentVersion(ctx, tx, row.jobID, observation)
		if err != nil {
			return BackfillSummary{}, err
		}
		legacyToBody[row.legacyVersionID] = bodyID
		command, err := tx.Exec(ctx, `
INSERT INTO run_job_version_references (
    run_id, job_id, source_metadata_version_id, body_content_version_id, observed_at
) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (run_id, job_id) DO NOTHING`, row.runID, row.jobID, metadataID, bodyID, observation.ObservedAt)
		if err != nil {
			return BackfillSummary{}, fmt.Errorf("insert canonical run reference: %w", err)
		}
		summary.VersionReferencesCreated += int(command.RowsAffected())
	}

	periodRows, err := tx.Query(ctx, `
SELECT id::text, presence_period_id::text, jd_version_id::text,
       valid_from_run_id::text, valid_from, valid_to_run_id::text, valid_to
FROM job_content_periods
ORDER BY id`)
	if err != nil {
		return BackfillSummary{}, fmt.Errorf("query legacy content periods: %w", err)
	}
	defer periodRows.Close()
	type legacyPeriodRow struct {
		presencePeriodID, legacyVersionID, validFromRunID string
		validFrom                                         time.Time
		validToRunID                                      *string
		validTo                                           *time.Time
	}
	legacyPeriods := make([]legacyPeriodRow, 0)
	for periodRows.Next() {
		var ignoredID string
		var period legacyPeriodRow
		if err := periodRows.Scan(&ignoredID, &period.presencePeriodID, &period.legacyVersionID, &period.validFromRunID, &period.validFrom, &period.validToRunID, &period.validTo); err != nil {
			return BackfillSummary{}, fmt.Errorf("scan legacy content period: %w", err)
		}
		legacyPeriods = append(legacyPeriods, period)
	}
	if err := periodRows.Err(); err != nil {
		return BackfillSummary{}, fmt.Errorf("iterate legacy content periods: %w", err)
	}
	periodRows.Close()
	for _, period := range legacyPeriods {
		bodyID, exists := legacyToBody[period.legacyVersionID]
		if !exists {
			summary.SkippedBodyPeriods++
			continue
		}
		command, err := tx.Exec(ctx, `
INSERT INTO job_body_content_periods (
    presence_period_id, body_content_version_id, valid_from_run_id, valid_from, valid_to_run_id, valid_to
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (presence_period_id, valid_from_run_id, body_content_version_id) DO NOTHING`,
			period.presencePeriodID, bodyID, period.validFromRunID, period.validFrom, period.validToRunID, period.validTo)
		if err != nil {
			return BackfillSummary{}, fmt.Errorf("insert canonical body content period: %w", err)
		}
		summary.BodyPeriodsCreated += int(command.RowsAffected())
	}
	return summary, nil
}

func observationFromLegacyVersion(sourceCompany, sourceJobID string, rawFields, displayFields []byte, observedAt time.Time) (Observation, bool) {
	var detail, display map[string]any
	if json.Unmarshal(rawFields, &detail) != nil || json.Unmarshal(displayFields, &display) != nil {
		return Observation{}, false
	}
	listItem := legacyListFallback(sourceJobID, display)
	observation, err := NormalizeObservation(sourceCompany, listItem, detail, observedAt)
	if err != nil {
		return Observation{}, false
	}
	return observation, true
}

// legacyListFallback uses only the old database representation when an M0/M1
// detail payload did not contain a field. It does not claim unavailable source
// IDs or types existed; those distinctions remain recoverable from COS.
func legacyListFallback(sourceJobID string, display map[string]any) map[string]any {
	listItem := map[string]any{"id": sourceJobID}
	for target, source := range map[string]string{
		"title":       "title",
		"description": "description",
		"jobRequire":  "requirements",
		"addition":    "additional",
		"category":    "category",
		"jobNature":   "nature",
		"projectName": "project",
	} {
		if value, exists := display[source]; exists {
			listItem[target] = value
		}
	}
	if locations, ok := display["locations"].([]any); ok {
		addressDetails := make([]any, 0, len(locations))
		for _, location := range locations {
			addressDetails = append(addressDetails, map[string]any{"addressDetail": location})
		}
		listItem["addressDetailList"] = addressDetails
	}
	return listItem
}

func upsertSourceMetadataVersion(ctx context.Context, tx pgx.Tx, jobID string, observation Observation) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `
INSERT INTO job_source_metadata_versions (
    job_id, metadata_sha256, normalization_version, metadata_payload
) VALUES ($1, $2, $3, $4)
ON CONFLICT (job_id, metadata_sha256)
DO UPDATE SET metadata_sha256 = EXCLUDED.metadata_sha256
RETURNING id::text`, jobID, observation.MetadataSHA256, observation.MetadataNormalizationVersion, []byte(observation.MetadataPayload)).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("upsert source metadata version: %w", err)
	}
	return id, nil
}

func upsertBodyContentVersion(ctx context.Context, tx pgx.Tx, jobID string, observation Observation) (string, error) {
	id, _, err := upsertBodyContentVersionWithCreated(ctx, tx, jobID, observation)
	return id, err
}

// upsertBodyContentVersionWithCreated distinguishes a new analysis input from
// a reused historic body. The flag is used only to enqueue downstream work;
// it is never inferred from a metadata or legacy merged-version change.
func upsertBodyContentVersionWithCreated(ctx context.Context, tx pgx.Tx, jobID string, observation Observation) (string, bool, error) {
	var id string
	err := tx.QueryRow(ctx, `
	SELECT id::text
FROM job_body_content_versions
WHERE job_id = $1 AND body_sha256 = $2`, jobID, observation.BodySHA256).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if err != pgx.ErrNoRows {
		return "", false, fmt.Errorf("find body content version: %w", err)
	}
	err = tx.QueryRow(ctx, `
INSERT INTO job_body_content_versions (
    job_id, body_sha256, normalization_version,
    description_text, job_require_text, addition_text,
    description_sha256, job_require_sha256, addition_sha256
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (job_id, body_sha256) DO NOTHING
RETURNING id::text`,
		jobID, observation.BodySHA256, observation.BodyNormalizationVersion,
		observation.DescriptionText, observation.JobRequireText, observation.AdditionText,
		observation.DescriptionSHA256, observation.JobRequireSHA256, observation.AdditionSHA256).Scan(&id)
	if err == nil {
		return id, true, nil
	}
	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx, `
SELECT id::text
FROM job_body_content_versions
WHERE job_id = $1 AND body_sha256 = $2`, jobID, observation.BodySHA256).Scan(&id)
	}
	if err != nil {
		return "", false, fmt.Errorf("upsert body content version: %w", err)
	}
	return id, false, nil
}

type StoredScope struct {
	ID string
	Scope
}

func (repository *Repository) EnsureScope(ctx context.Context, scope Scope) (StoredScope, error) {
	filter, err := json.Marshal(scope.Filter)
	if err != nil {
		return StoredScope{}, fmt.Errorf("encode scope filter: %w", err)
	}
	var id string
	err = repository.pool.QueryRow(ctx, `
INSERT INTO observation_scopes (source_company, filter_config, filter_sha256)
VALUES ($1, $2, $3)
ON CONFLICT (source_company, filter_sha256)
DO UPDATE SET filter_config = EXCLUDED.filter_config
RETURNING id::text`, scope.SourceCompany, filter, scope.FilterSHA256).Scan(&id)
	if err != nil {
		return StoredScope{}, fmt.Errorf("ensure observation scope: %w", err)
	}
	return StoredScope{ID: id, Scope: scope}, nil
}

// TryAcquireScopeLock holds a session-level advisory lock on a dedicated pool
// connection. The caller must invoke release after all state-changing work.
func (repository *Repository) TryAcquireScopeLock(ctx context.Context, scopeID string) (release func(), acquired bool, err error) {
	connection, err := repository.pool.Acquire(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("acquire lock connection: %w", err)
	}
	var locked bool
	err = connection.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1, 0))`, scopeID).Scan(&locked)
	if err != nil {
		connection.Release()
		return nil, false, fmt.Errorf("acquire scope advisory lock: %w", err)
	}
	if !locked {
		connection.Release()
		return func() {}, false, nil
	}
	return func() {
		_, _ = connection.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, scopeID)
		connection.Release()
	}, true, nil
}

type Run struct {
	ID            string
	ScopeID       string
	AttemptNumber int
}

func (repository *Repository) CreateRun(ctx context.Context, scope StoredScope, runID string, trigger TriggerKind, attemptNumber int, startedAt time.Time) (Run, error) {
	if !trigger.Valid() {
		return Run{}, fmt.Errorf("invalid synchronization trigger %q", trigger)
	}
	if attemptNumber <= 0 {
		return Run{}, fmt.Errorf("attempt number must be positive")
	}
	dataDate := asiaShanghaiDate(startedAt)
	filter, err := json.Marshal(scope.Filter)
	if err != nil {
		return Run{}, fmt.Errorf("encode scope filter: %w", err)
	}
	scheduleKey := fmt.Sprintf("%s:%s:%s:%d", scope.ID, dataDate.Format("2006-01-02"), trigger, attemptNumber)
	_, err = repository.pool.Exec(ctx, `
INSERT INTO collection_runs (
    id, schedule_key, source_company, filter_config, filter_sha256,
    observation_scope_id, data_date, trigger_kind, attempt_number, status, started_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'collecting', $10)`,
		runID, scheduleKey, scope.SourceCompany, filter, scope.FilterSHA256,
		scope.ID, dataDate, trigger, attemptNumber, startedAt.UTC())
	if err != nil {
		return Run{}, fmt.Errorf("create collection run: %w", err)
	}
	return Run{ID: runID, ScopeID: scope.ID, AttemptNumber: attemptNumber}, nil
}

func asiaShanghaiDate(value time.Time) time.Time {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		// This is part of the Go distribution. Falling back keeps a local test
		// environment without timezone data from producing a local-time date.
		location = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

type AuditEvent struct {
	Stage       string
	Severity    string
	Code        string
	Message     string
	Diagnostics map[string]any
}

func (repository *Repository) AppendAudit(ctx context.Context, run Run, event AuditEvent) error {
	diagnostics, err := safeDiagnostics(event.Diagnostics)
	if err != nil {
		return err
	}
	_, err = repository.pool.Exec(ctx, `
INSERT INTO collection_run_audit_events (run_id, stage, severity, code, attempt_number, message, diagnostics)
VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		run.ID, event.Stage, event.Severity, event.Code, run.AttemptNumber, SafeMessage(event.Message), diagnostics)
	if err != nil {
		return fmt.Errorf("append audit event: %w", err)
	}
	return nil
}

func (repository *Repository) SetRunStage(ctx context.Context, runID, status string) error {
	_, err := repository.pool.Exec(ctx, `UPDATE collection_runs SET status = $2, updated_at = now() WHERE id = $1`, runID, status)
	if err != nil {
		return fmt.Errorf("set run stage: %w", err)
	}
	return nil
}

func (repository *Repository) FailRun(ctx context.Context, run Run, stage string, cause error) error {
	// Errors from an HTTP client, a subprocess, or an object-store SDK may
	// contain untrusted response text. Raw source content belongs only in the
	// verified COS archive, never in PostgreSQL or logs. Preserve the useful
	// operational fact (where it failed) as a stable code instead.
	code, summary := safeFailure(stage)
	_, err := repository.pool.Exec(ctx, `
UPDATE collection_runs
SET status = 'failed', failure_stage = $2, failure_summary = $3, finished_at = now(), updated_at = now()
WHERE id = $1`, run.ID, stage, summary)
	if err != nil {
		return fmt.Errorf("mark collection run failed: %w", err)
	}
	return repository.AppendAudit(ctx, run, AuditEvent{
		Stage: stage, Severity: "error", Code: code, Message: summary,
	})
}

func safeFailure(stage string) (code, message string) {
	switch stage {
	case "collecting":
		return "collection_failed", "collection did not complete"
	case "validating":
		return "batch_validation_failed", "collected batch did not pass validation"
	case "missing-recheck":
		return "missing_recheck_failed", "missing-job recheck did not complete"
	case "archiving":
		return "archive_failed", "verified batch could not be archived"
	case "persisting":
		return "publication_failed", "archived batch could not be published"
	default:
		return "sync_failed", "synchronization did not complete"
	}
}

type OpenPresence struct {
	ID          string
	JobID       string
	SourceJobID string
}

func (repository *Repository) OpenPresence(ctx context.Context, scopeID string) ([]OpenPresence, error) {
	rows, err := repository.pool.Query(ctx, `
SELECT p.id::text, p.job_id::text, j.source_job_id
FROM job_presence_periods AS p
JOIN jobs AS j ON j.id = p.job_id
WHERE p.observation_scope_id = $1 AND p.closed_at IS NULL
ORDER BY j.source_job_id`, scopeID)
	if err != nil {
		return nil, fmt.Errorf("query open presence periods: %w", err)
	}
	defer rows.Close()
	periods := make([]OpenPresence, 0)
	for rows.Next() {
		var period OpenPresence
		if err := rows.Scan(&period.ID, &period.JobID, &period.SourceJobID); err != nil {
			return nil, fmt.Errorf("scan open presence period: %w", err)
		}
		periods = append(periods, period)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate open presence periods: %w", err)
	}
	return periods, nil
}

type ArchiveResult struct {
	URI            string
	ManifestSHA256 string
}

type PublishInput struct {
	Run          Run
	Scope        StoredScope
	Manifest     collector.Manifest
	Observations []Observation
	Rechecks     map[string]RecheckResult
	Archive      ArchiveResult
}

// Publish is the only method that may create or close SCD2 intervals. It is
// invoked only after object storage completed its integrity verification.
func (repository *Repository) Publish(ctx context.Context, input PublishInput) error {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin publication transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	// A transport retry can reach this boundary after the first transaction
	// committed but before its caller observed the success. Lock the run row so
	// replaying the same immutable run is a no-op rather than a second audit
	// event or a second state transition.
	var currentStatus string
	if err := tx.QueryRow(ctx, `SELECT status::text FROM collection_runs WHERE id = $1 FOR UPDATE`, input.Run.ID).Scan(&currentStatus); err != nil {
		return fmt.Errorf("lock collection run for publication: %w", err)
	}
	if currentStatus == "published" {
		return nil
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO observation_scope_publication_state (observation_scope_id)
VALUES ($1)
ON CONFLICT (observation_scope_id) DO NOTHING`, input.Scope.ID); err != nil {
		return fmt.Errorf("ensure scope publication state: %w", err)
	}
	if _, err := tx.Exec(ctx, `
SELECT observation_scope_id
FROM observation_scope_publication_state
WHERE observation_scope_id = $1
FOR UPDATE`, input.Scope.ID); err != nil {
		return fmt.Errorf("lock scope publication state: %w", err)
	}
	// A source publication takes a shared lock for its whole transaction. A
	// candidate promotion takes the corresponding exclusive lock, so it either
	// sees this newly committed body or makes this transaction wait and enqueue
	// against the newly published configuration. That closes the catch-up race
	// without coupling source collection to analysis execution.
	if err := lockAnalysisPublicationStateForSourcePublish(ctx, tx); err != nil {
		return err
	}

	observedJobIDs := make(map[string]struct{}, len(input.Observations))
	for _, observation := range input.Observations {
		if _, duplicate := observedJobIDs[observation.SourceJobID]; duplicate {
			return fmt.Errorf("duplicate observation for source job id %s", observation.SourceJobID)
		}
		observedJobIDs[observation.SourceJobID] = struct{}{}
		if err := publishObservation(ctx, tx, input, observation); err != nil {
			return err
		}
	}
	if err := publishMissingPeriods(ctx, tx, input, observedJobIDs); err != nil {
		return err
	}

	counts := input.Manifest.Counts
	if _, err := tx.Exec(ctx, `
UPDATE collection_runs
SET status = 'published', finished_at = $2, published_at = $2,
    list_reported_total = $3, list_items = $4, unique_source_job_ids = $5,
    detail_succeeded = $6, detail_failed = $7, archive_uri = $8,
    archive_sha256 = $9, archive_manifest_sha256 = $9, updated_at = now()
WHERE id = $1`, input.Run.ID, input.Manifest.FinishedAt.UTC(), counts.ListReportedTotal,
		counts.ListItems, counts.UniqueSourceJobID, counts.DetailSucceeded, counts.DetailFailed,
		input.Archive.URI, input.Archive.ManifestSHA256); err != nil {
		return fmt.Errorf("mark collection run published: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE observation_scope_publication_state
SET published_run_id = $2, updated_at = now()
WHERE observation_scope_id = $1`, input.Scope.ID, input.Run.ID); err != nil {
		return fmt.Errorf("advance scope publication state: %w", err)
	}
	if err := appendAuditTx(ctx, tx, input.Run, AuditEvent{
		Stage: "published", Severity: "info", Code: "run_published", Message: "validated batch archived and published",
		Diagnostics: map[string]any{"observations": len(input.Observations)},
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit publication transaction: %w", err)
	}
	return nil
}

func publishObservation(ctx context.Context, tx pgx.Tx, input PublishInput, observation Observation) error {
	var jobID string
	if err := tx.QueryRow(ctx, `
INSERT INTO jobs (id, source_company, source_job_id, first_seen_run_id, last_seen_run_id)
VALUES ($1, $2, $3, $4, $4)
ON CONFLICT (source_company, source_job_id)
DO UPDATE SET last_seen_run_id = EXCLUDED.last_seen_run_id, updated_at = now()
RETURNING id::text`, newUUID(), observation.SourceCompany, observation.SourceJobID, input.Run.ID).Scan(&jobID); err != nil {
		return fmt.Errorf("upsert job %s: %w", observation.SourceJobID, err)
	}
	var versionID string
	if err := tx.QueryRow(ctx, `
INSERT INTO jd_versions (id, job_id, content_sha256, raw_fields, display_text, collected_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (job_id, content_sha256)
DO UPDATE SET content_sha256 = EXCLUDED.content_sha256
RETURNING id::text`, newUUID(), jobID, observation.ContentSHA256, []byte(observation.RawFields), []byte(observation.DisplayFields), observation.ObservedAt).Scan(&versionID); err != nil {
		return fmt.Errorf("upsert JD version for job %s: %w", observation.SourceJobID, err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO run_jobs (run_id, job_id, jd_version_id, observed_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (run_id, job_id) DO NOTHING`, input.Run.ID, jobID, versionID, observation.ObservedAt); err != nil {
		return fmt.Errorf("insert run observation for job %s: %w", observation.SourceJobID, err)
	}
	metadataVersionID, err := upsertSourceMetadataVersion(ctx, tx, jobID, observation)
	if err != nil {
		return fmt.Errorf("upsert source metadata version for job %s: %w", observation.SourceJobID, err)
	}
	bodyVersionID, bodyCreated, err := upsertBodyContentVersionWithCreated(ctx, tx, jobID, observation)
	if err != nil {
		return fmt.Errorf("upsert body content version for job %s: %w", observation.SourceJobID, err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO run_job_version_references (
    run_id, job_id, source_metadata_version_id, body_content_version_id, observed_at
) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (run_id, job_id) DO NOTHING`,
		input.Run.ID, jobID, metadataVersionID, bodyVersionID, observation.ObservedAt); err != nil {
		return fmt.Errorf("insert canonical run reference for job %s: %w", observation.SourceJobID, err)
	}

	var presenceID string
	err = tx.QueryRow(ctx, `
SELECT id::text
FROM job_presence_periods
WHERE observation_scope_id = $1 AND job_id = $2 AND closed_at IS NULL
FOR UPDATE`, input.Scope.ID, jobID).Scan(&presenceID)
	if err == pgx.ErrNoRows {
		if err := tx.QueryRow(ctx, `
INSERT INTO job_presence_periods (
    id, observation_scope_id, job_id, opened_run_id, opened_at, last_seen_run_id, last_seen_at
) VALUES ($1, $2, $3, $4, $5, $4, $5)
RETURNING id::text`, newUUID(), input.Scope.ID, jobID, input.Run.ID, observation.ObservedAt).Scan(&presenceID); err != nil {
			return fmt.Errorf("open presence period for job %s: %w", observation.SourceJobID, err)
		}
	} else if err != nil {
		return fmt.Errorf("lock presence period for job %s: %w", observation.SourceJobID, err)
	} else if _, err := tx.Exec(ctx, `
UPDATE job_presence_periods
SET last_seen_run_id = $2, last_seen_at = $3, first_missing_run_id = NULL, first_missing_at = NULL
WHERE id = $1`, presenceID, input.Run.ID, observation.ObservedAt); err != nil {
		return fmt.Errorf("refresh presence period for job %s: %w", observation.SourceJobID, err)
	}

	var contentPeriodID, currentVersionID string
	var validFrom time.Time
	err = tx.QueryRow(ctx, `
SELECT id::text, jd_version_id::text, valid_from
FROM job_content_periods
WHERE presence_period_id = $1 AND valid_to IS NULL
FOR UPDATE`, presenceID).Scan(&contentPeriodID, &currentVersionID, &validFrom)
	if err == pgx.ErrNoRows {
		if _, err := tx.Exec(ctx, `
INSERT INTO job_content_periods (id, presence_period_id, jd_version_id, valid_from_run_id, valid_from)
VALUES ($1, $2, $3, $4, $5)`, newUUID(), presenceID, versionID, input.Run.ID, observation.ObservedAt); err != nil {
			return fmt.Errorf("open content period for job %s: %w", observation.SourceJobID, err)
		}
	} else if err != nil {
		return fmt.Errorf("lock content period for job %s: %w", observation.SourceJobID, err)
	} else if currentVersionID != versionID {
		boundary := observation.ObservedAt
		if !boundary.After(validFrom) {
			boundary = validFrom.Add(time.Microsecond)
		}
		if _, err := tx.Exec(ctx, `
UPDATE job_content_periods SET valid_to_run_id = $2, valid_to = $3 WHERE id = $1`, contentPeriodID, input.Run.ID, boundary); err != nil {
			return fmt.Errorf("close content period for job %s: %w", observation.SourceJobID, err)
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO job_content_periods (id, presence_period_id, jd_version_id, valid_from_run_id, valid_from)
VALUES ($1, $2, $3, $4, $5)`, newUUID(), presenceID, versionID, input.Run.ID, boundary); err != nil {
			return fmt.Errorf("open replacement content period for job %s: %w", observation.SourceJobID, err)
		}
	}
	if err := ensureBodyContentPeriod(ctx, tx, presenceID, bodyVersionID, input.Run.ID, observation.ObservedAt); err != nil {
		return fmt.Errorf("publish canonical body content period for job %s: %w", observation.SourceJobID, err)
	}
	if bodyCreated {
		if err := enqueuePublishedBodyAnalysis(ctx, tx, bodyVersionID); err != nil {
			return fmt.Errorf("enqueue body analysis for job %s: %w", observation.SourceJobID, err)
		}
	}
	return nil
}

func ensureBodyContentPeriod(ctx context.Context, tx pgx.Tx, presenceID, bodyVersionID, runID string, observedAt time.Time) error {
	var periodID, currentBodyVersionID string
	var validFrom time.Time
	err := tx.QueryRow(ctx, `
SELECT id::text, body_content_version_id::text, valid_from
FROM job_body_content_periods
WHERE presence_period_id = $1 AND valid_to IS NULL
FOR UPDATE`, presenceID).Scan(&periodID, &currentBodyVersionID, &validFrom)
	if err == pgx.ErrNoRows {
		if _, err := tx.Exec(ctx, `
INSERT INTO job_body_content_periods (
    id, presence_period_id, body_content_version_id, valid_from_run_id, valid_from
) VALUES ($1, $2, $3, $4, $5)`, newUUID(), presenceID, bodyVersionID, runID, observedAt); err != nil {
			return fmt.Errorf("open body content period: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("lock body content period: %w", err)
	}
	if currentBodyVersionID == bodyVersionID {
		return nil
	}
	boundary := observedAt
	if !boundary.After(validFrom) {
		boundary = validFrom.Add(time.Microsecond)
	}
	if _, err := tx.Exec(ctx, `
UPDATE job_body_content_periods
SET valid_to_run_id = $2, valid_to = $3
WHERE id = $1`, periodID, runID, boundary); err != nil {
		return fmt.Errorf("close body content period: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO job_body_content_periods (
    id, presence_period_id, body_content_version_id, valid_from_run_id, valid_from
) VALUES ($1, $2, $3, $4, $5)`, newUUID(), presenceID, bodyVersionID, runID, boundary); err != nil {
		return fmt.Errorf("open replacement body content period: %w", err)
	}
	return nil
}

// enqueuePublishedBodyAnalysis only targets the single published analysis
// configuration. Candidate configurations are queued by the explicit,
// all-history reanalysis flow; a source publication never causes a partial
// candidate result set to become current.
func enqueuePublishedBodyAnalysis(ctx context.Context, tx pgx.Tx, bodyVersionID string) error {
	var configurationID string
	err := tx.QueryRow(ctx, `
SELECT configuration.id::text
FROM technology_analysis_publication_state AS publication
JOIN technology_analysis_configurations AS configuration
  ON configuration.id = publication.configuration_id
WHERE publication.singleton = true AND configuration.status = 'published'`).Scan(&configurationID)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find published analysis configuration: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO technology_analysis_work_items (
    configuration_id, body_content_version_id, trigger
) VALUES ($1, $2, 'new_body')
ON CONFLICT (configuration_id, body_content_version_id) DO NOTHING`, configurationID, bodyVersionID); err != nil {
		return fmt.Errorf("enqueue published body analysis: %w", err)
	}
	return nil
}

func lockAnalysisPublicationStateForSourcePublish(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `
INSERT INTO technology_analysis_publication_state (singleton)
VALUES (true)
ON CONFLICT (singleton) DO NOTHING`); err != nil {
		return fmt.Errorf("ensure analysis publication state: %w", err)
	}
	if _, err := tx.Exec(ctx, `
SELECT singleton
FROM technology_analysis_publication_state
WHERE singleton = true
FOR SHARE`); err != nil {
		return fmt.Errorf("lock analysis publication state for source publication: %w", err)
	}
	return nil
}

func publishMissingPeriods(ctx context.Context, tx pgx.Tx, input PublishInput, observed map[string]struct{}) error {
	rows, err := tx.Query(ctx, `
SELECT p.id::text, p.job_id::text, j.source_job_id
FROM job_presence_periods AS p
JOIN jobs AS j ON j.id = p.job_id
WHERE p.observation_scope_id = $1 AND p.closed_at IS NULL
FOR UPDATE`, input.Scope.ID)
	if err != nil {
		return fmt.Errorf("lock missing presence periods: %w", err)
	}
	defer rows.Close()
	periods := make([]OpenPresence, 0)
	for rows.Next() {
		var period OpenPresence
		if err := rows.Scan(&period.ID, &period.JobID, &period.SourceJobID); err != nil {
			return fmt.Errorf("scan missing presence period: %w", err)
		}
		periods = append(periods, period)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate missing presence periods: %w", err)
	}
	for _, period := range periods {
		if _, listed := observed[period.SourceJobID]; listed {
			continue
		}
		result, exists := input.Rechecks[period.SourceJobID]
		if !exists {
			result = RecheckResult{Outcome: RecheckIndeterminate, Code: "missing_recheck_not_run"}
		}
		now := input.Manifest.FinishedAt.UTC()
		if _, err := tx.Exec(ctx, `
UPDATE job_presence_periods
SET first_missing_run_id = COALESCE(first_missing_run_id, $2),
    first_missing_at = COALESCE(first_missing_at, $3)
WHERE id = $1`, period.ID, input.Run.ID, now); err != nil {
			return fmt.Errorf("mark first missing job %s: %w", period.SourceJobID, err)
		}
		severity := "warning"
		message := "job was absent from the validated list"
		if result.Outcome == RecheckAvailable {
			severity = "info"
			message = "job was absent from list but detail remained available"
		}
		if err := appendAuditTx(ctx, tx, input.Run, AuditEvent{
			Stage: "missing-recheck", Severity: severity, Code: result.Code, Message: message,
			Diagnostics: map[string]any{"source_job_id": period.SourceJobID, "outcome": result.Outcome},
		}); err != nil {
			return err
		}
		if result.Outcome != RecheckDefinitelyUnavailable {
			continue
		}
		if _, err := tx.Exec(ctx, `
UPDATE job_presence_periods
SET closed_run_id = $2, closed_at = $3, closure_reason = 'detail_definitively_unavailable',
    closure_evidence = jsonb_build_object('recheck_code', $4::text)
WHERE id = $1`, period.ID, input.Run.ID, now, result.Code); err != nil {
			return fmt.Errorf("close presence period for job %s: %w", period.SourceJobID, err)
		}
		if _, err := tx.Exec(ctx, `
UPDATE job_content_periods
SET valid_to_run_id = $2,
    valid_to = CASE WHEN valid_from >= $3 THEN valid_from + interval '1 microsecond' ELSE $3 END
WHERE presence_period_id = $1 AND valid_to IS NULL`, period.ID, input.Run.ID, now); err != nil {
			return fmt.Errorf("close content period for job %s: %w", period.SourceJobID, err)
		}
		if _, err := tx.Exec(ctx, `
UPDATE job_body_content_periods
SET valid_to_run_id = $2,
    valid_to = CASE WHEN valid_from >= $3 THEN valid_from + interval '1 microsecond' ELSE $3 END
WHERE presence_period_id = $1 AND valid_to IS NULL`, period.ID, input.Run.ID, now); err != nil {
			return fmt.Errorf("close body content period for job %s: %w", period.SourceJobID, err)
		}
	}
	return nil
}

func appendAuditTx(ctx context.Context, tx pgx.Tx, run Run, event AuditEvent) error {
	diagnostics, err := safeDiagnostics(event.Diagnostics)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO collection_run_audit_events (run_id, stage, severity, code, attempt_number, message, diagnostics)
VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		run.ID, event.Stage, event.Severity, event.Code, run.AttemptNumber, SafeMessage(event.Message), diagnostics)
	if err != nil {
		return fmt.Errorf("append transactional audit event: %w", err)
	}
	return nil
}

func safeDiagnostics(input map[string]any) ([]byte, error) {
	if input == nil {
		return []byte(`{}`), nil
	}
	allowed := make(map[string]any, len(input))
	for key, value := range input {
		// Field names themselves are constrained so callers cannot smuggle a
		// complete request or raw body in under an arbitrary diagnostic key.
		if key == "source_job_id" || key == "outcome" || key == "http_status" || key == "object_key" || key == "attempt" || key == "observations" {
			allowed[key] = SafeMessage(fmt.Sprint(value))
		}
	}
	encoded, err := json.Marshal(allowed)
	if err != nil {
		return nil, fmt.Errorf("encode audit diagnostics: %w", err)
	}
	if len(encoded) > 8192 {
		return nil, fmt.Errorf("audit diagnostics exceed size limit")
	}
	return encoded, nil
}

func newUUID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		panic(fmt.Sprintf("read random UUID bytes: %v", err))
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(bytes[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:]
}
