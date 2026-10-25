package jdanalysis

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAISemanticProcessorCapturesProviderOutputBeforeRejectingInvalidContract(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-output-audit-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-output-audit-candidate"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.EnsureAISemanticResult(ctx, bodyID, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_analysis_work_items (ai_analysis_configuration_id, body_content_version_id, trigger)
VALUES ($1, $2, 'trial')`, aiConfiguration.ID, bodyID); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	client := &fixedAIOutputClient{response: []byte(`{"response_schema_version":"wrong"}`)}
	processor := AISemanticProcessor{
		Repository:         repository,
		OutputAudit:        NewAIOutputAuditStore(root),
		RequireOutputAudit: true,
		Runner:             AIModelAttemptRunner{Client: client},
	}
	summary, err := processor.ProcessPending(ctx, aiConfiguration.ID)
	if !errors.Is(err, ErrAISemanticWorkFailed) || summary.Failed != 1 || client.calls != 1 {
		t.Fatalf("ProcessPending() = summary:%#v err:%v calls:%d, want one captured safe failure", summary, err, client.calls)
	}
	assertAISemanticWorkState(t, repository, aiConfiguration.ID, bodyID, "failed_safely", "failed_safely", "not_analyzed")
	paths, err := filepath.Glob(filepath.Join(root, "*", "*", "*.response.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("captured response paths = %v, %v; want exactly one", paths, err)
	}
	captured, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(captured, client.response) {
		t.Fatalf("captured response = %q, want %q", captured, client.response)
	}
	var safeCode string
	if err := repository.pool.QueryRow(ctx, `
SELECT last_safe_error_code::text
FROM ai_analysis_work_items
WHERE ai_analysis_configuration_id = $1 AND body_content_version_id = $2`, aiConfiguration.ID, bodyID).Scan(&safeCode); err != nil {
		t.Fatal(err)
	}
	if safeCode != string(AISafeErrorInvalidResponseSchema) {
		t.Fatalf("safe error code = %q, want %q", safeCode, AISafeErrorInvalidResponseSchema)
	}
}

func TestAISemanticProcessorStopsBeforeProviderWhenAuditCapacityCannotBeReserved(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-output-audit-capacity-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-output-audit-capacity-candidate"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.EnsureAISemanticResult(ctx, bodyID, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_analysis_work_items (ai_analysis_configuration_id, body_content_version_id, trigger)
VALUES ($1, $2, 'trial')`, aiConfiguration.ID, bodyID); err != nil {
		t.Fatal(err)
	}

	client := &fixedAIOutputClient{response: []byte(`{"ignored":true}`)}
	processor := AISemanticProcessor{
		Repository: repository,
		OutputAudit: newAIOutputAuditStore(t.TempDir(), aiOutputAuditPolicy{
			CapacityBytes: aiOutputAuditReservationBytes - 1,
		}),
		RequireOutputAudit: true,
		Runner:             AIModelAttemptRunner{Client: client},
	}
	summary, err := processor.ProcessPending(ctx, aiConfiguration.ID)
	if !errors.Is(err, ErrAISemanticWorkFailed) || summary.Failed != 1 || client.calls != 0 {
		t.Fatalf("ProcessPending() = summary:%#v err:%v calls:%d, want pre-send audit failure", summary, err, client.calls)
	}
	var safeCode string
	if err := repository.pool.QueryRow(ctx, `
SELECT last_safe_error_code::text
FROM ai_analysis_work_items
WHERE ai_analysis_configuration_id = $1 AND body_content_version_id = $2`, aiConfiguration.ID, bodyID).Scan(&safeCode); err != nil {
		t.Fatal(err)
	}
	if safeCode != string(AISafeErrorOutputAuditCapture) {
		t.Fatalf("safe error code = %q, want %q", safeCode, AISafeErrorOutputAuditCapture)
	}
}

func TestAISemanticProcessorDoesNotCapturePublishedProviderResponse(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-output-audit-published-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-output-audit-published"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
UPDATE ai_analysis_configurations
SET status = 'published', published_at = now()
WHERE id = $1`, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.EnsureAISemanticResult(ctx, bodyID, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_analysis_work_items (ai_analysis_configuration_id, body_content_version_id, trigger)
VALUES ($1, $2, 'new_body')`, aiConfiguration.ID, bodyID); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	client := &fixedAIOutputClient{response: []byte(`{"response_schema_version":"wrong"}`)}
	processor := AISemanticProcessor{
		Repository:         repository,
		OutputAudit:        NewAIOutputAuditStore(root),
		RequireOutputAudit: true,
		Runner:             AIModelAttemptRunner{Client: client},
	}
	if _, err := processor.ProcessPending(ctx, aiConfiguration.ID); !errors.Is(err, ErrAISemanticWorkFailed) || client.calls != 1 {
		t.Fatalf("ProcessPending() error = %v, calls = %d; want published provider response without audit capture", err, client.calls)
	}
	paths, err := filepath.Glob(filepath.Join(root, "*", "*", "*.response.json"))
	if err != nil || len(paths) != 0 {
		t.Fatalf("published configuration captured response paths = %v, %v; want none", paths, err)
	}
	var safeCode string
	if err := repository.pool.QueryRow(ctx, `
SELECT last_safe_error_code::text
FROM ai_analysis_work_items
WHERE ai_analysis_configuration_id = $1 AND body_content_version_id = $2`, aiConfiguration.ID, bodyID).Scan(&safeCode); err != nil {
		t.Fatal(err)
	}
	if safeCode != string(AISafeErrorInvalidResponseSchema) {
		t.Fatalf("safe error code = %q, want %q", safeCode, AISafeErrorInvalidResponseSchema)
	}
}

func TestAISemanticProcessorDoesNotRetryProviderAfterAuditCaptureFailure(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-output-audit-capture-failure-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-output-audit-capture-failure"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.EnsureAISemanticResult(ctx, bodyID, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}
	var workID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_analysis_work_items (ai_analysis_configuration_id, body_content_version_id, trigger)
VALUES ($1, $2, 'trial')
RETURNING id::text`, aiConfiguration.ID, bodyID).Scan(&workID); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	store := NewAIOutputAuditStore(root)
	reservation, err := store.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reservation.Capture(AIOutputAuditIdentity{
		ConfigurationID:      aiConfiguration.ID,
		WorkItemID:           workID,
		AttemptNumber:        1,
		BodyContentVersionID: bodyID,
	}, []byte(`{"captured":"collision"}`)); err != nil {
		t.Fatal(err)
	}
	client := &fixedAIOutputClient{response: []byte(`{"response_schema_version":"wrong"}`)}
	processor := AISemanticProcessor{
		Repository:         repository,
		OutputAudit:        store,
		RequireOutputAudit: true,
		Runner:             AIModelAttemptRunner{Client: client},
	}
	if _, err := processor.ProcessPending(ctx, aiConfiguration.ID); !errors.Is(err, ErrAISemanticWorkFailed) || client.calls != 1 {
		t.Fatalf("first ProcessPending() error = %v, calls = %d; want one provider call then safe capture failure", err, client.calls)
	}
	if summary, err := processor.ProcessPending(ctx, aiConfiguration.ID); err != nil || summary.Claimed != 0 || client.calls != 1 {
		t.Fatalf("second ProcessPending() = summary:%#v err:%v calls:%d; want no retry", summary, err, client.calls)
	}
	var safeCode string
	if err := repository.pool.QueryRow(ctx, `
SELECT last_safe_error_code::text
FROM ai_analysis_work_items
WHERE id = $1`, workID).Scan(&safeCode); err != nil {
		t.Fatal(err)
	}
	if safeCode != string(AISafeErrorOutputAuditCapture) {
		t.Fatalf("safe error code = %q, want %q", safeCode, AISafeErrorOutputAuditCapture)
	}
}

type fixedAIOutputClient struct {
	calls    int
	response []byte
}

func (client *fixedAIOutputClient) Analyze(context.Context, AIModelInput) (AIModelResponse, error) {
	client.calls++
	return AIModelResponse{Content: client.response, Usage: AIModelUsage{InputTokens: 10, OutputTokens: 10, TotalTokens: 20}}, nil
}
