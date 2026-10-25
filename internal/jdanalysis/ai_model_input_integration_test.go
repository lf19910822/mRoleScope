package jdanalysis

import (
	"context"
	"testing"
)

func TestRepositoryBuildsAIModelInputFromPublishedDeterministicEvidenceOnly(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)

	technologyConfigurationID := prepareSyntheticAnalysisConfiguration(t, repository, dictionaryID, "synthetic-ai-model-context")
	if _, err := repository.PublishCompleteResult(ctx, ResultIdentity{
		BodyContentVersionID: bodyID,
		ConfigurationID:      technologyConfigurationID,
	}, []MentionInput{{
		TechnologyIdentityID: technologyID,
		Evidence: []EvidenceInput{
			{SourceField: SourceFieldDescription, RuleID: ruleID, Start: 0, End: 2},
			{SourceField: SourceFieldDescription, RuleID: ruleID, Start: 4, End: 6},
		},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.PromoteCandidateConfiguration(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	aiConfiguration, err := repository.PrepareCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-model-input"))
	if err != nil {
		t.Fatal(err)
	}

	input, err := repository.BuildAIModelInput(ctx, "request-01", aiConfiguration.Configuration.ID, bodyID)
	if err != nil {
		t.Fatalf("BuildAIModelInput() error = %v", err)
	}
	if got := input.VerifiedTechnologyMentions; len(got) != 2 || got[0].TechnologyKey != "go" || got[1].TechnologyKey != "go" {
		t.Fatalf("verified model context = %#v, want exactly the two published deterministic Go hits", got)
	}
	if len(input.Fields) != 1 || input.Fields[0].SourceField != SourceFieldDescription || len(input.Fields[0].Spans) != 2 {
		t.Fatalf("model fields = %#v, want only non-empty approved description spans", input.Fields)
	}
}
