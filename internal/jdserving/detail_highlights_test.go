package jdserving

import (
	"reflect"
	"testing"
)

func TestSanitizeTechnologyHighlightsMapsCurrentBodyEvidenceWithoutLeakingIt(t *testing.T) {
	sections := BodySections{
		Responsibilities: "负责 Go 服务",
		Requirements:     "熟悉 Python",
		Supplemental:     pointerTo("加分项 Docker"),
	}
	candidates := []technologyHighlightCandidate{
		{end: 5, sourceField: "description", start: 3},
		{end: 9, sourceField: "jobRequire", start: 3},
		{end: 10, sourceField: "addition", start: 4},
	}

	got := sanitizeTechnologyHighlights(candidates, sections)
	want := []TechnologyHighlight{
		{Section: BodySectionResponsibilities, Start: 3, End: 5},
		{Section: BodySectionRequirements, Start: 3, End: 9},
		{Section: BodySectionSupplemental, Start: 4, End: 10},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("safe technology highlights = %#v, want %#v", got, want)
	}
}

func TestSanitizeTechnologyHighlightsDropsUnsafeCoordinatesDuplicatesAndOverlaps(t *testing.T) {
	sections := BodySections{
		Responsibilities: "Go PostgreSQL",
		Requirements:     "Python",
	}
	candidates := []technologyHighlightCandidate{
		{end: 2, sourceField: "description", start: 0},
		{end: 2, sourceField: "description", start: 0},
		{end: 5, sourceField: "description", start: 1},
		{end: 12, sourceField: "description", start: 3},
		{end: 99, sourceField: "description", start: 3},
		{end: 2, sourceField: "addition", start: 0},
		{end: 0, sourceField: "jobRequire", start: 0},
	}

	got := sanitizeTechnologyHighlights(candidates, sections)
	want := []TechnologyHighlight{
		{Section: BodySectionResponsibilities, Start: 0, End: 2},
		{Section: BodySectionResponsibilities, Start: 3, End: 12},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("safe technology highlights = %#v, want %#v", got, want)
	}
}

func pointerTo(value string) *string { return &value }
