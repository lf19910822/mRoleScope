package jdanalysis

import (
	"fmt"
	"unicode/utf8"
)

// CodePointRange uses a zero-based, half-open Unicode code-point interval.
// It deliberately is not a UTF-8 byte offset and not a user-visible grapheme
// count: a combining character can occupy its own code point.
type CodePointRange struct {
	Start int
	End   int
}

func (interval CodePointRange) ValidFor(normalizedText string) bool {
	if !utf8.ValidString(normalizedText) || interval.Start < 0 || interval.End <= interval.Start {
		return false
	}
	return interval.End <= utf8.RuneCountInString(normalizedText)
}

// CodePointRangeFromByteOffsets is the only permitted conversion from Go's
// regexp/string byte positions to stored evidence coordinates.
func CodePointRangeFromByteOffsets(normalizedText string, byteStart, byteEnd int) (CodePointRange, error) {
	if !utf8.ValidString(normalizedText) {
		return CodePointRange{}, fmt.Errorf("normalized text is not valid UTF-8")
	}
	if byteStart < 0 || byteEnd <= byteStart || byteEnd > len(normalizedText) {
		return CodePointRange{}, fmt.Errorf("byte interval is out of range")
	}
	start, startBoundary := codePointOffset(normalizedText, byteStart)
	end, endBoundary := codePointOffset(normalizedText, byteEnd)
	if !startBoundary || !endBoundary {
		return CodePointRange{}, fmt.Errorf("byte interval does not align with Unicode code-point boundaries")
	}
	interval := CodePointRange{Start: start, End: end}
	if !interval.ValidFor(normalizedText) {
		return CodePointRange{}, fmt.Errorf("code-point interval is invalid")
	}
	return interval, nil
}

func codePointOffset(text string, byteOffset int) (int, bool) {
	if byteOffset < 0 || byteOffset > len(text) {
		return 0, false
	}
	codePoints := 0
	for byteIndex := 0; byteIndex < len(text); {
		if byteIndex == byteOffset {
			return codePoints, true
		}
		_, size := utf8.DecodeRuneInString(text[byteIndex:])
		byteIndex += size
		codePoints++
	}
	return codePoints, byteOffset == len(text)
}

func CodePointSlice(normalizedText string, interval CodePointRange) (string, error) {
	if !interval.ValidFor(normalizedText) {
		return "", fmt.Errorf("code-point interval is invalid")
	}
	startByte, ok := byteOffsetAtCodePoint(normalizedText, interval.Start)
	if !ok {
		return "", fmt.Errorf("code-point interval start is invalid")
	}
	endByte, ok := byteOffsetAtCodePoint(normalizedText, interval.End)
	if !ok {
		return "", fmt.Errorf("code-point interval end is invalid")
	}
	return normalizedText[startByte:endByte], nil
}

func byteOffsetAtCodePoint(text string, codePointOffset int) (int, bool) {
	if codePointOffset < 0 || !utf8.ValidString(text) {
		return 0, false
	}
	seen := 0
	for byteIndex := 0; byteIndex < len(text); {
		if seen == codePointOffset {
			return byteIndex, true
		}
		_, size := utf8.DecodeRuneInString(text[byteIndex:])
		byteIndex += size
		seen++
	}
	return len(text), seen == codePointOffset
}

// ValidateEvidenceInterval is used before remote persistence and publication.
// It replays the stored range against the exact normalized field text and the
// approved alias rule, without storing or logging the resulting text snippet.
func ValidateEvidenceInterval(sourceField, normalizedText string, interval CodePointRange, rule DictionaryRule) error {
	if !AllowedSourceField(sourceField) {
		return fmt.Errorf("evidence source field is outside the approved JD body scope")
	}
	if !rule.Executable() {
		return fmt.Errorf("evidence rule is not an approved executable rule")
	}
	segment, err := CodePointSlice(normalizedText, interval)
	if err != nil {
		return err
	}
	if !ruleMatchesAt(rule, segment, 0, len(segment)) {
		return fmt.Errorf("evidence interval does not match its approved alias")
	}
	return nil
}

func (match ByteMatch) CodePointRange(normalizedText string) (CodePointRange, error) {
	if !AllowedSourceField(match.SourceField) {
		return CodePointRange{}, fmt.Errorf("match source field is outside the approved JD body scope")
	}
	return CodePointRangeFromByteOffsets(normalizedText, match.ByteStart, match.ByteEnd)
}
