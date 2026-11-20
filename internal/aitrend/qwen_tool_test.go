package aitrend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQwenIntentProviderToolTurnRoundTrip(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		var body struct {
			Messages []qwenTrendMessage `json:"messages"`
			Tools    []qwenTool         `json:"tools"`
			Format   json.RawMessage    `json:"response_format"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if requests == 1 {
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
		if len(body.Format) == 0 {
			t.Fatal("final turn did not request the Intent response schema")
		}
		if len(body.Messages) != 8 || body.Messages[6].Role != "assistant" || body.Messages[7].Role != "tool" || !strings.Contains(body.Messages[7].Content, "average_daily_active_jobs") {
			t.Fatalf("continued messages = %#v", body.Messages)
		}
		writeQwenResponse(writer, map[string]any{
			"content": `{"contract_version":"ai-trend-draft-v1","sub_queries":[{"id":"languages","dimension":"technology","expressions":["cpp"],"ranking":{"metric":"avg_daily_active_jobs","order":"desc","limit":1}}],"time_range":{"kind":"month"},"output":{"series_metric":"daily_active_jobs","chart_type":"line"}}`,
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
