package httpserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"mrolescope/internal/jdserving"
)

// ReadinessChecker is intentionally narrow so route tests do not need a live database.
type ReadinessChecker interface {
	Ping(context.Context) error
}

// JDListReader is the narrow read boundary required by the public list route.
// It keeps HTTP tests independent from PostgreSQL and prevents handlers from
// joining source or analysis tables directly.
type JDListReader interface {
	ListJobs(context.Context, jdserving.ListJobsQuery) (jdserving.JobListResponse, error)
}

// JDDetailReader is the public detail-read boundary. It does not give the
// HTTP layer access to source, historical, or audit records.
type JDDetailReader interface {
	GetJobDetail(context.Context, string) (jdserving.JobDetailResponse, error)
}

// JDFilterOptionsReader supplies the display-safe structured filter vocabulary.
// It deliberately does not expose source, provenance, or audit reads.
type JDFilterOptionsReader interface {
	ListFilterOptions(context.Context) (jdserving.FilterOptionsResponse, error)
}

type Router struct {
	readiness ReadinessChecker
	jdDetail  JDDetailReader
	jdFilters JDFilterOptionsReader
	jdList    JDListReader
}

func New(readiness ReadinessChecker, readers ...JDListReader) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	server := Router{readiness: readiness}
	if len(readers) > 0 {
		server.jdList = readers[0]
		server.jdDetail, _ = readers[0].(JDDetailReader)
		server.jdFilters, _ = readers[0].(JDFilterOptionsReader)
	}
	router.GET("/healthz", server.health)
	router.GET("/readyz", server.ready)
	router.GET("/api/v1/jobs", server.listJobs)
	router.GET("/api/v1/jobs/filter-options", server.filterOptions)
	router.GET("/api/v1/jobs/:id", server.jobDetail)
	return router
}

func (r Router) health(context *gin.Context) {
	context.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (r Router) ready(context *gin.Context) {
	if r.readiness == nil || r.readiness.Ping(context.Request.Context()) != nil {
		context.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
		return
	}
	context.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (r Router) listJobs(context *gin.Context) {
	if r.jdList == nil {
		context.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}
	query, err := parseListJobsQuery(context)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid_query"})
		return
	}
	response, err := r.jdList.ListJobs(context.Request.Context(), query)
	if err != nil {
		context.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}
	context.JSON(http.StatusOK, response)
}

func (r Router) jobDetail(context *gin.Context) {
	if r.jdDetail == nil {
		context.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}
	response, err := r.jdDetail.GetJobDetail(context.Request.Context(), context.Param("id"))
	if errors.Is(err, jdserving.ErrJobNotFound) {
		context.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	if err != nil {
		context.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}
	context.JSON(http.StatusOK, response)
}

func (r Router) filterOptions(context *gin.Context) {
	if r.jdFilters == nil {
		context.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}
	response, err := r.jdFilters.ListFilterOptions(context.Request.Context())
	if err != nil {
		context.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}
	context.JSON(http.StatusOK, response)
}

func parseListJobsQuery(context *gin.Context) (jdserving.ListJobsQuery, error) {
	values := context.Request.URL.Query()
	query := jdserving.ListJobsQuery{
		Locations:          values["location"],
		JobFamilies:        values["job_family"],
		MainJobFamilies:    values["main_job_family"],
		OtherJobFamilies:   values["other_job_family"],
		Projects:           values["project"],
		TechnologyKeywords: values["technology"],
		Title:              strings.TrimSpace(values.Get("title")),
	}
	if len(query.JobFamilies) > 0 && (len(query.MainJobFamilies) > 0 || len(query.OtherJobFamilies) > 0) {
		return jdserving.ListJobsQuery{}, strconv.ErrSyntax
	}
	for _, state := range values["activity_state"] {
		switch jdserving.ActivityState(state) {
		case jdserving.ActivityStateEffective, jdserving.ActivityStateConfirmedClosed:
			query.ActivityStates = append(query.ActivityStates, jdserving.ActivityState(state))
		case "all":
			query.IncludeAllActivityStates = true
		default:
			return jdserving.ListJobsQuery{}, strconv.ErrSyntax
		}
	}
	if query.IncludeAllActivityStates && len(query.ActivityStates) > 0 {
		return jdserving.ListJobsQuery{}, strconv.ErrSyntax
	}
	for _, level := range values["education"] {
		switch jdserving.EducationLevel(level) {
		case jdserving.EducationLevelAssociateOrLower, jdserving.EducationLevelBachelor,
			jdserving.EducationLevelMaster, jdserving.EducationLevelDoctor:
			query.EducationLevels = append(query.EducationLevels, jdserving.EducationLevel(level))
		default:
			return jdserving.ListJobsQuery{}, strconv.ErrSyntax
		}
	}
	for _, value := range values["experience_year"] {
		years, err := strconv.Atoi(value)
		if err != nil || years < 1 {
			return jdserving.ListJobsQuery{}, strconv.ErrSyntax
		}
		query.ExperienceYears = append(query.ExperienceYears, years)
	}
	if minimumExperienceYears := values["minimum_experience_year"]; len(minimumExperienceYears) > 0 {
		if len(minimumExperienceYears) != 1 {
			return jdserving.ListJobsQuery{}, strconv.ErrSyntax
		}
		years, err := strconv.Atoi(minimumExperienceYears[0])
		if err != nil || years < 0 || years > 9 {
			return jdserving.ListJobsQuery{}, strconv.ErrSyntax
		}
		query.MinimumExperienceYears = &years
	}
	if date, found, err := parseOptionalDate(values.Get("first_discovered_from")); err != nil {
		return jdserving.ListJobsQuery{}, err
	} else if found {
		query.FirstDiscoveredFrom = &date
	}
	if date, found, err := parseOptionalDate(values.Get("first_discovered_until")); err != nil {
		return jdserving.ListJobsQuery{}, err
	} else if found {
		query.FirstDiscoveredUntil = &date
	}
	if query.FirstDiscoveredFrom != nil && query.FirstDiscoveredUntil != nil && query.FirstDiscoveredFrom.After(*query.FirstDiscoveredUntil) {
		return jdserving.ListJobsQuery{}, strconv.ErrSyntax
	}
	if rawUrgent := values.Get("urgent"); rawUrgent != "" {
		urgent, err := strconv.ParseBool(rawUrgent)
		if err != nil {
			return jdserving.ListJobsQuery{}, strconv.ErrSyntax
		}
		query.IsUrgent = &urgent
	}
	if rawPage := values.Get("page"); rawPage != "" {
		page, err := strconv.Atoi(rawPage)
		if err != nil || page < 1 {
			return jdserving.ListJobsQuery{}, strconv.ErrSyntax
		}
		query.PageNumber = page
	}
	if rawPageSize := values.Get("page_size"); rawPageSize != "" {
		pageSize, err := strconv.Atoi(rawPageSize)
		if err != nil || pageSize < 1 || pageSize > jdserving.MaximumPageSize {
			return jdserving.ListJobsQuery{}, strconv.ErrSyntax
		}
		query.PageSize = pageSize
	}
	if sortValues := values["sort"]; len(sortValues) > 0 {
		sortRules, legacySort, err := jdserving.ParseListSortRules(sortValues)
		if err != nil {
			return jdserving.ListJobsQuery{}, strconv.ErrSyntax
		}
		query.SortRules = sortRules
		query.Sort = legacySort
	}
	return query, nil
}

func parseOptionalDate(value string) (time.Time, bool, error) {
	if value == "" {
		return time.Time{}, false, nil
	}
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, false, err
	}
	return date, true, nil
}
