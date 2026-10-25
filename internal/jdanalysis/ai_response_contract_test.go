package jdanalysis

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAISemanticResponseContractAcceptsMinimalValidResponse(t *testing.T) {
	contract := NewAISemanticResponseContract(
		"request-01",
		[]string{"span-01"},
		[]string{"go"},
		[]string{"evidence-01"},
	)

	response, err := contract.Validate([]byte(validAISemanticResponseJSON()))
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if response.RequestID != "request-01" {
		t.Fatalf("RequestID = %q, want request-01", response.RequestID)
	}
}

func TestAISemanticResponseContractRejectsUntrustedModelOutput(t *testing.T) {
	contract := NewAISemanticResponseContract(
		"request-01",
		[]string{"span-01"},
		[]string{"go"},
		[]string{"evidence-01"},
	)

	for _, test := range []struct {
		name          string
		raw           string
		wantErrorCode AISafeErrorCode
	}{
		{
			name:          "Markdown prefix",
			raw:           "Here is the result:\n" + validAISemanticResponseJSON(),
			wantErrorCode: AISafeErrorInvalidResponseSchema,
		},
		{
			name:          "model probability",
			raw:           strings.Replace(validAISemanticResponseJSON(), `"candidate_mentions":[]`, `"candidate_mentions":[],"confidence":0.9`, 1),
			wantErrorCode: AISafeErrorInvalidResponseSchema,
		},
		{
			name:          "model coordinate",
			raw:           strings.Replace(validAISemanticResponseJSON(), `"evidence_span_ids":[]`, `"evidence_span_ids":[],"codepoint_start":0`, 1),
			wantErrorCode: AISafeErrorInvalidResponseSchema,
		},
		{
			name:          "nested extra field",
			raw:           strings.Replace(validAISemanticResponseJSON(), `"review_recommendation":"routine"`, `"review_recommendation":"routine","explanation":"synthetic"`, 1),
			wantErrorCode: AISafeErrorInvalidResponseSchema,
		},
		{
			name:          "mismatched request ID",
			raw:           strings.Replace(validAISemanticResponseJSON(), `"request_id":"request-01"`, `"request_id":"request-unknown"`, 1),
			wantErrorCode: AISafeErrorInvalidResponseSchema,
		},
		{
			name:          "unknown span ID",
			raw:           strings.Replace(validAISemanticResponseJSON(), `"evidence_span_ids":[]`, `"evidence_span_ids":["span-unknown"]`, 1),
			wantErrorCode: AISafeErrorInvalidEvidence,
		},
		{
			name:          "unknown technology key",
			raw:           validAISemanticResponseWithOption(`"technology-unknown"`, `"evidence-01"`),
			wantErrorCode: AISafeErrorInvalidEvidence,
		},
		{
			name:          "unknown technology evidence ID",
			raw:           validAISemanticResponseWithOption(`"go"`, `"evidence-unknown"`),
			wantErrorCode: AISafeErrorInvalidEvidence,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := contract.Validate([]byte(test.raw))
			if err == nil {
				t.Fatal("Validate() accepted an untrusted response")
			}
			if !strings.Contains(err.Error(), string(test.wantErrorCode)) {
				t.Fatalf("error = %q, want approved code %q", err, test.wantErrorCode)
			}
		})
	}
}

func TestAISemanticResponseContractAcceptsBusinessInconsistenciesForFieldLevelRecovery(t *testing.T) {
	contract := NewAISemanticResponseContract(
		"request-01",
		[]string{"span-01"},
		[]string{"go"},
		[]string{"evidence-01"},
	)

	for _, test := range []struct {
		name string
		raw  string
	}{
		{
			name: "configured experience without item",
			raw: `{
				"response_schema_version":"ai-jd-result-v1","request_id":"request-01",
				"experience":{"status":"configured","base_year":3,"items":[],"conditional_qualifications":[],"evidence_span_ids":["span-01"],"review_recommendation":"routine"},
				"minimum_education":{"status":"not_configured","level":null,"level_code":null,"evidence_span_ids":[],"review_recommendation":"routine"},
				"technology_requirement_groups":[],"candidate_mentions":[]
			}`,
		},
		{
			name: "inconsistent configured education code",
			raw: `{
				"response_schema_version":"ai-jd-result-v1","request_id":"request-01",
				"experience":{"status":"not_configured","base_year":null,"items":[],"conditional_qualifications":[],"evidence_span_ids":[],"review_recommendation":"routine"},
				"minimum_education":{"status":"configured","level":"bachelor","level_code":3,"evidence_span_ids":["span-01"],"review_recommendation":"routine"},
				"technology_requirement_groups":[],"candidate_mentions":[]
			}`,
		},
		{
			name: "non overall education condition",
			raw: `{
				"response_schema_version":"ai-jd-result-v1","request_id":"request-01",
				"experience":{"status":"configured","base_year":3,"items":[{"scope_type":"base","scope_key":"overall","min_years":3,"evidence_span_ids":["span-01"],"review_recommendation":"routine"}],"conditional_qualifications":[{"when_education":"bachelor","experience_scope":"technology","experience_scope_key":"go","min_years":3,"evidence_span_ids":["span-01"],"review_recommendation":"required_review"}],"evidence_span_ids":["span-01"],"review_recommendation":"required_review"},
				"minimum_education":{"status":"not_configured","level":null,"level_code":null,"evidence_span_ids":[],"review_recommendation":"routine"},
				"technology_requirement_groups":[],"candidate_mentions":[]
			}`,
		},
		{
			name: "qualification group without option",
			raw: `{
				"response_schema_version":"ai-jd-result-v1","request_id":"request-01",
				"experience":{"status":"not_configured","base_year":null,"items":[],"conditional_qualifications":[],"evidence_span_ids":[],"review_recommendation":"routine"},
				"minimum_education":{"status":"not_configured","level":null,"level_code":null,"evidence_span_ids":[],"review_recommendation":"routine"},
				"technology_requirement_groups":[{"local_id":"group-01","role":"qualification","necessity":"required","operator":"at_least","min_satisfied":1,"options":[],"evidence_span_ids":["span-01"],"review_recommendation":"routine"}],"candidate_mentions":[]
			}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := contract.Validate([]byte(test.raw)); err != nil {
				t.Fatalf("Validate() error = %v, want structurally valid response retained for field-level recovery", err)
			}
		})
	}
}

func TestV1AISemanticContractAssetsAreFrozenAndStrict(t *testing.T) {
	if got, want := AISystemPromptV1SHA256(), "c29802261906be7c55a54be1a5a663e87621783d78afae2b9afa06bb66a346ac"; got != want {
		t.Fatalf("system prompt SHA-256 = %q, want %q", got, want)
	}
	if got, want := AIResponseSchemaV1SHA256(), "5626a0d118d070d9146f6db8de5225528330aad19556390e39d2cd3f92ce0737"; got != want {
		t.Fatalf("response schema SHA-256 = %q, want %q", got, want)
	}
	if !json.Valid([]byte(AIResponseSchemaV1())) {
		t.Fatal("canonical response schema is not valid JSON")
	}
	if strings.Contains(AIResponseSchemaV1(), "confidence") || strings.Contains(AIResponseSchemaV1(), "codepoint_") {
		t.Fatal("canonical response schema permits model confidence or coordinates")
	}
	if !strings.Contains(AIResponseSchemaV1(), `"additionalProperties": false`) {
		t.Fatal("canonical response schema is not strict")
	}
	if !strings.Contains(AISystemPromptV1(), "不得输出 Markdown") || !strings.Contains(AISystemPromptV1(), AIResponseSchemaVersionV1) {
		t.Fatal("system prompt does not enforce the reviewed v0.1 response contract")
	}
	if !strings.Contains(AISystemPromptV1(), "无法调和") || !strings.Contains(AISystemPromptV1(), "优先采用 jobRequire") {
		t.Fatal("system prompt does not retain the reviewed jobRequire conflict-precedence policy")
	}
}

func TestV2AISystemPromptPreservesTheFrozenV1AssetAndAddsRepresentableScopes(t *testing.T) {
	if got, want := AISystemPromptV2SHA256(), "ddbcc4547b4d69aaca41ba1b1e33224bbd9b59edd46c64bed7709f072b062216"; got != want {
		t.Fatalf("v2 system prompt SHA-256 = %q, want %q", got, want)
	}
	if AISystemPromptV2() == AISystemPromptV1() {
		t.Fatal("v2 system prompt did not create a distinct immutable candidate identity")
	}
	for _, instruction := range []string{"不得翻译、归一化、加下划线", "experience_scope=base", "experience_scope_key=overall"} {
		if !strings.Contains(AISystemPromptV2(), instruction) {
			t.Fatalf("v2 system prompt is missing representable-scope instruction %q", instruction)
		}
	}
}

func TestV3AISystemPromptAddsCrossFieldContractInstructions(t *testing.T) {
	if got, want := AISystemPromptV3SHA256(), "9e2f84cd0b01902f819e828d44ff005080f0c732393f6e4acc51691415437903"; got != want {
		t.Fatalf("v3 system prompt SHA-256 = %q, want %q", got, want)
	}
	if AISystemPromptV3() == AISystemPromptV2() {
		t.Fatal("v3 system prompt did not create a distinct immutable candidate identity")
	}
	for _, instruction := range []string{"至少输出一项 items", "bachelor=2", "role=unspecified"} {
		if !strings.Contains(AISystemPromptV3(), instruction) {
			t.Fatalf("v3 system prompt is missing cross-field contract instruction %q", instruction)
		}
	}
}

func validAISemanticResponseJSON() string {
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
		"technology_requirement_groups":[],
		"candidate_mentions":[]
	}`
}

func validAISemanticResponseWithOption(technologyKey, evidenceID string) string {
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
			"min_satisfied":1,
			"options":[{
				"technology_key":` + technologyKey + `,
				"proficiency":null,
				"mention_evidence_ids":[` + evidenceID + `]
			}],
			"evidence_span_ids":["span-01"],
			"review_recommendation":"routine"
		}],
		"candidate_mentions":[]
	}`
}
