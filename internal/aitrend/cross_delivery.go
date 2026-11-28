package aitrend

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// emitCrossResults renders only validated server-side aggregate facts. Each
// delta is bounded, so long rankings can be streamed without the chart limit.
func (runner *TrendRunner) emitCrossResults(ctx context.Context, records []crossStatisticsRecord, snapshot *ServingSnapshot, chartAvailable bool, unavailable string, emit func(StreamEvent) error, usage ...func(int, int)) error {
	if len(records) == 0 {
		message := unavailable
		if message == "" {
			message = "当前交叉数量本次无法计算，不能将其当作零。"
		}
		if chartAvailable {
			message += "图表仍是各条件的独立趋势曲线，不表示交集或并集。"
		}
		if err := emitCrossText(ctx, message, emit); err != nil {
			return err
		}
		return emit(StreamEvent{Type: StreamEventAssistant, Status: StreamCompleted})
	}
	var insights []crossInsight
	if provider, ok := runner.Provider.(CrossInsightProvider); ok {
		facts, err := crossModelPayload(records)
		if err == nil {
			if err := emit(StreamEvent{Type: StreamEventOperation, Status: StreamRunning, Tool: string(ToolExplainDraft)}); err != nil {
				return err
			}
			insightContext, cancel := context.WithTimeout(ctx, 15*time.Second)
			content, insightInput, insightOutput, insightErr := provider.SelectCrossInsights(insightContext, facts)
			cancel()
			if len(usage) > 0 && usage[0] != nil {
				usage[0](insightInput, insightOutput)
			}
			if insightErr == nil {
				insights, insightErr = decodeCrossInsights(content, records)
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			status := StreamCompleted
			if insightErr != nil {
				status = StreamFailed
			}
			if err := emit(StreamEvent{Type: StreamEventOperation, Status: status, Tool: string(ToolExplainDraft)}); err != nil {
				return err
			}
		}
	}
	hasTechnology, hasFamily := false, false
	for _, record := range records {
		query := record.query
		recordHasFamily := query.GroupBy == DimensionJobFamily
		for _, group := range append(append([]CrossConditionGroup(nil), query.Conditions...), query.Exclusions...) {
			recordHasFamily = recordHasFamily || group.Dimension == DimensionJobFamily
		}
		if query.Time == CrossTimeHistoricalReference {
			if err := emitCrossText(ctx, "历史交叉数量暂不可用，以下为当前有效岗位的参考统计。\n", emit); err != nil {
				return err
			}
		}
		description := crossQueryDescription(query, snapshot)
		if err := emitCrossText(ctx, fmt.Sprintf("当前有效岗位中，%s：%d 个岗位（已去重）。\n", description, record.result.Total), emit); err != nil {
			return err
		}
		if recordHasFamily {
			familyScope := "岗位族口径：包含主、副岗位族。\n"
			if query.IncludeSecondary != nil && !*query.IncludeSecondary {
				familyScope = "岗位族口径：仅主岗位族。\n"
			}
			if err := emitCrossText(ctx, familyScope, emit); err != nil {
				return err
			}
		}
		for _, group := range append(append([]CrossConditionGroup(nil), query.Conditions...), query.Exclusions...) {
			hasTechnology = hasTechnology || group.Dimension == DimensionTechnology
			hasFamily = hasFamily || group.Dimension == DimensionJobFamily
		}
		hasTechnology = hasTechnology || query.GroupBy == DimensionTechnology
		hasFamily = hasFamily || query.GroupBy == DimensionJobFamily
		if query.GroupBy == "" {
			continue
		}
		pager, err := NewCrossStatisticsPager(record.result, query.GroupBy, query.RankingLimit, query.PageSize)
		if err != nil {
			return err
		}
		first := true
		for {
			page, err := pager.Next(ctx)
			if err != nil {
				return err
			}
			if first {
				if err := emitCrossText(ctx, fmt.Sprintf("%s排名：共 %d 项，计划展示 %d 项。\n", crossDimensionLabel(query.GroupBy), page.TotalGroups, page.RequestedCount), emit); err != nil {
					return err
				}
				first = false
			}
			for index, group := range page.Groups {
				if err := emitCrossText(ctx, fmt.Sprintf("%d. %s：%d 个\n", page.Offset+index+1, group.Label, group.Count), emit); err != nil {
					return err
				}
			}
			if !page.HasMore {
				if err := emitCrossText(ctx, fmt.Sprintf("已展示 %d / %d 项。\n", page.Shown, page.TotalGroups), emit); err != nil {
					return err
				}
				break
			}
		}
		if err := emitCrossText(ctx, "不同分组可能包含同一岗位，分组数量之和不一定等于去重总数。\n", emit); err != nil {
			return err
		}
	}
	for _, insight := range insights {
		if text, ok := crossInsightText(insight, records, snapshot); ok {
			if err := emitCrossText(ctx, text, emit); err != nil {
				return err
			}
		}
	}
	if hasTechnology {
		if err := emitCrossText(ctx, "技术指岗位内容中识别到的关键词，不代表实际使用或必须具备。\n", emit); err != nil {
			return err
		}
	}
	if chartAvailable {
		if err := emitCrossText(ctx, "图表是各条件的独立趋势曲线，不表示交集或并集；当前统计与历史曲线节点的口径可能不同。", emit); err != nil {
			return err
		}
		if hasFamily {
			if err := emitCrossText(ctx, "岗位族曲线使用曲线自身的主、副岗位族设置。", emit); err != nil {
				return err
			}
		}
	}
	return emit(StreamEvent{Type: StreamEventAssistant, Status: StreamCompleted})
}

func crossUnavailableReason(session *crossStatisticsSession) string {
	if session == nil {
		return ""
	}
	for _, clause := range session.scope.clauses {
		relations, err := crossScopeRelations(clause)
		if err != nil {
			if contractErr, ok := err.(*ContractError); ok {
				if strings.Contains(contractErr.Message, "跨维度并集") {
					return "跨维度并集统计本期暂不支持。"
				}
				if contractErr.Code == ErrorLimitExceeded {
					return "所需条件组合超出本次统计范围，无法完整计算。"
				}
				return "条件之间的关系无法确认，因此不能计算确定数量。"
			}
		}
		combinations := 1
		for _, relation := range relations {
			count := len(relation.mentions)
			if relation.mode == "ambiguous" {
				count += 2
			} else if relation.mode != "separate" {
				continue
			}
			if combinations > MaxCrossSubQueries/count {
				return "所需条件组合超出本次统计范围，无法完整计算。"
			}
			combinations *= count
		}
		if clause.uncertain {
			return "本次统计条件无法确认，因此不能计算确定数量。"
		}
	}
	return ""
}

func emitCrossChartUnavailable(ctx context.Context, emit func(StreamEvent) error) error {
	return emitCrossText(ctx, "本次未能生成符合范围的曲线草稿。\n", emit)
}

func emitCrossText(ctx context.Context, text string, emit func(StreamEvent) error) error {
	runes := []rune(text)
	for len(runes) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		length := len(runes)
		if length > 800 {
			length = 800
		}
		if err := emit(StreamEvent{Type: StreamEventAssistant, Status: StreamRunning, Delta: string(runes[:length])}); err != nil {
			return err
		}
		runes = runes[length:]
	}
	return nil
}

func crossQueryDescription(query CrossStatisticsQuery, snapshot *ServingSnapshot) string {
	parts := make([]string, 0, len(query.Conditions)+len(query.Exclusions))
	for _, group := range query.Conditions {
		parts = append(parts, crossConditionDescription(group, snapshot))
	}
	for _, group := range query.Exclusions {
		parts = append(parts, "排除"+crossConditionDescription(group, snapshot))
	}
	if len(parts) == 0 {
		return "全部当前范围"
	}
	return strings.Join(parts, "且")
}

func crossConditionDescription(group CrossConditionGroup, snapshot *ServingSnapshot) string {
	labels := make(map[string]string)
	if snapshot != nil {
		for _, option := range (ServingToolService{}).optionsForDimension(snapshot.options, group.Dimension) {
			labels[option.Value] = option.Label
		}
		for _, category := range snapshot.categories {
			labels[category.Value] = category.Label
		}
	}
	values := make([]string, 0, len(group.Values)+len(group.Categories))
	for _, value := range append(append([]string(nil), group.Values...), group.Categories...) {
		label := labels[value]
		if label == "" {
			label = value
		}
		values = append(values, label)
	}
	joiner := "或"
	if group.Operator == CrossAll {
		joiner = "且"
	}
	return crossDimensionLabel(group.Dimension) + "（" + strings.Join(values, joiner) + "）"
}

func crossDimensionLabel(dimension Dimension) string {
	switch dimension {
	case DimensionTechnology:
		return "技术关键词"
	case DimensionProject:
		return "项目"
	case DimensionJobFamily:
		return "岗位族"
	case DimensionLocation:
		return "地点"
	default:
		return "分组"
	}
}
