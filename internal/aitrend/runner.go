package aitrend

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"mrolescope/internal/jdserving"
)

// TrendRunner connects the validated intent flow to Serving-owned readers.
// It deliberately builds only formal-value conditions and lets the existing
// dashboard remain the owner of curve points and application semantics.
type TrendRunner struct {
	Provider    IntentProvider
	Tools       ServingToolService
	Audit       *AuditStore
	MaxDuration time.Duration
}

func NewTrendRunner(provider IntentProvider, options FilterOptionsReader, summary jdserving.TrendSummaryReader, events TrendEventReader) (*TrendRunner, error) {
	if provider == nil || options == nil || summary == nil {
		return nil, &ContractError{Code: ErrorConfigurationFailure, Message: "trend AI is not configured"}
	}
	return &TrendRunner{Provider: provider, Tools: ServingToolService{Options: options, Summary: summary, Events: events}}, nil
}

func (runner *TrendRunner) Run(ctx context.Context, query string, emit func(StreamEvent) error) error {
	if runner == nil || runner.Provider == nil || runner.Tools.Options == nil || runner.Tools.Summary == nil {
		return &ContractError{Code: ErrorConfigurationFailure, Message: "trend AI is not configured"}
	}
	options, err := runner.Tools.Options.ListFilterOptions(ctx)
	if err != nil {
		return &ContractError{Code: ErrorBackendFailure, Message: "读取正式筛选值失败"}
	}
	resolver := resolverFromOptions(options)
	tools := runner.Tools
	tools.Resolver = resolver
	registry, err := tools.Registry()
	if err != nil {
		return err
	}

	var groups []CandidateGroup
	var plannedIntent Intent
	inputTokens, outputTokens := 0, 0
	auditOperations := make([]auditOperation, 0, MaxToolCalls+1)
	auditStarted := time.Now()
	streamEmit := func(event StreamEvent) error {
		if event.Type == StreamEventOperation {
			auditOperations = appendAuditOperation(auditOperations, event)
		}
		return emit(event)
	}
	orchestrator := Orchestrator{
		Analyzer: func(analyzeContext context.Context, correction *ContractError) ([]byte, error) {
			if usageProvider, ok := runner.Provider.(IntentUsageProvider); ok {
				content, input, output, providerErr := usageProvider.AnalyzeIntentUsage(analyzeContext, query, correction)
				inputTokens += input
				outputTokens += output
				return content, providerErr
			}
			return runner.Provider.AnalyzeIntent(analyzeContext, query, correction)
		},
		Planner: func(_ context.Context, intent Intent, _ []ToolCallResult) ([]ToolCall, error) {
			intent = normalizeIntentForQuery(query, intent)
			plannedIntent = intent
			groups = candidateGroups(intent, resolver)
			return planServingTools(intent, groups), nil
		},
		Registry:        registry,
		MaxDuration:     runner.MaxDuration,
		MaxInputTokens:  12_000,
		MaxOutputTokens: 16_000,
		Emit:            streamEmit,
	}
	result, err := orchestrator.RunWithInput(ctx, query)
	if err != nil {
		runner.persistAudit(ctx, query, result, nil, inputTokens, outputTokens, auditOperations, auditStarted, err)
		return err
	}
	if plannedIntent.ContractVersion == "" {
		plannedIntent = result.Intent
	}
	applySummaryScores(groups, result.ToolResults, plannedIntent)
	filterFailedSummaryCandidates(groups, result.ToolResults)

	partialFailure := result.Partial
	warnings := make([]string, 0)
	for _, toolResult := range result.ToolResults {
		if toolResult.SafeError != nil {
			partialFailure = true
			break
		}
	}
	if unsupportedMetric(plannedIntent) {
		warnings = append(warnings, "部分比较指标暂不直接作为纵轴，已按岗位数量生成曲线。")
	}
	statusStopped := result.Stopped
	draft, err := BuildDraft(groups, DraftBuildOptions{
		Warnings:       warnings,
		Explanation:    draftExplanation(plannedIntent),
		Stopped:        statusStopped,
		PartialFailure: partialFailure,
	})
	if err != nil {
		return err
	}
	if len(draft.Curves) > 0 {
		if err := emitValidatedDraft(ctx, registry, draft, streamEmit); err != nil {
			return err
		}
	}
	if !result.Stopped {
		if explanationProvider, ok := runner.Provider.(IntentExplanationUsageProvider); ok {
			if err := streamEmit(StreamEvent{Type: StreamEventOperation, Status: StreamRunning, Tool: string(ToolExplainDraft)}); err != nil {
				runner.persistAudit(ctx, query, result, &draft, inputTokens, outputTokens, auditOperations, auditStarted, err)
				return err
			}
			explanation, input, output, explanationErr := explanationProvider.ExplainDraftUsage(ctx, query, buildDraftExplanationContext(plannedIntent, draft, groups, result.ToolResults))
			inputTokens += input
			outputTokens += output
			if explanationErr != nil {
				if err := streamEmit(StreamEvent{Type: StreamEventOperation, Status: StreamFailed, Tool: string(ToolExplainDraft), Error: safeErrorFrom(explanationErr)}); err != nil {
					runner.persistAudit(ctx, query, result, &draft, inputTokens, outputTokens, auditOperations, auditStarted, err)
					return err
				}
			} else {
				draft.Explanation = explanation
				if err := streamEmit(StreamEvent{Type: StreamEventOperation, Status: StreamCompleted, Tool: string(ToolExplainDraft)}); err != nil {
					runner.persistAudit(ctx, query, result, &draft, inputTokens, outputTokens, auditOperations, auditStarted, err)
					return err
				}
			}
		}
	}
	draftStatus := StreamCompleted
	terminalStatus := StreamSuccess
	if result.Stopped {
		draftStatus = StreamStopped
		terminalStatus = StreamStopped
	} else if draft.Status == DraftPartial {
		draftStatus = StreamPartial
		terminalStatus = StreamPartial
	}
	if err := streamEmit(StreamEvent{Type: StreamEventDraft, Status: draftStatus, Draft: &draft}); err != nil {
		runner.persistAudit(ctx, query, result, &draft, inputTokens, outputTokens, auditOperations, auditStarted, err)
		return err
	}
	err = streamEmit(StreamEvent{Type: StreamEventDone, Status: terminalStatus})
	runner.persistAudit(ctx, query, result, &draft, inputTokens, outputTokens, auditOperations, auditStarted, err)
	return err
}

func normalizeIntentForQuery(query string, intent Intent) Intent {
	category := technologyCategoryFromQuery(query)
	if category == "" {
		return intent
	}
	normalized := intent
	normalized.SubQueries = append([]SubQuery(nil), intent.SubQueries...)
	for index := range normalized.SubQueries {
		subQuery := &normalized.SubQueries[index]
		if subQuery.Dimension != DimensionTechnology {
			continue
		}
		broad := len(subQuery.Expressions) == 0
		for _, expression := range subQuery.Expressions {
			if isBroadExpression(expression) {
				broad = true
				break
			}
		}
		if broad {
			subQuery.Expressions = []string{category}
		}
	}
	return normalized
}

func technologyCategoryFromQuery(query string) string {
	value := normalizeExpression(query)
	switch {
	case strings.Contains(value, "编程语言"), strings.Contains(value, "程序语言"), strings.Contains(value, "programming language"):
		return "编程语言"
	case strings.Contains(value, "数据库"), strings.Contains(value, "database"):
		return "数据库"
	case strings.Contains(value, "前端框架"), strings.Contains(value, "frontend framework"):
		return "前端框架"
	default:
		return ""
	}
}

type auditOperation struct {
	Tool          string
	Status        StreamStatus
	SafeErrorCode *string
	StartedAt     time.Time
	EndedAt       time.Time
}

func appendAuditOperation(operations []auditOperation, event StreamEvent) []auditOperation {
	if event.Tool == string(ToolAnalyzeIntent) || event.Tool == string(ToolExplainDraft) {
		// Model progress is user-visible but is not a Serving tool call. Keep it
		// out of the tool audit so the 20-call budget and tool metrics remain
		// comparable with previous requests.
		return operations
	}
	now := time.Now()
	if event.Status == StreamRunning {
		return append(operations, auditOperation{Tool: event.Tool, Status: event.Status, StartedAt: now})
	}
	for index := len(operations) - 1; index >= 0; index-- {
		if operations[index].Tool == event.Tool && operations[index].EndedAt.IsZero() {
			operations[index].Status = event.Status
			if event.Error != nil && event.Error.Code != "" {
				code := string(event.Error.Code)
				operations[index].SafeErrorCode = &code
			}
			operations[index].EndedAt = now
			return operations
		}
	}
	return operations
}

func (runner *TrendRunner) persistAudit(ctx context.Context, query string, result OrchestrationResult, draft *Draft, inputTokens, outputTokens int, operations []auditOperation, startedAt time.Time, runErr error) {
	if runner == nil || runner.Audit == nil {
		return
	}
	// A stopped request has a canceled context by design. Audit persistence is
	// best-effort but must still retain the token/call summary, so detach it
	// from request cancellation while keeping a short bounded write timeout.
	auditContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if inputTokens == 0 {
		inputTokens = (len([]byte(query)) + 3) / 4
	}
	status := string(StreamSuccess)
	if result.Stopped {
		status = string(StreamStopped)
	} else if result.Partial {
		status = string(StreamPartial)
	} else if runErr != nil {
		status = string(StreamFailure)
	}
	createdAt := startedAt
	completedAt := time.Now()
	requestHash := HashRequest([]byte(query))
	record := AuditRecord{
		RequestHash: requestHash, ContractVersion: ContractVersionV1, ValidationStatus: "valid",
		Status: status, AIAttemptCount: result.AIAttempts, ToolCallCount: len(operations),
		InputTokenCount: inputTokens, OutputTokenCount: outputTokens, Duration: completedAt.Sub(createdAt),
		CreatedAt: createdAt, CompletedAt: &completedAt, ExpiresAt: createdAt.Add(AuditRetention),
	}
	if result.Intent.ContractVersion != "" {
		if data, err := MarshalIntent(result.Intent); err == nil {
			hash := HashRequest(data)
			record.IntentHash = &hash
		}
	}
	if draft != nil {
		if data, err := json.Marshal(draft); err == nil {
			hash := HashRequest(data)
			record.DraftHash = &hash
		}
		record.DraftCurveCount = len(draft.Curves)
		for _, curve := range draft.Curves {
			if curve.Selected {
				record.DraftSelectedCount++
			}
		}
		record.DraftWarningCount = len(draft.Warnings)
	}
	if runErr != nil {
		safe := safeErrorFrom(runErr)
		record.SafeErrorCode = stringPointer(string(safe.Code))
	}
	for _, operation := range operations {
		if operation.Status == StreamFailed {
			record.ToolFailureCount++
		}
	}
	auditID, err := runner.Audit.Insert(auditContext, record)
	if err != nil {
		return
	}
	for index, operation := range operations {
		if index >= MaxToolCalls || operation.Tool == "" {
			break
		}
		ended := operation.EndedAt
		statusValue := string(operation.Status)
		if ended.IsZero() {
			// An operation can still be open when the client cancels or the
			// stream callback fails. PostgreSQL requires a terminal timestamp
			// for every status except running, so record that interrupted work
			// as skipped rather than creating an invalid audit row.
			statusValue = "skipped"
			ended = completedAt
		} else if statusValue == "" || statusValue == string(StreamRunning) {
			statusValue = string(StreamFailed)
		}
		_ = runner.Audit.InsertTool(auditContext, AuditToolRecord{AuditID: auditID, Sequence: index + 1, ToolName: operation.Tool, Status: statusValue, SafeErrorCode: operation.SafeErrorCode, Duration: ended.Sub(operation.StartedAt), StartedAt: operation.StartedAt, CompletedAt: &ended})
	}
	_ = runner.Audit.Cleanup(auditContext)
}

func stringPointer(value string) *string { return &value }

func resolverFromOptions(options jdserving.FilterOptionsResponse) Resolver {
	values := map[Dimension][]Candidate{
		DimensionTechnology: make([]Candidate, 0, len(options.TechnologyKeywords)),
		DimensionProject:    make([]Candidate, 0, len(options.Projects)),
		DimensionJobFamily:  make([]Candidate, 0, len(options.JobFamilies)),
		DimensionLocation:   make([]Candidate, 0, len(options.Locations)),
	}
	aliases := map[Dimension]map[string][]string{DimensionTechnology: make(map[string][]string)}
	for _, option := range options.TechnologyKeywords {
		values[DimensionTechnology] = append(values[DimensionTechnology], Candidate{Dimension: DimensionTechnology, Value: option.Value, Label: option.Label, MatchStatus: MatchExact})
		key := normalizeExpression(option.Category.Value)
		if key != "" {
			aliases[DimensionTechnology][key] = append(aliases[DimensionTechnology][key], option.Value)
		}
		labelKey := normalizeExpression(option.Category.Label)
		if labelKey != "" {
			aliases[DimensionTechnology][labelKey] = append(aliases[DimensionTechnology][labelKey], option.Value)
		}
	}
	for _, option := range options.Projects {
		if option.Value == "unrecognized" {
			continue
		}
		values[DimensionProject] = append(values[DimensionProject], Candidate{Dimension: DimensionProject, Value: option.Value, Label: option.Label, MatchStatus: MatchExact})
	}
	for _, option := range options.JobFamilies {
		values[DimensionJobFamily] = append(values[DimensionJobFamily], Candidate{Dimension: DimensionJobFamily, Value: option.Value, Label: option.Label, MatchStatus: MatchExact})
	}
	for _, option := range options.Locations {
		values[DimensionLocation] = append(values[DimensionLocation], Candidate{Dimension: DimensionLocation, Value: option.Value, Label: option.Label, MatchStatus: MatchExact})
	}
	return Resolver{Values: values, Aliases: aliases}
}

func candidateGroups(intent Intent, resolver Resolver) []CandidateGroup {
	groups := make([]CandidateGroup, 0, len(intent.SubQueries))
	for _, query := range intent.SubQueries {
		candidates := make([]Candidate, 0)
		for _, expression := range query.Expressions {
			if len(candidates) >= MaxToolCandidates {
				break
			}
			candidates = append(candidates, resolveIntentExpression(resolver, query, expression)...)
		}
		candidates = MergeCandidates(candidates)
		if len(candidates) > MaxToolCandidates {
			candidates = candidates[:MaxToolCandidates]
		}
		groups = append(groups, CandidateGroup{SubQueryID: query.ID, Candidates: candidates})
	}
	return groups
}

func resolveIntentExpression(resolver Resolver, query SubQuery, expression string) []Candidate {
	resolved := resolver.ResolveWithExclusions(query.Dimension, expression, query.Exclusions, query.ID)
	if len(resolved) > 0 {
		return applyQueryOptions(resolved, query)
	}
	if isBroadExpression(expression) {
		values := append([]Candidate(nil), resolver.Values[query.Dimension]...)
		for index := range values {
			values[index].SubQueryID = query.ID
			values[index].MatchStatus = MatchExpansion
			values[index].Score = 40
			values[index].Scope = "扩展推荐"
		}
		return filterExcluded(values, query.Exclusions)
	}
	if expanded := resolver.ResolveNatural(expression, query.ID); len(expanded) > 0 {
		filtered := expanded[:0]
		for _, candidate := range expanded {
			if candidate.Dimension != query.Dimension {
				continue
			}
			filtered = append(filtered, candidate)
		}
		return applyQueryOptions(filterExcluded(filtered, query.Exclusions), query)
	}
	return nil
}

func applyQueryOptions(candidates []Candidate, query SubQuery) []Candidate {
	for index := range candidates {
		candidates[index].Operator = query.Operator
		if query.IncludeSecondary != nil {
			candidates[index].IncludeSecondary = *query.IncludeSecondary
		}
	}
	return candidates
}

func filterExcluded(candidates []Candidate, exclusions []string) []Candidate {
	if len(exclusions) == 0 {
		return candidates
	}
	blocked := make(map[string]struct{}, len(exclusions))
	for _, exclusion := range exclusions {
		blocked[normalizeExpression(exclusion)] = struct{}{}
	}
	filtered := candidates[:0]
	for _, candidate := range candidates {
		if _, ok := blocked[normalizeExpression(candidate.Value)]; ok {
			continue
		}
		if _, ok := blocked[normalizeExpression(candidate.Label)]; ok {
			continue
		}
		filtered = append(filtered, candidate)
	}
	return filtered
}

func isBroadExpression(expression string) bool {
	value := normalizeExpression(expression)
	if value == "" || value == "*" || value == "all" || value == "any" || value == "all values" {
		return true
	}
	for _, marker := range []string{"哪些", "所有", "全部", "使用了哪些", "项目", "数据库", "技术", "岗位族", "地点", "project", "projects", "database", "databases"} {
		if value == marker {
			return true
		}
		if strings.Contains(value, marker) && (strings.Contains(value, "哪些") || strings.HasPrefix(value, "所有") || strings.HasPrefix(value, "全部")) {
			return true
		}
	}
	return false
}

func planServingTools(intent Intent, groups []CandidateGroup) []ToolCall {
	// Reserve one registry call for final draft validation.
	maxPlanningCalls := MaxToolCalls - 1
	calls := make([]ToolCall, 0, maxPlanningCalls)
	dimensions := make(map[Dimension]struct{})
	for _, query := range intent.SubQueries {
		dimensions[query.Dimension] = struct{}{}
	}
	orderedDimensions := []Dimension{DimensionTechnology, DimensionProject, DimensionJobFamily, DimensionLocation}
	for _, dimension := range orderedDimensions {
		if _, ok := dimensions[dimension]; !ok || len(calls) >= maxPlanningCalls {
			continue
		}
		calls = append(calls, ToolCall{Name: ToolListAvailableValues, Params: mustMarshal(valueRequest{Dimension: dimension})})
	}
	for _, query := range intent.SubQueries {
		if len(query.Expressions) == 0 || len(calls) >= maxPlanningCalls {
			continue
		}
		expression := strings.TrimSpace(query.Expressions[0])
		if expression == "" || isBroadExpression(expression) {
			continue
		}
		calls = append(calls, ToolCall{Name: ToolRankValues, Params: mustMarshal(rankRequest{Dimension: query.Dimension, Expression: expression, SubQueryID: query.ID})})
	}
	requests := make([]trendToolRequest, 0, MaxBatchCandidates)
	for _, group := range groups {
		for _, candidate := range group.Candidates {
			requests = append(requests, candidateTrendRequest(intent.TimeRange, candidate))
			if len(requests) == MaxBatchCandidates {
				if len(calls) >= maxPlanningCalls {
					return calls
				}
				calls = append(calls, ToolCall{Name: ToolBatchTrendSummary, Params: mustMarshal(requests)})
				requests = requests[:0]
				if len(calls) == maxPlanningCalls {
					return calls
				}
			}
		}
	}
	if len(requests) > 0 && len(calls) < maxPlanningCalls {
		calls = append(calls, ToolCall{Name: ToolBatchTrendSummary, Params: mustMarshal(requests)})
	}
	return calls
}

func candidateTrendRequest(timeRange TimeRange, candidate Candidate) trendToolRequest {
	return trendToolRequest{
		Dimension:        candidate.Dimension,
		Value:            candidate.Value,
		Range:            RangeKind(timeRange.Kind),
		StartDate:        timeRange.StartDate,
		EndDate:          timeRange.EndDate,
		IncludeSecondary: candidate.IncludeSecondary,
		Operator:         candidate.Operator,
	}
}

func applySummaryScores(groups []CandidateGroup, results []ToolCallResult, intent Intent) {
	metricBySubQuery := make(map[string]Metric, len(intent.SubQueries))
	orderBySubQuery := make(map[string]Order, len(intent.SubQueries))
	for _, query := range intent.SubQueries {
		metricBySubQuery[query.ID] = query.Ranking.Metric
		orderBySubQuery[query.ID] = query.Ranking.Order
	}
	for _, result := range results {
		if result.Name != ToolBatchTrendSummary || !result.Available {
			continue
		}
		var summaries []summaryToolResponse
		if json.Unmarshal(result.Payload, &summaries) != nil {
			continue
		}
		for _, summary := range summaries {
			if summary.Error != nil {
				continue
			}
			for groupIndex := range groups {
				for candidateIndex := range groups[groupIndex].Candidates {
					candidate := &groups[groupIndex].Candidates[candidateIndex]
					if candidate.Dimension != Dimension(summary.Curve.Dimension) || candidate.Value != summary.Curve.Value || candidate.IncludeSecondary != summary.Curve.IncludeSecondary {
						continue
					}
					for _, subQueryID := range candidateSubQueryIDs(*candidate, groups[groupIndex].SubQueryID) {
						value, ok := summaryMetric(summary, metricBySubQuery[subQueryID])
						if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
							continue
						}
						score := int(math.Round(value * 1000))
						if orderBySubQuery[subQueryID] == OrderAscending {
							score = -score
						}
						candidate.Score = score
					}
				}
			}
		}
	}
	for index := range groups {
		sort.SliceStable(groups[index].Candidates, func(left, right int) bool {
			if groups[index].Candidates[left].Score != groups[index].Candidates[right].Score {
				return groups[index].Candidates[left].Score > groups[index].Candidates[right].Score
			}
			return groups[index].Candidates[left].Value < groups[index].Candidates[right].Value
		})
	}
}

func filterFailedSummaryCandidates(groups []CandidateGroup, results []ToolCallResult) {
	failed := make(map[string]struct{})
	for _, result := range results {
		if result.Name != ToolBatchTrendSummary || !result.Available {
			continue
		}
		var summaries []summaryToolResponse
		if json.Unmarshal(result.Payload, &summaries) != nil {
			continue
		}
		for _, summary := range summaries {
			if summary.Error != nil {
				failed[explanationCurveKey(Dimension(summary.Curve.Dimension), summary.Curve.Value, summary.Curve.IncludeSecondary)] = struct{}{}
			}
		}
	}
	if len(failed) == 0 {
		return
	}
	for groupIndex := range groups {
		kept := groups[groupIndex].Candidates[:0]
		for _, candidate := range groups[groupIndex].Candidates {
			if _, ok := failed[explanationCurveKey(candidate.Dimension, candidate.Value, candidate.IncludeSecondary)]; ok {
				continue
			}
			kept = append(kept, candidate)
		}
		groups[groupIndex].Candidates = kept
	}
}

func candidateSubQueryIDs(candidate Candidate, fallback string) []string {
	if len(candidate.SubQueryIDs) > 0 {
		return candidate.SubQueryIDs
	}
	if candidate.SubQueryID != "" {
		return []string{candidate.SubQueryID}
	}
	return []string{fallback}
}

func summaryMetric(summary summaryToolResponse, metric Metric) (float64, bool) {
	switch metric {
	case MetricAverageDailyActiveJobs:
		if summary.AverageDailyActiveJobs == nil {
			return 0, false
		}
		return *summary.AverageDailyActiveJobs, true
	case MetricPeriodEndActiveJobs:
		if summary.PeriodEndActiveJobs == nil {
			return 0, false
		}
		return float64(*summary.PeriodEndActiveJobs), true
	case MetricPeakDailyActiveJobs:
		if summary.PeakDailyActiveJobs == nil {
			return 0, false
		}
		return float64(summary.PeakDailyActiveJobs.Count), true
	case MetricPeriodUniqueJobs:
		if summary.PeriodUniqueJobs == nil {
			return 0, false
		}
		return float64(*summary.PeriodUniqueJobs), true
	case MetricEnteredJobs, MetricModifiedJobs, MetricLeftJobs, MetricReopenedJobs:
		value, ok := summary.Events[jdserving.TrendEventType(strings.TrimSuffix(string(metric), "_jobs"))]
		return float64(value), ok
	default:
		return 0, false
	}
}

func unsupportedMetric(intent Intent) bool {
	for _, query := range intent.SubQueries {
		if query.Ranking.Metric == MetricAbsoluteChange || query.Ranking.Metric == MetricRelativeChange {
			return true
		}
	}
	return false
}

func draftExplanation(intent Intent) string {
	metric := "当前周期岗位平均数量"
	for _, query := range intent.SubQueries {
		switch query.Ranking.Metric {
		case MetricPeriodEndActiveJobs:
			metric = "当前周期末岗位数量"
		case MetricPeakDailyActiveJobs:
			metric = "当前周期岗位峰值"
		case MetricPeriodUniqueJobs:
			metric = "当前周期出现过的岗位数量"
		}
		break
	}
	return fmt.Sprintf("按%s整理候选，应用前仍可修改。", metric)
}

func buildDraftExplanationContext(intent Intent, draft Draft, groups []CandidateGroup, results []ToolCallResult) DraftExplanationContext {
	context := DraftExplanationContext{Candidates: make([]DraftExplanationCandidate, 0, len(draft.Curves))}
	if len(intent.SubQueries) > 0 {
		context.Dimension = intent.SubQueries[0].Dimension
		context.Metric = intent.SubQueries[0].Ranking.Metric
		context.Order = intent.SubQueries[0].Ranking.Order
	}
	if len(intent.TimeRange.Kind) > 0 {
		context.TimeRange = intent.TimeRange.Kind
	}
	metricValues := explanationMetricValues(intent, groups, results)
	for _, curve := range draft.Curves {
		candidate := DraftExplanationCandidate{Label: curve.Label, MatchStatus: curve.MatchStatus, Scope: curve.Scope}
		if value, ok := metricValues[explanationCurveKey(curve.Condition.Dimension, curve.Condition.Value, curve.Condition.IncludeSecondary)]; ok {
			candidate.MetricValue = &value
		}
		context.Candidates = append(context.Candidates, candidate)
	}
	seenTools := make(map[string]struct{})
	for _, result := range results {
		name := string(result.Name)
		if name == "" {
			continue
		}
		if _, ok := seenTools[name]; ok {
			continue
		}
		seenTools[name] = struct{}{}
		context.ToolOperations = append(context.ToolOperations, name)
	}
	return context
}

func explanationMetricValues(intent Intent, groups []CandidateGroup, results []ToolCallResult) map[string]float64 {
	metricBySubQuery := make(map[string]Metric, len(intent.SubQueries))
	for _, query := range intent.SubQueries {
		metricBySubQuery[query.ID] = query.Ranking.Metric
	}
	values := make(map[string]float64)
	for _, result := range results {
		if result.Name != ToolBatchTrendSummary || !result.Available {
			continue
		}
		var summaries []summaryToolResponse
		if json.Unmarshal(result.Payload, &summaries) != nil {
			continue
		}
		for _, summary := range summaries {
			for _, group := range groups {
				for _, candidate := range group.Candidates {
					if candidate.Dimension != Dimension(summary.Curve.Dimension) || candidate.Value != summary.Curve.Value || candidate.IncludeSecondary != summary.Curve.IncludeSecondary {
						continue
					}
					for _, subQueryID := range candidateSubQueryIDs(candidate, group.SubQueryID) {
						metricValue, ok := summaryMetric(summary, metricBySubQuery[subQueryID])
						if ok {
							values[explanationCurveKey(candidate.Dimension, candidate.Value, candidate.IncludeSecondary)] = metricValue
						}
					}
				}
			}
		}
	}
	return values
}

func explanationCurveKey(dimension Dimension, value string, includeSecondary bool) string {
	return fmt.Sprintf("%s:%s:%t", dimension, value, includeSecondary)
}

func emitValidatedDraft(ctx context.Context, registry *ToolRegistry, draft Draft, emit func(StreamEvent) error) error {
	params := mustMarshal([]Draft{draft})
	call := ToolCall{Name: ToolBatchValidateDraft, Params: params}
	if err := emit(StreamEvent{Type: StreamEventOperation, Status: StreamRunning, Tool: string(call.Name), Params: call.Params}); err != nil {
		return err
	}
	_, err := registry.Call(ctx, call)
	if err != nil {
		return emit(StreamEvent{Type: StreamEventOperation, Status: StreamFailed, Tool: string(call.Name), Params: call.Params, Error: safeErrorFrom(err)})
	}
	return emit(StreamEvent{Type: StreamEventOperation, Status: StreamCompleted, Tool: string(call.Name), Params: call.Params})
}
