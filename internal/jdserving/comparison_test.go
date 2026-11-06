package jdserving

import "testing"

func TestCompareJobDetailsIgnoresCollectionOrderAndFormatsChangedDimensions(t *testing.T) {
	old := comparisonFixture()
	latest := old
	latest.Locations = []string{"深圳", "上海"}
	latest.TechnologyKeywords = []FilterOption{{Label: "Rust", Value: "rust"}, {Label: "Go", Value: "go"}}
	latest.Education = &EducationRequirement{ExplicitRequirement: true, LowerBound: EducationLevelMaster}
	latest.Experience = &ExperienceRequirement{ExplicitRequirement: true, MinimumYears: 5}

	comparison := compareJobDetails(old, latest)
	if comparison.Status != ComparisonStatusAvailable {
		t.Fatalf("status = %q", comparison.Status)
	}
	if len(comparison.DimensionChanges) != 3 {
		t.Fatalf("dimension changes = %#v", comparison.DimensionChanges)
	}
	assertDimensionChange(t, comparison.DimensionChanges, "学历要求", "本科及以上", "硕士及以上")
	assertDimensionChange(t, comparison.DimensionChanges, "最低工作年限", "最低 3 年", "最低 5 年")
	assertDimensionChange(t, comparison.DimensionChanges, "技术关键词", "Go", "Go、Rust")
}

func TestCompareJobDetailsReturnsEmptyComparisonWhenVisibleFactsMatch(t *testing.T) {
	comparison := compareJobDetails(comparisonFixture(), comparisonFixture())
	if comparison.Status != ComparisonStatusAvailable || len(comparison.DimensionChanges) != 0 || len(comparison.BodySections) != 0 {
		t.Fatalf("empty comparison = %#v", comparison)
	}
}

func TestCompareJobDetailsIncludesScalarTitleAndUrgentChanges(t *testing.T) {
	old := comparisonFixture()
	latest := old
	latest.Title = "高级平台工程师"
	latest.IsUrgent = true
	comparison := compareJobDetails(old, latest)
	assertDimensionChange(t, comparison.DimensionChanges, "岗位名称", "平台工程师", "高级平台工程师")
	assertDimensionChange(t, comparison.DimensionChanges, "紧急招聘", "否", "是")
}

func TestCompareJobDetailsUsesDashForMissingRequirements(t *testing.T) {
	old := comparisonFixture()
	old.Education = nil
	old.Experience = nil
	latest := old
	latest.Education = &EducationRequirement{ExplicitRequirement: true, LowerBound: EducationLevelBachelor}
	latest.Experience = &ExperienceRequirement{ExplicitRequirement: true, MinimumYears: 1}
	comparison := compareJobDetails(old, latest)
	assertDimensionChange(t, comparison.DimensionChanges, "学历要求", "-", "本科及以上")
	assertDimensionChange(t, comparison.DimensionChanges, "最低工作年限", "-", "最低 1 年")
}

func TestCompareJobDetailsProducesOrderedBodyFragments(t *testing.T) {
	old := comparisonFixture()
	latest := old
	latest.Body.Responsibilities = "负责 Go 服务"
	old.Body.Responsibilities = "负责旧服务"
	latest.Body.Requirements = "熟悉 Go 与 Rust"
	old.Body.Requirements = "熟悉 Go"

	comparison := compareJobDetails(old, latest)
	section := bodyComparison(t, comparison, BodySectionRequirements)
	assertFragments(t, section.Fragments,
		ComparisonFragment{Kind: ComparisonFragmentUnchanged, Text: "熟悉 Go"},
		ComparisonFragment{Kind: ComparisonFragmentInserted, Text: " 与 Rust"},
	)
	section = bodyComparison(t, comparison, BodySectionResponsibilities)
	assertFragments(t, section.Fragments,
		ComparisonFragment{Kind: ComparisonFragmentUnchanged, Text: "负责"},
		ComparisonFragment{Kind: ComparisonFragmentDeleted, Text: "旧"},
		ComparisonFragment{Kind: ComparisonFragmentInserted, Text: " Go "},
		ComparisonFragment{Kind: ComparisonFragmentUnchanged, Text: "服务"},
	)
}

func TestCompareJobDetailsRepresentsSectionAdditionRemovalAndReorder(t *testing.T) {
	old := comparisonFixture()
	latest := old
	old.Body.Supplemental = stringPtr("旧补充")
	latest.Body.Supplemental = nil
	old.Body.Responsibilities = "1. 负责平台\n2. 维护稳定"
	latest.Body.Responsibilities = "1. 维护稳定\n2. 负责平台"

	comparison := compareJobDetails(old, latest)
	supplemental := bodyComparison(t, comparison, BodySectionSupplemental)
	assertFragments(t, supplemental.Fragments, ComparisonFragment{Kind: ComparisonFragmentDeleted, Text: "旧补充"})
	responsibilities := bodyComparison(t, comparison, BodySectionResponsibilities)
	assertFragments(t, responsibilities.Fragments,
		ComparisonFragment{Kind: ComparisonFragmentDeleted, Text: "1. 负责平台\n2. 维护稳定"},
		ComparisonFragment{Kind: ComparisonFragmentInserted, Text: "1. 维护稳定\n2. 负责平台"},
	)
}

func TestCompareJobDetailsNormalizesWhitespaceButPreservesUnicodeText(t *testing.T) {
	old := comparisonFixture()
	latest := old
	old.Body.Requirements = "熟悉 Go\r\n并维护服务"
	latest.Body.Requirements = "熟悉  Go\n并维护服务"
	if comparison := compareJobDetails(old, latest); len(comparison.BodySections) != 0 {
		t.Fatalf("whitespace-only comparison = %#v", comparison)
	}

	latest.Body.Requirements = "熟悉 Go\n并维护🚀服务"
	comparison := compareJobDetails(old, latest)
	section := bodyComparison(t, comparison, BodySectionRequirements)
	if len(section.Fragments) == 0 || !containsFragment(section.Fragments, ComparisonFragmentInserted, "🚀") {
		t.Fatalf("unicode fragments = %#v", section.Fragments)
	}
}

func comparisonFixture() JobDetailResponse {
	return JobDetailResponse{
		Title: "平台工程师", Locations: []string{"上海", "深圳"},
		MainJobFamily:      &FilterOption{Label: "后端", Value: "backend"},
		OtherJobFamilies:   []FilterOption{{Label: "平台", Value: "platform"}},
		TechnologyKeywords: []FilterOption{{Label: "Go", Value: "go"}},
		Projects:           []FilterOption{{Label: "项目A", Value: "a"}},
		Education:          &EducationRequirement{ExplicitRequirement: true, LowerBound: EducationLevelBachelor},
		Experience:         &ExperienceRequirement{ExplicitRequirement: true, MinimumYears: 3},
		Body:               BodySections{Responsibilities: "建设平台", Requirements: "熟悉 Go"},
	}
}

func assertDimensionChange(t *testing.T, changes []ComparisonDimensionChange, field, historical, latest string) {
	t.Helper()
	for _, change := range changes {
		if change.Field == field {
			if change.Historical != historical || change.Latest != latest {
				t.Fatalf("%s = %#v", field, change)
			}
			return
		}
	}
	t.Fatalf("missing dimension change %q: %#v", field, changes)
}

func bodyComparison(t *testing.T, comparison JobComparison, section BodySection) ComparisonBodySection {
	t.Helper()
	for _, candidate := range comparison.BodySections {
		if candidate.Section == section {
			return candidate
		}
	}
	t.Fatalf("missing body comparison %q: %#v", section, comparison.BodySections)
	return ComparisonBodySection{}
}

func assertFragments(t *testing.T, got []ComparisonFragment, want ...ComparisonFragment) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("fragments = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("fragment %d = %#v, want %#v", index, got[index], want[index])
		}
	}
}

func containsFragment(fragments []ComparisonFragment, kind ComparisonFragmentKind, text string) bool {
	for _, fragment := range fragments {
		if fragment.Kind == kind && fragment.Text == text {
			return true
		}
	}
	return false
}

func stringPtr(value string) *string { return &value }
