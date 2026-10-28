package jdanalysis

import "testing"

func TestParseEducationRequirementNormalizesReviewedDegreeAliases(t *testing.T) {
	tests := []struct {
		name      string
		input     EducationParserInput
		wantLevel EducationLevel
		wantRule  string
		wantAlias string
		wantRange CodePointRange
		wantMode  EducationRequirementModality
	}{
		{
			name:      "associate degree alias",
			input:     EducationParserInput{JobRequire: "学历要求：大专及以上"},
			wantLevel: EducationLevelAssociateOrLower,
			wantRule:  "education.associate_or_lower.dazhuan",
			wantAlias: "大专",
			wantRange: CodePointRange{Start: 5, End: 7},
			wantMode:  EducationRequirementModalityRequired,
		},
		{
			name:      "bachelor degree alias",
			input:     EducationParserInput{JobRequire: "本科及以上"},
			wantLevel: EducationLevelBachelor,
			wantRule:  "education.bachelor.benke",
			wantAlias: "本科",
			wantRange: CodePointRange{Start: 0, End: 2},
			wantMode:  EducationRequirementModalityRequired,
		},
		{
			name:      "product maps postgraduate to master",
			input:     EducationParserInput{JobRequire: "学历：研究生及以上"},
			wantLevel: EducationLevelMaster,
			wantRule:  "education.master.postgraduate",
			wantAlias: "研究生",
			wantRange: CodePointRange{Start: 3, End: 6},
			wantMode:  EducationRequirementModalityRequired,
		},
		{
			name:      "longest doctorate alias takes precedence over postgraduate",
			input:     EducationParserInput{JobRequire: "学历：博士研究生及以上"},
			wantLevel: EducationLevelDoctor,
			wantRule:  "education.doctor.doctoral_student",
			wantAlias: "博士研究生",
			wantRange: CodePointRange{Start: 3, End: 8},
			wantMode:  EducationRequirementModalityRequired,
		},
		{
			name:      "english doctorate alias",
			input:     EducationParserInput{JobRequire: "学历：Ph.D.及以上"},
			wantLevel: EducationLevelDoctor,
			wantRule:  "education.doctor.phd_dotted",
			wantAlias: "Ph.D.",
			wantRange: CodePointRange{Start: 3, End: 8},
			wantMode:  EducationRequirementModalityRequired,
		},
		{
			name:      "english masters alias",
			input:     EducationParserInput{JobRequire: "学历：MBA及以上"},
			wantLevel: EducationLevelMaster,
			wantRule:  "education.master.mba",
			wantAlias: "MBA",
			wantRange: CodePointRange{Start: 3, End: 6},
			wantMode:  EducationRequirementModalityRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseEducationRequirement(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != EducationRequirementConfigured {
				t.Fatalf("status = %q, want %q", got.Status, EducationRequirementConfigured)
			}
			if got.LowerBoundLevel != test.wantLevel || got.UpperBoundLevel != "" {
				t.Fatalf("bounds = lower:%q upper:%q, want lower:%q upper empty", got.LowerBoundLevel, got.UpperBoundLevel, test.wantLevel)
			}
			if len(got.Evidence) != 1 {
				t.Fatalf("evidence = %#v, want one selected match", got.Evidence)
			}
			wantEvidence := EducationRequirementEvidence{
				SourceField:    SourceFieldJobRequire,
				RuleKey:        test.wantRule,
				MatchedAlias:   test.wantAlias,
				CodePointRange: test.wantRange,
				Modality:       test.wantMode,
			}
			if got.Evidence[0] != wantEvidence {
				t.Fatalf("evidence = %#v, want %#v", got.Evidence[0], wantEvidence)
			}
		})
	}
}

func TestParseEducationRequirementResolvesQualificationModalityAndBounds(t *testing.T) {
	tests := []struct {
		name         string
		input        EducationParserInput
		wantStatus   EducationRequirementStatus
		wantLower    EducationLevel
		wantUpper    EducationLevel
		wantEvidence []EducationRequirementEvidence
	}{
		{
			name:       "required lower bound does not promote preferred degree",
			input:      EducationParserInput{JobRequire: "本科及以上，硕士优先"},
			wantStatus: EducationRequirementConfigured,
			wantLower:  EducationLevelBachelor,
			wantEvidence: []EducationRequirementEvidence{
				{SourceField: SourceFieldJobRequire, RuleKey: "education.bachelor.benke", MatchedAlias: "本科", CodePointRange: CodePointRange{Start: 0, End: 2}, Modality: EducationRequirementModalityRequired},
				{SourceField: SourceFieldJobRequire, RuleKey: "education.master.shuoshi", MatchedAlias: "硕士", CodePointRange: CodePointRange{Start: 6, End: 8}, Modality: EducationRequirementModalityPreferred},
			},
		},
		{
			name:       "upper bound is not an invented minimum",
			input:      EducationParserInput{JobRequire: "本科及以下"},
			wantStatus: EducationRequirementConfigured,
			wantUpper:  EducationLevelBachelor,
			wantEvidence: []EducationRequirementEvidence{
				{SourceField: SourceFieldJobRequire, RuleKey: "education.bachelor.benke", MatchedAlias: "本科", CodePointRange: CodePointRange{Start: 0, End: 2}, Modality: EducationRequirementModalityRequired},
			},
		},
		{
			name:       "unrestricted education does not promote preferred doctorate",
			input:      EducationParserInput{JobRequire: "学历不限，博士优先"},
			wantStatus: EducationRequirementNotConfigured,
			wantEvidence: []EducationRequirementEvidence{
				{SourceField: SourceFieldJobRequire, RuleKey: "education.doctor.boshi", MatchedAlias: "博士", CodePointRange: CodePointRange{Start: 5, End: 7}, Modality: EducationRequirementModalityPreferred},
			},
		},
		{
			name:       "addition retains preferred evidence but cannot establish a hard requirement",
			input:      EducationParserInput{Addition: "硕士优先"},
			wantStatus: EducationRequirementNotConfigured,
			wantEvidence: []EducationRequirementEvidence{
				{SourceField: SourceFieldAddition, RuleKey: "education.master.shuoshi", MatchedAlias: "硕士", CodePointRange: CodePointRange{Start: 0, End: 2}, Modality: EducationRequirementModalityPreferred},
			},
		},
		{
			name:       "explicit alternative selects its lowest permitted level",
			input:      EducationParserInput{JobRequire: "本科或硕士均可"},
			wantStatus: EducationRequirementConfigured,
			wantLower:  EducationLevelBachelor,
			wantEvidence: []EducationRequirementEvidence{
				{SourceField: SourceFieldJobRequire, RuleKey: "education.bachelor.benke", MatchedAlias: "本科", CodePointRange: CodePointRange{Start: 0, End: 2}, Modality: EducationRequirementModalityRequired},
				{SourceField: SourceFieldJobRequire, RuleKey: "education.master.shuoshi", MatchedAlias: "硕士", CodePointRange: CodePointRange{Start: 3, End: 5}, Modality: EducationRequirementModalityRequired},
			},
		},
		{
			name:       "unscoped conflicting hard requirements are uncertain",
			input:      EducationParserInput{JobRequire: "算法岗硕士；工程岗本科"},
			wantStatus: EducationRequirementUncertain,
			wantEvidence: []EducationRequirementEvidence{
				{SourceField: SourceFieldJobRequire, RuleKey: "education.master.shuoshi", MatchedAlias: "硕士", CodePointRange: CodePointRange{Start: 3, End: 5}, Modality: EducationRequirementModalityRequired},
				{SourceField: SourceFieldJobRequire, RuleKey: "education.bachelor.benke", MatchedAlias: "本科", CodePointRange: CodePointRange{Start: 9, End: 11}, Modality: EducationRequirementModalityRequired},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseEducationRequirement(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != test.wantStatus || got.LowerBoundLevel != test.wantLower || got.UpperBoundLevel != test.wantUpper {
				t.Fatalf("result = %#v, want status:%q lower:%q upper:%q", got, test.wantStatus, test.wantLower, test.wantUpper)
			}
			if len(got.Evidence) != len(test.wantEvidence) {
				t.Fatalf("evidence = %#v, want %#v", got.Evidence, test.wantEvidence)
			}
			for index := range test.wantEvidence {
				if got.Evidence[index] != test.wantEvidence[index] {
					t.Fatalf("evidence[%d] = %#v, want %#v", index, got.Evidence[index], test.wantEvidence[index])
				}
			}
		})
	}
}

func TestParseEducationRequirementExcludesNonCandidateEducationPhrases(t *testing.T) {
	tests := []struct {
		name  string
		input EducationParserInput
	}{
		{
			name:  "responsibility only clause",
			input: EducationParserInput{JobRequire: "岗位职责：负责本科课程体系建设"},
		},
		{
			name:  "postdoctoral role is not a doctoral requirement",
			input: EducationParserInput{JobRequire: "博士后及以上相关经历"},
		},
		{
			name:  "postgraduate project is not a masters requirement",
			input: EducationParserInput{JobRequire: "具备研究生项目管理经验"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseEducationRequirement(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != EducationRequirementNotConfigured || got.LowerBoundLevel != "" || got.UpperBoundLevel != "" || len(got.Evidence) != 0 {
				t.Fatalf("result = %#v, want no formal education bound or evidence", got)
			}
		})
	}
}

func TestParseEducationRequirementUsesDescriptionOnlyUnderQualificationHeading(t *testing.T) {
	configured, err := ParseEducationRequirement(EducationParserInput{Description: "任职资格：本科及以上"})
	if err != nil {
		t.Fatal(err)
	}
	if configured.Status != EducationRequirementConfigured || configured.LowerBoundLevel != EducationLevelBachelor || len(configured.Evidence) != 1 || configured.Evidence[0] != (EducationRequirementEvidence{
		SourceField: SourceFieldDescription, RuleKey: "education.bachelor.benke", MatchedAlias: "本科",
		CodePointRange: CodePointRange{Start: 5, End: 7}, Modality: EducationRequirementModalityRequired,
	}) {
		t.Fatalf("qualification heading result = %#v", configured)
	}

	responsibility, err := ParseEducationRequirement(EducationParserInput{Description: "岗位职责：负责本科课程体系建设"})
	if err != nil {
		t.Fatal(err)
	}
	if responsibility.Status != EducationRequirementNotConfigured || responsibility.LowerBoundLevel != "" || responsibility.UpperBoundLevel != "" || len(responsibility.Evidence) != 0 {
		t.Fatalf("description responsibility result = %#v, want no formal education requirement", responsibility)
	}
}
