package aitrend

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQwenProviderStreamsAssistantMessageDeltasAndUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Accept") != "text/event-stream" {
			t.Fatalf("Accept = %q", request.Header.Get("Accept"))
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["stream"] != true {
			t.Fatalf("stream = %#v", body["stream"])
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"你好，\"}}]}\n\n"))
		_, _ = writer.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"\"}}]}\n\n"))
		_, _ = writer.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"可以查看趋势\"}}],\"usage\":{\"prompt_tokens\":31,\"completion_tokens\":14}}\n\n"))
		_, _ = writer.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()
	provider := &QwenIntentProvider{apiKey: "test-key", endpoint: server.URL, httpClient: server.Client()}
	var chunks []string
	inputTokens, outputTokens, err := provider.StreamAssistantMessage(context.Background(), AssistantReplyContext{Query: "你好", ResponseModes: []ResponseMode{ResponseModeGuidance}}, func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil || strings.Join(chunks, "") != "你好，可以查看趋势" || inputTokens != 31 || outputTokens != 14 {
		t.Fatalf("chunks=%q usage=%d/%d err=%v", chunks, inputTokens, outputTokens, err)
	}
}

func TestQwenProviderStopsStreamingWhenConsumerStops(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"第一段\"}}]}\n\n"))
	}))
	defer server.Close()
	provider := &QwenIntentProvider{apiKey: "test-key", endpoint: server.URL, httpClient: server.Client()}
	sentinel := errors.New("consumer stopped")
	_, _, err := provider.StreamAssistantMessage(context.Background(), AssistantReplyContext{Query: "你好", ResponseModes: []ResponseMode{ResponseModeGuidance}}, func(string) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatalf("consumer error = %v, want %v", err, sentinel)
	}
}

func TestQwenProviderClassifiesStreamUpstreamFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "upstream unavailable", http.StatusBadGateway)
	}))
	defer server.Close()
	provider := &QwenIntentProvider{apiKey: "test-key", endpoint: server.URL, httpClient: server.Client()}
	_, _, err := provider.StreamAssistantMessage(context.Background(), AssistantReplyContext{Query: "你好", ResponseModes: []ResponseMode{ResponseModeGuidance}}, func(string) error { return nil })
	if err == nil {
		t.Fatal("stream upstream failure was accepted")
	}
	contractErr, ok := err.(*ContractError)
	if !ok || contractErr.Code != ErrorProviderFailure {
		t.Fatalf("stream error = %#v", err)
	}
}
