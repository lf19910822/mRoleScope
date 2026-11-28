package aitrend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mrolescope/internal/jdserving"
)

type crossInsightTestProvider struct {
	*scriptedToolProvider
	response json.RawMessage
	err      error
	facts    json.RawMessage
}

func TestQwenCrossInsightRequestContainsOnlySafeAggregates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Messages []qwenTrendMessage `json:"messages"`
			Format   json.RawMessage    `json:"response_format"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Messages) != 2 || !strings.Contains(body.Messages[1].Content, `"total_jobs":12`) || strings.Contains(body.Messages[1].Content, "job_id") || strings.Contains(body.Messages[1].Content, "JD 正文") || !strings.Contains(string(body.Format), "ai-current-cross-insights-v1") {
			t.Fatalf("unsafe or incomplete explanation request: %#v", body)
		}
		writeQwenResponse(writer, map[string]any{"content": `{"insights":[{"kind":"top_group","sub_query_id":"q"}]}`}, 5, 2)
	}))
	defer server.Close()
	provider := &QwenIntentProvider{apiKey: "test-key", endpoint: server.URL, httpClient: server.Client()}
	facts := json.RawMessage(`{"results":[{"sub_query_id":"q","time":"current","total_jobs":12}]}`)
	result, input, output, err := provider.SelectCrossInsights(context.Background(), facts)
	if err != nil || !strings.Contains(string(result), "top_group") || input != 5 || output != 2 {
		t.Fatalf("insight selection=%s input=%d output=%d err=%v", result, input, output, err)
	}
}

func (provider *crossInsightTestProvider) SelectCrossInsights(_ context.Context, facts json.RawMessage) (json.RawMessage, int, int, error) {
	provider.facts = append(json.RawMessage(nil), facts...)
	return provider.response, 1, 1, provider.err
}

func TestCrossInsightUsesVerifiedFactsForTopGroup(t *testing.T) {
	snapshot := crossTestSnapshot()
	records := []crossStatisticsRecord{{query: CrossStatisticsQuery{ID: "q", Time: CrossTimeCurrent, GroupBy: DimensionJobFamily, Conditions: []CrossConditionGroup{crossCategory("game-engines")}}, result: jdserving.CrossStatisticsResult{Total: 12, Groups: []jdserving.CrossStatisticsGroup{{Value: "backend", Label: "后端", Count: 9}, {Value: "frontend", Label: "前端", Count: 3}}}}}
	provider := &crossInsightTestProvider{scriptedToolProvider: &scriptedToolProvider{}, response: json.RawMessage(`{"insights":[{"kind":"top_group","sub_query_id":"q"}]}`)}
	runner := &TrendRunner{Provider: provider}
	var output strings.Builder
	if err := runner.emitCrossResults(context.Background(), records, snapshot, false, "", func(event StreamEvent) error { output.WriteString(event.Delta); return nil }); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "岗位最多的岗位族是 后端，共 9 个岗位") || !strings.Contains(string(provider.facts), `"total_jobs":12`) || !strings.Contains(string(provider.facts), `"count":9`) {
		t.Fatalf("insight and verified evidence differ: reply=%q facts=%s", output.String(), provider.facts)
	}
}

func TestCrossInsightRejectsIncorrectFreeTextAndRetainsFact(t *testing.T) {
	snapshot := crossTestSnapshot()
	records := []crossStatisticsRecord{{query: CrossStatisticsQuery{ID: "q", Time: CrossTimeCurrent, GroupBy: DimensionJobFamily, Conditions: []CrossConditionGroup{crossCategory("game-engines")}}, result: jdserving.CrossStatisticsResult{Total: 12, Groups: []jdserving.CrossStatisticsGroup{{Value: "backend", Label: "后端", Count: 9}, {Value: "frontend", Label: "前端", Count: 3}}}}}
	for _, response := range []json.RawMessage{
		json.RawMessage(`{"insights":[{"kind":"top_group","sub_query_id":"q","text":"前端第一，有21个岗位"}]}`),
		json.RawMessage(`{"insights":[{"kind":"top_group","sub_query_id":"unknown"}]}`),
	} {
		provider := &crossInsightTestProvider{scriptedToolProvider: &scriptedToolProvider{}, response: response}
		var output strings.Builder
		if err := (&TrendRunner{Provider: provider}).emitCrossResults(context.Background(), records, snapshot, false, "", func(event StreamEvent) error { output.WriteString(event.Delta); return nil }); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), "12 个岗位") || strings.Contains(output.String(), "前端第一") || strings.Contains(output.String(), "21个") {
			t.Fatalf("unverified claim leaked or verified count was lost: %q", output.String())
		}
	}
	provider := &crossInsightTestProvider{scriptedToolProvider: &scriptedToolProvider{}, err: errors.New("provider unavailable")}
	var output strings.Builder
	if err := (&TrendRunner{Provider: provider}).emitCrossResults(context.Background(), records, snapshot, false, "", func(event StreamEvent) error { output.WriteString(event.Delta); return nil }); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "12 个岗位") {
		t.Fatalf("provider failure lost verified facts: %q", output.String())
	}
}

func TestCrossModelReceivesCompleteSummaryWhenRankingExceedsOnePage(t *testing.T) {
	result := jdserving.CrossStatisticsResult{Total: 12}
	for index := 0; index < 12; index++ {
		result.Groups = append(result.Groups, jdserving.CrossStatisticsGroup{Value: fmt.Sprintf("g-%02d", index), Label: fmt.Sprintf("分组%d", index), Count: 9})
	}
	records := []crossStatisticsRecord{{query: CrossStatisticsQuery{ID: "q", Time: CrossTimeCurrent, GroupBy: DimensionProject}, result: result}}
	payload, err := crossModelPayload(records)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Results []crossModelResult `json:"results"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Results) != 1 || len(decoded.Results[0].Groups) != 8 || decoded.Results[0].TopComplete || decoded.Results[0].TotalGroups != 12 || decoded.Results[0].TopTies != 12 || decoded.Results[0].TopCount == nil || *decoded.Results[0].TopCount != 9 {
		t.Fatalf("model did not get a complete winner summary: %s", payload)
	}
	text, ok := crossInsightText(crossInsight{Kind: "top_group", SubQueryID: "q"}, records, crossTestSnapshot())
	if !ok || !strings.Contains(text, "12 个项目分组并列第一") {
		t.Fatalf("explanation failed to account for unseen tied groups: %q", text)
	}
}
