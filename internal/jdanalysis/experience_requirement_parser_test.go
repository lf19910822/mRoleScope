package jdanalysis

import "testing"

func TestParseExperienceRequirementRecognizesReviewedRequiredYearForms(t *testing.T) {
	tests := []struct {
		name       string
		jobRequire string
		wantYears  int
		wantRule   string
		wantRange  CodePointRange
	}{
		{
			name:       "arabic years or more",
			jobRequire: "3年以上相关工作经验",
			wantYears:  3,
			wantRule:   "experience.arabic.years_or_more",
			wantRange:  CodePointRange{Start: 0, End: 10},
		},
		{
			name:       "chinese numeral years or more",
			jobRequire: "三年以上相关工作经验",
			wantYears:  3,
			wantRule:   "experience.chinese.years_or_more",
			wantRange:  CodePointRange{Start: 0, End: 10},
		},
		{
			name:       "range uses lower endpoint",
			jobRequire: "3-5年相关工作经验",
			wantYears:  3,
			wantRule:   "experience.arabic.range",
			wantRange:  CodePointRange{Start: 0, End: 10},
		},
		{
			name:       "explicit minimum marker",
			jobRequire: "至少2年工作经验",
			wantYears:  2,
			wantRule:   "experience.arabic.explicit_minimum",
			wantRange:  CodePointRange{Start: 0, End: 8},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseExperienceRequirement(ExperienceParserInput{JobRequire: test.jobRequire})
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != ExperienceRequirementConfigured || got.BaseYear != test.wantYears {
				t.Fatalf("result = %#v, want configured base year %d", got, test.wantYears)
			}
			if len(got.Evidence) != 1 {
				t.Fatalf("evidence = %#v, want one item", got.Evidence)
			}
			if got.Evidence[0] != (ExperienceRequirementEvidence{
				SourceField:    SourceFieldJobRequire,
				RuleKey:        test.wantRule,
				MatchedText:    test.jobRequire,
				MinimumYears:   test.wantYears,
				CodePointRange: test.wantRange,
				Modality:       ExperienceRequirementModalityRequired,
			}) {
				t.Fatalf("evidence = %#v", got.Evidence[0])
			}
		})
	}
}

func TestParseExperienceRequirementUsesMaximumOfIndependentRequiredBounds(t *testing.T) {
	got, err := ParseExperienceRequirement(ExperienceParserInput{
		JobRequire: "3年以上工作经验；5年以上Go开发经验",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != ExperienceRequirementConfigured || got.BaseYear != 5 {
		t.Fatalf("result = %#v, want configured base year 5", got)
	}
	if len(got.Evidence) != 2 || got.Evidence[0].MinimumYears != 3 || got.Evidence[1].MinimumYears != 5 {
		t.Fatalf("evidence = %#v, want required three- and five-year items", got.Evidence)
	}
}

func TestParseExperienceRequirementScopesPriorityAndExclusionsLocally(t *testing.T) {
	tests := []struct {
		name           string
		jobRequire     string
		wantStatus     ExperienceRequirementStatus
		wantBaseYear   int
		wantModalities []ExperienceRequirementModality
	}{
		{
			name:           "trailing priority marker excludes its own experience",
			jobRequire:     "有A领域3年经验者优先",
			wantStatus:     ExperienceRequirementNotConfigured,
			wantModalities: []ExperienceRequirementModality{ExperienceRequirementModalityPreferred},
		},
		{
			name:           "leading priority marker excludes its own experience",
			jobRequire:     "优先考虑有3年Go开发经验",
			wantStatus:     ExperienceRequirementNotConfigured,
			wantModalities: []ExperienceRequirementModality{ExperienceRequirementModalityPreferred},
		},
		{
			name:           "priority cannot cross strong clause boundary",
			jobRequire:     "3年以上工作经验；有A领域5年经验者优先",
			wantStatus:     ExperienceRequirementConfigured,
			wantBaseYear:   3,
			wantModalities: []ExperienceRequirementModality{ExperienceRequirementModalityRequired, ExperienceRequirementModalityPreferred},
		},
		{
			name:           "explicitly unrestricted experience does not promote preferred evidence",
			jobRequire:     "经验不限；3年以上Go开发经验优先",
			wantStatus:     ExperienceRequirementNotConfigured,
			wantModalities: []ExperienceRequirementModality{ExperienceRequirementModalityPreferred},
		},
		{
			name:       "project operation tenure is not candidate experience",
			jobRequire: "项目已运营5年，候选人年龄30岁",
			wantStatus: ExperienceRequirementNotConfigured,
		},
		{
			name:       "calendar year is not candidate experience",
			jobRequire: "负责2024年版本发布",
			wantStatus: ExperienceRequirementNotConfigured,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseExperienceRequirement(ExperienceParserInput{JobRequire: test.jobRequire})
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != test.wantStatus || got.BaseYear != test.wantBaseYear {
				t.Fatalf("result = %#v, want status:%q base:%d", got, test.wantStatus, test.wantBaseYear)
			}
			if len(got.Evidence) != len(test.wantModalities) {
				t.Fatalf("evidence = %#v, want %d items", got.Evidence, len(test.wantModalities))
			}
			for index, wantModality := range test.wantModalities {
				if got.Evidence[index].Modality != wantModality {
					t.Fatalf("evidence[%d] modality = %q, want %q", index, got.Evidence[index].Modality, wantModality)
				}
			}
		})
	}
}

func TestParseExperienceRequirementHandlesConditionalAmbiguousAndFieldScopedEvidence(t *testing.T) {
	tests := []struct {
		name           string
		input          ExperienceParserInput
		wantStatus     ExperienceRequirementStatus
		wantBaseYear   int
		wantModalities []ExperienceRequirementModality
		wantSources    []string
	}{
		{
			name:           "degree conditioned branches are not a universal baseline",
			input:          ExperienceParserInput{JobRequire: "本科3年相关工作经验、硕士2年相关工作经验"},
			wantStatus:     ExperienceRequirementNotConfigured,
			wantSources:    []string{SourceFieldJobRequire, SourceFieldJobRequire},
			wantModalities: []ExperienceRequirementModality{ExperienceRequirementModalityConditional, ExperienceRequirementModalityConditional},
		},
		{
			name:           "degree in a prior strong clause does not condition required experience",
			input:          ExperienceParserInput{JobRequire: "本科及以上；3年以上工作经验"},
			wantStatus:     ExperienceRequirementConfigured,
			wantBaseYear:   3,
			wantSources:    []string{SourceFieldJobRequire},
			wantModalities: []ExperienceRequirementModality{ExperienceRequirementModalityRequired},
		},
		{
			name:           "one priority marker cannot choose between two same atom year expressions",
			input:          ExperienceParserInput{JobRequire: "3年相关工作经验、5年相关工作经验优先"},
			wantStatus:     ExperienceRequirementUncertain,
			wantSources:    []string{SourceFieldJobRequire, SourceFieldJobRequire},
			wantModalities: []ExperienceRequirementModality{ExperienceRequirementModalityUncertain, ExperienceRequirementModalityUncertain},
		},
		{
			name:           "description needs an explicit qualification heading",
			input:          ExperienceParserInput{Description: "岗位职责：3年以上工作经验"},
			wantStatus:     ExperienceRequirementNotConfigured,
			wantModalities: []ExperienceRequirementModality{},
		},
		{
			name:           "qualification heading in description is eligible",
			input:          ExperienceParserInput{Description: "任职资格：3年以上工作经验"},
			wantStatus:     ExperienceRequirementConfigured,
			wantBaseYear:   3,
			wantSources:    []string{SourceFieldDescription},
			wantModalities: []ExperienceRequirementModality{ExperienceRequirementModalityRequired},
		},
		{
			name:           "qualification heading scopes following description lines",
			input:          ExperienceParserInput{Description: "岗位职责：负责交付\n任职要求：\n1、3年以上工作经验"},
			wantStatus:     ExperienceRequirementConfigured,
			wantBaseYear:   3,
			wantSources:    []string{SourceFieldDescription},
			wantModalities: []ExperienceRequirementModality{ExperienceRequirementModalityRequired},
		},
		{
			name:           "description candidate after a later responsibility heading is excluded",
			input:          ExperienceParserInput{Description: "任职要求：\n3年以上工作经验\n岗位职责：\n5年以上项目运营经验"},
			wantStatus:     ExperienceRequirementConfigured,
			wantBaseYear:   3,
			wantSources:    []string{SourceFieldDescription},
			wantModalities: []ExperienceRequirementModality{ExperienceRequirementModalityRequired},
		},
		{
			name:           "addition retains preferred evidence only",
			input:          ExperienceParserInput{Addition: "3年以上工作经验"},
			wantStatus:     ExperienceRequirementNotConfigured,
			wantSources:    []string{SourceFieldAddition},
			wantModalities: []ExperienceRequirementModality{ExperienceRequirementModalityPreferred},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseExperienceRequirement(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != test.wantStatus || got.BaseYear != test.wantBaseYear {
				t.Fatalf("result = %#v, want status:%q base:%d", got, test.wantStatus, test.wantBaseYear)
			}
			if len(got.Evidence) != len(test.wantModalities) || len(got.Evidence) != len(test.wantSources) {
				t.Fatalf("evidence = %#v", got.Evidence)
			}
			for index := range got.Evidence {
				if got.Evidence[index].Modality != test.wantModalities[index] || got.Evidence[index].SourceField != test.wantSources[index] {
					t.Fatalf("evidence[%d] = %#v, want source:%q modality:%q", index, got.Evidence[index], test.wantSources[index], test.wantModalities[index])
				}
			}
		})
	}
}
