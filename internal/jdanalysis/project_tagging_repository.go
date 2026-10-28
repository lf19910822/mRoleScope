package jdanalysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"

	"github.com/jackc/pgx/v5"
)

type ProjectTagIdentity struct {
	SourceMetadataVersionID string
	ParserVersion           string
	DictionaryVersion       string
	DictionarySHA256        string
}

type ProjectTagPublishResult struct {
	ResultID string
	Reused   bool
}

type StoredProjectTagResult struct {
	ID              string
	TitleIdentityID string
	Identity        ProjectTagIdentity
	Result          ProjectTitleTagResult
}

type projectTitleIdentity struct {
	ID                   string
	NormalizationVersion string
	SHA256               string
	Title                string
}

// PublishProjectTagResult stores one immutable title-only result. It derives
// the normalized title solely from the referenced source-metadata version.
func (repository *Repository) PublishProjectTagResult(ctx context.Context, identity ProjectTagIdentity, result ProjectTitleTagResult) (ProjectTagPublishResult, error) {
	if err := validateProjectTagIdentity(identity); err != nil {
		return ProjectTagPublishResult{}, err
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ProjectTagPublishResult{}, fmt.Errorf("begin deterministic project-tag publication: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	titleIdentity, err := ensureProjectTitleObservation(ctx, tx, identity.SourceMetadataVersionID)
	if err != nil {
		return ProjectTagPublishResult{}, err
	}
	if err := validateProjectTitleTagResult(titleIdentity.Title, result); err != nil {
		return ProjectTagPublishResult{}, err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, titleIdentity.ID+"|"+identity.ParserVersion+"|"+identity.DictionaryVersion+"|"+identity.DictionarySHA256); err != nil {
		return ProjectTagPublishResult{}, fmt.Errorf("lock deterministic project-tag identity: %w", err)
	}
	resultID, reused, err := insertProjectTagResult(ctx, tx, titleIdentity.ID, identity)
	if err != nil {
		return ProjectTagPublishResult{}, err
	}
	if reused {
		if err := tx.Commit(ctx); err != nil {
			return ProjectTagPublishResult{}, fmt.Errorf("commit reused deterministic project-tag publication: %w", err)
		}
		return ProjectTagPublishResult{ResultID: resultID, Reused: true}, nil
	}
	for _, assignment := range result.Assignments {
		assignmentID, err := insertProjectTagAssignment(ctx, tx, resultID, assignment)
		if err != nil {
			return ProjectTagPublishResult{}, err
		}
		for _, evidence := range assignment.Evidence {
			if err := insertProjectTagEvidence(ctx, tx, assignmentID, titleIdentity, evidence); err != nil {
				return ProjectTagPublishResult{}, err
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return ProjectTagPublishResult{}, fmt.Errorf("commit deterministic project-tag publication: %w", err)
	}
	return ProjectTagPublishResult{ResultID: resultID}, nil
}

func validateProjectTagIdentity(identity ProjectTagIdentity) error {
	if identity.SourceMetadataVersionID == "" || identity.ParserVersion == "" || identity.DictionaryVersion == "" || len(identity.DictionarySHA256) != 64 {
		return fmt.Errorf("deterministic project-tag identity is incomplete")
	}
	for _, character := range identity.DictionarySHA256 {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
			return fmt.Errorf("deterministic project-tag dictionary digest is invalid")
		}
	}
	return nil
}

func ensureProjectTitleObservation(ctx context.Context, tx pgx.Tx, sourceMetadataVersionID string) (projectTitleIdentity, error) {
	var normalizationVersion, title string
	err := tx.QueryRow(ctx, `
SELECT normalization_version, metadata_payload #>> '{title,value}'
FROM job_source_metadata_versions
WHERE id = $1
  AND metadata_payload #>> '{title,state}' = 'string'`, sourceMetadataVersionID).Scan(&normalizationVersion, &title)
	if err != nil {
		return projectTitleIdentity{}, fmt.Errorf("load normalized project title: %w", err)
	}
	digest := sha256.Sum256([]byte(title))
	titleSHA256 := hex.EncodeToString(digest[:])
	identity := projectTitleIdentity{NormalizationVersion: normalizationVersion, SHA256: titleSHA256, Title: title}
	err = tx.QueryRow(ctx, `
INSERT INTO project_title_identities (title_sha256, normalization_version, normalized_title)
VALUES ($1, $2, $3)
ON CONFLICT (normalization_version, title_sha256) DO NOTHING
RETURNING id::text`, titleSHA256, normalizationVersion, title).Scan(&identity.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
SELECT id::text
FROM project_title_identities
WHERE normalization_version = $1 AND title_sha256 = $2`, normalizationVersion, titleSHA256).Scan(&identity.ID)
	}
	if err != nil {
		return projectTitleIdentity{}, fmt.Errorf("upsert deterministic project title identity: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO project_title_observations (source_metadata_version_id, project_title_identity_id)
VALUES ($1, $2)
ON CONFLICT (source_metadata_version_id) DO NOTHING`, sourceMetadataVersionID, identity.ID); err != nil {
		return projectTitleIdentity{}, fmt.Errorf("link source metadata to deterministic project title: %w", err)
	}
	var observedIdentityID string
	if err := tx.QueryRow(ctx, `
SELECT project_title_identity_id::text
FROM project_title_observations
WHERE source_metadata_version_id = $1`, sourceMetadataVersionID).Scan(&observedIdentityID); err != nil {
		return projectTitleIdentity{}, fmt.Errorf("load deterministic project title observation: %w", err)
	}
	if observedIdentityID != identity.ID {
		return projectTitleIdentity{}, fmt.Errorf("source metadata title identity conflicts with immutable observation")
	}
	return identity, nil
}

func validateProjectTitleTagResult(title string, result ProjectTitleTagResult) error {
	expected, err := ParseProjectTitle(title)
	if err != nil {
		return fmt.Errorf("replay deterministic project-title parser: %w", err)
	}
	if !reflect.DeepEqual(result, expected) {
		return fmt.Errorf("deterministic project-tag result does not match the approved title dictionary")
	}
	seenAssignments := make(map[string]struct{}, len(result.Assignments))
	for _, assignment := range result.Assignments {
		if assignment.ProjectKey == "" || assignment.DisplayName == "" || !validProjectTagClassification(assignment.Classification) || len(assignment.Evidence) == 0 {
			return fmt.Errorf("deterministic project-tag assignment is invalid")
		}
		if _, exists := seenAssignments[assignment.ProjectKey]; exists {
			return fmt.Errorf("deterministic project-tag result repeats a project key")
		}
		seenAssignments[assignment.ProjectKey] = struct{}{}
		seenEvidence := make(map[ProjectTitleEvidence]struct{}, len(assignment.Evidence))
		for _, evidence := range assignment.Evidence {
			if evidence.MatchedKeyword == "" || !evidence.CodePointRange.ValidFor(title) {
				return fmt.Errorf("deterministic project-tag evidence is invalid")
			}
			if _, exists := seenEvidence[evidence]; exists {
				return fmt.Errorf("deterministic project-tag assignment repeats evidence")
			}
			seenEvidence[evidence] = struct{}{}
		}
	}
	return nil
}

func insertProjectTagResult(ctx context.Context, tx pgx.Tx, titleIdentityID string, identity ProjectTagIdentity) (string, bool, error) {
	var resultID string
	err := tx.QueryRow(ctx, `
INSERT INTO project_tag_results (
    project_title_identity_id, parser_version, dictionary_version, dictionary_sha256, status
) VALUES ($1, $2, $3, $4, 'complete')
ON CONFLICT (project_title_identity_id, parser_version, dictionary_version, dictionary_sha256) DO NOTHING
RETURNING id::text`, titleIdentityID, identity.ParserVersion, identity.DictionaryVersion, identity.DictionarySHA256).Scan(&resultID)
	if err == nil {
		return resultID, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", false, fmt.Errorf("insert deterministic project-tag result: %w", err)
	}
	if err := tx.QueryRow(ctx, `
SELECT id::text
FROM project_tag_results
WHERE project_title_identity_id = $1
  AND parser_version = $2
  AND dictionary_version = $3
  AND dictionary_sha256 = $4`, titleIdentityID, identity.ParserVersion, identity.DictionaryVersion, identity.DictionarySHA256).Scan(&resultID); err != nil {
		return "", false, fmt.Errorf("load existing deterministic project-tag result: %w", err)
	}
	return resultID, true, nil
}

func insertProjectTagAssignment(ctx context.Context, tx pgx.Tx, resultID string, assignment ProjectTagAssignment) (string, error) {
	var assignmentID string
	if err := tx.QueryRow(ctx, `
INSERT INTO project_tag_assignments (project_tag_result_id, project_key, display_name, classification)
VALUES ($1, $2, $3, $4::project_tag_classification)
RETURNING id::text`, resultID, assignment.ProjectKey, assignment.DisplayName, assignment.Classification).Scan(&assignmentID); err != nil {
		return "", fmt.Errorf("insert deterministic project-tag assignment: %w", err)
	}
	return assignmentID, nil
}

func insertProjectTagEvidence(ctx context.Context, tx pgx.Tx, assignmentID string, title projectTitleIdentity, evidence ProjectTitleEvidence) error {
	if _, err := CodePointSlice(title.Title, evidence.CodePointRange); err != nil {
		return fmt.Errorf("replay deterministic project-tag evidence: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO project_tag_evidence (
    project_tag_assignment_id, matched_keyword, normalization_version,
    normalized_title_sha256, codepoint_start, codepoint_end
) VALUES ($1, $2, $3, $4, $5, $6)`, assignmentID, evidence.MatchedKeyword,
		title.NormalizationVersion, title.SHA256, evidence.CodePointRange.Start, evidence.CodePointRange.End); err != nil {
		return fmt.Errorf("insert deterministic project-tag evidence: %w", err)
	}
	return nil
}

func (repository *Repository) ReadDeterministicProjectTagResult(ctx context.Context, identity ProjectTagIdentity) (StoredProjectTagResult, error) {
	if err := validateProjectTagIdentity(identity); err != nil {
		return StoredProjectTagResult{}, err
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return StoredProjectTagResult{}, fmt.Errorf("begin deterministic project-tag read: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	stored, err := readDeterministicProjectTagResult(ctx, tx, identity)
	if err != nil {
		return StoredProjectTagResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return StoredProjectTagResult{}, fmt.Errorf("commit deterministic project-tag read: %w", err)
	}
	return stored, nil
}

func readDeterministicProjectTagResult(ctx context.Context, tx pgx.Tx, identity ProjectTagIdentity) (StoredProjectTagResult, error) {
	stored := StoredProjectTagResult{Identity: identity}
	if err := tx.QueryRow(ctx, `
SELECT result.id::text, result.project_title_identity_id::text
FROM project_tag_results AS result
JOIN project_title_observations AS observation ON observation.project_title_identity_id = result.project_title_identity_id
WHERE observation.source_metadata_version_id = $1
  AND result.parser_version = $2
  AND result.dictionary_version = $3
  AND result.dictionary_sha256 = $4`, identity.SourceMetadataVersionID, identity.ParserVersion, identity.DictionaryVersion, identity.DictionarySHA256).Scan(&stored.ID, &stored.TitleIdentityID); err != nil {
		return StoredProjectTagResult{}, fmt.Errorf("load deterministic project-tag result: %w", err)
	}
	rows, err := tx.Query(ctx, `
SELECT assignment.project_key, assignment.display_name, assignment.classification::text,
       evidence.matched_keyword, evidence.codepoint_start, evidence.codepoint_end
FROM project_tag_assignments AS assignment
LEFT JOIN project_tag_evidence AS evidence ON evidence.project_tag_assignment_id = assignment.id
WHERE assignment.project_tag_result_id = $1
ORDER BY assignment.project_key, evidence.codepoint_start, evidence.codepoint_end, evidence.matched_keyword`, stored.ID)
	if err != nil {
		return StoredProjectTagResult{}, fmt.Errorf("load deterministic project-tag assignments: %w", err)
	}
	defer rows.Close()
	indexes := make(map[string]int)
	for rows.Next() {
		var projectKey, displayName, classification string
		var keyword *string
		var start, end *int
		if err := rows.Scan(&projectKey, &displayName, &classification, &keyword, &start, &end); err != nil {
			return StoredProjectTagResult{}, fmt.Errorf("scan deterministic project-tag assignment: %w", err)
		}
		index, exists := indexes[projectKey]
		if !exists {
			index = len(stored.Result.Assignments)
			indexes[projectKey] = index
			stored.Result.Assignments = append(stored.Result.Assignments, ProjectTagAssignment{ProjectKey: projectKey, DisplayName: displayName, Classification: ProjectTagClassification(classification)})
		}
		if keyword != nil && start != nil && end != nil {
			stored.Result.Assignments[index].Evidence = append(stored.Result.Assignments[index].Evidence, ProjectTitleEvidence{MatchedKeyword: *keyword, CodePointRange: CodePointRange{Start: *start, End: *end}})
		}
	}
	if err := rows.Err(); err != nil {
		return StoredProjectTagResult{}, fmt.Errorf("iterate deterministic project-tag assignments: %w", err)
	}
	return stored, nil
}

// ReadFormalProjectTags has no fallback to source project metadata, bodies,
// technology analysis, or AI projections. A missing deterministic result is
// unavailable; a present result with no assignments is identified but empty.
func (repository *Repository) ReadFormalProjectTags(ctx context.Context, sourceMetadataVersionID string) (ProjectTitleTagResult, bool, error) {
	stored, err := repository.ReadDeterministicProjectTagResult(ctx, ProjectTagIdentity{
		SourceMetadataVersionID: sourceMetadataVersionID,
		ParserVersion:           ProjectTagParserVersion(),
		DictionaryVersion:       ProjectTagDictionaryVersion(),
		DictionarySHA256:        ProjectTagDictionaryDigest(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ProjectTitleTagResult{}, false, nil
	}
	if err != nil {
		return ProjectTitleTagResult{}, false, err
	}
	return stored.Result, true, nil
}
