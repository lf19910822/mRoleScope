package jdanalysis

import (
	"context"
	"testing"
)

func TestExpandAIPlusCandidateQueuesOnlyRemainingEligibleBodiesOnce(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-expansion-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-expansion-candidate"))
	if err != nil {
		t.Fatal(err)
	}
	seedCompletedReviewedAITrialForRelease(t, repository, aiConfiguration.ID, aiTrialSampleSize)

	first, err := repository.ExpandAIPlusCandidate(ctx, aiConfiguration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.AIConfigurationID != aiConfiguration.ID || first.Queued != 1 || first.ExistingExpansionWorkCount != 0 {
		t.Fatalf("first AI expansion summary = %#v", first)
	}
	var workBodyID, trigger, status string
	if err := repository.pool.QueryRow(ctx, `
SELECT body_content_version_id::text, trigger::text, status::text
FROM ai_analysis_work_items
WHERE ai_analysis_configuration_id = $1 AND trigger = 'expansion'`, aiConfiguration.ID).Scan(&workBodyID, &trigger, &status); err != nil {
		t.Fatal(err)
	}
	if workBodyID != bodyID || trigger != "expansion" || status != "pending" {
		t.Fatalf("expanded AI work = body:%s trigger:%s status:%s", workBodyID, trigger, status)
	}

	second, err := repository.ExpandAIPlusCandidate(ctx, aiConfiguration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Queued != 0 || second.ExistingExpansionWorkCount != 1 {
		t.Fatalf("repeat AI expansion summary = %#v", second)
	}
}

func TestExpandAIPlusCandidateAllowsTerminalTrialWithoutSemanticReviews(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-expansion-mechanical-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-expansion-mechanical-candidate"))
	if err != nil {
		t.Fatal(err)
	}
	seedTerminalAITrialForMechanicalPublication(t, repository, aiConfiguration.ID, aiTrialSampleSize, false)

	summary, err := repository.ExpandAIPlusCandidate(ctx, aiConfiguration.ID)
	if err != nil {
		t.Fatalf("mechanically terminal Plus trial must expand without semantic reviews: %v", err)
	}
	if summary.Queued != 1 || summary.ExistingExpansionWorkCount != 0 {
		t.Fatalf("mechanical AI expansion summary = %#v", summary)
	}
}
