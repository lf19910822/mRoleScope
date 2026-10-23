package jdanalysis

import (
	"context"
	"errors"
	"testing"
)

func TestReleaseV02BaselinePublishesFrozenDictionaryAndConfigurationTogether(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	_, _, _, _ = insertSyntheticAnalysisPrerequisites(t, repository)

	configurationID, dictionaryID := prepareCompleteV02ReleaseCandidate(t, repository)
	first, err := repository.ReleaseV02Baseline(ctx, "github-reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if first.ConfigurationID != configurationID || first.DictionaryVersionID != dictionaryID ||
		!first.Published || first.Reused || first.Publisher != "github-reviewer" ||
		first.ReleaseGateVersion != V02ReleaseGateVersion {
		t.Fatalf("first v0.2 release = %#v", first)
	}
	assertPublishedConfiguration(t, repository, configurationID, "published")
	assertDictionaryStatus(t, repository, dictionaryID, "published")
	assertReleaseEvent(t, repository, configurationID, dictionaryID, "published", "", "github-reviewer", 1)

	second, err := repository.ReleaseV02Baseline(ctx, "other-reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if !second.Published || !second.Reused || second.ConfigurationID != configurationID || second.Publisher != "github-reviewer" {
		t.Fatalf("reused v0.2 release = %#v", second)
	}
	assertReleaseEvent(t, repository, configurationID, dictionaryID, "published", "", "github-reviewer", 1)

	if _, err := repository.pool.Exec(ctx, `
UPDATE technology_dictionary_rules
SET alias = 'synthetic-mutated-alias'
WHERE dictionary_version_id = $1`, dictionaryID); err == nil {
		t.Fatal("published dictionary rule mutation was accepted")
	}
	if _, err := repository.pool.Exec(ctx, `
DELETE FROM technology_analysis_configuration_rules
WHERE configuration_id = $1`, configurationID); err == nil {
		t.Fatal("published analysis configuration rule-set mutation was accepted")
	}
}

func TestReleaseV02BaselineRejectsIncompleteCandidateWithoutChangingPublication(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	_, _, _, _ = insertSyntheticAnalysisPrerequisites(t, repository)

	seed := CandidateV02Seed()
	imported, err := repository.ImportCandidateDictionary(ctx, seed, "remote-operator")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ApproveAllV02ReviewedRules(ctx, "remote-reviewer"); err != nil {
		t.Fatal(err)
	}
	prepared, err := repository.PrepareCandidateConfigurationForDictionary(ctx, seed.VersionLabel, V01ParserVersion, "remote-operator")
	if err != nil {
		t.Fatal(err)
	}

	result, err := repository.ReleaseV02Baseline(ctx, "github-reviewer")
	if !errors.Is(err, ErrCandidateNotReady) {
		t.Fatalf("incomplete v0.2 release = %#v err=%v, want ErrCandidateNotReady", result, err)
	}
	assertConfigurationStatus(t, repository, prepared.ConfigurationID, "candidate")
	assertDictionaryStatus(t, repository, imported.DictionaryVersionID, "candidate")
	assertNoPublishedConfiguration(t, repository)
	assertReleaseEvent(t, repository, prepared.ConfigurationID, imported.DictionaryVersionID, "published", "", "", 0)
}

func TestReleaseV02BaselineRequiresTheCompleteReviewedRulePolicy(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	_, _, _, _ = insertSyntheticAnalysisPrerequisites(t, repository)

	seed := CandidateV02Seed()
	imported, err := repository.ImportCandidateDictionary(ctx, seed, "remote-operator")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ApproveV02ReviewedRule(ctx, V02ApprovedRuleKeys()[0], "remote-reviewer"); err != nil {
		t.Fatal(err)
	}
	prepared, err := repository.PrepareCandidateConfigurationForDictionary(ctx, seed.VersionLabel, V01ParserVersion, "remote-operator")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, prepared.ConfigurationID); err != nil {
		t.Fatal(err)
	}

	if _, err := repository.ReleaseV02Baseline(ctx, "github-reviewer"); !errors.Is(err, ErrReleaseGateFailed) {
		t.Fatalf("partial-review v0.2 release error = %v, want ErrReleaseGateFailed", err)
	}
	assertConfigurationStatus(t, repository, prepared.ConfigurationID, "candidate")
	assertDictionaryStatus(t, repository, imported.DictionaryVersionID, "candidate")
	assertNoPublishedConfiguration(t, repository)
}

func TestWithdrawV02BaselineRetiresOnlyThePublishedBaselineAndPreservesHistory(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	_, _, _, _ = insertSyntheticAnalysisPrerequisites(t, repository)

	configurationID, dictionaryID := prepareCompleteV02ReleaseCandidate(t, repository)
	if _, err := repository.ReleaseV02Baseline(ctx, ""); err != nil {
		t.Fatal(err)
	}
	var resultsBefore, evidenceBefore int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM technology_analysis_results WHERE configuration_id = $1),
  (SELECT count(*)
   FROM technology_mention_evidence AS evidence
   JOIN technology_mentions AS mention ON mention.id = evidence.technology_mention_id
   JOIN technology_analysis_results AS result ON result.id = mention.analysis_result_id
   WHERE result.configuration_id = $1)`, configurationID).Scan(&resultsBefore, &evidenceBefore); err != nil {
		t.Fatal(err)
	}

	if _, err := repository.WithdrawV02Baseline(ctx, "operator", WithdrawalCategory("ordinary_analysis_failure")); !errors.Is(err, ErrWithdrawalCategory) {
		t.Fatalf("ordinary failure withdrawal error = %v, want ErrWithdrawalCategory", err)
	}
	assertPublishedConfiguration(t, repository, configurationID, "published")
	assertDictionaryStatus(t, repository, dictionaryID, "published")

	withdrawal, err := repository.WithdrawV02Baseline(ctx, "", WithdrawalMaterialResultQualityDefect)
	if err != nil {
		t.Fatal(err)
	}
	if !withdrawal.Withdrawn || withdrawal.Reused || withdrawal.ConfigurationID != configurationID ||
		withdrawal.DictionaryVersionID != dictionaryID || withdrawal.Publisher != FallbackReleasePublisher ||
		withdrawal.Category != WithdrawalMaterialResultQualityDefect {
		t.Fatalf("v0.2 withdrawal = %#v", withdrawal)
	}
	assertNoPublishedConfiguration(t, repository)
	assertConfigurationStatus(t, repository, configurationID, "retired")
	assertDictionaryStatus(t, repository, dictionaryID, "retired")
	assertReleaseEvent(t, repository, configurationID, dictionaryID, "withdrawn", string(WithdrawalMaterialResultQualityDefect), FallbackReleasePublisher, 1)

	if _, err := repository.PublishedConfigurationID(ctx); !errors.Is(err, ErrNoPublishedConfiguration) {
		t.Fatalf("published configuration after withdrawal error = %v, want ErrNoPublishedConfiguration", err)
	}
	var resultsAfter, evidenceAfter int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM technology_analysis_results WHERE configuration_id = $1),
  (SELECT count(*)
   FROM technology_mention_evidence AS evidence
   JOIN technology_mentions AS mention ON mention.id = evidence.technology_mention_id
   JOIN technology_analysis_results AS result ON result.id = mention.analysis_result_id
   WHERE result.configuration_id = $1)`, configurationID).Scan(&resultsAfter, &evidenceAfter); err != nil {
		t.Fatal(err)
	}
	if resultsAfter != resultsBefore || evidenceAfter != evidenceBefore {
		t.Fatalf("withdrawal changed historical automatic output: results %d/%d evidence %d/%d", resultsBefore, resultsAfter, evidenceBefore, evidenceAfter)
	}
	if _, err := repository.WithdrawV02Baseline(ctx, "operator", WithdrawalSecurityPrivacyIncident); !errors.Is(err, ErrBaselineWithdrawalState) {
		t.Fatalf("repeated v0.2 withdrawal error = %v, want ErrBaselineWithdrawalState", err)
	}
}

func prepareCompleteV02ReleaseCandidate(t *testing.T, repository *Repository) (configurationID, dictionaryID string) {
	t.Helper()
	ctx := context.Background()
	seed := CandidateV02Seed()
	imported, err := repository.ImportCandidateDictionary(ctx, seed, "remote-operator")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ApproveAllV02ReviewedRules(ctx, "remote-reviewer"); err != nil {
		t.Fatal(err)
	}
	prepared, err := repository.PrepareCandidateConfigurationForDictionary(ctx, seed.VersionLabel, V01ParserVersion, "remote-operator")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, prepared.ConfigurationID); err != nil {
		t.Fatal(err)
	}
	return prepared.ConfigurationID, imported.DictionaryVersionID
}

func assertDictionaryStatus(t *testing.T, repository *Repository, dictionaryID, wantStatus string) {
	t.Helper()
	var status string
	if err := repository.pool.QueryRow(context.Background(), `
SELECT status::text
FROM technology_dictionary_versions
WHERE id = $1`, dictionaryID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != wantStatus {
		t.Fatalf("dictionary status = %q, want %q", status, wantStatus)
	}
}

func assertNoPublishedConfiguration(t *testing.T, repository *Repository) {
	t.Helper()
	var configurationID *string
	if err := repository.pool.QueryRow(context.Background(), `
SELECT configuration_id::text
FROM technology_analysis_publication_state
WHERE singleton = true`).Scan(&configurationID); err != nil {
		t.Fatal(err)
	}
	if configurationID != nil {
		t.Fatalf("unexpected published configuration %q", *configurationID)
	}
}

func assertReleaseEvent(t *testing.T, repository *Repository, configurationID, dictionaryID, eventType, withdrawalCategory, publisher string, wantCount int) {
	t.Helper()
	var count int
	if err := repository.pool.QueryRow(context.Background(), `
SELECT count(*)
FROM technology_analysis_release_events
WHERE configuration_id = $1
  AND dictionary_version_id = $2
  AND event_type::text = $3
  AND COALESCE(withdrawal_category::text, '') = $4
  AND ($5 = '' OR publisher = $5)`, configurationID, dictionaryID, eventType, withdrawalCategory, publisher).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != wantCount {
		t.Fatalf("release event count = %d, want %d", count, wantCount)
	}
}
