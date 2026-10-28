package jdanalysis

import (
	"context"
	"fmt"
)

// EducationRequirementProcessingSummary contains safe aggregates only. It
// intentionally has no source text, model configuration, provider, or
// credential field because deterministic education processing needs none.
type EducationRequirementProcessingSummary struct {
	Processed int
	Reused    int
}

// ProcessPendingEducationRequirements catches up every normalized body
// version that lacks the current parser/lexicon identity. Repeated calls are
// idempotent: the immutable result identity is the durable work selection.
func ProcessPendingEducationRequirements(ctx context.Context, repository *Repository) (EducationRequirementProcessingSummary, error) {
	identity := EducationRequirementIdentity{
		ParserVersion:  EducationRequirementParserVersion(),
		LexiconVersion: EducationRequirementLexiconVersion(),
		LexiconSHA256:  EducationRequirementLexiconDigest(),
	}
	bodyContentVersionIDs, err := repository.pendingEducationRequirementBodyContentVersions(ctx, identity)
	if err != nil {
		return EducationRequirementProcessingSummary{}, err
	}
	summary := EducationRequirementProcessingSummary{}
	for _, bodyContentVersionID := range bodyContentVersionIDs {
		input, err := repository.loadEducationParserInput(ctx, bodyContentVersionID)
		if err != nil {
			return EducationRequirementProcessingSummary{}, err
		}
		requirement, err := ParseEducationRequirement(input)
		if err != nil {
			return EducationRequirementProcessingSummary{}, fmt.Errorf("parse deterministic education requirement: %w", err)
		}
		identity.BodyContentVersionID = bodyContentVersionID
		published, err := repository.PublishEducationRequirement(ctx, identity, requirement)
		if err != nil {
			return EducationRequirementProcessingSummary{}, err
		}
		if published.Reused {
			summary.Reused++
			continue
		}
		summary.Processed++
	}
	return summary, nil
}

func (repository *Repository) pendingEducationRequirementBodyContentVersions(ctx context.Context, identity EducationRequirementIdentity) ([]string, error) {
	rows, err := repository.pool.Query(ctx, `
SELECT body.id::text
FROM job_body_content_versions AS body
WHERE NOT EXISTS (
    SELECT 1
    FROM education_requirement_results AS result
    WHERE result.body_content_version_id = body.id
      AND result.parser_version = $1
      AND result.lexicon_version = $2
      AND result.lexicon_sha256 = $3
)
ORDER BY body.created_at, body.id`, identity.ParserVersion, identity.LexiconVersion, identity.LexiconSHA256)
	if err != nil {
		return nil, fmt.Errorf("select pending deterministic education bodies: %w", err)
	}
	defer rows.Close()
	bodyContentVersionIDs := make([]string, 0)
	for rows.Next() {
		var bodyContentVersionID string
		if err := rows.Scan(&bodyContentVersionID); err != nil {
			return nil, fmt.Errorf("scan pending deterministic education body: %w", err)
		}
		bodyContentVersionIDs = append(bodyContentVersionIDs, bodyContentVersionID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending deterministic education bodies: %w", err)
	}
	return bodyContentVersionIDs, nil
}

func (repository *Repository) loadEducationParserInput(ctx context.Context, bodyContentVersionID string) (EducationParserInput, error) {
	input := EducationParserInput{}
	if err := repository.pool.QueryRow(ctx, `
SELECT description_text, job_require_text, addition_text
FROM job_body_content_versions
WHERE id = $1`, bodyContentVersionID).Scan(&input.Description, &input.JobRequire, &input.Addition); err != nil {
		return EducationParserInput{}, fmt.Errorf("load deterministic education parser input: %w", err)
	}
	return input, nil
}
