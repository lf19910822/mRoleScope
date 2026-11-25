package aitrend

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLangfuseDebugSinkMapsGenerationAndToolToOTLP(t *testing.T) {
	var received langfuseExportRequest
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != LangfuseOTELPath {
			t.Fatalf("path = %q", request.URL.Path)
		}
		wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("pk-test:sk-test"))
		if request.Header.Get("Authorization") != wantAuth || request.Header.Get("x-langfuse-ingestion-version") != "4" {
			t.Fatalf("headers leaked or missing: auth=%t version=%q", request.Header.Get("Authorization") != "", request.Header.Get("x-langfuse-ingestion-version"))
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	sink, err := NewLangfuseDebugSink(LangfuseDebugConfig{BaseURL: server.URL, ProjectID: "project-1", PublicKey: "pk-test", SecretKey: "sk-test", MaxBytes: 1 << 20}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Record(DebugTraceEvent{Type: DebugEventModelRequest, Operation: "analyze_intent", Body: json.RawMessage(`{"model":"qwen-plus","messages":[{"role":"user","content":"哪些数据库"}]}`)}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Record(DebugTraceEvent{Type: DebugEventModelResponse, Operation: "analyze_intent", Body: json.RawMessage(`{"usage":{"prompt_tokens":12,"completion_tokens":8},"choices":[]}`)}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Record(DebugTraceEvent{Type: DebugEventToolCall, Tool: "batch_trend_summary", Params: json.RawMessage(`[{"value":"redis"}]`)}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Record(DebugTraceEvent{Type: DebugEventToolResult, Tool: "batch_trend_summary", Result: json.RawMessage(`[{"average_daily_active_jobs":10.125}]`)}); err != nil {
		t.Fatal(err)
	}
	status := sink.Flush(context.Background())
	if status.Status != "completed" || status.TraceID == "" || !strings.Contains(status.TraceURL, "/project/project-1/traces/") {
		t.Fatalf("status=%#v", status)
	}
	if len(received.ResourceSpans) != 1 || len(received.ResourceSpans[0].ScopeSpans[0].Spans) != 3 {
		t.Fatalf("spans=%#v", received)
	}
	encoded, _ := json.Marshal(received)
	if strings.Contains(string(encoded), "Authorization") || strings.Contains(string(encoded), "sk-test") {
		t.Fatalf("credentials leaked in payload: %s", encoded)
	}
}

func TestLangfuseDebugSinkDisabledWithoutCompleteRuntimeConfig(t *testing.T) {
	sink, err := NewLangfuseDebugSink(LangfuseDebugConfig{BaseURL: "https://jp.cloud.langfuse.com", PublicKey: "pk-test"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Record(DebugTraceEvent{Type: DebugEventDraft}); err != nil {
		t.Fatal(err)
	}
	if status := sink.Flush(context.Background()); status.Status != "disabled" {
		t.Fatalf("status=%#v", status)
	}
}

func TestLangfuseDebugSinkUploadFailureDoesNotReturnRecordError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	sink, err := NewLangfuseDebugSink(LangfuseDebugConfig{BaseURL: server.URL, ProjectID: "project-1", PublicKey: "pk-test", SecretKey: "sk-test", FlushTimeout: time.Second, MaxBytes: 1 << 20}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Record(DebugTraceEvent{Type: DebugEventTerminal, Status: string(StreamSuccess)}); err != nil {
		t.Fatal(err)
	}
	if status := sink.Flush(context.Background()); status.Status != "failed" || status.ErrorCode != "http_502" {
		t.Fatalf("status=%#v", status)
	}
}
