package jdanalysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// TrendFormalReleaseStatus is the lifecycle state of a complete selection of
// deterministic interpretation identities for historical trend reads.
type TrendFormalReleaseStatus string

const (
	TrendFormalReleaseCandidate  TrendFormalReleaseStatus = "candidate"
	TrendFormalReleasePublished  TrendFormalReleaseStatus = "published"
	TrendFormalReleaseSuperseded TrendFormalReleaseStatus = "superseded"
)

// TrendFormalRelease exposes only the release identity needed by the analysis
// lifecycle. It deliberately contains no source, body, result, or audit data.
type TrendFormalRelease struct {
	ID             string
	IdentitySHA256 string
	Reused         bool
	Status         TrendFormalReleaseStatus
}

var ErrNoPublishedTrendTechnologyConfiguration = errors.New("no published technology configuration for trend release")
var ErrTrendFormalReleaseIncomplete = errors.New("trend formal release is incomplete")
var ErrNoPublishedTrendFormalRelease = errors.New("no published trend formal release")

type trendFormalReleaseComponents struct {
	Education                 EducationRequirementIdentity
	Experience                ExperienceRequirementIdentity
	JobFamily                 JobFamilyIdentity
	Project                   ProjectTagIdentity
	TechnologyConfigurationID string
}

// PrepareTrendFormalRelease records the current deterministic identities as a
// single immutable candidate. It performs no source parsing and does not make
// the candidate visible to historical trend readers; publication remains a
// separate full-coverage gate.
func (repository *Repository) PrepareTrendFormalRelease(ctx context.Context) (TrendFormalRelease, error) {
	if repository == nil || repository.pool == nil {
		return TrendFormalRelease{}, fmt.Errorf("analysis repository is unavailable")
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return TrendFormalRelease{}, fmt.Errorf("begin trend formal release preparation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	technologyConfigurationID, err := readPublishedTrendTechnologyConfiguration(ctx, tx)
	if err != nil {
		return TrendFormalRelease{}, err
	}
	components := currentTrendFormalReleaseComponents(technologyConfigurationID)
	identitySHA256 := trendFormalReleaseIdentityDigest(components)

	var release TrendFormalRelease
	err = tx.QueryRow(ctx, `
INSERT INTO trend_formal_releases (identity_sha256)
VALUES ($1)
ON CONFLICT (identity_sha256) DO NOTHING
RETURNING id::text, identity_sha256, status::text`, identitySHA256).Scan(
		&release.ID, &release.IdentitySHA256, &release.Status,
	)
	if err == pgx.ErrNoRows {
		if err := tx.QueryRow(ctx, `
SELECT id::text, identity_sha256, status::text
FROM trend_formal_releases
WHERE identity_sha256 = $1`, identitySHA256).Scan(
			&release.ID, &release.IdentitySHA256, &release.Status,
		); err != nil {
			return TrendFormalRelease{}, fmt.Errorf("load existing trend formal release: %w", err)
		}
		release.Reused = true
		if err := tx.Commit(ctx); err != nil {
			return TrendFormalRelease{}, fmt.Errorf("commit reused trend formal release preparation: %w", err)
		}
		return release, nil
	}
	if err != nil {
		return TrendFormalRelease{}, fmt.Errorf("create trend formal release candidate: %w", err)
	}
	if err := insertTrendFormalReleaseComponents(ctx, tx, release.ID, components); err != nil {
		return TrendFormalRelease{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TrendFormalRelease{}, fmt.Errorf("commit trend formal release preparation: %w", err)
	}
	return release, nil
}

func readPublishedTrendTechnologyConfiguration(ctx context.Context, tx pgx.Tx) (string, error) {
	var configurationID *string
	err := tx.QueryRow(ctx, `
SELECT publication.configuration_id::text
FROM technology_analysis_publication_state AS publication
WHERE publication.singleton = true
FOR SHARE`).Scan(&configurationID)
	if err != nil {
		return "", fmt.Errorf("read published technology configuration for trend release: %w", err)
	}
	if configurationID == nil {
		return "", ErrNoPublishedTrendTechnologyConfiguration
	}
	var status string
	if err := tx.QueryRow(ctx, `
SELECT status::text
FROM technology_analysis_configurations
WHERE id = $1`, *configurationID).Scan(&status); err != nil {
		return "", fmt.Errorf("read trend technology configuration status: %w", err)
	}
	if status != "published" {
		return "", ErrNoPublishedTrendTechnologyConfiguration
	}
	return *configurationID, nil
}

func currentTrendFormalReleaseComponents(technologyConfigurationID string) trendFormalReleaseComponents {
	return trendFormalReleaseComponents{
		TechnologyConfigurationID: technologyConfigurationID,
		Education: EducationRequirementIdentity{
			ParserVersion:  EducationRequirementParserVersion(),
			LexiconVersion: EducationRequirementLexiconVersion(),
			LexiconSHA256:  EducationRequirementLexiconDigest(),
		},
		Experience: ExperienceRequirementIdentity{
			ParserVersion:  ExperienceRequirementParserVersion(),
			LexiconVersion: ExperienceRequirementLexiconVersion(),
			LexiconSHA256:  ExperienceRequirementLexiconDigest(),
		},
		Project: ProjectTagIdentity{
			ParserVersion:     ProjectTagParserVersion(),
			DictionaryVersion: ProjectTagDictionaryVersion(),
			DictionarySHA256:  ProjectTagDictionaryDigest(),
		},
		JobFamily: CurrentJobFamilyIdentity("trend-formal-release"),
	}
}

func trendFormalReleaseIdentityDigest(components trendFormalReleaseComponents) string {
	parts := []string{
		"technology=" + components.TechnologyConfigurationID,
		"education=" + components.Education.ParserVersion + ":" + components.Education.LexiconVersion + ":" + components.Education.LexiconSHA256,
		"experience=" + components.Experience.ParserVersion + ":" + components.Experience.LexiconVersion + ":" + components.Experience.LexiconSHA256,
		"project=" + components.Project.ParserVersion + ":" + components.Project.DictionaryVersion + ":" + components.Project.DictionarySHA256,
		"job_family=" + components.JobFamily.ParserVersion + ":" + components.JobFamily.DictionaryVersion + ":" + components.JobFamily.DictionarySHA256 + ":" +
			components.JobFamily.ProjectMaskParserVersion + ":" + components.JobFamily.ProjectMaskDictionaryVersion + ":" + components.JobFamily.ProjectMaskDictionarySHA256,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

func insertTrendFormalReleaseComponents(ctx context.Context, tx pgx.Tx, releaseID string, components trendFormalReleaseComponents) error {
	statements := []struct {
		query string
		args  []any
	}{
		{
			query: `INSERT INTO trend_formal_release_components (
    trend_formal_release_id, dimension, technology_analysis_configuration_id
) VALUES ($1, 'technology', $2)`,
			args: []any{releaseID, components.TechnologyConfigurationID},
		},
		{
			query: `INSERT INTO trend_formal_release_components (
    trend_formal_release_id, dimension, parser_version, lexicon_version, lexicon_sha256
) VALUES ($1, 'education', $2, $3, $4)`,
			args: []any{releaseID, components.Education.ParserVersion, components.Education.LexiconVersion, components.Education.LexiconSHA256},
		},
		{
			query: `INSERT INTO trend_formal_release_components (
    trend_formal_release_id, dimension, parser_version, lexicon_version, lexicon_sha256
) VALUES ($1, 'experience', $2, $3, $4)`,
			args: []any{releaseID, components.Experience.ParserVersion, components.Experience.LexiconVersion, components.Experience.LexiconSHA256},
		},
		{
			query: `INSERT INTO trend_formal_release_components (
    trend_formal_release_id, dimension, parser_version, dictionary_version, dictionary_sha256
) VALUES ($1, 'project', $2, $3, $4)`,
			args: []any{releaseID, components.Project.ParserVersion, components.Project.DictionaryVersion, components.Project.DictionarySHA256},
		},
		{
			query: `INSERT INTO trend_formal_release_components (
    trend_formal_release_id, dimension,
    parser_version, dictionary_version, dictionary_sha256,
    project_mask_parser_version, project_mask_dictionary_version, project_mask_dictionary_sha256
) VALUES ($1, 'job_family', $2, $3, $4, $5, $6, $7)`,
			args: []any{
				releaseID,
				components.JobFamily.ParserVersion,
				components.JobFamily.DictionaryVersion,
				components.JobFamily.DictionarySHA256,
				components.JobFamily.ProjectMaskParserVersion,
				components.JobFamily.ProjectMaskDictionaryVersion,
				components.JobFamily.ProjectMaskDictionarySHA256,
			},
		},
	}
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement.query, statement.args...); err != nil {
			return fmt.Errorf("append trend formal release component: %w", err)
		}
	}
	return nil
}

// PublishTrendFormalRelease advances a prepared release only when its five
// exact component identities cover every historical Serving membership that
// requires a formal body/title result. It switches releases atomically, so a
// reader can never observe a partially caught-up interpretation.
func (repository *Repository) PublishTrendFormalRelease(ctx context.Context, releaseID string) (TrendFormalRelease, error) {
	if repository == nil || repository.pool == nil || releaseID == "" {
		return TrendFormalRelease{}, fmt.Errorf("trend formal release id is required")
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return TrendFormalRelease{}, fmt.Errorf("begin trend formal release publication: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "trend-formal-release"); err != nil {
		return TrendFormalRelease{}, fmt.Errorf("lock trend formal release publication: %w", err)
	}

	release, err := loadTrendFormalReleaseForUpdate(ctx, tx, releaseID)
	if err != nil {
		return TrendFormalRelease{}, err
	}
	if release.Status == TrendFormalReleasePublished {
		if err := tx.Commit(ctx); err != nil {
			return TrendFormalRelease{}, fmt.Errorf("commit reused trend formal release publication: %w", err)
		}
		release.Reused = true
		return release, nil
	}
	if release.Status != TrendFormalReleaseCandidate {
		return TrendFormalRelease{}, fmt.Errorf("trend formal release is not publishable")
	}
	components, err := loadTrendFormalReleaseComponents(ctx, tx, releaseID)
	if err != nil {
		return TrendFormalRelease{}, err
	}
	missing, err := countUncoveredTrendFormalInputs(ctx, tx, components)
	if err != nil {
		return TrendFormalRelease{}, err
	}
	if missing != 0 {
		return TrendFormalRelease{}, ErrTrendFormalReleaseIncomplete
	}
	if _, err := tx.Exec(ctx, `
UPDATE trend_formal_releases
SET status = 'superseded', superseded_at = now()
WHERE status = 'published'`); err != nil {
		return TrendFormalRelease{}, fmt.Errorf("supersede prior trend formal release: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE trend_formal_releases
SET status = 'published', published_at = now()
WHERE id = $1`, releaseID); err != nil {
		return TrendFormalRelease{}, fmt.Errorf("publish complete trend formal release: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return TrendFormalRelease{}, fmt.Errorf("commit trend formal release publication: %w", err)
	}
	release.Status = TrendFormalReleasePublished
	return release, nil
}

// ReadPublishedTrendFormalRelease returns the one completed interpretation
// selection visible to historical trend readers. It is an analysis lifecycle
// read, not a public HTTP DTO.
func (repository *Repository) ReadPublishedTrendFormalRelease(ctx context.Context) (TrendFormalRelease, error) {
	if repository == nil || repository.pool == nil {
		return TrendFormalRelease{}, fmt.Errorf("analysis repository is unavailable")
	}
	release := TrendFormalRelease{}
	err := repository.pool.QueryRow(ctx, `
SELECT id::text, identity_sha256, status::text
FROM trend_formal_releases
WHERE status = 'published'`).Scan(&release.ID, &release.IdentitySHA256, &release.Status)
	if err == pgx.ErrNoRows {
		return TrendFormalRelease{}, ErrNoPublishedTrendFormalRelease
	}
	if err != nil {
		return TrendFormalRelease{}, fmt.Errorf("read published trend formal release: %w", err)
	}
	return release, nil
}

func loadTrendFormalReleaseForUpdate(ctx context.Context, tx pgx.Tx, releaseID string) (TrendFormalRelease, error) {
	release := TrendFormalRelease{}
	err := tx.QueryRow(ctx, `
SELECT id::text, identity_sha256, status::text
FROM trend_formal_releases
WHERE id = $1
FOR UPDATE`, releaseID).Scan(&release.ID, &release.IdentitySHA256, &release.Status)
	if err == pgx.ErrNoRows {
		return TrendFormalRelease{}, fmt.Errorf("trend formal release is unavailable")
	}
	if err != nil {
		return TrendFormalRelease{}, fmt.Errorf("lock trend formal release: %w", err)
	}
	return release, nil
}

func loadTrendFormalReleaseComponents(ctx context.Context, tx pgx.Tx, releaseID string) (trendFormalReleaseComponents, error) {
	rows, err := tx.Query(ctx, `
SELECT dimension::text,
       technology_analysis_configuration_id::text,
       parser_version, lexicon_version, lexicon_sha256,
       dictionary_version, dictionary_sha256,
       project_mask_parser_version, project_mask_dictionary_version, project_mask_dictionary_sha256
FROM trend_formal_release_components
WHERE trend_formal_release_id = $1
ORDER BY dimension`, releaseID)
	if err != nil {
		return trendFormalReleaseComponents{}, fmt.Errorf("select trend formal release components: %w", err)
	}
	defer rows.Close()

	components := trendFormalReleaseComponents{}
	seen := map[string]bool{}
	for rows.Next() {
		var dimension string
		var technologyID, parserVersion, lexiconVersion, lexiconSHA256, dictionaryVersion, dictionarySHA256 *string
		var maskParserVersion, maskDictionaryVersion, maskDictionarySHA256 *string
		if err := rows.Scan(
			&dimension,
			&technologyID,
			&parserVersion, &lexiconVersion, &lexiconSHA256,
			&dictionaryVersion, &dictionarySHA256,
			&maskParserVersion, &maskDictionaryVersion, &maskDictionarySHA256,
		); err != nil {
			return trendFormalReleaseComponents{}, fmt.Errorf("scan trend formal release component: %w", err)
		}
		seen[dimension] = true
		switch dimension {
		case "technology":
			if technologyID == nil {
				return trendFormalReleaseComponents{}, fmt.Errorf("trend technology component is incomplete")
			}
			components.TechnologyConfigurationID = *technologyID
		case "education":
			if parserVersion == nil || lexiconVersion == nil || lexiconSHA256 == nil {
				return trendFormalReleaseComponents{}, fmt.Errorf("trend education component is incomplete")
			}
			components.Education = EducationRequirementIdentity{ParserVersion: *parserVersion, LexiconVersion: *lexiconVersion, LexiconSHA256: *lexiconSHA256}
		case "experience":
			if parserVersion == nil || lexiconVersion == nil || lexiconSHA256 == nil {
				return trendFormalReleaseComponents{}, fmt.Errorf("trend experience component is incomplete")
			}
			components.Experience = ExperienceRequirementIdentity{ParserVersion: *parserVersion, LexiconVersion: *lexiconVersion, LexiconSHA256: *lexiconSHA256}
		case "project":
			if parserVersion == nil || dictionaryVersion == nil || dictionarySHA256 == nil {
				return trendFormalReleaseComponents{}, fmt.Errorf("trend project component is incomplete")
			}
			components.Project = ProjectTagIdentity{ParserVersion: *parserVersion, DictionaryVersion: *dictionaryVersion, DictionarySHA256: *dictionarySHA256}
		case "job_family":
			if parserVersion == nil || dictionaryVersion == nil || dictionarySHA256 == nil ||
				maskParserVersion == nil || maskDictionaryVersion == nil || maskDictionarySHA256 == nil {
				return trendFormalReleaseComponents{}, fmt.Errorf("trend job family component is incomplete")
			}
			components.JobFamily = JobFamilyIdentity{
				ParserVersion:                *parserVersion,
				DictionaryVersion:            *dictionaryVersion,
				DictionarySHA256:             *dictionarySHA256,
				ProjectMaskParserVersion:     *maskParserVersion,
				ProjectMaskDictionaryVersion: *maskDictionaryVersion,
				ProjectMaskDictionarySHA256:  *maskDictionarySHA256,
			}
		}
	}
	if err := rows.Err(); err != nil {
		return trendFormalReleaseComponents{}, fmt.Errorf("iterate trend formal release components: %w", err)
	}
	for _, dimension := range []string{"technology", "education", "experience", "project", "job_family"} {
		if !seen[dimension] {
			return trendFormalReleaseComponents{}, fmt.Errorf("trend formal release component is missing")
		}
	}
	return components, nil
}

func countUncoveredTrendFormalInputs(ctx context.Context, tx pgx.Tx, components trendFormalReleaseComponents) (int, error) {
	var uncovered int
	err := tx.QueryRow(ctx, `
SELECT
    (
        SELECT count(*)
        FROM serving_jd_inventory_memberships AS membership
        LEFT JOIN technology_analysis_results AS result
          ON result.body_content_version_id = membership.body_content_version_id
         AND result.configuration_id = $1
         AND result.status = 'complete'
         AND result.completed_at IS NOT NULL
        WHERE result.id IS NULL
    )
    + (
        SELECT count(*)
        FROM serving_jd_inventory_memberships AS membership
        LEFT JOIN education_requirement_results AS result
          ON result.body_content_version_id = membership.body_content_version_id
         AND result.parser_version = $2
         AND result.lexicon_version = $3
         AND result.lexicon_sha256 = $4
         AND result.completed_at IS NOT NULL
        WHERE result.id IS NULL
    )
    + (
        SELECT count(*)
        FROM serving_jd_inventory_memberships AS membership
        LEFT JOIN experience_requirement_results AS result
          ON result.body_content_version_id = membership.body_content_version_id
         AND result.parser_version = $5
         AND result.lexicon_version = $6
         AND result.lexicon_sha256 = $7
         AND result.completed_at IS NOT NULL
        WHERE result.id IS NULL
    )
    + (
        SELECT count(*)
        FROM serving_jd_inventory_memberships AS membership
        JOIN job_source_metadata_versions AS metadata
          ON metadata.id = membership.source_metadata_version_id
        LEFT JOIN project_title_observations AS observation
          ON observation.source_metadata_version_id = metadata.id
        LEFT JOIN project_tag_results AS result
          ON result.project_title_identity_id = observation.project_title_identity_id
         AND result.parser_version = $8
         AND result.dictionary_version = $9
         AND result.dictionary_sha256 = $10
         AND result.completed_at IS NOT NULL
        WHERE metadata.metadata_payload #>> '{title,state}' = 'string'
          AND result.id IS NULL
    )
    + (
        SELECT count(*)
        FROM serving_jd_inventory_memberships AS membership
        JOIN job_source_metadata_versions AS metadata
          ON metadata.id = membership.source_metadata_version_id
        LEFT JOIN job_family_observations AS observation
          ON observation.source_metadata_version_id = metadata.id
        LEFT JOIN job_family_results AS result
          ON result.job_family_title_identity_id = observation.job_family_title_identity_id
         AND result.parser_version = $11
         AND result.dictionary_version = $12
         AND result.dictionary_sha256 = $13
         AND result.project_mask_parser_version = $14
         AND result.project_mask_dictionary_version = $15
         AND result.project_mask_dictionary_sha256 = $16
         AND result.completed_at IS NOT NULL
        WHERE metadata.metadata_payload #>> '{title,state}' = 'string'
          AND (observation.status IS NULL OR observation.status = 'in_scope')
          AND result.id IS NULL
    )`,
		components.TechnologyConfigurationID,
		components.Education.ParserVersion, components.Education.LexiconVersion, components.Education.LexiconSHA256,
		components.Experience.ParserVersion, components.Experience.LexiconVersion, components.Experience.LexiconSHA256,
		components.Project.ParserVersion, components.Project.DictionaryVersion, components.Project.DictionarySHA256,
		components.JobFamily.ParserVersion, components.JobFamily.DictionaryVersion, components.JobFamily.DictionarySHA256,
		components.JobFamily.ProjectMaskParserVersion, components.JobFamily.ProjectMaskDictionaryVersion, components.JobFamily.ProjectMaskDictionarySHA256,
	).Scan(&uncovered)
	if err != nil {
		return 0, fmt.Errorf("count uncovered trend formal inputs: %w", err)
	}
	return uncovered, nil
}
