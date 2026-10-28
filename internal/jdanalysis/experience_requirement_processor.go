package jdanalysis

import (
	"context"
	"fmt"
)

// ExperienceRequirementProcessingSummary contains safe counts only. It never
// includes JD text, evidence snippets, model data, provider data, or secrets.
type ExperienceRequirementProcessingSummary struct {
	Processed     int
	Reused        int
	Configured    int
	NotConfigured int
	Uncertain     int
}

// ProcessPendingExperienceRequirements catches up every normalized body
// version missing the current deterministic parser/lexicon identity. It has
// no provider dependency and is idempotent across unchanged repeated runs.
func ProcessPendingExperienceRequirements(ctx context.Context, repository *Repository) (ExperienceRequirementProcessingSummary, error) {
	identity := ExperienceRequirementIdentity{
		ParserVersion:  ExperienceRequirementParserVersion(),
		LexiconVersion: ExperienceRequirementLexiconVersion(),
		LexiconSHA256:  ExperienceRequirementLexiconDigest(),
	}
	bodyContentVersionIDs, err := repository.pendingExperienceRequirementBodyContentVersions(ctx, identity)
	if err != nil {
		return ExperienceRequirementProcessingSummary{}, err
	}
	summary := ExperienceRequirementProcessingSummary{}
	for _, bodyContentVersionID := range bodyContentVersionIDs {
		input, err := repository.loadExperienceParserInput(ctx, bodyContentVersionID)
		if err != nil {
			return ExperienceRequirementProcessingSummary{}, err
		}
		requirement, err := ParseExperienceRequirement(input)
		if err != nil {
			return ExperienceRequirementProcessingSummary{}, fmt.Errorf("parse deterministic experience requirement: %w", err)
		}
		identity.BodyContentVersionID = bodyContentVersionID
		published, err := repository.PublishExperienceRequirement(ctx, identity, requirement)
		if err != nil {
			return ExperienceRequirementProcessingSummary{}, err
		}
		if published.Reused {
			summary.Reused++
			continue
		}
		summary.Processed++
		switch requirement.Status {
		case ExperienceRequirementConfigured:
			summary.Configured++
		case ExperienceRequirementNotConfigured:
			summary.NotConfigured++
		case ExperienceRequirementUncertain:
			summary.Uncertain++
		}
	}
	return summary, nil
}

func (repository *Repository) pendingExperienceRequirementBodyContentVersions(ctx context.Context, identity ExperienceRequirementIdentity) ([]string, error) {
	rows, err := repository.pool.Query(ctx, `
SELECT body.id::text
FROM job_body_content_versions AS body
WHERE NOT EXISTS (
    SELECT 1
    FROM experience_requirement_results AS result
    WHERE result.body_content_version_id = body.id
      AND result.parser_version = $1
      AND result.lexicon_version = $2
      AND result.lexicon_sha256 = $3
)
ORDER BY body.created_at, body.id`, identity.ParserVersion, identity.LexiconVersion, identity.LexiconSHA256)
	if err != nil {
		return nil, fmt.Errorf("select pending deterministic experience bodies: %w", err)
	}
	defer rows.Close()
	bodyContentVersionIDs := make([]string, 0)
	for rows.Next() {
		var bodyContentVersionID string
		if err := rows.Scan(&bodyContentVersionID); err != nil {
			return nil, fmt.Errorf("scan pending deterministic experience body: %w", err)
		}
		bodyContentVersionIDs = append(bodyContentVersionIDs, bodyContentVersionID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending deterministic experience bodies: %w", err)
	}
	return bodyContentVersionIDs, nil
}

func (repository *Repository) loadExperienceParserInput(ctx context.Context, bodyContentVersionID string) (ExperienceParserInput, error) {
	input := ExperienceParserInput{}
	if err := repository.pool.QueryRow(ctx, `
SELECT description_text, job_require_text, addition_text
FROM job_body_content_versions
WHERE id = $1`, bodyContentVersionID).Scan(&input.Description, &input.JobRequire, &input.Addition); err != nil {
		return ExperienceParserInput{}, fmt.Errorf("load deterministic experience parser input: %w", err)
	}
	return input, nil
}
