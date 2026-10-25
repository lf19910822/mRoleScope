package jdanalysis

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestControlledAIOutputAuditRecordsOnlyImmutableNonTextVerdict(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-output-audit-review-reference")
	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 34)
	prepared, err := repository.PrepareControlledAIOutputAuditDiagnostic(ctx, "llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}

	rawOutput := []byte(`{"response_schema_version":"wrong","synthetic_output":"only-on-cvm"}`)
	processor := AISemanticProcessor{
		Repository:                   repository,
		RequirePersistentTrialBudget: true,
		OutputAudit:                  NewAIOutputAuditStore(t.TempDir()),
		RequireOutputAudit:           true,
		Runner: AIModelAttemptRunner{
			Client:    &fixedAIOutputClient{response: rawOutput},
			Estimator: fixedAIInputTokenEstimator{tokens: 100},
		},
	}
	if _, err := processor.ProcessPending(ctx, prepared.Plus.ID); !errors.Is(err, ErrAISemanticWorkFailed) {
		t.Fatalf("diagnostic processor error = %v, want safely rejected schema", err)
	}

	var screen bytes.Buffer
	reviewed, err := RunControlledAIOutputAuditSession(
		ctx, repository, processor.OutputAudit, []string{prepared.Plus.ID}, "qwen-plus", true,
		strings.NewReader("schema_contract_mismatch\n"), &screen,
	)
	if err != nil || !reviewed {
		t.Fatalf("controlled output-audit session = reviewed:%t err:%v", reviewed, err)
	}
	if !strings.Contains(screen.String(), string(rawOutput)) || !strings.Contains(screen.String(), "reconstructed_input") {
		t.Fatalf("direct-terminal audit omitted raw output or reconstructed input: %q", screen.String())
	}

	var workID, bodyContentVersionID, outputSHA256, reviewer, verdict string
	var attemptNumber int
	if err := repository.pool.QueryRow(ctx, `
SELECT review.work_item_id::text,
       review.body_content_version_id::text,
       review.attempt_number,
       review.output_sha256,
       review.reviewer,
       review.verdict::text
FROM ai_output_audit_reviews AS review
WHERE review.ai_analysis_configuration_id = $1`, prepared.Plus.ID).Scan(
		&workID, &bodyContentVersionID, &attemptNumber, &outputSHA256, &reviewer, &verdict,
	); err != nil {
		t.Fatal(err)
	}
	if reviewer != AIControlledOutputAuditReviewer || verdict != "schema_contract_mismatch" || outputSHA256 != sha256Hex(string(rawOutput)) {
		t.Fatalf("non-text audit record = reviewer:%q verdict:%q output SHA:%q", reviewer, verdict, outputSHA256)
	}
	if err := repository.RecordAIOutputAuditReview(ctx, AIOutputAuditReviewInput{
		AIConfigurationID:    prepared.Plus.ID,
		AttemptNumber:        attemptNumber,
		BodyContentVersionID: bodyContentVersionID,
		OutputSHA256:         outputSHA256,
		Verdict:              "schema_contract_mismatch",
		WorkItemID:           workID,
	}); err == nil {
		t.Fatal("output-audit verdict was recorded more than once")
	}
	if _, err := repository.pool.Exec(ctx, `
UPDATE ai_output_audit_reviews
SET verdict = 'prompt_noncompliance'
WHERE work_item_id = $1 AND attempt_number = $2 AND reviewer = 'FAN'`, workID, attemptNumber); err == nil {
		t.Fatal("output-audit verdict was mutable")
	}
	if _, err := repository.pool.Exec(ctx, `
DELETE FROM ai_output_audit_reviews
WHERE work_item_id = $1 AND attempt_number = $2 AND reviewer = 'FAN'`, workID, attemptNumber); err == nil {
		t.Fatal("output-audit verdict was deletable")
	}

	rows, err := repository.pool.Query(ctx, `
SELECT column_name
FROM information_schema.columns
WHERE table_schema = 'public' AND table_name = 'ai_output_audit_reviews'
ORDER BY column_name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	columns := make([]string, 0)
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	wantColumns := []string{
		"ai_analysis_configuration_id", "attempt_number", "body_content_version_id", "id",
		"output_sha256", "reviewed_at", "reviewer", "verdict", "work_item_id",
	}
	sort.Strings(wantColumns)
	if !equalStrings(columns, wantColumns) {
		t.Fatalf("output-audit review columns = %q, want only binding IDs, output SHA, FAN, fixed verdict, and time", columns)
	}
}

func TestAIOutputAuditPendingStatusCountsOnlyCapturedContractFailures(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-output-audit-status-reference")
	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 34)
	prepared, err := repository.PrepareControlledAIOutputAuditDiagnostic(ctx, "llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	store := NewAIOutputAuditStore(t.TempDir())
	processor := AISemanticProcessor{
		Repository:                   repository,
		RequirePersistentTrialBudget: true,
		OutputAudit:                  store,
		RequireOutputAudit:           true,
		Runner: AIModelAttemptRunner{
			Client:    &fixedAIOutputClient{response: []byte(`{"response_schema_version":"wrong"}`)},
			Estimator: fixedAIInputTokenEstimator{tokens: 100},
		},
	}
	if _, err := processor.ProcessPending(ctx, prepared.Plus.ID); !errors.Is(err, ErrAISemanticWorkFailed) {
		t.Fatalf("diagnostic processor error = %v, want safely rejected schema", err)
	}

	pending, err := repository.ReadAIOutputAuditPendingStatus(ctx, []string{prepared.Plus.ID}, store)
	if err != nil {
		t.Fatal(err)
	}
	if pending.PendingReviewCount != aiOutputAuditDiagnosticSampleSize || len(pending.FailureCounts) != 1 ||
		pending.FailureCounts[0].SafeErrorCode != string(AISafeErrorInvalidResponseSchema) || pending.FailureCounts[0].Count != aiOutputAuditDiagnosticSampleSize {
		t.Fatalf("safe pending output-audit status = %#v", pending)
	}
}

func TestControlledAIOutputAuditSkipsAnExpiredArtifactAndReviewsTheNextCapturedFailure(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-output-audit-expired-selection-reference")
	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 34)
	prepared, err := repository.PrepareControlledAIOutputAuditDiagnostic(ctx, "llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	store := NewAIOutputAuditStore(t.TempDir())
	processor := AISemanticProcessor{
		Repository:                   repository,
		RequirePersistentTrialBudget: true,
		OutputAudit:                  store,
		RequireOutputAudit:           true,
		Runner: AIModelAttemptRunner{
			Client:    &fixedAIOutputClient{response: []byte(`{"response_schema_version":"wrong"}`)},
			Estimator: fixedAIInputTokenEstimator{tokens: 100},
		},
	}
	if _, err := processor.ProcessPending(ctx, prepared.Plus.ID); !errors.Is(err, ErrAISemanticWorkFailed) {
		t.Fatalf("diagnostic processor error = %v, want safely rejected schema", err)
	}

	var workID, bodyContentVersionID string
	var attemptNumber int
	if err := repository.pool.QueryRow(ctx, `
SELECT work.id::text, work.body_content_version_id::text, attempt.attempt_number
FROM ai_analysis_work_items AS work
JOIN ai_analysis_attempts AS attempt
  ON attempt.work_item_id = work.id AND attempt.status = 'succeeded'
WHERE work.ai_analysis_configuration_id = $1
ORDER BY work.completed_at, work.id, attempt.attempt_number
LIMIT 1`, prepared.Plus.ID).Scan(&workID, &bodyContentVersionID, &attemptNumber); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(store.root, prepared.Plus.ID, workID, fmt.Sprintf("attempt-%02d", attemptNumber))
	for _, path := range []string{base + ".response.json", base + aiOutputAuditMetadataFileExtension} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}

	var screen bytes.Buffer
	reviewed, err := RunControlledAIOutputAuditSession(
		ctx, repository, store, []string{prepared.Plus.ID}, "qwen-plus", true,
		strings.NewReader("schema_contract_mismatch\n"), &screen,
	)
	if err != nil || !reviewed {
		t.Fatalf("audit session after first artifact expiry = reviewed:%t err:%v; want next captured failure", reviewed, err)
	}
	var reviewedWorkID string
	if err := repository.pool.QueryRow(ctx, `
SELECT work_item_id::text
FROM ai_output_audit_reviews
WHERE ai_analysis_configuration_id = $1`, prepared.Plus.ID).Scan(&reviewedWorkID); err != nil {
		t.Fatal(err)
	}
	if reviewedWorkID == workID {
		t.Fatal("audit recorded a review for the expired artifact instead of a retained artifact")
	}
	if bodyContentVersionID == "" {
		t.Fatal("expired artifact fixture did not bind a body version")
	}
}
