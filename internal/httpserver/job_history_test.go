package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mrolescope/internal/jdserving"
)

type jdHistoryReaderFunc func(context.Context, string) (jdserving.JobHistoryResponse, error)

func (function jdHistoryReaderFunc) GetJobHistory(ctx context.Context, id string) (jdserving.JobHistoryResponse, error) {
	return function(ctx, id)
}

func (function jdHistoryReaderFunc) GetJobHistoryDetail(context.Context, string, time.Time, jdserving.HistoryNodeType) (jdserving.JobDetailResponse, error) {
	return jdserving.JobDetailResponse{}, nil
}

func (jdHistoryReaderFunc) ListJobs(context.Context, jdserving.ListJobsQuery) (jdserving.JobListResponse, error) {
	return jdserving.JobListResponse{}, nil
}

func TestJobHistoryRouteReturnsDisplaySafeTimeline(t *testing.T) {
	reader := jdHistoryReaderFunc(func(_ context.Context, id string) (jdserving.JobHistoryResponse, error) {
		if id != "jd_opaque" {
			t.Fatalf("history ID = %q", id)
		}
		return jdserving.JobHistoryResponse{Nodes: []jdserving.HistoryNode{{
			Date: "2026-11-01", Type: jdserving.HistoryNodeModified,
			Tags:    []jdserving.HistoryNodeTag{jdserving.HistoryNodeTagBodyResponsibilities},
			Summary: "岗位职责", CanViewHistorical: true, ConnectPrevious: true,
		}}}, nil
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/jd_opaque/history", nil)
	response := httptest.NewRecorder()
	New(nil, reader).ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "body_responsibilities") ||
		strings.Contains(response.Body.String(), "run_id") || strings.Contains(response.Body.String(), "parser") {
		t.Fatalf("history response = %d %s", response.Code, response.Body.String())
	}
}

func TestJobHistoryDetailRouteValidatesDateAndNodeType(t *testing.T) {
	called := false
	reader := jdHistoryReaderFunc(func(context.Context, string) (jdserving.JobHistoryResponse, error) {
		return jdserving.JobHistoryResponse{}, nil
	})
	readerDetail := historyDetailReaderWithFunc{history: reader, detail: func(_ context.Context, id string, date time.Time, nodeType jdserving.HistoryNodeType) (jdserving.JobDetailResponse, error) {
		called = true
		if id != "jd_opaque" || date.Format("2006-01-02") != "2026-11-02" || nodeType != jdserving.HistoryNodeModified {
			t.Fatalf("history detail query = id:%q date:%s type:%q", id, date, nodeType)
		}
		return jdserving.JobDetailResponse{ID: id, Title: "历史岗位"}, nil
	}}

	response := httptest.NewRecorder()
	New(nil, readerDetail).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/jobs/jd_opaque/history/detail?date=2026-11-02&type=modified", nil))
	if response.Code != http.StatusOK || !called || !strings.Contains(response.Body.String(), "历史岗位") {
		t.Fatalf("history detail response = %d %s", response.Code, response.Body.String())
	}

	for _, query := range []string{"date=2026-11-02", "date=bad&type=modified", "date=2026-11-02&type=unknown", "date=2026-11-02&type=modified&extra=x"} {
		called = false
		response := httptest.NewRecorder()
		New(nil, readerDetail).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/jobs/jd_opaque/history/detail?"+query, nil))
		if response.Code != http.StatusBadRequest || called {
			t.Fatalf("invalid history detail query %q = %d %s", query, response.Code, response.Body.String())
		}
	}
}

type historyDetailReaderWithFunc struct {
	history jdHistoryReaderFunc
	detail  func(context.Context, string, time.Time, jdserving.HistoryNodeType) (jdserving.JobDetailResponse, error)
}

func (reader historyDetailReaderWithFunc) GetJobHistory(ctx context.Context, id string) (jdserving.JobHistoryResponse, error) {
	return reader.history.GetJobHistory(ctx, id)
}

func (reader historyDetailReaderWithFunc) GetJobHistoryDetail(ctx context.Context, id string, date time.Time, nodeType jdserving.HistoryNodeType) (jdserving.JobDetailResponse, error) {
	return reader.detail(ctx, id, date, nodeType)
}

func (historyDetailReaderWithFunc) ListJobs(context.Context, jdserving.ListJobsQuery) (jdserving.JobListResponse, error) {
	return jdserving.JobListResponse{}, nil
}
