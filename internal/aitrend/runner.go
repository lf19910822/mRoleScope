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

type auditOperation struct {
	Tool      string
	Status    StreamStatus
	StartedAt time.Time
	EndedAt   time.Time
}

func appendAuditOperation(operations []auditOperation, event StreamEvent) []auditOperation {
	now := time.Now()
	if event.Status == StreamRunning {
		return append(operations, auditOperation{Tool: event.Tool, Status: event.Status, StartedAt: now})
	}
	for index := len(operations) - 1; index >= 0; index-- {
		if operations[index].Tool == event.Tool && operations[index].EndedAt.IsZero() {
			operations[index].Status = event.Status
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
		Status: status, AIAttemptCount: result.AIAttempts, ToolCallCount: len(result.ToolResults),
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
	for _, toolResult := range result.ToolResults {
		if toolResult.SafeError != nil {
			record.ToolFailureCount++
		}
	}
	auditID, err := runner.Audit.Insert(ctx, record)
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
		_ = runner.Audit.InsertTool(ctx, AuditToolRecord{AuditID: auditID, Sequence: index + 1, ToolName: operation.Tool, Status: statusValue, Duration: ended.Sub(operation.StartedAt), StartedAt: operation.StartedAt, CompletedAt: &ended})
	}
	_ = runner.Audit.Cleanup(ctx)
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
	for _, marker := range []string{"哪些", "所有", "全部", "使用了哪些", "项目", "数据库", "技术", "岗位族", "地点", "project", "projects", "database", "databases"} {
		if value == marker || strings.Contains(value, marker) && (strings.Contains(value, "哪些") || marker == value) {
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
		calls = append(calls, ToolCall{Name: ToolRankValues, Params: mustMarshal(rankRequest{Dimension: query.Dimension, Expression: query.Expressions[0], SubQueryID: query.ID})})
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
