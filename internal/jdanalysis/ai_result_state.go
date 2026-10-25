package jdanalysis

import "context"

// CompleteAISemanticResult records a validated business conclusion. It cannot
// use not_analyzed: that state is reserved for a result whose execution never
// produced an accepted semantic conclusion.
func (repository *Repository) CompleteAISemanticResult(ctx context.Context, resultID, semanticStatus string) error {
	if resultID == "" || !validCompletedAISemanticStatus(semanticStatus) {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	command, err := repository.pool.Exec(ctx, `
UPDATE ai_semantic_results
SET execution_status = 'complete', semantic_status = $2::ai_semantic_value_status,
    safe_error_code = NULL, completed_at = now()
WHERE id = $1
  AND execution_status IN ('pending', 'running')`, resultID, semanticStatus)
	if err != nil || command.RowsAffected() != 1 {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	return nil
}

// FailAISemanticResult records a bounded execution outcome without claiming
// anything about the JD's semantic requirements.
func (repository *Repository) FailAISemanticResult(ctx context.Context, resultID string, code AISafeErrorCode) error {
	if resultID == "" || !code.valid() {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	command, err := repository.pool.Exec(ctx, `
UPDATE ai_semantic_results
SET execution_status = 'failed_safely', semantic_status = 'not_analyzed',
    safe_error_code = $2, completed_at = now()
WHERE id = $1
  AND execution_status IN ('pending', 'running')`, resultID, code)
	if err != nil || command.RowsAffected() != 1 {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	return nil
}

func validCompletedAISemanticStatus(status string) bool {
	return status == "configured" || status == "not_configured" || status == "uncertain"
}
