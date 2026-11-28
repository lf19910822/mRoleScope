package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mrolescope/internal/aitrend"
	"mrolescope/internal/jdserving"
)

type aiTrendRunnerFunc func(context.Context, string, func(aitrend.StreamEvent) error) error

func (function aiTrendRunnerFunc) Run(ctx context.Context, query string, emit func(aitrend.StreamEvent) error) error {
	return function(ctx, query, emit)
}

type aiTrendDebugRunnerFunc func(context.Context, string, func(aitrend.StreamEvent) error, aitrend.DebugTraceSink) error

func (function aiTrendDebugRunnerFunc) RunDebug(ctx context.Context, query string, emit func(aitrend.StreamEvent) error, sink aitrend.DebugTraceSink) error {
	return function(ctx, query, emit, sink)
}

func TestAITrendDebugRejectsWhenDisabledOrNonLoopback(t *testing.T) {
	runner := aiTrendDebugRunnerFunc(func(context.Context, string, func(aitrend.StreamEvent) error, aitrend.DebugTraceSink) error {
		t.Fatal("debug runner was invoked")
		return nil
	})
	for _, remote := range []string{"127.0.0.1:1234", "192.0.2.10:1234"} {
		router := NewWithAccessAndAIAndDebug(nil, nil, nil, runner, AIDebugConfig{Enabled: remote == "192.0.2.10:1234", MaxEvents: 10, MaxBytes: 4096}, nil)
		request := httptest.NewRequest(http.MethodPost, "/internal/ai-debug", strings.NewReader(`{"query":"哪些数据库"}`))
		request.RemoteAddr = remote
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("remote=%s status=%d body=%s", remote, response.Code, response.Body.String())
		}
	}
}

func TestAITrendDebugStreamsDetailedEventsOnlyOnLoopback(t *testing.T) {
	runner := aiTrendDebugRunnerFunc(func(_ context.Context, query string, emit func(aitrend.StreamEvent) error, sink aitrend.DebugTraceSink) error {
		if query != "哪些数据库" {
			t.Fatalf("query=%q", query)
		}
		if err := sink.Record(aitrend.DebugTraceEvent{Type: aitrend.DebugEventModelRequest, Operation: "analyze_intent", Body: json.RawMessage(`{"model":"qwen-plus"}`)}); err != nil {
			return err
		}
		event := aitrend.StreamEvent{Type: aitrend.StreamEventDraft, Status: aitrend.StreamCompleted, Draft: &aitrend.Draft{ContractVersion: aitrend.ContractVersionV1, Status: aitrend.DraftComplete}}
		if err := sink.Record(aitrend.DebugTraceEvent{Type: aitrend.DebugEventStream, Status: string(event.Status), Stream: &event}); err != nil {
			return err
		}
		return emit(event)
	})
	router := NewWithAccessAndAIAndDebug(nil, nil, nil, runner, AIDebugConfig{Enabled: true, MaxEvents: 10, MaxBytes: 4096}, nil)
	request := httptest.NewRequest(http.MethodPost, "/internal/ai-debug", strings.NewReader(`{"query":"哪些数据库"}`))
	request.RemoteAddr = "127.0.0.1:1234"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"type":"model_request"`) || !strings.Contains(response.Body.String(), `"type":"stream_event"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAITrendDebugRejectsMalformedAndOversizedRequests(t *testing.T) {
	runner := aiTrendDebugRunnerFunc(func(context.Context, string, func(aitrend.StreamEvent) error, aitrend.DebugTraceSink) error {
		t.Fatal("debug runner was invoked")
		return nil
	})
	router := NewWithAccessAndAIAndDebug(nil, nil, nil, runner, AIDebugConfig{Enabled: true, MaxEvents: 10, MaxBytes: 4096}, nil)
	for _, body := range []string{"{", `{"query":"` + strings.Repeat("x", 9000) + `"}`} {
		request := httptest.NewRequest(http.MethodPost, "/internal/ai-debug", strings.NewReader(body))
		request.RemoteAddr = "127.0.0.1:1234"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body length=%d status=%d body=%s", len(body), response.Code, response.Body.String())
		}
	}
}

func TestAITrendDebugRejectsInvalidSessionContext(t *testing.T) {
	runner := aiTrendDebugRunnerFunc(func(context.Context, string, func(aitrend.StreamEvent) error, aitrend.DebugTraceSink) error {
		t.Fatal("debug runner was invoked")
		return nil
	})
	router := NewWithAccessAndAIAndDebug(nil, nil, nil, runner, AIDebugConfig{Enabled: true, MaxEvents: 10, MaxBytes: 4096}, nil)
	request := httptest.NewRequest(http.MethodPost, "/internal/ai-debug", strings.NewReader(`{"query":"哪些数据库","session_id":"评测"}`))
	request.RemoteAddr = "127.0.0.1:1234"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "invalid_trace_context") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAITrendDebugPropagatesExplicitSessionContext(t *testing.T) {
	runner := aiTrendDebugRunnerFunc(func(ctx context.Context, _ string, _ func(aitrend.StreamEvent) error, sink aitrend.DebugTraceSink) error {
		if err := sink.Record(aitrend.DebugTraceEvent{Type: aitrend.DebugEventTerminal, Status: string(aitrend.StreamSuccess)}); err != nil {
			return err
		}
		return nil
	})
	router := NewWithAccessAndAIAndDebug(nil, nil, nil, runner, AIDebugConfig{Enabled: true, MaxEvents: 10, MaxBytes: 4096}, nil)
	request := httptest.NewRequest(http.MethodPost, "/internal/ai-debug", strings.NewReader(`{"query":"哪些数据库","session_id":"debug:database"}`))
	request.RemoteAddr = "127.0.0.1:1234"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"type":"terminal"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func (aiTrendRunnerFunc) ListJobs(context.Context, jdserving.ListJobsQuery) (jdserving.JobListResponse, error) {
	return jdserving.JobListResponse{}, nil
}

func TestAITrendDraftStreamsNDJSONOperationEvents(t *testing.T) {
	runner := aiTrendRunnerFunc(func(_ context.Context, query string, emit func(aitrend.StreamEvent) error) error {
		if query != "哪些数据库" {
			return nil
		}
		if err := emit(aitrend.StreamEvent{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolClassifyServiceModes), Params: []byte(`{"service_tags":["trend"],"internal":"secret"}`)}); err != nil {
			return err
		}
		if err := emit(aitrend.StreamEvent{Type: aitrend.StreamEventOperation, Status: aitrend.StreamCompleted, Tool: string(aitrend.ToolClassifyServiceModes)}); err != nil {
			return err
		}
		if err := emit(aitrend.StreamEvent{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolAnalyzeIntent)}); err != nil {
			return err
		}
		if err := emit(aitrend.StreamEvent{Type: aitrend.StreamEventOperation, Status: aitrend.StreamCompleted, Tool: string(aitrend.ToolAnalyzeIntent)}); err != nil {
			return err
		}
		if err := emit(aitrend.StreamEvent{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolListTechnologyCategories), Params: []byte(`{"category":"databases-and-storage"}`)}); err != nil {
			return err
		}
		return emit(aitrend.StreamEvent{Type: aitrend.StreamEventDone, Status: aitrend.StreamSuccess})
	})
	router := New(nil, runner)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/job-trends/ai-draft", strings.NewReader(`{"query":"哪些数据库"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Type") != "application/x-ndjson; charset=utf-8" {
		t.Fatalf("content type = %q", response.Header().Get("Content-Type"))
	}
	if lines := strings.Count(response.Body.String(), "\n"); lines != 6 {
		t.Fatalf("stream lines = %d body=%s", lines, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"tool":"analyze_intent"`) {
		t.Fatalf("model operation missing: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"tool":"classify_service_modes"`) || strings.Contains(response.Body.String(), "service_tags") || strings.Contains(response.Body.String(), "internal") || strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("classification operation leaked raw content: %s", response.Body.String())
	}
}

func TestAITrendDraftRejectsUnknownInput(t *testing.T) {
	router := New(nil, aiTrendRunnerFunc(func(context.Context, string, func(aitrend.StreamEvent) error) error { return nil }))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/job-trends/ai-draft", strings.NewReader(`{"query":"x","raw":"secret"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestAITrendDraftFiltersUnsafeToolParameters(t *testing.T) {
	runner := aiTrendRunnerFunc(func(_ context.Context, _ string, emit func(aitrend.StreamEvent) error) error {
		if err := emit(aitrend.StreamEvent{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolListAvailableValues), Params: []byte(`{"dimension":"technology","raw_sql":"select secret"}`)}); err != nil {
			return err
		}
		return emit(aitrend.StreamEvent{Type: aitrend.StreamEventDone, Status: aitrend.StreamSuccess})
	})
	router := New(nil, runner)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/job-trends/ai-draft", strings.NewReader(`{"query":"哪些技术"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if strings.Contains(response.Body.String(), "raw_sql") || strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("unsafe parameters leaked: %s", response.Body.String())
	}
}

func TestAITrendDraftRedactsCrossStatisticsToolParameters(t *testing.T) {
	runner := aiTrendRunnerFunc(func(_ context.Context, _ string, emit func(aitrend.StreamEvent) error) error {
		return emit(aitrend.StreamEvent{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolCrossStatistics), Params: []byte(`{"sub_queries":[{"id":"q","sql":"SELECT secret","job_id":"secret-job"}]}`)})
	})
	router := New(nil, runner)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/job-trends/ai-draft", strings.NewReader(`{"query":"上海 Java 岗位"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if !strings.Contains(response.Body.String(), `"tool":"query_current_job_statistics"`) || strings.Contains(response.Body.String(), "secret") || strings.Contains(response.Body.String(), "job_id") || strings.Contains(response.Body.String(), "SELECT") {
		t.Fatalf("cross-statistics tool event is unsafe: %s", response.Body.String())
	}
}

func TestAITrendDraftKeepsSafeBatchParameters(t *testing.T) {
	runner := aiTrendRunnerFunc(func(_ context.Context, _ string, emit func(aitrend.StreamEvent) error) error {
		return emit(aitrend.StreamEvent{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolBatchTrendSummary), Params: []byte(`[{"dimension":"technology","value":"redis","range":"month","raw_sql":"secret"}]`)})
	})
	router := New(nil, runner)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/job-trends/ai-draft", strings.NewReader(`{"query":"哪些数据库"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	body := response.Body.String()
	if strings.Contains(body, "raw_sql") || strings.Contains(body, "secret") {
		t.Fatalf("unsafe batch parameters leaked: %s", body)
	}
	if !strings.Contains(body, "redis") || !strings.Contains(body, "month") {
		t.Fatalf("safe batch parameters were removed: %s", body)
	}
}

func TestAITrendDraftAllowsExplanationOperation(t *testing.T) {
	runner := aiTrendRunnerFunc(func(_ context.Context, _ string, emit func(aitrend.StreamEvent) error) error {
		if err := emit(aitrend.StreamEvent{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolExplainDraft)}); err != nil {
			return err
		}
		if err := emit(aitrend.StreamEvent{Type: aitrend.StreamEventOperation, Status: aitrend.StreamCompleted, Tool: string(aitrend.ToolExplainDraft)}); err != nil {
			return err
		}
		return emit(aitrend.StreamEvent{Type: aitrend.StreamEventDone, Status: aitrend.StreamSuccess})
	})
	router := New(nil, runner)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/job-trends/ai-draft", strings.NewReader(`{"query":"哪些项目"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"tool":"explain_draft"`) {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestAITrendDraftAllowsTrendRankingOperation(t *testing.T) {
	runner := aiTrendRunnerFunc(func(_ context.Context, _ string, emit func(aitrend.StreamEvent) error) error {
		if err := emit(aitrend.StreamEvent{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolRankTrendValues), Params: []byte(`{"dimension":"project","range":"month","metric":"avg_daily_active_jobs","limit":8,"sub_query_id":"projects"}`)}); err != nil {
			return err
		}
		if err := emit(aitrend.StreamEvent{Type: aitrend.StreamEventOperation, Status: aitrend.StreamCompleted, Tool: string(aitrend.ToolRankTrendValues)}); err != nil {
			return err
		}
		return emit(aitrend.StreamEvent{Type: aitrend.StreamEventDone, Status: aitrend.StreamSuccess})
	})
	router := New(nil, runner)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/job-trends/ai-draft", strings.NewReader(`{"query":"哪些项目岗位最多？"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"tool":"rank_trend_values"`) || !strings.Contains(response.Body.String(), `"status":"success"`) {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestAITrendDraftStreamsAssistantMessage(t *testing.T) {
	runner := aiTrendRunnerFunc(func(_ context.Context, _ string, emit func(aitrend.StreamEvent) error) error {
		if err := emit(aitrend.StreamEvent{Type: aitrend.StreamEventAssistant, Status: aitrend.StreamRunning, Delta: "你好，"}); err != nil {
			return err
		}
		if err := emit(aitrend.StreamEvent{Type: aitrend.StreamEventAssistant, Status: aitrend.StreamCompleted}); err != nil {
			return err
		}
		return emit(aitrend.StreamEvent{Type: aitrend.StreamEventDone, Status: aitrend.StreamSuccess})
	})
	router := New(nil, runner)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/job-trends/ai-draft", strings.NewReader(`{"query":"你好"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"type":"assistant_message"`) || !strings.Contains(response.Body.String(), `"delta":"你好，"`) {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}
