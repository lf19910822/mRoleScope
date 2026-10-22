package jdanalysis

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestProcessorConsumesPostgreSQLQueueAndRetriesFailures(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, _, _ := insertSyntheticAnalysisPrerequisites(t, repository)
	configurationID := "00000000-0000-4000-8000-000000000506"
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO technology_analysis_configurations (
    id, parser_version, dictionary_version_id, created_by
) VALUES ($1, 'synthetic-parser-v1', $2, 'test')`, configurationID, dictionaryID); err != nil {
		t.Fatal(err)
	}

	inserted, err := repository.EnqueueBody(ctx, configurationID, bodyID, TriggerNewBody)
	if err != nil || !inserted {
		t.Fatalf("first enqueue = inserted:%t err:%v", inserted, err)
	}
	inserted, err = repository.EnqueueBody(ctx, configurationID, bodyID, TriggerNewBody)
	if err != nil || inserted {
		t.Fatalf("duplicate enqueue = inserted:%t err:%v", inserted, err)
	}

	release, acquired, err := repository.TryAcquireConfigurationLock(ctx, configurationID)
	if err != nil || !acquired {
		t.Fatalf("acquire configuration lock = acquired:%t err:%v", acquired, err)
	}
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, configurationID); !errors.Is(err, ErrConfigurationBusy) {
		t.Fatalf("process while lock is held error = %v, want ErrConfigurationBusy", err)
	}
	release()

	first, err := (Processor{Repository: repository}).ProcessPending(ctx, configurationID)
	if err != nil {
		t.Fatal(err)
	}
	if first.Claimed != 1 || first.Completed != 1 || first.Failed != 0 {
		t.Fatalf("first processing summary = %#v", first)
	}
	assertAnalysisWork(t, repository, bodyID, configurationID, "complete", 1, "")
	assertAnalysisRun(t, repository, first.RunID, "complete", 1, 1, 0, "")
	var resultCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT count(*)
FROM technology_analysis_results
WHERE body_content_version_id = $1 AND parser_version = 'synthetic-parser-v1' AND dictionary_version_id = $2`, bodyID, dictionaryID).Scan(&resultCount); err != nil {
		t.Fatal(err)
	}
	if resultCount != 1 {
		t.Fatalf("analysis result count = %d, want 1", resultCount)
	}

	failingBodyID := insertSyntheticAnalysisBody(t, repository, "00000000-0000-4000-8000-000000000507", "Go")
	inserted, err = repository.EnqueueBody(ctx, configurationID, failingBodyID, TriggerNewBody)
	if err != nil || !inserted {
		t.Fatalf("enqueue failing body = inserted:%t err:%v", inserted, err)
	}
	failingProcessor := Processor{
		Repository: repository,
		Publish: func(context.Context, ResultIdentity, []MentionInput) (PublishResult, error) {
			return PublishResult{}, fmt.Errorf("password=synthetic-secret <p>synthetic JD body</p> token=synthetic-token")
		},
	}
	failed, err := failingProcessor.ProcessPending(ctx, configurationID)
	if !errors.Is(err, ErrAnalysisWorkFailed) {
		t.Fatalf("failed processing error = %v, want ErrAnalysisWorkFailed", err)
	}
	if failed.Claimed != 1 || failed.Completed != 0 || failed.Failed != 1 {
		t.Fatalf("failed processing summary = %#v", failed)
	}
	assertAnalysisWork(t, repository, failingBodyID, configurationID, "pending", 1, "analysis_result_failed")
	assertAnalysisRun(t, repository, failed.RunID, "failed", 1, 0, 1, "analysis_work_failed")
	var auditCode, auditMessage string
	if err := repository.pool.QueryRow(ctx, `
SELECT code, message
FROM technology_analysis_audit_events
WHERE run_id = $1 AND severity = 'error'`, failed.RunID).Scan(&auditCode, &auditMessage); err != nil {
		t.Fatal(err)
	}
	if auditCode != "analysis_work_failed" || auditMessage != "analysis work failed safely" {
		t.Fatalf("unsafe failure audit = code:%q message:%q", auditCode, auditMessage)
	}

	retried, err := (Processor{Repository: repository}).ProcessPending(ctx, configurationID)
	if err != nil {
		t.Fatal(err)
	}
	if retried.Claimed != 1 || retried.Completed != 1 || retried.Failed != 0 {
		t.Fatalf("retry processing summary = %#v", retried)
	}
	assertAnalysisWork(t, repository, failingBodyID, configurationID, "complete", 2, "")
}

func TestCandidateConfigurationReanalysisCatchesUpAndPublishesAtomically(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, _, _ := insertSyntheticAnalysisPrerequisites(t, repository)

	oldPreparation, err := repository.PrepareCandidateConfiguration(ctx, CandidateConfigurationInput{
		ParserVersion: "synthetic-parser-old", DictionaryVersionID: dictionaryID, CreatedBy: "test",
	})
	if err != nil || oldPreparation.Queued != 1 {
		t.Fatalf("prepare initial configuration = %#v err=%v", oldPreparation, err)
	}
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, oldPreparation.ConfigurationID); err != nil {
		t.Fatal(err)
	}
	oldPromotion, err := repository.PromoteCandidateConfiguration(ctx, oldPreparation.ConfigurationID)
	if err != nil || !oldPromotion.Published || oldPromotion.Reused {
		t.Fatalf("promote initial configuration = %#v err=%v", oldPromotion, err)
	}
	assertPublishedConfiguration(t, repository, oldPreparation.ConfigurationID, "published")

	candidate, err := repository.PrepareCandidateConfiguration(ctx, CandidateConfigurationInput{
		ParserVersion: "synthetic-parser-next", DictionaryVersionID: dictionaryID, CreatedBy: "test",
	})
	if err != nil || candidate.Queued != 1 || candidate.Reused {
		t.Fatalf("prepare candidate configuration = %#v err=%v", candidate, err)
	}
	repeated, err := repository.PrepareCandidateConfiguration(ctx, CandidateConfigurationInput{
		ParserVersion: "synthetic-parser-next", DictionaryVersionID: dictionaryID, CreatedBy: "test",
	})
	if err != nil || repeated.ConfigurationID != candidate.ConfigurationID || repeated.Queued != 0 {
		t.Fatalf("repeat candidate preparation = %#v err=%v", repeated, err)
	}
	notReady, err := repository.PromoteCandidateConfiguration(ctx, candidate.ConfigurationID)
	if !errors.Is(err, ErrCandidateNotReady) || notReady.Pending != 1 || notReady.MissingResults != 1 || notReady.Published {
		t.Fatalf("premature candidate promotion = %#v err=%v", notReady, err)
	}
	assertPublishedConfiguration(t, repository, oldPreparation.ConfigurationID, "published")
	assertConfigurationStatus(t, repository, candidate.ConfigurationID, "candidate")
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, candidate.ConfigurationID); err != nil {
		t.Fatal(err)
	}

	failedCandidate, err := repository.PrepareCandidateConfiguration(ctx, CandidateConfigurationInput{
		ParserVersion: "synthetic-parser-failed", DictionaryVersionID: dictionaryID, CreatedBy: "test",
	})
	if err != nil || failedCandidate.Queued != 1 {
		t.Fatalf("prepare failed candidate = %#v err=%v", failedCandidate, err)
	}
	if err := repository.FailCandidateConfiguration(ctx, failedCandidate.ConfigurationID, "release_validation_failed"); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.PromoteCandidateConfiguration(ctx, failedCandidate.ConfigurationID); !errors.Is(err, ErrCandidateFailed) {
		t.Fatalf("failed candidate promotion error = %v, want ErrCandidateFailed", err)
	}
	assertConfigurationStatus(t, repository, failedCandidate.ConfigurationID, "failed")
	assertPublishedConfiguration(t, repository, oldPreparation.ConfigurationID, "published")

	newBodyID := insertSyntheticAnalysisBody(t, repository, "00000000-0000-4000-8000-000000000508", "Go")
	catchUp, err := repository.PromoteCandidateConfiguration(ctx, candidate.ConfigurationID)
	if !errors.Is(err, ErrCandidateNotReady) || catchUp.Queued != 1 || catchUp.Pending != 1 || catchUp.MissingResults != 1 {
		t.Fatalf("candidate catch-up promotion = %#v err=%v", catchUp, err)
	}
	assertAnalysisWork(t, repository, newBodyID, candidate.ConfigurationID, "pending", 0, "")
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, candidate.ConfigurationID); err != nil {
		t.Fatal(err)
	}
	promotion, err := repository.PromoteCandidateConfiguration(ctx, candidate.ConfigurationID)
	if err != nil || !promotion.Published || promotion.Reused || promotion.Pending != 0 || promotion.MissingResults != 0 {
		t.Fatalf("candidate promotion = %#v err=%v", promotion, err)
	}
	assertPublishedConfiguration(t, repository, candidate.ConfigurationID, "published")
	assertConfigurationStatus(t, repository, oldPreparation.ConfigurationID, "superseded")
	reusedPromotion, err := repository.PromoteCandidateConfiguration(ctx, candidate.ConfigurationID)
	if err != nil || !reusedPromotion.Published || !reusedPromotion.Reused {
		t.Fatalf("reused candidate promotion = %#v err=%v", reusedPromotion, err)
	}
	if bodyID == newBodyID {
		t.Fatal("candidate reanalysis did not use a distinct synthetic body version")
	}
}

func TestReviewRecordsAreIndependentAndBoundToExactEvidence(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	first, err := repository.PublishCompleteResult(ctx, ResultIdentity{
		BodyContentVersionID: bodyID, ParserVersion: "synthetic-review-parser", DictionaryVersionID: dictionaryID,
	}, []MentionInput{{
		TechnologyIdentityID: technologyID,
		Evidence: []EvidenceInput{
			{SourceField: SourceFieldDescription, RuleID: ruleID, Start: 0, End: 2},
			{SourceField: SourceFieldDescription, RuleID: ruleID, Start: 4, End: 6},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var firstEvidenceID string
	if err := repository.pool.QueryRow(ctx, `
SELECT evidence.id::text
FROM technology_mention_evidence AS evidence
JOIN technology_mentions AS mention ON mention.id = evidence.technology_mention_id
WHERE mention.analysis_result_id = $1
ORDER BY evidence.codepoint_start
LIMIT 1`, first.ResultID).Scan(&firstEvidenceID); err != nil {
		t.Fatal(err)
	}
	remoteEvidence, err := repository.LoadRemoteReviewEvidence(ctx, first.ResultID, firstEvidenceID)
	if err != nil {
		t.Fatal(err)
	}
	if remoteEvidence.SourceField != SourceFieldDescription || remoteEvidence.MatchedAlias != "Go" || remoteEvidence.CodePointStart != 0 || remoteEvidence.CodePointEnd != 2 || remoteEvidence.NormalizedFieldText != "Go, Go" {
		t.Fatalf("controlled review evidence = %#v", remoteEvidence)
	}
	reviewID, err := repository.RecordReview(ctx, ReviewInput{
		AnalysisResultID: first.ResultID, TechnologyMentionEvidenceID: firstEvidenceID,
		Verdict: ReviewIncorrect, ErrorCategory: "synthetic_boundary_case", Reviewer: "remote-reviewer",
	})
	if err != nil || reviewID == "" {
		t.Fatalf("record review id=%q err=%v", reviewID, err)
	}
	reviews, err := repository.ListReviewsForResult(ctx, first.ResultID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reviews) != 1 || reviews[0].Verdict != ReviewIncorrect || reviews[0].ErrorCategory != "synthetic_boundary_case" || reviews[0].Reviewer != "remote-reviewer" {
		t.Fatalf("reviews = %#v", reviews)
	}
	var firstMentionCount, firstEvidenceCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM technology_mentions WHERE analysis_result_id = $1),
  (SELECT count(*) FROM technology_mention_evidence AS evidence JOIN technology_mentions AS mention ON mention.id = evidence.technology_mention_id WHERE mention.analysis_result_id = $1)`, first.ResultID).Scan(&firstMentionCount, &firstEvidenceCount); err != nil {
		t.Fatal(err)
	}
	if firstMentionCount != 1 || firstEvidenceCount != 2 {
		t.Fatalf("review modified automatic result = mentions:%d evidence:%d", firstMentionCount, firstEvidenceCount)
	}

	secondBodyID := insertSyntheticAnalysisBody(t, repository, "00000000-0000-4000-8000-000000000509", "Go")
	second, err := repository.PublishCompleteResult(ctx, ResultIdentity{
		BodyContentVersionID: secondBodyID, ParserVersion: "synthetic-review-parser", DictionaryVersionID: dictionaryID,
	}, []MentionInput{{
		TechnologyIdentityID: technologyID,
		Evidence:             []EvidenceInput{{SourceField: SourceFieldDescription, RuleID: ruleID, Start: 0, End: 2}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	secondReviews, err := repository.ListReviewsForResult(ctx, second.ResultID)
	if err != nil {
		t.Fatal(err)
	}
	if len(secondReviews) != 0 {
		t.Fatalf("review inherited by a different body result: %#v", secondReviews)
	}
	if _, err := repository.LoadRemoteReviewEvidence(ctx, second.ResultID, firstEvidenceID); err == nil {
		t.Fatal("review query accepted evidence from another result")
	}
	if _, err := repository.RecordReview(ctx, ReviewInput{
		AnalysisResultID: second.ResultID, TechnologyMentionEvidenceID: firstEvidenceID,
		Verdict: ReviewCorrect, Reviewer: "remote-reviewer",
	}); err == nil {
		t.Fatal("review record accepted evidence from another result")
	}
}

func insertSyntheticAnalysisBody(t *testing.T, repository *Repository, bodyID, description string) string {
	t.Helper()
	const jobID = "00000000-0000-4000-8000-000000000501"
	if _, err := repository.pool.Exec(context.Background(), `
INSERT INTO job_body_content_versions (
    id, job_id, body_sha256, normalization_version,
    description_text, job_require_text, addition_text,
    description_sha256, job_require_sha256, addition_sha256
) VALUES ($1, $2, $3, 'jd-body-v1', $4, '', '', $5, $6, $7)`,
		bodyID, jobID, testSHA256(description+"||"), description,
		testSHA256(description), testSHA256(""), testSHA256("")); err != nil {
		t.Fatal(err)
	}
	return bodyID
}

func assertPublishedConfiguration(t *testing.T, repository *Repository, configurationID, wantStatus string) {
	t.Helper()
	var currentID, status string
	if err := repository.pool.QueryRow(context.Background(), `
SELECT publication.configuration_id::text, configuration.status::text
FROM technology_analysis_publication_state AS publication
JOIN technology_analysis_configurations AS configuration ON configuration.id = publication.configuration_id
WHERE publication.singleton = true`).Scan(&currentID, &status); err != nil {
		t.Fatal(err)
	}
	if currentID != configurationID || status != wantStatus {
		t.Fatalf("published configuration = id:%q status:%q, want id:%q status:%q", currentID, status, configurationID, wantStatus)
	}
}

func assertConfigurationStatus(t *testing.T, repository *Repository, configurationID, wantStatus string) {
	t.Helper()
	var status string
	if err := repository.pool.QueryRow(context.Background(), `
SELECT status::text FROM technology_analysis_configurations WHERE id = $1`, configurationID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != wantStatus {
		t.Fatalf("configuration status = %q, want %q", status, wantStatus)
	}
}

func assertAnalysisWork(t *testing.T, repository *Repository, bodyID, configurationID, wantStatus string, wantAttempts int, wantSafeCode string) {
	t.Helper()
	var status, safeCode string
	var attempts int
	if err := repository.pool.QueryRow(context.Background(), `
SELECT status::text, attempt_count, COALESCE(last_safe_error_code, '')
FROM technology_analysis_work_items
WHERE body_content_version_id = $1 AND configuration_id = $2`, bodyID, configurationID).Scan(&status, &attempts, &safeCode); err != nil {
		t.Fatal(err)
	}
	if status != wantStatus || attempts != wantAttempts || safeCode != wantSafeCode {
		t.Fatalf("analysis work = status:%q attempts:%d safeCode:%q, want status:%q attempts:%d safeCode:%q", status, attempts, safeCode, wantStatus, wantAttempts, wantSafeCode)
	}
}

func assertAnalysisRun(t *testing.T, repository *Repository, runID, wantStatus string, wantClaimed, wantCompleted, wantFailed int, wantSafeCode string) {
	t.Helper()
	var status, safeCode string
	var claimed, completed, failed int
	if err := repository.pool.QueryRow(context.Background(), `
SELECT status::text, claimed_count, completed_count, failed_count, COALESCE(safe_failure_code, '')
FROM technology_analysis_runs
WHERE id = $1`, runID).Scan(&status, &claimed, &completed, &failed, &safeCode); err != nil {
		t.Fatal(err)
	}
	if status != wantStatus || claimed != wantClaimed || completed != wantCompleted || failed != wantFailed || safeCode != wantSafeCode {
		t.Fatalf("analysis run = status:%q counts:%d/%d/%d safeCode:%q, want status:%q counts:%d/%d/%d safeCode:%q", status, claimed, completed, failed, safeCode, wantStatus, wantClaimed, wantCompleted, wantFailed, wantSafeCode)
	}
}
