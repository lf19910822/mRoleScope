package jdanalysis

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ExperienceRequirementIdentity identifies one immutable deterministic
// interpretation of exactly one normalized JD body version.
type ExperienceRequirementIdentity struct {
	BodyContentVersionID string
	ParserVersion        string
	LexiconVersion       string
	LexiconSHA256        string
}

type ExperienceRequirementPublishResult struct {
	ResultID string
	Reused   bool
}

type StoredExperienceRequirement struct {
	ID          string
	Identity    ExperienceRequirementIdentity
	Requirement ExperienceRequirement
}

// PublishExperienceRequirement persists a fully computed deterministic result.
// It is independent of every AI projection, configuration, and audit table.
func (repository *Repository) PublishExperienceRequirement(ctx context.Context, identity ExperienceRequirementIdentity, requirement ExperienceRequirement) (ExperienceRequirementPublishResult, error) {
	if err := validateExperienceRequirementIdentity(identity); err != nil {
		return ExperienceRequirementPublishResult{}, err
	}
	if err := validateExperienceRequirement(requirement); err != nil {
		return ExperienceRequirementPublishResult{}, err
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ExperienceRequirementPublishResult{}, fmt.Errorf("begin deterministic experience publication: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	body, err := loadBodyContent(ctx, tx, identity.BodyContentVersionID)
	if err != nil {
		return ExperienceRequirementPublishResult{}, err
	}
	resultID, err := insertExperienceRequirementResult(ctx, tx, identity, requirement)
	if errors.Is(err, pgx.ErrNoRows) {
		var existingID string
		if err := tx.QueryRow(ctx, `
SELECT id::text
FROM experience_requirement_results
WHERE body_content_version_id = $1
  AND parser_version = $2
  AND lexicon_version = $3
  AND lexicon_sha256 = $4`, identity.BodyContentVersionID, identity.ParserVersion, identity.LexiconVersion, identity.LexiconSHA256).Scan(&existingID); err != nil {
			return ExperienceRequirementPublishResult{}, fmt.Errorf("load existing deterministic experience result: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return ExperienceRequirementPublishResult{}, fmt.Errorf("commit reused deterministic experience publication: %w", err)
		}
		return ExperienceRequirementPublishResult{ResultID: existingID, Reused: true}, nil
	}
	if err != nil {
		return ExperienceRequirementPublishResult{}, err
	}
	for _, evidence := range requirement.Evidence {
		if err := insertExperienceRequirementEvidence(ctx, tx, resultID, body, evidence); err != nil {
			return ExperienceRequirementPublishResult{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return ExperienceRequirementPublishResult{}, fmt.Errorf("commit deterministic experience publication: %w", err)
	}
	return ExperienceRequirementPublishResult{ResultID: resultID}, nil
}

func validateExperienceRequirementIdentity(identity ExperienceRequirementIdentity) error {
	if identity.BodyContentVersionID == "" || identity.ParserVersion == "" || identity.LexiconVersion == "" || len(identity.LexiconSHA256) != 64 {
		return fmt.Errorf("deterministic experience result identity is incomplete")
	}
	for _, character := range identity.LexiconSHA256 {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
			return fmt.Errorf("deterministic experience lexicon digest is invalid")
		}
	}
	return nil
}

func validateExperienceRequirement(requirement ExperienceRequirement) error {
	if requirement.Status != ExperienceRequirementConfigured && requirement.Status != ExperienceRequirementNotConfigured && requirement.Status != ExperienceRequirementUncertain {
		return fmt.Errorf("deterministic experience result status is invalid")
	}
	if requirement.Status == ExperienceRequirementConfigured && requirement.BaseYear <= 0 {
		return fmt.Errorf("configured deterministic experience result has no positive base year")
	}
	if requirement.Status != ExperienceRequirementConfigured && requirement.BaseYear != 0 {
		return fmt.Errorf("non-configured deterministic experience result has a base year")
	}
	for _, evidence := range requirement.Evidence {
		if !AllowedSourceField(evidence.SourceField) || evidence.RuleKey == "" || evidence.MatchedText == "" || evidence.MinimumYears <= 0 || evidence.CodePointRange.Start < 0 || evidence.CodePointRange.End <= evidence.CodePointRange.Start || !validExperienceRequirementModality(evidence.Modality) {
			return fmt.Errorf("deterministic experience evidence is invalid")
		}
	}
	return nil
}

func validExperienceRequirementModality(modality ExperienceRequirementModality) bool {
	return modality == ExperienceRequirementModalityRequired ||
		modality == ExperienceRequirementModalityPreferred ||
		modality == ExperienceRequirementModalityConditional ||
		modality == ExperienceRequirementModalityUncertain
}

func insertExperienceRequirementResult(ctx context.Context, tx pgx.Tx, identity ExperienceRequirementIdentity, requirement ExperienceRequirement) (string, error) {
	var resultID string
	err := tx.QueryRow(ctx, `
INSERT INTO experience_requirement_results (
    body_content_version_id, parser_version, lexicon_version, lexicon_sha256,
    status, base_year
) VALUES ($1, $2, $3, $4, $5::experience_requirement_result_status, NULLIF($6, 0))
ON CONFLICT (body_content_version_id, parser_version, lexicon_version, lexicon_sha256) DO NOTHING
RETURNING id::text`,
		identity.BodyContentVersionID, identity.ParserVersion, identity.LexiconVersion, identity.LexiconSHA256,
		requirement.Status, requirement.BaseYear).Scan(&resultID)
	if err != nil && err != pgx.ErrNoRows {
		return "", fmt.Errorf("insert deterministic experience result: %w", err)
	}
	return resultID, err
}

func insertExperienceRequirementEvidence(ctx context.Context, tx pgx.Tx, resultID string, body bodyContent, evidence ExperienceRequirementEvidence) error {
	text, fieldSHA, ok := body.field(evidence.SourceField)
	if !ok {
		return fmt.Errorf("deterministic experience evidence source is outside approved JD body scope")
	}
	matched, err := CodePointSlice(text, evidence.CodePointRange)
	if err != nil || matched != evidence.MatchedText {
		return fmt.Errorf("deterministic experience evidence does not match its normalized field")
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO experience_requirement_evidence (
    experience_requirement_result_id, rule_key, modality, minimum_years,
    source_field, normalization_version, normalized_field_sha256,
    codepoint_start, codepoint_end
) VALUES ($1, $2, $3::experience_requirement_modality, $4, $5, $6, $7, $8, $9)`,
		resultID, evidence.RuleKey, evidence.Modality, evidence.MinimumYears,
		evidence.SourceField, body.normalizationVersion, fieldSHA,
		evidence.CodePointRange.Start, evidence.CodePointRange.End); err != nil {
		return fmt.Errorf("insert deterministic experience evidence: %w", err)
	}
	return nil
}

// ReadDeterministicExperienceRequirement reads one exact immutable result
// identity and replays its evidence from the original normalized body text.
func (repository *Repository) ReadDeterministicExperienceRequirement(ctx context.Context, identity ExperienceRequirementIdentity) (StoredExperienceRequirement, error) {
	if err := validateExperienceRequirementIdentity(identity); err != nil {
		return StoredExperienceRequirement{}, err
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return StoredExperienceRequirement{}, fmt.Errorf("begin deterministic experience read: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	stored, err := readDeterministicExperienceRequirement(ctx, tx, identity)
	if err != nil {
		return StoredExperienceRequirement{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return StoredExperienceRequirement{}, fmt.Errorf("commit deterministic experience read: %w", err)
	}
	return stored, nil
}

func readDeterministicExperienceRequirement(ctx context.Context, tx pgx.Tx, identity ExperienceRequirementIdentity) (StoredExperienceRequirement, error) {
	stored := StoredExperienceRequirement{Identity: identity}
	var baseYear *int
	if err := tx.QueryRow(ctx, `
SELECT id::text, status::text, base_year
FROM experience_requirement_results
WHERE body_content_version_id = $1
  AND parser_version = $2
  AND lexicon_version = $3
  AND lexicon_sha256 = $4`, identity.BodyContentVersionID, identity.ParserVersion, identity.LexiconVersion, identity.LexiconSHA256).Scan(
		&stored.ID, &stored.Requirement.Status, &baseYear); err != nil {
		return StoredExperienceRequirement{}, fmt.Errorf("load deterministic experience result: %w", err)
	}
	if baseYear != nil {
		stored.Requirement.BaseYear = *baseYear
	}
	body, err := loadBodyContent(ctx, tx, identity.BodyContentVersionID)
	if err != nil {
		return StoredExperienceRequirement{}, err
	}
	rows, err := tx.Query(ctx, `
SELECT rule_key, modality::text, minimum_years, source_field, codepoint_start, codepoint_end
FROM experience_requirement_evidence
WHERE experience_requirement_result_id = $1
ORDER BY CASE source_field
    WHEN 'jobRequire' THEN 1
    WHEN 'description' THEN 2
    WHEN 'addition' THEN 3
    ELSE 4
END, codepoint_start, codepoint_end, rule_key`, stored.ID)
	if err != nil {
		return StoredExperienceRequirement{}, fmt.Errorf("load deterministic experience evidence: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var evidence ExperienceRequirementEvidence
		if err := rows.Scan(&evidence.RuleKey, &evidence.Modality, &evidence.MinimumYears, &evidence.SourceField, &evidence.CodePointRange.Start, &evidence.CodePointRange.End); err != nil {
			return StoredExperienceRequirement{}, fmt.Errorf("scan deterministic experience evidence: %w", err)
		}
		fieldText, _, ok := body.field(evidence.SourceField)
		if !ok {
			return StoredExperienceRequirement{}, fmt.Errorf("deterministic experience evidence has invalid source field")
		}
		matched, err := CodePointSlice(fieldText, evidence.CodePointRange)
		if err != nil {
			return StoredExperienceRequirement{}, fmt.Errorf("replay deterministic experience evidence: %w", err)
		}
		evidence.MatchedText = matched
		stored.Requirement.Evidence = append(stored.Requirement.Evidence, evidence)
	}
	if err := rows.Err(); err != nil {
		return StoredExperienceRequirement{}, fmt.Errorf("iterate deterministic experience evidence: %w", err)
	}
	return stored, nil
}

// ReadFormalExperienceRequirement reads only the current deterministic parser
// and lexicon identity. Missing deterministic processing is unavailable rather
// than an opportunity to substitute an AI semantic projection.
func (repository *Repository) ReadFormalExperienceRequirement(ctx context.Context, bodyContentVersionID string) (ExperienceRequirement, bool, error) {
	stored, err := repository.ReadDeterministicExperienceRequirement(ctx, ExperienceRequirementIdentity{
		BodyContentVersionID: bodyContentVersionID,
		ParserVersion:        ExperienceRequirementParserVersion(),
		LexiconVersion:       ExperienceRequirementLexiconVersion(),
		LexiconSHA256:        ExperienceRequirementLexiconDigest(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ExperienceRequirement{}, false, nil
	}
	if err != nil {
		return ExperienceRequirement{}, false, err
	}
	return stored.Requirement, true, nil
}
