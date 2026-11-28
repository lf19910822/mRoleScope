package aitrend

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

var crossExplicitRankingCount = regexp.MustCompile(`(?i)(?:前|top\s*)([0-9]{1,6})`)
var crossChineseRankingCount = regexp.MustCompile(`前([一二两三四五六七八九十百]{1,5})`)
var crossDisplayRankingCount = regexp.MustCompile(`(?:展示|显示|列出|给出|提供)\s*([0-9]{1,6})\s*(?:个|项|名|条)`)
var crossChineseDisplayRankingCount = regexp.MustCompile(`(?:展示|显示|列出|给出|提供)\s*([一二两三四五六七八九十百]{1,5})\s*(?:个|项|名|条)`)
var crossHistoricalDate = regexp.MustCompile(`(?:19|20)[0-9]{2}(?:年|[-/])`)

// crossScopeEvidence comes only from the user's wording and the request's
// formal vocabulary. Tool calls and model-produced intents are never evidence.
type crossScopeEvidence struct {
	clauses []crossScopeClause
}

type crossScopeClause struct {
	text      string
	mentions  []crossScopeMention
	groupBy   Dimension
	uncertain bool
}

type crossScopeMention struct {
	dimension Dimension
	value     string
	category  bool
	excluded  bool
	start     int
	end       int
}

type crossScopeTerm struct {
	text      string
	dimension Dimension
	value     string
	category  bool
}

// crossScopeSelection is populated only by an explicit user selection. The
// current text-only HTTP request does not supply selections, so no dashboard
// state is inferred implicitly.
type crossScopeSelection struct {
	clause    int
	dimension Dimension
	value     string
	category  bool
	excluded  bool
}

func requiresCrossStatistics(scope crossScopeEvidence) bool {
	for _, clause := range scope.clauses {
		positive := make(map[Dimension]int)
		for _, mention := range clause.mentions {
			if !mention.excluded {
				positive[mention.dimension]++
			}
		}
		if len(positive) > 1 {
			return true
		}
		for dimension, count := range positive {
			if count > 1 || clause.groupBy != "" && clause.groupBy != dimension {
				return true
			}
		}
		for _, marker := range []string{"交集", "并集", "排除", "不包括", "不包含", "同时"} {
			if strings.Contains(clause.text, marker) && (strings.Contains(clause.text, "岗位") || strings.Contains(clause.text, "数量")) {
				return true
			}
		}
	}
	return false
}

func buildCrossScopeEvidence(query string, snapshot *ServingSnapshot, selections ...crossScopeSelection) crossScopeEvidence {
	if snapshot == nil {
		return crossScopeEvidence{clauses: []crossScopeClause{{uncertain: true}}}
	}
	terms := make([]crossScopeTerm, 0)
	add := func(dimension Dimension, value, label string, category bool) {
		for _, text := range []string{value, label} {
			text = normalizeExpression(text)
			if text != "" {
				terms = append(terms, crossScopeTerm{text: text, dimension: dimension, value: value, category: category})
			}
		}
	}
	for _, option := range snapshot.options.TechnologyKeywords {
		add(DimensionTechnology, option.Value, option.Label, false)
	}
	for _, option := range snapshot.categories {
		add(DimensionTechnology, option.Value, option.Label, true)
		for _, alias := range categoryAliases(option.Label) {
			add(DimensionTechnology, option.Value, alias, true)
		}
	}
	for _, entry := range []struct {
		dimension Dimension
		options   []FilterOption
	}{
		{DimensionProject, func() []FilterOption {
			return (ServingToolService{}).optionsForDimension(snapshot.options, DimensionProject)
		}()},
		{DimensionJobFamily, func() []FilterOption {
			return (ServingToolService{}).optionsForDimension(snapshot.options, DimensionJobFamily)
		}()},
		{DimensionLocation, func() []FilterOption {
			return (ServingToolService{}).optionsForDimension(snapshot.options, DimensionLocation)
		}()},
	} {
		for _, option := range entry.options {
			add(entry.dimension, option.Value, option.Label, false)
		}
	}
	// A clear repeated question boundary can separate conditions without
	// treating every Chinese conjunction as a separate question.
	text := strings.ToLower(query)
	for _, marker := range []string{"岗位数和", "岗位数量和", "岗位数以及", "岗位数量以及"} {
		text = strings.ReplaceAll(text, marker, strings.TrimSuffix(strings.TrimSuffix(marker, "以及"), "和")+"；")
	}
	parts := splitCrossScopeClauses(text, terms)
	result := crossScopeEvidence{}
	for _, part := range parts {
		clause := crossScopeClause{text: strings.TrimSpace(part)}
		if clause.text == "" {
			continue
		}
		clause.mentions, clause.uncertain = matchCrossScopeTerms(clause.text, terms)
		for _, marker := range []string{"不排除", "不需要排除", "不是不", "并非不"} {
			if strings.Contains(clause.text, marker) {
				clause.uncertain = true
			}
		}
		clause.groupBy = crossRequestedGrouping(clause.text, clause.mentions)
		if strings.Contains(clause.text, "分别") || strings.Contains(clause.text, "各自") {
			counts := make(map[Dimension]int)
			for _, mention := range clause.mentions {
				if !mention.excluded {
					counts[mention.dimension]++
				}
			}
			multiDimensions := 0
			for _, count := range counts {
				if count > 1 {
					multiDimensions++
				}
			}
			if multiDimensions > 1 {
				clause.uncertain = true
			}
		}
		result.clauses = append(result.clauses, clause)
	}
	for _, selection := range selections {
		if selection.clause < 0 || selection.clause >= len(result.clauses) || !isCrossDimension(selection.dimension) {
			if len(result.clauses) == 0 {
				result.clauses = append(result.clauses, crossScopeClause{uncertain: true})
			} else {
				result.clauses[0].uncertain = true
			}
			continue
		}
		clause := &result.clauses[selection.clause]
		confirmed := false
		if selection.category && selection.dimension == DimensionTechnology {
			values, formal, ok := snapshot.categoryValues(selection.value)
			confirmed = ok && formal == selection.value && len(values) > 0
		} else if !selection.category {
			for _, option := range (ServingToolService{}).optionsForDimension(snapshot.options, selection.dimension) {
				if option.Value == selection.value {
					confirmed = true
					break
				}
			}
		}
		if !confirmed {
			clause.uncertain = true
			continue
		}
		duplicate := false
		for _, mention := range clause.mentions {
			if mention.dimension == selection.dimension && mention.value == selection.value && mention.category == selection.category {
				if mention.excluded != selection.excluded {
					clause.uncertain = true
				}
				duplicate = true
			}
		}
		if !duplicate {
			clause.mentions = append(clause.mentions, crossScopeMention{dimension: selection.dimension, value: selection.value, category: selection.category, excluded: selection.excluded, start: len(clause.text), end: len(clause.text)})
		}
	}
	return result
}

// splitCrossScopeClauses treats a comma as a sub-question boundary only when
// its left side finishes a count question and its right side starts another
// independently recognizable cross-statistics question. This keeps ordinary
// value lists such as "Java、Python" within one scope.
func splitCrossScopeClauses(text string, terms []crossScopeTerm) []string {
	parts := strings.FieldsFunc(text, func(r rune) bool { return r == '？' || r == '?' || r == '；' || r == ';' || r == '\n' })
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		for {
			index, width, found := crossIndependentCommaBoundary(part, terms)
			if !found {
				if part != "" {
					result = append(result, part)
				}
				break
			}
			result = append(result, strings.TrimSpace(part[:index]))
			part = strings.TrimSpace(part[index+width:])
		}
	}
	return result
}

func crossIndependentCommaBoundary(text string, terms []crossScopeTerm) (int, int, bool) {
	for offset := 0; offset < len(text); {
		chinese := strings.Index(text[offset:], "，")
		ascii := strings.Index(text[offset:], ",")
		index, width := -1, 0
		switch {
		case chinese >= 0 && (ascii < 0 || chinese <= ascii):
			index, width = offset+chinese, len("，")
		case ascii >= 0:
			index, width = offset+ascii, 1
		default:
			return 0, 0, false
		}
		left, right := text[:index], strings.TrimSpace(text[index+width:])
		if crossEndsWithCountQuestion(left) && crossStartsIndependentStatisticsQuestion(right, terms) {
			return index, width, true
		}
		offset = index + width
	}
	return 0, 0, false
}

func crossEndsWithCountQuestion(text string) bool {
	text = strings.TrimSpace(text)
	for _, suffix := range []string{"岗位数量", "岗位总数", "岗位数", "职位数量", "职位数", "有多少", "数量"} {
		if strings.HasSuffix(text, suffix) {
			return true
		}
	}
	return false
}

func crossStartsIndependentStatisticsQuestion(text string, terms []crossScopeTerm) bool {
	if !strings.Contains(text, "岗位") && !strings.Contains(text, "数量") && !strings.Contains(text, "多少") {
		return false
	}
	mentions, uncertain := matchCrossScopeTerms(text, terms)
	if uncertain {
		return false
	}
	dimensions := make(map[Dimension]struct{})
	for _, mention := range mentions {
		if !mention.excluded {
			dimensions[mention.dimension] = struct{}{}
		}
	}
	return len(dimensions) >= 2
}

func matchCrossScopeTerms(text string, terms []crossScopeTerm) ([]crossScopeMention, bool) {
	var matches []crossScopeMention
	for _, term := range terms {
		for from := 0; from < len(text); {
			index := strings.Index(text[from:], term.text)
			if index < 0 {
				break
			}
			start := from + index
			end := start + len(term.text)
			from = end
			if !crossTermBoundary(text, start, end, term.text) {
				continue
			}
			matches = append(matches, crossScopeMention{dimension: term.dimension, value: term.value, category: term.category, start: start, end: end})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].start != matches[j].start {
			return matches[i].start < matches[j].start
		}
		return matches[i].end > matches[j].end
	})
	result := make([]crossScopeMention, 0, len(matches))
	uncertain := false
	for _, match := range matches {
		if len(result) > 0 && match.start < result[len(result)-1].end {
			previous := result[len(result)-1]
			if match.start == previous.start && match.end == previous.end && (match.dimension != previous.dimension || match.value != previous.value || match.category != previous.category) {
				uncertain = true
			}
			continue
		}
		prefix := text[:match.start]
		if match.category && !crossCategoryPrefixIsKnown(prefix) {
			uncertain = true
			continue
		}
		if match.dimension == DimensionJobFamily && !match.category {
			suffix := text[match.end:]
			if strings.HasPrefix(suffix, "工程师") || strings.HasPrefix(suffix, "开发工程师") {
				uncertain = true
				continue
			}
		}
		between := prefix
		if len(result) > 0 {
			between = text[result[len(result)-1].end:match.start]
		}
		match.excluded = crossHasNegation(between)
		if crossHasPostfixNegation(text[match.end:]) {
			match.excluded = true
		}
		if !match.excluded && len(result) > 0 && result[len(result)-1].excluded && crossOnlyConjunction(between) {
			match.excluded = true
		}
		result = append(result, match)
	}
	if len(result) > 0 && crossHasNegation(text[result[len(result)-1].end:]) {
		// An exclusion whose value is not in the formal vocabulary cannot be
		// silently removed, even when earlier positive values are known.
		uncertain = true
	}
	return result, uncertain
}

func crossTermBoundary(text string, start, end int, term string) bool {
	first, last := []rune(term)[0], []rune(term)[len([]rune(term))-1]
	if isASCIIWord(first) && start > 0 {
		previous := []rune(text[:start])
		if len(previous) > 0 && isASCIIWord(previous[len(previous)-1]) {
			return false
		}
	}
	if isASCIIWord(last) && end < len(text) {
		next := []rune(text[end:])
		if len(next) > 0 && isASCIIWord(next[0]) {
			return false
		}
	}
	return true
}

func isASCIIWord(r rune) bool {
	return r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_')
}

func crossCategoryPrefixIsKnown(prefix string) bool {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return true
	}
	for _, allowed := range []string{"的", "哪些", "哪个", "使用", "包含", "含有", "涉及", "关于", "查看", "查询", "统计", "排除", "不看", "不含", "不包括", "不包含", "除去", "以及", "或者", "和", "与", "、", "，", ",", "(", "上个月", "上月", "上周", "去年", "当前", "现在", "近一个月", "近一年"} {
		if strings.HasSuffix(prefix, allowed) {
			return true
		}
	}
	last := []rune(prefix)
	return len(last) > 0 && !unicode.IsLetter(last[len(last)-1]) && !unicode.IsDigit(last[len(last)-1])
}

func crossHasNegation(text string) bool {
	for _, marker := range []string{"排除", "不包括", "不包含", "不含", "不看", "不要", "去掉", "除去", "除了", "不考虑", "没有", "无需", "不需要", "不使用", "不用", "并非", "不涉及"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func crossHasPostfixNegation(text string) bool {
	text = strings.TrimSpace(text)
	return strings.HasPrefix(text, "之外") || strings.HasPrefix(text, "以外")
}

func crossOnlyConjunction(text string) bool {
	text = strings.TrimSpace(text)
	switch text {
	case "和", "与", "、", "及", "以及", "或", "或者", "和/或":
		return true
	default:
		return false
	}
}

func crossRequestedGrouping(text string, mentions []crossScopeMention) Dimension {
	if (strings.Contains(text, "岗位族") || strings.Contains(text, "岗位类别")) && (strings.Contains(text, "哪些") || strings.Contains(text, "哪个") || strings.Contains(text, "排行") || strings.Contains(text, "排名") || strings.Contains(text, "分组") || strings.Contains(text, "按岗位族") || crossExplicitRankingCount.MatchString(text) || crossChineseRankingCount.MatchString(text)) {
		return DimensionJobFamily
	}
	if strings.Contains(text, "按项目") || strings.Contains(text, "哪些项目") || strings.Contains(text, "项目排名") {
		return DimensionProject
	}
	if strings.Contains(text, "按地点") || strings.Contains(text, "哪些地点") || strings.Contains(text, "城市排名") {
		return DimensionLocation
	}
	if strings.Contains(text, "按技术") || strings.Contains(text, "哪些技术") || strings.Contains(text, "技术排名") || strings.Contains(text, "技术对应") {
		return DimensionTechnology
	}
	for _, mention := range mentions {
		if mention.category && !mention.excluded && (strings.Contains(text, "哪些") || strings.Contains(text, "排名")) && strings.Contains(text, "技术") {
			return DimensionTechnology
		}
	}
	return ""
}

// validateCrossScopePlan compares each sub-question with independently
// recognized user evidence. A model-proposed value cannot confirm itself.
func validateCrossScopePlan(plan CrossStatisticsPlan, scope crossScopeEvidence) error {
	if err := ValidateCrossStatisticsPlan(plan); err != nil {
		return err
	}
	if len(scope.clauses) == 0 {
		return contractError(ErrorInvalidValue, "scope", "无法从用户问题确认统计范围")
	}
	assignments := make([][]CrossStatisticsQuery, len(scope.clauses))
	for _, query := range plan.SubQueries {
		matched := -1
		for index, clause := range scope.clauses {
			if validateCrossScopeQuery(query, clause) == nil {
				if matched != -1 {
					return contractError(ErrorInvalidCombination, "scope", "子问题归属不明确")
				}
				matched = index
			}
		}
		if matched < 0 {
			return contractError(ErrorInvalidCombination, "scope", "统计条件与用户确认的范围不一致")
		}
		assignments[matched] = append(assignments[matched], query)
	}
	for index, clause := range scope.clauses {
		if clause.uncertain {
			return contractError(ErrorInvalidCombination, "scope", "用户范围含义或正式映射不明确")
		}
		if len(clause.mentions) == 0 && clause.groupBy == "" {
			return contractError(ErrorInvalidValue, "scope", "存在无法确认范围的子问题")
		}
		if len(assignments[index]) == 0 {
			return contractError(ErrorMissingField, "scope", "缺少用户子问题的统计")
		}
		relations, err := crossScopeRelations(clause)
		if err != nil {
			return err
		}
		var unresolved []crossScopeRelation
		for relationIndex := range relations {
			if relations[relationIndex].mode == "ambiguous" || relations[relationIndex].mode == "separate" {
				unresolved = append(unresolved, relations[relationIndex])
			}
		}
		if len(unresolved) == 0 {
			if len(assignments[index]) != 1 {
				return contractError(ErrorInvalidCombination, "scope", "统计结果与子问题数量不一致")
			}
			continue
		}
		expected := 1
		for _, relation := range unresolved {
			count := len(relation.mentions)
			if relation.mode == "ambiguous" {
				count += 2
			}
			if expected > MaxCrossSubQueries/count {
				return contractError(ErrorLimitExceeded, "scope", "关系不明确的组合超出单次统计范围")
			}
			expected *= count
		}
		seen := make(map[string]bool)
		for _, query := range assignments[index] {
			kinds := make([]string, 0, len(unresolved))
			for _, relation := range unresolved {
				kinds = append(kinds, crossRelationQueryKind(query, relation))
			}
			key := strings.Join(kinds, "\x00")
			if seen[key] {
				return contractError(ErrorInvalidCombination, "scope", "同维度统计口径重复")
			}
			seen[key] = true
		}
		if len(seen) != expected {
			return contractError(ErrorMissingField, "scope", "同维度条件缺少分别、并集或交集口径")
		}
	}
	return nil
}

// normalizeCrossRanking takes the requested display size from the user's
// wording, so a model cannot silently turn "全部" into the chart limit.
func normalizeCrossRanking(plan CrossStatisticsPlan, scope crossScopeEvidence) CrossStatisticsPlan {
	plan.SubQueries = append([]CrossStatisticsQuery(nil), plan.SubQueries...)
	for index := range plan.SubQueries {
		query := &plan.SubQueries[index]
		if query.GroupBy == "" {
			continue
		}
		for _, clause := range scope.clauses {
			if validateCrossScopeQuery(*query, clause) != nil {
				continue
			}
			if strings.Contains(clause.text, "全部") || strings.Contains(clause.text, "所有") {
				value := 0
				query.RankingLimit = &value
			} else if value, ok := crossRequestedRankingLimit(clause.text); ok {
				query.RankingLimit = &value
			} else if isExplicitSingleRankingQuery(clause.text) {
				value := 1
				query.RankingLimit = &value
			} else {
				query.RankingLimit = nil
			}
			break
		}
	}
	return plan
}

func crossRequestedRankingLimit(text string) (int, bool) {
	for _, expression := range []*regexp.Regexp{crossExplicitRankingCount, crossDisplayRankingCount} {
		if match := expression.FindStringSubmatch(text); len(match) == 2 {
			value, err := strconv.Atoi(match[1])
			if err == nil && value > 0 && value <= 1_000_000 {
				return value, true
			}
		}
	}
	for _, expression := range []*regexp.Regexp{crossChineseRankingCount, crossChineseDisplayRankingCount} {
		if match := expression.FindStringSubmatch(text); len(match) == 2 {
			if value := crossChineseNumber(match[1]); value > 0 && value <= 1_000_000 {
				return value, true
			}
		}
	}
	return 0, false
}

func crossChineseNumber(value string) int {
	digits := map[rune]int{'一': 1, '二': 2, '两': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
	total, pending, previousUnit := 0, 0, 1000
	for _, character := range value {
		if digit, ok := digits[character]; ok {
			if pending != 0 {
				return 0
			}
			pending = digit
			continue
		}
		unit := 0
		if character == '十' {
			unit = 10
		} else if character == '百' {
			unit = 100
		}
		if unit == 0 || unit >= previousUnit {
			return 0
		}
		if pending == 0 {
			pending = 1
		}
		total += pending * unit
		pending = 0
		previousUnit = unit
	}
	return total + pending
}

type crossScopeRelation struct {
	dimension Dimension
	mentions  []crossScopeMention
	// Each clause is an OR of keys; clauses themselves are ANDed.
	clauses [][]string
	mode    string
}

func crossScopeRelations(clause crossScopeClause) ([]crossScopeRelation, error) {
	positive := make([]crossScopeMention, 0, len(clause.mentions))
	seen := make(map[string]bool)
	for _, mention := range clause.mentions {
		if mention.excluded {
			continue
		}
		key := crossScopeMentionKey(mention.dimension, mention.value, mention.category)
		if !seen[key] {
			positive = append(positive, mention)
			seen[key] = true
		}
	}
	for index := 1; index < len(positive); index++ {
		first, second := positive[index-1], positive[index]
		if first.dimension != second.dimension && strings.Contains(clause.text[first.end:second.start], "或") {
			return nil, contractError(ErrorInvalidCombination, "scope", "暂不支持跨维度并集")
		}
	}
	dimensions := []Dimension{DimensionTechnology, DimensionProject, DimensionJobFamily, DimensionLocation}
	var relations []crossScopeRelation
	for _, dimension := range dimensions {
		var mentions []crossScopeMention
		var positions []int
		for index, mention := range positive {
			if mention.dimension == dimension {
				mentions = append(mentions, mention)
				positions = append(positions, index)
			}
		}
		if len(mentions) < 2 {
			continue
		}
		edges := make([]string, 0, len(mentions)-1)
		hasOr, hasExplicitAnd, hasUnspecifiedAnd := false, false, false
		for index := 1; index < len(mentions); index++ {
			if positions[index] != positions[index-1]+1 {
				return nil, contractError(ErrorInvalidCombination, "scope", "同维度条件的归属不明确")
			}
			between := clause.text[mentions[index-1].end:mentions[index].start]
			switch {
			case strings.Contains(between, "或"):
				edges = append(edges, "or")
				hasOr = true
			case strings.Contains(between, "且") || strings.Contains(between, "并且"):
				edges = append(edges, "and")
				hasExplicitAnd = true
			case strings.Contains(between, "和") || strings.Contains(between, "与") || strings.Contains(between, "、") || strings.Contains(between, "以及"):
				edges = append(edges, "and")
				hasUnspecifiedAnd = true
			default:
				return nil, contractError(ErrorInvalidCombination, "scope", "同维度值之间的关系不明确")
			}
		}
		explicitAll := strings.Contains(clause.text, "同时") || strings.Contains(clause.text, "交集") || strings.Contains(clause.text, "同一岗位") || strings.Contains(clause.text, "同一个岗位") || strings.Contains(clause.text, "都命中") || strings.Contains(clause.text, "均命中")
		explicitSeparate := strings.Contains(clause.text, "分别") || strings.Contains(clause.text, "各自") || strings.Contains(clause.text, "比较") || strings.Contains(clause.text, "哪个更多")
		if hasOr && hasUnspecifiedAnd && !explicitAll {
			return nil, contractError(ErrorInvalidCombination, "scope", "同维度混合条件的关系不明确")
		}
		relation := crossScopeRelation{dimension: dimension, mentions: mentions}
		switch {
		case !hasOr && !hasExplicitAnd && !explicitAll && explicitSeparate:
			relation.mode = "separate"
		case !hasOr && !hasExplicitAnd && !explicitAll:
			relation.mode = "ambiguous"
		default:
			relation.mode = "confirmed"
			if hasOr && (hasExplicitAnd || hasUnspecifiedAnd) && !strings.ContainsAny(clause.text, "(（") {
				return nil, contractError(ErrorInvalidCombination, "scope", "混合条件的括号范围不明确")
			}
			var err error
			relation.clauses, err = crossExpressionClauses(clause.text, mentions, edges)
			if err != nil {
				return nil, err
			}
		}
		relations = append(relations, relation)
	}
	return relations, nil
}

func validateCrossScopeQuery(query CrossStatisticsQuery, clause crossScopeClause) error {
	if clause.uncertain || query.GroupBy != clause.groupBy {
		return contractError(ErrorInvalidCombination, "scope", "分组维度或用户范围无法确认")
	}
	relations, err := crossScopeRelations(clause)
	if err != nil {
		return err
	}
	positive, negative := make(map[string]bool), make(map[string]bool)
	for _, mention := range clause.mentions {
		key := crossScopeMentionKey(mention.dimension, mention.value, mention.category)
		if mention.excluded {
			negative[key] = true
		} else {
			positive[key] = true
		}
	}
	seenPositive, seenNegative := make(map[string]bool), make(map[string]bool)
	for _, entry := range []struct {
		groups   []CrossConditionGroup
		allowed  map[string]bool
		observed map[string]bool
		repeatOK bool
	}{{query.Conditions, positive, seenPositive, true}, {query.Exclusions, negative, seenNegative, false}} {
		for _, group := range entry.groups {
			for _, value := range group.Values {
				key := crossScopeMentionKey(group.Dimension, value, false)
				if !entry.allowed[key] {
					return contractError(ErrorInvalidValue, "scope", "模型添加了用户未确认的正式值")
				}
				if entry.observed[key] && !entry.repeatOK {
					return contractError(ErrorInvalidCombination, "scope", "模型重复使用同一条件改变了组合关系")
				}
				entry.observed[key] = true
			}
			for _, category := range group.Categories {
				key := crossScopeMentionKey(group.Dimension, category, true)
				if !entry.allowed[key] {
					return contractError(ErrorInvalidValue, "scope", "模型添加了用户未确认的类别")
				}
				if entry.observed[key] && !entry.repeatOK {
					return contractError(ErrorInvalidCombination, "scope", "模型重复使用同一类别改变了组合关系")
				}
				entry.observed[key] = true
			}
		}
	}
	for key := range negative {
		if !seenNegative[key] {
			return contractError(ErrorMissingField, "scope", "模型遗漏了用户排除条件")
		}
	}
	if len(negative) > 1 {
		// Multi-value exclusions need an explicit conjunction. In particular,
		// NOT (A AND B) must not be replaced by (NOT A) AND (NOT B), or vice
		// versa, because the two exclude different sets of jobs.
		exclusionText := clause.text
		for _, marker := range []string{"排除", "不包括", "不包含", "不含", "不看", "去掉", "除去"} {
			if start := strings.Index(exclusionText, marker); start >= 0 {
				exclusionText = exclusionText[start:]
				break
			}
		}
		explicitAll := strings.Contains(exclusionText, "同时") || strings.Contains(exclusionText, "交集")
		explicitAny := strings.Contains(exclusionText, "或") || strings.Contains(exclusionText, "任一")
		if !explicitAll && !explicitAny {
			return contractError(ErrorInvalidCombination, "scope", "多个排除条件的组合关系不明确")
		}
		if explicitAll && (len(query.Exclusions) != 1 || query.Exclusions[0].Operator != CrossAll) {
			return contractError(ErrorInvalidCombination, "scope", "排除条件的交集被改变")
		}
		if explicitAny {
			for _, group := range query.Exclusions {
				if group.Operator == CrossAll && len(group.Values)+len(group.Categories) > 1 {
					return contractError(ErrorInvalidCombination, "scope", "排除条件的并集被改变")
				}
			}
		}
	}
	for key := range positive {
		if seenPositive[key] {
			continue
		}
		if crossKeyBelongsToUnresolvedRelation(key, relations) {
			continue
		}
		return contractError(ErrorMissingField, "scope", "模型遗漏了用户条件")
	}
	for _, relation := range relations {
		kind := crossRelationQueryKind(query, relation)
		if relation.mode == "confirmed" && kind != "confirmed" || relation.mode == "separate" && !strings.HasPrefix(kind, "single:") || relation.mode == "ambiguous" && kind == "" {
			return contractError(ErrorInvalidCombination, "scope", "条件并集或交集与用户要求不一致")
		}
	}
	primaryOnly := strings.Contains(clause.text, "仅主岗位族") || strings.Contains(clause.text, "只看主岗位族") || strings.Contains(clause.text, "只统计主岗位族")
	if query.IncludeSecondary != nil && !*query.IncludeSecondary != primaryOnly || query.IncludeSecondary == nil && primaryOnly {
		return contractError(ErrorInvalidCombination, "include_secondary", "岗位族主副口径与用户要求不一致")
	}
	if crossHistoricalQuestion(clause.text) != (query.Time == CrossTimeHistoricalReference) {
		return contractError(ErrorInvalidCombination, "time", "当前参考数据的时间口径不一致")
	}
	return nil
}

func crossHistoricalQuestion(text string) bool {
	if crossHistoricalDate.MatchString(text) {
		return true
	}
	for _, marker := range []string{"上个月", "上月", "本月", "这个月", "去年", "今年", "上周", "本周", "上季度", "本季度", "上半年", "下半年", "历史", "过去", "最近", "近一", "近三", "近六", "近十", "近1", "近3", "近6", "近12", "去年", "前天", "昨天"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func crossScopeMentionKey(dimension Dimension, value string, category bool) string {
	if category {
		return string(dimension) + ":category:" + value
	}
	return string(dimension) + ":value:" + value
}

func crossKeyBelongsToUnresolvedRelation(key string, relations []crossScopeRelation) bool {
	for _, relation := range relations {
		if relation.mode != "ambiguous" && relation.mode != "separate" {
			continue
		}
		for _, mention := range relation.mentions {
			if crossScopeMentionKey(mention.dimension, mention.value, mention.category) == key {
				return true
			}
		}
	}
	return false
}

func crossRelationQueryKind(query CrossStatisticsQuery, relation crossScopeRelation) string {
	var clauses [][]string
	var included []string
	for _, group := range query.Conditions {
		if group.Dimension != relation.dimension {
			continue
		}
		keys := make([]string, 0, len(group.Values)+len(group.Categories))
		for _, value := range group.Values {
			keys = append(keys, crossScopeMentionKey(group.Dimension, value, false))
		}
		for _, category := range group.Categories {
			keys = append(keys, crossScopeMentionKey(group.Dimension, category, true))
		}
		included = append(included, keys...)
		if group.Operator == CrossAll {
			for _, key := range keys {
				clauses = append(clauses, []string{key})
			}
		} else {
			clauses = append(clauses, keys)
		}
	}
	if relation.mode == "confirmed" {
		if canonicalCrossClauses(clauses) == canonicalCrossClauses(relation.clauses) {
			return "confirmed"
		}
		return ""
	}
	if len(included) == 1 {
		return "single:" + included[0]
	}
	if len(included) != len(relation.mentions) {
		return ""
	}
	allKeys := make([]string, 0, len(relation.mentions))
	allClauses := make([][]string, 0, len(relation.mentions))
	for _, mention := range relation.mentions {
		key := crossScopeMentionKey(mention.dimension, mention.value, mention.category)
		allKeys = append(allKeys, key)
		allClauses = append(allClauses, []string{key})
	}
	if canonicalCrossClauses(clauses) == canonicalCrossClauses([][]string{allKeys}) {
		return "any"
	}
	if canonicalCrossClauses(clauses) == canonicalCrossClauses(allClauses) {
		return "all"
	}
	return ""
}

func canonicalCrossClauses(clauses [][]string) string {
	result := make([]string, 0, len(clauses))
	for _, clause := range clauses {
		values := append([]string(nil), clause...)
		sort.Strings(values)
		result = append(result, strings.Join(values, "\x00"))
	}
	sort.Strings(result)
	return strings.Join(result, "\x01")
}
