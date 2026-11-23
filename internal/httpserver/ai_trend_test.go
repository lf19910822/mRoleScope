package httpserver

import (
	"context"
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

func (aiTrendRunnerFunc) ListJobs(context.Context, jdserving.ListJobsQuery) (jdserving.JobListResponse, error) {
	return jdserving.JobListResponse{}, nil
}

func TestAITrendDraftStreamsNDJSONOperationEvents(t *testing.T) {
	runner := aiTrendRunnerFunc(func(_ context.Context, query string, emit func(aitrend.StreamEvent) error) error {
		if query != "哪些数据库" {
			return nil
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
	if lines := strings.Count(response.Body.String(), "\n"); lines != 4 {
		t.Fatalf("stream lines = %d body=%s", lines, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"tool":"analyze_intent"`) {
		t.Fatalf("model operation missing: %s", response.Body.String())
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
