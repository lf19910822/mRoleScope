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
	Values  map[Dimension][]Candidate
	Aliases map[Dimension]map[string][]string
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
		aliases := resolver.Aliases[dimension][needle]
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
		return boundCandidates(candidates)
	}
	return nil
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
		if candidate.MatchStatus == "" {
			candidate.MatchStatus = MatchPossible
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
		if candidate.SubQueryID != "" {
			candidate.SubQueryIDs = append(candidate.SubQueryIDs, candidate.SubQueryID)
		}
		seen[key] = struct{}{}
		merged = append(merged, candidate)
	}
	return merged
}

func mergeCandidateMetadata(left, right Candidate) Candidate {
	ids := append([]string(nil), left.SubQueryIDs...)
	if left.SubQueryID != "" && len(ids) == 0 {
		ids = append(ids, left.SubQueryID)
	}
	for _, id := range append(right.SubQueryIDs, right.SubQueryID) {
		if id == "" {
			continue
		}
		found := false
		for _, existing := range ids {
			if existing == id {
				found = true
				break
			}
		}
		if !found {
			ids = append(ids, id)
		}
	}
	left.SubQueryIDs = ids
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
