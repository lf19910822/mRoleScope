package jdanalysis

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// PersistentAITrialBudget is the database-backed pre-send gate for the one
// current Qwen Plus trial. Separate worker processes obtain independent values
// of this type but resolve to the same immutable budget group.
type PersistentAITrialBudget struct {
	configurationID string
	groupID         string
	profile         AITrialPricingProfile
	repository      *Repository
}

// PersistentAITrialBudgetReservation is an opaque, durable reservation. It
// has no model input, request text, response, candidate text, or credential.
type PersistentAITrialBudgetReservation struct {
	configurationID string
	groupID         string
	workItemID      string
	attemptNumber   int
}

// NewPersistentAITrialBudget validates that an immutable Qwen Plus candidate
// belongs to the frozen five-body trial and that its stored pricing digest
// equals the reviewed in-image profile. Historical Flash comparisons are
// deliberately unreadable through this execution path.
func (repository *Repository) NewPersistentAITrialBudget(ctx context.Context, configurationID string) (*PersistentAITrialBudget, error) {
	if repository == nil || configurationID == "" {
		return nil, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	var groupID, pricingVersion, pricingSHA256 string
	err := repository.pool.QueryRow(ctx, `
SELECT budget_group.id::text,
       configuration.pricing_profile_version,
       configuration.pricing_profile_sha256
FROM ai_analysis_configurations AS configuration
JOIN ai_analysis_trial_budget_group_members AS member
  ON member.ai_analysis_configuration_id = configuration.id
JOIN ai_analysis_trial_budget_groups AS budget_group
  ON budget_group.id = member.ai_analysis_trial_budget_group_id
WHERE configuration.id = $1
  AND configuration.model = 'qwen-plus'
  AND configuration.status = 'candidate'`, configurationID).Scan(
		&groupID, &pricingVersion, &pricingSHA256,
	)
	if err != nil {
		return nil, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	profile := QwenPlusTrialPricingProfile()
	if pricingVersion != profile.Version || pricingSHA256 != profile.SHA256() || !validAITrialPricingProfile(profile) {
		return nil, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	return &PersistentAITrialBudget{
		configurationID: configurationID,
		groupID:         groupID,
		profile:         profile,
		repository:      repository,
	}, nil
}

// Reserve commits one worst-case physical attempt before a provider can be
// called. The fixed group row is locked during the calculation, making limits
// hold across processes instead of only within one runner's memory.
func (budget *PersistentAITrialBudget) Reserve(ctx context.Context, workItemID string, attemptNumber, estimatedInputTokens int) (PersistentAITrialBudgetReservation, error) {
	if budget == nil || budget.repository == nil || workItemID == "" || attemptNumber < 1 || attemptNumber > aiModelMaximumAttempts || estimatedInputTokens < 0 {
		return PersistentAITrialBudgetReservation{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	if estimatedInputTokens > budget.profile.MaxInputTokens {
		return PersistentAITrialBudgetReservation{}, NewSafeAIError(AISafeErrorInputTooLarge, nil)
	}
	tx, err := budget.repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return PersistentAITrialBudgetReservation{}, fmt.Errorf("begin persistent AI trial budget reservation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	maximumLogicalTasks, maximumAttempts, maximumCharge, err := lockPersistentAITrialBudgetGroup(ctx, tx, budget.groupID)
	if err != nil {
		return PersistentAITrialBudgetReservation{}, err
	}
	var eligibleWork bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM ai_analysis_work_items
    WHERE id = $1
      AND ai_analysis_configuration_id = $2
      AND trigger = 'trial'
      AND status IN ('pending', 'running')
)`, workItemID, budget.configurationID).Scan(&eligibleWork); err != nil {
		return PersistentAITrialBudgetReservation{}, fmt.Errorf("verify persistent AI trial work: %w", err)
	}
	if !eligibleWork {
		return PersistentAITrialBudgetReservation{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}

	var attemptCount, logicalTaskCount int
	var chargedNanoRMB, reservedNanoRMB int64
	if err := tx.QueryRow(ctx, persistentAITrialBudgetSnapshotSQL, budget.groupID).Scan(
		&logicalTaskCount, &attemptCount, &chargedNanoRMB, &reservedNanoRMB,
	); err != nil {
		return PersistentAITrialBudgetReservation{}, fmt.Errorf("read persistent AI trial budget: %w", err)
	}
	var hasPriorReservationForWork bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM ai_analysis_trial_budget_reservations
    WHERE ai_analysis_trial_budget_group_id = $1
      AND work_item_id = $2
)`, budget.groupID, workItemID).Scan(&hasPriorReservationForWork); err != nil {
		return PersistentAITrialBudgetReservation{}, fmt.Errorf("inspect persistent AI trial logical work: %w", err)
	}
	if (!hasPriorReservationForWork && logicalTaskCount >= maximumLogicalTasks) || attemptCount >= maximumAttempts {
		return PersistentAITrialBudgetReservation{}, NewSafeAIError(AISafeErrorBudgetExhausted, nil)
	}

	// The reserve is based on the configuration's fixed request maxima rather
	// than an estimator alone. Returned usage can reduce this charge, but can
	// never increase a successful pre-send reservation beyond it.
	reservedCharge := budget.profile.quote(budget.profile.MaxInputTokens, budget.profile.MaxOutputTokens)
	if reservedNanoRMB+reservedCharge > maximumCharge {
		return PersistentAITrialBudgetReservation{}, NewSafeAIError(AISafeErrorBudgetExhausted, nil)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO ai_analysis_trial_budget_reservations (
    ai_analysis_trial_budget_group_id, ai_analysis_configuration_id,
    work_item_id, attempt_number, estimated_input_tokens, reserved_charge_nano_rmb
) VALUES ($1, $2, $3, $4, $5, $6)`, budget.groupID, budget.configurationID,
		workItemID, attemptNumber, estimatedInputTokens, reservedCharge); err != nil {
		return PersistentAITrialBudgetReservation{}, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return PersistentAITrialBudgetReservation{}, fmt.Errorf("commit persistent AI trial budget reservation: %w", err)
	}
	return PersistentAITrialBudgetReservation{
		configurationID: budget.configurationID,
		groupID:         budget.groupID,
		workItemID:      workItemID,
		attemptNumber:   attemptNumber,
	}, nil
}

// RecordUsage replaces a committed worst-case reservation with returned,
// bounded usage. Missing or invalid usage remains an unsettled reservation and
// therefore continues to consume its full safety allowance.
func (budget *PersistentAITrialBudget) RecordUsage(ctx context.Context, reservation PersistentAITrialBudgetReservation, usage AIModelUsage) error {
	if budget == nil || budget.repository == nil || reservation.configurationID != budget.configurationID || reservation.groupID != budget.groupID || reservation.workItemID == "" || !usage.valid() || usage.TotalTokens != usage.InputTokens+usage.OutputTokens || usage.InputTokens > budget.profile.MaxInputTokens || usage.OutputTokens > budget.profile.MaxOutputTokens {
		return NewSafeAIError(AISafeErrorBudgetExhausted, nil)
	}
	tx, err := budget.repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin persistent AI trial usage settlement: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, _, maximumCharge, err := lockPersistentAITrialBudgetGroup(ctx, tx, budget.groupID)
	if err != nil {
		return err
	}
	actualCharge := budget.profile.quote(usage.InputTokens, usage.OutputTokens)
	var reservedCharge int64
	err = tx.QueryRow(ctx, `
SELECT reserved_charge_nano_rmb
FROM ai_analysis_trial_budget_reservations
WHERE ai_analysis_trial_budget_group_id = $1
  AND ai_analysis_configuration_id = $2
  AND work_item_id = $3
  AND attempt_number = $4
  AND status = 'reserved'
FOR UPDATE`, budget.groupID, budget.configurationID, reservation.workItemID, reservation.attemptNumber).Scan(&reservedCharge)
	if err != nil {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	if actualCharge > reservedCharge {
		return NewSafeAIError(AISafeErrorBudgetExhausted, nil)
	}
	var _, _, _, reservedNanoRMB int64
	if err := tx.QueryRow(ctx, persistentAITrialBudgetSnapshotSQL, budget.groupID).Scan(new(int), new(int), new(int64), &reservedNanoRMB); err != nil {
		return fmt.Errorf("read persistent AI trial budget for settlement: %w", err)
	}
	if reservedNanoRMB-reservedCharge+actualCharge > maximumCharge {
		return NewSafeAIError(AISafeErrorBudgetExhausted, nil)
	}
	command, err := tx.Exec(ctx, `
UPDATE ai_analysis_trial_budget_reservations
SET status = 'settled', actual_input_tokens = $5, actual_output_tokens = $6,
    actual_charge_nano_rmb = $7, settled_at = now()
WHERE ai_analysis_trial_budget_group_id = $1
  AND ai_analysis_configuration_id = $2
  AND work_item_id = $3
  AND attempt_number = $4
  AND status = 'reserved'`, budget.groupID, budget.configurationID, reservation.workItemID, reservation.attemptNumber,
		usage.InputTokens, usage.OutputTokens, actualCharge)
	if err != nil || command.RowsAffected() != 1 {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit persistent AI trial usage settlement: %w", err)
	}
	return nil
}

// Snapshot returns the aggregate state of the one shared trial budget. The
// result is safe to expose in tests and controlled status output.
func (budget *PersistentAITrialBudget) Snapshot(ctx context.Context) (AITrialBudgetSnapshot, error) {
	if budget == nil || budget.repository == nil {
		return AITrialBudgetSnapshot{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	var snapshot AITrialBudgetSnapshot
	err := budget.repository.pool.QueryRow(ctx, persistentAITrialBudgetSnapshotSQL, budget.groupID).Scan(
		&snapshot.LogicalTaskCount, &snapshot.AttemptCount, &snapshot.ChargedNanoRMB, &snapshot.ReservedNanoRMB,
	)
	if err != nil {
		return AITrialBudgetSnapshot{}, fmt.Errorf("read persistent AI trial budget snapshot: %w", err)
	}
	return snapshot, nil
}

const persistentAITrialBudgetSnapshotSQL = `
SELECT count(DISTINCT work_item_id),
       count(*),
       COALESCE(sum(CASE WHEN status = 'settled' THEN actual_charge_nano_rmb ELSE 0 END), 0),
       COALESCE(sum(CASE WHEN status = 'settled' THEN actual_charge_nano_rmb ELSE reserved_charge_nano_rmb END), 0)
FROM ai_analysis_trial_budget_reservations
WHERE ai_analysis_trial_budget_group_id = $1`

func lockPersistentAITrialBudgetGroup(ctx context.Context, tx pgx.Tx, groupID string) (maximumLogicalTasks, maximumAttempts int, maximumCharge int64, err error) {
	err = tx.QueryRow(ctx, `
SELECT maximum_logical_task_count, maximum_physical_attempt_count, maximum_charge_nano_rmb
FROM ai_analysis_trial_budget_groups
WHERE id = $1
FOR UPDATE`, groupID).Scan(&maximumLogicalTasks, &maximumAttempts, &maximumCharge)
	if err != nil {
		return 0, 0, 0, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	return maximumLogicalTasks, maximumAttempts, maximumCharge, nil
}
