package aitrend

import (
	"fmt"
	"strings"
)

// validateTrendCategoryScope also protects category-only trend questions,
// which may not require a cross-statistics call at all.
func validateTrendCategoryScope(intent Intent, groups []CandidateGroup, session *crossStatisticsSession, snapshot *ServingSnapshot) error {
	if session == nil || snapshot == nil {
		return nil
	}
	if len(session.scope.clauses) != 1 {
		for _, clause := range session.scope.clauses {
			for _, mention := range clause.mentions {
				if mention.category {
					return contractError(ErrorInvalidCombination, "scope", "无法确认多个子问题的类别归属")
				}
			}
		}
		return nil
	}
	clause := session.scope.clauses[0]
	if clause.uncertain {
		return contractError(ErrorInvalidCombination, "scope", "用户范围或正式映射不明确")
	}
	for _, query := range intent.SubQueries {
		for _, expression := range query.Expressions {
			if !isBroadExpression(expression) {
				continue
			}
			for _, mention := range clause.mentions {
				if mention.dimension == query.Dimension {
					return contractError(ErrorInvalidCombination, "scope", "用户指定的范围不能改成全局查询")
				}
			}
			if !crossExplicitWholeDimension(clause.text, query.Dimension) {
				return contractError(ErrorInvalidCombination, "scope", "无法确认用户要求整个维度")
			}
		}
	}
	positive, negative := make(map[string]bool), make(map[string]bool)
	for _, mention := range clause.mentions {
		if !mention.category {
			continue
		}
		values, _, ok := snapshot.categoryValues(mention.value)
		if !ok || len(values) == 0 {
			return contractError(ErrorUnavailableValue, "scope", "无法确认正式类别及其候选")
		}
		target := positive
		if mention.excluded {
			target = negative
		}
		for _, value := range values {
			target[value.Value] = true
		}
	}
	if len(positive) > 0 && clause.groupBy == DimensionTechnology {
		for _, query := range intent.SubQueries {
			if query.Dimension != DimensionTechnology {
				return contractError(ErrorInvalidCombination, "scope", "技术类别问题不能改为其他维度排名")
			}
		}
	}
	for _, group := range groups {
		for _, candidate := range group.Candidates {
			if candidate.Dimension != DimensionTechnology {
				continue
			}
			if negative[candidate.Value] || len(positive) > 0 && clause.groupBy == DimensionTechnology && !positive[candidate.Value] {
				return contractError(ErrorInvalidCombination, "scope", "趋势候选超出用户确认的类别范围")
			}
		}
	}
	return nil
}

func crossExplicitWholeDimension(text string, dimension Dimension) bool {
	text = strings.TrimSpace(text)
	for _, prefix := range []string{"请问", "请", "目前", "现在", "当前", "帮我看看", "帮我看", "看看"} {
		text = strings.TrimPrefix(text, prefix)
		text = strings.TrimSpace(text)
	}
	var starts []string
	switch dimension {
	case DimensionTechnology:
		starts = []string{"哪些技术", "有哪些技术", "所有技术", "全部技术", "技术关键词", "哪个技术", "使用最多的技术", "岗位最多的技术"}
	case DimensionProject:
		starts = []string{"哪些项目", "有哪些项目", "所有项目", "全部项目", "项目排名", "哪个项目", "岗位最多的项目"}
	case DimensionJobFamily:
		starts = []string{"哪些岗位族", "有哪些岗位族", "所有岗位族", "全部岗位族", "岗位族排名", "哪个岗位族"}
	case DimensionLocation:
		starts = []string{"哪些地点", "哪些城市", "所有地点", "全部地点", "所有城市", "全部城市", "地点排名", "哪个地点", "哪个城市"}
	}
	for _, start := range starts {
		if strings.HasPrefix(text, start) {
			return true
		}
	}
	return false
}

// includeCrossChartConditions supplies any clearly named positive conditions
// omitted by the model. Each remains an independent single-dimension curve.
func includeCrossChartConditions(intent Intent, session *crossStatisticsSession, snapshot *ServingSnapshot) Intent {
	if session == nil || snapshot == nil || len(session.scope.clauses) != 1 || session.scope.clauses[0].uncertain {
		return intent
	}
	intent.SubQueries = append([]SubQuery(nil), intent.SubQueries...)
	usedIDs := make(map[string]bool, len(intent.SubQueries))
	for _, query := range intent.SubQueries {
		usedIDs[query.ID] = true
	}
	for index, mention := range session.scope.clauses[0].mentions {
		if mention.excluded {
			continue
		}
		found := false
		for _, query := range intent.SubQueries {
			if query.Dimension != mention.dimension {
				continue
			}
			for _, expression := range query.Expressions {
				if normalizeExpression(expression) == normalizeExpression(mention.value) {
					found = true
				}
				if mention.category {
					_, formal, ok := snapshot.categoryValues(expression)
					if ok && formal == mention.value {
						found = true
					}
				}
			}
		}
		if found {
			continue
		}
		id := fmt.Sprintf("cross-condition-%d", index+1)
		for usedIDs[id] {
			id += "x"
		}
		usedIDs[id] = true
		intent.SubQueries = append(intent.SubQueries, SubQuery{
			ID: id, Dimension: mention.dimension, Expressions: []string{mention.value},
			Ranking: RankingIntent{Metric: MetricAverageDailyActiveJobs, Order: OrderDescending, Limit: DefaultRankingLimit},
		})
	}
	for index := range intent.SubQueries {
		query := &intent.SubQueries[index]
		query.Exclusions = append([]string(nil), query.Exclusions...)
		for _, mention := range session.scope.clauses[0].mentions {
			if !mention.excluded || mention.dimension != query.Dimension {
				continue
			}
			if mention.category {
				options, _, ok := snapshot.categoryValues(mention.value)
				if ok {
					for _, option := range options {
						query.Exclusions = append(query.Exclusions, option.Value)
					}
				}
			} else {
				query.Exclusions = append(query.Exclusions, mention.value)
			}
		}
	}
	return intent
}

// validateCrossDraftScope allows only independent curves supported by the
// user's explicit conditions or the verified grouped result. It deliberately
// refuses to guess how model-generated curve IDs map to multiple clauses.
func validateCrossDraftScope(draft Draft, session *crossStatisticsSession, snapshot *ServingSnapshot) error {
	if session == nil || snapshot == nil || len(session.scope.clauses) != 1 || session.scope.clauses[0].uncertain {
		return contractError(ErrorInvalidCombination, "curves", "无法确认曲线所属的用户范围")
	}
	allowed := make(map[string]bool)
	excluded := make(map[string]bool)
	for _, mention := range session.scope.clauses[0].mentions {
		target := allowed
		if mention.excluded {
			target = excluded
		}
		if mention.category {
			options, _, ok := snapshot.categoryValues(mention.value)
			if !ok || len(options) == 0 {
				return contractError(ErrorUnavailableValue, "curves", "无法确认曲线的正式类别")
			}
			for _, option := range options {
				target[crossScopeMentionKey(mention.dimension, option.Value, false)] = true
			}
		} else {
			target[crossScopeMentionKey(mention.dimension, mention.value, false)] = true
		}
	}
	for _, record := range session.records {
		for _, group := range record.result.Groups {
			allowed[crossScopeMentionKey(record.query.GroupBy, group.Value, false)] = true
		}
	}
	for _, curve := range draft.Curves {
		key := crossScopeMentionKey(curve.Condition.Dimension, curve.Condition.Value, false)
		if !allowed[key] || excluded[key] {
			return contractError(ErrorInvalidCombination, "curves", "曲线超出用户确认的范围")
		}
		for _, record := range session.records {
			if curve.Condition.Dimension == DimensionJobFamily && curve.Condition.IncludeSecondary && record.query.IncludeSecondary != nil && !*record.query.IncludeSecondary {
				return contractError(ErrorInvalidCombination, "curves", "岗位族曲线包含用户排除的副岗位族")
			}
		}
	}
	return nil
}
