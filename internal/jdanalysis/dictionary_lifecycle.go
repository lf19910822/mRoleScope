package jdanalysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

const V01ParserVersion = "technology-mentions-v1"

// DictionaryImportSummary deliberately contains no rule aliases or source
// text. It is safe for the controlled remote worker to log.
type DictionaryImportSummary struct {
	DictionaryVersionID string
	RulesCreated        int
	CandidatesCreated   int
	Reused              bool
}

// DictionaryApprovalSummary describes one explicit, durable human approval.
// The caller must supply an authenticated operator identity; free-form
// rationale is intentionally not part of the remote operation interface.
type DictionaryApprovalSummary struct {
	RuleKey string
	Reused  bool
}

// ImportCandidateDictionary persists a reviewed dictionary document exactly as
// proposed: candidate and conditional rules remain non-executable. The seed's
// serialized hash prevents a version label from being silently reused for a
// different mapping.
func (repository *Repository) ImportCandidateDictionary(ctx context.Context, seed DictionarySeed, createdBy string) (DictionaryImportSummary, error) {
	if err := seed.Validate(); err != nil {
		return DictionaryImportSummary{}, fmt.Errorf("validate candidate dictionary seed: %w", err)
	}
	if !validOperator(createdBy) {
		return DictionaryImportSummary{}, fmt.Errorf("candidate dictionary operator is invalid")
	}
	for _, rule := range seed.Rules {
		if rule.State == RuleEnabled {
			return DictionaryImportSummary{}, fmt.Errorf("candidate dictionary seed includes an enabled rule")
		}
	}
	hash, err := dictionarySeedSHA256(seed)
	if err != nil {
		return DictionaryImportSummary{}, err
	}

	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return DictionaryImportSummary{}, fmt.Errorf("begin candidate dictionary import: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	versionID, reused, err := importDictionaryVersion(ctx, tx, seed, hash, createdBy)
	if err != nil {
		return DictionaryImportSummary{}, err
	}
	identityIDs := make(map[string]string, len(seed.Rules))
	for _, rule := range seed.Rules {
		identityID, err := upsertTechnologyIdentity(ctx, tx, rule)
		if err != nil {
			return DictionaryImportSummary{}, err
		}
		identityIDs[rule.TechnologyKey] = identityID
	}

	summary := DictionaryImportSummary{DictionaryVersionID: versionID, Reused: reused}
	for _, rule := range seed.Rules {
		command, err := tx.Exec(ctx, `
INSERT INTO technology_dictionary_rules (
    dictionary_version_id, technology_identity_id, rule_key, alias, state,
    case_mode, boundary_mode, overlap_priority, rule_summary
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (dictionary_version_id, rule_key) DO NOTHING`,
			versionID, identityIDs[rule.TechnologyKey], rule.RuleKey, rule.Alias, rule.State,
			rule.CaseMode, rule.BoundaryMode, rule.OverlapPriority, rule.Summary)
		if err != nil {
			return DictionaryImportSummary{}, fmt.Errorf("insert candidate dictionary rule: %w", err)
		}
		summary.RulesCreated += int(command.RowsAffected())
	}
	for _, candidate := range seed.Candidates {
		var proposedIdentityID *string
		if candidate.ProposedTechnologyKey != "" {
			identityID, exists := identityIDs[candidate.ProposedTechnologyKey]
			if !exists {
				return DictionaryImportSummary{}, fmt.Errorf("candidate references an unknown technology identity")
			}
			proposedIdentityID = &identityID
		}
		command, err := tx.Exec(ctx, `
INSERT INTO technology_dictionary_candidates (
    dictionary_version_id, candidate_text, proposed_technology_identity_id, state, candidate_summary
) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (dictionary_version_id, candidate_text) DO NOTHING`,
			versionID, candidate.Text, proposedIdentityID, candidate.State, candidate.Summary)
		if err != nil {
			return DictionaryImportSummary{}, fmt.Errorf("insert dictionary review candidate: %w", err)
		}
		summary.CandidatesCreated += int(command.RowsAffected())
	}
	if err := tx.Commit(ctx); err != nil {
		return DictionaryImportSummary{}, fmt.Errorf("commit candidate dictionary import: %w", err)
	}
	return summary, nil
}

func importDictionaryVersion(ctx context.Context, tx pgx.Tx, seed DictionarySeed, hash, createdBy string) (string, bool, error) {
	var versionID, existingHash string
	err := tx.QueryRow(ctx, `
INSERT INTO technology_dictionary_versions (
    version_label, content_sha256, status, source_document, created_by
) VALUES ($1, $2, 'candidate', $3, $4)
ON CONFLICT (version_label) DO NOTHING
RETURNING id::text, content_sha256`, seed.VersionLabel, hash, seed.SourceDocument, createdBy).Scan(&versionID, &existingHash)
	if err == nil {
		return versionID, false, nil
	}
	if err != pgx.ErrNoRows {
		return "", false, fmt.Errorf("create candidate dictionary version: %w", err)
	}
	if err := tx.QueryRow(ctx, `
SELECT id::text, content_sha256
FROM technology_dictionary_versions
WHERE version_label = $1`, seed.VersionLabel).Scan(&versionID, &existingHash); err != nil {
		return "", false, fmt.Errorf("find existing candidate dictionary version: %w", err)
	}
	if existingHash != hash {
		return "", false, fmt.Errorf("candidate dictionary version label conflicts with different content")
	}
	return versionID, true, nil
}

func upsertTechnologyIdentity(ctx context.Context, tx pgx.Tx, rule DictionaryRule) (string, error) {
	var identityID string
	err := tx.QueryRow(ctx, `
INSERT INTO technology_identities (technology_key, canonical_name, technology_kind)
VALUES ($1, $2, $3)
ON CONFLICT (technology_key)
DO UPDATE SET technology_key = EXCLUDED.technology_key
RETURNING id::text`, rule.TechnologyKey, rule.CanonicalName, rule.TechnologyKind).Scan(&identityID)
	if err != nil {
		return "", fmt.Errorf("upsert technology identity: %w", err)
	}
	return identityID, nil
}

// ApproveCandidateRule moves one deterministic candidate rule to enabled and
// records the immutable decision. Conditional rules must be handled by a
// later context-aware policy; they can never be promoted through this path.
func (repository *Repository) ApproveCandidateRule(ctx context.Context, dictionaryVersionLabel, ruleKey, reviewer string) (DictionaryApprovalSummary, error) {
	if dictionaryVersionLabel == "" || !validRuleKey(ruleKey) || !validOperator(reviewer) {
		return DictionaryApprovalSummary{}, fmt.Errorf("dictionary approval input is invalid")
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return DictionaryApprovalSummary{}, fmt.Errorf("begin dictionary approval: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var ruleID, state string
	err = tx.QueryRow(ctx, `
SELECT rule.id::text, rule.state::text
FROM technology_dictionary_rules AS rule
JOIN technology_dictionary_versions AS version ON version.id = rule.dictionary_version_id
WHERE version.version_label = $1 AND rule.rule_key = $2
FOR UPDATE`, dictionaryVersionLabel, ruleKey).Scan(&ruleID, &state)
	if err == pgx.ErrNoRows {
		return DictionaryApprovalSummary{}, fmt.Errorf("dictionary rule is unavailable for approval")
	}
	if err != nil {
		return DictionaryApprovalSummary{}, fmt.Errorf("lock dictionary rule for approval: %w", err)
	}
	summary := DictionaryApprovalSummary{RuleKey: ruleKey}
	if state == string(RuleEnabled) {
		summary.Reused = true
		if err := tx.Commit(ctx); err != nil {
			return DictionaryApprovalSummary{}, fmt.Errorf("commit reused dictionary approval: %w", err)
		}
		return summary, nil
	}
	if state != string(RuleCandidate) {
		return DictionaryApprovalSummary{}, fmt.Errorf("dictionary rule requires a context-aware policy")
	}
	if _, err := tx.Exec(ctx, `
UPDATE technology_dictionary_rules
SET state = 'enabled', approved_by = $2, approved_at = now()
WHERE id = $1`, ruleID, reviewer); err != nil {
		return DictionaryApprovalSummary{}, fmt.Errorf("enable approved dictionary rule: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO technology_dictionary_rule_decisions (rule_id, decision, reviewer, rationale)
VALUES ($1, 'approved_enabled', $2, 'approved through the fixed v0.1 remote operation')`, ruleID, reviewer); err != nil {
		return DictionaryApprovalSummary{}, fmt.Errorf("record dictionary approval: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return DictionaryApprovalSummary{}, fmt.Errorf("commit dictionary approval: %w", err)
	}
	return summary, nil
}

// PrepareCandidateConfigurationForDictionary creates the durable reanalysis
// queue only after at least one deterministic rule has an explicit approval.
func (repository *Repository) PrepareCandidateConfigurationForDictionary(ctx context.Context, dictionaryVersionLabel, parserVersion, createdBy string) (CandidatePreparation, error) {
	if dictionaryVersionLabel == "" || parserVersion == "" || !validOperator(createdBy) {
		return CandidatePreparation{}, fmt.Errorf("candidate configuration input is invalid")
	}
	var dictionaryVersionID string
	var enabledRules int
	err := repository.pool.QueryRow(ctx, `
SELECT version.id::text,
       count(rule.id) FILTER (WHERE rule.state = 'enabled')
FROM technology_dictionary_versions AS version
LEFT JOIN technology_dictionary_rules AS rule ON rule.dictionary_version_id = version.id
WHERE version.version_label = $1 AND version.status = 'candidate'
GROUP BY version.id`, dictionaryVersionLabel).Scan(&dictionaryVersionID, &enabledRules)
	if err == pgx.ErrNoRows {
		return CandidatePreparation{}, fmt.Errorf("candidate dictionary version is unavailable")
	}
	if err != nil {
		return CandidatePreparation{}, fmt.Errorf("load candidate dictionary configuration: %w", err)
	}
	if enabledRules == 0 {
		return CandidatePreparation{}, fmt.Errorf("candidate dictionary has no approved deterministic rules")
	}
	return repository.PrepareCandidateConfiguration(ctx, CandidateConfigurationInput{
		ParserVersion: parserVersion, DictionaryVersionID: dictionaryVersionID, CreatedBy: createdBy,
	})
}

// CandidateConfigurationID resolves the one un-published configuration for a
// fixed parser/dictionary pair. This is intentionally separate from normal
// pending analysis, which must consume only the published pointer.
func (repository *Repository) CandidateConfigurationID(ctx context.Context, dictionaryVersionLabel, parserVersion string) (string, error) {
	var configurationID string
	err := repository.pool.QueryRow(ctx, `
SELECT configuration.id::text
FROM technology_analysis_configurations AS configuration
JOIN technology_dictionary_versions AS version ON version.id = configuration.dictionary_version_id
WHERE version.version_label = $1
  AND configuration.parser_version = $2
  AND configuration.status = 'candidate'`, dictionaryVersionLabel, parserVersion).Scan(&configurationID)
	if err == pgx.ErrNoRows {
		return "", fmt.Errorf("candidate analysis configuration is unavailable")
	}
	if err != nil {
		return "", fmt.Errorf("find candidate analysis configuration: %w", err)
	}
	return configurationID, nil
}

func dictionarySeedSHA256(seed DictionarySeed) (string, error) {
	encoded, err := json.Marshal(seed)
	if err != nil {
		return "", fmt.Errorf("encode candidate dictionary seed: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func validOperator(value string) bool {
	if len(value) == 0 || len(value) > 256 {
		return false
	}
	for index, character := range value {
		if index == 0 && !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9')) {
			return false
		}
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func validRuleKey(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for index, character := range value {
		if index == 0 && !((character >= 'a' && character <= 'z') || (character >= '0' && character <= '9')) {
			return false
		}
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '.' || character == '-' {
			continue
		}
		return false
	}
	return !strings.HasSuffix(value, ".")
}
