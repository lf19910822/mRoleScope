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

// JDHistoricalDetailReader exposes a selected historical JD version without
// letting HTTP inspect source-version or run identities.
type JDHistoricalDetailReader interface {
	GetHistoricalJobDetail(context.Context, string, time.Time) (jdserving.JobDetailResponse, error)
}

// JDHistoryReader owns the derived per-job timeline and adjacent-node detail
// selection. HTTP only validates the opaque ID and bounded date/type query.
type JDHistoryReader interface {
	GetJobHistory(context.Context, string) (jdserving.JobHistoryResponse, error)
	GetJobHistoryDetail(context.Context, string, time.Time, jdserving.HistoryNodeType) (jdserving.JobDetailResponse, error)
}

// JDFilterOptionsReader supplies the display-safe structured filter vocabulary.
// It deliberately does not expose source, provenance, or audit reads.
type JDFilterOptionsReader interface {
	ListFilterOptions(context.Context) (jdserving.FilterOptionsResponse, error)
}

// JDTrendReader is the only historical-curve dependency the HTTP layer may
// use. Snapshot reconstruction remains behind the Serving seam.
type JDTrendReader interface {
	ReadTrend(context.Context, jdserving.TrendQuery) (jdserving.TrendSeries, error)
}

// JDTrendDrilldownReader keeps node snapshot reconstruction behind the
// Serving seam; HTTP only validates bounded display-safe query parameters.
type JDTrendDrilldownReader interface {
	ReadTrendDrilldown(context.Context, jdserving.TrendDrilldownQuery) (jdserving.TrendDrilldownResponse, error)
}

type Router struct {
	readiness          ReadinessChecker
	jdDetail           JDDetailReader
	jdHistoricalDetail JDHistoricalDetailReader
	jdHistory          JDHistoryReader
	jdFilters          JDFilterOptionsReader
	jdList             JDListReader
	jdDrilldown        JDTrendDrilldownReader
	jdTrend            JDTrendReader
}

func New(readiness ReadinessChecker, readers ...JDListReader) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	server := Router{readiness: readiness}
	if len(readers) > 0 {
		server.jdList = readers[0]
		server.jdDetail, _ = readers[0].(JDDetailReader)
		server.jdHistoricalDetail, _ = readers[0].(JDHistoricalDetailReader)
		server.jdHistory, _ = readers[0].(JDHistoryReader)
		server.jdFilters, _ = readers[0].(JDFilterOptionsReader)
		server.jdDrilldown, _ = readers[0].(JDTrendDrilldownReader)
		server.jdTrend, _ = readers[0].(JDTrendReader)
	}
	router.GET("/healthz", server.health)
	router.GET("/readyz", server.ready)
	router.GET("/api/v1/jobs", server.listJobs)
	router.GET("/api/v1/jobs/filter-options", server.filterOptions)
	router.GET("/api/v1/jobs/:id", server.jobDetail)
	router.GET("/api/v1/jobs/:id/detail", server.jobDetail)
	router.GET("/api/v1/jobs/:id/history", server.jobHistory)
	router.GET("/api/v1/jobs/:id/history/detail", server.jobHistoryDetail)
	router.GET("/api/v1/job-trends", server.jobTrend)
	router.GET("/api/v1/job-trends/drilldown", server.jobTrendDrilldown)
	return router
}

func (r Router) jobHistory(context *gin.Context) {
	if r.jdHistory == nil {
		context.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}
	if len(context.Request.URL.Query()) != 0 {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid_query"})
		return
	}
	response, err := r.jdHistory.GetJobHistory(context.Request.Context(), context.Param("id"))
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

func (r Router) jobHistoryDetail(context *gin.Context) {
	if r.jdHistory == nil {
		context.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}
	nodeDate, nodeType, err := parseHistoryDetailQuery(context)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid_query"})
		return
	}
	response, err := r.jdHistory.GetJobHistoryDetail(context.Request.Context(), context.Param("id"), nodeDate, nodeType)
	if errors.Is(err, jdserving.ErrJobNotFound) {
		context.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	if errors.Is(err, jdserving.ErrInvalidTrendQuery) {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid_query"})
		return
	}
	if errors.Is(err, jdserving.ErrHistoricalDetailUnavailable) {
		context.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}
	if err != nil {
		context.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}
	context.JSON(http.StatusOK, response)
}

func (r Router) jobTrendDrilldown(context *gin.Context) {
	if r.jdDrilldown == nil {
		context.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}
	query, err := parseTrendDrilldownQuery(context)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid_query"})
		return
	}
	response, err := r.jdDrilldown.ReadTrendDrilldown(context.Request.Context(), query)
	if errors.Is(err, jdserving.ErrInvalidTrendQuery) {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid_query"})
		return
	}
	if err != nil {
		context.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}
	context.JSON(http.StatusOK, response)
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
	version, nodeDate, err := parseDetailVersion(context)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid_query"})
		return
	}
	var response jdserving.JobDetailResponse
	if version == "historical" {
		if r.jdHistoricalDetail == nil {
			context.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
			return
		}
		response, err = r.jdHistoricalDetail.GetHistoricalJobDetail(context.Request.Context(), context.Param("id"), nodeDate)
	} else {
		response, err = r.jdDetail.GetJobDetail(context.Request.Context(), context.Param("id"))
	}
	if errors.Is(err, jdserving.ErrJobNotFound) {
		context.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	if errors.Is(err, jdserving.ErrInvalidTrendQuery) {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid_query"})
		return
	}
	if err != nil {
		context.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}
	context.JSON(http.StatusOK, response)
}

func parseDetailVersion(context *gin.Context) (string, time.Time, error) {
	values := context.Request.URL.Query()
	for key := range values {
		if key != "version" && key != "date" {
			return "", time.Time{}, strconv.ErrSyntax
		}
	}
	version := values["version"]
	if len(version) == 0 {
		if len(values["date"]) != 0 {
			return "", time.Time{}, strconv.ErrSyntax
		}
		return "latest", time.Time{}, nil
	}
	if len(version) != 1 || len(values["date"]) > 1 {
		return "", time.Time{}, strconv.ErrSyntax
	}
	switch strings.TrimSpace(version[0]) {
	case "latest":
		if len(values["date"]) != 0 {
			return "", time.Time{}, strconv.ErrSyntax
		}
		return "latest", time.Time{}, nil
	case "historical":
		if len(values["date"]) != 1 {
			return "", time.Time{}, strconv.ErrSyntax
		}
		shanghai, err := time.LoadLocation("Asia/Shanghai")
		if err != nil {
			return "", time.Time{}, strconv.ErrSyntax
		}
		nodeDate, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(values["date"][0]), shanghai)
		if err != nil {
			return "", time.Time{}, strconv.ErrSyntax
		}
		today := time.Now().In(shanghai)
		if nodeDate.After(time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, shanghai)) {
			return "", time.Time{}, strconv.ErrSyntax
		}
		return "historical", nodeDate, nil
	default:
		return "", time.Time{}, strconv.ErrSyntax
	}
}

func parseHistoryDetailQuery(context *gin.Context) (time.Time, jdserving.HistoryNodeType, error) {
	values := context.Request.URL.Query()
	for key := range values {
		if key != "date" && key != "type" {
			return time.Time{}, "", strconv.ErrSyntax
		}
	}
	if len(values["date"]) != 1 || len(values["type"]) != 1 {
		return time.Time{}, "", strconv.ErrSyntax
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Time{}, "", strconv.ErrSyntax
	}
	date, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(values["date"][0]), location)
	if err != nil {
		return time.Time{}, "", strconv.ErrSyntax
	}
	today := time.Now().In(location)
	todayStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, location)
	if date.After(todayStart) {
		return time.Time{}, "", strconv.ErrSyntax
	}
	nodeType := jdserving.HistoryNodeType(strings.TrimSpace(values["type"][0]))
	switch nodeType {
	case jdserving.HistoryNodeDiscovered, jdserving.HistoryNodeModified,
		jdserving.HistoryNodeConfirmedClosed, jdserving.HistoryNodeReopened, jdserving.HistoryNodeLatest:
		return date, nodeType, nil
	default:
		return time.Time{}, "", strconv.ErrSyntax
	}
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

func (r Router) jobTrend(context *gin.Context) {
	if r.jdTrend == nil {
		context.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}
	query, err := parseTrendQuery(context)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid_query"})
		return
	}
	response, err := r.jdTrend.ReadTrend(context.Request.Context(), query)
	if errors.Is(err, jdserving.ErrInvalidTrendQuery) {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid_query"})
		return
	}
	if err != nil {
		context.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}
	context.JSON(http.StatusOK, response)
}

func parseTrendQuery(context *gin.Context) (jdserving.TrendQuery, error) {
	values := context.Request.URL.Query()
	for key := range values {
		switch key {
		case "range", "dimension", "value", "operator", "include_secondary", "start_date", "end_date":
		default:
			return jdserving.TrendQuery{}, strconv.ErrSyntax
		}
	}
	one := func(key string, required bool) (string, bool) {
		items := values[key]
		if len(items) != 1 {
			return "", false
		}
		value := strings.TrimSpace(items[0])
		if required && value == "" {
			return "", false
		}
		return value, true
	}
	rawRange, valid := one("range", true)
	if !valid {
		return jdserving.TrendQuery{}, strconv.ErrSyntax
	}
	rawDimension, valid := one("dimension", true)
	if !valid {
		return jdserving.TrendQuery{}, strconv.ErrSyntax
	}
	value, valid := one("value", true)
	if !valid {
		return jdserving.TrendQuery{}, strconv.ErrSyntax
	}
	query := jdserving.TrendQuery{Range: jdserving.TrendRange(rawRange)}
	switch query.Range {
	case jdserving.TrendRangeWeek, jdserving.TrendRangeMonth, jdserving.TrendRangeQuarter,
		jdserving.TrendRangeYear, jdserving.TrendRangeAll:
		if _, present := values["start_date"]; present {
			return jdserving.TrendQuery{}, strconv.ErrSyntax
		}
		if _, present := values["end_date"]; present {
			return jdserving.TrendQuery{}, strconv.ErrSyntax
		}
	case jdserving.TrendRangeCustom:
		startDate, ok := one("start_date", true)
		if !ok {
			return jdserving.TrendQuery{}, strconv.ErrSyntax
		}
		endDate, ok := one("end_date", true)
		if !ok {
			return jdserving.TrendQuery{}, strconv.ErrSyntax
		}
		shanghai, err := time.LoadLocation("Asia/Shanghai")
		if err != nil {
			return jdserving.TrendQuery{}, strconv.ErrSyntax
		}
		parsedStart, err := time.ParseInLocation("2006-01-02", startDate, shanghai)
		if err != nil {
			return jdserving.TrendQuery{}, strconv.ErrSyntax
		}
		parsedEnd, err := time.ParseInLocation("2006-01-02", endDate, shanghai)
		if err != nil || parsedStart.After(parsedEnd) {
			return jdserving.TrendQuery{}, strconv.ErrSyntax
		}
		query.StartDate, query.EndDate = parsedStart, parsedEnd
	default:
		return jdserving.TrendQuery{}, strconv.ErrSyntax
	}
	query.Condition.Dimension = jdserving.TrendDimension(rawDimension)
	query.Condition.Value = value
	switch query.Condition.Dimension {
	case jdserving.TrendDimensionEducation, jdserving.TrendDimensionExperience:
		rawOperator, present := one("operator", true)
		if !present {
			return jdserving.TrendQuery{}, strconv.ErrSyntax
		}
		query.Condition.Operator = jdserving.TrendComparison(rawOperator)
		switch query.Condition.Operator {
		case jdserving.TrendComparisonEqual, jdserving.TrendComparisonGreaterThan,
			jdserving.TrendComparisonGreaterThanOrEqual, jdserving.TrendComparisonLessThan,
			jdserving.TrendComparisonLessThanOrEqual:
		default:
			return jdserving.TrendQuery{}, strconv.ErrSyntax
		}
		if _, present := values["include_secondary"]; present {
			return jdserving.TrendQuery{}, strconv.ErrSyntax
		}
	case jdserving.TrendDimensionJobFamily:
		if _, present := values["operator"]; present {
			return jdserving.TrendQuery{}, strconv.ErrSyntax
		}
		if rawSecondary, present := one("include_secondary", false); present {
			if rawSecondary != "true" && rawSecondary != "false" {
				return jdserving.TrendQuery{}, strconv.ErrSyntax
			}
			query.Condition.IncludeSecondary = rawSecondary == "true"
		} else if _, supplied := values["include_secondary"]; supplied {
			return jdserving.TrendQuery{}, strconv.ErrSyntax
		}
	case jdserving.TrendDimensionTechnology, jdserving.TrendDimensionProject, jdserving.TrendDimensionLocation:
		if _, present := values["operator"]; present {
			return jdserving.TrendQuery{}, strconv.ErrSyntax
		}
		if _, present := values["include_secondary"]; present {
			return jdserving.TrendQuery{}, strconv.ErrSyntax
		}
	default:
		return jdserving.TrendQuery{}, strconv.ErrSyntax
	}
	return query, nil
}

func parseTrendDrilldownQuery(context *gin.Context) (jdserving.TrendDrilldownQuery, error) {
	values := context.Request.URL.Query()
	for key := range values {
		switch key {
		case "date", "count", "page", "page_size", "title", "dimension", "value", "operator", "include_secondary":
		default:
			return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
		}
	}
	one := func(key string, required bool) (string, bool) {
		items := values[key]
		if len(items) != 1 {
			return "", false
		}
		value := strings.TrimSpace(items[0])
		if required && value == "" {
			return "", false
		}
		return value, true
	}
	date, ok := one("date", true)
	if !ok {
		return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
	}
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
	}
	nodeDate, err := time.ParseInLocation("2006-01-02", date, shanghai)
	if err != nil {
		return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
	}
	today := time.Now().In(shanghai)
	if nodeDate.After(time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, shanghai)) {
		return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
	}
	count, ok := one("count", true)
	if !ok {
		return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
	}
	expectedCount, err := strconv.Atoi(count)
	if err != nil || expectedCount < 0 {
		return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
	}
	pageNumber, pageSize := 1, jdserving.DefaultPageSize()
	if raw, present := one("page", false); present {
		pageNumber, err = strconv.Atoi(raw)
		if err != nil || pageNumber < 1 {
			return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
		}
	}
	if raw, present := one("page_size", false); present {
		pageSize, err = strconv.Atoi(raw)
		if err != nil || pageSize < 1 || pageSize > jdserving.MaximumPageSize {
			return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
		}
	}
	dimension, ok := one("dimension", true)
	if !ok {
		return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
	}
	value, ok := one("value", true)
	if !ok {
		return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
	}
	condition := jdserving.TrendCondition{Dimension: jdserving.TrendDimension(dimension), Value: value}
	switch condition.Dimension {
	case jdserving.TrendDimensionEducation, jdserving.TrendDimensionExperience:
		rawOperator, present := one("operator", true)
		if !present {
			return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
		}
		condition.Operator = jdserving.TrendComparison(rawOperator)
		if condition.Operator != jdserving.TrendComparisonEqual && condition.Operator != jdserving.TrendComparisonGreaterThan && condition.Operator != jdserving.TrendComparisonGreaterThanOrEqual && condition.Operator != jdserving.TrendComparisonLessThan && condition.Operator != jdserving.TrendComparisonLessThanOrEqual {
			return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
		}
		if _, present := values["include_secondary"]; present {
			return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
		}
	case jdserving.TrendDimensionJobFamily:
		if _, present := values["operator"]; present {
			return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
		}
		if rawSecondary, present := one("include_secondary", false); present {
			if rawSecondary != "true" && rawSecondary != "false" {
				return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
			}
			condition.IncludeSecondary = rawSecondary == "true"
		} else if _, supplied := values["include_secondary"]; supplied {
			return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
		}
	case jdserving.TrendDimensionTechnology, jdserving.TrendDimensionProject, jdserving.TrendDimensionLocation:
		if _, present := values["operator"]; present {
			return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
		}
		if _, present := values["include_secondary"]; present {
			return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
		}
	default:
		return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
	}
	title := strings.TrimSpace(values.Get("title"))
	if len(values["title"]) > 1 {
		return jdserving.TrendDrilldownQuery{}, strconv.ErrSyntax
	}
	return jdserving.TrendDrilldownQuery{Condition: condition, ExpectedCount: expectedCount, NodeDate: nodeDate, PageNumber: pageNumber, PageSize: pageSize, Title: title}, nil
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
