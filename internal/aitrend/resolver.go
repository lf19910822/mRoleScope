package aitrend

import (
	"sort"
	"strings"
)

type Candidate struct {
	Dimension        Dimension
	Value            string
	Label            string
	MatchStatus      MatchStatus
	Score            int
	SubQueryID       string
	SubQueryIDs      []string
	Operator         *Comparison
	IncludeSecondary bool
	Scope            string
}

type Resolver struct {
	Values          map[Dimension][]Candidate
	Aliases         map[Dimension]map[string][]string
	CategoryAliases map[Dimension]map[string]struct{}
}

// Resolve performs deterministic, bounded matching over values supplied by
// the current Serving publication. It does not create values or query storage.
func (resolver Resolver) Resolve(dimension Dimension, expression, subQueryID string) []Candidate {
	return resolver.resolve(dimension, expression, subQueryID, false)
}

// ResolveNatural widens an expression to known vocabulary aliases only when
// an exact/fuzzy formal match is unavailable. Expanded rows remain marked so
// the draft can explain why they were shown.
func (resolver Resolver) ResolveNatural(expression, subQueryID string) []Candidate {
	for _, dimension := range []Dimension{DimensionTechnology, DimensionProject, DimensionJobFamily, DimensionLocation} {
		if direct := resolver.Resolve(dimension, expression, subQueryID); len(direct) > 0 {
			for index := range direct {
				direct[index].MatchStatus = MatchPossible
			}
			return direct
		}
	}
	needle := normalizeExpression(expression)
	for _, dimension := range []Dimension{DimensionTechnology, DimensionProject, DimensionJobFamily, DimensionLocation} {
		categoryAlias := false
		aliases := resolver.Aliases[dimension][needle]
		if _, ok := resolver.CategoryAliases[dimension][needle]; ok {
			categoryAlias = true
		}
		if len(aliases) == 0 {
			for alias, values := range resolver.Aliases[dimension] {
				if !categoryAliasMatchesExpression(needle, alias) {
					continue
				}
				aliases = append(aliases, values...)
				if _, ok := resolver.CategoryAliases[dimension][alias]; ok {
					categoryAlias = true
				}
			}
		}
		if len(aliases) == 0 {
			aliases = defaultAliasValues(dimension, needle)
		}
		if len(aliases) == 0 {
			continue
		}
		candidates := make([]Candidate, 0)
		for _, alias := range aliases {
			for _, candidate := range resolver.Values[dimension] {
				if normalizeExpression(candidate.Value) != normalizeExpression(alias) && normalizeExpression(candidate.Label) != normalizeExpression(alias) {
					continue
				}
				candidate.SubQueryID = subQueryID
				candidate.MatchStatus = MatchExpansion
				candidate.Score = 40
				candidates = append(candidates, candidate)
			}
		}
		candidates = MergeCandidates(candidates)
		if categoryAlias {
			return candidates
		}
		return boundCandidates(candidates)
	}
	return nil
}

// ResolveCategory preserves every formally published member until the
// summary/budget stage. It prefers the longest recognized category alias so a
// shorter overlapping category name cannot broaden the requested range.
func (resolver Resolver) ResolveCategory(dimension Dimension, expression, subQueryID string) ([]Candidate, bool) {
	needle := normalizeExpression(expression)
	bestLength := 0
	var matchedAliases []string
	for alias := range resolver.CategoryAliases[dimension] {
		if !categoryAliasMatchesExpression(needle, alias) {
			continue
		}
		length := len([]rune(alias))
		if length > bestLength {
			bestLength = length
			matchedAliases = matchedAliases[:0]
		}
		if length == bestLength {
			matchedAliases = append(matchedAliases, alias)
		}
	}
	if bestLength == 0 {
		return nil, false
	}
	keys := make(map[string]bool)
	for _, alias := range matchedAliases {
		for _, value := range resolver.Aliases[dimension][alias] {
			keys[normalizeExpression(value)] = true
		}
	}
	candidates := make([]Candidate, 0, len(keys))
	for _, value := range resolver.Values[dimension] {
		if !keys[normalizeExpression(value.Value)] && !keys[normalizeExpression(value.Label)] {
			continue
		}
		value.SubQueryID = subQueryID
		value.MatchStatus = MatchExpansion
		value.Score = 40
		candidates = append(candidates, value)
	}
	return MergeCandidates(candidates), true
}

func categoryAliasMatchesExpression(expression, alias string) bool {
	if alias == "" || expression == alias {
		return alias != ""
	}
	for _, prefix := range []string{"哪些", "所有", "全部", "使用哪些", "使用了哪些", "哪一个", "哪个", "主要"} {
		if strings.HasPrefix(expression, prefix) && strings.Contains(expression[len(prefix):], alias) {
			return true
		}
	}
	return false
}

// ResolveAcrossDimensions preserves ambiguity when a phrase could name more
// than one formal dimension. The caller can label each row as possible.
func (resolver Resolver) ResolveAcrossDimensions(expression, subQueryID string) []Candidate {
	result := make([]Candidate, 0)
	for _, dimension := range []Dimension{DimensionTechnology, DimensionProject, DimensionJobFamily, DimensionLocation} {
		result = append(result, resolver.Resolve(dimension, expression, subQueryID)...)
	}
	return MergeCandidates(result)
}

func (resolver Resolver) resolve(dimension Dimension, expression, subQueryID string, includeAll bool) []Candidate {
	needle := normalizeExpression(expression)
	if needle == "" {
		return nil
	}
	values := resolver.Values[dimension]
	matched := make([]Candidate, 0)
	for _, value := range values {
		candidate := value
		candidate.SubQueryID = subQueryID
		candidate.Score = candidateScore(needle, normalizeExpression(value.Value), normalizeExpression(value.Label))
		if candidate.Score == 0 {
			continue
		}
		if candidate.MatchStatus == "" || candidate.MatchStatus == MatchExact {
			if candidate.Score == 100 {
				candidate.MatchStatus = MatchExact
			} else {
				candidate.MatchStatus = MatchPossible
			}
		}
		matched = append(matched, candidate)
	}
	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].MatchStatus != matched[j].MatchStatus {
			return matchPriority(matched[i].MatchStatus) < matchPriority(matched[j].MatchStatus)
		}
		if matched[i].Score != matched[j].Score {
			return matched[i].Score > matched[j].Score
		}
		return matched[i].Value < matched[j].Value
	})
	if !includeAll && len(matched) > MaxExpressionCandidates {
		matched = matched[:MaxExpressionCandidates]
	}
	return matched
}

func (resolver Resolver) ResolveWithExclusions(dimension Dimension, expression string, exclusions []string, subQueryID string) []Candidate {
	blocked := make(map[string]struct{}, len(exclusions))
	for _, exclusion := range exclusions {
		blocked[normalizeExpression(exclusion)] = struct{}{}
	}
	resolved := resolver.resolve(dimension, expression, subQueryID, true)
	// A formal value or label is an exact scope boundary. Do not add fuzzy
	// substring matches such as "go" for the formal value "mongodb" when the
	// model has already selected a concrete candidate.
	exact := resolved[:0]
	for _, candidate := range resolved {
		if candidate.Score == 100 {
			exact = append(exact, candidate)
		}
	}
	if len(exact) > 0 {
		resolved = exact
	}
	filtered := resolved[:0]
	for _, candidate := range resolved {
		if _, excluded := blocked[normalizeExpression(candidate.Value)]; excluded {
			continue
		}
		if _, excluded := blocked[normalizeExpression(candidate.Label)]; excluded {
			continue
		}
		filtered = append(filtered, candidate)
	}
	return boundCandidates(filtered)
}

func boundCandidates(candidates []Candidate) []Candidate {
	if len(candidates) > MaxExpressionCandidates {
		return candidates[:MaxExpressionCandidates]
	}
	return candidates
}

func defaultAliasValues(dimension Dimension, expression string) []string {
	switch dimension {
	case DimensionTechnology:
		switch expression {
		case "数据库", "数据库技术", "database", "databases", "storage":
			return []string{"mysql", "postgresql", "redis", "mongodb"}
		case "前端", "frontend", "frontend framework":
			return []string{"vue", "react", "angular"}
		}
	case DimensionProject:
		if expression == "项目" || expression == "project" || expression == "projects" {
			return nil
		}
	case DimensionJobFamily:
		if expression == "岗位族" || expression == "job family" || expression == "job families" {
			return nil
		}
	case DimensionLocation:
		if expression == "地点" || expression == "工作地点" || expression == "location" {
			return nil
		}
	}
	return nil
}

func MergeCandidates(candidates []Candidate) []Candidate {
	seen := make(map[string]struct{}, len(candidates))
	merged := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		key := candidateIdentity(candidate)
		if _, exists := seen[key]; exists {
			for index := range merged {
				if candidateIdentity(merged[index]) != key {
					continue
				}
				merged[index] = mergeCandidateMetadata(merged[index], candidate)
				break
			}
			continue
		}
		candidate.SubQueryIDs = uniqueSubQueryIDs(candidate.SubQueryIDs, []string{candidate.SubQueryID})
		seen[key] = struct{}{}
		merged = append(merged, candidate)
	}
	return merged
}

func mergeCandidateMetadata(left, right Candidate) Candidate {
	left.SubQueryIDs = uniqueSubQueryIDs(left.SubQueryIDs, []string{left.SubQueryID}, right.SubQueryIDs, []string{right.SubQueryID})
	if left.MatchStatus == MatchPossible && right.MatchStatus != MatchPossible {
		left.MatchStatus = right.MatchStatus
	}
	if left.Score < right.Score {
		left.Score = right.Score
	}
	if left.Scope == "" {
		left.Scope = right.Scope
	}
	return left
}

func uniqueSubQueryIDs(groups ...[]string) []string {
	ids := make([]string, 0)
	seen := make(map[string]struct{})
	for _, group := range groups {
		for _, id := range group {
			if id == "" {
				continue
			}
			if _, exists := seen[id]; !exists {
				seen[id] = struct{}{}
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// AllocateCandidates reserves one candidate for each sub-query in order,
// then fills remaining slots by score and stable input order.
func AllocateCandidates(groups [][]Candidate) []Candidate {
	if len(groups) == 0 {
		return nil
	}
	selected := make([]Candidate, 0, MaxCurves)
	used := make(map[string]struct{})
	for _, group := range groups {
		for _, candidate := range group {
			key := candidateIdentity(candidate)
			if _, exists := used[key]; exists {
				continue
			}
			selected = append(selected, candidate)
			used[key] = struct{}{}
			break
		}
		if len(selected) == MaxCurves {
			return selected
		}
	}
	remaining := make([]Candidate, 0)
	for _, group := range groups {
		for _, candidate := range group {
			if _, exists := used[candidateIdentity(candidate)]; !exists {
				remaining = append(remaining, candidate)
			}
		}
	}
	sort.SliceStable(remaining, func(i, j int) bool {
		if remaining[i].Score != remaining[j].Score {
			return remaining[i].Score > remaining[j].Score
		}
		return remaining[i].Value < remaining[j].Value
	})
	for _, candidate := range remaining {
		if len(selected) == MaxCurves {
			break
		}
		key := candidateIdentity(candidate)
		if _, exists := used[key]; exists {
			continue
		}
		selected = append(selected, candidate)
		used[key] = struct{}{}
	}
	return selected
}

func normalizeExpression(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func candidateScore(needle, value, label string) int {
	if needle == value || needle == label {
		return 100
	}
	if strings.HasPrefix(value, needle) || strings.HasPrefix(label, needle) {
		return 80
	}
	if strings.Contains(value, needle) || strings.Contains(label, needle) {
		return 60
	}
	if strings.Contains(needle, value) || strings.Contains(needle, label) {
		return 40
	}
	return 0
}

func matchPriority(status MatchStatus) int {
	switch status {
	case MatchExact:
		return 0
	case MatchAlias:
		return 1
	case MatchPossible:
		return 2
	case MatchExpansion:
		return 3
	default:
		return 4
	}
}

func candidateIdentity(candidate Candidate) string {
	operator := ""
	if candidate.Operator != nil {
		operator = string(*candidate.Operator)
	}
	return string(candidate.Dimension) + "|" + candidate.Value + "|" + operator + "|" + boolString(candidate.IncludeSecondary)
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
