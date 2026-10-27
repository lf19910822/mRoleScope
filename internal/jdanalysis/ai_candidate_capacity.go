package jdanalysis

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const aiPendingCandidateCapacity = 500

// DictionaryCandidateGovernanceState records a candidate term's human
// governance lifecycle separately from its non-executable dictionary rule
// state. Only pending_review consumes one global candidate-capacity slot.
type DictionaryCandidateGovernanceState string

const (
	DictionaryCandidateGovernancePendingReview DictionaryCandidateGovernanceState = "pending_review"
	DictionaryCandidateGovernanceApproved      DictionaryCandidateGovernanceState = "approved"
	DictionaryCandidateGovernanceRejected      DictionaryCandidateGovernanceState = "rejected"
)

func (state DictionaryCandidateGovernanceState) validResolution() bool {
	return state == DictionaryCandidateGovernanceApproved || state == DictionaryCandidateGovernanceRejected
}

// reservePendingDictionaryCandidate either reuses a candidate already present
// in this dictionary version or reserves one global pending-review term. The
// advisory transaction lock makes the distinct-term capacity deterministic
// under concurrent processors.
func (repository *Repository) reservePendingDictionaryCandidate(ctx context.Context, dictionaryVersionID, candidateText string) (string, bool, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", false, fmt.Errorf("begin AI candidate capacity reservation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	candidateID, accepted, err := reservePendingDictionaryCandidateTx(ctx, tx, dictionaryVersionID, candidateText)
	if err != nil || !accepted {
		return candidateID, accepted, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", false, fmt.Errorf("commit AI candidate capacity reservation: %w", err)
	}
	return candidateID, true, nil
}

func reservePendingDictionaryCandidateTx(ctx context.Context, tx pgx.Tx, dictionaryVersionID, candidateText string) (string, bool, error) {
	if _, err := tx.Exec(ctx, `
SELECT pg_advisory_xact_lock(hashtextextended('mrolescope:ai-pending-candidate-capacity-v1', 0))`); err != nil {
		return "", false, fmt.Errorf("lock AI candidate capacity: %w", err)
	}

	var existingID string
	err := tx.QueryRow(ctx, `
SELECT id::text
FROM technology_dictionary_candidates
WHERE dictionary_version_id = $1
  AND lower(candidate_text) = lower($2)
ORDER BY created_at, id
LIMIT 1
FOR UPDATE`, dictionaryVersionID, candidateText).Scan(&existingID)
	if err == nil {
		return existingID, true, nil
	}
	if err != pgx.ErrNoRows {
		return "", false, fmt.Errorf("find existing AI dictionary candidate: %w", err)
	}

	var pendingTermExists bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM technology_dictionary_candidates
    WHERE governance_status = 'pending_review'
      AND lower(candidate_text) = lower($1)
)`, candidateText).Scan(&pendingTermExists); err != nil {
		return "", false, fmt.Errorf("check existing pending AI candidate term: %w", err)
	}
	if !pendingTermExists {
		var pendingTerms int
		if err := tx.QueryRow(ctx, `
SELECT count(DISTINCT lower(candidate_text))
FROM technology_dictionary_candidates
WHERE governance_status = 'pending_review'`).Scan(&pendingTerms); err != nil {
			return "", false, fmt.Errorf("count pending AI candidate terms: %w", err)
		}
		if pendingTerms >= aiPendingCandidateCapacity {
			return "", false, nil
		}
	}

	var candidateID string
	err = tx.QueryRow(ctx, `
INSERT INTO technology_dictionary_candidates (
    dictionary_version_id, candidate_text, state, governance_status, candidate_summary
) VALUES ($1, $2, 'candidate', 'pending_review', 'AI semantic candidate requires human approval')
RETURNING id::text`, dictionaryVersionID, candidateText).Scan(&candidateID)
	if err != nil {
		return "", false, fmt.Errorf("insert AI dictionary candidate: %w", err)
	}
	return candidateID, true, nil
}

// ResolveDictionaryCandidateGovernance records a terminal human decision
// without deleting the candidate row. Resolved rows remain auditable but no
// longer consume pending-review capacity.
func (repository *Repository) ResolveDictionaryCandidateGovernance(ctx context.Context, dictionaryVersionID, candidateText string, state DictionaryCandidateGovernanceState) error {
	if dictionaryVersionID == "" || candidateText == "" || !state.validResolution() {
		return fmt.Errorf("invalid dictionary candidate governance resolution")
	}
	command, err := repository.pool.Exec(ctx, `
UPDATE technology_dictionary_candidates
SET governance_status = $3
WHERE dictionary_version_id = $1
  AND lower(candidate_text) = lower($2)
  AND governance_status = 'pending_review'`, dictionaryVersionID, candidateText, state)
	if err != nil {
		return fmt.Errorf("resolve dictionary candidate governance: %w", err)
	}
	if command.RowsAffected() == 1 {
		return nil
	}
	return fmt.Errorf("pending dictionary candidate is unavailable for resolution")
}
