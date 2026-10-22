package jdanalysis

import (
	"strings"
	"testing"
)

func TestCodePointRangeFromByteOffsets(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		matched    string
		occurrence int
		want       CodePointRange
	}{
		{name: "ASCII", text: "Use Go", matched: "Go", want: CodePointRange{Start: 4, End: 6}},
		{name: "Chinese punctuation", text: "中文：Go。", matched: "Go", want: CodePointRange{Start: 3, End: 5}},
		{name: "combining character", text: "e\u0301 Go", matched: "Go", want: CodePointRange{Start: 3, End: 5}},
		{name: "adjacent first", text: "GoGo", matched: "Go", occurrence: 0, want: CodePointRange{Start: 0, End: 2}},
		{name: "adjacent second", text: "GoGo", matched: "Go", occurrence: 1, want: CodePointRange{Start: 2, End: 4}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start := nthByteIndex(test.text, test.matched, test.occurrence)
			interval, err := CodePointRangeFromByteOffsets(test.text, start, start+len(test.matched))
			if err != nil {
				t.Fatal(err)
			}
			if interval != test.want {
				t.Fatalf("range = %#v, want %#v", interval, test.want)
			}
			segment, err := CodePointSlice(test.text, interval)
			if err != nil || segment != test.matched {
				t.Fatalf("slice = %q err=%v", segment, err)
			}
		})
	}
}

func TestCodePointConversionRejectsInvalidByteBoundaries(t *testing.T) {
	text := "中文Go"
	for _, interval := range [][2]int{{-1, 1}, {0, 0}, {1, 3}, {0, len(text) + 1}} {
		if _, err := CodePointRangeFromByteOffsets(text, interval[0], interval[1]); err == nil {
			t.Fatalf("invalid byte interval %v was accepted", interval)
		}
	}
}

func TestValidateEvidenceIntervalReplaysApprovedAlias(t *testing.T) {
	rule := approvedRule("go", "Go", "Go", CaseSensitive, BoundaryASCIIIdentifier)
	text := "中文Go，Go"
	firstStart := strings.Index(text, "Go")
	first, err := CodePointRangeFromByteOffsets(text, firstStart, firstStart+len("Go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateEvidenceInterval(SourceFieldDescription, text, first, rule); err != nil {
		t.Fatal(err)
	}
	wrong := CodePointRange{Start: first.Start - 1, End: first.End}
	if err := ValidateEvidenceInterval(SourceFieldDescription, text, wrong, rule); err == nil {
		t.Fatal("non-alias interval was accepted")
	}
	if err := ValidateEvidenceInterval("title", text, first, rule); err == nil {
		t.Fatal("title interval was accepted")
	}
}

func TestMatcherCoordinatesRemainFieldIndependent(t *testing.T) {
	matcher := mustMatcher(t, approvedRule("go", "Go", "Go", CaseSensitive, BoundaryASCIIIdentifier))
	fields := map[string]string{
		SourceFieldDescription: "中文Go",
		SourceFieldJobRequire:  "Go",
	}
	matches, err := matcher.MatchBodyFields(fields)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("matches = %#v", matches)
	}
	first, err := matches[0].CodePointRange(fields[matches[0].SourceField])
	if err != nil || first != (CodePointRange{Start: 2, End: 4}) {
		t.Fatalf("description coordinate = %#v err=%v", first, err)
	}
	second, err := matches[1].CodePointRange(fields[matches[1].SourceField])
	if err != nil || second != (CodePointRange{Start: 0, End: 2}) {
		t.Fatalf("jobRequire coordinate = %#v err=%v", second, err)
	}
}

func nthByteIndex(text, needle string, occurrence int) int {
	start := 0
	for index := 0; index <= occurrence; index++ {
		found := strings.Index(text[start:], needle)
		if found < 0 {
			return -1
		}
		start += found
		if index == occurrence {
			return start
		}
		start += len(needle)
	}
	return -1
}
