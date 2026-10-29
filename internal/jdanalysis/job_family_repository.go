package jdanalysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

type JobFamilyIdentity struct {
	SourceMetadataVersionID      string
	ParserVersion                string
	DictionaryVersion            string
	DictionarySHA256             string
	ProjectMaskParserVersion     string
	ProjectMaskDictionaryVersion string
	ProjectMaskDictionarySHA256  string
}

type JobFamilyPublishResult struct {
	ResultID string
	Reused   bool
}

type JobFamilyFormalResult struct {
	Status            JobFamilyTitleStatus
	PrimaryFamilyKey  JobFamilyKey
	FamilyMatchCounts map[JobFamilyKey]int
	Evidence          []JobFamilyTitleEvidence
}

type jobFamilyTitleIdentity struct {
	ID                   string
	NormalizationVersion string
	SHA256               string
	Title                string
}

type jobFamilyObservation struct {
	Status        JobFamilyTitleStatus
	TitleIdentity *jobFamilyTitleIdentity
}

func CurrentJobFamilyIdentity(sourceMetadataVersionID string) JobFamilyIdentity {
	return JobFamilyIdentity{
		SourceMetadataVersionID:      sourceMetadataVersionID,
		ParserVersion:                JobFamilyParserVersion(),
		DictionaryVersion:            JobFamilyDictionaryVersion(),
		DictionarySHA256:             JobFamilyDictionaryDigest(),
		ProjectMaskParserVersion:     ProjectTagParserVersion(),
		ProjectMaskDictionaryVersion: ProjectTagDictionaryVersion(),
		ProjectMaskDictionarySHA256:  ProjectTagDictionaryDigest(),
	}
}

// PublishJobFamilyResult validates and stores the current immutable
// title-level result for one source-metadata version. It derives parser and
// dictionary provenance locally, so callers cannot label a current parse as a
// different ruleset. It never reads JD bodies or source-project metadata.
func (repository *Repository) PublishJobFamilyResult(ctx context.Context, sourceMetadataVersionID string, result JobFamilyTitleResult) (JobFamilyPublishResult, error) {
	return repository.publishJobFamilyResult(ctx, CurrentJobFamilyIdentity(sourceMetadataVersionID), result)
}

// publishJobFamilyResult is the single persistence implementation. It accepts
// an explicit identity only for controlled package-internal reprocessing and
// migration tests; production callers use PublishJobFamilyResult.
func (repository *Repository) publishJobFamilyResult(ctx context.Context, identity JobFamilyIdentity, result JobFamilyTitleResult) (JobFamilyPublishResult, error) {
	if err := validateJobFamilyIdentity(identity); err != nil {
		return JobFamilyPublishResult{}, err
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return JobFamilyPublishResult{}, fmt.Errorf("begin deterministic job-family publication: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	observation, err := ensureJobFamilyObservation(ctx, tx, identity.SourceMetadataVersionID)
	if err != nil {
		return JobFamilyPublishResult{}, err
	}
	if observation.Status != JobFamilyInScope || observation.TitleIdentity == nil {
		return JobFamilyPublishResult{}, fmt.Errorf("job-family publication requires an in-scope valid title")
	}
	if err := validateJobFamilyTitleResult(observation.TitleIdentity.Title, result); err != nil {
		return JobFamilyPublishResult{}, err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, jobFamilyIdentityLockKey(observation.TitleIdentity.ID, identity)); err != nil {
		return JobFamilyPublishResult{}, fmt.Errorf("lock deterministic job-family identity: %w", err)
	}
	resultID, reused, err := insertJobFamilyResult(ctx, tx, observation.TitleIdentity.ID, identity, result)
	if err != nil {
		return JobFamilyPublishResult{}, err
	}
	if reused {
		if err := tx.Commit(ctx); err != nil {
			return JobFamilyPublishResult{}, fmt.Errorf("commit reused deterministic job-family publication: %w", err)
		}
		return JobFamilyPublishResult{ResultID: resultID, Reused: true}, nil
	}
	if err := insertJobFamilyMatches(ctx, tx, resultID, *observation.TitleIdentity, result); err != nil {
		return JobFamilyPublishResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return JobFamilyPublishResult{}, fmt.Errorf("commit deterministic job-family publication: %w", err)
	}
	return JobFamilyPublishResult{ResultID: resultID}, nil
}

func validateJobFamilyIdentity(identity JobFamilyIdentity) error {
	if identity.SourceMetadataVersionID == "" || identity.ParserVersion == "" || identity.DictionaryVersion == "" || identity.ProjectMaskParserVersion == "" || identity.ProjectMaskDictionaryVersion == "" {
		return fmt.Errorf("deterministic job-family identity is incomplete")
	}
	for _, digest := range []string{identity.DictionarySHA256, identity.ProjectMaskDictionarySHA256} {
		if len(digest) != 64 {
			return fmt.Errorf("deterministic job-family dictionary digest is invalid")
		}
		for _, character := range digest {
			if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
				return fmt.Errorf("deterministic job-family dictionary digest is invalid")
			}
		}
	}
	return nil
}

func jobFamilyIdentityLockKey(titleIdentityID string, identity JobFamilyIdentity) string {
	return strings.Join([]string{
		titleIdentityID, identity.ParserVersion, identity.DictionaryVersion, identity.DictionarySHA256,
		identity.ProjectMaskParserVersion, identity.ProjectMaskDictionaryVersion, identity.ProjectMaskDictionarySHA256,
	}, "|")
}

func ensureJobFamilyObservation(ctx context.Context, tx pgx.Tx, sourceMetadataVersionID string) (jobFamilyObservation, error) {
	var normalizationVersion string
	var titleState, titleValue, categoryState, categoryValue *string
	if err := tx.QueryRow(ctx, `
SELECT normalization_version,
       metadata_payload #>> '{title,state}',
       metadata_payload #>> '{title,value}',
       metadata_payload #>> '{official_category,id,state}',
       metadata_payload #>> '{official_category,id,value}'
FROM job_source_metadata_versions
WHERE id = $1`, sourceMetadataVersionID).Scan(&normalizationVersion, &titleState, &titleValue, &categoryState, &categoryValue); err != nil {
		return jobFamilyObservation{}, fmt.Errorf("load job-family source metadata: %w", err)
	}

	status := jobFamilyObservationStatus(categoryState, categoryValue, titleState, titleValue)
	observation := jobFamilyObservation{Status: status}
	if titleState != nil && *titleState == "string" && titleValue != nil && strings.TrimSpace(*titleValue) != "" {
		identity, err := ensureJobFamilyTitleIdentity(ctx, tx, normalizationVersion, *titleValue)
		if err != nil {
			return jobFamilyObservation{}, err
		}
		observation.TitleIdentity = &identity
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO job_family_observations (source_metadata_version_id, job_family_title_identity_id, status)
VALUES ($1, $2, $3::job_family_observation_status)
ON CONFLICT (source_metadata_version_id) DO NOTHING`, sourceMetadataVersionID, nullableJobFamilyTitleIdentityID(observation.TitleIdentity), status); err != nil {
		return jobFamilyObservation{}, fmt.Errorf("insert job-family observation: %w", err)
	}
	var storedStatus string
	var storedTitleIdentityID *string
	if err := tx.QueryRow(ctx, `
SELECT status::text, job_family_title_identity_id::text
FROM job_family_observations
WHERE source_metadata_version_id = $1`, sourceMetadataVersionID).Scan(&storedStatus, &storedTitleIdentityID); err != nil {
		return jobFamilyObservation{}, fmt.Errorf("load job-family observation: %w", err)
	}
	if JobFamilyTitleStatus(storedStatus) != observation.Status || !sameNullableJobFamilyTitleIdentity(storedTitleIdentityID, observation.TitleIdentity) {
		return jobFamilyObservation{}, fmt.Errorf("source metadata conflicts with immutable job-family observation")
	}
	return observation, nil
}

func jobFamilyObservationStatus(categoryState, categoryValue, titleState, titleValue *string) JobFamilyTitleStatus {
	if categoryState == nil || categoryValue == nil || (*categoryState != "string" && *categoryState != "number") || strings.TrimSpace(*categoryValue) == "" {
		return JobFamilyScopeUnknown
	}
	if *categoryValue != "1" {
		return JobFamilyOutOfScope
	}
	if titleState == nil || *titleState != "string" || titleValue == nil || strings.TrimSpace(*titleValue) == "" {
		return JobFamilyInvalidTitle
	}
	return JobFamilyInScope
}

func nullableJobFamilyTitleIdentityID(identity *jobFamilyTitleIdentity) any {
	if identity == nil {
		return nil
	}
	return identity.ID
}

func sameNullableJobFamilyTitleIdentity(storedID *string, identity *jobFamilyTitleIdentity) bool {
	if storedID == nil || identity == nil {
		return storedID == nil && identity == nil
	}
	return *storedID == identity.ID
}

func ensureJobFamilyTitleIdentity(ctx context.Context, tx pgx.Tx, normalizationVersion, title string) (jobFamilyTitleIdentity, error) {
	digest := sha256.Sum256([]byte(title))
	identity := jobFamilyTitleIdentity{
		NormalizationVersion: normalizationVersion,
		SHA256:               hex.EncodeToString(digest[:]),
		Title:                title,
	}
	err := tx.QueryRow(ctx, `
INSERT INTO job_family_title_identities (title_sha256, normalization_version, normalized_title)
VALUES ($1, $2, $3)
ON CONFLICT (normalization_version, title_sha256) DO NOTHING
RETURNING id::text`, identity.SHA256, identity.NormalizationVersion, identity.Title).Scan(&identity.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
SELECT id::text
FROM job_family_title_identities
WHERE normalization_version = $1 AND title_sha256 = $2`, identity.NormalizationVersion, identity.SHA256).Scan(&identity.ID)
	}
	if err != nil {
		return jobFamilyTitleIdentity{}, fmt.Errorf("upsert deterministic job-family title identity: %w", err)
	}
	return identity, nil
}

func validateJobFamilyTitleResult(title string, result JobFamilyTitleResult) error {
	expected, err := ParseJobFamilyTitle(title)
	if err != nil {
		return fmt.Errorf("replay deterministic job-family parser: %w", err)
	}
	if !reflect.DeepEqual(result, expected) {
		return fmt.Errorf("deterministic job-family result does not match the approved title dictionary")
	}
	return nil
}

func insertJobFamilyResult(ctx context.Context, tx pgx.Tx, titleIdentityID string, identity JobFamilyIdentity, result JobFamilyTitleResult) (string, bool, error) {
	var resultID string
	err := tx.QueryRow(ctx, `
INSERT INTO job_family_results (
    job_family_title_identity_id, parser_version, dictionary_version, dictionary_sha256,
    project_mask_parser_version, project_mask_dictionary_version, project_mask_dictionary_sha256,
    status, primary_family_key
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::job_family_result_status, NULLIF($9, ''))
ON CONFLICT (
    job_family_title_identity_id, parser_version, dictionary_version, dictionary_sha256,
    project_mask_parser_version, project_mask_dictionary_version, project_mask_dictionary_sha256
) DO NOTHING
RETURNING id::text`, titleIdentityID, identity.ParserVersion, identity.DictionaryVersion, identity.DictionarySHA256,
		identity.ProjectMaskParserVersion, identity.ProjectMaskDictionaryVersion, identity.ProjectMaskDictionarySHA256,
		result.Status, result.PrimaryFamilyKey).Scan(&resultID)
	if err == nil {
		return resultID, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", false, fmt.Errorf("insert deterministic job-family result: %w", err)
	}
	if err := tx.QueryRow(ctx, `
SELECT id::text
FROM job_family_results
WHERE job_family_title_identity_id = $1
  AND parser_version = $2 AND dictionary_version = $3 AND dictionary_sha256 = $4
  AND project_mask_parser_version = $5
  AND project_mask_dictionary_version = $6
  AND project_mask_dictionary_sha256 = $7`, titleIdentityID, identity.ParserVersion, identity.DictionaryVersion, identity.DictionarySHA256,
		identity.ProjectMaskParserVersion, identity.ProjectMaskDictionaryVersion, identity.ProjectMaskDictionarySHA256).Scan(&resultID); err != nil {
		return "", false, fmt.Errorf("load existing deterministic job-family result: %w", err)
	}
	return resultID, true, nil
}

func insertJobFamilyMatches(ctx context.Context, tx pgx.Tx, resultID string, title jobFamilyTitleIdentity, result JobFamilyTitleResult) error {
	evidenceByFamily := make(map[JobFamilyKey][]JobFamilyTitleEvidence)
	for _, evidence := range result.Evidence {
		evidenceByFamily[evidence.FamilyKey] = append(evidenceByFamily[evidence.FamilyKey], evidence)
	}
	familyKeys := make([]JobFamilyKey, 0, len(result.FamilyMatchCounts))
	for familyKey := range result.FamilyMatchCounts {
		familyKeys = append(familyKeys, familyKey)
	}
	sort.Slice(familyKeys, func(left, right int) bool { return familyKeys[left] < familyKeys[right] })
	for _, familyKey := range familyKeys {
		var matchID string
		if err := tx.QueryRow(ctx, `
INSERT INTO job_family_matches (job_family_result_id, family_key, match_count)
VALUES ($1, $2, $3)
RETURNING id::text`, resultID, familyKey, result.FamilyMatchCounts[familyKey]).Scan(&matchID); err != nil {
			return fmt.Errorf("insert deterministic job-family match: %w", err)
		}
		for _, evidence := range evidenceByFamily[familyKey] {
			if _, err := CodePointSlice(title.Title, evidence.CodePointRange); err != nil {
				return fmt.Errorf("replay deterministic job-family evidence: %w", err)
			}
			if _, err := tx.Exec(ctx, `
INSERT INTO job_family_match_evidence (
    job_family_match_id, matched_alias, normalization_version,
    normalized_title_sha256, codepoint_start, codepoint_end
) VALUES ($1, $2, $3, $4, $5, $6)`, matchID, evidence.MatchedAlias,
				title.NormalizationVersion, title.SHA256, evidence.CodePointRange.Start, evidence.CodePointRange.End); err != nil {
				return fmt.Errorf("insert deterministic job-family evidence: %w", err)
			}
		}
	}
	return nil
}

// ReadFormalJobFamily returns explicit durable observation statuses. An
// in-scope title without its current completed result is unavailable rather
// than falling back to an older title or another analysis dimension.
func (repository *Repository) ReadFormalJobFamily(ctx context.Context, sourceMetadataVersionID string) (JobFamilyFormalResult, bool, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return JobFamilyFormalResult{}, false, fmt.Errorf("begin deterministic job-family read: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	formal, available, err := readFormalJobFamily(ctx, tx, sourceMetadataVersionID)
	if err != nil {
		return JobFamilyFormalResult{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return JobFamilyFormalResult{}, false, fmt.Errorf("commit deterministic job-family read: %w", err)
	}
	return formal, available, nil
}

func readFormalJobFamily(ctx context.Context, tx pgx.Tx, sourceMetadataVersionID string) (JobFamilyFormalResult, bool, error) {
	var status string
	var titleIdentityID *string
	err := tx.QueryRow(ctx, `
SELECT status::text, job_family_title_identity_id::text
FROM job_family_observations
WHERE source_metadata_version_id = $1`, sourceMetadataVersionID).Scan(&status, &titleIdentityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return JobFamilyFormalResult{}, false, nil
	}
	if err != nil {
		return JobFamilyFormalResult{}, false, fmt.Errorf("load formal job-family observation: %w", err)
	}
	formal := JobFamilyFormalResult{Status: JobFamilyTitleStatus(status), FamilyMatchCounts: make(map[JobFamilyKey]int)}
	if formal.Status != JobFamilyInScope {
		return formal, true, nil
	}
	if titleIdentityID == nil {
		return JobFamilyFormalResult{}, false, fmt.Errorf("in-scope job-family observation has no title identity")
	}
	identity := CurrentJobFamilyIdentity(sourceMetadataVersionID)
	var primary *string
	err = tx.QueryRow(ctx, `
SELECT status::text, primary_family_key
FROM job_family_results
WHERE job_family_title_identity_id = $1
  AND parser_version = $2 AND dictionary_version = $3 AND dictionary_sha256 = $4
  AND project_mask_parser_version = $5
  AND project_mask_dictionary_version = $6
  AND project_mask_dictionary_sha256 = $7`, *titleIdentityID, identity.ParserVersion, identity.DictionaryVersion, identity.DictionarySHA256,
		identity.ProjectMaskParserVersion, identity.ProjectMaskDictionaryVersion, identity.ProjectMaskDictionarySHA256).Scan(&status, &primary)
	if errors.Is(err, pgx.ErrNoRows) {
		return JobFamilyFormalResult{}, false, nil
	}
	if err != nil {
		return JobFamilyFormalResult{}, false, fmt.Errorf("load formal job-family result: %w", err)
	}
	formal.Status = JobFamilyTitleStatus(status)
	if primary != nil {
		formal.PrimaryFamilyKey = JobFamilyKey(*primary)
	}
	rows, err := tx.Query(ctx, `
SELECT match.family_key, match.match_count, evidence.matched_alias,
       evidence.codepoint_start, evidence.codepoint_end
FROM job_family_matches AS match
LEFT JOIN job_family_match_evidence AS evidence ON evidence.job_family_match_id = match.id
JOIN job_family_results AS result ON result.id = match.job_family_result_id
WHERE result.job_family_title_identity_id = $1
  AND result.parser_version = $2 AND result.dictionary_version = $3 AND result.dictionary_sha256 = $4
  AND result.project_mask_parser_version = $5
  AND result.project_mask_dictionary_version = $6
  AND result.project_mask_dictionary_sha256 = $7
ORDER BY match.family_key, evidence.codepoint_start, evidence.codepoint_end, evidence.matched_alias`, *titleIdentityID,
		identity.ParserVersion, identity.DictionaryVersion, identity.DictionarySHA256,
		identity.ProjectMaskParserVersion, identity.ProjectMaskDictionaryVersion, identity.ProjectMaskDictionarySHA256)
	if err != nil {
		return JobFamilyFormalResult{}, false, fmt.Errorf("load formal job-family matches: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var familyKey, alias *string
		var count *int
		var start, end *int
		if err := rows.Scan(&familyKey, &count, &alias, &start, &end); err != nil {
			return JobFamilyFormalResult{}, false, fmt.Errorf("scan formal job-family match: %w", err)
		}
		if familyKey == nil || count == nil {
			return JobFamilyFormalResult{}, false, fmt.Errorf("formal job-family match is incomplete")
		}
		key := JobFamilyKey(*familyKey)
		formal.FamilyMatchCounts[key] = *count
		if alias != nil && start != nil && end != nil {
			formal.Evidence = append(formal.Evidence, JobFamilyTitleEvidence{FamilyKey: key, MatchedAlias: *alias, CodePointRange: CodePointRange{Start: *start, End: *end}})
		}
	}
	if err := rows.Err(); err != nil {
		return JobFamilyFormalResult{}, false, fmt.Errorf("iterate formal job-family matches: %w", err)
	}
	return formal, true, nil
}
