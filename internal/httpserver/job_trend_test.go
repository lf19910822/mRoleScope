package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mrolescope/internal/jdserving"
)

type jdTrendReaderFunc func(context.Context, jdserving.TrendQuery) (jdserving.TrendSeries, error)

func (function jdTrendReaderFunc) ReadTrend(ctx context.Context, query jdserving.TrendQuery) (jdserving.TrendSeries, error) {
	return function(ctx, query)
}

func TestJobTrendRouteAcceptsInclusiveCustomDateRange(t *testing.T) {
	reader := jdTrendReaderFunc(func(_ context.Context, query jdserving.TrendQuery) (jdserving.TrendSeries, error) {
		shanghai, err := time.LoadLocation("Asia/Shanghai")
		if err != nil {
			t.Fatal(err)
		}
		wantStart := time.Date(2026, time.October, 1, 0, 0, 0, 0, shanghai)
		wantEnd := time.Date(2026, time.October, 15, 0, 0, 0, 0, shanghai)
		if query.Range != jdserving.TrendRangeCustom || !query.StartDate.Equal(wantStart) || !query.EndDate.Equal(wantEnd) {
			t.Fatalf("custom trend query = %#v, want range and dates", query)
		}
		count := 1
		return jdserving.TrendSeries{Points: []jdserving.TrendPoint{{Date: "2026-10-01", Count: &count}}}, nil
	})
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/job-trends?range=custom&start_date=2026-10-01&end_date=2026-10-15&dimension=location&value=%E4%B8%8A%E6%B5%B7", nil)
	response := httptest.NewRecorder()
	New(nil, reader).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("custom trend route status = %d body:%s", response.Code, response.Body.String())
	}
}

func (jdTrendReaderFunc) ListJobs(context.Context, jdserving.ListJobsQuery) (jdserving.JobListResponse, error) {
	return jdserving.JobListResponse{}, nil
}

type jdTrendDrilldownReaderFunc func(context.Context, jdserving.TrendDrilldownQuery) (jdserving.TrendDrilldownResponse, error)

func (function jdTrendDrilldownReaderFunc) ReadTrendDrilldown(ctx context.Context, query jdserving.TrendDrilldownQuery) (jdserving.TrendDrilldownResponse, error) {
	return function(ctx, query)
}

func (jdTrendDrilldownReaderFunc) ListJobs(context.Context, jdserving.ListJobsQuery) (jdserving.JobListResponse, error) {
	return jdserving.JobListResponse{}, nil
}

type jdTrendEventReaderFunc func(context.Context, jdserving.TrendEventStatsQuery) (jdserving.TrendEventStatsResponse, error)

func (function jdTrendEventReaderFunc) ReadTrendEventStats(ctx context.Context, query jdserving.TrendEventStatsQuery) (jdserving.TrendEventStatsResponse, error) {
	return function(ctx, query)
}

func (jdTrendEventReaderFunc) ListJobs(context.Context, jdserving.ListJobsQuery) (jdserving.JobListResponse, error) {
	return jdserving.JobListResponse{}, nil
}

func TestJobTrendEventsRoutePassesOneBoundedCurveAndReturnsSafeRows(t *testing.T) {
	reader := jdTrendEventReaderFunc(func(_ context.Context, query jdserving.TrendEventStatsQuery) (jdserving.TrendEventStatsResponse, error) {
		if query.Range != jdserving.TrendRangeMonth || query.Condition.Dimension != jdserving.TrendDimensionTechnology || query.Condition.Value != "go" {
			t.Fatalf("event query = %#v", query)
		}
		return jdserving.TrendEventStatsResponse{
			Curve:  jdserving.TrendCurve{Dimension: jdserving.TrendDimensionTechnology, Label: "Go", Value: "go"},
			Status: "available",
			Rows:   []jdserving.TrendEventRow{{Type: jdserving.TrendEventEntered, Current: 1}},
		}, nil
	})
	router := New(nil, reader)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/job-trends/events?range=month&dimension=technology&value=go", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("event route status = %d body:%s", response.Code, response.Body.String())
	}
	for _, forbidden := range []string{
		"job_id", "source_version", "source_metadata", "body_content", "parser", "dictionary",
		"lexicon", "audit", "repair", "credential", "password", "token", "internal_error",
	} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("event response leaked %q: %s", forbidden, response.Body.String())
		}
	}
}

func TestJobTrendEventsRouteMapsReaderFailureToSafeResponse(t *testing.T) {
	reader := jdTrendEventReaderFunc(func(context.Context, jdserving.TrendEventStatsQuery) (jdserving.TrendEventStatsResponse, error) {
		return jdserving.TrendEventStatsResponse{}, errors.New("database password internal_error")
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/job-trends/events?range=month&dimension=technology&value=go", nil)
	response := httptest.NewRecorder()
	New(nil, reader).ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || response.Body.String() != `{"error":"service_unavailable"}` {
		t.Fatalf("event reader failure response = %d %s", response.Code, response.Body.String())
	}
}

func TestJobTrendDrilldownRouteAcceptsPeriodEventQuery(t *testing.T) {
	reader := jdTrendDrilldownReaderFunc(func(_ context.Context, query jdserving.TrendDrilldownQuery) (jdserving.TrendDrilldownResponse, error) {
		if query.EventType != jdserving.TrendEventEntered || query.EventStartDate.Format("2006-01-02") != "2026-10-01" {
			t.Fatalf("event drilldown query = %#v", query)
		}
		return jdserving.TrendDrilldownResponse{Mode: "event", EventType: query.EventType}, nil
	})
	router := New(nil, reader)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/job-trends/drilldown?event=entered&start_date=2026-10-01&end_date=2026-10-15&count=0&dimension=technology&value=go", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("event drilldown route status = %d body:%s", response.Code, response.Body.String())
	}
}

func TestJobTrendDrilldownRouteRejectsUnsupportedPeriodEventDimension(t *testing.T) {
	reader := jdTrendDrilldownReaderFunc(func(context.Context, jdserving.TrendDrilldownQuery) (jdserving.TrendDrilldownResponse, error) {
		t.Fatal("unsupported event dimension must not reach the reader")
		return jdserving.TrendDrilldownResponse{}, nil
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/job-trends/drilldown?event=entered&start_date=2026-10-01&end_date=2026-10-15&count=0&dimension=education&operator=eq&value=bachelor", nil)
	response := httptest.NewRecorder()
	New(nil, reader).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || response.Body.String() != `{"error":"invalid_query"}` {
		t.Fatalf("unsupported event dimension response = %d %s", response.Code, response.Body.String())
	}
}

type jdHistoricalDetailReaderFunc func(context.Context, string, time.Time) (jdserving.JobDetailResponse, error)

func (function jdHistoricalDetailReaderFunc) GetHistoricalJobDetail(ctx context.Context, id string, nodeDate time.Time) (jdserving.JobDetailResponse, error) {
	return function(ctx, id, nodeDate)
}

func (jdHistoricalDetailReaderFunc) GetJobDetail(context.Context, string) (jdserving.JobDetailResponse, error) {
	return jdserving.JobDetailResponse{}, nil
}

func (jdHistoricalDetailReaderFunc) ListJobs(context.Context, jdserving.ListJobsQuery) (jdserving.JobListResponse, error) {
	return jdserving.JobListResponse{}, nil
}

func TestJobDetailRouteSelectsHistoricalNodeDateWithoutLeakingProvenance(t *testing.T) {
	reader := jdHistoricalDetailReaderFunc(func(_ context.Context, id string, nodeDate time.Time) (jdserving.JobDetailResponse, error) {
		if id != "jd_opaque" || nodeDate.Format("2006-01-02") != "2026-10-15" {
			t.Fatalf("historical detail request = id:%q date:%s", id, nodeDate)
		}
		return jdserving.JobDetailResponse{ID: id, Title: "历史平台工程师"}, nil
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/jd_opaque?version=historical&date=2026-10-15", nil)
	response := httptest.NewRecorder()
	New(nil, reader).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("historical detail status = %d body:%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "历史平台工程师") {
		t.Fatalf("historical detail did not use the historical reader: %s", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "version_id") || strings.Contains(response.Body.String(), "run_id") {
		t.Fatalf("historical detail leaked provenance: %s", response.Body.String())
	}
}

func TestJobDetailRouteReturnsDisplaySafeHistoricalComparison(t *testing.T) {
	reader := jdHistoricalDetailReaderFunc(func(context.Context, string, time.Time) (jdserving.JobDetailResponse, error) {
		return jdserving.JobDetailResponse{
			ID:    "jd_opaque",
			Title: "历史平台工程师",
			Comparison: &jdserving.JobComparison{
				Status: jdserving.ComparisonStatusAvailable,
				DimensionChanges: []jdserving.ComparisonDimensionChange{{
					Field: "技术关键词", Historical: "Go", Latest: "Go、Rust",
				}},
			},
		}, nil
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/jd_opaque/detail?version=historical&date=2026-10-15", nil)
	response := httptest.NewRecorder()
	New(nil, reader).ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"comparison"`) ||
		!strings.Contains(response.Body.String(), `"dimension_changes"`) || strings.Contains(response.Body.String(), "body_content_version") {
		t.Fatalf("historical comparison response = %d %s", response.Code, response.Body.String())
	}
}

func TestJobDetailRouteRejectsUnsafeHistoricalSelectors(t *testing.T) {
	reader := jdHistoricalDetailReaderFunc(func(context.Context, string, time.Time) (jdserving.JobDetailResponse, error) {
		t.Fatal("historical reader must not receive an invalid selector")
		return jdserving.JobDetailResponse{}, nil
	})
	for _, query := range []string{
		"version=historical",
		"version=historical&date=not-a-date",
		"version=latest&date=2026-10-15",
		"version=other",
		"version=historical&date=2026-10-15&date=2026-10-16",
	} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/jd_opaque/detail?"+query, nil)
		response := httptest.NewRecorder()
		New(nil, reader).ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || response.Body.String() != "{\"error\":\"invalid_query\"}" {
			t.Fatalf("invalid historical selector %q = %d %s", query, response.Code, response.Body.String())
		}
	}
}

func TestJobTrendDrilldownRoutePassesOneBoundedNodeQuery(t *testing.T) {
	matchesExpected := false
	reader := jdTrendDrilldownReaderFunc(func(_ context.Context, query jdserving.TrendDrilldownQuery) (jdserving.TrendDrilldownResponse, error) {
		if query.Condition != (jdserving.TrendCondition{Dimension: jdserving.TrendDimensionTechnology, Value: "go"}) ||
			query.ExpectedCount != 2 || query.NodeDate.Format("2006-01-02") != "2026-10-15" ||
			query.PageNumber != 2 || query.PageSize != 20 || query.Title != "平台" {
			t.Fatalf("drilldown query = %#v", query)
		}
		return jdserving.TrendDrilldownResponse{
			CountMatchesExpected: &matchesExpected,
			Items:                []jdserving.TrendDrilldownItem{{CanViewLatest: true, ID: "jd_opaque", Status: jdserving.TrendDrilldownStatusCurrent, Title: "平台工程师"}},
			MatchedTotal:         2,
			NodeDate:             "2026-10-15",
			Page:                 jdserving.PageInfo{Number: 2, PageSize: 20, Total: 2},
		}, nil
	})
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/job-trends/drilldown?date=2026-10-15&count=2&page=2&page_size=20&title=%E5%B9%B3%E5%8F%B0&dimension=technology&value=go", nil)
	response := httptest.NewRecorder()
	New(nil, reader).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("drilldown route status = %d body:%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "version") || strings.Contains(response.Body.String(), "run") {
		t.Fatalf("drilldown response leaked implementation details: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"count_matches_expected":false`) {
		t.Fatalf("drilldown response did not include safe node consistency state: %s", response.Body.String())
	}
}

func TestHistoricalDetailUnavailableIsSafeAndNeverFallsBackToLatest(t *testing.T) {
	reader := jdHistoricalDetailReaderFunc(func(context.Context, string, time.Time) (jdserving.JobDetailResponse, error) {
		return jdserving.JobDetailResponse{}, jdserving.ErrHistoricalDetailUnavailable
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/jd_opaque/detail?version=historical&date=2026-10-15", nil)
	response := httptest.NewRecorder()
	New(nil, reader).ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || response.Body.String() != `{"error":"service_unavailable"}` {
		t.Fatalf("historical unavailable response = %d %s", response.Code, response.Body.String())
	}
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

func TestJobTrendEventsRouteReturnsUnavailableStatusWithoutInternalDetails(t *testing.T) {
	reader := jdTrendEventReaderFunc(func(_ context.Context, _ jdserving.TrendEventStatsQuery) (jdserving.TrendEventStatsResponse, error) {
		return jdserving.TrendEventStatsResponse{
			Curve:  jdserving.TrendCurve{Dimension: jdserving.TrendDimensionTechnology, Label: "C++", Value: "c++"},
			Status: "unavailable",
		}, nil
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/job-trends/events?range=month&dimension=technology&value=c%2B%2B", nil)
	response := httptest.NewRecorder()
	New(nil, reader).ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"unavailable"`) {
		t.Fatalf("unavailable event response = %d %s", response.Code, response.Body.String())
	}
	for _, forbidden := range []string{"job_id", "source_metadata", "parser_version", "dictionary_version", "audit", "repair"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("event response leaked %q: %s", forbidden, response.Body.String())
		}
	}
}

func TestJobTrendEventsRouteRejectsUnsupportedAndFutureQueries(t *testing.T) {
	reader := jdTrendEventReaderFunc(func(context.Context, jdserving.TrendEventStatsQuery) (jdserving.TrendEventStatsResponse, error) {
		t.Fatal("event reader must not receive an invalid query")
		return jdserving.TrendEventStatsResponse{}, nil
	})
	for _, rawQuery := range []string{
		"range=month&dimension=education&operator=eq&value=bachelor",
		"range=month&dimension=experience&operator=gte&value=3",
		"range=custom&start_date=2099-01-01&end_date=2099-01-14&dimension=technology&value=go",
		"range=all&dimension=technology&value=go",
	} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/job-trends/events?"+rawQuery, nil)
		response := httptest.NewRecorder()
		New(nil, reader).ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || response.Body.String() != "{\"error\":\"invalid_query\"}" {
			t.Fatalf("invalid event response for %q = status:%d body:%s", rawQuery, response.Code, response.Body.String())
		}
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
		"range=custom&start_date=2026-10-01&dimension=location&value=%E4%B8%8A%E6%B5%B7",
		"range=custom&start_date=2026-10-01&start_date=2026-10-02&end_date=2026-10-15&dimension=location&value=%E4%B8%8A%E6%B5%B7",
		"range=custom&start_date=2026-10-15&end_date=2026-10-01&dimension=location&value=%E4%B8%8A%E6%B5%B7",
		"range=custom&start_date=not-a-date&end_date=2026-10-15&dimension=location&value=%E4%B8%8A%E6%B5%B7",
		"range=month&start_date=2026-10-01&end_date=2026-10-15&dimension=location&value=%E4%B8%8A%E6%B5%B7",
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
