package jdanalysis

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

// CandidateAIConfigurationInput contains only fixed, non-secret contract
// identifiers and digests. Prompt bodies, JSON schemas, pricing documents,
// JD text, and credentials deliberately do not cross this repository seam.
type CandidateAIConfigurationInput struct {
	TechnologyConfigurationID string
	Provider                  string
	Model                     string
	ConfigurationSHA256       string
	RequestProfileSHA256      string
	PromptSHA256              string
	ResponseSchemaVersion     string
	ResponseSchemaSHA256      string
	InputContractVersion      string
	InputContractSHA256       string
	ValidatorVersion          string
	ValidatorSHA256           string
	PricingProfileVersion     string
	PricingProfileSHA256      string
	CreatedBy                 string
}

type AIConfiguration struct {
	ID     string
	Status string
}

type CandidateAIConfigurationPreparation struct {
	Configuration AIConfiguration
	Reused        bool
}

// AITrialPreparationInput binds the fixed Plus baseline candidate and the
// fixed Flash comparison to one published deterministic configuration. Both
// configurations retain their own request and pricing profiles, while sharing
// the semantic prompt, schema, input, and validator contracts.
type AITrialPreparationInput struct {
	Plus  CandidateAIConfigurationInput
	Flash CandidateAIConfigurationInput
}

// AITrialPreparation exposes only opaque identities and safe counts. The body
// IDs are the durable, frozen membership of the remote trial set; it has no
// source text, metadata, spans, or provider payloads.
type AITrialPreparation struct {
	BudgetGroupID         string
	Plus                  AIConfiguration
	Flash                 AIConfiguration
	BodyContentVersionIDs []string
	PlusQueued            int
	FlashQueued           int
	Reused                bool
}

// AIConfigurationSummary is the readable, non-secret frozen contract. It
// deliberately exposes only identifiers, versions, and digests; callers must
// resolve approved prompt/schema assets locally rather than persist their
// content alongside analysis facts.
type AIConfigurationSummary struct {
	AIConfiguration
	TechnologyConfigurationID string
	Provider                  string
	Model                     string
	ConfigurationSHA256       string
	RequestProfileSHA256      string
	PromptSHA256              string
	ResponseSchemaVersion     string
	ResponseSchemaSHA256      string
	InputContractVersion      string
	InputContractSHA256       string
	ValidatorVersion          string
	ValidatorSHA256           string
	PricingProfileVersion     string
	PricingProfileSHA256      string
}

// AISemanticResult is a redacted identity and status summary. It never
// contains input text, spans, provider output, or a free-form error.
type AISemanticResult struct {
	ID              string
	ExecutionStatus string
	SemanticStatus  string
	Reused          bool
}

// CreateCandidateAIConfiguration persists the immutable identity of an AI
// candidate. The database independently rejects references to any
// deterministic configuration that was not published.
func (repository *Repository) CreateCandidateAIConfiguration(ctx context.Context, input CandidateAIConfigurationInput) (AIConfiguration, error) {
	preparation, err := repository.PrepareCandidateAIConfiguration(ctx, input)
	if err != nil {
		return AIConfiguration{}, err
	}
	return preparation.Configuration, nil
}

// PrepareCandidateAIConfiguration returns the existing immutable candidate
// for an exact frozen identity. Any contract change must carry a different
// configuration digest and therefore creates a distinct candidate instead of
// rewriting historical results.
func (repository *Repository) PrepareCandidateAIConfiguration(ctx context.Context, input CandidateAIConfigurationInput) (CandidateAIConfigurationPreparation, error) {
	if !validCandidateAIConfigurationInput(input) {
		return CandidateAIConfigurationPreparation{}, fmt.Errorf("candidate AI configuration input is invalid")
	}

	var configuration AIConfiguration
	err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_analysis_configurations (
    technology_analysis_configuration_id, provider, model,
    configuration_sha256, request_profile_sha256, prompt_sha256,
    response_schema_version, response_schema_sha256,
    input_contract_version, input_contract_sha256,
    validator_version, validator_sha256,
    pricing_profile_version, pricing_profile_sha256, created_by
)
SELECT $1, $2::ai_analysis_provider, $3,
       $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
WHERE EXISTS (
    SELECT 1
    FROM technology_analysis_configurations
    WHERE id = $1 AND status = 'published'
)
ON CONFLICT (configuration_sha256) DO NOTHING
RETURNING id::text, status::text`,
		input.TechnologyConfigurationID, input.Provider, input.Model,
		input.ConfigurationSHA256, input.RequestProfileSHA256, input.PromptSHA256,
		input.ResponseSchemaVersion, input.ResponseSchemaSHA256,
		input.InputContractVersion, input.InputContractSHA256,
		input.ValidatorVersion, input.ValidatorSHA256,
		input.PricingProfileVersion, input.PricingProfileSHA256, input.CreatedBy,
	).Scan(&configuration.ID, &configuration.Status)
	if err == nil {
		return CandidateAIConfigurationPreparation{Configuration: configuration}, nil
	}
	if err != pgx.ErrNoRows {
		return CandidateAIConfigurationPreparation{}, fmt.Errorf("prepare candidate AI configuration: %w", err)
	}

	stored, err := repository.loadAIConfigurationIdentity(ctx, input.ConfigurationSHA256)
	if err == pgx.ErrNoRows {
		return CandidateAIConfigurationPreparation{}, fmt.Errorf("candidate AI configuration requires a published technology configuration")
	}
	if err != nil {
		return CandidateAIConfigurationPreparation{}, fmt.Errorf("load candidate AI configuration: %w", err)
	}
	if !stored.matches(input) {
		return CandidateAIConfigurationPreparation{}, fmt.Errorf("candidate AI configuration digest conflicts with a different frozen identity")
	}
	return CandidateAIConfigurationPreparation{
		Configuration: AIConfiguration{ID: stored.ID, Status: stored.Status},
		Reused:        true,
	}, nil
}

// PrepareAITrial creates or resumes the one controlled five-body comparison.
// Work-item membership itself is the frozen sample ledger: a later body can
// never silently expand either trial configuration.
func (repository *Repository) PrepareAITrial(ctx context.Context, input AITrialPreparationInput) (AITrialPreparation, error) {
	if !validAITrialPreparationInput(input) {
		return AITrialPreparation{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AITrialPreparation{}, fmt.Errorf("begin AI trial preparation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	plus, plusReused, err := prepareAITrialConfigurationTx(ctx, tx, input.Plus, "candidate")
	if err != nil {
		return AITrialPreparation{}, err
	}
	flash, flashReused, err := prepareAITrialConfigurationTx(ctx, tx, input.Flash, "comparison")
	if err != nil {
		return AITrialPreparation{}, err
	}
	budgetGroupID, err := ensureAITrialBudgetGroupTx(ctx, tx, plus.ID, flash.ID)
	if err != nil {
		return AITrialPreparation{}, err
	}

	plusSample, err := loadAITrialWorkSample(ctx, tx, plus.ID)
	if err != nil {
		return AITrialPreparation{}, err
	}
	flashSample, err := loadAITrialWorkSample(ctx, tx, flash.ID)
	if err != nil {
		return AITrialPreparation{}, err
	}
	preparation := AITrialPreparation{BudgetGroupID: budgetGroupID, Plus: plus, Flash: flash}
	if len(plusSample) != 0 || len(flashSample) != 0 {
		if len(plusSample) != formalAITrialSampleSize(input) || !equalStringSlices(plusSample, flashSample) {
			return AITrialPreparation{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
		}
		preparation.BodyContentVersionIDs = plusSample
		preparation.Reused = true
	} else {
		metrics, err := loadAITrialSamplingMetrics(ctx, tx, input.Plus.TechnologyConfigurationID)
		if err != nil {
			return AITrialPreparation{}, err
		}
		sample, err := selectFormalAITrialSample(input, metrics)
		if err != nil {
			return AITrialPreparation{}, err
		}
		sort.Strings(sample)
		plusQueued, err := enqueueAITrialWork(ctx, tx, plus.ID, sample, "candidate")
		if err != nil {
			return AITrialPreparation{}, err
		}
		flashQueued, err := enqueueAITrialWork(ctx, tx, flash.ID, sample, "comparison")
		if err != nil {
			return AITrialPreparation{}, err
		}
		if plusQueued != formalAITrialSampleSize(input) || flashQueued != formalAITrialSampleSize(input) {
			return AITrialPreparation{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
		}
		preparation.BodyContentVersionIDs = sample
		preparation.PlusQueued = plusQueued
		preparation.FlashQueued = flashQueued
		preparation.Reused = plusReused && flashReused
	}
	if err := tx.Commit(ctx); err != nil {
		return AITrialPreparation{}, fmt.Errorf("commit AI trial preparation: %w", err)
	}
	return preparation, nil
}

// selectFormalAITrialSample keeps the only historical 30-body contract
// reproducible without allowing the current fixed lifecycle to select it.
// Every current or unknown internal contract receives the five-body policy.
func selectFormalAITrialSample(input AITrialPreparationInput, metrics []AIBodySamplingMetric) ([]string, error) {
	if formalAITrialSampleSize(input) == aiLegacyTrialSampleSizeV1 {
		return SelectAITrialSampleV1(metrics)
	}
	return SelectAITrialSampleV2(metrics)
}

func formalAITrialSampleSize(input AITrialPreparationInput) int {
	if input.Plus.InputContractSHA256 == legacyThirtyBodyControlledAIInputContractSHA256() &&
		input.Flash.InputContractSHA256 == legacyThirtyBodyControlledAIInputContractSHA256() {
		return aiLegacyTrialSampleSizeV1
	}
	return aiTrialSampleSize
}

// ensureAITrialBudgetGroupTx binds the fixed Plus and Flash configurations to
// one immutable accounting boundary. Every subsequent pre-send reservation
// must resolve through this membership instead of receiving a per-process
// allowance. The group contains no prompt, text, response, price input, or
// credential.
func ensureAITrialBudgetGroupTx(ctx context.Context, tx pgx.Tx, plusConfigurationID, flashConfigurationID string) (string, error) {
	if plusConfigurationID == "" || flashConfigurationID == "" || plusConfigurationID == flashConfigurationID {
		return "", NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	if _, err := tx.Exec(ctx, `
SELECT id
FROM ai_analysis_configurations
WHERE id IN ($1, $2)
ORDER BY id
FOR UPDATE`, plusConfigurationID, flashConfigurationID); err != nil {
		return "", fmt.Errorf("lock AI trial configurations for budget group: %w", err)
	}
	var existingGroupID string
	err := tx.QueryRow(ctx, `
SELECT budget_group.id::text
FROM ai_analysis_trial_budget_groups AS budget_group
JOIN ai_analysis_trial_budget_group_members AS plus_member
  ON plus_member.ai_analysis_trial_budget_group_id = budget_group.id
 AND plus_member.ai_analysis_configuration_id = $1
JOIN ai_analysis_trial_budget_group_members AS flash_member
  ON flash_member.ai_analysis_trial_budget_group_id = budget_group.id
 AND flash_member.ai_analysis_configuration_id = $2`, plusConfigurationID, flashConfigurationID).Scan(&existingGroupID)
	if err == nil {
		return existingGroupID, nil
	}
	if err != pgx.ErrNoRows {
		return "", fmt.Errorf("load AI trial budget group: %w", err)
	}

	var alreadyBound int
	if err := tx.QueryRow(ctx, `
SELECT count(*)
FROM ai_analysis_trial_budget_group_members
WHERE ai_analysis_configuration_id IN ($1, $2)`, plusConfigurationID, flashConfigurationID).Scan(&alreadyBound); err != nil {
		return "", fmt.Errorf("inspect AI trial budget membership: %w", err)
	}
	if alreadyBound != 0 {
		return "", NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	if err := tx.QueryRow(ctx, `
INSERT INTO ai_analysis_trial_budget_groups DEFAULT VALUES
RETURNING id::text`).Scan(&existingGroupID); err != nil {
		return "", fmt.Errorf("create AI trial budget group: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO ai_analysis_trial_budget_group_members (
    ai_analysis_trial_budget_group_id, ai_analysis_configuration_id
) VALUES ($1, $2), ($1, $3)`, existingGroupID, plusConfigurationID, flashConfigurationID); err != nil {
		return "", fmt.Errorf("bind AI trial budget group members: %w", err)
	}
	return existingGroupID, nil
}

func prepareAITrialConfigurationTx(ctx context.Context, tx pgx.Tx, input CandidateAIConfigurationInput, expectedStatus string) (AIConfiguration, bool, error) {
	var configuration AIConfiguration
	err := tx.QueryRow(ctx, `
INSERT INTO ai_analysis_configurations (
    technology_analysis_configuration_id, provider, model,
    configuration_sha256, request_profile_sha256, prompt_sha256,
    response_schema_version, response_schema_sha256,
    input_contract_version, input_contract_sha256,
    validator_version, validator_sha256,
    pricing_profile_version, pricing_profile_sha256, status, created_by
)
SELECT $1, $2::ai_analysis_provider, $3,
       $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
       $15::ai_analysis_configuration_status, $16
WHERE EXISTS (
    SELECT 1
    FROM technology_analysis_configurations
    WHERE id = $1 AND status = 'published'
)
ON CONFLICT (configuration_sha256) DO NOTHING
RETURNING id::text, status::text`,
		input.TechnologyConfigurationID, input.Provider, input.Model,
		input.ConfigurationSHA256, input.RequestProfileSHA256, input.PromptSHA256,
		input.ResponseSchemaVersion, input.ResponseSchemaSHA256,
		input.InputContractVersion, input.InputContractSHA256,
		input.ValidatorVersion, input.ValidatorSHA256,
		input.PricingProfileVersion, input.PricingProfileSHA256, expectedStatus, input.CreatedBy,
	).Scan(&configuration.ID, &configuration.Status)
	if err == nil {
		return configuration, false, nil
	}
	if err != pgx.ErrNoRows {
		return AIConfiguration{}, false, fmt.Errorf("prepare AI trial configuration: %w", err)
	}

	stored, err := loadAIConfigurationIdentityTx(ctx, tx, input.ConfigurationSHA256)
	if err == pgx.ErrNoRows {
		return AIConfiguration{}, false, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	if err != nil {
		return AIConfiguration{}, false, fmt.Errorf("load AI trial configuration: %w", err)
	}
	if !stored.matches(input) || stored.Status != expectedStatus {
		return AIConfiguration{}, false, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	return AIConfiguration{ID: stored.ID, Status: stored.Status}, true, nil
}

func loadAITrialSamplingMetrics(ctx context.Context, tx pgx.Tx, technologyConfigurationID string) ([]AIBodySamplingMetric, error) {
	rows, err := tx.Query(ctx, `
SELECT body.id::text,
       char_length(body.description_text) + char_length(body.job_require_text) + char_length(body.addition_text),
       (CASE WHEN body.description_text <> '' THEN 1 ELSE 0 END
        + CASE WHEN body.job_require_text <> '' THEN 1 ELSE 0 END
        + CASE WHEN body.addition_text <> '' THEN 1 ELSE 0 END),
       count(DISTINCT mention.id)
FROM job_body_content_versions AS body
JOIN technology_analysis_results AS result
  ON result.body_content_version_id = body.id
 AND result.configuration_id = $1
 AND result.status = 'complete'
JOIN technology_analysis_configurations AS configuration
  ON configuration.id = result.configuration_id
 AND configuration.status = 'published'
LEFT JOIN technology_mentions AS mention ON mention.analysis_result_id = result.id
GROUP BY body.id, body.description_text, body.job_require_text, body.addition_text
ORDER BY body.id`, technologyConfigurationID)
	if err != nil {
		return nil, fmt.Errorf("load AI trial sampling metrics: %w", err)
	}
	defer rows.Close()
	metrics := make([]AIBodySamplingMetric, 0)
	for rows.Next() {
		var metric AIBodySamplingMetric
		if err := rows.Scan(&metric.BodyContentVersionID, &metric.BodyLengthCodePoints, &metric.CompleteFieldCount, &metric.VerifiedMentionCount); err != nil {
			return nil, fmt.Errorf("scan AI trial sampling metrics: %w", err)
		}
		metrics = append(metrics, metric)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate AI trial sampling metrics: %w", err)
	}
	return metrics, nil
}

func loadAITrialWorkSample(ctx context.Context, tx pgx.Tx, configurationID string) ([]string, error) {
	rows, err := tx.Query(ctx, `
SELECT body_content_version_id::text
FROM ai_analysis_work_items
WHERE ai_analysis_configuration_id = $1
ORDER BY body_content_version_id`, configurationID)
	if err != nil {
		return nil, fmt.Errorf("load frozen AI trial work sample: %w", err)
	}
	defer rows.Close()
	sample := make([]string, 0)
	for rows.Next() {
		var bodyContentVersionID string
		if err := rows.Scan(&bodyContentVersionID); err != nil {
			return nil, fmt.Errorf("scan frozen AI trial work sample: %w", err)
		}
		sample = append(sample, bodyContentVersionID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate frozen AI trial work sample: %w", err)
	}
	return sample, nil
}

func enqueueAITrialWork(ctx context.Context, tx pgx.Tx, configurationID string, bodyContentVersionIDs []string, expectedConfigurationStatus string) (int, error) {
	queued := 0
	for _, bodyContentVersionID := range bodyContentVersionIDs {
		if _, err := tx.Exec(ctx, `
INSERT INTO ai_semantic_results (body_content_version_id, ai_analysis_configuration_id)
SELECT $1, configuration.id
FROM ai_analysis_configurations AS configuration
WHERE configuration.id = $2
  AND configuration.status = $3::ai_analysis_configuration_status
ON CONFLICT (body_content_version_id, ai_analysis_configuration_id) DO NOTHING`,
			bodyContentVersionID, configurationID, expectedConfigurationStatus); err != nil {
			return 0, fmt.Errorf("create AI trial semantic result: %w", err)
		}
		command, err := tx.Exec(ctx, `
INSERT INTO ai_analysis_work_items (
    ai_analysis_configuration_id, body_content_version_id, trigger
)
SELECT configuration.id, $1, 'trial'::ai_analysis_trigger
FROM ai_analysis_configurations AS configuration
WHERE configuration.id = $2
  AND configuration.status = $3::ai_analysis_configuration_status
ON CONFLICT (body_content_version_id, ai_analysis_configuration_id) DO NOTHING`,
			bodyContentVersionID, configurationID, expectedConfigurationStatus)
		if err != nil {
			return 0, fmt.Errorf("queue AI trial work: %w", err)
		}
		queued += int(command.RowsAffected())
	}
	return queued, nil
}

func loadAIConfigurationIdentityTx(ctx context.Context, tx pgx.Tx, configurationSHA256 string) (storedAIConfigurationIdentity, error) {
	return scanStoredAIConfigurationIdentity(tx.QueryRow(ctx, `
SELECT `+aiConfigurationIdentityColumns+`
FROM ai_analysis_configurations
WHERE configuration_sha256 = $1`, configurationSHA256))
}

func (repository *Repository) ReadAIConfiguration(ctx context.Context, configurationID string) (AIConfigurationSummary, error) {
	if configurationID == "" {
		return AIConfigurationSummary{}, fmt.Errorf("AI configuration id is required")
	}
	stored, err := scanStoredAIConfigurationIdentity(repository.pool.QueryRow(ctx, `
SELECT `+aiConfigurationIdentityColumns+`
FROM ai_analysis_configurations
WHERE id = $1`, configurationID))
	if err == pgx.ErrNoRows {
		return AIConfigurationSummary{}, fmt.Errorf("AI configuration is unavailable")
	}
	if err != nil {
		return AIConfigurationSummary{}, fmt.Errorf("read AI configuration: %w", err)
	}
	return stored.summary(), nil
}

type storedAIConfigurationIdentity struct {
	AIConfiguration
	TechnologyConfigurationID string
	Provider                  string
	Model                     string
	ConfigurationSHA256       string
	RequestProfileSHA256      string
	PromptSHA256              string
	ResponseSchemaVersion     string
	ResponseSchemaSHA256      string
	InputContractVersion      string
	InputContractSHA256       string
	ValidatorVersion          string
	ValidatorSHA256           string
	PricingProfileVersion     string
	PricingProfileSHA256      string
}

const aiConfigurationIdentityColumns = `
id::text, status::text, technology_analysis_configuration_id::text,
provider::text, model, configuration_sha256, request_profile_sha256, prompt_sha256,
response_schema_version, response_schema_sha256,
input_contract_version, input_contract_sha256,
validator_version, validator_sha256,
pricing_profile_version, pricing_profile_sha256`

func scanStoredAIConfigurationIdentity(row pgx.Row) (storedAIConfigurationIdentity, error) {
	var stored storedAIConfigurationIdentity
	err := row.Scan(
		&stored.ID, &stored.Status, &stored.TechnologyConfigurationID,
		&stored.Provider, &stored.Model, &stored.ConfigurationSHA256, &stored.RequestProfileSHA256, &stored.PromptSHA256,
		&stored.ResponseSchemaVersion, &stored.ResponseSchemaSHA256,
		&stored.InputContractVersion, &stored.InputContractSHA256,
		&stored.ValidatorVersion, &stored.ValidatorSHA256,
		&stored.PricingProfileVersion, &stored.PricingProfileSHA256,
	)
	return stored, err
}

func (repository *Repository) loadAIConfigurationIdentity(ctx context.Context, configurationSHA256 string) (storedAIConfigurationIdentity, error) {
	return scanStoredAIConfigurationIdentity(repository.pool.QueryRow(ctx, `
SELECT `+aiConfigurationIdentityColumns+`
FROM ai_analysis_configurations
WHERE configuration_sha256 = $1`, configurationSHA256))
}

func (stored storedAIConfigurationIdentity) summary() AIConfigurationSummary {
	return AIConfigurationSummary{
		AIConfiguration:           stored.AIConfiguration,
		TechnologyConfigurationID: stored.TechnologyConfigurationID,
		Provider:                  stored.Provider,
		Model:                     stored.Model,
		ConfigurationSHA256:       stored.ConfigurationSHA256,
		RequestProfileSHA256:      stored.RequestProfileSHA256,
		PromptSHA256:              stored.PromptSHA256,
		ResponseSchemaVersion:     stored.ResponseSchemaVersion,
		ResponseSchemaSHA256:      stored.ResponseSchemaSHA256,
		InputContractVersion:      stored.InputContractVersion,
		InputContractSHA256:       stored.InputContractSHA256,
		ValidatorVersion:          stored.ValidatorVersion,
		ValidatorSHA256:           stored.ValidatorSHA256,
		PricingProfileVersion:     stored.PricingProfileVersion,
		PricingProfileSHA256:      stored.PricingProfileSHA256,
	}
}

func (stored storedAIConfigurationIdentity) matches(input CandidateAIConfigurationInput) bool {
	return stored.TechnologyConfigurationID == input.TechnologyConfigurationID &&
		stored.Provider == input.Provider &&
		stored.Model == input.Model &&
		stored.RequestProfileSHA256 == input.RequestProfileSHA256 &&
		stored.PromptSHA256 == input.PromptSHA256 &&
		stored.ResponseSchemaVersion == input.ResponseSchemaVersion &&
		stored.ResponseSchemaSHA256 == input.ResponseSchemaSHA256 &&
		stored.InputContractVersion == input.InputContractVersion &&
		stored.InputContractSHA256 == input.InputContractSHA256 &&
		stored.ValidatorVersion == input.ValidatorVersion &&
		stored.ValidatorSHA256 == input.ValidatorSHA256 &&
		stored.PricingProfileVersion == input.PricingProfileVersion &&
		stored.PricingProfileSHA256 == input.PricingProfileSHA256
}

// EnsureAISemanticResult creates the one durable result slot for an eligible
// body/configuration identity. Calling it again returns that same slot, so a
// later worker can resume safely without creating a second model task.
func (repository *Repository) EnsureAISemanticResult(ctx context.Context, bodyContentVersionID, aiConfigurationID string) (AISemanticResult, error) {
	if bodyContentVersionID == "" || aiConfigurationID == "" {
		return AISemanticResult{}, fmt.Errorf("AI semantic result identity is incomplete")
	}

	var result AISemanticResult
	err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_semantic_results (body_content_version_id, ai_analysis_configuration_id)
SELECT $1, configuration.id
FROM ai_analysis_configurations AS configuration
WHERE configuration.id = $2
  AND configuration.status IN ('candidate', 'comparison', 'published')
ON CONFLICT (body_content_version_id, ai_analysis_configuration_id) DO NOTHING
RETURNING id::text, execution_status::text, semantic_status::text`,
		bodyContentVersionID, aiConfigurationID,
	).Scan(&result.ID, &result.ExecutionStatus, &result.SemanticStatus)
	if err == nil {
		return result, nil
	}
	if err != pgx.ErrNoRows {
		return AISemanticResult{}, fmt.Errorf("create AI semantic result: %w", err)
	}

	err = repository.pool.QueryRow(ctx, `
SELECT id::text, execution_status::text, semantic_status::text
FROM ai_semantic_results
WHERE body_content_version_id = $1 AND ai_analysis_configuration_id = $2`,
		bodyContentVersionID, aiConfigurationID,
	).Scan(&result.ID, &result.ExecutionStatus, &result.SemanticStatus)
	if err == pgx.ErrNoRows {
		return AISemanticResult{}, fmt.Errorf("AI semantic result requires an eligible body and configuration")
	}
	if err != nil {
		return AISemanticResult{}, fmt.Errorf("load AI semantic result: %w", err)
	}
	result.Reused = true
	return result, nil
}

func validCandidateAIConfigurationInput(input CandidateAIConfigurationInput) bool {
	return validAIConfigurationInput(input, "qwen-plus")
}

func validAITrialPreparationInput(input AITrialPreparationInput) bool {
	if !validAIConfigurationInput(input.Plus, "qwen-plus") || !validAIConfigurationInput(input.Flash, "qwen-flash") ||
		input.Plus.ConfigurationSHA256 == input.Flash.ConfigurationSHA256 {
		return false
	}
	return input.Plus.TechnologyConfigurationID == input.Flash.TechnologyConfigurationID &&
		input.Plus.Provider == input.Flash.Provider &&
		input.Plus.PromptSHA256 == input.Flash.PromptSHA256 &&
		input.Plus.ResponseSchemaVersion == input.Flash.ResponseSchemaVersion &&
		input.Plus.ResponseSchemaSHA256 == input.Flash.ResponseSchemaSHA256 &&
		input.Plus.InputContractVersion == input.Flash.InputContractVersion &&
		input.Plus.InputContractSHA256 == input.Flash.InputContractSHA256 &&
		input.Plus.ValidatorVersion == input.Flash.ValidatorVersion &&
		input.Plus.ValidatorSHA256 == input.Flash.ValidatorSHA256 &&
		input.Plus.CreatedBy == input.Flash.CreatedBy
}

func validAIConfigurationInput(input CandidateAIConfigurationInput, expectedModel string) bool {
	if input.TechnologyConfigurationID == "" || input.Provider != "bailian" || input.Model != expectedModel ||
		!validSHA256(input.ConfigurationSHA256) || !validSHA256(input.RequestProfileSHA256) ||
		!validSHA256(input.PromptSHA256) || !validSHA256(input.ResponseSchemaSHA256) ||
		!validSHA256(input.InputContractSHA256) || !validSHA256(input.ValidatorSHA256) ||
		!validSHA256(input.PricingProfileSHA256) {
		return false
	}
	return validBoundedLabel(input.ResponseSchemaVersion, 128) &&
		validBoundedLabel(input.InputContractVersion, 128) &&
		validBoundedLabel(input.ValidatorVersion, 128) &&
		validBoundedLabel(input.PricingProfileVersion, 128) &&
		validBoundedLabel(input.CreatedBy, 256)
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

func validBoundedLabel(value string, maximum int) bool {
	return len(strings.TrimSpace(value)) > 0 && len(value) <= maximum
}
