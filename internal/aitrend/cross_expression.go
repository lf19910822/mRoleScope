package aitrend

import (
	"sort"
	"strings"
)

type crossExpressionToken struct {
	kind string
	key  string
}

// crossExpressionClauses converts one dimension's explicit boolean expression
// into AND-of-OR clauses, the restricted format accepted by Serving. This also
// handles (A AND B) OR C without changing its meaning.
func crossExpressionClauses(text string, mentions []crossScopeMention, edges []string) ([][]string, error) {
	stacks, err := crossParenthesisStacks(text, mentions)
	if err != nil {
		return nil, err
	}
	var tokens []crossExpressionToken
	for range stacks[0] {
		tokens = append(tokens, crossExpressionToken{kind: "("})
	}
	tokens = append(tokens, crossExpressionToken{kind: "value", key: crossScopeMentionKey(mentions[0].dimension, mentions[0].value, mentions[0].category)})
	for index, edge := range edges {
		previous, next := stacks[index], stacks[index+1]
		common := 0
		for common < len(previous) && common < len(next) && previous[common] == next[common] {
			common++
		}
		for closed := common; closed < len(previous); closed++ {
			tokens = append(tokens, crossExpressionToken{kind: ")"})
		}
		tokens = append(tokens, crossExpressionToken{kind: edge})
		for opened := common; opened < len(next); opened++ {
			tokens = append(tokens, crossExpressionToken{kind: "("})
		}
		mention := mentions[index+1]
		tokens = append(tokens, crossExpressionToken{kind: "value", key: crossScopeMentionKey(mention.dimension, mention.value, mention.category)})
	}
	for range stacks[len(stacks)-1] {
		tokens = append(tokens, crossExpressionToken{kind: ")"})
	}
	var operators []string
	var values [][][]string
	priority := func(operator string) int {
		if operator == "and" {
			return 2
		}
		if operator == "or" {
			return 1
		}
		return 0
	}
	apply := func() error {
		if len(operators) == 0 || len(values) < 2 {
			return contractError(ErrorInvalidCombination, "scope", "条件组合格式不明确")
		}
		operator := operators[len(operators)-1]
		operators = operators[:len(operators)-1]
		left, right := values[len(values)-2], values[len(values)-1]
		values = values[:len(values)-2]
		var combined [][]string
		if operator == "and" {
			combined = append(append([][]string(nil), left...), right...)
		} else if operator == "or" {
			if len(left)*len(right) > MaxCrossConditionGroups {
				return contractError(ErrorLimitExceeded, "scope", "条件组合超出单次统计范围")
			}
			for _, leftClause := range left {
				for _, rightClause := range right {
					merged := append(append([]string(nil), leftClause...), rightClause...)
					combined = append(combined, uniqueCrossKeys(merged))
				}
			}
		} else {
			return contractError(ErrorInvalidCombination, "scope", "条件组合格式不明确")
		}
		combined = uniqueCrossClauses(combined)
		if len(combined) > MaxCrossConditionGroups {
			return contractError(ErrorLimitExceeded, "scope", "条件组合超出单次统计范围")
		}
		values = append(values, combined)
		return nil
	}
	for _, token := range tokens {
		switch token.kind {
		case "value":
			values = append(values, [][]string{{token.key}})
		case "(":
			operators = append(operators, token.kind)
		case ")":
			for len(operators) > 0 && operators[len(operators)-1] != "(" {
				if err := apply(); err != nil {
					return nil, err
				}
			}
			if len(operators) == 0 {
				return nil, contractError(ErrorInvalidCombination, "scope", "条件括号不匹配")
			}
			operators = operators[:len(operators)-1]
		case "and", "or":
			for len(operators) > 0 && priority(operators[len(operators)-1]) >= priority(token.kind) {
				if err := apply(); err != nil {
					return nil, err
				}
			}
			operators = append(operators, token.kind)
		}
	}
	for len(operators) > 0 {
		if operators[len(operators)-1] == "(" {
			return nil, contractError(ErrorInvalidCombination, "scope", "条件括号不匹配")
		}
		if err := apply(); err != nil {
			return nil, err
		}
	}
	if len(values) != 1 {
		return nil, contractError(ErrorInvalidCombination, "scope", "条件组合格式不明确")
	}
	return values[0], nil
}

func crossParenthesisStacks(text string, mentions []crossScopeMention) ([][]int, error) {
	stack := make([]int, 0)
	nextID, cursor := 0, 0
	process := func(segment string) error {
		for _, character := range segment {
			switch character {
			case '(', '（':
				nextID++
				stack = append(stack, nextID)
			case ')', '）':
				if len(stack) == 0 {
					return contractError(ErrorInvalidCombination, "scope", "条件括号不匹配")
				}
				stack = stack[:len(stack)-1]
			}
		}
		return nil
	}
	stacks := make([][]int, 0, len(mentions))
	for _, mention := range mentions {
		if err := process(text[cursor:mention.start]); err != nil {
			return nil, err
		}
		stacks = append(stacks, append([]int(nil), stack...))
		cursor = mention.end
	}
	if err := process(text[cursor:]); err != nil || len(stack) != 0 {
		return nil, contractError(ErrorInvalidCombination, "scope", "条件括号不匹配")
	}
	return stacks, nil
}

func uniqueCrossKeys(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func uniqueCrossClauses(clauses [][]string) [][]string {
	seen := make(map[string]bool, len(clauses))
	result := make([][]string, 0, len(clauses))
	for _, clause := range clauses {
		clause = uniqueCrossKeys(clause)
		key := strings.Join(clause, "\x00")
		if !seen[key] {
			seen[key] = true
			result = append(result, clause)
		}
	}
	return result
}
