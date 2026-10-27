package jdanalysis

import (
	"encoding/json"
	"testing"
)

func TestAIModelInputMapsStructurallyValidSemanticProjection(t *testing.T) {
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
	if ambiguous := projection.RequirementGroups[3]; ambiguous.CandidateRequirement || ambiguous.ReviewTier != "routine" {
		t.Fatalf("ambiguous group = %#v, want the model-supplied non-candidate routine review tier", ambiguous)
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
	if projection.Experience.Status != "uncertain" || projection.Experience.BaseYear != nil || projection.Experience.ReviewTier != "routine" {
		t.Fatalf("uncertain experience became a configured/zero value: %#v", projection.Experience)
	}
	if projection.MinimumEducation.Status != "not_configured" || projection.MinimumEducation.Level != "" {
		t.Fatalf("not configured education = %#v", projection.MinimumEducation)
	}
}

func TestAIModelInputKeepsSupportedBaseYearWithoutDetailedItems(t *testing.T) {
	input := syntheticExperienceOnlyModelInput(t, "候选人需具备三年以上服务端开发经验。")
	baseYear := 3
	response := emptySemanticResponse(input)
	response.Experience = AIExperienceResponse{
		Status:                    "configured",
		BaseYear:                  &baseYear,
		Items:                     []AIExperienceItemResponse{},
		ConditionalQualifications: []AIConditionalQualificationResponse{},
		EvidenceSpanIDs:           []string{"description:s01"},
		ReviewRecommendation:      "routine",
	}

	projection, err := validateAndMapSyntheticResponse(input, response)
	if err != nil {
		t.Fatalf("ValidateAndMapResponse() error = %v", err)
	}
	if projection.Experience.Status != "configured" || projection.Experience.BaseYear == nil || *projection.Experience.BaseYear != 3 || len(projection.Experience.Items) != 0 || projection.Experience.ReviewTier != "routine" {
		t.Fatalf("supported summary-only experience = %#v", projection.Experience)
	}
}

func TestAIModelInputDropsUnknownSpanWithoutDiscardingSupportedExperience(t *testing.T) {
	input := syntheticExperienceOnlyModelInput(t, "候选人需具备三年以上服务端开发经验。")
	baseYear := 3
	response := emptySemanticResponse(input)
	response.Experience = AIExperienceResponse{
		Status:                    "configured",
		BaseYear:                  &baseYear,
		Items:                     []AIExperienceItemResponse{},
		ConditionalQualifications: []AIConditionalQualificationResponse{},
		EvidenceSpanIDs:           []string{"unknown-span", "description:s01"},
		ReviewRecommendation:      "routine",
	}

	projection, err := validateAndMapSyntheticResponse(input, response)
	if err != nil {
		t.Fatalf("ValidateAndMapResponse() error = %v, want local evidence sanitation", err)
	}
	if projection.Experience.BaseYear == nil || *projection.Experience.BaseYear != 3 {
		t.Fatalf("experience base year = %#v, want retained model conclusion", projection.Experience)
	}
	if len(projection.Experience.Evidence) != 1 || projection.Experience.Evidence[0].SpanID != "description:s01" {
		t.Fatalf("experience evidence = %#v, want only the known body span", projection.Experience.Evidence)
	}
}

func TestAIModelInputDropsInvalidGroupEvidenceWithoutDiscardingItsOptions(t *testing.T) {
	input := syntheticSemanticModelInput(t)
	response := syntheticConfiguredSemanticResponse()
	response.TechnologyRequirementGroups[0].EvidenceSpanIDs = []string{"unknown-span"}
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}

	projection, err := input.ValidateAndMapResponse(raw)
	if err != nil {
		t.Fatalf("ValidateAndMapResponse() error = %v, want local group-evidence sanitation", err)
	}
	if len(projection.RequirementGroups) != 4 || len(projection.RequirementGroups[0].Options) != 5 {
		t.Fatalf("requirement groups = %#v, want all model groups and first group options retained", projection.RequirementGroups)
	}
	if len(projection.RequirementGroups[0].Evidence) != 0 {
		t.Fatalf("first group evidence = %#v, want the unknown reference removed", projection.RequirementGroups[0].Evidence)
	}
}

func TestAIModelInputDropsUnknownFormalOptionWithoutDiscardingGovernedSibling(t *testing.T) {
	input := syntheticSemanticModelInput(t)
	response := emptySemanticResponse(input)
	response.TechnologyRequirementGroups = []AITechnologyRequirementGroupResponse{{
		LocalID:              "mixed-governance-group",
		Role:                 "qualification",
		Necessity:            "required",
		Operator:             nil,
		MinSatisfied:         nil,
		EvidenceSpanIDs:      []string{"description:s01"},
		ReviewRecommendation: "routine",
		Options: []AITechnologyRequirementOptionResponse{
			{TechnologyKey: "ungoverned-tool", MentionEvidenceIDs: []string{"evidence-unknown"}},
			{TechnologyKey: "go", MentionEvidenceIDs: []string{"evidence-go"}},
		},
	}}

	projection, err := validateAndMapSyntheticResponse(input, response)
	if err != nil {
		t.Fatalf("ValidateAndMapResponse() error = %v, want local option sanitation", err)
	}
	if len(projection.RequirementGroups) != 1 {
		t.Fatalf("requirement groups = %#v, want retained group", projection.RequirementGroups)
	}
	options := projection.RequirementGroups[0].Options
	if len(options) != 1 || options[0].TechnologyKey != "go" {
		t.Fatalf("mapped options = %#v, want only governed go option", options)
	}
}

func TestAIModelInputKeepsStructurallyValidModelRelationWithoutPolicyRejection(t *testing.T) {
	input := syntheticSemanticModelInput(t)
	minimumTwo := 2
	atLeast := "at_least"
	response := emptySemanticResponse(input)
	response.TechnologyRequirementGroups = []AITechnologyRequirementGroupResponse{{
		LocalID:              "model-supplied-relation",
		Role:                 "qualification",
		Necessity:            "required",
		Operator:             &atLeast,
		MinSatisfied:         &minimumTwo,
		Options:              []AITechnologyRequirementOptionResponse{{TechnologyKey: "go", MentionEvidenceIDs: []string{"evidence-go"}}},
		EvidenceSpanIDs:      []string{"description:s01"},
		ReviewRecommendation: "routine",
	}}

	projection, err := validateAndMapSyntheticResponse(input, response)
	if err != nil {
		t.Fatalf("ValidateAndMapResponse() error = %v, want structurally valid model relation retained", err)
	}
	if len(projection.RequirementGroups) != 1 || projection.RequirementGroups[0].MinSatisfied != 2 || len(projection.RequirementGroups[0].Options) != 1 {
		t.Fatalf("model relation projection = %#v, want at_least=2 with its one option retained", projection.RequirementGroups)
	}
}

func TestAIModelInputKeepsStructurallyValidModelExperienceWithoutRecomputingYears(t *testing.T) {
	input := syntheticExperienceOnlyModelInput(t, "候选人需具备三年以上服务端开发经验。")
	baseYear := 3
	response := emptySemanticResponse(input)
	response.Experience = AIExperienceResponse{
		Status:   "configured",
		BaseYear: &baseYear,
		Items: []AIExperienceItemResponse{{
			ScopeType: "base", ScopeKey: "overall", MinYears: 5,
			EvidenceSpanIDs: []string{"description:s01"}, ReviewRecommendation: "routine",
		}},
		ConditionalQualifications: []AIConditionalQualificationResponse{},
		EvidenceSpanIDs:           []string{"description:s01"},
		ReviewRecommendation:      "routine",
	}

	projection, err := validateAndMapSyntheticResponse(input, response)
	if err != nil {
		t.Fatalf("ValidateAndMapResponse() error = %v, want model years retained without a policy gate", err)
	}
	if projection.Experience.BaseYear == nil || *projection.Experience.BaseYear != 3 || len(projection.Experience.Items) != 1 || projection.Experience.Items[0].MinimumYears != 5 {
		t.Fatalf("experience projection = %#v, want model base year and item year retained", projection.Experience)
	}
}

func TestAIModelInputDropsInvalidOptionEvidenceWithoutDiscardingGovernedOption(t *testing.T) {
	input := syntheticSemanticModelInput(t)
	minimumOne := 1
	atLeast := "at_least"
	response := emptySemanticResponse(input)
	response.TechnologyRequirementGroups = []AITechnologyRequirementGroupResponse{{
		LocalID: "partially-invalid-option-evidence", Role: "qualification", Necessity: "required", Operator: &atLeast, MinSatisfied: &minimumOne,
		Options: []AITechnologyRequirementOptionResponse{{
			TechnologyKey: "go", MentionEvidenceIDs: []string{"evidence-unknown", "evidence-go"},
		}},
		EvidenceSpanIDs: []string{"description:s01"}, ReviewRecommendation: "routine",
	}}

	projection, err := validateAndMapSyntheticResponse(input, response)
	if err != nil {
		t.Fatalf("ValidateAndMapResponse() error = %v, want only the bad option reference removed", err)
	}
	if len(projection.RequirementGroups) != 1 || len(projection.RequirementGroups[0].Options) != 1 {
		t.Fatalf("requirement projection = %#v, want the governed option retained", projection.RequirementGroups)
	}
	if got := projection.RequirementGroups[0].Options[0].MentionEvidenceIDs; len(got) != 1 || got[0] != "evidence-go" {
		t.Fatalf("option evidence IDs = %#v, want only evidence-go", got)
	}
}

func TestAIModelInputKeepsStructurallyValidEducationWithoutSemanticConsistencyGate(t *testing.T) {
	input := syntheticSemanticModelInput(t)
	bachelor := "bachelor"
	doctorCode := 4
	response := emptySemanticResponse(input)
	response.MinimumEducation = AIMinimumEducationResponse{
		Status:               "configured",
		Level:                &bachelor,
		LevelCode:            &doctorCode,
		EvidenceSpanIDs:      []string{"unknown-span", "jobRequire:s01"},
		ReviewRecommendation: "routine",
	}

	projection, err := validateAndMapSyntheticResponse(input, response)
	if err != nil {
		t.Fatalf("ValidateAndMapResponse() error = %v, want model education retained", err)
	}
	if projection.MinimumEducation.Status != "configured" || projection.MinimumEducation.Level != "bachelor" {
		t.Fatalf("education projection = %#v, want the model level retained", projection.MinimumEducation)
	}
	if len(projection.MinimumEducation.Evidence) != 1 || projection.MinimumEducation.Evidence[0].SpanID != "jobRequire:s01" {
		t.Fatalf("education evidence = %#v, want only the known body span", projection.MinimumEducation.Evidence)
	}
}

func TestAIModelInputKeepsModelExperienceDomainsThatFitTheNormalizedShape(t *testing.T) {
	input := syntheticExperienceOnlyModelInput(t, "3年以上嵌入式开发经验。具备云计算，后台开发或音视频/流媒体等方向经验。3年以上消费电子/智能硬件测试经验。")
	baseYear := 3
	response := emptySemanticResponse(input)
	response.Experience = AIExperienceResponse{
		Status:   "configured",
		BaseYear: &baseYear,
		Items: []AIExperienceItemResponse{
			{ScopeType: "base", ScopeKey: "overall", MinYears: 3, EvidenceSpanIDs: []string{"description:s01"}, ReviewRecommendation: "routine"},
			{ScopeType: "domain", ScopeKey: "嵌入式开发", MinYears: 3, EvidenceSpanIDs: []string{"description:s01"}, ReviewRecommendation: "routine"},
			{ScopeType: "domain", ScopeKey: "云计算", MinYears: 3, EvidenceSpanIDs: []string{"description:s02"}, ReviewRecommendation: "routine"},
			{ScopeType: "domain", ScopeKey: "消费电子/智能硬件测试", MinYears: 3, EvidenceSpanIDs: []string{"description:s04"}, ReviewRecommendation: "routine"},
		},
		ConditionalQualifications: []AIConditionalQualificationResponse{},
		EvidenceSpanIDs:           []string{"description:s01"},
		ReviewRecommendation:      "routine",
	}

	projection, err := validateAndMapSyntheticResponse(input, response)
	if err != nil {
		t.Fatalf("ValidateAndMapResponse() error = %v", err)
	}
	if projection.Experience.BaseYear == nil || *projection.Experience.BaseYear != 3 {
		t.Fatalf("overall experience was lost with an alternative domain: %#v", projection.Experience)
	}
	if len(projection.Experience.Items) != 4 {
		t.Fatalf("experience items = %#v, want every model-supplied item that the normalized shape can represent", projection.Experience.Items)
	}
	if projection.Experience.Items[0].SubjectKind != "overall" || projection.Experience.Items[1].SubjectKey != "嵌入式开发" || projection.Experience.Items[2].SubjectKey != "云计算" || projection.Experience.Items[3].SubjectKey != "消费电子/智能硬件测试" {
		t.Fatalf("experience items = %#v, want the model-supplied overall and domain projections", projection.Experience.Items)
	}
}

func TestAIModelInputDoesNotRewriteModelRequirementGroups(t *testing.T) {
	body := syntheticAISpanBody("", "精通C++编程。熟悉Python语言。Go或Rust任一即可。", "")
	spans, err := NewAISpanIndex(body)
	if err != nil {
		t.Fatal(err)
	}
	mentions := []VerifiedTechnologyMention{
		{BodyContentVersionID: body.BodyContentVersionID, CanonicalName: "C++", CodePointRange: CodePointRange{Start: 2, End: 5}, MentionEvidenceID: "evidence-cpp", NormalizationVersion: body.NormalizationVersion, NormalizedFieldSHA256: body.JobRequireSHA256, SourceField: SourceFieldJobRequire, TechnologyKey: "cpp"},
		{BodyContentVersionID: body.BodyContentVersionID, CanonicalName: "Python", CodePointRange: CodePointRange{Start: 10, End: 16}, MentionEvidenceID: "evidence-python", NormalizationVersion: body.NormalizationVersion, NormalizedFieldSHA256: body.JobRequireSHA256, SourceField: SourceFieldJobRequire, TechnologyKey: "python"},
		{BodyContentVersionID: body.BodyContentVersionID, CanonicalName: "Go", CodePointRange: CodePointRange{Start: 19, End: 21}, MentionEvidenceID: "evidence-go", NormalizationVersion: body.NormalizationVersion, NormalizedFieldSHA256: body.JobRequireSHA256, SourceField: SourceFieldJobRequire, TechnologyKey: "go"},
		{BodyContentVersionID: body.BodyContentVersionID, CanonicalName: "Rust", CodePointRange: CodePointRange{Start: 22, End: 26}, MentionEvidenceID: "evidence-rust", NormalizationVersion: body.NormalizationVersion, NormalizedFieldSHA256: body.JobRequireSHA256, SourceField: SourceFieldJobRequire, TechnologyKey: "rust"},
	}
	input, err := BuildAIModelInput("requirement-relations", body, spans, mentions)
	if err != nil {
		t.Fatal(err)
	}
	operator := "at_least"
	minimumOne := 1
	response := emptySemanticResponse(input)
	response.TechnologyRequirementGroups = []AITechnologyRequirementGroupResponse{
		{
			LocalID: "separate-cpp-python", Role: "qualification", Necessity: "required", Operator: &operator, MinSatisfied: &minimumOne,
			Options: []AITechnologyRequirementOptionResponse{
				{TechnologyKey: "cpp", MentionEvidenceIDs: []string{"evidence-cpp"}},
				{TechnologyKey: "python", MentionEvidenceIDs: []string{"evidence-python"}},
			},
			EvidenceSpanIDs: []string{"jobRequire:s01", "jobRequire:s02"}, ReviewRecommendation: "routine",
		},
		{
			LocalID: "alternative-go-rust", Role: "qualification", Necessity: "required", Operator: &operator, MinSatisfied: &minimumOne,
			Options: []AITechnologyRequirementOptionResponse{
				{TechnologyKey: "go", MentionEvidenceIDs: []string{"evidence-go"}},
				{TechnologyKey: "rust", MentionEvidenceIDs: []string{"evidence-rust"}},
			},
			EvidenceSpanIDs: []string{"jobRequire:s03"}, ReviewRecommendation: "routine",
		},
	}

	projection, err := validateAndMapSyntheticResponse(input, response)
	if err != nil {
		t.Fatalf("ValidateAndMapResponse() error = %v", err)
	}
	if len(projection.RequirementGroups) != 2 {
		t.Fatalf("requirement groups = %#v, want the two model-supplied groups without local splitting", projection.RequirementGroups)
	}
	if group := projection.RequirementGroups[0]; group.LocalID != "separate-cpp-python" || len(group.Options) != 2 || group.Options[0].TechnologyKey != "cpp" || group.Options[1].TechnologyKey != "python" || len(group.Evidence) != 2 {
		t.Fatalf("first model group was rewritten: %#v", group)
	}
	if group := projection.RequirementGroups[1]; group.LocalID != "alternative-go-rust" || group.Operator != "at_least" || group.MinSatisfied != 1 || len(group.Options) != 2 {
		t.Fatalf("second model group was rewritten: %#v", group)
	}
}

func TestAIModelInputKeepsTheFirstThreeStructurallyValidCandidatesRegardlessOfModelRole(t *testing.T) {
	body := syntheticAISpanBody("ToolA。ToolB。ToolC。ToolD。泛化概念。", "", "")
	spans, err := NewAISpanIndex(body)
	if err != nil {
		t.Fatal(err)
	}
	input, err := BuildAIModelInput("candidate-governance", body, spans, nil)
	if err != nil {
		t.Fatal(err)
	}
	response := emptySemanticResponse(input)
	response.CandidateMentions = []AICandidateMentionResponse{
		{CandidateText: "ToolA", SpanID: "description:s01", Role: "qualification", Necessity: "required", ReviewRecommendation: "required_review"},
		{CandidateText: "ToolB", SpanID: "description:s02", Role: "responsibility", Necessity: "required", ReviewRecommendation: "required_review"},
		{CandidateText: "ToolC", SpanID: "description:s03", Role: "qualification", Necessity: "preferred", ReviewRecommendation: "required_review"},
		{CandidateText: "ToolD", SpanID: "description:s04", Role: "qualification", Necessity: "required", ReviewRecommendation: "required_review"},
		{CandidateText: "泛化概念", SpanID: "description:s05", Role: "qualification", Necessity: "required", ReviewRecommendation: "required_review"},
	}

	projection, err := validateAndMapSyntheticResponse(input, response)
	if err != nil {
		t.Fatalf("ValidateAndMapResponse() error = %v", err)
	}
	if len(projection.CandidateMentions) != 3 || projection.CandidateMentions[0].CandidateText != "ToolA" || projection.CandidateMentions[1].CandidateText != "ToolB" || projection.CandidateMentions[2].CandidateText != "ToolC" {
		t.Fatalf("candidate projections = %#v, want the first three source-exact model candidates", projection.CandidateMentions)
	}
}

func TestAIModelInputKeepsSourceTermForUnsupportedCandidateNormalization(t *testing.T) {
	body := syntheticAISpanBody("OriginalSDK。", "", "")
	spans, err := NewAISpanIndex(body)
	if err != nil {
		t.Fatal(err)
	}
	input, err := buildAIModelInputForResponseSchemaVersion(AIResponseSchemaVersionV3, "candidate-normalization", body, spans, nil)
	if err != nil {
		t.Fatal(err)
	}
	response := emptySemanticResponse(input)
	response.ResponseSchemaVersion = AIResponseSchemaVersionV3
	response.CandidateMentions = []AICandidateMentionResponse{{
		CandidateText: "Original Software Development Kit", SourceTerm: "OriginalSDK", SpanID: "description:s01",
		Role: "qualification", Necessity: "required", ReviewRecommendation: "required_review",
	}}

	projection, err := validateAndMapSyntheticResponse(input, response)
	if err != nil {
		t.Fatalf("ValidateAndMapResponse() error = %v", err)
	}
	if len(projection.CandidateMentions) != 0 {
		t.Fatalf("a non-source-exact normalization entered candidate governance: %#v", projection.CandidateMentions)
	}
	if len(projection.PendingUnresolvedSourceTerms) != 1 || projection.PendingUnresolvedSourceTerms[0].SourceTerm != "OriginalSDK" {
		t.Fatalf("unresolved source terms = %#v, want exact OriginalSDK", projection.PendingUnresolvedSourceTerms)
	}
}

func validateAndMapSyntheticResponse(input AIModelInput, response AISemanticResponse) (AISemanticProjection, error) {
	raw, err := json.Marshal(response)
	if err != nil {
		return AISemanticProjection{}, err
	}
	return input.ValidateAndMapResponse(raw)
}

func emptySemanticResponse(input AIModelInput) AISemanticResponse {
	return AISemanticResponse{
		CandidateMentions:           []AICandidateMentionResponse{},
		Experience:                  AIExperienceResponse{Status: "not_configured", Items: []AIExperienceItemResponse{}, ConditionalQualifications: []AIConditionalQualificationResponse{}, EvidenceSpanIDs: []string{}, ReviewRecommendation: "routine"},
		MinimumEducation:            AIMinimumEducationResponse{Status: "not_configured", EvidenceSpanIDs: []string{}, ReviewRecommendation: "routine"},
		RequestID:                   input.RequestID,
		ResponseSchemaVersion:       AIResponseSchemaVersionV1,
		TechnologyRequirementGroups: []AITechnologyRequirementGroupResponse{},
	}
}

func syntheticExperienceOnlyModelInput(t *testing.T, description string) AIModelInput {
	t.Helper()
	body := syntheticAISpanBody(description, "", "")
	spans, err := NewAISpanIndex(body)
	if err != nil {
		t.Fatal(err)
	}
	input, err := BuildAIModelInput("experience-only", body, spans, nil)
	if err != nil {
		t.Fatal(err)
	}
	return input
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
	if projection.Experience.Status != "configured" || projection.Experience.BaseYear == nil || *projection.Experience.BaseYear != 3 || len(projection.Experience.Items) != 0 || projection.Experience.ReviewTier != "routine" {
		t.Fatalf("experience recovery = %#v, want persisted base year with locally omitted unrepresentable branch", projection.Experience)
	}
	if projection.MinimumEducation.Status != "configured" || projection.MinimumEducation.Level != "bachelor" || projection.MinimumEducation.ReviewTier != "routine" {
		t.Fatalf("education recovery = %#v, want model projection without semantic reclassification", projection.MinimumEducation)
	}
	if len(projection.RequirementGroups) != 2 || projection.RequirementGroups[0].Role != "qualification" || !projection.RequirementGroups[0].CandidateRequirement || projection.RequirementGroups[0].ReviewTier != "routine" || len(projection.RequirementGroups[0].Options) != 0 || projection.RequirementGroups[1].Role != "responsibility" || projection.RequirementGroups[1].CandidateRequirement || projection.RequirementGroups[1].ReviewTier != "routine" || len(projection.RequirementGroups[1].Options) != 1 {
		t.Fatalf("requirement recovery = %#v, want the model groups with only invalid options removed", projection.RequirementGroups)
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
