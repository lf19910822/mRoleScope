package aitrend

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestQwenIntentProviderToolTurnRoundTrip(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		var body struct {
			Messages []qwenTrendMessage `json:"messages"`
			Tools    []qwenTool         `json:"tools"`
			Format   json.RawMessage    `json:"response_format"`
			Result   string             `json:"result_format"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if requests == 1 {
			if body.Result != "message" {
				t.Fatalf("tool turn result format = %q, want message", body.Result)
			}
			if len(body.Tools) == 0 {
				t.Fatal("first model turn did not receive tools")
			}
			if len(body.Format) != 0 {
				t.Fatalf("discovery turn unexpectedly forced response format: %s", body.Format)
			}
			if len(body.Messages) != 2 || body.Messages[1].Role != "user" {
				t.Fatalf("first messages = %#v", body.Messages)
			}
			writeQwenResponse(writer, map[string]any{
				"content": "",
				"tool_calls": []any{map[string]any{
					"id": "call-category", "type": "function",
					"function": map[string]string{"name": string(ToolListTechnologyCategories), "arguments": `{}`},
				}},
			}, 12, 8)
			return
		}
		if requests == 2 {
			if len(body.Format) != 0 {
				t.Fatalf("second discovery turn unexpectedly forced response format: %s", body.Format)
			}
			if len(body.Messages) != 4 || body.Messages[2].Role != "assistant" || body.Messages[3].Role != "tool" || !strings.Contains(body.Messages[3].Content, "programming-languages") {
				t.Fatalf("continued messages = %#v", body.Messages)
			}
			writeQwenResponse(writer, map[string]any{
				"content": "",
				"tool_calls": []any{map[string]any{
					"id": "call-values", "type": "function",
					"function": map[string]string{"name": string(ToolListCategoryValues), "arguments": `{"category":"programming-languages"}`},
				}},
			}, 18, 9)
			return
		}
		if requests == 3 {
			if len(body.Format) != 0 {
				t.Fatalf("summary planning turn unexpectedly forced response format: %s", body.Format)
			}
			if len(body.Messages) != 6 || body.Messages[4].Role != "assistant" || body.Messages[5].Role != "tool" || !strings.Contains(body.Messages[5].Content, "cpp") {
				t.Fatalf("continued messages = %#v", body.Messages)
			}
			writeQwenResponse(writer, map[string]any{
				"content": "",
				"tool_calls": []any{map[string]any{
					"id": "call-summary", "type": "function",
					"function": map[string]string{"name": string(ToolBatchTrendSummary), "arguments": `[{"dimension":"technology","value":"cpp","range":"month"}]`},
				}},
			}, 21, 13)
			return
		}
		if !strings.Contains(string(body.Format), `"type":"json_schema"`) || !strings.Contains(string(body.Format), AgentContractVersionV1) {
			t.Fatalf("final evidence turn did not request Agent schema: %s", body.Format)
		}
		if strings.Contains(string(body.Format), `"uniqueItems"`) || strings.Contains(string(body.Format), `"contains"`) || strings.Contains(string(body.Format), `"minContains"`) || strings.Contains(string(body.Format), `"maxContains"`) {
			t.Fatalf("final evidence turn used Qwen-incompatible array schema keywords: %s", body.Format)
		}
		if body.Result != "" || len(body.Tools) != 0 {
			t.Fatalf("final evidence turn retained tool protocol fields: result=%q tools=%d", body.Result, len(body.Tools))
		}
		if len(body.Messages) != 3 || body.Messages[2].Role != "user" || !strings.Contains(body.Messages[2].Content, "average_daily_active_jobs") {
			t.Fatalf("continued messages = %#v", body.Messages)
		}
		writeQwenResponse(writer, map[string]any{
			"content": `{"contract_version":"ai-trend-agent-v1","response_modes":["trend"],"trend_intent":{"contract_version":"ai-trend-draft-v1","sub_queries":[{"id":"languages","dimension":"technology","expressions":["cpp"],"ranking":{"metric":"avg_daily_active_jobs","order":"desc","limit":1}}],"time_range":{"kind":"month"},"output":{"series_metric":"daily_active_jobs","chart_type":"line"}}}`,
		}, 21, 13)
	}))
	defer server.Close()
	provider := &QwenIntentProvider{apiKey: "test-key", endpoint: server.URL, httpClient: server.Client()}
	turn, err := provider.AnalyzeIntentTurn(context.Background(), "使用最多的编程语言是什么", nil, nil, true)
	if err != nil || len(turn.ToolCalls) != 1 || turn.ToolCalls[0].Name != ToolListTechnologyCategories || turn.InputTokens != 12 || turn.OutputTokens != 8 {
		t.Fatalf("first turn = %#v err=%v", turn, err)
	}
	history := []ToolConversation{{
		Calls:   turn.ToolCalls,
		Results: []ToolCallResult{{Name: ToolListTechnologyCategories, Payload: json.RawMessage(`[{"value":"programming-languages","label":"编程语言"}]`), Available: true}},
	}}
	turn, err = provider.AnalyzeIntentTurn(context.Background(), "使用最多的编程语言是什么", nil, history, true)
	if err != nil || len(turn.Intent) != 0 || len(turn.ToolCalls) != 1 || turn.ToolCalls[0].Name != ToolListCategoryValues || turn.InputTokens != 18 || turn.OutputTokens != 9 {
		t.Fatalf("second turn = %#v err=%v", turn, err)
	}
	history = append(history, ToolConversation{
		Calls:   turn.ToolCalls,
		Results: []ToolCallResult{{Name: ToolListCategoryValues, Payload: json.RawMessage(`[{"value":"cpp","label":"C++"}]`), Available: true}},
	})
	turn, err = provider.AnalyzeIntentTurn(context.Background(), "使用最多的编程语言是什么", nil, history, true)
	if err != nil || len(turn.Intent) != 0 || len(turn.ToolCalls) != 1 || turn.ToolCalls[0].Name != ToolBatchTrendSummary || turn.InputTokens != 21 || turn.OutputTokens != 13 {
		t.Fatalf("summary turn = %#v err=%v", turn, err)
	}
	history = append(history, ToolConversation{
		Calls:   turn.ToolCalls,
		Results: []ToolCallResult{{Name: ToolBatchTrendSummary, Payload: json.RawMessage(`[{"curve":{"dimension":"technology","value":"cpp"},"average_daily_active_jobs":42}]`), Available: true}},
	})
	turn, err = provider.AnalyzeIntentTurn(context.Background(), "使用最多的编程语言是什么", nil, history, true)
	if err != nil || len(turn.Intent) == 0 || len(turn.ToolCalls) != 0 || turn.InputTokens != 21 || turn.OutputTokens != 13 {
		t.Fatalf("final turn = %#v err=%v", turn, err)
	}
	if requests != 4 {
		t.Fatalf("provider requests = %d, want 4", requests)
	}
}

func TestQwenIntentProviderAddsRepairInstructionAfterModelCorrectableToolError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Messages []qwenTrendMessage `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Messages) < 2 || !strings.Contains(body.Messages[1].Content, "上一轮工具调用失败") || !strings.Contains(body.Messages[1].Content, "修正参数") {
			t.Fatalf("repair instruction missing from model prompt: %#v", body.Messages)
		}
		writeQwenResponse(writer, map[string]any{"content": "", "tool_calls": []any{map[string]any{
			"id": "call-formal-category", "type": "function",
			"function": map[string]string{"name": string(ToolListCategoryValues), "arguments": `{"category":"databases-and-storage"}`},
		}}}, 4, 2)
	}))
	defer server.Close()
	provider := &QwenIntentProvider{apiKey: "test-key", endpoint: server.URL, httpClient: server.Client()}
	history := []ToolConversation{{
		Calls:   []ToolCall{{ID: "bad-category", Name: ToolListCategoryValues, Params: json.RawMessage(`{"category":"database"}`)}},
		Results: []ToolCallResult{{Name: ToolListCategoryValues, Available: false, SafeError: &SafeError{Code: ErrorInvalidValue, Field: "category", Message: "technology category must be a current formal category value or label"}}},
	}}
	turn, err := provider.AnalyzeIntentTurn(context.Background(), "哪些数据库？", nil, history, true)
	if err != nil || len(turn.ToolCalls) != 1 || turn.ToolCalls[0].Name != ToolListCategoryValues {
		t.Fatalf("corrective tool turn=%#v err=%v", turn, err)
	}
}

func TestQwenIntentProviderDoesNotTreatBackendFailureAsParameterCorrection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Messages []qwenTrendMessage `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(body.Messages[1].Content, "修正参数") {
			t.Fatalf("backend failure was presented as parameter correction: %#v", body.Messages[1])
		}
		writeQwenResponse(writer, map[string]any{"content": `{"contract_version":"ai-trend-agent-v1","response_modes":["guidance"]}`}, 4, 2)
	}))
	defer server.Close()
	provider := &QwenIntentProvider{apiKey: "test-key", endpoint: server.URL, httpClient: server.Client()}
	history := []ToolConversation{{
		Calls:   []ToolCall{{ID: "summary", Name: ToolBatchTrendSummary, Params: json.RawMessage(`[]`)}},
		Results: []ToolCallResult{{Name: ToolBatchTrendSummary, Available: false, SafeError: &SafeError{Code: ErrorBackendFailure, Field: "serving", Message: "tool execution failed"}}},
	}}
	if _, err := provider.AnalyzeIntentTurn(context.Background(), "哪些项目？", nil, history, true); err != nil {
		t.Fatal(err)
	}
}

func TestQwenIntentProviderClassifiesServiceModesWithStrictNoToolRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Messages []qwenTrendMessage `json:"messages"`
			Tools    []qwenTool         `json:"tools"`
			Format   json.RawMessage    `json:"response_format"`
			Max      int                `json:"max_tokens"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Messages) != 2 || len(body.Tools) != 0 || body.Max != 180 || !strings.Contains(body.Messages[0].Content, "service_tags") || !strings.Contains(string(body.Format), ServiceModeContractVersionV1) {
			t.Fatalf("classification request = %#v format=%s", body, body.Format)
		}
		writeQwenResponse(writer, map[string]any{"content": `{"contract_version":"ai-trend-service-mode-v1","service_tags":["guidance","trend"]}`}, 41, 8)
	}))
	defer server.Close()
	provider := &QwenIntentProvider{apiKey: "test-key", endpoint: server.URL, httpClient: server.Client()}
	classification, input, output, err := provider.ClassifyServiceModes(context.Background(), "比较 Go 和 Python，并说明如何修改")
	if err != nil || input != 41 || output != 8 || len(classification.ServiceTags) != 2 || classification.ServiceTags[0] != ServiceTagTrend || classification.ServiceTags[1] != ServiceTagGuidance {
		t.Fatalf("classification=%#v usage=%d/%d err=%v", classification, input, output, err)
	}
}

func TestQwenIntentProviderUsesServerDeterminedServiceTagsInPlannerPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Messages []qwenTrendMessage `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Messages) != 2 || !strings.Contains(body.Messages[1].Content, `service_tags 为 ["trend","guidance"]`) || !strings.Contains(body.Messages[1].Content, "不要重新判断或修改服务类别") {
			t.Fatalf("planner prompt did not carry server tags: %#v", body.Messages)
		}
		writeQwenResponse(writer, map[string]any{"content": "", "tool_calls": []any{map[string]any{
			"id": "call-summary", "type": "function",
			"function": map[string]string{"name": string(ToolBatchTrendSummary), "arguments": `[{"dimension":"technology","value":"go","range":"month"}]`},
		}}}, 4, 2)
	}))
	defer server.Close()
	provider := &QwenIntentProvider{apiKey: "test-key", endpoint: server.URL, httpClient: server.Client()}
	ctx := withServiceTags(context.Background(), []ServiceTag{ServiceTagTrend, ServiceTagGuidance})
	turn, err := provider.AnalyzeIntentTurn(ctx, "比较 Go 并说明如何修改", nil, nil, true)
	if err != nil || len(turn.ToolCalls) != 1 || turn.ToolCalls[0].Name != ToolBatchTrendSummary {
		t.Fatalf("planner turn = %#v err=%v", turn, err)
	}
}

func TestQwenIntentProviderMapsClassificationFailures(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantCode   SafeErrorCode
	}{
		{name: "provider failure", statusCode: http.StatusBadGateway, body: `upstream unavailable`, wantCode: ErrorProviderFailure},
		{name: "invalid json", statusCode: http.StatusOK, body: `{"choices":[{"message":{"content":"not-json"}}],"usage":{"prompt_tokens":2,"completion_tokens":1}}`, wantCode: ErrorInvalidJSON},
		{name: "unknown tag", statusCode: http.StatusOK, body: `{"choices":[{"message":{"content":"{\"contract_version\":\"ai-trend-service-mode-v1\",\"service_tags\":[\"trend\",\"export\"]}"}}],"usage":{"prompt_tokens":2,"completion_tokens":1}}`, wantCode: ErrorInvalidEnum},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.statusCode)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			provider := &QwenIntentProvider{apiKey: "test-key", endpoint: server.URL, httpClient: server.Client()}
			_, _, _, err := provider.ClassifyServiceModes(context.Background(), "你好")
			if err == nil {
				t.Fatal("classification failure was accepted")
			}
			contractErr, ok := err.(*ContractError)
			if !ok || contractErr.Code != test.wantCode {
				t.Fatalf("classification error = %#v, want %s", err, test.wantCode)
			}
		})
	}
}

func TestQwenIntentProviderPropagatesClassificationTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()
	provider := &QwenIntentProvider{apiKey: "test-key", endpoint: server.URL, httpClient: server.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	_, _, _, err := provider.ClassifyServiceModes(ctx, "你好")
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("classification timeout = %v, want deadline exceeded", err)
	}
}

func TestQwenIntentProviderKeepsToolCallsWhenMessageAlsoHasText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeQwenResponse(writer, map[string]any{
			"content": "我会先读取项目候选。",
			"tool_calls": []any{map[string]any{
				"id": "call-projects", "type": "function",
				"function": map[string]string{"name": string(ToolListAvailableValues), "arguments": `{"dimension":"project"}`},
			}},
		}, 10, 4)
	}))
	defer server.Close()
	provider := &QwenIntentProvider{apiKey: "test-key", endpoint: server.URL, httpClient: server.Client()}
	turn, err := provider.AnalyzeIntentTurn(context.Background(), "哪些项目岗位最多？并说明怎么修改", nil, nil, true)
	if err != nil || len(turn.ToolCalls) != 1 || turn.ToolCalls[0].Name != ToolListAvailableValues {
		t.Fatalf("turn = %#v err=%v", turn, err)
	}
}

func TestQwenIntentProviderFallsBackToGuidanceForInitialPlainText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeQwenResponse(writer, map[string]any{"content": "你好，我可以帮助你分析趋势。"}, 11, 7)
	}))
	defer server.Close()
	provider := &QwenIntentProvider{apiKey: "test-key", endpoint: server.URL, httpClient: server.Client()}
	turn, err := provider.AnalyzeIntentTurn(context.Background(), "你好", nil, nil, true)
	if err != nil || string(turn.Intent) != `{"contract_version":"ai-trend-agent-v1","response_modes":["guidance"]}` || turn.InputTokens != 11 || turn.OutputTokens != 7 {
		t.Fatalf("turn = %#v err=%v", turn, err)
	}
}

func writeQwenResponse(writer http.ResponseWriter, message map[string]any, promptTokens, completionTokens int) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"choices": []any{map[string]any{"message": message}},
		"usage":   map[string]int{"prompt_tokens": promptTokens, "completion_tokens": completionTokens, "total_tokens": promptTokens + completionTokens},
	})
}

func TestMarshalToolResultForModelOmitsInternalCallMetadata(t *testing.T) {
	data := marshalToolResultForModel(ToolCallResult{
		Name:       ToolBatchTrendSummary,
		CallNumber: 19,
		Payload:    json.RawMessage(`[{"curve":{"value":"cpp"}}]`),
		SafeError:  &SafeError{Code: ErrorBackendFailure, Message: "tool execution failed"},
	})
	if strings.Contains(string(data), "CallNumber") || strings.Contains(string(data), "19") || !strings.Contains(string(data), "cpp") || !strings.Contains(string(data), "backend_failure") {
		t.Fatalf("model tool result leaked or dropped safe data: %s", data)
	}
}

func TestUnwrapIntentEnvelopeAcceptsProviderWrapper(t *testing.T) {
	content := []byte(`{"intent":{"contract_version":"ai-trend-draft-v1"}}`)
	if got := string(unwrapIntentEnvelope(content)); got != `{"contract_version":"ai-trend-draft-v1"}` {
		t.Fatalf("unwrapped intent = %s", got)
	}
	plain := []byte(`{"contract_version":"ai-trend-draft-v1"}`)
	if got := string(unwrapIntentEnvelope(plain)); got != string(plain) {
		t.Fatalf("plain intent changed = %s", got)
	}
	stringEnvelope := []byte(`{"type":"final","intent":"{\"contract_version\":\"ai-trend-draft-v1\"}"}`)
	if got := string(unwrapIntentEnvelope(stringEnvelope)); got != `{"contract_version":"ai-trend-draft-v1"}` {
		t.Fatalf("string intent envelope = %s", got)
	}
	typeEnvelope := []byte(`{"type":"final","contract_version":"ai-trend-draft-v1"}`)
	if got := string(unwrapIntentEnvelope(typeEnvelope)); got != `{"contract_version":"ai-trend-draft-v1"}` {
		t.Fatalf("type intent envelope = %s", got)
	}
}
