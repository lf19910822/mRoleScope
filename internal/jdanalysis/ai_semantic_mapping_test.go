package jdanalysis

import (
	"encoding/json"
	"testing"
)

func TestAIModelInputMapsConservativeSemanticProjection(t *testing.T) {
	input := syntheticSemanticModelInput(t)
	response := syntheticConfiguredSemanticResponse()
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}

	projection, err := input.ValidateAndMapResponse(raw)
	if err != nil {
		t.Fatalf("ValidateAndMapResponse() error = %v", err)
	}
	if projection.Experience.BaseYear == nil || *projection.Experience.BaseYear != 3 || len(projection.Experience.ConditionalQualifications) != 2 {
		t.Fatalf("experience projection = %#v, want base 3 and two explicit branches", projection.Experience)
	}
	if projection.MinimumEducation.Level != "doctor" || len(projection.MinimumEducation.Evidence) != 1 || projection.MinimumEducation.Evidence[0].SourceField != SourceFieldJobRequire {
		t.Fatalf("minimum education projection = %#v, want jobRequire-supported doctor conclusion", projection.MinimumEducation)
	}
	if len(projection.RequirementGroups) != 4 {
		t.Fatalf("requirement groups = %#v, want four groups", projection.RequirementGroups)
	}
	qualification := projection.RequirementGroups[0]
	if !qualification.CandidateRequirement || qualification.MinSatisfied != 1 || qualification.Necessity != "required" || len(qualification.Options) != 5 {
		t.Fatalf("qualification group = %#v", qualification)
	}
	for index, want := range []string{"p1", "p2", "p3", "p4", ""} {
		got := ""
		if qualification.Options[index].Proficiency != nil {
			got = *qualification.Options[index].Proficiency
		}
		if got != want {
			t.Fatalf("option %d proficiency = %q, want %q", index, got, want)
		}
	}
	if preferred := projection.RequirementGroups[1]; !preferred.CandidateRequirement || preferred.Necessity != "preferred" {
		t.Fatalf("preferred qualification group = %#v", preferred)
	}
	if responsibility := projection.RequirementGroups[2]; responsibility.CandidateRequirement || responsibility.Role != "responsibility" {
		t.Fatalf("responsibility group incorrectly affects candidate matching: %#v", responsibility)
	}
	if ambiguous := projection.RequirementGroups[3]; ambiguous.CandidateRequirement || ambiguous.ReviewTier != "required_review" {
		t.Fatalf("ambiguous group = %#v, want non-candidate required review", ambiguous)
	}
	if len(projection.CandidateMentions) != 1 || projection.CandidateMentions[0].Evidence.CodePointRange != (CodePointRange{Start: 24, End: 33}) || projection.CandidateMentions[0].ReviewTier != "required_review" {
		t.Fatalf("candidate projection = %#v", projection.CandidateMentions)
	}
}

func TestAIModelInputMapsUncertainWithoutInventingAZeroValue(t *testing.T) {
	input := syntheticSemanticModelInput(t)
	response := syntheticConfiguredSemanticResponse()
	response.Experience = AIExperienceResponse{
		Status:                    "uncertain",
		BaseYear:                  nil,
		Items:                     []AIExperienceItemResponse{},
		ConditionalQualifications: []AIConditionalQualificationResponse{},
		EvidenceSpanIDs:           []string{"description:s01"},
		ReviewRecommendation:      "routine",
	}
	response.MinimumEducation = AIMinimumEducationResponse{
		Status:               "not_configured",
		Level:                nil,
		LevelCode:            nil,
		EvidenceSpanIDs:      []string{},
		ReviewRecommendation: "routine",
	}
	response.TechnologyRequirementGroups = []AITechnologyRequirementGroupResponse{}
	response.CandidateMentions = []AICandidateMentionResponse{}
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}

	projection, err := input.ValidateAndMapResponse(raw)
	if err != nil {
		t.Fatalf("ValidateAndMapResponse() error = %v", err)
	}
	if projection.Experience.Status != "uncertain" || projection.Experience.BaseYear != nil || projection.Experience.ReviewTier != "required_review" {
		t.Fatalf("uncertain experience became a configured/zero value: %#v", projection.Experience)
	}
	if projection.MinimumEducation.Status != "not_configured" || projection.MinimumEducation.Level != "" {
		t.Fatalf("not configured education = %#v", projection.MinimumEducation)
	}
}

func TestAIModelInputRejectsNonBaseConditionalQualification(t *testing.T) {
	input := syntheticSemanticModelInput(t)
	response := syntheticConfiguredSemanticResponse()
	response.Experience.ConditionalQualifications[0].ExperienceScope = "technology"
	response.Experience.ConditionalQualifications[0].ExperienceScopeKey = "go"
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := input.ValidateAndMapResponse(raw); err == nil {
		t.Fatal("ValidateAndMapResponse() accepted a conditional qualification outside the representable overall scope")
	}
}

func TestAIModelInputV2RecoversLocallyMalformedFieldsWithoutDiscardingJD(t *testing.T) {
	input := syntheticSemanticModelInput(t)
	input.validatorVersion = AIValidatorVersionV2
	bachelor, wrongBachelorCode := "bachelor", 3
	baseYear := 3
	minimumSatisfied := 1
	atLeast := "at_least"
	response := AISemanticResponse{
		ResponseSchemaVersion: AIResponseSchemaVersionV1,
		RequestID:             input.RequestID,
		Experience: AIExperienceResponse{
			Status:          "configured",
			BaseYear:        &baseYear,
			EvidenceSpanIDs: []string{"jobRequire:s01"},
			Items:           []AIExperienceItemResponse{},
			ConditionalQualifications: []AIConditionalQualificationResponse{{
				WhenEducation: "bachelor", ExperienceScope: "technology", ExperienceScopeKey: "go", MinYears: 3,
				EvidenceSpanIDs: []string{"jobRequire:s01"}, ReviewRecommendation: "required_review",
			}},
			ReviewRecommendation: "routine",
		},
		MinimumEducation: AIMinimumEducationResponse{
			Status: "configured", Level: &bachelor, LevelCode: &wrongBachelorCode,
			EvidenceSpanIDs: []string{"jobRequire:s01"}, ReviewRecommendation: "routine",
		},
		TechnologyRequirementGroups: []AITechnologyRequirementGroupResponse{
			{
				LocalID: "invalid-qualification", Role: "qualification", Necessity: "required",
				Operator: &atLeast, MinSatisfied: &minimumSatisfied, Options: []AITechnologyRequirementOptionResponse{},
				EvidenceSpanIDs: []string{"description:s01"}, ReviewRecommendation: "routine",
			},
			{
				LocalID: "verified-responsibility", Role: "responsibility", Necessity: "unspecified",
				Options: []AITechnologyRequirementOptionResponse{
					{TechnologyKey: "go", MentionEvidenceIDs: []string{"evidence-go"}},
					{TechnologyKey: "go", MentionEvidenceIDs: []string{"evidence-go"}},
				},
				EvidenceSpanIDs: []string{"description:s01"}, ReviewRecommendation: "routine",
			},
		},
		CandidateMentions: []AICandidateMentionResponse{
			{CandidateText: "not present", SpanID: "description:s01", Role: "qualification", Necessity: "required", ReviewRecommendation: "required_review"},
		},
	}
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}

	projection, err := input.ValidateAndMapResponse(raw)
	if err != nil {
		t.Fatalf("ValidateAndMapResponse() error = %v; local defects must not reject the JD", err)
	}
	if projection.Experience.Status != "configured" || projection.Experience.BaseYear == nil || *projection.Experience.BaseYear != 3 || len(projection.Experience.Items) != 1 || projection.Experience.Items[0].SubjectKind != "overall" || projection.Experience.ReviewTier != "required_review" {
		t.Fatalf("experience recovery = %#v, want synthesized overall item with required review", projection.Experience)
	}
	if projection.MinimumEducation.Status != "configured" || projection.MinimumEducation.Level != "bachelor" || projection.MinimumEducation.ReviewTier != "required_review" {
		t.Fatalf("education recovery = %#v, want verified level with required review", projection.MinimumEducation)
	}
	if len(projection.RequirementGroups) != 1 || projection.RequirementGroups[0].Role != "responsibility" || projection.RequirementGroups[0].CandidateRequirement || projection.RequirementGroups[0].ReviewTier != "required_review" || len(projection.RequirementGroups[0].Options) != 1 {
		t.Fatalf("requirement recovery = %#v, want only non-candidate review group", projection.RequirementGroups)
	}
	if len(projection.CandidateMentions) != 0 {
		t.Fatalf("unlocatable candidate was persisted in projection: %#v", projection.CandidateMentions)
	}
}

func syntheticConfiguredSemanticResponse() AISemanticResponse {
	baseYear := 3
	doctorLevel := "doctor"
	doctorCode := 4
	minSatisfied := 1
	atLeast := "at_least"
	p1, p2, p3, p4 := "P1", "P2", "P3", "P4"
	return AISemanticResponse{
		ResponseSchemaVersion: AIResponseSchemaVersionV1,
		RequestID:             "request-01",
		Experience: AIExperienceResponse{
			Status:   "configured",
			BaseYear: &baseYear,
			Items: []AIExperienceItemResponse{{
				ScopeType:            "base",
				ScopeKey:             "overall",
				MinYears:             3,
				EvidenceSpanIDs:      []string{"jobRequire:s01"},
				ReviewRecommendation: "routine",
			}},
			ConditionalQualifications: []AIConditionalQualificationResponse{
				{WhenEducation: "bachelor", ExperienceScope: "base", ExperienceScopeKey: "overall", MinYears: 3, EvidenceSpanIDs: []string{"jobRequire:s01"}, ReviewRecommendation: "routine"},
				{WhenEducation: "doctor", ExperienceScope: "base", ExperienceScopeKey: "overall", MinYears: 2, EvidenceSpanIDs: []string{"jobRequire:s01"}, ReviewRecommendation: "routine"},
			},
			EvidenceSpanIDs:      []string{"jobRequire:s01"},
			ReviewRecommendation: "routine",
		},
		MinimumEducation: AIMinimumEducationResponse{
			Status:               "configured",
			Level:                &doctorLevel,
			LevelCode:            &doctorCode,
			EvidenceSpanIDs:      []string{"jobRequire:s01"},
			ReviewRecommendation: "routine",
		},
		TechnologyRequirementGroups: []AITechnologyRequirementGroupResponse{
			{
				LocalID:      "qualification-01",
				Role:         "qualification",
				Necessity:    "required",
				Operator:     &atLeast,
				MinSatisfied: &minSatisfied,
				Options: []AITechnologyRequirementOptionResponse{
					{TechnologyKey: "go", Proficiency: &p1, MentionEvidenceIDs: []string{"evidence-go"}},
					{TechnologyKey: "java", Proficiency: &p2, MentionEvidenceIDs: []string{"evidence-java"}},
					{TechnologyKey: "rust", Proficiency: &p3, MentionEvidenceIDs: []string{"evidence-rust"}},
					{TechnologyKey: "python", Proficiency: &p4, MentionEvidenceIDs: []string{"evidence-python"}},
					{TechnologyKey: "lua", Proficiency: nil, MentionEvidenceIDs: []string{"evidence-lua"}},
				},
				EvidenceSpanIDs:      []string{"description:s01"},
				ReviewRecommendation: "routine",
			},
			{
				LocalID:              "preferred-01",
				Role:                 "qualification",
				Necessity:            "preferred",
				Operator:             &atLeast,
				MinSatisfied:         &minSatisfied,
				Options:              []AITechnologyRequirementOptionResponse{{TechnologyKey: "java", Proficiency: nil, MentionEvidenceIDs: []string{"evidence-java"}}},
				EvidenceSpanIDs:      []string{"description:s02"},
				ReviewRecommendation: "routine",
			},
			{
				LocalID:              "responsibility-01",
				Role:                 "responsibility",
				Necessity:            "required",
				Operator:             nil,
				MinSatisfied:         nil,
				Options:              []AITechnologyRequirementOptionResponse{{TechnologyKey: "go", Proficiency: nil, MentionEvidenceIDs: []string{"evidence-go"}}},
				EvidenceSpanIDs:      []string{"description:s01"},
				ReviewRecommendation: "routine",
			},
			{
				LocalID:              "ambiguous-01",
				Role:                 "unspecified",
				Necessity:            "unspecified",
				Operator:             nil,
				MinSatisfied:         nil,
				Options:              []AITechnologyRequirementOptionResponse{{TechnologyKey: "java", Proficiency: nil, MentionEvidenceIDs: []string{"evidence-java"}}},
				EvidenceSpanIDs:      []string{"description:s02"},
				ReviewRecommendation: "routine",
			},
		},
		CandidateMentions: []AICandidateMentionResponse{{
			CandidateText:        "OtherTool",
			SpanID:               "description:s06",
			Role:                 "qualification",
			Necessity:            "preferred",
			ReviewRecommendation: "required_review",
		}},
	}
}

func syntheticSemanticModelInput(t *testing.T) AIModelInput {
	t.Helper()
	body := syntheticAISpanBody("Go；Java；Rust；Python；Lua；OtherTool。", "Master。", "")
	spans, err := NewAISpanIndex(body)
	if err != nil {
		t.Fatal(err)
	}
	mentions := []VerifiedTechnologyMention{
		{BodyContentVersionID: body.BodyContentVersionID, CanonicalName: "Go", CodePointRange: CodePointRange{Start: 0, End: 2}, MentionEvidenceID: "evidence-go", NormalizationVersion: body.NormalizationVersion, NormalizedFieldSHA256: body.DescriptionSHA256, SourceField: SourceFieldDescription, TechnologyKey: "go"},
		{BodyContentVersionID: body.BodyContentVersionID, CanonicalName: "Java", CodePointRange: CodePointRange{Start: 3, End: 7}, MentionEvidenceID: "evidence-java", NormalizationVersion: body.NormalizationVersion, NormalizedFieldSHA256: body.DescriptionSHA256, SourceField: SourceFieldDescription, TechnologyKey: "java"},
		{BodyContentVersionID: body.BodyContentVersionID, CanonicalName: "Rust", CodePointRange: CodePointRange{Start: 8, End: 12}, MentionEvidenceID: "evidence-rust", NormalizationVersion: body.NormalizationVersion, NormalizedFieldSHA256: body.DescriptionSHA256, SourceField: SourceFieldDescription, TechnologyKey: "rust"},
		{BodyContentVersionID: body.BodyContentVersionID, CanonicalName: "Python", CodePointRange: CodePointRange{Start: 13, End: 19}, MentionEvidenceID: "evidence-python", NormalizationVersion: body.NormalizationVersion, NormalizedFieldSHA256: body.DescriptionSHA256, SourceField: SourceFieldDescription, TechnologyKey: "python"},
		{BodyContentVersionID: body.BodyContentVersionID, CanonicalName: "Lua", CodePointRange: CodePointRange{Start: 20, End: 23}, MentionEvidenceID: "evidence-lua", NormalizationVersion: body.NormalizationVersion, NormalizedFieldSHA256: body.DescriptionSHA256, SourceField: SourceFieldDescription, TechnologyKey: "lua"},
	}
	input, err := BuildAIModelInput("request-01", body, spans, mentions)
	if err != nil {
		t.Fatal(err)
	}
	return input
}
