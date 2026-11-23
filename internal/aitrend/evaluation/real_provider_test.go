package evaluation

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"mrolescope/internal/aitrend"
)

func TestRunnerUsesQwenProviderAgainstEvaluationSnapshot(t *testing.T) {
	snapshot, err := LoadSnapshot(filepath.Join("testdata", "serving-snapshot-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	requests := 0
	client := &http.Client{Transport: realEvaluationRoundTripper(func(request *http.Request) (*http.Response, error) {
		requests++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		if strings.Contains(string(body), `"stream":true`) {
			return jsonHTTPResponse(http.StatusOK, "text/event-stream", "data: {\"choices\":[{\"delta\":{\"content\":\"当前支持趋势分析。\"}}],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":4}}\n\ndata: [DONE]\n\n"), nil
		}
		return jsonHTTPResponse(http.StatusOK, "application/json", `{"choices":[{"message":{"content":"{\"contract_version\":\"ai-trend-agent-v1\",\"response_modes\":[\"guidance\"]}"}}],"usage":{"prompt_tokens":11,"completion_tokens":5,"total_tokens":16}}`), nil
	})}
	provider, err := aitrend.NewQwenIntentProvider("synthetic-key", "llm-mrolescope", client)
	if err != nil {
		t.Fatal(err)
	}
	runner, err := NewRunner(provider, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.Run(context.Background(), "你好")
	if err != nil {
		t.Fatal(err)
	}
	if result.Draft != nil || result.Stats.ResponseModes[0] != aitrend.ResponseModeGuidance || result.Stats.InputTokens != 18 || result.Stats.OutputTokens != 9 {
		t.Fatalf("real provider result = %#v", result)
	}
	if requests != 2 || assistantEventText(result.Events) != "当前支持趋势分析。" {
		t.Fatalf("requests=%d events=%#v", requests, result.Events)
	}
}

func TestRunnerUsesQwenProviderForTrendDraftAndReply(t *testing.T) {
	snapshot, err := LoadSnapshot(filepath.Join("testdata", "serving-snapshot-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	requests := 0
	client := &http.Client{Transport: realEvaluationRoundTripper(func(request *http.Request) (*http.Response, error) {
		requests++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		if strings.Contains(string(body), `"stream":true`) {
			return jsonHTTPResponse(http.StatusOK, "text/event-stream", "data: {\"choices\":[{\"delta\":{\"content\":\"已按 Go 和 Python 的岗位数量整理曲线。\"}}],\"usage\":{\"prompt_tokens\":13,\"completion_tokens\":8}}\n\ndata: [DONE]\n\n"), nil
		}
		if requests == 1 {
			return jsonHTTPResponse(http.StatusOK, "application/json", `{"choices":[{"message":{"content":"","tool_calls":[{"id":"summary","type":"function","function":{"name":"batch_trend_summary","arguments":"[{\"dimension\":\"technology\",\"value\":\"go\",\"range\":\"all\"},{\"dimension\":\"technology\",\"value\":\"python\",\"range\":\"all\"}]"}}]}}],"usage":{"prompt_tokens":17,"completion_tokens":9,"total_tokens":26}}`), nil
		}
		return jsonHTTPResponse(http.StatusOK, "application/json", `{"choices":[{"message":{"content":"{\"contract_version\":\"ai-trend-agent-v1\",\"response_modes\":[\"trend\",\"guidance\"],\"trend_intent\":{\"contract_version\":\"ai-trend-draft-v1\",\"sub_queries\":[{\"id\":\"compare\",\"dimension\":\"technology\",\"expressions\":[\"go\",\"python\"],\"ranking\":{\"metric\":\"avg_daily_active_jobs\",\"order\":\"desc\",\"limit\":2}}],\"time_range\":{\"kind\":\"all\"},\"output\":{\"series_metric\":\"daily_active_jobs\",\"chart_type\":\"line\"}}}"}}],"usage":{"prompt_tokens":19,"completion_tokens":12,"total_tokens":31}}`), nil
	})}
	provider, err := aitrend.NewQwenIntentProvider("synthetic-key", "llm-mrolescope", client)
	if err != nil {
		t.Fatal(err)
	}
	runner, err := NewRunner(provider, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.Run(context.Background(), "比较 Go 和 Python 的全部历史岗位数量")
	if err != nil {
		t.Fatal(err)
	}
	if result.Draft == nil || len(result.Draft.Curves) != 2 || requests != 3 || assistantEventText(result.Events) != "已按 Go 和 Python 的岗位数量整理曲线。" {
		t.Fatalf("requests=%d draft=%#v events=%#v", requests, result.Draft, result.Events)
	}
	if result.Stats.InputTokens != 49 || result.Stats.OutputTokens != 29 {
		t.Fatalf("usage=%d/%d", result.Stats.InputTokens, result.Stats.OutputTokens)
	}
}

type realEvaluationRoundTripper func(*http.Request) (*http.Response, error)

func (function realEvaluationRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func jsonHTTPResponse(status int, contentType, body string) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Header:        http.Header{"Content-Type": []string{contentType}},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
	}
}

func assistantEventText(events []aitrend.StreamEvent) string {
	var builder strings.Builder
	for _, event := range events {
		if event.Type == aitrend.StreamEventAssistant {
			builder.WriteString(event.Delta)
		}
	}
	return builder.String()
}
