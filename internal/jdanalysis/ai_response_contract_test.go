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

func TestAISemanticResponseContractAcceptsV2OnlyUnderV2Contract(t *testing.T) {
	contract, err := newAISemanticResponseContract(AIResponseSchemaVersionV2,
		"request-01", []string{"span-01"}, []string{"go"}, []string{"evidence-01"})
	if err != nil {
		t.Fatal(err)
	}
	raw := strings.Replace(validAISemanticResponseJSON(), AIResponseSchemaVersionV1, AIResponseSchemaVersionV2, 1)
	if _, err := contract.Validate([]byte(raw)); err != nil {
		t.Fatalf("v2 contract rejected its own valid response: %v", err)
	}

	v1 := NewAISemanticResponseContract("request-01", []string{"span-01"}, []string{"go"}, []string{"evidence-01"})
	if _, err := v1.Validate([]byte(raw)); err == nil || !strings.Contains(err.Error(), string(AISafeErrorInvalidResponseSchema)) {
		t.Fatalf("v1 contract accepted v2 response: %v", err)
	}
}

func TestAISemanticResponseContractV3RequiresSourceExactCandidateTerm(t *testing.T) {
	contract, err := newAISemanticResponseContract(AIResponseSchemaVersionV3,
		"request-01", []string{"span-01"}, []string{"go"}, []string{"evidence-01"})
	if err != nil {
		t.Fatal(err)
	}
	raw := strings.Replace(validAISemanticResponseJSON(), AIResponseSchemaVersionV1, AIResponseSchemaVersionV3, 1)
	raw = strings.Replace(raw, `"candidate_mentions":[]`, `"candidate_mentions":[{"candidate_text":"OriginalSDK","source_term":"OriginalSDK","span_id":"span-01","role":"qualification","necessity":"required","review_recommendation":"required_review"}]`, 1)
	if _, err := contract.Validate([]byte(raw)); err != nil {
		t.Fatalf("v3 contract rejected an exact source candidate: %v", err)
	}

	missingSourceTerm := strings.Replace(raw, `,"source_term":"OriginalSDK"`, "", 1)
	if _, err := contract.Validate([]byte(missingSourceTerm)); err == nil {
		t.Fatal("v3 contract accepted a candidate without source_term")
	}

	v2, err := newAISemanticResponseContract(AIResponseSchemaVersionV2,
		"request-01", []string{"span-01"}, []string{"go"}, []string{"evidence-01"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v2.Validate([]byte(raw)); err == nil {
		t.Fatal("v2 contract accepted a v3 candidate field")
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

func TestV2AISemanticContractAssetsFreezeCompactJSONInstruction(t *testing.T) {
	if AISystemPromptV2SHA256() == AISystemPromptV1SHA256() {
		t.Fatal("v2 system prompt digest must differ from the historical v1 prompt")
	}
	if AIResponseSchemaV2SHA256() == AIResponseSchemaV1SHA256() {
		t.Fatal("v2 response schema digest must differ from the historical v1 schema")
	}
	if !json.Valid([]byte(AIResponseSchemaV2())) || !strings.Contains(AIResponseSchemaV2(), `"const": "ai-jd-result-v2"`) || !strings.Contains(AIResponseSchemaV2(), `"additionalProperties": false`) {
		t.Fatal("v2 response schema is not a strict versioned JSON schema")
	}
	prompt := AISystemPromptV2()
	if !strings.Contains(prompt, AIResponseSchemaVersionV2) || !strings.Contains(prompt, "紧凑 JSON") || !strings.Contains(prompt, "不得输出缩进、换行、空行或无意义空白") {
		t.Fatal("v2 system prompt does not require compact JSON without formatting waste")
	}
	if !strings.Contains(prompt, "items=[]") || !strings.Contains(prompt, "省略所有该句的 domain items") || !strings.Contains(prompt, "不同片段中分别陈述的正向 required 资格") {
		t.Fatal("v2 system prompt does not retain the adjudicated experience and independent-requirement rules")
	}
}

func TestV3AISemanticContractAssetsFreezeCandidateSourceTermRules(t *testing.T) {
	if AISystemPromptV3SHA256() == AISystemPromptV2SHA256() || AIResponseSchemaV3SHA256() == AIResponseSchemaV2SHA256() {
		t.Fatal("v3 candidate-governance assets must have distinct frozen digests")
	}
	if !json.Valid([]byte(AIResponseSchemaV3())) || !strings.Contains(AIResponseSchemaV3(), `"const": "ai-jd-result-v3"`) || !strings.Contains(AIResponseSchemaV3(), `"source_term"`) {
		t.Fatal("v3 schema does not require the source-term candidate contract")
	}
	prompt := AISystemPromptV3()
	if !strings.Contains(prompt, AIResponseSchemaVersionV3) || !strings.Contains(prompt, "最多输出三个") || !strings.Contains(prompt, "source_term") || !strings.Contains(prompt, "职责") {
		t.Fatal("v3 prompt does not contain the adjudicated candidate-governance boundary")
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
