package jdanalysis

import (
	"context"
	"fmt"
)

type ProjectTagProcessingSummary struct {
	Processed    int
	Reused       int
	Unidentified int
}

// ProcessPendingProjectTags performs the provider-free title baseline and
// daily incrementality using only the shipped parser and dictionary identity.
func ProcessPendingProjectTags(ctx context.Context, repository *Repository) (ProjectTagProcessingSummary, error) {
	return processPendingProjectTagsForIdentity(ctx, repository, ProjectTagParserVersion(), ProjectTagDictionaryVersion(), ProjectTagDictionaryDigest())
}

func processPendingProjectTagsForIdentity(ctx context.Context, repository *Repository, parserVersion, dictionaryVersion, dictionarySHA256 string) (ProjectTagProcessingSummary, error) {
	identity := ProjectTagIdentity{ParserVersion: parserVersion, DictionaryVersion: dictionaryVersion, DictionarySHA256: dictionarySHA256}
	if err := validateProjectTagIdentity(ProjectTagIdentity{SourceMetadataVersionID: "processor", ParserVersion: parserVersion, DictionaryVersion: dictionaryVersion, DictionarySHA256: dictionarySHA256}); err != nil {
		return ProjectTagProcessingSummary{}, err
	}
	if err := repository.registerProjectTitleObservations(ctx); err != nil {
		return ProjectTagProcessingSummary{}, err
	}
	pending, err := repository.pendingProjectTitleInputs(ctx, identity)
	if err != nil {
		return ProjectTagProcessingSummary{}, err
	}
	summary := ProjectTagProcessingSummary{}
	for _, input := range pending {
		parsed, err := ParseProjectTitle(input.Title)
		if err != nil {
			return ProjectTagProcessingSummary{}, fmt.Errorf("parse deterministic project title: %w", err)
		}
		identity.SourceMetadataVersionID = input.SourceMetadataVersionID
		published, err := repository.PublishProjectTagResult(ctx, identity, parsed)
		if err != nil {
			return ProjectTagProcessingSummary{}, err
		}
		if published.Reused {
			summary.Reused++
			continue
		}
		summary.Processed++
		if len(parsed.Assignments) == 0 {
			summary.Unidentified++
		}
	}
	return summary, nil
}

type pendingProjectTitleInput struct {
	SourceMetadataVersionID string
	Title                   string
}

func (repository *Repository) registerProjectTitleObservations(ctx context.Context) error {
	rows, err := repository.pool.Query(ctx, `
SELECT metadata.id::text
FROM job_source_metadata_versions AS metadata
WHERE metadata.metadata_payload #>> '{title,state}' = 'string'
  AND NOT EXISTS (
      SELECT 1
      FROM project_title_observations AS observation
      WHERE observation.source_metadata_version_id = metadata.id
  )
ORDER BY metadata.created_at, metadata.id`)
	if err != nil {
		return fmt.Errorf("select unobserved project titles: %w", err)
	}
	defer rows.Close()
	metadataIDs := make([]string, 0)
	for rows.Next() {
		var metadataID string
		if err := rows.Scan(&metadataID); err != nil {
			return fmt.Errorf("scan unobserved project title: %w", err)
		}
		metadataIDs = append(metadataIDs, metadataID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate unobserved project titles: %w", err)
	}
	for _, metadataID := range metadataIDs {
		tx, err := repository.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin project title observation: %w", err)
		}
		_, ensureErr := ensureProjectTitleObservation(ctx, tx, metadataID)
		if ensureErr == nil {
			ensureErr = tx.Commit(ctx)
		}
		if ensureErr != nil {
			_ = tx.Rollback(context.Background())
			return ensureErr
		}
	}
	return nil
}

func (repository *Repository) pendingProjectTitleInputs(ctx context.Context, identity ProjectTagIdentity) ([]pendingProjectTitleInput, error) {
	rows, err := repository.pool.Query(ctx, `
SELECT DISTINCT ON (title.id)
       observation.source_metadata_version_id::text, title.normalized_title
FROM project_title_identities AS title
JOIN project_title_observations AS observation ON observation.project_title_identity_id = title.id
WHERE NOT EXISTS (
    SELECT 1
    FROM project_tag_results AS result
    WHERE result.project_title_identity_id = title.id
      AND result.parser_version = $1
      AND result.dictionary_version = $2
      AND result.dictionary_sha256 = $3
)
ORDER BY title.id, observation.source_metadata_version_id`, identity.ParserVersion, identity.DictionaryVersion, identity.DictionarySHA256)
	if err != nil {
		return nil, fmt.Errorf("select pending deterministic project titles: %w", err)
	}
	defer rows.Close()
	inputs := make([]pendingProjectTitleInput, 0)
	for rows.Next() {
		var input pendingProjectTitleInput
		if err := rows.Scan(&input.SourceMetadataVersionID, &input.Title); err != nil {
			return nil, fmt.Errorf("scan pending deterministic project title: %w", err)
		}
		inputs = append(inputs, input)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending deterministic project titles: %w", err)
	}
	return inputs, nil
}
