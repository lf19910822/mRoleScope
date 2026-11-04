package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mrolescope/internal/jdserving"
)

type jdTrendReaderFunc func(context.Context, jdserving.TrendQuery) (jdserving.TrendSeries, error)

func (function jdTrendReaderFunc) ReadTrend(ctx context.Context, query jdserving.TrendQuery) (jdserving.TrendSeries, error) {
	return function(ctx, query)
}

func (jdTrendReaderFunc) ListJobs(context.Context, jdserving.ListJobsQuery) (jdserving.JobListResponse, error) {
	return jdserving.JobListResponse{}, nil
}

func TestJobTrendRoutePassesOneValidatedCurveAndReturnsOnlySafeFields(t *testing.T) {
	reader := jdTrendReaderFunc(func(_ context.Context, query jdserving.TrendQuery) (jdserving.TrendSeries, error) {
		if query.Range != jdserving.TrendRangeMonth || query.Condition != (jdserving.TrendCondition{
			Dimension:        jdserving.TrendDimensionJobFamily,
			IncludeSecondary: true,
			Value:            "client",
		}) {
			t.Fatalf("trend query = %#v", query)
		}
		count := 2
		return jdserving.TrendSeries{
			Curve: jdserving.TrendCurve{
				Dimension:        jdserving.TrendDimensionJobFamily,
				IncludeSecondary: true,
				Label:            "游戏客户端",
				Value:            "client",
			},
			Freshness: "opaque-safe-marker",
			Points: []jdserving.TrendPoint{
				{Date: "2026-11-03", Count: &count},
				{Date: "2026-11-04", Count: nil},
			},
		}, nil
	})
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/job-trends?range=month&dimension=job_family&value=client&include_secondary=true", nil)
	response := httptest.NewRecorder()
	New(nil, reader).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("trend route status = %d body:%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "parser") || strings.Contains(response.Body.String(), "dictionary") ||
		strings.Contains(response.Body.String(), "audit") || strings.Contains(response.Body.String(), "job_id") {
		t.Fatalf("trend response leaked internal data: %s", response.Body.String())
	}
	var payload jdserving.TrendSeries
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode trend response: %v", err)
	}
	if payload.Freshness != "opaque-safe-marker" || len(payload.Points) != 2 || payload.Points[0].Count == nil ||
		*payload.Points[0].Count != 2 || payload.Points[1].Count != nil {
		t.Fatalf("trend response = %#v", payload)
	}
}

func TestJobTrendRouteRejectsInvalidOrIncompatibleQueryShapes(t *testing.T) {
	reader := jdTrendReaderFunc(func(context.Context, jdserving.TrendQuery) (jdserving.TrendSeries, error) {
		t.Fatal("trend reader must not receive an invalid query")
		return jdserving.TrendSeries{}, nil
	})
	for _, rawQuery := range []string{
		"range=week&dimension=education&value=bachelor",
		"range=week&dimension=location&value=%E4%B8%8A%E6%B5%B7&operator=eq",
		"range=week&dimension=technology&value=go&include_secondary=true",
		"range=week&dimension=job_family&value=backend&include_secondary=not-a-bool",
		"range=week&dimension=location&value=%E4%B8%8A%E6%B5%B7&value=%E5%8C%97%E4%BA%AC",
		"range=week&dimension=unknown&value=value",
		"range=decade&dimension=location&value=%E4%B8%8A%E6%B5%B7",
		"range=week&dimension=location&value=%E4%B8%8A%E6%B5%B7&unexpected=value",
	} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/job-trends?"+rawQuery, nil)
		response := httptest.NewRecorder()
		New(nil, reader).ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || response.Body.String() != "{\"error\":\"invalid_query\"}" {
			t.Fatalf("invalid trend response for %q = status:%d body:%s", rawQuery, response.Code, response.Body.String())
		}
	}
}

func TestJobTrendRouteMapsInvalidAndUnavailableReaderErrorsSafely(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{name: "invalid", err: jdserving.ErrInvalidTrendQuery, wantStatus: http.StatusBadRequest, wantBody: "{\"error\":\"invalid_query\"}"},
		{name: "unavailable", err: errors.New("database unavailable"), wantStatus: http.StatusServiceUnavailable, wantBody: "{\"error\":\"service_unavailable\"}"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := jdTrendReaderFunc(func(context.Context, jdserving.TrendQuery) (jdserving.TrendSeries, error) {
				return jdserving.TrendSeries{}, test.err
			})
			request := httptest.NewRequest(http.MethodGet, "/api/v1/job-trends?range=week&dimension=location&value=%E4%B8%8A%E6%B5%B7", nil)
			response := httptest.NewRecorder()
			New(nil, reader).ServeHTTP(response, request)
			if response.Code != test.wantStatus || response.Body.String() != test.wantBody {
				t.Fatalf("trend error response = status:%d body:%s", response.Code, response.Body.String())
			}
		})
	}
}
