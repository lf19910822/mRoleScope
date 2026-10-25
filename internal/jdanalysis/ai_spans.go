package jdanalysis

import (
	"fmt"
	"unicode"
)

// AIInputBody is the remote-only normalized body supplied to the span builder.
// Its text is deliberately not retained by AISpanIndex or returned by its
// public interface.
type AIInputBody struct {
	Addition             string
	AdditionSHA256       string
	BodyContentVersionID string
	Description          string
	DescriptionSHA256    string
	JobRequire           string
	JobRequireSHA256     string
	NormalizationVersion string
}

// AISpan is the program-owned location of one deterministic sentence or
// clause. It contains no source fragment, so it is safe to persist or report
// once the surrounding result has passed validation.
type AISpan struct {
	CodePointRange        CodePointRange
	ID                    string
	NormalizationVersion  string
	NormalizedFieldSHA256 string
	SourceField           string
}

// AISpanIndex is an immutable, text-free reverse map for the spans generated
// for one body version. A caller can only obtain coordinates and provenance.
type AISpanIndex struct {
	bodyContentVersionID string
	spans                []AISpan
	spansByID            map[string]AISpan
}

// NewAISpanIndex deterministically segments exactly the three approved
// normalized fields. It verifies each supplied field hash before retaining any
// coordinate metadata, preventing a span from being bound to the wrong text.
func NewAISpanIndex(body AIInputBody) (AISpanIndex, error) {
	if body.BodyContentVersionID == "" || body.NormalizationVersion == "" {
		return AISpanIndex{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}

	fields := []aiSpanField{
		{sourceField: SourceFieldDescription, text: body.Description, sha256: body.DescriptionSHA256},
		{sourceField: SourceFieldJobRequire, text: body.JobRequire, sha256: body.JobRequireSHA256},
		{sourceField: SourceFieldAddition, text: body.Addition, sha256: body.AdditionSHA256},
	}
	index := AISpanIndex{
		bodyContentVersionID: body.BodyContentVersionID,
		spansByID:            make(map[string]AISpan),
	}
	for _, field := range fields {
		if field.sha256 != sha256Hex(field.text) {
			return AISpanIndex{}, NewSafeAIError(AISafeErrorConfigurationInvalid, fmt.Errorf("normalized field hash mismatch"))
		}
		for position, codePointRange := range aiSpanRanges(field.text) {
			span := AISpan{
				CodePointRange:        codePointRange,
				ID:                    fmt.Sprintf("%s:s%02d", field.sourceField, position+1),
				NormalizationVersion:  body.NormalizationVersion,
				NormalizedFieldSHA256: field.sha256,
				SourceField:           field.sourceField,
			}
			index.spans = append(index.spans, span)
			index.spansByID[span.ID] = span
		}
	}
	return index, nil
}

// Lookup resolves an opaque span identifier only within this index's body
// version. It returns no text and cannot resolve cross-body identifiers.
func (index AISpanIndex) Lookup(spanID string) (AISpan, bool) {
	span, exists := index.spansByID[spanID]
	return span, exists
}

// Spans returns the deterministic field/order sequence without source text.
func (index AISpanIndex) Spans() []AISpan {
	return append([]AISpan(nil), index.spans...)
}

type aiSpanField struct {
	sha256      string
	sourceField string
	text        string
}

func aiSpanRanges(text string) []CodePointRange {
	runes := []rune(text)
	ranges := make([]CodePointRange, 0)
	start := -1
	for position, value := range runes {
		if aiSpanSeparator(runes, position) {
			appendAISpanRange(&ranges, runes, start, position)
			start = -1
			continue
		}
		if start < 0 && !unicode.IsSpace(value) {
			start = position
		}
	}
	appendAISpanRange(&ranges, runes, start, len(runes))
	return ranges
}

func aiSpanSeparator(runes []rune, position int) bool {
	switch runes[position] {
	case '\n', '\r', '。', '！', '？', '；', ';', '，', ',':
		return true
	case '.':
		return !isDecimalPoint(runes, position)
	case '!', '?':
		return true
	default:
		return false
	}
}

func appendAISpanRange(ranges *[]CodePointRange, runes []rune, start, end int) {
	if start < 0 {
		return
	}
	for end > start && unicode.IsSpace(runes[end-1]) {
		end--
	}
	if end > start {
		*ranges = append(*ranges, CodePointRange{Start: start, End: end})
	}
}

func isDecimalPoint(runes []rune, position int) bool {
	return position > 0 && position+1 < len(runes) && unicode.IsDigit(runes[position-1]) && unicode.IsDigit(runes[position+1])
}
