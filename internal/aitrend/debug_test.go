package aitrend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type debugEventCollector struct{ events []DebugTraceEvent }

func (collector *debugEventCollector) Record(event DebugTraceEvent) error {
	collector.events = append(collector.events, event)
	return nil
}

func TestDebugTraceRecorderOrdersAndBoundsEvents(t *testing.T) {
	var events []DebugTraceEvent
	recorder, err := NewDebugTraceRecorder(func(event DebugTraceEvent) error {
		events = append(events, event)
		return nil
	}, 2, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if err := recorder.Record(DebugTraceEvent{Type: DebugEventModelRequest, Body: json.RawMessage(`{"messages":[]}`)}); err != nil {
		t.Fatal(err)
	}
	if err := recorder.Record(DebugTraceEvent{Type: DebugEventFinalIntent}); err != nil {
		t.Fatal(err)
	}
	if err := recorder.Record(DebugTraceEvent{Type: DebugEventDraft}); err == nil {
		t.Fatal("third event unexpectedly succeeded")
	}
	if len(events) != 2 || events[0].Sequence != 1 || events[1].Sequence != 2 {
		t.Fatalf("events = %#v", events)
	}
}

func TestDebugTraceRecorderRejectsOversizedEventBeforeEmit(t *testing.T) {
	emitted := false
	recorder, err := NewDebugTraceRecorder(func(DebugTraceEvent) error {
		emitted = true
		return nil
	}, 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	err = recorder.Record(DebugTraceEvent{Type: DebugEventModelResponse, Body: json.RawMessage(`{"content":"` + strings.Repeat("x", 100) + `"}`)})
	if err == nil || emitted {
		t.Fatalf("err=%v emitted=%v", err, emitted)
	}
}

func TestDebugTraceEventDoesNotContainHTTPHeaders(t *testing.T) {
	event := DebugTraceEvent{Type: DebugEventModelRequest, Body: json.RawMessage(`{"model":"qwen-plus","authorization":"Bearer secret","nested":{"token":"secret"}}`), Params: json.RawMessage(`{"invite_code":"secret"}`)}
	var captured DebugTraceEvent
	recorder, err := NewDebugTraceRecorder(func(value DebugTraceEvent) error {
		captured = value
		return nil
	}, 1, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if err := recorder.Record(event); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(captured)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "Authorization") || strings.Contains(string(data), "Bearer") || strings.Contains(string(data), "secret") {
		t.Fatalf("debug event contains header material: %s", data)
	}
}

func TestQwenProviderDebugCapturesBodiesWithoutAuthorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("provider request authorization was not set")
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"{\"contract_version\":\"ai-trend-agent-v1\",\"response_modes\":[\"guidance\"]}"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	}))
	defer server.Close()
	provider := &QwenIntentProvider{apiKey: "test-key", endpoint: server.URL, httpClient: server.Client()}
	collector := &debugEventCollector{}
	content, _, _, err := provider.AnalyzeIntentUsage(withDebugTrace(context.Background(), collector), "你好", nil)
	if err != nil || len(content) == 0 || len(collector.events) != 2 {
		t.Fatalf("content=%s events=%d err=%v", content, len(collector.events), err)
	}
	for _, event := range collector.events {
		data, _ := json.Marshal(event)
		if strings.Contains(string(data), "Bearer test-key") || strings.Contains(string(data), "Authorization") {
			t.Fatalf("debug event leaked authorization: %s", data)
		}
	}
}
