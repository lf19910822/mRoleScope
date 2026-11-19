package aitrend

import (
	"testing"

	"mrolescope/internal/jdserving"
)

func TestResolverUsesFormalValuesAndBoundsFuzzyExpansion(t *testing.T) {
	resolver := Resolver{Values: map[Dimension][]Candidate{
		DimensionTechnology: {
			{Dimension: DimensionTechnology, Value: "java", Label: "Java", MatchStatus: MatchExact},
			{Dimension: DimensionTechnology, Value: "javascript", Label: "JavaScript"},
			{Dimension: DimensionTechnology, Value: "java-backend", Label: "Java 后端"},
			{Dimension: DimensionTechnology, Value: "java-test", Label: "Java 测试"},
		},
	}}
	got := resolver.Resolve(DimensionTechnology, "Java", "q1")
	if len(got) != MaxExpressionCandidates {
		t.Fatalf("got %d candidates, want %d", len(got), MaxExpressionCandidates)
	}
	if got[0].Value != "java" || got[0].MatchStatus != MatchExact {
		t.Fatalf("first candidate = %#v", got[0])
	}
}

func TestResolverTieBreaksByFormalValue(t *testing.T) {
	resolver := Resolver{Values: map[Dimension][]Candidate{
		DimensionProject: {
			{Dimension: DimensionProject, Value: "zeta", Label: "平台"},
			{Dimension: DimensionProject, Value: "alpha", Label: "平台"},
		},
	}}
	got := resolver.Resolve(DimensionProject, "平台", "q1")
	if len(got) != 2 || got[0].Value != "alpha" {
		t.Fatalf("candidates = %#v, want alpha first", got)
	}
}

func TestResolverExcludesFormalValues(t *testing.T) {
	resolver := Resolver{Values: map[Dimension][]Candidate{
		DimensionTechnology: {
			{Dimension: DimensionTechnology, Value: "mysql", Label: "MySQL", MatchStatus: MatchExact},
			{Dimension: DimensionTechnology, Value: "redis", Label: "Redis", MatchStatus: MatchExact},
		},
	}}
	got := resolver.ResolveWithExclusions(DimensionTechnology, "数据库", []string{"MySQL"}, "q1")
	for _, candidate := range got {
		if candidate.Value == "mysql" {
			t.Fatal("excluded candidate was retained")
		}
	}
}

func TestAllocateCandidatesReservesEachGroup(t *testing.T) {
	groups := [][]Candidate{
		{{Dimension: DimensionTechnology, Value: "mysql", Score: 100, SubQueryID: "database"}, {Dimension: DimensionTechnology, Value: "redis", Score: 90, SubQueryID: "database"}},
		{{Dimension: DimensionProject, Value: "alpha", Score: 10, SubQueryID: "project"}},
	}
	got := AllocateCandidates(groups)
	if len(got) != 3 || got[0].Value != "mysql" || got[1].Value != "alpha" || got[2].Value != "redis" {
		t.Fatalf("allocated = %#v", got)
	}
}

func TestMergeCandidatesKeepsDifferentJobFamilyScope(t *testing.T) {
	primary := false
	secondary := true
	got := MergeCandidates([]Candidate{
		{Dimension: DimensionJobFamily, Value: "backend", IncludeSecondary: primary},
		{Dimension: DimensionJobFamily, Value: "backend", IncludeSecondary: secondary},
	})
	if len(got) != 2 {
		t.Fatalf("merged = %#v, want two scopes", got)
	}
}

func TestResolverNaturalExpansionAndAmbiguity(t *testing.T) {
	resolver := Resolver{Values: map[Dimension][]Candidate{
		DimensionTechnology: {
			{Dimension: DimensionTechnology, Value: "mysql", Label: "MySQL"},
			{Dimension: DimensionTechnology, Value: "redis", Label: "Redis"},
		},
		DimensionProject: {{Dimension: DimensionProject, Value: "project", Label: "项目"}},
	}}
	got := resolver.ResolveNatural("数据库", "q1")
	if len(got) == 0 || got[0].MatchStatus != MatchExpansion {
		t.Fatalf("natural expansion = %#v", got)
	}
	ambiguous := resolver.ResolveAcrossDimensions("project", "q2")
	if len(ambiguous) == 0 || ambiguous[0].Dimension != DimensionProject {
		t.Fatalf("ambiguous candidates = %#v", ambiguous)
	}
}

func TestResolverTreatsEmptyExpressionAsBroadDimension(t *testing.T) {
	resolver := Resolver{Values: map[Dimension][]Candidate{
		DimensionProject: {
			{Dimension: DimensionProject, Value: "alpha", Label: "Alpha"},
			{Dimension: DimensionProject, Value: "beta", Label: "Beta"},
		},
	}}
	query := SubQuery{ID: "projects", Dimension: DimensionProject, Expressions: []string{""}}
	got := resolveIntentExpression(resolver, query, "")
	if len(got) != 2 {
		t.Fatalf("broad candidates = %#v", got)
	}
	for _, candidate := range got {
		if candidate.MatchStatus != MatchExpansion || candidate.Scope != "扩展推荐" {
			t.Fatalf("candidate = %#v, want expansion recommendation", candidate)
		}
	}
}

func TestBroadExpressionRecognizesWildcardAndWholeDimensionPhrases(t *testing.T) {
	for _, expression := range []string{"*", "all", "全部项目", "所有岗位族"} {
		if !isBroadExpression(expression) {
			t.Fatalf("isBroadExpression(%q) = false", expression)
		}
	}
}

func TestResolverProjectCandidatesExcludeUnrecognizedPseudoValue(t *testing.T) {
	resolver := resolverFromOptions(jdserving.FilterOptionsResponse{Projects: []jdserving.FilterOption{
		{Value: "alpha", Label: "Alpha"},
		{Value: "unrecognized", Label: "未识别项目"},
	}})
	for _, candidate := range resolver.Values[DimensionProject] {
		if candidate.Value == "unrecognized" {
			t.Fatalf("resolver retained pseudo value: %#v", resolver.Values[DimensionProject])
		}
	}
}

func TestPlanServingToolsSkipsBroadRankExpression(t *testing.T) {
	intent := Intent{SubQueries: []SubQuery{{ID: "projects", Dimension: DimensionProject, Expressions: []string{""}}}}
	calls := planServingTools(intent, []CandidateGroup{{SubQueryID: "projects", Candidates: []Candidate{{Dimension: DimensionProject, Value: "alpha"}}}})
	for _, call := range calls {
		if call.Name != ToolRankValues {
			continue
		}
		t.Fatalf("broad query emitted rank call: %#v", call)
	}
}
