package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"mrolescope/internal/jdserving"
)

type jdListReaderFunc func(context.Context, jdserving.ListJobsQuery) (jdserving.JobListResponse, error)

func (function jdListReaderFunc) ListJobs(ctx context.Context, query jdserving.ListJobsQuery) (jdserving.JobListResponse, error) {
	return function(ctx, query)
}

type jdDetailReaderFunc func(context.Context, string) (jdserving.JobDetailResponse, error)

func (function jdDetailReaderFunc) GetJobDetail(ctx context.Context, id string) (jdserving.JobDetailResponse, error) {
	return function(ctx, id)
}

func (function jdDetailReaderFunc) ListJobs(context.Context, jdserving.ListJobsQuery) (jdserving.JobListResponse, error) {
	return jdserving.JobListResponse{}, nil
}

type jdFilterOptionsReaderFunc func(context.Context) (jdserving.FilterOptionsResponse, error)

func (function jdFilterOptionsReaderFunc) ListFilterOptions(ctx context.Context) (jdserving.FilterOptionsResponse, error) {
	return function(ctx)
}

func (function jdFilterOptionsReaderFunc) ListJobs(context.Context, jdserving.ListJobsQuery) (jdserving.JobListResponse, error) {
	return jdserving.JobListResponse{}, nil
}

func TestJDListRoutePassesComposableStructuredFilters(t *testing.T) {
	called := false
	reader := jdListReaderFunc(func(_ context.Context, query jdserving.ListJobsQuery) (jdserving.JobListResponse, error) {
		called = true
		if query.Title != "服务端" || !reflect.DeepEqual(query.MainJobFamilies, []string{"backend", "frontend"}) ||
			!reflect.DeepEqual(query.OtherJobFamilies, []string{"security", "operations_sre"}) ||
			!reflect.DeepEqual(query.TechnologyKeywords, []string{"go", "python"}) ||
			!reflect.DeepEqual(query.Projects, []string{"genshin_impact", "star_rail"}) ||
			!reflect.DeepEqual(query.Locations, []string{"上海", "北京"}) ||
			!reflect.DeepEqual(query.ExperienceYears, []int{3, 5}) ||
			!reflect.DeepEqual(query.EducationLevels, []jdserving.EducationLevel{jdserving.EducationLevelBachelor, jdserving.EducationLevelMaster}) ||
			!reflect.DeepEqual(query.ActivityStates, []jdserving.ActivityState{jdserving.ActivityStateEffective}) ||
			query.IsUrgent == nil || !*query.IsUrgent || query.PageNumber != 2 || query.PageSize != 25 {
			t.Fatalf("parsed list query = %#v", query)
		}
		if query.FirstDiscoveredFrom == nil || query.FirstDiscoveredUntil == nil ||
			query.FirstDiscoveredFrom.Format("2006-01-02") != "2026-10-01" ||
			query.FirstDiscoveredUntil.Format("2006-01-02") != "2026-10-31" {
			t.Fatalf("parsed first-discovered dates = from:%v until:%v", query.FirstDiscoveredFrom, query.FirstDiscoveredUntil)
		}
		return jdserving.JobListResponse{
			Items: []jdserving.JobListItem{{ID: "jd_opaque", Title: "服务端工程师", ActivityState: jdserving.ActivityStateEffective}},
			Page:  jdserving.PageInfo{Number: 2, PageSize: 25, Total: 1},
		}, nil
	})
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/jobs?title=%E6%9C%8D%E5%8A%A1%E7%AB%AF&main_job_family=backend&main_job_family=frontend&other_job_family=security&other_job_family=operations_sre&technology=go&technology=python&education=bachelor&education=master&experience_year=3&experience_year=5&project=genshin_impact&project=star_rail&location=%E4%B8%8A%E6%B5%B7&location=%E5%8C%97%E4%BA%AC&activity_state=effective&first_discovered_from=2026-10-01&first_discovered_until=2026-10-31&urgent=true&page=2&page_size=25", nil)
	response := httptest.NewRecorder()
	New(nil, reader).ServeHTTP(response, request)
	if !called || response.Code != http.StatusOK {
		t.Fatalf("list route = called:%t status:%d", called, response.Code)
	}
	var payload jdserving.JobListResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode typed list response: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].ID != "jd_opaque" || payload.Page.Total != 1 {
		t.Fatalf("typed list response = %#v", payload)
	}
}

func TestJDListRoutePassesUnifiedJobFamiliesAndOrderedSortRules(t *testing.T) {
	reader := jdListReaderFunc(func(_ context.Context, query jdserving.ListJobsQuery) (jdserving.JobListResponse, error) {
		if !reflect.DeepEqual(query.JobFamilies, []string{"backend", "ai_application"}) {
			t.Fatalf("unified job families = %#v", query.JobFamilies)
		}
		wantSortRules := []jdserving.SortRule{
			{Field: jdserving.SortFieldExperience, Direction: jdserving.SortDirectionDescending},
			{Field: jdserving.SortFieldEducation, Direction: jdserving.SortDirectionAscending},
			{Field: jdserving.SortFieldFirstDiscovered, Direction: jdserving.SortDirectionDescending},
			{Field: jdserving.SortFieldUrgent, Direction: jdserving.SortDirectionAscending},
		}
		if !reflect.DeepEqual(query.SortRules, wantSortRules) {
			t.Fatalf("ordered sort rules = %#v, want %#v", query.SortRules, wantSortRules)
		}
		return jdserving.JobListResponse{}, nil
	})
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/jobs?job_family=backend&job_family=ai_application&sort=experience_desc&sort=education_asc&sort=first_discovered_desc&sort=urgent_asc", nil)
	response := httptest.NewRecorder()
	New(nil, reader).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("list route status = %d body:%s", response.Code, response.Body.String())
	}
}

func TestJDListRoutePassesMinimumExperienceThresholdAndExpandedSortRules(t *testing.T) {
	reader := jdListReaderFunc(func(_ context.Context, query jdserving.ListJobsQuery) (jdserving.JobListResponse, error) {
		if query.MinimumExperienceYears == nil || *query.MinimumExperienceYears != 0 {
			t.Fatalf("minimum experience threshold = %#v, want 0", query.MinimumExperienceYears)
		}
		wantSortRules := []jdserving.SortRule{
			{Field: jdserving.SortFieldFirstDiscovered, Direction: jdserving.SortDirectionDescending},
			{Field: jdserving.SortFieldUrgent, Direction: jdserving.SortDirectionDescending},
		}
		if !reflect.DeepEqual(query.SortRules, wantSortRules) {
			t.Fatalf("expanded sort rules = %#v, want %#v", query.SortRules, wantSortRules)
		}
		return jdserving.JobListResponse{}, nil
	})
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/jobs?minimum_experience_year=0&sort=first_discovered_desc&sort=urgent_desc", nil)
	response := httptest.NewRecorder()
	New(nil, reader).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("list route status = %d body:%s", response.Code, response.Body.String())
	}
}

func TestJDDetailRouteReturnsOnlyTypedServingDetail(t *testing.T) {
	reader := jdDetailReaderFunc(func(_ context.Context, id string) (jdserving.JobDetailResponse, error) {
		if id != "jd_opaque" {
			t.Fatalf("detail ID = %q", id)
		}
		return jdserving.JobDetailResponse{
			ID:            id,
			Title:         "服务端工程师",
			ActivityState: jdserving.ActivityStateConfirmedClosed,
			Body: jdserving.BodySections{
				Responsibilities: "负责服务端",
				Requirements:     "熟悉 Go",
			},
		}, nil
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/jd_opaque", nil)
	response := httptest.NewRecorder()
	New(nil, reader).ServeHTTP(response, request)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "supplemental") {
		t.Fatalf("detail response = status:%d body:%s", response.Code, response.Body.String())
	}
	var payload jdserving.JobDetailResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode typed detail response: %v", err)
	}
	if payload.ID != "jd_opaque" || payload.Body.Responsibilities != "负责服务端" || payload.ActivityState != jdserving.ActivityStateConfirmedClosed {
		t.Fatalf("typed detail response = %#v", payload)
	}
}

func TestJDListRouteRejectsInvalidStructuredValues(t *testing.T) {
	reader := jdListReaderFunc(func(context.Context, jdserving.ListJobsQuery) (jdserving.JobListResponse, error) {
		t.Fatal("reader must not receive invalid client input")
		return jdserving.JobListResponse{}, nil
	})
	for _, rawQuery := range []string{
		"education=invalid",
		"page=0",
		"page_size=101",
		"sort=invalid",
		"sort=education_desc&sort=education_asc",
		"sort=title&sort=experience_desc",
		"sort=education_desc&sort=experience_desc&sort=first_discovered_desc&sort=urgent_asc&sort=activity_state_desc",
		"minimum_experience_year=-1",
		"minimum_experience_year=10",
		"minimum_experience_year=three",
		"job_family=backend&main_job_family=backend",
	} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/jobs?"+rawQuery, nil)
		response := httptest.NewRecorder()
		New(nil, reader).ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || response.Body.String() != "{\"error\":\"invalid_query\"}" {
			t.Fatalf("invalid list response for %q = status:%d body:%s", rawQuery, response.Code, response.Body.String())
		}
	}
}

func TestJDListRoutePreservesExplicitAllActivityStates(t *testing.T) {
	reader := jdListReaderFunc(func(_ context.Context, query jdserving.ListJobsQuery) (jdserving.JobListResponse, error) {
		if !query.IncludeAllActivityStates || len(query.ActivityStates) != 0 {
			t.Fatalf("all activity-state query = %#v", query)
		}
		return jdserving.JobListResponse{}, nil
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/jobs?activity_state=all", nil)
	response := httptest.NewRecorder()
	New(nil, reader).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("all activity-state response status = %d", response.Code)
	}
}

func TestJDFilterOptionsRouteReturnsOnlyDisplaySafeOptions(t *testing.T) {
	reader := jdFilterOptionsReaderFunc(func(context.Context) (jdserving.FilterOptionsResponse, error) {
		return jdserving.FilterOptionsResponse{
			ActivityStates:     []jdserving.FilterOption{{Value: "effective", Label: "仍有效"}},
			EducationLevels:    []jdserving.FilterOption{{Value: "bachelor", Label: "本科"}},
			ExperienceYears:    []jdserving.FilterOption{{Value: "3", Label: "3 年"}},
			JobFamilies:        []jdserving.FilterOption{{Value: "backend", Label: "后端"}},
			Locations:          []jdserving.FilterOption{{Value: "上海", Label: "上海"}},
			MainJobFamilies:    []jdserving.FilterOption{{Value: "backend", Label: "后端"}},
			OtherJobFamilies:   []jdserving.FilterOption{},
			Projects:           []jdserving.FilterOption{{Value: "genshin_impact", Label: "原神"}},
			TechnologyKeywords: []jdserving.FilterOption{{Value: "go", Label: "Go"}},
			UrgentOptions:      []jdserving.FilterOption{{Value: "true", Label: "紧急"}},
		}, nil
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/filter-options", nil)
	response := httptest.NewRecorder()
	New(nil, reader).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("filter-options status = %d body:%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "parser") || strings.Contains(response.Body.String(), "audit") || strings.Contains(response.Body.String(), "id\":") {
		t.Fatalf("filter-options leaked internal information: %s", response.Body.String())
	}
	var payload jdserving.FilterOptionsResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode typed filter-options response: %v", err)
	}
	if len(payload.JobFamilies) != 1 || payload.JobFamilies[0].Value != "backend" ||
		len(payload.MainJobFamilies) != 1 || payload.MainJobFamilies[0].Value != "backend" ||
		len(payload.TechnologyKeywords) != 1 {
		t.Fatalf("filter-options payload = %#v", payload)
	}
}

func TestParseOptionalDateUsesCalendarDates(t *testing.T) {
	date, found, err := parseOptionalDate("2026-10-30")
	if err != nil || !found || !date.Equal(time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("parsed date = value:%v found:%t err:%v", date, found, err)
	}
}
