package jdanalysis

import (
	"context"
	"strings"
	"testing"
)

func TestCandidateDictionaryLifecycleRequiresExplicitRuleApproval(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	_, _, _, _ = insertSyntheticAnalysisPrerequisites(t, repository)

	seed := CandidateV01Seed()
	first, err := repository.ImportCandidateDictionary(ctx, seed, "remote-operator")
	if err != nil {
		t.Fatal(err)
	}
	if first.DictionaryVersionID == "" || first.Reused || first.RulesCreated != len(seed.Rules) || first.CandidatesCreated != len(seed.Candidates) {
		t.Fatalf("first candidate dictionary import = %#v", first)
	}
	second, err := repository.ImportCandidateDictionary(ctx, seed, "remote-operator")
	if err != nil {
		t.Fatal(err)
	}
	if !second.Reused || second.DictionaryVersionID != first.DictionaryVersionID || second.RulesCreated != 0 || second.CandidatesCreated != 0 {
		t.Fatalf("repeated candidate dictionary import = %#v", second)
	}

	if _, err := repository.PrepareCandidateConfigurationForDictionary(ctx, seed.VersionLabel, V01ParserVersion, "remote-operator"); err == nil || !strings.Contains(err.Error(), "no approved deterministic rules") {
		t.Fatalf("candidate preparation without an approval error = %v", err)
	}
	if _, err := repository.ApproveCandidateRule(ctx, seed.VersionLabel, "go.01", "remote-reviewer"); err == nil || !strings.Contains(err.Error(), "context-aware policy") {
		t.Fatalf("conditional rule approval error = %v", err)
	}

	approval, err := repository.ApproveCandidateRule(ctx, seed.VersionLabel, "kubernetes.01", "remote-reviewer")
	if err != nil || approval.Reused || approval.RuleKey != "kubernetes.01" {
		t.Fatalf("candidate rule approval = %#v err=%v", approval, err)
	}
	repeatedApproval, err := repository.ApproveCandidateRule(ctx, seed.VersionLabel, "kubernetes.01", "other-reviewer")
	if err != nil || !repeatedApproval.Reused {
		t.Fatalf("repeated candidate rule approval = %#v err=%v", repeatedApproval, err)
	}

	prepared, err := repository.PrepareCandidateConfigurationForDictionary(ctx, seed.VersionLabel, V01ParserVersion, "remote-operator")
	if err != nil || prepared.Reused || prepared.ConfigurationID == "" || prepared.Queued != 1 {
		t.Fatalf("candidate configuration preparation = %#v err=%v", prepared, err)
	}
	candidateID, err := repository.CandidateConfigurationID(ctx, seed.VersionLabel, V01ParserVersion)
	if err != nil || candidateID != prepared.ConfigurationID {
		t.Fatalf("candidate configuration lookup id=%q err=%v", candidateID, err)
	}

	var enabledRules, decisions, queued int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM technology_dictionary_rules WHERE dictionary_version_id = $1 AND state = 'enabled'),
  (SELECT count(*)
   FROM technology_dictionary_rule_decisions AS decision
   JOIN technology_dictionary_rules AS rule ON rule.id = decision.rule_id
   WHERE rule.dictionary_version_id = $1),
  (SELECT count(*) FROM technology_analysis_work_items WHERE configuration_id = $2)`,
		first.DictionaryVersionID, prepared.ConfigurationID).Scan(&enabledRules, &decisions, &queued); err != nil {
		t.Fatal(err)
	}
	if enabledRules != 1 || decisions != 1 || queued != 1 {
		t.Fatalf("candidate lifecycle counts = enabled:%d decisions:%d queued:%d", enabledRules, decisions, queued)
	}
}

func TestAdditionalCandidateRuleApprovalCreatesDistinctReanalysisSnapshot(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	_, _, _, _ = insertSyntheticAnalysisPrerequisites(t, repository)
	secondBodyID := insertSyntheticAnalysisBody(t, repository, "00000000-0000-4000-8000-000000000510", "Kubernetes K8s")

	seed := CandidateV01Seed()
	if _, err := repository.ImportCandidateDictionary(ctx, seed, "remote-operator"); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ApproveCandidateRule(ctx, seed.VersionLabel, "kubernetes.01", "remote-reviewer"); err != nil {
		t.Fatal(err)
	}
	first, err := repository.PrepareCandidateConfigurationForDictionary(ctx, seed.VersionLabel, V01ParserVersion, "remote-operator")
	if err != nil || first.Queued != 2 {
		t.Fatalf("first candidate preparation = %#v err=%v", first, err)
	}
	if _, err := repository.ApproveCandidateRule(ctx, seed.VersionLabel, "kubernetes.02", "remote-reviewer"); err != nil {
		t.Fatal(err)
	}
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, first.ConfigurationID); err != nil {
		t.Fatal(err)
	}
	var firstSnapshotNewRuleEvidence int
	if err := repository.pool.QueryRow(ctx, `
SELECT count(*)
FROM technology_mention_evidence AS evidence
JOIN technology_mentions AS mention ON mention.id = evidence.technology_mention_id
JOIN technology_analysis_results AS result ON result.id = mention.analysis_result_id
JOIN technology_dictionary_rules AS rule ON rule.id = evidence.dictionary_rule_id
WHERE result.configuration_id = $1
  AND result.body_content_version_id = $2
  AND rule.rule_key = 'kubernetes.02'`, first.ConfigurationID, secondBodyID).Scan(&firstSnapshotNewRuleEvidence); err != nil {
		t.Fatal(err)
	}
	if firstSnapshotNewRuleEvidence != 0 {
		t.Fatalf("first snapshot unexpectedly used later-approved rule: %d", firstSnapshotNewRuleEvidence)
	}

	second, err := repository.PrepareCandidateConfigurationForDictionary(ctx, seed.VersionLabel, V01ParserVersion, "remote-operator")
	if err != nil || second.ConfigurationID == first.ConfigurationID || second.Queued != 2 {
		t.Fatalf("second candidate preparation = %#v first=%#v err=%v", second, first, err)
	}
	currentConfigurationID, err := repository.CandidateConfigurationID(ctx, seed.VersionLabel, V01ParserVersion)
	if err != nil || currentConfigurationID != second.ConfigurationID {
		t.Fatalf("current candidate configuration = %q, want %q err=%v", currentConfigurationID, second.ConfigurationID, err)
	}
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, second.ConfigurationID); err != nil {
		t.Fatal(err)
	}

	var firstRuleEvidence, secondRuleEvidence int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  count(*) FILTER (WHERE rule.rule_key = 'kubernetes.01'),
  count(*) FILTER (WHERE rule.rule_key = 'kubernetes.02')
FROM technology_mention_evidence AS evidence
JOIN technology_mentions AS mention ON mention.id = evidence.technology_mention_id
JOIN technology_analysis_results AS result ON result.id = mention.analysis_result_id
JOIN technology_dictionary_rules AS rule ON rule.id = evidence.dictionary_rule_id
WHERE result.configuration_id = $1 AND result.body_content_version_id = $2`, second.ConfigurationID, secondBodyID).Scan(&firstRuleEvidence, &secondRuleEvidence); err != nil {
		t.Fatal(err)
	}
	if firstRuleEvidence != 1 || secondRuleEvidence != 1 {
		t.Fatalf("second snapshot evidence = kubernetes.01:%d kubernetes.02:%d", firstRuleEvidence, secondRuleEvidence)
	}

	var firstSnapshotResults, secondSnapshotResults int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  count(*) FILTER (WHERE configuration_id = $1 AND status = 'complete'),
  count(*) FILTER (WHERE configuration_id = $2 AND status = 'complete')
FROM technology_analysis_results`, first.ConfigurationID, second.ConfigurationID).Scan(&firstSnapshotResults, &secondSnapshotResults); err != nil {
		t.Fatal(err)
	}
	if firstSnapshotResults != 2 || secondSnapshotResults != 2 {
		t.Fatalf("complete results by snapshot = first:%d second:%d", firstSnapshotResults, secondSnapshotResults)
	}
}
