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
