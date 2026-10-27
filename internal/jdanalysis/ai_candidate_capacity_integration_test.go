package jdanalysis

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func TestAIPendingCandidateCapacityIsGlobalConcurrentAndReleasesOnResolution(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	_, dictionaryID, _, _ := insertSyntheticAnalysisPrerequisites(t, repository)

	for index := 0; index < aiPendingCandidateCapacity-1; index++ {
		if _, err := repository.pool.Exec(ctx, `
INSERT INTO technology_dictionary_candidates (
    dictionary_version_id, candidate_text, state, governance_status, candidate_summary
) VALUES ($1, $2, 'candidate', 'pending_review', 'synthetic pending candidate')`,
			dictionaryID, fmt.Sprintf("ExistingCandidate%03d", index)); err != nil {
			t.Fatal(err)
		}
	}

	const contenders = 4
	start := make(chan struct{})
	type outcome struct {
		text     string
		accepted bool
		err      error
	}
	outcomes := make(chan outcome, contenders)
	var group sync.WaitGroup
	for index := 0; index < contenders; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			text := fmt.Sprintf("ConcurrentCandidate%03d", index)
			<-start
			_, accepted, err := repository.reservePendingDictionaryCandidate(context.Background(), dictionaryID, text)
			outcomes <- outcome{text: text, accepted: accepted, err: err}
		}(index)
	}
	close(start)
	group.Wait()
	close(outcomes)

	acceptedText := ""
	acceptedCount := 0
	for outcome := range outcomes {
		if outcome.err != nil {
			t.Fatal(outcome.err)
		}
		if outcome.accepted {
			acceptedCount++
			acceptedText = outcome.text
		}
	}
	if acceptedCount != 1 {
		t.Fatalf("concurrent accepted candidates = %d, want exactly one", acceptedCount)
	}
	assertPendingCandidateCapacity(t, repository, aiPendingCandidateCapacity)

	if _, accepted, err := repository.reservePendingDictionaryCandidate(ctx, dictionaryID, "BeyondCapacity"); err != nil || accepted {
		t.Fatalf("candidate above the global cap = accepted:%t err:%v, want false/nil", accepted, err)
	}
	if err := repository.ResolveDictionaryCandidateGovernance(ctx, dictionaryID, acceptedText, DictionaryCandidateGovernanceRejected); err != nil {
		t.Fatal(err)
	}
	if _, accepted, err := repository.reservePendingDictionaryCandidate(ctx, dictionaryID, "ReleasedCapacity"); err != nil || !accepted {
		t.Fatalf("candidate after resolution = accepted:%t err:%v, want true/nil", accepted, err)
	}
	assertPendingCandidateCapacity(t, repository, aiPendingCandidateCapacity)

	var rejectedCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT count(*)
FROM technology_dictionary_candidates
WHERE dictionary_version_id = $1
  AND candidate_text = $2
  AND governance_status = 'rejected'`, dictionaryID, acceptedText).Scan(&rejectedCount); err != nil {
		t.Fatal(err)
	}
	if rejectedCount != 1 {
		t.Fatalf("resolved candidate history count = %d, want 1", rejectedCount)
	}
}

func TestAIPendingCandidateCapacityReusesTheSamePendingTermWithoutAnotherSlot(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	_, dictionaryID, _, _ := insertSyntheticAnalysisPrerequisites(t, repository)

	if _, accepted, err := repository.reservePendingDictionaryCandidate(ctx, dictionaryID, "SharedCandidate"); err != nil || !accepted {
		t.Fatalf("first candidate = accepted:%t err:%v, want true/nil", accepted, err)
	}
	if _, accepted, err := repository.reservePendingDictionaryCandidate(ctx, dictionaryID, "sharedcandidate"); err != nil || !accepted {
		t.Fatalf("case-insensitive reuse = accepted:%t err:%v, want true/nil", accepted, err)
	}
	assertPendingCandidateCapacity(t, repository, 1)

	var storedCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT count(*)
FROM technology_dictionary_candidates
WHERE dictionary_version_id = $1
  AND lower(candidate_text) = lower('SharedCandidate')`, dictionaryID).Scan(&storedCount); err != nil {
		t.Fatal(err)
	}
	if storedCount != 1 {
		t.Fatalf("same pending candidate rows = %d, want 1", storedCount)
	}
}

func assertPendingCandidateCapacity(t *testing.T, repository *Repository, want int) {
	t.Helper()
	var got int
	if err := repository.pool.QueryRow(context.Background(), `
SELECT count(DISTINCT lower(candidate_text))
FROM technology_dictionary_candidates
WHERE governance_status = 'pending_review'`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("pending candidate capacity = %d, want %d", got, want)
	}
}
