package jdanalysis

import (
	"strconv"
	"strings"
	"testing"
)

func TestAIModelInputBuildsOnlyVerifiedMentionsAndResolvesProgramOwnedEvidence(t *testing.T) {
	body := syntheticAISpanBody("Use Go and NewTool.", "", "")
	spans, err := NewAISpanIndex(body)
	if err != nil {
		t.Fatal(err)
	}
	input, err := BuildAIModelInput("request-01", body, spans, []VerifiedTechnologyMention{
		{
			BodyContentVersionID:  body.BodyContentVersionID,
			CanonicalName:         "Go",
			CodePointRange:        CodePointRange{Start: 4, End: 6},
			MentionEvidenceID:     "evidence-go",
			NormalizationVersion:  body.NormalizationVersion,
			NormalizedFieldSHA256: body.DescriptionSHA256,
			SourceField:           SourceFieldDescription,
			TechnologyKey:         "go",
		},
	})
	if err != nil {
		t.Fatalf("BuildAIModelInput() error = %v", err)
	}
	if got, want := input.InputContractVersion, AIInputContractVersionV1; got != want {
		t.Fatalf("InputContractVersion = %q, want %q", got, want)
	}
	if got := input.VerifiedTechnologyMentions; len(got) != 1 || got[0].MentionEvidenceID != "evidence-go" || got[0].SpanID != "description:s01" {
		t.Fatalf("VerifiedTechnologyMentions = %#v, want the single actual deterministic hit", got)
	}

	response, err := input.ValidateResponse([]byte(validAIModelResponseWithGoOption(1)))
	if err != nil {
		t.Fatalf("ValidateResponse() error = %v", err)
	}
	evidence, err := input.ResolveSpanIDs(response.TechnologyRequirementGroups[0].EvidenceSpanIDs)
	if err != nil {
		t.Fatalf("ResolveSpanIDs() error = %v", err)
	}
	wantEvidence := AISemanticEvidence{
		CodePointRange:        CodePointRange{Start: 0, End: 18},
		NormalizedFieldSHA256: body.DescriptionSHA256,
		NormalizationVersion:  body.NormalizationVersion,
		SourceField:           SourceFieldDescription,
		SpanID:                "description:s01",
	}
	if len(evidence) != 1 || evidence[0] != wantEvidence {
		t.Fatalf("ResolveSpanIDs() = %#v, want %#v", evidence, []AISemanticEvidence{wantEvidence})
	}

	candidateEvidence, err := input.LocateCandidate("NewTool", "description:s01")
	if err != nil {
		t.Fatalf("LocateCandidate() error = %v", err)
	}
	if got, want := candidateEvidence.CodePointRange, (CodePointRange{Start: 11, End: 18}); got != want {
		t.Fatalf("candidate range = %#v, want %#v", got, want)
	}
	if _, err := input.LocateCandidate("not-present", "description:s01"); err == nil {
		t.Fatal("LocateCandidate() accepted a candidate not exactly recoverable from its span")
	}
}

func TestAIModelInputRejectsCrossBodyMentionsAndKeepsModelRequirementCounts(t *testing.T) {
	body := syntheticAISpanBody("Use Go.", "", "")
	spans, err := NewAISpanIndex(body)
	if err != nil {
		t.Fatal(err)
	}

	_, err = BuildAIModelInput("request-01", body, spans, []VerifiedTechnologyMention{{
		BodyContentVersionID:  "another-body-version",
		CanonicalName:         "Go",
		CodePointRange:        CodePointRange{Start: 4, End: 6},
		MentionEvidenceID:     "evidence-go",
		NormalizationVersion:  body.NormalizationVersion,
		NormalizedFieldSHA256: body.DescriptionSHA256,
		SourceField:           SourceFieldDescription,
		TechnologyKey:         "go",
	}})
	if err == nil || !strings.Contains(err.Error(), string(AISafeErrorInvalidEvidence)) {
		t.Fatalf("cross-body mention error = %v, want redacted invalid evidence error", err)
	}

	input, err := BuildAIModelInput("request-01", body, spans, []VerifiedTechnologyMention{{
		BodyContentVersionID:  body.BodyContentVersionID,
		CanonicalName:         "Go",
		CodePointRange:        CodePointRange{Start: 4, End: 6},
		MentionEvidenceID:     "evidence-go",
		NormalizationVersion:  body.NormalizationVersion,
		NormalizedFieldSHA256: body.DescriptionSHA256,
		SourceField:           SourceFieldDescription,
		TechnologyKey:         "go",
	}})
	if err != nil {
		t.Fatal(err)
	}
	response, err := input.ValidateAndMapResponse([]byte(validAIModelResponseWithGoOption(2)))
	if err != nil {
		t.Fatalf("ValidateAndMapResponse() error = %v, want model relation retained", err)
	}
	if len(response.RequirementGroups) != 1 || response.RequirementGroups[0].MinSatisfied != 2 || len(response.RequirementGroups[0].Options) != 1 {
		t.Fatalf("requirement groups = %#v, want the model-supplied at_least=2 relation", response.RequirementGroups)
	}
}

func validAIModelResponseWithGoOption(minSatisfied int) string {
	return `{
		"response_schema_version":"ai-jd-result-v1",
		"request_id":"request-01",
		"experience":{
			"status":"not_configured",
			"base_year":null,
			"items":[],
			"conditional_qualifications":[],
			"evidence_span_ids":[],
			"review_recommendation":"routine"
		},
		"minimum_education":{
			"status":"not_configured",
			"level":null,
			"level_code":null,
			"evidence_span_ids":[],
			"review_recommendation":"routine"
		},
		"technology_requirement_groups":[{
			"local_id":"group-01",
			"role":"qualification",
			"necessity":"required",
			"operator":"at_least",
			"min_satisfied":` + strconv.Itoa(minSatisfied) + `,
			"options":[{
				"technology_key":"go",
				"proficiency":null,
				"mention_evidence_ids":["evidence-go"]
			}],
			"evidence_span_ids":["description:s01"],
			"review_recommendation":"routine"
		}],
		"candidate_mentions":[]
	}`
}
