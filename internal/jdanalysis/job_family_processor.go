package jdanalysis

import (
	"context"
	"fmt"
)

type JobFamilyProcessingSummary struct {
	Processed         int
	Reused            int
	Classified        int
	Unclassified      int
	OutOfScope        int
	ScopeUnknown      int
	InvalidTitle      int
	RetryableFailures int
}

type pendingJobFamilyTitleInput struct {
	SourceMetadataVersionID string
	TitleIdentityID         string
	Title                   string
}

// ProcessPendingJobFamilies independently attempts every missing immutable
// observation, then catches up in-scope title identities lacking the current
// parser and dictionary identities. A local failure is isolated and audited so
// remaining input and downstream deterministic stages can still progress.
func ProcessPendingJobFamilies(ctx context.Context, repository *Repository) (JobFamilyProcessingSummary, error) {
	identity := CurrentJobFamilyIdentity("processor")
	return processPendingJobFamiliesForIdentity(ctx, repository, identity)
}

func processPendingJobFamiliesForIdentity(ctx context.Context, repository *Repository, identity JobFamilyIdentity) (JobFamilyProcessingSummary, error) {
	if err := validateJobFamilyIdentity(identity); err != nil {
		return JobFamilyProcessingSummary{}, err
	}
	observationFailures, observationAuditErr := repository.registerJobFamilyObservations(ctx, identity)
	statusCounts, err := repository.jobFamilyObservationStatusCounts(ctx)
	if err != nil {
		return JobFamilyProcessingSummary{}, err
	}
	pending, err := repository.pendingJobFamilyTitleInputs(ctx, identity)
	if err != nil {
		return JobFamilyProcessingSummary{}, err
	}
	summary := JobFamilyProcessingSummary{
		OutOfScope:        statusCounts.OutOfScope,
		ScopeUnknown:      statusCounts.ScopeUnknown,
		InvalidTitle:      statusCounts.InvalidTitle,
		RetryableFailures: observationFailures,
	}
	retryAuditErr := observationAuditErr
	for _, input := range pending {
		parsed, parseErr := ParseJobFamilyTitle(input.Title)
		if parseErr == nil {
			var published JobFamilyPublishResult
			published, parseErr = repository.PublishJobFamilyResult(ctx, input.SourceMetadataVersionID, parsed)
			if parseErr == nil {
				if published.Reused {
					summary.Reused++
					continue
				}
				summary.Processed++
				switch parsed.Status {
				case JobFamilyClassified:
					summary.Classified++
				case JobFamilyUnclassified:
					summary.Unclassified++
				}
				continue
			}
		}
		summary.RetryableFailures++
		current := identity
		current.SourceMetadataVersionID = input.SourceMetadataVersionID
		if auditErr := repository.RecordJobFamilyRetryAudit(ctx, input.TitleIdentityID, current, "job_family_processing_failed"); auditErr != nil {
			// A retry-audit storage outage must be observable, but it must not
			// turn one failed title into a batch-wide processing block. Keep
			// trying independent titles and surface the first safe aggregate
			// failure after the loop; the failed title remains pending because
			// no completed result was published.
			if retryAuditErr == nil {
				retryAuditErr = fmt.Errorf("record deterministic job-family retry audit: %w", auditErr)
			}
		}
	}
	return summary, retryAuditErr
}

type jobFamilyObservationStatusCounts struct {
	OutOfScope   int
	ScopeUnknown int
	InvalidTitle int
}

// jobFamilyObservationStatusCounts reports safe corpus-level counts for the
// explicit outcomes that are intentionally skipped before title parsing.
func (repository *Repository) jobFamilyObservationStatusCounts(ctx context.Context) (jobFamilyObservationStatusCounts, error) {
	counts := jobFamilyObservationStatusCounts{}
	rows, err := repository.pool.Query(ctx, `
SELECT status::text, count(*)
FROM job_family_observations
WHERE status IN ('out_of_scope', 'scope_unknown', 'invalid_title')
GROUP BY status`)
	if err != nil {
		return jobFamilyObservationStatusCounts{}, fmt.Errorf("count skipped deterministic job-family observations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return jobFamilyObservationStatusCounts{}, fmt.Errorf("scan skipped deterministic job-family observation count: %w", err)
		}
		switch JobFamilyTitleStatus(status) {
		case JobFamilyOutOfScope:
			counts.OutOfScope = count
		case JobFamilyScopeUnknown:
			counts.ScopeUnknown = count
		case JobFamilyInvalidTitle:
			counts.InvalidTitle = count
		}
	}
	if err := rows.Err(); err != nil {
		return jobFamilyObservationStatusCounts{}, fmt.Errorf("iterate skipped deterministic job-family observation counts: %w", err)
	}
	return counts, nil
}

// registerJobFamilyObservations retains each observation in its own
// transaction. A single source-metadata failure must not prevent unrelated
// observations or title results from being delivered in the same run.
func (repository *Repository) registerJobFamilyObservations(ctx context.Context, identity JobFamilyIdentity) (int, error) {
	rows, err := repository.pool.Query(ctx, `
SELECT metadata.id::text
FROM job_source_metadata_versions AS metadata
WHERE NOT EXISTS (
    SELECT 1
    FROM job_family_observations AS observation
    WHERE observation.source_metadata_version_id = metadata.id
)
ORDER BY metadata.created_at, metadata.id`)
	if err != nil {
		return 0, fmt.Errorf("select unobserved job-family metadata: %w", err)
	}
	defer rows.Close()
	metadataIDs := make([]string, 0)
	for rows.Next() {
		var metadataID string
		if err := rows.Scan(&metadataID); err != nil {
			return 0, fmt.Errorf("scan unobserved job-family metadata: %w", err)
		}
		metadataIDs = append(metadataIDs, metadataID)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate unobserved job-family metadata: %w", err)
	}
	var retryAuditErr error
	failures := 0
	for _, metadataID := range metadataIDs {
		tx, err := repository.pool.Begin(ctx)
		ensureErr := err
		if ensureErr == nil {
			_, ensureErr = ensureJobFamilyObservation(ctx, tx, metadataID)
		}
		if ensureErr == nil {
			ensureErr = tx.Commit(ctx)
		}
		if ensureErr != nil {
			if tx != nil {
				_ = tx.Rollback(context.Background())
			}
			failures++
			current := identity
			current.SourceMetadataVersionID = metadataID
			if auditErr := repository.RecordJobFamilyObservationRetryAudit(ctx, current, "job_family_observation_failed"); auditErr != nil && retryAuditErr == nil {
				// Do not let an audit outage stop independent observations. Surface
				// only an aggregate safe operation error after the complete pass.
				retryAuditErr = fmt.Errorf("record deterministic job-family observation retry audit: %w", auditErr)
			}
		}
	}
	return failures, retryAuditErr
}

func (repository *Repository) pendingJobFamilyTitleInputs(ctx context.Context, identity JobFamilyIdentity) ([]pendingJobFamilyTitleInput, error) {
	rows, err := repository.pool.Query(ctx, `
SELECT DISTINCT ON (title.id)
       observation.source_metadata_version_id::text, title.id::text, title.normalized_title
FROM job_family_title_identities AS title
JOIN job_family_observations AS observation
  ON observation.job_family_title_identity_id = title.id
WHERE observation.status = 'in_scope'
  AND NOT EXISTS (
      SELECT 1
      FROM job_family_results AS result
      WHERE result.job_family_title_identity_id = title.id
        AND result.parser_version = $1
        AND result.dictionary_version = $2
        AND result.dictionary_sha256 = $3
        AND result.project_mask_parser_version = $4
        AND result.project_mask_dictionary_version = $5
        AND result.project_mask_dictionary_sha256 = $6
  )
ORDER BY title.id, observation.source_metadata_version_id`, identity.ParserVersion, identity.DictionaryVersion, identity.DictionarySHA256,
		identity.ProjectMaskParserVersion, identity.ProjectMaskDictionaryVersion, identity.ProjectMaskDictionarySHA256)
	if err != nil {
		return nil, fmt.Errorf("select pending deterministic job-family titles: %w", err)
	}
	defer rows.Close()
	inputs := make([]pendingJobFamilyTitleInput, 0)
	for rows.Next() {
		var input pendingJobFamilyTitleInput
		if err := rows.Scan(&input.SourceMetadataVersionID, &input.TitleIdentityID, &input.Title); err != nil {
			return nil, fmt.Errorf("scan pending deterministic job-family title: %w", err)
		}
		inputs = append(inputs, input)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending deterministic job-family titles: %w", err)
	}
	return inputs, nil
}

// RecordJobFamilyRetryAudit retains only a stable safe error code and
// immutable processing identity. It intentionally records no title or JD text.
func (repository *Repository) RecordJobFamilyRetryAudit(ctx context.Context, titleIdentityID string, identity JobFamilyIdentity, safeErrorCode string) error {
	if titleIdentityID == "" || safeErrorCode == "" {
		return fmt.Errorf("deterministic job-family retry audit is incomplete")
	}
	if err := validateJobFamilyIdentity(identity); err != nil {
		return err
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO job_family_retry_audits (
    job_family_title_identity_id, parser_version, dictionary_version, dictionary_sha256,
    project_mask_parser_version, project_mask_dictionary_version, project_mask_dictionary_sha256,
    safe_error_code
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, titleIdentityID,
		identity.ParserVersion, identity.DictionaryVersion, identity.DictionarySHA256,
		identity.ProjectMaskParserVersion, identity.ProjectMaskDictionaryVersion, identity.ProjectMaskDictionarySHA256,
		safeErrorCode); err != nil {
		return fmt.Errorf("insert deterministic job-family retry audit: %w", err)
	}
	return nil
}

// RecordJobFamilyObservationRetryAudit retains a safe retry event for a
// source metadata version whose observation could not be created. It does not
// store a title, a JD body, or any source payload field.
func (repository *Repository) RecordJobFamilyObservationRetryAudit(ctx context.Context, identity JobFamilyIdentity, safeErrorCode string) error {
	if safeErrorCode == "" {
		return fmt.Errorf("deterministic job-family observation retry audit is incomplete")
	}
	if err := validateJobFamilyIdentity(identity); err != nil {
		return err
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO job_family_observation_retry_audits (
    source_metadata_version_id, parser_version, dictionary_version, dictionary_sha256,
    project_mask_parser_version, project_mask_dictionary_version, project_mask_dictionary_sha256,
    safe_error_code
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, identity.SourceMetadataVersionID,
		identity.ParserVersion, identity.DictionaryVersion, identity.DictionarySHA256,
		identity.ProjectMaskParserVersion, identity.ProjectMaskDictionaryVersion, identity.ProjectMaskDictionarySHA256,
		safeErrorCode); err != nil {
		return fmt.Errorf("insert deterministic job-family observation retry audit: %w", err)
	}
	return nil
}
