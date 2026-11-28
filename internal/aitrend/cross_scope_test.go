package aitrend

import (
	"fmt"
	"testing"

	"mrolescope/internal/jdserving"
)

func crossTestSnapshot() *ServingSnapshot {
	engines := jdserving.FilterOption{Value: "game-engines", Label: "游戏引擎"}
	languages := jdserving.FilterOption{Value: "programming-languages", Label: "编程语言"}
	return newServingSnapshot(jdserving.FilterOptionsResponse{
		TechnologyKeywords: []jdserving.TechnologyKeywordFilterOption{
			{Category: engines, Value: "unity", Label: "Unity"},
			{Category: engines, Value: "unreal", Label: "Unreal"},
			{Category: languages, Value: "java", Label: "Java"},
			{Category: languages, Value: "python", Label: "Python"},
			{Category: languages, Value: "go", Label: "Go"},
		},
		JobFamilies: []jdserving.FilterOption{{Value: "backend", Label: "后端"}, {Value: "frontend", Label: "前端"}},
		Locations:   []jdserving.FilterOption{{Value: "上海", Label: "上海"}, {Value: "北京", Label: "北京"}},
		Projects:    []jdserving.FilterOption{{Value: "alpha", Label: "Alpha"}},
	})
}

func TestCrossScopeSupportsMultipleSameDimensionGroups(t *testing.T) {
	for _, test := range []struct {
		name, text string
		conditions []CrossConditionGroup
	}{
		{"three-value union", "上海的 Java 或 Python 或 Go 岗位有多少", []CrossConditionGroup{crossValue(DimensionLocation, CrossAny, "上海"), crossValue(DimensionTechnology, CrossAny, "java", "python", "go")}},
		{"unions in two dimensions", "上海或北京的 Java 或 Python 岗位有多少", []CrossConditionGroup{crossValue(DimensionLocation, CrossAny, "上海", "北京"), crossValue(DimensionTechnology, CrossAny, "java", "python")}},
		{"mixed union and intersection", "上海的（Java 或 Python）且 Go 岗位有多少", []CrossConditionGroup{crossValue(DimensionLocation, CrossAny, "上海"), crossValue(DimensionTechnology, CrossAny, "java", "python"), crossValue(DimensionTechnology, CrossAny, "go")}},
		{"union containing an intersection", "（Java 且 Python）或 Go 岗位有多少", []CrossConditionGroup{crossValue(DimensionTechnology, CrossAny, "java", "go"), crossValue(DimensionTechnology, CrossAny, "python", "go")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			scope := buildCrossScopeEvidence(test.text, crossTestSnapshot())
			plan := CrossStatisticsPlan{SubQueries: []CrossStatisticsQuery{{ID: "q", Time: CrossTimeCurrent, Conditions: test.conditions}}}
			if err := validateCrossScopePlan(plan, scope); err != nil {
				t.Fatalf("valid combination rejected: %v, scope=%#v", err, scope)
			}
			bad := plan
			bad.SubQueries = append([]CrossStatisticsQuery(nil), plan.SubQueries...)
			bad.SubQueries[0].Conditions = append([]CrossConditionGroup(nil), test.conditions...)
			for index := range bad.SubQueries[0].Conditions {
				if len(bad.SubQueries[0].Conditions[index].Values) > 1 {
					bad.SubQueries[0].Conditions[index].Operator = CrossAll
					break
				}
			}
			if err := validateCrossScopePlan(bad, scope); err == nil {
				t.Fatal("model changed an OR group to an AND group")
			}
		})
	}
}

func TestCrossScopeDoesNotGuessMixedParentheses(t *testing.T) {
	conditions := []CrossConditionGroup{crossValue(DimensionTechnology, CrossAny, "java"), crossValue(DimensionTechnology, CrossAny, "python", "go")}
	plan := CrossStatisticsPlan{SubQueries: []CrossStatisticsQuery{{ID: "q", Time: CrossTimeCurrent, Conditions: conditions}}}
	for _, text := range []string{"（Java 且 Python）或 Go 岗位有多少", "Java 且 Python 或 Go 岗位有多少"} {
		if err := validateCrossScopePlan(plan, buildCrossScopeEvidence(text, crossTestSnapshot())); err == nil {
			t.Fatalf("mixed conditions were silently reinterpreted: %q", text)
		}
	}
}

func TestCrossScopeRequiresEveryCountForThreeAmbiguousValues(t *testing.T) {
	scope := buildCrossScopeEvidence("Java 和 Python 和 Go 的后端岗位有多少", crossTestSnapshot())
	query := func(id string, operator CrossOperator, values ...string) CrossStatisticsQuery {
		return CrossStatisticsQuery{ID: id, Time: CrossTimeCurrent, Conditions: []CrossConditionGroup{
			crossValue(DimensionTechnology, operator, values...), crossValue(DimensionJobFamily, CrossAny, "backend"),
		}}
	}
	plan := CrossStatisticsPlan{SubQueries: []CrossStatisticsQuery{
		query("java", CrossAny, "java"), query("python", CrossAny, "python"), query("go", CrossAny, "go"),
		query("union", CrossAny, "java", "python", "go"), query("intersection", CrossAll, "java", "python", "go"),
	}}
	if err := validateCrossScopePlan(plan, scope); err != nil {
		t.Fatalf("three-value ambiguous plan rejected: %v, scope=%#v", err, scope)
	}
	plan.SubQueries = plan.SubQueries[1:]
	if err := validateCrossScopePlan(plan, scope); err == nil {
		t.Fatal("ambiguous plan without each individual count was accepted")
	}
}

func TestCrossScopeSupportsTwoAmbiguousDimensionsWithinBudget(t *testing.T) {
	scope := buildCrossScopeEvidence("上海和北京的 Java 和 Python 岗位有多少", crossTestSnapshot())
	locationVariants := []CrossConditionGroup{
		crossValue(DimensionLocation, CrossAny, "上海"), crossValue(DimensionLocation, CrossAny, "北京"),
		crossValue(DimensionLocation, CrossAny, "上海", "北京"), crossValue(DimensionLocation, CrossAll, "上海", "北京"),
	}
	technologyVariants := []CrossConditionGroup{
		crossValue(DimensionTechnology, CrossAny, "java"), crossValue(DimensionTechnology, CrossAny, "python"),
		crossValue(DimensionTechnology, CrossAny, "java", "python"), crossValue(DimensionTechnology, CrossAll, "java", "python"),
	}
	plan := CrossStatisticsPlan{}
	for locationIndex, location := range locationVariants {
		for technologyIndex, technology := range technologyVariants {
			plan.SubQueries = append(plan.SubQueries, CrossStatisticsQuery{
				ID: fmt.Sprintf("q%d-%d", locationIndex, technologyIndex), Time: CrossTimeCurrent,
				Conditions: []CrossConditionGroup{location, technology},
			})
		}
	}
	if err := validateCrossScopePlan(plan, scope); err != nil {
		t.Fatalf("two ambiguous dimensions within budget were rejected: %v, scope=%#v", err, scope)
	}
	plan.SubQueries = plan.SubQueries[:15]
	if err := validateCrossScopePlan(plan, scope); err == nil {
		t.Fatal("incomplete combination of ambiguous dimensions was accepted")
	}
}

func crossValue(dimension Dimension, operator CrossOperator, values ...string) CrossConditionGroup {
	return CrossConditionGroup{Dimension: dimension, Operator: operator, Values: values}
}

func crossCategory(category string) CrossConditionGroup {
	return CrossConditionGroup{Dimension: DimensionTechnology, Operator: CrossAny, Categories: []string{category}}
}

func TestCrossScopeDistinguishesTechnologyRankingAndFamilyGrouping(t *testing.T) {
	snapshot := crossTestSnapshot()
	globalFamilies := CrossStatisticsPlan{SubQueries: []CrossStatisticsQuery{{ID: "q", Time: CrossTimeCurrent, GroupBy: DimensionJobFamily}}}
	scope := buildCrossScopeEvidence("哪些游戏引擎技术对应的岗位最多？", snapshot)
	if err := validateCrossScopePlan(globalFamilies, scope); err == nil {
		t.Fatal("global job-family ranking escaped a confirmed technology category")
	}
	familiesWithinEngines := CrossStatisticsPlan{SubQueries: []CrossStatisticsQuery{{ID: "q", Time: CrossTimeCurrent, GroupBy: DimensionJobFamily, Conditions: []CrossConditionGroup{crossCategory("game-engines")}}}}
	scope = buildCrossScopeEvidence("游戏引擎相关岗位中哪些岗位族最多？", snapshot)
	if err := validateCrossScopePlan(familiesWithinEngines, scope); err != nil {
		t.Fatalf("valid family grouping within the category was rejected: %v, scope=%#v", err, scope)
	}
	if err := validateCrossScopePlan(globalFamilies, scope); err == nil {
		t.Fatal("family grouping dropped the user's category filter")
	}
}

func TestCrossScopePreservesNegationAndSubQuestions(t *testing.T) {
	snapshot := crossTestSnapshot()
	scope := buildCrossScopeEvidence("游戏引擎后端岗位排除 Unity", snapshot)
	query := CrossStatisticsQuery{ID: "q", Time: CrossTimeCurrent, Conditions: []CrossConditionGroup{
		crossCategory("game-engines"), crossValue(DimensionJobFamily, CrossAny, "backend"),
	}, Exclusions: []CrossConditionGroup{crossValue(DimensionTechnology, CrossAny, "unity")}}
	if err := validateCrossScopePlan(CrossStatisticsPlan{SubQueries: []CrossStatisticsQuery{query}}, scope); err != nil {
		t.Fatalf("valid exclusion rejected: %v, scope=%#v", err, scope)
	}
	query.Exclusions = nil
	if err := validateCrossScopePlan(CrossStatisticsPlan{SubQueries: []CrossStatisticsQuery{query}}, scope); err == nil {
		t.Fatal("missing Unity exclusion accepted")
	}
	negativeCategory := buildCrossScopeEvidence("不看游戏引擎", snapshot)
	if len(negativeCategory.clauses) != 1 || len(negativeCategory.clauses[0].mentions) != 1 || !negativeCategory.clauses[0].mentions[0].excluded {
		t.Fatalf("negated category became a positive scope: %#v", negativeCategory)
	}
	scope = buildCrossScopeEvidence("上海 Java 岗位数和北京 Python 岗位数", snapshot)
	shanghai := CrossStatisticsQuery{ID: "sh", Time: CrossTimeCurrent, Conditions: []CrossConditionGroup{crossValue(DimensionLocation, CrossAny, "上海"), crossValue(DimensionTechnology, CrossAny, "java")}}
	beijing := CrossStatisticsQuery{ID: "bj", Time: CrossTimeCurrent, Conditions: []CrossConditionGroup{crossValue(DimensionLocation, CrossAny, "北京"), crossValue(DimensionTechnology, CrossAny, "python")}}
	if err := validateCrossScopePlan(CrossStatisticsPlan{SubQueries: []CrossStatisticsQuery{shanghai, beijing}}, scope); err != nil {
		t.Fatalf("valid independent subquestions rejected: %v, scope=%#v", err, scope)
	}
	shanghai.Conditions[1] = crossValue(DimensionTechnology, CrossAny, "python")
	if err := validateCrossScopePlan(CrossStatisticsPlan{SubQueries: []CrossStatisticsQuery{shanghai, beijing}}, scope); err == nil {
		t.Fatal("a different subquestion's condition was accepted")
	}
}

func TestCrossScopeRequiresAllAmbiguousConjunctionCounts(t *testing.T) {
	scope := buildCrossScopeEvidence("Unity 和 Unreal 的后端岗位有多少？", crossTestSnapshot())
	query := func(id string, operator CrossOperator, values ...string) CrossStatisticsQuery {
		return CrossStatisticsQuery{ID: id, Time: CrossTimeCurrent, Conditions: []CrossConditionGroup{
			crossValue(DimensionTechnology, operator, values...), crossValue(DimensionJobFamily, CrossAny, "backend"),
		}}
	}
	plan := CrossStatisticsPlan{SubQueries: []CrossStatisticsQuery{
		query("first", CrossAny, "unity"), query("second", CrossAny, "unreal"),
		query("union", CrossAny, "unity", "unreal"), query("intersection", CrossAll, "unity", "unreal"),
	}}
	if err := validateCrossScopePlan(plan, scope); err != nil {
		t.Fatalf("ambiguous four-result plan rejected: %v, scope=%#v", err, scope)
	}
	plan.SubQueries = plan.SubQueries[:3]
	if err := validateCrossScopePlan(plan, scope); err == nil {
		t.Fatal("ambiguous conjunction accepted without the intersection")
	}
}

func TestCrossScopeSupportsCategoryAndValueInSameDimension(t *testing.T) {
	scope := buildCrossScopeEvidence("游戏引擎和 Java 岗位有多少", crossTestSnapshot())
	plan := CrossStatisticsPlan{SubQueries: []CrossStatisticsQuery{
		{ID: "category", Time: CrossTimeCurrent, Conditions: []CrossConditionGroup{crossCategory("game-engines")}},
		{ID: "value", Time: CrossTimeCurrent, Conditions: []CrossConditionGroup{crossValue(DimensionTechnology, CrossAny, "java")}},
		{ID: "union", Time: CrossTimeCurrent, Conditions: []CrossConditionGroup{{Dimension: DimensionTechnology, Operator: CrossAny, Categories: []string{"game-engines"}, Values: []string{"java"}}}},
		{ID: "intersection", Time: CrossTimeCurrent, Conditions: []CrossConditionGroup{crossCategory("game-engines"), crossValue(DimensionTechnology, CrossAny, "java")}},
	}}
	if err := validateCrossScopePlan(plan, scope); err != nil {
		t.Fatalf("category/value conjunction was rejected: %v, scope=%#v", err, scope)
	}
}

func TestCrossScopeRejectsUnknownValuesAndCrossDimensionUnion(t *testing.T) {
	snapshot := crossTestSnapshot()
	for _, test := range []struct {
		name  string
		text  string
		query CrossStatisticsQuery
	}{
		{"unknown category", "量子游戏引擎岗位最多", CrossStatisticsQuery{ID: "q", Time: CrossTimeCurrent, Conditions: []CrossConditionGroup{crossCategory("game-engines")}}},
		{"unconfirmed alias", "JS 岗位有多少", CrossStatisticsQuery{ID: "q", Time: CrossTimeCurrent, Conditions: []CrossConditionGroup{crossValue(DimensionTechnology, CrossAny, "java")}}},
		{"unknown exclusion", "Java 岗位排除 Rust", CrossStatisticsQuery{ID: "q", Time: CrossTimeCurrent, Conditions: []CrossConditionGroup{crossValue(DimensionTechnology, CrossAny, "java")}}},
		{"duplicate group narrows a union", "Java 或 Python 岗位有多少", CrossStatisticsQuery{ID: "q", Time: CrossTimeCurrent, Conditions: []CrossConditionGroup{crossValue(DimensionTechnology, CrossAny, "java", "python"), crossValue(DimensionTechnology, CrossAny, "java")}}},
		{"cross dimension union", "上海岗位或 Java 岗位有多少", CrossStatisticsQuery{ID: "q", Time: CrossTimeCurrent, Conditions: []CrossConditionGroup{crossValue(DimensionLocation, CrossAny, "上海"), crossValue(DimensionTechnology, CrossAny, "java")}}},
		{"concrete title is not automatically a family", "后端工程师有多少", CrossStatisticsQuery{ID: "q", Time: CrossTimeCurrent, Conditions: []CrossConditionGroup{crossValue(DimensionJobFamily, CrossAny, "backend")}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateCrossScopePlan(CrossStatisticsPlan{SubQueries: []CrossStatisticsQuery{test.query}}, buildCrossScopeEvidence(test.text, snapshot)); err == nil {
				t.Fatal("unconfirmed or unsupported scope was accepted")
			}
		})
	}
}

func TestCrossScopeAcceptsOnlyFormalExplicitSelections(t *testing.T) {
	snapshot := crossTestSnapshot()
	selection := crossScopeSelection{clause: 0, dimension: DimensionTechnology, value: "java"}
	scope := buildCrossScopeEvidence("统计当前岗位", snapshot, selection)
	plan := CrossStatisticsPlan{SubQueries: []CrossStatisticsQuery{{ID: "q", Time: CrossTimeCurrent, Conditions: []CrossConditionGroup{crossValue(DimensionTechnology, CrossAny, "java")}}}}
	if err := validateCrossScopePlan(plan, scope); err != nil {
		t.Fatalf("formal explicit selection rejected: %v", err)
	}
	selection.value = "not-a-formal-key"
	if err := validateCrossScopePlan(plan, buildCrossScopeEvidence("统计当前岗位", snapshot, selection)); err == nil {
		t.Fatal("unknown explicit selection was treated as a confirmed value")
	}
}

func TestCrossScopeRecognizesConfirmedCategoryAlias(t *testing.T) {
	snapshot := newServingSnapshot(jdserving.FilterOptionsResponse{TechnologyKeywords: []jdserving.TechnologyKeywordFilterOption{
		{Category: jdserving.FilterOption{Value: "databases-and-storage", Label: "数据库与存储"}, Value: "mysql", Label: "MySQL"},
	}})
	scope := buildCrossScopeEvidence("数据库技术排名", snapshot)
	query := CrossStatisticsQuery{ID: "q", Time: CrossTimeCurrent, GroupBy: DimensionTechnology, Conditions: []CrossConditionGroup{crossCategory("databases-and-storage")}}
	if err := validateCrossScopePlan(CrossStatisticsPlan{SubQueries: []CrossStatisticsQuery{query}}, scope); err != nil {
		t.Fatalf("confirmed category alias rejected: %v, scope=%#v", err, scope)
	}
}

func TestCrossScopeRecognizesPostfixExclusion(t *testing.T) {
	scope := buildCrossScopeEvidence("Unity 以外的 Java 岗位", crossTestSnapshot())
	query := CrossStatisticsQuery{ID: "q", Time: CrossTimeCurrent,
		Conditions: []CrossConditionGroup{crossValue(DimensionTechnology, CrossAny, "java")},
		Exclusions: []CrossConditionGroup{crossValue(DimensionTechnology, CrossAny, "unity")},
	}
	if err := validateCrossScopePlan(CrossStatisticsPlan{SubQueries: []CrossStatisticsQuery{query}}, scope); err != nil {
		t.Fatalf("postfix exclusion was not preserved: %v, scope=%#v", err, scope)
	}
}

func TestCrossScopeUsesTheUsersRequestedRankingLength(t *testing.T) {
	for _, test := range []struct {
		text string
		want int
	}{
		{"游戏引擎岗位中所有岗位族排名", 0},
		{"游戏引擎岗位的岗位族前20名", 20},
		{"游戏引擎岗位的岗位族前十二名", 12},
		{"游戏引擎岗位中展示20个岗位族排名", 20},
		{"游戏引擎岗位中列出二十项岗位族排名", 20},
		{"游戏引擎岗位中哪个岗位族最多", 1},
	} {
		t.Run(test.text, func(t *testing.T) {
			limit := 8
			plan := CrossStatisticsPlan{SubQueries: []CrossStatisticsQuery{{ID: "q", Time: CrossTimeCurrent, GroupBy: DimensionJobFamily, RankingLimit: &limit, Conditions: []CrossConditionGroup{crossCategory("game-engines")}}}}
			scope := buildCrossScopeEvidence(test.text, crossTestSnapshot())
			if err := validateCrossScopePlan(plan, scope); err != nil {
				t.Fatalf("valid ranking scope: %v, %#v", err, scope)
			}
			got := normalizeCrossRanking(plan, scope).SubQueries[0].RankingLimit
			if got == nil || *got != test.want {
				t.Fatalf("ranking limit = %v, want %d", got, test.want)
			}
		})
	}
}

func TestCrossScopeSeparatesCommaDelimitedSubQuestions(t *testing.T) {
	snapshot := crossTestSnapshot()
	scope := buildCrossScopeEvidence("上海 Java 岗位数，北京 Python 岗位数", snapshot)
	if len(scope.clauses) != 2 {
		t.Fatalf("comma-delimited questions were not separated: %#v", scope)
	}
	shanghai := CrossStatisticsQuery{ID: "sh", Time: CrossTimeCurrent, Conditions: []CrossConditionGroup{
		crossValue(DimensionLocation, CrossAny, "上海"), crossValue(DimensionTechnology, CrossAny, "java"),
	}}
	beijing := CrossStatisticsQuery{ID: "bj", Time: CrossTimeCurrent, Conditions: []CrossConditionGroup{
		crossValue(DimensionLocation, CrossAny, "北京"), crossValue(DimensionTechnology, CrossAny, "python"),
	}}
	if err := validateCrossScopePlan(CrossStatisticsPlan{SubQueries: []CrossStatisticsQuery{shanghai, beijing}}, scope); err != nil {
		t.Fatalf("comma-delimited independent questions were rejected: %v, scope=%#v", err, scope)
	}
	unchanged := buildCrossScopeEvidence("上海的 Java, Python 岗位数", snapshot)
	if len(unchanged.clauses) != 1 {
		t.Fatalf("ordinary value list was split into independent questions: %#v", unchanged)
	}
}

func TestTrendScopeBlocksCategoryDimensionChangeAndNegationLoss(t *testing.T) {
	snapshot := crossTestSnapshot()
	intent := Intent{SubQueries: []SubQuery{{ID: "q", Dimension: DimensionJobFamily, Expressions: []string{"backend"}}}}
	groups := []CandidateGroup{{SubQueryID: "q", Candidates: []Candidate{{Dimension: DimensionJobFamily, Value: "backend"}}}}
	session := newCrossStatisticsSession("哪些游戏引擎技术对应的岗位最多", snapshot)
	if err := validateTrendCategoryScope(intent, groups, session, snapshot); err == nil {
		t.Fatal("technology category ranking changed to global job-family ranking")
	}
	intent.SubQueries[0].Dimension = DimensionTechnology
	groups[0].Candidates = []Candidate{{Dimension: DimensionTechnology, Value: "java"}}
	if err := validateTrendCategoryScope(intent, groups, session, snapshot); err == nil {
		t.Fatal("unrelated technology escaped the confirmed category")
	}
	groups[0].Candidates[0].Value = "unity"
	if err := validateTrendCategoryScope(intent, groups, session, snapshot); err != nil {
		t.Fatalf("valid category candidate rejected: %v", err)
	}
	negative := newCrossStatisticsSession("不看游戏引擎的技术趋势", snapshot)
	if err := validateTrendCategoryScope(intent, groups, negative, snapshot); err == nil {
		t.Fatal("negated category was included in the draft")
	}
}

func TestTrendScopeDoesNotBroadenAnUnknownCategory(t *testing.T) {
	snapshot := crossTestSnapshot()
	intent := Intent{SubQueries: []SubQuery{{ID: "q", Dimension: DimensionTechnology, Expressions: []string{"*"}}}}
	groups := []CandidateGroup{{SubQueryID: "q", Candidates: []Candidate{{Dimension: DimensionTechnology, Value: "java"}}}}
	for _, text := range []string{"哪些量子框架的岗位最多", "量子分类有哪些技术岗位", "Java 岗位趋势"} {
		if err := validateTrendCategoryScope(intent, groups, newCrossStatisticsSession(text, snapshot), snapshot); err == nil {
			t.Fatalf("unconfirmed scope %q became a global trend", text)
		}
	}
	if err := validateTrendCategoryScope(intent, groups, newCrossStatisticsSession("哪些技术岗位最多", snapshot), snapshot); err != nil {
		t.Fatalf("explicit whole-dimension request rejected: %v", err)
	}
}
