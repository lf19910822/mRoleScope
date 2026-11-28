package aitrend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// CrossInsightProvider can select bounded explanations over verified facts.
// It never returns free text to the user or supplies numeric values itself.
type CrossInsightProvider interface {
	SelectCrossInsights(context.Context, json.RawMessage) (json.RawMessage, int, int, error)
}

type crossInsight struct {
	Kind            string `json:"kind"`
	SubQueryID      string `json:"sub_query_id"`
	OtherSubQueryID string `json:"other_sub_query_id,omitempty"`
}

type crossInsightPlan struct {
	Insights []crossInsight `json:"insights"`
}

func decodeCrossInsights(data []byte, records []crossStatisticsRecord) ([]crossInsight, error) {
	var plan crossInsightPlan
	if err := decodeStrict(data, &plan); err != nil || len(plan.Insights) > MaxCrossSubQueries {
		return nil, contractError(ErrorInvalidJSON, "insights", "统计解释格式无效")
	}
	byID := make(map[string]crossStatisticsRecord, len(records))
	for _, record := range records {
		byID[record.query.ID] = record
	}
	seen := make(map[string]bool)
	for _, insight := range plan.Insights {
		record, ok := byID[insight.SubQueryID]
		if !ok {
			return nil, contractError(ErrorInvalidValue, "insights", "统计解释引用了未确认的结果")
		}
		switch insight.Kind {
		case "top_group":
			if record.query.GroupBy == "" || len(record.result.Groups) == 0 || insight.OtherSubQueryID != "" {
				return nil, contractError(ErrorInvalidCombination, "insights", "统计解释缺少对应排名")
			}
		case "compare_counts":
			if _, ok := byID[insight.OtherSubQueryID]; !ok || insight.OtherSubQueryID == insight.SubQueryID {
				return nil, contractError(ErrorInvalidCombination, "insights", "统计解释缺少可比较的结果")
			}
		default:
			return nil, contractError(ErrorInvalidEnum, "insights", "统计解释类型无效")
		}
		key := insight.Kind + ":" + insight.SubQueryID + ":" + insight.OtherSubQueryID
		if seen[key] {
			return nil, contractError(ErrorInvalidCombination, "insights", "统计解释重复")
		}
		seen[key] = true
	}
	return plan.Insights, nil
}

func crossInsightText(insight crossInsight, records []crossStatisticsRecord, snapshot *ServingSnapshot) (string, bool) {
	var first, second *crossStatisticsRecord
	for index := range records {
		if records[index].query.ID == insight.SubQueryID {
			first = &records[index]
		}
		if records[index].query.ID == insight.OtherSubQueryID {
			second = &records[index]
		}
	}
	if first == nil {
		return "", false
	}
	switch insight.Kind {
	case "top_group":
		if len(first.result.Groups) == 0 || first.query.GroupBy == "" {
			return "", false
		}
		maximum := first.result.Groups[0].Count
		ties := 0
		for _, group := range first.result.Groups {
			if group.Count != maximum {
				break
			}
			ties++
		}
		if ties > 1 {
			return fmt.Sprintf("在该范围内，%d 个%s分组并列第一，各有 %d 个岗位。\n", ties, crossDimensionLabel(first.query.GroupBy), maximum), true
		}
		return fmt.Sprintf("在该范围内，岗位最多的%s是 %s，共 %d 个岗位。\n", crossDimensionLabel(first.query.GroupBy), first.result.Groups[0].Label, maximum), true
	case "compare_counts":
		if second == nil || first.query.Time != second.query.Time {
			return "", false
		}
		comparison := "相同"
		if first.result.Total > second.result.Total {
			comparison = "更多"
		} else if first.result.Total < second.result.Total {
			comparison = "更少"
		}
		if comparison == "相同" {
			return fmt.Sprintf("按当前统计，%s与%s的岗位数相同。\n", crossQueryDescription(first.query, snapshot), crossQueryDescription(second.query, snapshot)), true
		}
		return fmt.Sprintf("按当前统计，%s的岗位数比%s%s。\n", crossQueryDescription(first.query, snapshot), crossQueryDescription(second.query, snapshot), comparison), true
	default:
		return "", false
	}
}

var crossInsightSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"required":["insights"],"properties":{"insights":{"type":"array","maxItems":16,"items":{"type":"object","additionalProperties":false,"required":["kind","sub_query_id"],"properties":{"kind":{"enum":["top_group","compare_counts"]},"sub_query_id":{"type":"string"},"other_sub_query_id":{"type":"string"}}}}}}`)

func (provider *QwenIntentProvider) SelectCrossInsights(ctx context.Context, facts json.RawMessage) (json.RawMessage, int, int, error) {
	if provider == nil || provider.apiKey == "" || provider.httpClient == nil || len(facts) == 0 || len(facts) > MaxToolOutputBytes || !json.Valid(facts) {
		return nil, 0, 0, contractError(ErrorConfigurationFailure, "insights", "统计解释暂不可用")
	}
	body, err := json.Marshal(qwenTrendRequest{
		Model: "qwen-plus", EnableThinking: false, Temperature: 0, MaxTokens: 400,
		Messages: []qwenTrendMessage{
			{Role: "system", Content: "根据已验证的当前岗位聚合数据选择需要解释的事实。只能返回结构化引用：top_group 引用一个有分组结果的子问题；compare_counts 引用两个子问题。不要输出数字、排名文字、自由文本或超出数据的推断。可返回空 insights。"},
			{Role: "user", Content: string(facts)},
		},
		ResponseFormat: &qwenTrendResponseFormat{Type: "json_schema", JSONSchema: &qwenTrendJSONSchema{Name: "ai-current-cross-insights-v1", Strict: true, Schema: crossInsightSchema}},
	})
	if err != nil {
		return nil, 0, 0, contractError(ErrorProviderFailure, "insights", "统计解释暂不可用")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, 0, contractError(ErrorProviderFailure, "insights", "统计解释暂不可用")
	}
	request.Header.Set("Authorization", "Bearer "+provider.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := provider.do(ctx, "select_cross_insights", request)
	if err != nil {
		return nil, 0, 0, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, 0, 0, contractError(ErrorProviderFailure, "insights", "统计解释暂不可用")
	}
	var payload qwenTrendResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil || len(payload.Choices) != 1 {
		return nil, 0, 0, contractError(ErrorProviderFailure, "insights", "统计解释暂不可用")
	}
	content := strings.TrimSpace(payload.Choices[0].Message.Content)
	if len(content) > MaxToolOutputBytes || !json.Valid([]byte(content)) || payload.Usage.PromptTokens < 0 || payload.Usage.CompletionTokens < 0 {
		return nil, 0, 0, contractError(ErrorInvalidJSON, "insights", "统计解释格式无效")
	}
	return json.RawMessage(content), payload.Usage.PromptTokens, payload.Usage.CompletionTokens, nil
}
