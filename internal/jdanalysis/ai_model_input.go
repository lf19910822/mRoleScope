package jdanalysis

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// AIModelInput is the only in-memory payload allowed to cross from the CVM to
// a model adapter. It contains three normalized-field spans and verified
// deterministic hits only; it must never be persisted, logged, or returned by
// an operations/status command.
type AIModelInput struct {
	BodyContentVersionID       string                      `json:"body_content_version_id"`
	Fields                     []AIModelInputField         `json:"fields"`
	InputContractVersion       string                      `json:"input_contract_version"`
	RequestID                  string                      `json:"request_id"`
	VerifiedTechnologyMentions []VerifiedTechnologyMention `json:"verified_technology_mentions"`

	fieldTextBySource map[string]string
	responseContract  AISemanticResponseContract
	spanIndex         AISpanIndex
}

// AIModelInputField contains the model-visible text for one approved field.
type AIModelInputField struct {
	SourceField string             `json:"source_field"`
	Spans       []AIModelInputSpan `json:"spans"`
}

// AIModelInputSpan is a transient, model-visible rendering of a program-owned
// span. Its text must not cross a persistence or observability seam.
type AIModelInputSpan struct {
	SpanID string `json:"span_id"`
	Text   string `json:"text"`
}

// AISemanticEvidence is the text-free, program-owned semantic evidence that
// later persistence stores for an accepted conclusion.
type AISemanticEvidence struct {
	CodePointRange        CodePointRange
	NormalizedFieldSHA256 string
	NormalizationVersion  string
	SourceField           string
	SpanID                string
}

// VerifiedTechnologyMention is a deterministic evidence hit that can be
// submitted to the model. The provenance-only fields are intentionally omitted
// from JSON; BuildAIModelInput verifies them before producing model input.
type VerifiedTechnologyMention struct {
	BodyContentVersionID  string         `json:"-"`
	CanonicalName         string         `json:"canonical_name"`
	CodePointRange        CodePointRange `json:"-"`
	MentionEvidenceID     string         `json:"mention_evidence_id"`
	NormalizationVersion  string         `json:"-"`
	NormalizedFieldSHA256 string         `json:"-"`
	SourceField           string         `json:"source_field"`
	SpanID                string         `json:"span_id"`
	TechnologyKey         string         `json:"technology_key"`
}

// BuildAIModelInput constructs the transient, controlled model input for one
// exact body version. It rejects any evidence not tied to the supplied body,
// normalized field hash, and CVM-generated span map.
func BuildAIModelInput(requestID string, body AIInputBody, spanIndex AISpanIndex, mentions []VerifiedTechnologyMention) (AIModelInput, error) {
	return buildAIModelInputForResponseSchemaVersion(AIResponseSchemaVersionV1, requestID, body, spanIndex, mentions)
}

func buildAIModelInputForResponseSchemaVersion(responseSchemaVersion, requestID string, body AIInputBody, spanIndex AISpanIndex, mentions []VerifiedTechnologyMention) (AIModelInput, error) {
	if !knownAIResponseSchemaVersion(responseSchemaVersion) {
		return AIModelInput{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	if requestID == "" || spanIndex.bodyContentVersionID != body.BodyContentVersionID {
		return AIModelInput{}, NewSafeAIError(AISafeErrorInvalidEvidence, nil)
	}
	if err := validateSpanIndexForBody(body, spanIndex); err != nil {
		return AIModelInput{}, NewSafeAIError(AISafeErrorInvalidEvidence, err)
	}

	input := AIModelInput{
		BodyContentVersionID: body.BodyContentVersionID,
		InputContractVersion: AIInputContractVersionV1,
		RequestID:            requestID,
		fieldTextBySource: map[string]string{
			SourceFieldDescription: body.Description,
			SourceFieldJobRequire:  body.JobRequire,
			SourceFieldAddition:    body.Addition,
		},
		spanIndex: spanIndex,
	}
	input.Fields = buildAIModelInputFields(body, spanIndex)
	input.VerifiedTechnologyMentions = make([]VerifiedTechnologyMention, 0, len(mentions))
	mentionReferences := make(map[string]aiMentionReference, len(mentions))
	technologyKeys := make([]string, 0, len(mentions))
	for _, mention := range mentions {
		span, err := validateVerifiedTechnologyMention(body, spanIndex, mention)
		if err != nil {
			return AIModelInput{}, NewSafeAIError(AISafeErrorInvalidEvidence, err)
		}
		if _, duplicate := mentionReferences[mention.MentionEvidenceID]; duplicate {
			return AIModelInput{}, NewSafeAIError(AISafeErrorInvalidEvidence, nil)
		}
		mention.SpanID = span.ID
		input.VerifiedTechnologyMentions = append(input.VerifiedTechnologyMentions, mention)
		mentionReferences[mention.MentionEvidenceID] = aiMentionReference{spanID: span.ID, technologyKey: mention.TechnologyKey}
		technologyKeys = append(technologyKeys, mention.TechnologyKey)
	}
	sort.Slice(input.VerifiedTechnologyMentions, func(left, right int) bool {
		leftMention := input.VerifiedTechnologyMentions[left]
		rightMention := input.VerifiedTechnologyMentions[right]
		if leftMention.SourceField != rightMention.SourceField {
			return aiSourceFieldOrder(leftMention.SourceField) < aiSourceFieldOrder(rightMention.SourceField)
		}
		if leftMention.SpanID != rightMention.SpanID {
			return leftMention.SpanID < rightMention.SpanID
		}
		return leftMention.MentionEvidenceID < rightMention.MentionEvidenceID
	})
	spanIDs := make([]string, 0, len(spanIndex.spans))
	for _, span := range spanIndex.spans {
		spanIDs = append(spanIDs, span.ID)
	}
	mentionEvidenceIDs := make([]string, 0, len(mentionReferences))
	for mentionEvidenceID := range mentionReferences {
		mentionEvidenceIDs = append(mentionEvidenceIDs, mentionEvidenceID)
	}
	responseContract, err := newAISemanticResponseContract(responseSchemaVersion, requestID, spanIDs, technologyKeys, mentionEvidenceIDs)
	if err != nil {
		return AIModelInput{}, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	input.responseContract = responseContract
	input.responseContract.knownMentionReferences = mentionReferences
	return input, nil
}

// BuildAIModelInput reads one remote body and only the complete deterministic
// evidence frozen by the AI configuration's published technology
// configuration. It deliberately has no path to source metadata, COS objects,
// title, job summary, or a complete dictionary.
func (repository *Repository) BuildAIModelInput(ctx context.Context, requestID, aiConfigurationID, bodyContentVersionID string) (AIModelInput, error) {
	if requestID == "" || aiConfigurationID == "" || bodyContentVersionID == "" {
		return AIModelInput{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	var technologyConfigurationID, responseSchemaVersion string
	err := repository.pool.QueryRow(ctx, `
SELECT technology_configuration.id::text, ai_configuration.response_schema_version
FROM ai_analysis_configurations AS ai_configuration
JOIN technology_analysis_configurations AS technology_configuration
  ON technology_configuration.id = ai_configuration.technology_analysis_configuration_id
WHERE ai_configuration.id = $1
	  AND technology_configuration.status = 'published'`, aiConfigurationID).Scan(&technologyConfigurationID, &responseSchemaVersion)
	if err != nil {
		return AIModelInput{}, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	bodyContent, err := repository.loadBodyContentFromPool(ctx, bodyContentVersionID)
	if err != nil {
		return AIModelInput{}, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	body := AIInputBody{
		Addition:             bodyContent.addition,
		AdditionSHA256:       bodyContent.additionSHA,
		BodyContentVersionID: bodyContentVersionID,
		Description:          bodyContent.description,
		DescriptionSHA256:    bodyContent.descriptionSHA,
		JobRequire:           bodyContent.jobRequire,
		JobRequireSHA256:     bodyContent.jobRequireSHA,
		NormalizationVersion: bodyContent.normalizationVersion,
	}
	spanIndex, err := NewAISpanIndex(body)
	if err != nil {
		return AIModelInput{}, err
	}
	mentions, err := repository.loadVerifiedTechnologyMentions(ctx, bodyContentVersionID, technologyConfigurationID)
	if err != nil {
		return AIModelInput{}, NewSafeAIError(AISafeErrorInvalidEvidence, err)
	}
	return buildAIModelInputForResponseSchemaVersion(responseSchemaVersion, requestID, body, spanIndex, mentions)
}

// LocateCandidate finds a candidate exactly once in its approved span and
// returns a program-owned interval. Candidate text remains transient and is
// deliberately absent from the returned evidence and every error message.
func (input AIModelInput) LocateCandidate(candidateText, spanID string) (AISemanticEvidence, error) {
	span, found := input.spanIndex.Lookup(spanID)
	if !found || candidateText == "" {
		return AISemanticEvidence{}, NewSafeAIError(AISafeErrorInvalidEvidence, nil)
	}
	fieldText, found := input.fieldTextBySource[span.SourceField]
	if !found {
		return AISemanticEvidence{}, NewSafeAIError(AISafeErrorInvalidEvidence, nil)
	}
	spanText, err := CodePointSlice(fieldText, span.CodePointRange)
	if err != nil {
		return AISemanticEvidence{}, NewSafeAIError(AISafeErrorInvalidEvidence, err)
	}
	firstByteOffset := strings.Index(spanText, candidateText)
	if firstByteOffset < 0 || strings.Index(spanText[firstByteOffset+len(candidateText):], candidateText) >= 0 {
		return AISemanticEvidence{}, NewSafeAIError(AISafeErrorInvalidEvidence, nil)
	}
	localRange, err := CodePointRangeFromByteOffsets(spanText, firstByteOffset, firstByteOffset+len(candidateText))
	if err != nil {
		return AISemanticEvidence{}, NewSafeAIError(AISafeErrorInvalidEvidence, err)
	}
	return AISemanticEvidence{
		CodePointRange:        CodePointRange{Start: span.CodePointRange.Start + localRange.Start, End: span.CodePointRange.Start + localRange.End},
		NormalizedFieldSHA256: span.NormalizedFieldSHA256,
		NormalizationVersion:  span.NormalizationVersion,
		SourceField:           span.SourceField,
		SpanID:                span.ID,
	}, nil
}

// ResolveSpanIDs converts response references into persistence-ready,
// program-owned field/hash/code-point evidence. It cannot fabricate an
// interval from a model coordinate.
func (input AIModelInput) ResolveSpanIDs(spanIDs []string) ([]AISemanticEvidence, error) {
	evidence := make([]AISemanticEvidence, 0, len(spanIDs))
	for _, spanID := range spanIDs {
		span, found := input.spanIndex.Lookup(spanID)
		if !found {
			return nil, NewSafeAIError(AISafeErrorInvalidEvidence, nil)
		}
		evidence = append(evidence, AISemanticEvidence{
			CodePointRange:        span.CodePointRange,
			NormalizedFieldSHA256: span.NormalizedFieldSHA256,
			NormalizationVersion:  span.NormalizationVersion,
			SourceField:           span.SourceField,
			SpanID:                span.ID,
		})
	}
	return evidence, nil
}

// ValidateResponse validates an in-memory provider response against this exact
// input request. It never stores or includes the raw response in an error.
func (input AIModelInput) ValidateResponse(raw []byte) (AISemanticResponse, error) {
	return input.responseContract.Validate(raw)
}

func aiSourceFieldOrder(sourceField string) int {
	switch sourceField {
	case SourceFieldDescription:
		return 0
	case SourceFieldJobRequire:
		return 1
	case SourceFieldAddition:
		return 2
	default:
		return 3
	}
}

func buildAIModelInputFields(body AIInputBody, spanIndex AISpanIndex) []AIModelInputField {
	fields := make([]AIModelInputField, 0, 3)
	for _, sourceField := range []string{SourceFieldDescription, SourceFieldJobRequire, SourceFieldAddition} {
		text, _, exists := aiInputBodyText(body, sourceField)
		if !exists {
			continue
		}
		modelField := AIModelInputField{SourceField: sourceField}
		for _, span := range spanIndex.spans {
			if span.SourceField != sourceField {
				continue
			}
			spanText, err := CodePointSlice(text, span.CodePointRange)
			if err != nil {
				return nil
			}
			modelField.Spans = append(modelField.Spans, AIModelInputSpan{SpanID: span.ID, Text: spanText})
		}
		if len(modelField.Spans) > 0 {
			fields = append(fields, modelField)
		}
	}
	return fields
}

func validateSpanIndexForBody(body AIInputBody, spanIndex AISpanIndex) error {
	expected, err := NewAISpanIndex(body)
	if err != nil {
		return err
	}
	if expected.bodyContentVersionID != spanIndex.bodyContentVersionID || len(expected.spans) != len(spanIndex.spans) {
		return fmt.Errorf("span index is not bound to the supplied body")
	}
	for position := range expected.spans {
		if expected.spans[position] != spanIndex.spans[position] {
			return fmt.Errorf("span index is not bound to the supplied body")
		}
	}
	return nil
}

func validateVerifiedTechnologyMention(body AIInputBody, spanIndex AISpanIndex, mention VerifiedTechnologyMention) (AISpan, error) {
	if mention.BodyContentVersionID != body.BodyContentVersionID || mention.MentionEvidenceID == "" || mention.TechnologyKey == "" || mention.CanonicalName == "" || !AllowedSourceField(mention.SourceField) {
		return AISpan{}, fmt.Errorf("verified mention identity is invalid")
	}
	fieldText, fieldSHA256, exists := aiInputBodyText(body, mention.SourceField)
	if !exists || mention.NormalizationVersion != body.NormalizationVersion || mention.NormalizedFieldSHA256 != fieldSHA256 {
		return AISpan{}, fmt.Errorf("verified mention provenance is invalid")
	}
	if _, err := CodePointSlice(fieldText, mention.CodePointRange); err != nil {
		return AISpan{}, err
	}
	for _, span := range spanIndex.spans {
		if span.SourceField == mention.SourceField && span.CodePointRange.Start <= mention.CodePointRange.Start && mention.CodePointRange.End <= span.CodePointRange.End {
			return span, nil
		}
	}
	return AISpan{}, fmt.Errorf("verified mention does not belong to one input span")
}

func aiInputBodyText(body AIInputBody, sourceField string) (string, string, bool) {
	switch sourceField {
	case SourceFieldDescription:
		return body.Description, body.DescriptionSHA256, true
	case SourceFieldJobRequire:
		return body.JobRequire, body.JobRequireSHA256, true
	case SourceFieldAddition:
		return body.Addition, body.AdditionSHA256, true
	default:
		return "", "", false
	}
}

func (repository *Repository) loadVerifiedTechnologyMentions(ctx context.Context, bodyContentVersionID, technologyConfigurationID string) ([]VerifiedTechnologyMention, error) {
	rows, err := repository.pool.Query(ctx, `
SELECT evidence.id::text, identity.technology_key, identity.canonical_name,
       evidence.source_field, evidence.normalization_version,
       evidence.normalized_field_sha256, evidence.codepoint_start,
       evidence.codepoint_end
FROM technology_mention_evidence AS evidence
JOIN technology_mentions AS mention ON mention.id = evidence.technology_mention_id
JOIN technology_analysis_results AS result ON result.id = mention.analysis_result_id
JOIN technology_identities AS identity ON identity.id = mention.technology_identity_id
WHERE result.body_content_version_id = $1
  AND result.configuration_id = $2
  AND result.status = 'complete'
ORDER BY CASE evidence.source_field
           WHEN 'description' THEN 1
           WHEN 'jobRequire' THEN 2
           WHEN 'addition' THEN 3
         END,
         evidence.codepoint_start, evidence.codepoint_end, evidence.id`, bodyContentVersionID, technologyConfigurationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	mentions := make([]VerifiedTechnologyMention, 0)
	for rows.Next() {
		mention := VerifiedTechnologyMention{BodyContentVersionID: bodyContentVersionID}
		if err := rows.Scan(
			&mention.MentionEvidenceID, &mention.TechnologyKey, &mention.CanonicalName,
			&mention.SourceField, &mention.NormalizationVersion,
			&mention.NormalizedFieldSHA256, &mention.CodePointRange.Start,
			&mention.CodePointRange.End,
		); err != nil {
			return nil, err
		}
		mentions = append(mentions, mention)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return mentions, nil
}
