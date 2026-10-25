package jdanalysis

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	// AIInputContractVersionV1 identifies the CVM-owned request segment contract.
	AIInputContractVersionV1 = "ai-jd-input-v1"
	// AIResponseSchemaVersionV1 identifies the reviewed, strict response contract.
	AIResponseSchemaVersionV1 = "ai-jd-result-v1"
)

//go:embed assets/ai_system_prompt_v1.txt
var aiSystemPromptV1 string

//go:embed assets/ai_system_prompt_v2.txt
var aiSystemPromptV2 string

//go:embed assets/ai_response_schema_v1.json
var aiResponseSchemaV1 string

// AISemanticResponse is the in-memory result allowed by the v0.1 response
// contract. It intentionally contains only structured fields and opaque input
// references; no model-provided character locations are representable.
type AISemanticResponse struct {
	CandidateMentions           []AICandidateMentionResponse           `json:"candidate_mentions"`
	Experience                  AIExperienceResponse                   `json:"experience"`
	MinimumEducation            AIMinimumEducationResponse             `json:"minimum_education"`
	RequestID                   string                                 `json:"request_id"`
	ResponseSchemaVersion       string                                 `json:"response_schema_version"`
	TechnologyRequirementGroups []AITechnologyRequirementGroupResponse `json:"technology_requirement_groups"`
}

// AICandidateMentionResponse is a dictionary-external candidate that remains
// in memory until its text can be revalidated remotely.
type AICandidateMentionResponse struct {
	CandidateText        string `json:"candidate_text"`
	Necessity            string `json:"necessity"`
	ReviewRecommendation string `json:"review_recommendation"`
	Role                 string `json:"role"`
	SpanID               string `json:"span_id"`
}

// AIConditionalQualificationResponse is an explicit education/experience
// branch returned by the model.
type AIConditionalQualificationResponse struct {
	EvidenceSpanIDs      []string `json:"evidence_span_ids"`
	ExperienceScope      string   `json:"experience_scope"`
	ExperienceScopeKey   string   `json:"experience_scope_key"`
	MinYears             int      `json:"min_years"`
	ReviewRecommendation string   `json:"review_recommendation"`
	WhenEducation        string   `json:"when_education"`
}

// AIExperienceItemResponse is one explicit lower-bound experience conclusion.
type AIExperienceItemResponse struct {
	EvidenceSpanIDs      []string `json:"evidence_span_ids"`
	MinYears             int      `json:"min_years"`
	ReviewRecommendation string   `json:"review_recommendation"`
	ScopeKey             string   `json:"scope_key"`
	ScopeType            string   `json:"scope_type"`
}

// AIExperienceResponse groups the experience conclusions for one body.
type AIExperienceResponse struct {
	BaseYear                  *int                                 `json:"base_year"`
	ConditionalQualifications []AIConditionalQualificationResponse `json:"conditional_qualifications"`
	EvidenceSpanIDs           []string                             `json:"evidence_span_ids"`
	Items                     []AIExperienceItemResponse           `json:"items"`
	ReviewRecommendation      string                               `json:"review_recommendation"`
	Status                    string                               `json:"status"`
}

// AIMinimumEducationResponse is the one minimum-education conclusion for a
// body. Level and LevelCode remain nil when no conclusion is configured.
type AIMinimumEducationResponse struct {
	EvidenceSpanIDs      []string `json:"evidence_span_ids"`
	Level                *string  `json:"level"`
	LevelCode            *int     `json:"level_code"`
	ReviewRecommendation string   `json:"review_recommendation"`
	Status               string   `json:"status"`
}

// AITechnologyRequirementGroupResponse represents only a model claim; later
// validators decide which claims become persisted requirement groups.
type AITechnologyRequirementGroupResponse struct {
	EvidenceSpanIDs      []string                                `json:"evidence_span_ids"`
	LocalID              string                                  `json:"local_id"`
	MinSatisfied         *int                                    `json:"min_satisfied"`
	Necessity            string                                  `json:"necessity"`
	Operator             *string                                 `json:"operator"`
	Options              []AITechnologyRequirementOptionResponse `json:"options"`
	ReviewRecommendation string                                  `json:"review_recommendation"`
	Role                 string                                  `json:"role"`
}

// AITechnologyRequirementOptionResponse references a verified deterministic
// technology mention rather than supplying a coordinate or a new technology ID.
type AITechnologyRequirementOptionResponse struct {
	MentionEvidenceIDs []string `json:"mention_evidence_ids"`
	Proficiency        *string  `json:"proficiency"`
	TechnologyKey      string   `json:"technology_key"`
}

// AISemanticResponseContract holds the opaque IDs available to one model
// request. It has no body text, source location or credential fields.
type AISemanticResponseContract struct {
	knownMentionEvidenceIDs map[string]struct{}
	knownMentionReferences  map[string]aiMentionReference
	knownSpanIDs            map[string]struct{}
	knownTechnologyKeys     map[string]struct{}
	requestID               string
}

// NewAISemanticResponseContract makes the public validation seam for one
// prebuilt model request. Identifiers not supplied here cannot be accepted from
// the model response.
func NewAISemanticResponseContract(requestID string, spanIDs, technologyKeys, mentionEvidenceIDs []string) AISemanticResponseContract {
	return AISemanticResponseContract{
		knownMentionEvidenceIDs: toIdentifierSet(mentionEvidenceIDs),
		knownSpanIDs:            toIdentifierSet(spanIDs),
		knownTechnologyKeys:     toIdentifierSet(technologyKeys),
		requestID:               requestID,
	}
}

type aiMentionReference struct {
	spanID        string
	technologyKey string
}

// AIResponseSchemaV1 returns the reviewed canonical strict JSON Schema. It is
// a controlled repository asset, not a model-provided schema.
func AIResponseSchemaV1() string {
	return aiResponseSchemaV1
}

// AIResponseSchemaV1SHA256 returns the stable digest recorded in an immutable
// AI analysis configuration.
func AIResponseSchemaV1SHA256() string {
	return sha256Hex(aiResponseSchemaV1)
}

// AISystemPromptV1 returns the reviewed system prompt used with the v0.1
// contract. It is a controlled repository asset, never a model response.
func AISystemPromptV1() string {
	return aiSystemPromptV1
}

// AISystemPromptV1SHA256 returns the stable digest recorded in an immutable AI
// analysis configuration.
func AISystemPromptV1SHA256() string {
	return sha256Hex(aiSystemPromptV1)
}

// AISystemPromptV2 returns the active reviewed prompt. It keeps the v1 asset
// available for immutable historic configurations and their CVM-only audits.
func AISystemPromptV2() string {
	return aiSystemPromptV2
}

// AISystemPromptV2SHA256 returns the digest frozen into newly prepared AI
// configurations that use the active prompt.
func AISystemPromptV2SHA256() string {
	return sha256Hex(aiSystemPromptV2)
}

// Validate accepts only one strict v0.1 JSON response whose opaque references
// belong to this exact request. Raw response text is deliberately discarded on
// every error path.
func (contract AISemanticResponseContract) Validate(raw []byte) (AISemanticResponse, error) {
	if err := validateAISemanticResponseShape(raw); err != nil {
		return AISemanticResponse{}, NewSafeAIError(AISafeErrorInvalidResponseSchema, err)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var response AISemanticResponse
	if err := decoder.Decode(&response); err != nil {
		return AISemanticResponse{}, NewSafeAIError(AISafeErrorInvalidResponseSchema, err)
	}
	if response.ResponseSchemaVersion != AIResponseSchemaVersionV1 || response.RequestID != contract.requestID {
		return AISemanticResponse{}, NewSafeAIError(AISafeErrorInvalidResponseSchema, nil)
	}
	if err := contract.validateReferences(response); err != nil {
		return AISemanticResponse{}, NewSafeAIError(AISafeErrorInvalidEvidence, err)
	}
	return response, nil
}

func (contract AISemanticResponseContract) validateReferences(response AISemanticResponse) error {
	if err := contract.validateSpanIDs(response.Experience.EvidenceSpanIDs); err != nil {
		return err
	}
	for _, item := range response.Experience.Items {
		if err := contract.validateSpanIDs(item.EvidenceSpanIDs); err != nil {
			return err
		}
		if item.ScopeType == "technology" && !contract.hasTechnologyKey(item.ScopeKey) {
			return fmt.Errorf("unknown technology key")
		}
	}
	for _, conditional := range response.Experience.ConditionalQualifications {
		if err := contract.validateSpanIDs(conditional.EvidenceSpanIDs); err != nil {
			return err
		}
		if conditional.ExperienceScope == "technology" && !contract.hasTechnologyKey(conditional.ExperienceScopeKey) {
			return fmt.Errorf("unknown technology key")
		}
	}
	if err := contract.validateSpanIDs(response.MinimumEducation.EvidenceSpanIDs); err != nil {
		return err
	}
	for _, group := range response.TechnologyRequirementGroups {
		if err := contract.validateSpanIDs(group.EvidenceSpanIDs); err != nil {
			return err
		}
		for _, option := range group.Options {
			if !contract.hasTechnologyKey(option.TechnologyKey) {
				return fmt.Errorf("unknown technology key")
			}
			for _, evidenceID := range option.MentionEvidenceIDs {
				if _, exists := contract.knownMentionEvidenceIDs[evidenceID]; !exists {
					return fmt.Errorf("unknown mention evidence ID")
				}
				if reference, exists := contract.knownMentionReferences[evidenceID]; exists && reference.technologyKey != option.TechnologyKey {
					return fmt.Errorf("technology key does not match mention evidence")
				}
			}
		}
	}
	for _, candidate := range response.CandidateMentions {
		if err := contract.validateSpanIDs([]string{candidate.SpanID}); err != nil {
			return err
		}
	}
	return nil
}

func (contract AISemanticResponseContract) hasTechnologyKey(key string) bool {
	_, exists := contract.knownTechnologyKeys[key]
	return exists
}

func (contract AISemanticResponseContract) validateSpanIDs(spanIDs []string) error {
	for _, spanID := range spanIDs {
		if _, exists := contract.knownSpanIDs[spanID]; !exists {
			return fmt.Errorf("unknown span ID")
		}
	}
	return nil
}

func validateAISemanticResponseShape(raw []byte) error {
	root, err := strictObject(raw, []string{
		"candidate_mentions", "experience", "minimum_education", "request_id", "response_schema_version", "technology_requirement_groups",
	})
	if err != nil {
		return err
	}
	if err := strictString(root["response_schema_version"], AIResponseSchemaVersionV1); err != nil {
		return err
	}
	if err := strictNonEmptyString(root["request_id"]); err != nil {
		return err
	}
	if err := validateExperienceShape(root["experience"]); err != nil {
		return err
	}
	if err := validateMinimumEducationShape(root["minimum_education"]); err != nil {
		return err
	}
	if err := strictArray(root["technology_requirement_groups"], validateTechnologyRequirementGroupShape); err != nil {
		return err
	}
	return strictArray(root["candidate_mentions"], validateCandidateMentionShape)
}

func validateCandidateMentionShape(raw json.RawMessage) error {
	object, err := strictObject(raw, []string{"candidate_text", "necessity", "review_recommendation", "role", "span_id"})
	if err != nil {
		return err
	}
	if err := strictNonEmptyString(object["candidate_text"]); err != nil {
		return err
	}
	if err := strictEnum(object["necessity"], "required", "preferred", "unspecified"); err != nil {
		return err
	}
	if err := strictEnum(object["review_recommendation"], "required_review"); err != nil {
		return err
	}
	if err := strictEnum(object["role"], "qualification", "responsibility", "mention", "unspecified"); err != nil {
		return err
	}
	return strictNonEmptyString(object["span_id"])
}

func validateConditionalQualificationShape(raw json.RawMessage) error {
	object, err := strictObject(raw, []string{"evidence_span_ids", "experience_scope", "experience_scope_key", "min_years", "review_recommendation", "when_education"})
	if err != nil {
		return err
	}
	if err := strictStringArray(object["evidence_span_ids"]); err != nil {
		return err
	}
	if err := strictEnum(object["experience_scope"], "base", "technology", "domain"); err != nil {
		return err
	}
	if err := strictNonEmptyString(object["experience_scope_key"]); err != nil {
		return err
	}
	if err := strictPositiveInteger(object["min_years"]); err != nil {
		return err
	}
	if err := strictReviewRecommendation(object["review_recommendation"]); err != nil {
		return err
	}
	return strictEnum(object["when_education"], "junior_college_or_below", "bachelor", "master", "doctor")
}

func validateExperienceItemShape(raw json.RawMessage) error {
	object, err := strictObject(raw, []string{"evidence_span_ids", "min_years", "review_recommendation", "scope_key", "scope_type"})
	if err != nil {
		return err
	}
	if err := strictStringArray(object["evidence_span_ids"]); err != nil {
		return err
	}
	if err := strictPositiveInteger(object["min_years"]); err != nil {
		return err
	}
	if err := strictReviewRecommendation(object["review_recommendation"]); err != nil {
		return err
	}
	if err := strictNonEmptyString(object["scope_key"]); err != nil {
		return err
	}
	return strictEnum(object["scope_type"], "base", "technology", "domain")
}

func validateExperienceShape(raw json.RawMessage) error {
	object, err := strictObject(raw, []string{"base_year", "conditional_qualifications", "evidence_span_ids", "items", "review_recommendation", "status"})
	if err != nil {
		return err
	}
	if err := strictNullablePositiveInteger(object["base_year"]); err != nil {
		return err
	}
	if err := strictArray(object["conditional_qualifications"], validateConditionalQualificationShape); err != nil {
		return err
	}
	if err := strictStringArray(object["evidence_span_ids"]); err != nil {
		return err
	}
	if err := strictArray(object["items"], validateExperienceItemShape); err != nil {
		return err
	}
	if err := strictReviewRecommendation(object["review_recommendation"]); err != nil {
		return err
	}
	return strictEnum(object["status"], "configured", "not_configured", "uncertain")
}

func validateMinimumEducationShape(raw json.RawMessage) error {
	object, err := strictObject(raw, []string{"evidence_span_ids", "level", "level_code", "review_recommendation", "status"})
	if err != nil {
		return err
	}
	if err := strictStringArray(object["evidence_span_ids"]); err != nil {
		return err
	}
	if err := strictNullableEnum(object["level"], "junior_college_or_below", "bachelor", "master", "doctor"); err != nil {
		return err
	}
	if err := strictNullableIntegerIn(object["level_code"], 1, 2, 3, 4); err != nil {
		return err
	}
	if err := strictReviewRecommendation(object["review_recommendation"]); err != nil {
		return err
	}
	return strictEnum(object["status"], "configured", "not_configured", "uncertain")
}

func validateTechnologyRequirementGroupShape(raw json.RawMessage) error {
	object, err := strictObject(raw, []string{"evidence_span_ids", "local_id", "min_satisfied", "necessity", "operator", "options", "review_recommendation", "role"})
	if err != nil {
		return err
	}
	if err := strictStringArray(object["evidence_span_ids"]); err != nil {
		return err
	}
	if err := strictNonEmptyString(object["local_id"]); err != nil {
		return err
	}
	if err := strictNullablePositiveInteger(object["min_satisfied"]); err != nil {
		return err
	}
	if err := strictEnum(object["necessity"], "required", "preferred", "unspecified"); err != nil {
		return err
	}
	if err := strictNullableEnum(object["operator"], "at_least"); err != nil {
		return err
	}
	if err := strictArray(object["options"], validateTechnologyRequirementOptionShape); err != nil {
		return err
	}
	if err := strictReviewRecommendation(object["review_recommendation"]); err != nil {
		return err
	}
	if err := strictEnum(object["role"], "qualification", "responsibility", "mention", "unspecified"); err != nil {
		return err
	}

	var options []json.RawMessage
	if err := json.Unmarshal(object["options"], &options); err != nil {
		return err
	}
	var operator *string
	if err := json.Unmarshal(object["operator"], &operator); err != nil {
		return err
	}
	var minSatisfied *int
	if err := json.Unmarshal(object["min_satisfied"], &minSatisfied); err != nil {
		return err
	}
	var role string
	if err := json.Unmarshal(object["role"], &role); err != nil {
		return err
	}
	if (operator == nil) != (minSatisfied == nil) {
		return fmt.Errorf("requirement operator and minimum satisfaction must be paired")
	}
	if operator != nil && (*minSatisfied > len(options) || len(options) == 0) {
		return fmt.Errorf("requirement minimum satisfaction exceeds its options")
	}
	if role == "qualification" && (operator == nil || minSatisfied == nil || len(options) == 0) {
		return fmt.Errorf("qualification group is incomplete")
	}
	return nil
}

func validateTechnologyRequirementOptionShape(raw json.RawMessage) error {
	object, err := strictObject(raw, []string{"mention_evidence_ids", "proficiency", "technology_key"})
	if err != nil {
		return err
	}
	if err := strictStringArray(object["mention_evidence_ids"]); err != nil {
		return err
	}
	if err := strictNullableEnum(object["proficiency"], "P1", "P2", "P3", "P4"); err != nil {
		return err
	}
	return strictNonEmptyString(object["technology_key"])
}

func strictArray(raw json.RawMessage, validateItem func(json.RawMessage) error) error {
	if !isJSONArray(raw) {
		return fmt.Errorf("value must be an array")
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return err
	}
	for _, item := range items {
		if err := validateItem(item); err != nil {
			return err
		}
	}
	return nil
}

func strictEnum(raw json.RawMessage, allowed ...string) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	for _, candidate := range allowed {
		if value == candidate {
			return nil
		}
	}
	return fmt.Errorf("value is outside the allowed enum")
}

func strictNonEmptyString(raw json.RawMessage) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	if value == "" {
		return fmt.Errorf("string must not be empty")
	}
	return nil
}

func strictNullableEnum(raw json.RawMessage, allowed ...string) error {
	if isJSONNull(raw) {
		return nil
	}
	return strictEnum(raw, allowed...)
}

func strictNullableIntegerIn(raw json.RawMessage, allowed ...int) error {
	if isJSONNull(raw) {
		return nil
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	for _, candidate := range allowed {
		if value == candidate {
			return nil
		}
	}
	return fmt.Errorf("integer is outside the allowed enum")
}

func strictNullablePositiveInteger(raw json.RawMessage) error {
	if isJSONNull(raw) {
		return nil
	}
	return strictPositiveInteger(raw)
}

func strictObject(raw json.RawMessage, required []string) (map[string]json.RawMessage, error) {
	if !isJSONObject(raw) {
		return nil, fmt.Errorf("value must be an object")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(required))
	for _, field := range required {
		allowed[field] = struct{}{}
		if _, exists := object[field]; !exists {
			return nil, fmt.Errorf("required field is missing")
		}
	}
	for field := range object {
		if _, exists := allowed[field]; !exists {
			return nil, fmt.Errorf("unknown field")
		}
	}
	return object, nil
}

func strictPositiveInteger(raw json.RawMessage) error {
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	if value <= 0 {
		return fmt.Errorf("integer must be positive")
	}
	return nil
}

func strictReviewRecommendation(raw json.RawMessage) error {
	return strictEnum(raw, "routine", "spot_check", "required_review")
}

func strictString(raw json.RawMessage, expected string) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	if value != expected {
		return fmt.Errorf("unexpected string")
	}
	return nil
}

func strictStringArray(raw json.RawMessage) error {
	return strictArray(raw, strictNonEmptyString)
}

func isJSONArray(raw json.RawMessage) bool {
	return strings.HasPrefix(strings.TrimSpace(string(raw)), "[")
}

func isJSONNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}

func isJSONObject(raw json.RawMessage) bool {
	return strings.HasPrefix(strings.TrimSpace(string(raw)), "{")
}

func sha256Hex(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func toIdentifierSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != "" {
			result[value] = struct{}{}
		}
	}
	return result
}
