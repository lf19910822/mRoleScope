package jdanalysis

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// EducationRequirementIdentity identifies one immutable deterministic
// interpretation of exactly one normalized JD body version.
type EducationRequirementIdentity struct {
	BodyContentVersionID string
	ParserVersion        string
	LexiconVersion       string
	LexiconSHA256        string
}

type EducationRequirementPublishResult struct {
	ResultID string
	Reused   bool
}

type StoredEducationRequirement struct {
	ID          string
	Identity    EducationRequirementIdentity
	Requirement EducationRequirement
}

// PublishEducationRequirement persists one fully computed deterministic
// result. It does not query or mutate any AI result, artifact, or audit table.
func (repository *Repository) PublishEducationRequirement(ctx context.Context, identity EducationRequirementIdentity, requirement EducationRequirement) (EducationRequirementPublishResult, error) {
	if err := validateEducationRequirementIdentity(identity); err != nil {
		return EducationRequirementPublishResult{}, err
	}
	if err := validateEducationRequirement(requirement); err != nil {
		return EducationRequirementPublishResult{}, err
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return EducationRequirementPublishResult{}, fmt.Errorf("begin deterministic education publication: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	body, err := loadBodyContent(ctx, tx, identity.BodyContentVersionID)
	if err != nil {
		return EducationRequirementPublishResult{}, err
	}
	resultID, err := insertEducationRequirementResult(ctx, tx, identity, requirement)
	if errors.Is(err, pgx.ErrNoRows) {
		var existingID string
		if err := tx.QueryRow(ctx, `
SELECT id::text
FROM education_requirement_results
WHERE body_content_version_id = $1
  AND parser_version = $2
  AND lexicon_version = $3
  AND lexicon_sha256 = $4`, identity.BodyContentVersionID, identity.ParserVersion, identity.LexiconVersion, identity.LexiconSHA256).Scan(&existingID); err != nil {
			return EducationRequirementPublishResult{}, fmt.Errorf("load existing deterministic education result: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return EducationRequirementPublishResult{}, fmt.Errorf("commit reused deterministic education publication: %w", err)
		}
		return EducationRequirementPublishResult{ResultID: existingID, Reused: true}, nil
	}
	if err != nil {
		return EducationRequirementPublishResult{}, err
	}
	for _, evidence := range requirement.Evidence {
		if err := insertEducationRequirementEvidence(ctx, tx, resultID, body, evidence); err != nil {
			return EducationRequirementPublishResult{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return EducationRequirementPublishResult{}, fmt.Errorf("commit deterministic education publication: %w", err)
	}
	return EducationRequirementPublishResult{ResultID: resultID}, nil
}

func validateEducationRequirementIdentity(identity EducationRequirementIdentity) error {
	if identity.BodyContentVersionID == "" || identity.ParserVersion == "" || identity.LexiconVersion == "" || len(identity.LexiconSHA256) != 64 {
		return fmt.Errorf("deterministic education result identity is incomplete")
	}
	for _, character := range identity.LexiconSHA256 {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
			return fmt.Errorf("deterministic education lexicon digest is invalid")
		}
	}
	return nil
}

func validateEducationRequirement(requirement EducationRequirement) error {
	if requirement.Status != EducationRequirementConfigured && requirement.Status != EducationRequirementNotConfigured && requirement.Status != EducationRequirementUncertain {
		return fmt.Errorf("deterministic education result status is invalid")
	}
	if !validOptionalEducationLevel(requirement.LowerBoundLevel) || !validOptionalEducationLevel(requirement.UpperBoundLevel) {
		return fmt.Errorf("deterministic education result level is invalid")
	}
	if requirement.Status == EducationRequirementConfigured && requirement.LowerBoundLevel == "" && requirement.UpperBoundLevel == "" {
		return fmt.Errorf("configured deterministic education result has no bound")
	}
	if requirement.Status != EducationRequirementConfigured && (requirement.LowerBoundLevel != "" || requirement.UpperBoundLevel != "") {
		return fmt.Errorf("non-configured deterministic education result has a bound")
	}
	for _, evidence := range requirement.Evidence {
		if !AllowedSourceField(evidence.SourceField) || evidence.RuleKey == "" || evidence.MatchedAlias == "" || evidence.CodePointRange.Start < 0 || evidence.CodePointRange.End <= evidence.CodePointRange.Start || !validEducationRequirementModality(evidence.Modality) {
			return fmt.Errorf("deterministic education evidence is invalid")
		}
	}
	return nil
}

func validOptionalEducationLevel(level EducationLevel) bool {
	return level == "" || validEducationLevel(level)
}

func validEducationRequirementModality(modality EducationRequirementModality) bool {
	return modality == EducationRequirementModalityRequired || modality == EducationRequirementModalityPreferred || modality == EducationRequirementModalityUnrestricted
}

func insertEducationRequirementResult(ctx context.Context, tx pgx.Tx, identity EducationRequirementIdentity, requirement EducationRequirement) (string, error) {
	var resultID string
	err := tx.QueryRow(ctx, `
INSERT INTO education_requirement_results (
    body_content_version_id, parser_version, lexicon_version, lexicon_sha256,
    status, lower_bound_level, upper_bound_level
) VALUES ($1, $2, $3, $4, $5::education_requirement_result_status,
          NULLIF($6, '')::deterministic_education_level,
          NULLIF($7, '')::deterministic_education_level)
ON CONFLICT (body_content_version_id, parser_version, lexicon_version, lexicon_sha256) DO NOTHING
RETURNING id::text`,
		identity.BodyContentVersionID, identity.ParserVersion, identity.LexiconVersion, identity.LexiconSHA256,
		requirement.Status, requirement.LowerBoundLevel, requirement.UpperBoundLevel).Scan(&resultID)
	if err != nil && err != pgx.ErrNoRows {
		return "", fmt.Errorf("insert deterministic education result: %w", err)
	}
	return resultID, err
}

func insertEducationRequirementEvidence(ctx context.Context, tx pgx.Tx, resultID string, body bodyContent, evidence EducationRequirementEvidence) error {
	text, fieldSHA, ok := body.field(evidence.SourceField)
	if !ok {
		return fmt.Errorf("deterministic education evidence source is outside approved JD body scope")
	}
	matched, err := CodePointSlice(text, evidence.CodePointRange)
	if err != nil || matched != evidence.MatchedAlias {
		return fmt.Errorf("deterministic education evidence does not match its normalized field")
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO education_requirement_evidence (
    education_requirement_result_id, rule_key, modality, source_field,
    normalization_version, normalized_field_sha256, codepoint_start, codepoint_end
) VALUES ($1, $2, $3::education_requirement_modality, $4, $5, $6, $7, $8)`,
		resultID, evidence.RuleKey, evidence.Modality, evidence.SourceField,
		body.normalizationVersion, fieldSHA, evidence.CodePointRange.Start, evidence.CodePointRange.End); err != nil {
		return fmt.Errorf("insert deterministic education evidence: %w", err)
	}
	return nil
}

// ReadDeterministicEducationRequirement reads one exact immutable result
// identity. Its evidence aliases are replayed from the original normalized
// body version; they are deliberately not duplicated in PostgreSQL.
func (repository *Repository) ReadDeterministicEducationRequirement(ctx context.Context, identity EducationRequirementIdentity) (StoredEducationRequirement, error) {
	if err := validateEducationRequirementIdentity(identity); err != nil {
		return StoredEducationRequirement{}, err
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return StoredEducationRequirement{}, fmt.Errorf("begin deterministic education read: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	stored, err := readDeterministicEducationRequirement(ctx, tx, identity)
	if err != nil {
		return StoredEducationRequirement{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return StoredEducationRequirement{}, fmt.Errorf("commit deterministic education read: %w", err)
	}
	return stored, nil
}

func readDeterministicEducationRequirement(ctx context.Context, tx pgx.Tx, identity EducationRequirementIdentity) (StoredEducationRequirement, error) {
	stored := StoredEducationRequirement{Identity: identity}
	var lowerBound, upperBound *string
	if err := tx.QueryRow(ctx, `
SELECT id::text, status::text, lower_bound_level::text, upper_bound_level::text
FROM education_requirement_results
WHERE body_content_version_id = $1
  AND parser_version = $2
  AND lexicon_version = $3
  AND lexicon_sha256 = $4`, identity.BodyContentVersionID, identity.ParserVersion, identity.LexiconVersion, identity.LexiconSHA256).Scan(
		&stored.ID, &stored.Requirement.Status, &lowerBound, &upperBound); err != nil {
		return StoredEducationRequirement{}, fmt.Errorf("load deterministic education result: %w", err)
	}
	if lowerBound != nil {
		stored.Requirement.LowerBoundLevel = EducationLevel(*lowerBound)
	}
	if upperBound != nil {
		stored.Requirement.UpperBoundLevel = EducationLevel(*upperBound)
	}
	body, err := loadBodyContent(ctx, tx, identity.BodyContentVersionID)
	if err != nil {
		return StoredEducationRequirement{}, err
	}
	rows, err := tx.Query(ctx, `
SELECT rule_key, modality::text, source_field, codepoint_start, codepoint_end
FROM education_requirement_evidence
WHERE education_requirement_result_id = $1
ORDER BY CASE source_field
    WHEN 'jobRequire' THEN 1
    WHEN 'description' THEN 2
    WHEN 'addition' THEN 3
    ELSE 4
END, codepoint_start, codepoint_end, rule_key`, stored.ID)
	if err != nil {
		return StoredEducationRequirement{}, fmt.Errorf("load deterministic education evidence: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var evidence EducationRequirementEvidence
		if err := rows.Scan(&evidence.RuleKey, &evidence.Modality, &evidence.SourceField, &evidence.CodePointRange.Start, &evidence.CodePointRange.End); err != nil {
			return StoredEducationRequirement{}, fmt.Errorf("scan deterministic education evidence: %w", err)
		}
		fieldText, _, ok := body.field(evidence.SourceField)
		if !ok {
			return StoredEducationRequirement{}, fmt.Errorf("deterministic education evidence has invalid source field")
		}
		matched, err := CodePointSlice(fieldText, evidence.CodePointRange)
		if err != nil {
			return StoredEducationRequirement{}, fmt.Errorf("replay deterministic education evidence: %w", err)
		}
		evidence.MatchedAlias = matched
		stored.Requirement.Evidence = append(stored.Requirement.Evidence, evidence)
	}
	if err := rows.Err(); err != nil {
		return StoredEducationRequirement{}, fmt.Errorf("iterate deterministic education evidence: %w", err)
	}
	return stored, nil
}

// ReadFormalEducationRequirement uses only the current deterministic parser
// and lexicon identity. Missing deterministic processing is reported as
// unavailable; it never falls back to an AI education projection.
func (repository *Repository) ReadFormalEducationRequirement(ctx context.Context, bodyContentVersionID string) (EducationRequirement, bool, error) {
	stored, err := repository.ReadDeterministicEducationRequirement(ctx, EducationRequirementIdentity{
		BodyContentVersionID: bodyContentVersionID,
		ParserVersion:        EducationRequirementParserVersion(),
		LexiconVersion:       EducationRequirementLexiconVersion(),
		LexiconSHA256:        EducationRequirementLexiconDigest(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return EducationRequirement{}, false, nil
	}
	if err != nil {
		return EducationRequirement{}, false, err
	}
	return stored.Requirement, true, nil
}
