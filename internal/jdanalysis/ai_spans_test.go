package jdanalysis

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestAISpanIndexBuildsStableThreeFieldCodePointRanges(t *testing.T) {
	body := syntheticAISpanBody(
		"甲。e\u0301 test，后段\n最后",
		"Go 1.20；C++！",
		"补充？",
	)

	index, err := NewAISpanIndex(body)
	if err != nil {
		t.Fatalf("NewAISpanIndex() error = %v", err)
	}

	want := []AISpan{
		{ID: "description:s01", SourceField: SourceFieldDescription, NormalizationVersion: "jd-body-v1", NormalizedFieldSHA256: syntheticSHA256("甲。e\u0301 test，后段\n最后"), CodePointRange: CodePointRange{Start: 0, End: 1}},
		{ID: "description:s02", SourceField: SourceFieldDescription, NormalizationVersion: "jd-body-v1", NormalizedFieldSHA256: syntheticSHA256("甲。e\u0301 test，后段\n最后"), CodePointRange: CodePointRange{Start: 2, End: 9}},
		{ID: "description:s03", SourceField: SourceFieldDescription, NormalizationVersion: "jd-body-v1", NormalizedFieldSHA256: syntheticSHA256("甲。e\u0301 test，后段\n最后"), CodePointRange: CodePointRange{Start: 10, End: 12}},
		{ID: "description:s04", SourceField: SourceFieldDescription, NormalizationVersion: "jd-body-v1", NormalizedFieldSHA256: syntheticSHA256("甲。e\u0301 test，后段\n最后"), CodePointRange: CodePointRange{Start: 13, End: 15}},
		{ID: "jobRequire:s01", SourceField: SourceFieldJobRequire, NormalizationVersion: "jd-body-v1", NormalizedFieldSHA256: syntheticSHA256("Go 1.20；C++！"), CodePointRange: CodePointRange{Start: 0, End: 7}},
		{ID: "jobRequire:s02", SourceField: SourceFieldJobRequire, NormalizationVersion: "jd-body-v1", NormalizedFieldSHA256: syntheticSHA256("Go 1.20；C++！"), CodePointRange: CodePointRange{Start: 8, End: 11}},
		{ID: "addition:s01", SourceField: SourceFieldAddition, NormalizationVersion: "jd-body-v1", NormalizedFieldSHA256: syntheticSHA256("补充？"), CodePointRange: CodePointRange{Start: 0, End: 2}},
	}
	if got := index.Spans(); !equalAISpans(got, want) {
		t.Fatalf("Spans() = %#v, want %#v", got, want)
	}

	span, found := index.Lookup("jobRequire:s02")
	if !found || span != want[5] {
		t.Fatalf("Lookup(jobRequire:s02) = %#v, %t; want %#v, true", span, found, want[5])
	}
	if _, found := index.Lookup("jobRequire:s99"); found {
		t.Fatal("Lookup() accepted an unknown span ID")
	}
}

func TestAISpanIndexDoesNotExposeTextAndRejectsHashMismatches(t *testing.T) {
	const syntheticFragment = "SYNTHETIC_BODY_FRAGMENT_MUST_NOT_APPEAR"
	body := syntheticAISpanBody(syntheticFragment+"。", "", "")

	index, err := NewAISpanIndex(body)
	if err != nil {
		t.Fatalf("NewAISpanIndex() error = %v", err)
	}
	encoded, err := json.Marshal(index.Spans())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), syntheticFragment) {
		t.Fatalf("span metadata leaked source text: %s", encoded)
	}

	body.DescriptionSHA256 = strings.Repeat("0", 64)
	_, err = NewAISpanIndex(body)
	if err == nil {
		t.Fatal("NewAISpanIndex() accepted a mismatched normalized field hash")
	}
	if !strings.Contains(err.Error(), string(AISafeErrorConfigurationInvalid)) || strings.Contains(err.Error(), syntheticFragment) {
		t.Fatalf("hash mismatch error is not redacted: %q", err)
	}
}

func syntheticAISpanBody(description, jobRequire, addition string) AIInputBody {
	return AIInputBody{
		BodyContentVersionID: "body-version-01",
		NormalizationVersion: "jd-body-v1",
		Description:          description,
		DescriptionSHA256:    syntheticSHA256(description),
		JobRequire:           jobRequire,
		JobRequireSHA256:     syntheticSHA256(jobRequire),
		Addition:             addition,
		AdditionSHA256:       syntheticSHA256(addition),
	}
}

func syntheticSHA256(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func equalAISpans(got, want []AISpan) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range want {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
