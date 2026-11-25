package aitrend

import (
	"context"
	"encoding/json"
	"errors"
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
	Provider            IntentProvider
	Tools               ServingToolService
	Audit               *AuditStore
	ServiceModeFeedback *ServiceModeFeedback
	MaxDuration         time.Duration
	StatsSink           func(TrendRunStats)
	Debug               DebugTraceSink
	Langfuse            LangfuseDebugConfig
}

// TrendRunStats is an in-memory handoff for evaluation and diagnostics. It is
// never part of the public user response and does not replace aggregate audit.
type TrendRunStats struct {
	InputTokens          int
	OutputTokens         int
	ServiceTags          []ServiceTag
	ClassificationStatus string
	ClassificationInput  int
	ClassificationOutput int
	ClassificationTime   time.Duration
	ToolCalls            int
	ToolFailures         int
	DraftCurveCount      int
	Intent               *Intent
	ResponseModes        []ResponseMode
	ReplyQualityWarnings []string
	StartedAt            time.Time
	CompletedAt          time.Time
}

func NewTrendRunner(provider IntentProvider, options FilterOptionsReader, summary jdserving.TrendSummaryReader, events TrendEventReader) (*TrendRunner, error) {
	if provider == nil || options == nil || summary == nil {
		return nil, &ContractError{Code: ErrorConfigurationFailure, Message: "trend AI is not configured"}
	}
	return &TrendRunner{Provider: provider, Tools: ServingToolService{Options: options, Summary: summary, Events: events}}, nil
}

// RunDebug executes one bounded run with the supplied operator trace sink.
// It deliberately uses a copy with Audit disabled so no production Intent
// hash or aggregate audit row is created.
func (runner *TrendRunner) RunDebug(ctx context.Context, query string, emit func(StreamEvent) error, sink DebugTraceSink) error {
	if runner == nil || sink == nil {
		return &ContractError{Code: ErrorConfigurationFailure, Message: "AI debug trace is not configured"}
	}
	debugRunner := *runner
	debugRunner.Audit = nil
	langfuseSink, langfuseErr := NewLangfuseDebugSink(debugRunner.Langfuse, nil)
	if langfuseErr != nil {
		langfuseSink, _ = NewLangfuseDebugSink(LangfuseDebugConfig{}, nil)
	}
	combined := NewDebugTraceFanout(sink, langfuseSink)
	debugRunner.Debug = combined
	err := debugRunner.Run(ctx, query, emit)
	status := string(StreamSuccess)
	var safeErr *SafeError
	if err != nil {
		status = string(StreamFailure)
		safeErr = safeErrorFrom(err)
	}
	_ = combined.Record(DebugTraceEvent{Type: DebugEventTerminal, Status: status, Error: safeErr})
	if langfuseSink != nil && debugRunner.Langfuse.Enabled() {
		flushContext := ctx
		if debugRunner.Langfuse.FlushTimeout > 0 {
			var cancel context.CancelFunc
			flushContext, cancel = context.WithTimeout(ctx, debugRunner.Langfuse.FlushTimeout)
			defer cancel()
		}
		status := langfuseSink.Flush(flushContext)
		_ = sink.Record(DebugTraceEvent{Type: DebugEventTraceLink, TraceID: status.TraceID, TraceURL: status.TraceURL, ExportStatus: status.Status, ExportErrorCode: status.ErrorCode})
	}
	return err
}

func (runner *TrendRunner) Run(ctx context.Context, query string, emit func(StreamEvent) error) error {
	ctx = withDebugTrace(ctx, runner.Debug)
	startedAt := time.Now()
	var inputTokens, outputTokens int
	var serviceTags []ServiceTag
	classificationStatus := ""
	var classificationInput, classificationOutput int
	var classificationDuration time.Duration
	var result OrchestrationResult
	var draft *Draft
	var replyQualityWarnings []string
	defer func() {
		if runner == nil || runner.StatsSink == nil {
			return
		}
		failures := 0
		for _, toolResult := range result.ToolResults {
			if toolResult.SafeError != nil {
				failures++
			}
		}
		// Tool failure details remain in the stream and aggregate AuditStore;
		// this handoff only carries bounded counters for the evaluator.
		totalInputTokens, totalOutputTokens := inputTokens, outputTokens
		if totalInputTokens == 0 {
			totalInputTokens = result.InputTokens
		}
		if totalOutputTokens == 0 {
			totalOutputTokens = result.OutputTokens
		}
		runner.StatsSink(TrendRunStats{
			InputTokens: totalInputTokens, OutputTokens: totalOutputTokens,
			ServiceTags: append([]ServiceTag(nil), result.ServiceTags...), ClassificationStatus: classificationStatus,
			ClassificationInput: classificationInput, ClassificationOutput: classificationOutput, ClassificationTime: classificationDuration,
			ToolCalls: len(result.ToolResults), ToolFailures: failures,
			DraftCurveCount: func() int {
				if draft == nil {
					return 0
				}
				return len(draft.Curves)
			}(),
			ResponseModes:        append([]ResponseMode(nil), result.AgentResponse.ResponseModes...),
			ReplyQualityWarnings: append([]string(nil), replyQualityWarnings...),
			Intent: func() *Intent {
				if result.Intent.ContractVersion == "" {
					return nil
				}
				intent := result.Intent
				return &intent
			}(),
			StartedAt: startedAt, CompletedAt: time.Now(),
		})
	}()
	if runner == nil || runner.Provider == nil || runner.Tools.Options == nil || runner.Tools.Summary == nil {
		return &ContractError{Code: ErrorConfigurationFailure, Message: "trend AI is not configured"}
	}
	var groups []CandidateGroup
	var plannedIntent Intent
	toolProvider, usesToolSession := runner.Provider.(IntentToolProvider)
	var resolver Resolver
	var registry *ToolRegistry
	var err error
	auditOperations := make([]auditOperation, 0, MaxToolCalls+1)
	auditStarted := time.Now()
	streamEmit := func(event StreamEvent) error {
		if event.Type == StreamEventOperation {
			auditOperations = appendAuditOperation(auditOperations, event)
		}
		if err := recordDebug(ctx, DebugTraceEvent{Type: DebugEventStream, Stream: &event, Tool: event.Tool, Status: string(event.Status)}); err != nil {
			return err
		}
		if event.Type == StreamEventValidation {
			if err := recordDebug(ctx, DebugTraceEvent{Type: DebugEventValidation, Tool: event.Tool, Status: string(event.Status), Error: event.Error, Params: debugJSON(event.Params)}); err != nil {
				return err
			}
		}
		return emit(event)
	}
	classifier, hasClassifier := runner.Provider.(IntentServiceModeClassifier)
	if hasClassifier {
		classificationStarted := time.Now()
		classificationStatus = "running"
		if err := streamEmit(StreamEvent{Type: StreamEventOperation, Status: StreamRunning, Tool: string(ToolClassifyServiceModes)}); err != nil {
			return err
		}
		classification, input, output, classifyErr := classifier.ClassifyServiceModes(ctx, query)
		classificationInput, classificationOutput = input, output
		inputTokens += input
		outputTokens += output
		classificationDuration = time.Since(classificationStarted)
		if classifyErr != nil {
			classificationStatus = "failed"
			serviceTags = []ServiceTag{ServiceTagGuidance}
			_ = streamEmit(StreamEvent{Type: StreamEventOperation, Status: StreamFailed, Tool: string(ToolClassifyServiceModes), Error: safeErrorFrom(classifyErr)})
		} else {
			serviceTags, classifyErr = NormalizeServiceTags(classification.ServiceTags)
			if classifyErr != nil {
				classificationStatus = "failed"
				serviceTags = []ServiceTag{ServiceTagGuidance}
				_ = streamEmit(StreamEvent{Type: StreamEventOperation, Status: StreamFailed, Tool: string(ToolClassifyServiceModes), Error: safeErrorFrom(classifyErr)})
			} else {
				classificationStatus = "completed"
				_ = streamEmit(StreamEvent{Type: StreamEventOperation, Status: StreamCompleted, Tool: string(ToolClassifyServiceModes)})
			}
		}
		result.ServiceTags = append([]ServiceTag(nil), serviceTags...)
		result.ClassificationStatus = classificationStatus
		result.ClassificationInput = classificationInput
		result.ClassificationOutput = classificationOutput
		result.ClassificationTime = classificationDuration
	}
	if hasClassifier && !serviceTagsContain(serviceTags, ServiceTagTrend) {
		modes, modeErr := ResponseModesFromServiceTags(serviceTags)
		if modeErr != nil {
			return modeErr
		}
		result = OrchestrationResult{AgentResponse: AgentResponse{ContractVersion: AgentContractVersionV1, ResponseModes: modes}, ServiceTags: append([]ServiceTag(nil), serviceTags...), ClassificationStatus: classificationStatus, ClassificationInput: classificationInput, ClassificationOutput: classificationOutput, ClassificationTime: classificationDuration}
		guidanceInput, guidanceOutput, warnings, replyErr := runner.streamAssistantReply(ctx, query, modes, nil, false, streamEmit)
		inputTokens += guidanceInput
		outputTokens += guidanceOutput
		replyQualityWarnings = append(replyQualityWarnings, warnings...)
		result.ReplyQualityWarnings = append([]string(nil), replyQualityWarnings...)
		if replyErr != nil {
			if errors.Is(replyErr, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
				_ = streamEmit(StreamEvent{Type: StreamEventDone, Status: StreamStopped})
				runner.persistAudit(ctx, query, result, nil, inputTokens, outputTokens, auditOperations, auditStarted, nil)
				return nil
			}
			runner.persistAudit(ctx, query, result, nil, inputTokens, outputTokens, auditOperations, auditStarted, replyErr)
			return replyErr
		}
		err = streamEmit(StreamEvent{Type: StreamEventDone, Status: StreamSuccess})
		runner.persistAudit(ctx, query, result, nil, inputTokens, outputTokens, auditOperations, auditStarted, err)
		return err
	}
	options, err := runner.Tools.Options.ListFilterOptions(ctx)
	if err != nil {
		return &ContractError{Code: ErrorBackendFailure, Message: "读取正式筛选值失败"}
	}
	resolver = resolverFromOptions(options)
	tools := runner.Tools
	tools.Snapshot = newServingSnapshot(options)
	tools.Resolver = resolver
	registry, err = tools.Registry()
	if err != nil {
		return err
	}
	orchestrator := Orchestrator{
		ToolAnalyzer: toolProvider,
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
			intent = normalizeIntentForQuery(query, intent, resolver)
			plannedIntent = intent
			groups = candidateGroups(intent, resolver)
			return planServingTools(intent, groups), nil
		},
		Registry:        registry,
		MaxDuration:     runner.MaxDuration,
		MaxInputTokens:  12_000,
		MaxOutputTokens: 16_000,
		Emit:            streamEmit,
		ServiceTags:     append([]ServiceTag(nil), serviceTags...),
	}
	result, err = orchestrator.RunWithInput(ctx, query)
	result.ServiceTags = append([]ServiceTag(nil), serviceTags...)
	result.ClassificationStatus = classificationStatus
	result.ClassificationInput = classificationInput
	result.ClassificationOutput = classificationOutput
	result.ClassificationTime = classificationDuration
	if err != nil {
		runner.persistAudit(ctx, query, result, nil, inputTokens, outputTokens, auditOperations, auditStarted, err)
		return err
	}
	inputTokens += result.InputTokens
	outputTokens += result.OutputTokens
	// Let the Agent classify a conflicting question as guidance-only. Legacy
	// trend responses still receive the established contract error so the
	// existing non-Agent seam keeps its validation boundary.
	if conflictingTrendRanges(query) && !isGuidanceOnly(result.AgentResponse) {
		conflictErr := &ContractError{Code: ErrorInvalidCombination, Field: "time_range", Message: "用户同时指定了互相冲突的时间范围"}
		runner.persistAudit(ctx, query, result, nil, inputTokens, outputTokens, auditOperations, auditStarted, conflictErr)
		return conflictErr
	}
	if isGuidanceOnly(result.AgentResponse) {
		guidanceInput, guidanceOutput, warnings, replyErr := runner.streamAssistantReply(ctx, query, result.AgentResponse.ResponseModes, nil, false, streamEmit)
		inputTokens += guidanceInput
		outputTokens += guidanceOutput
		replyQualityWarnings = append(replyQualityWarnings, warnings...)
		result.ReplyQualityWarnings = append([]string(nil), replyQualityWarnings...)
		if replyErr != nil {
			if errors.Is(replyErr, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
				_ = streamEmit(StreamEvent{Type: StreamEventDone, Status: StreamStopped})
				runner.persistAudit(ctx, query, result, nil, inputTokens, outputTokens, auditOperations, auditStarted, nil)
				return nil
			}
			runner.persistAudit(ctx, query, result, nil, inputTokens, outputTokens, auditOperations, auditStarted, replyErr)
			return replyErr
		}
		err = streamEmit(StreamEvent{Type: StreamEventDone, Status: StreamSuccess})
		runner.persistAudit(ctx, query, result, nil, inputTokens, outputTokens, auditOperations, auditStarted, err)
		return err
	}
	if plannedIntent.ContractVersion == "" {
		plannedIntent = normalizeIntentForQuery(query, result.Intent, resolver)
		groups = candidateGroups(plannedIntent, resolver)
	}
	summaryCompletionPartial, completionErr := completeMissingTrendSummaries(ctx, registry, groups, plannedIntent, &result.ToolResults, streamEmit)
	if completionErr != nil {
		runner.persistAudit(ctx, query, result, nil, inputTokens, outputTokens, auditOperations, auditStarted, completionErr)
		return completionErr
	}
	if summaryCompletionPartial {
		result.Partial = true
	}
	applySummaryScores(groups, result.ToolResults, plannedIntent)
	rankApplied := applyTrendRankResults(groups, result.ToolResults)
	if !rankApplied {
		boundBroadCandidates(groups)
	}
	if usesToolSession {
		filterCandidatesToSummaryResults(groups, result.ToolResults)
	}
	filterFailedSummaryCandidates(groups, result.ToolResults)

	partialFailure := result.Partial
	warnings := make([]string, 0)
	if summaryCompletionPartial {
		warnings = append(warnings, "部分候选趋势摘要未能读取，结果可能不完整。")
	}
	for _, toolResult := range result.ToolResults {
		if toolResult.SafeError != nil {
			// A malformed model tool request is corrected in the tool session;
			// once a later turn succeeds it must not downgrade the final Draft.
			if !toolResult.Available && isModelToolContractError(&ContractError{Code: toolResult.SafeError.Code}) {
				continue
			}
			partialFailure = true
			break
		}
	}
	if unsupportedMetric(plannedIntent) {
		warnings = append(warnings, "部分比较指标暂不直接作为纵轴，已按岗位数量生成曲线。")
	}
	if unsupportedMetricPhrase(query) {
		warnings = append(warnings, "当前暂不支持直接绘制占比或变化率，已按岗位数量生成曲线。")
		partialFailure = true
	}
	if unsupportedChartPhrase(query) {
		warnings = append(warnings, "当前看板使用折线图，已保留岗位趋势曲线。")
		partialFailure = true
	}
	globalTechnologyFallback := hasProjectTechnologyConstraint(query, plannedIntent, resolver)
	if globalTechnologyFallback {
		warnings = append(warnings, "当前技术曲线为全局范围，暂未按项目过滤。")
		partialFailure = true
	}
	statusStopped := result.Stopped
	explanation := draftExplanation(plannedIntent)
	if globalTechnologyFallback {
		explanation += "技术曲线为全局范围，暂未按项目过滤。"
	}
	builtDraft, err := BuildDraft(groups, DraftBuildOptions{
		Warnings:       warnings,
		Explanation:    explanation,
		Stopped:        statusStopped,
		PartialFailure: partialFailure,
	})
	if err != nil {
		return err
	}
	draft = &builtDraft
	if err := recordDebug(ctx, DebugTraceEvent{Type: DebugEventDraft, Draft: draft}); err != nil {
		runner.persistAudit(ctx, query, result, draft, inputTokens, outputTokens, auditOperations, auditStarted, err)
		return err
	}
	if len(draft.Curves) > 0 {
		if err := emitValidatedDraft(ctx, registry, *draft, streamEmit); err != nil {
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
	if err := streamEmit(StreamEvent{Type: StreamEventDraft, Status: draftStatus, Draft: draft}); err != nil {
		runner.persistAudit(ctx, query, result, draft, inputTokens, outputTokens, auditOperations, auditStarted, err)
		return err
	}
	if !result.Stopped {
		modes := result.AgentResponse.ResponseModes
		if len(modes) == 0 {
			modes = []ResponseMode{ResponseModeTrend}
		}
		if _, streamProvider := runner.Provider.(AssistantReplyStreamProvider); streamProvider || !hasLegacyExplanationProvider(runner.Provider) {
			input, output, warnings, replyErr := runner.streamAssistantReply(ctx, query, modes, func() *DraftExplanationContext {
				context := buildDraftExplanationContext(plannedIntent, *draft, groups, result.ToolResults)
				return &context
			}(), true, streamEmit)
			inputTokens += input
			outputTokens += output
			replyQualityWarnings = append(replyQualityWarnings, warnings...)
			result.ReplyQualityWarnings = append([]string(nil), replyQualityWarnings...)
			if replyErr != nil {
				if errors.Is(replyErr, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
					draft.Status = DraftStopped
					_ = streamEmit(StreamEvent{Type: StreamEventDraft, Status: StreamStopped, Draft: draft})
					_ = streamEmit(StreamEvent{Type: StreamEventDone, Status: StreamStopped})
					runner.persistAudit(ctx, query, result, draft, inputTokens, outputTokens, auditOperations, auditStarted, nil)
					return nil
				}
				runner.persistAudit(ctx, query, result, draft, inputTokens, outputTokens, auditOperations, auditStarted, replyErr)
				return replyErr
			}
		} else if explanationProvider, ok := runner.Provider.(IntentExplanationUsageProvider); ok {
			// Keep the legacy non-streaming test/provider seam working. The
			// initial Draft has already been emitted; send a second Draft with
			// the completed legacy explanation for old consumers.
			if err := streamEmit(StreamEvent{Type: StreamEventOperation, Status: StreamRunning, Tool: string(ToolExplainDraft)}); err != nil {
				runner.persistAudit(ctx, query, result, draft, inputTokens, outputTokens, auditOperations, auditStarted, err)
				return err
			}
			explanation, input, output, explanationErr := explanationProvider.ExplainDraftUsage(ctx, query, buildDraftExplanationContext(plannedIntent, *draft, groups, result.ToolResults))
			inputTokens += input
			outputTokens += output
			if explanationErr != nil {
				if err := streamEmit(StreamEvent{Type: StreamEventOperation, Status: StreamFailed, Tool: string(ToolExplainDraft), Error: safeErrorFrom(explanationErr)}); err != nil {
					runner.persistAudit(ctx, query, result, draft, inputTokens, outputTokens, auditOperations, auditStarted, err)
					return err
				}
			} else {
				draft.Explanation = explanation
				if err := streamEmit(StreamEvent{Type: StreamEventOperation, Status: StreamCompleted, Tool: string(ToolExplainDraft)}); err != nil {
					runner.persistAudit(ctx, query, result, draft, inputTokens, outputTokens, auditOperations, auditStarted, err)
					return err
				}
				if err := streamEmit(StreamEvent{Type: StreamEventDraft, Status: draftStatus, Draft: draft}); err != nil {
					runner.persistAudit(ctx, query, result, draft, inputTokens, outputTokens, auditOperations, auditStarted, err)
					return err
				}
			}
		}
	}
	err = streamEmit(StreamEvent{Type: StreamEventDone, Status: terminalStatus})
	runner.persistAudit(ctx, query, result, draft, inputTokens, outputTokens, auditOperations, auditStarted, err)
	return err
}

func hasLegacyExplanationProvider(provider IntentProvider) bool {
	_, ok := provider.(IntentExplanationUsageProvider)
	return ok
}

func isGuidanceOnly(response AgentResponse) bool {
	if response.ContractVersion != AgentContractVersionV1 || response.TrendIntent != nil {
		return false
	}
	for _, mode := range response.ResponseModes {
		if mode == ResponseModeGuidance {
			return true
		}
	}
	return false
}

func (runner *TrendRunner) streamAssistantReply(ctx context.Context, query string, modes []ResponseMode, draftContext *DraftExplanationContext, emitOperation bool, emit func(StreamEvent) error) (int, int, []string, error) {
	replyContext := AssistantReplyContext{Query: query, ResponseModes: append([]ResponseMode(nil), modes...), Draft: draftContext, ServiceModeFeedback: safeServiceModeFeedback(runner.ServiceModeFeedback)}
	var fullReply strings.Builder
	if provider, ok := runner.Provider.(AssistantReplyStreamProvider); ok {
		if emitOperation {
			if err := emit(StreamEvent{Type: StreamEventOperation, Status: StreamRunning, Tool: string(ToolExplainDraft)}); err != nil {
				return 0, 0, nil, err
			}
		}
		inputTokens, outputTokens, err := provider.StreamAssistantMessage(ctx, replyContext, func(delta string) error {
			fullReply.WriteString(delta)
			return emit(StreamEvent{Type: StreamEventAssistant, Status: StreamRunning, Delta: delta})
		})
		if err != nil {
			if emitOperation {
				_ = emit(StreamEvent{Type: StreamEventOperation, Status: StreamFailed, Tool: string(ToolExplainDraft), Error: safeErrorFrom(err)})
			}
			return inputTokens, outputTokens, AssistantReplyQualityWarnings(fullReply.String()), err
		}
		if err := emit(StreamEvent{Type: StreamEventAssistant, Status: StreamCompleted}); err != nil {
			return inputTokens, outputTokens, AssistantReplyQualityWarnings(fullReply.String()), err
		}
		if emitOperation {
			if err := emit(StreamEvent{Type: StreamEventOperation, Status: StreamCompleted, Tool: string(ToolExplainDraft)}); err != nil {
				return inputTokens, outputTokens, AssistantReplyQualityWarnings(fullReply.String()), err
			}
		}
		return inputTokens, outputTokens, AssistantReplyQualityWarnings(fullReply.String()), nil
	}
	// Provider-free tests and compatibility adapters still receive a bounded
	// deterministic message until they implement the stream seam.
	fallback := "当前可以分析项目、技术关键词、岗位族和地点的岗位趋势。"
	if len(modes) > 0 && modes[0] == ResponseModeTrend {
		fallback = "已按当前趋势结果整理曲线候选，应用前仍可修改。"
	}
	if emitOperation {
		if err := emit(StreamEvent{Type: StreamEventOperation, Status: StreamRunning, Tool: string(ToolExplainDraft)}); err != nil {
			return 0, 0, nil, err
		}
	}
	fullReply.WriteString(fallback)
	if err := emit(StreamEvent{Type: StreamEventAssistant, Status: StreamRunning, Delta: fallback}); err != nil {
		return 0, 0, AssistantReplyQualityWarnings(fullReply.String()), err
	}
	if err := emit(StreamEvent{Type: StreamEventAssistant, Status: StreamCompleted}); err != nil {
		return 0, 0, AssistantReplyQualityWarnings(fullReply.String()), err
	}
	if emitOperation {
		if err := emit(StreamEvent{Type: StreamEventOperation, Status: StreamCompleted, Tool: string(ToolExplainDraft)}); err != nil {
			return 0, 0, AssistantReplyQualityWarnings(fullReply.String()), err
		}
	}
	return 0, 0, AssistantReplyQualityWarnings(fullReply.String()), nil
}

func normalizeIntentForQuery(query string, intent Intent, resolver Resolver) Intent {
	categories := technologyCategoriesFromQuery(query, resolver)
	if len(categories) != 1 {
		return intent
	}
	category := categories[0]
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
		normalizeRankingLimitForQuery(query, subQuery, resolver)
	}
	return normalized
}

// normalizeRankingLimitForQuery supplies the numeric default that the model
// cannot infer from the phrase “产品默认数量”. Explicit singular questions
// remain top-one; plural/category questions that were accidentally emitted as
// top-one receive the bounded product default before candidate allocation.
func normalizeRankingLimitForQuery(query string, subQuery *SubQuery, resolver Resolver) {
	if subQuery == nil {
		return
	}
	value := normalizeExpression(query)
	if hasExplicitRankingCount(value) || !isDefaultRankingQuery(value, subQuery, resolver) {
		return
	}
	subQuery.Ranking.Limit = DefaultRankingLimit
}

func hasExplicitRankingCount(query string) bool {
	for _, marker := range []string{"前1", "前2", "前3", "前4", "前5", "前6", "前7", "前8", "前 1", "前 2", "前 3", "前 4", "前 5", "前 6", "前 7", "前 8", "前一", "前二", "前三", "前四", "前五", "前六", "前七", "前八", "top 1", "top 2", "top 3", "top 4", "top 5", "top 6", "top 7", "top 8"} {
		if strings.Contains(query, marker) {
			return true
		}
	}
	return false
}

func isDefaultRankingQuery(query string, subQuery *SubQuery, resolver Resolver) bool {
	if isExplicitSingleRankingQuery(query) {
		return false
	}
	if subQuery == nil {
		return false
	}
	plural := strings.Contains(query, "哪些") || strings.Contains(query, "有哪些") || strings.Contains(query, "看看趋势") || strings.Contains(query, "全部") || strings.Contains(query, "所有")
	if subQuery.Dimension == DimensionTechnology && len(technologyCategoriesFromQuery(query, resolver)) > 0 {
		return true
	}
	if plural {
		for _, expression := range subQuery.Expressions {
			if isBroadExpression(expression) {
				return true
			}
		}
	}
	return false
}

func isExplicitSingleRankingQuery(query string) bool {
	if query == "" || strings.Contains(query, "哪些") || strings.Contains(query, "有哪些") || strings.Contains(query, "所有") || strings.Contains(query, "全部") {
		return false
	}
	for _, marker := range []string{"哪一个", "哪个", "谁第一", "第一名", "top 1", "top one", "使用最多的", "岗位最多的"} {
		if strings.Contains(query, marker) {
			return true
		}
	}
	return false
}

func technologyCategoriesFromQuery(query string, resolver Resolver) []string {
	value := normalizeExpression(query)
	if value == "" {
		return nil
	}
	type categoryMatch struct {
		alias  string
		length int
	}
	matches := make([]categoryMatch, 0)
	for alias := range resolver.Aliases[DimensionTechnology] {
		if alias == "" || !strings.Contains(value, alias) {
			continue
		}
		matches = append(matches, categoryMatch{alias: alias, length: len([]rune(alias))})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].length != matches[j].length {
			return matches[i].length > matches[j].length
		}
		return matches[i].alias < matches[j].alias
	})
	result := make([]string, 0, len(matches))
	for _, match := range matches {
		duplicate := false
		for _, existing := range result {
			if existing == match.alias {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result = append(result, match.alias)
		}
	}
	return result
}

func hasProjectTechnologyConstraint(query string, intent Intent, resolver Resolver) bool {
	hasTechnology := false
	for _, subQuery := range intent.SubQueries {
		if subQuery.Dimension == DimensionTechnology {
			hasTechnology = true
			break
		}
	}
	if !hasTechnology {
		return false
	}
	needle := normalizeExpression(query)
	for _, project := range resolver.Values[DimensionProject] {
		if strings.Contains(needle, normalizeExpression(project.Value)) || strings.Contains(needle, normalizeExpression(project.Label)) {
			return true
		}
	}
	return strings.Contains(needle, "项目") && strings.Contains(needle, "技术") && len(resolver.Values[DimensionProject]) > 0
}

func conflictingTrendRanges(query string) bool {
	value := normalizeExpression(query)
	month := strings.Contains(value, "本月") || strings.Contains(value, "这个月")
	year := strings.Contains(value, "去年") || strings.Contains(value, "上一年")
	quarter := strings.Contains(value, "本季度") || strings.Contains(value, "这个季度")
	week := strings.Contains(value, "本周") || strings.Contains(value, "这周")
	count := 0
	for _, found := range []bool{month, year, quarter, week} {
		if found {
			count++
		}
	}
	return count > 1
}

func unsupportedMetricPhrase(query string) bool {
	value := normalizeExpression(query)
	return strings.Contains(value, "占比") || strings.Contains(value, "比例") || strings.Contains(value, "百分比") || strings.Contains(value, "变化率")
}

func unsupportedChartPhrase(query string) bool {
	value := normalizeExpression(query)
	return strings.Contains(value, "柱状图") || strings.Contains(value, "柱形图") || strings.Contains(value, "饼图") || strings.Contains(value, "bar chart") || strings.Contains(value, "pie chart")
}

type auditOperation struct {
	Tool          string
	Status        StreamStatus
	SafeErrorCode *string
	StartedAt     time.Time
	EndedAt       time.Time
}

func appendAuditOperation(operations []auditOperation, event StreamEvent) []auditOperation {
	if event.Tool == string(ToolClassifyServiceModes) || event.Tool == string(ToolAnalyzeIntent) || event.Tool == string(ToolExplainDraft) {
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
	contractVersion := ContractVersionV1
	if result.AgentResponse.ContractVersion != "" {
		contractVersion = result.AgentResponse.ContractVersion
	}
	record := AuditRecord{
		RequestHash: requestHash, ContractVersion: contractVersion, ValidationStatus: "valid",
		Status: status, AIAttemptCount: result.AIAttempts, ToolCallCount: len(operations),
		InputTokenCount: inputTokens, OutputTokenCount: outputTokens, Duration: completedAt.Sub(createdAt),
		ResponseModes:        append([]ResponseMode(nil), result.AgentResponse.ResponseModes...),
		ServiceTags:          append([]ServiceTag(nil), result.ServiceTags...),
		ClassificationStatus: result.ClassificationStatus, ClassificationInputTokenCount: result.ClassificationInput,
		ClassificationOutputTokenCount: result.ClassificationOutput, ClassificationDuration: result.ClassificationTime,
		ReplyWarningCount: len(result.ReplyQualityWarnings),
		CreatedAt:         createdAt, CompletedAt: &completedAt, ExpiresAt: createdAt.Add(AuditRetention),
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
	categoryAliasesByDimension := map[Dimension]map[string]struct{}{DimensionTechnology: make(map[string]struct{})}
	for _, option := range options.TechnologyKeywords {
		values[DimensionTechnology] = append(values[DimensionTechnology], Candidate{Dimension: DimensionTechnology, Value: option.Value, Label: option.Label, MatchStatus: MatchExact})
		key := normalizeExpression(option.Category.Value)
		if key != "" {
			aliases[DimensionTechnology][key] = append(aliases[DimensionTechnology][key], option.Value)
			categoryAliasesByDimension[DimensionTechnology][key] = struct{}{}
		}
		for _, categoryAlias := range categoryAliases(option.Category.Label) {
			aliases[DimensionTechnology][categoryAlias] = append(aliases[DimensionTechnology][categoryAlias], option.Value)
			categoryAliasesByDimension[DimensionTechnology][categoryAlias] = struct{}{}
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
	return Resolver{Values: values, Aliases: aliases, CategoryAliases: categoryAliasesByDimension}
}

func candidateGroups(intent Intent, resolver Resolver) []CandidateGroup {
	groups := make([]CandidateGroup, 0, len(intent.SubQueries))
	for _, query := range intent.SubQueries {
		candidates := make([]Candidate, 0)
		broad := false
		for _, expression := range query.Expressions {
			if isBroadExpression(expression) {
				broad = true
			}
			if len(candidates) >= MaxToolCandidates && !broad {
				break
			}
			candidates = append(candidates, resolveIntentExpression(resolver, query, expression)...)
		}
		candidates = MergeCandidates(candidates)
		if !broad && len(candidates) > MaxToolCandidates {
			candidates = candidates[:MaxToolCandidates]
		}
		groups = append(groups, CandidateGroup{SubQueryID: query.ID, Candidates: candidates, Limit: query.Ranking.Limit})
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
	for _, marker := range []string{"项目", "岗位族", "地点", "project", "projects", "job family", "job families", "location"} {
		if value == marker {
			return true
		}
		if strings.Contains(value, marker) && (strings.Contains(value, "哪些") || strings.HasPrefix(value, "所有") || strings.HasPrefix(value, "全部")) {
			return true
		}
	}
	return false
}

func categoryAliases(label string) []string {
	normalized := normalizeExpression(label)
	if normalized == "" {
		return nil
	}
	result := []string{normalized}
	parts := strings.FieldsFunc(normalized, func(r rune) bool {
		switch r {
		case '与', '和', '/', '|', ',', '，':
			return true
		default:
			return false
		}
	})
	for _, part := range parts {
		part = normalizeExpression(part)
		if part == "" || part == normalized {
			continue
		}
		result = append(result, part)
	}
	return result
}

func planServingTools(intent Intent, groups []CandidateGroup) []ToolCall {
	// Reserve one registry call for final draft validation.
	maxPlanningCalls := MaxToolCalls - 1
	calls := make([]ToolCall, 0, maxPlanningCalls)
	rankedSubQueries := make(map[string]struct{})
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
		if expression == "" {
			continue
		}
		if isBroadExpression(expression) {
			calls = append(calls, ToolCall{Name: ToolRankTrendValues, Params: mustMarshal(trendRankRequest{Dimension: query.Dimension, Range: RangeKind(intent.TimeRange.Kind), StartDate: intent.TimeRange.StartDate, EndDate: intent.TimeRange.EndDate, Metric: query.Ranking.Metric, Limit: query.Ranking.Limit, SubQueryID: query.ID, IncludeSecondary: query.IncludeSecondary != nil && *query.IncludeSecondary})})
			rankedSubQueries[query.ID] = struct{}{}
			continue
		}
		calls = append(calls, ToolCall{Name: ToolRankValues, Params: mustMarshal(rankRequest{Dimension: query.Dimension, Expression: expression, SubQueryID: query.ID})})
	}
	requests := make([]trendToolRequest, 0, MaxBatchCandidates)
	for _, group := range groups {
		if _, ranked := rankedSubQueries[group.SubQueryID]; ranked {
			continue
		}
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

func applyTrendRankResults(groups []CandidateGroup, results []ToolCallResult) bool {
	rankedBySubQuery := make(map[string][]rankedTrendValue)
	for _, result := range results {
		if result.Name != ToolRankTrendValues || !result.Available {
			continue
		}
		var response rankedTrendValuesResponse
		if err := json.Unmarshal(result.Payload, &response); err != nil {
			continue
		}
		for _, value := range response.Values {
			rankedBySubQuery[value.SubQueryID] = append(rankedBySubQuery[value.SubQueryID], value)
		}
	}
	if len(rankedBySubQuery) == 0 {
		return false
	}
	for index := range groups {
		ranked, ok := rankedBySubQuery[groups[index].SubQueryID]
		if !ok {
			continue
		}
		byValue := make(map[string]Candidate, len(groups[index].Candidates))
		for _, candidate := range groups[index].Candidates {
			byValue[candidate.Value] = candidate
		}
		filtered := make([]Candidate, 0, len(ranked))
		for rank, value := range ranked {
			candidate, exists := byValue[value.Value]
			if !exists {
				candidate = Candidate{Dimension: value.Dimension, Value: value.Value, Label: value.Label, MatchStatus: MatchExact, SubQueryID: groups[index].SubQueryID}
			}
			candidate.Score = len(ranked) - rank
			filtered = append(filtered, candidate)
		}
		groups[index].Candidates = filtered
	}
	return true
}

func boundBroadCandidates(groups []CandidateGroup) {
	for index := range groups {
		if len(groups[index].Candidates) > MaxToolCandidates {
			groups[index].Candidates = groups[index].Candidates[:MaxToolCandidates]
		}
	}
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

type trendSummaryCoverageKey struct {
	Dimension        Dimension
	Value            string
	IncludeSecondary bool
	Operator         string
}

func candidateSummaryCoverageKey(candidate Candidate) trendSummaryCoverageKey {
	operator := ""
	if candidate.Operator != nil {
		operator = string(*candidate.Operator)
	}
	return trendSummaryCoverageKey{Dimension: candidate.Dimension, Value: candidate.Value, IncludeSecondary: candidate.IncludeSecondary, Operator: operator}
}

func summaryCoverageKey(summary summaryToolResponse) trendSummaryCoverageKey {
	return trendSummaryCoverageKey{
		Dimension: Dimension(summary.Curve.Dimension), Value: summary.Curve.Value,
		IncludeSecondary: summary.Curve.IncludeSecondary, Operator: string(summary.Curve.Operator),
	}
}

// completeMissingTrendSummaries makes the model's bounded tool exchange an
// input to ranking rather than the ranking boundary. A category may contain
// more values than the model chose to put in its first batch; the server owns
// the formal snapshot, so it can safely fetch the remaining aggregates and
// sort the complete candidate set before building the Draft.
func completeMissingTrendSummaries(ctx context.Context, registry *ToolRegistry, groups []CandidateGroup, intent Intent, results *[]ToolCallResult, emit func(StreamEvent) error) (bool, error) {
	if registry == nil || results == nil {
		return false, nil
	}
	covered := make(map[trendSummaryCoverageKey]struct{})
	rankedSubQueries := make(map[string]struct{})
	for _, result := range *results {
		if result.Name == ToolRankTrendValues && result.Available {
			var ranked rankedTrendValuesResponse
			if json.Unmarshal(result.Payload, &ranked) == nil {
				for _, value := range ranked.Values {
					rankedSubQueries[value.SubQueryID] = struct{}{}
				}
			}
			continue
		}
		if !result.Available || (result.Name != ToolBatchTrendSummary && result.Name != ToolReadTrendSummary) {
			continue
		}
		if result.Name == ToolReadTrendSummary {
			var summary summaryToolResponse
			if json.Unmarshal(result.Payload, &summary) == nil {
				covered[summaryCoverageKey(summary)] = struct{}{}
			}
			continue
		}
		var summaries []summaryToolResponse
		if json.Unmarshal(result.Payload, &summaries) != nil {
			continue
		}
		for _, summary := range summaries {
			covered[summaryCoverageKey(summary)] = struct{}{}
		}
	}

	requests := make([]trendToolRequest, 0)
	requestKeys := make(map[trendSummaryCoverageKey]struct{})
	for _, group := range groups {
		if _, ranked := rankedSubQueries[group.SubQueryID]; ranked {
			continue
		}
		for _, candidate := range group.Candidates {
			key := candidateSummaryCoverageKey(candidate)
			if _, ok := covered[key]; ok {
				continue
			}
			if _, ok := requestKeys[key]; ok {
				continue
			}
			requestKeys[key] = struct{}{}
			requests = append(requests, candidateTrendRequest(intent.TimeRange, candidate))
		}
	}
	if len(requests) == 0 {
		return false, nil
	}

	remainingCalls := MaxToolCalls - registry.Calls() - 1 // reserve final draft validation
	if remainingCalls < 0 {
		remainingCalls = 0
	}
	allowed := remainingCalls * MaxBatchCandidates
	partial := false
	if allowed < len(requests) {
		partial = true
		requests = requests[:allowed]
	}
	for start := 0; start < len(requests); start += MaxBatchCandidates {
		end := start + MaxBatchCandidates
		if end > len(requests) {
			end = len(requests)
		}
		batch := requests[start:end]
		params := mustMarshal(batch)
		if err := recordDebug(ctx, DebugTraceEvent{Type: DebugEventToolCall, Tool: string(ToolBatchTrendSummary), Params: debugJSON(params)}); err != nil {
			return partial, err
		}
		if err := emit(StreamEvent{Type: StreamEventOperation, Status: StreamRunning, Tool: string(ToolBatchTrendSummary), Params: params}); err != nil {
			return partial, err
		}
		toolResult, callErr := registry.Call(ctx, ToolCall{Name: ToolBatchTrendSummary, Params: params})
		if callErr != nil {
			partial = true
			toolResult.Name = ToolBatchTrendSummary
			toolResult.SafeError = safeErrorFrom(callErr)
			toolResult.Available = false
		} else {
			if toolResult.SafeError == nil {
				toolResult.SafeError = batchTrendPayloadFailure(toolResult.Payload)
			}
			if toolResult.SafeError == nil {
				toolResult.SafeError = batchTrendPayloadPartialFailure(toolResult.Payload)
			}
			if toolResult.SafeError != nil || batchTrendPayloadHasItemErrors(toolResult.Payload) {
				partial = true
			}
		}
		status := StreamCompleted
		if toolResult.SafeError != nil {
			status = StreamFailed
		}
		if err := emit(StreamEvent{Type: StreamEventOperation, Status: status, Tool: string(ToolBatchTrendSummary), Params: params, Error: toolResult.SafeError}); err != nil {
			return partial, err
		}
		debugResult := DebugTraceEvent{Type: DebugEventToolResult, Tool: string(ToolBatchTrendSummary), Params: debugJSON(params), Result: debugJSON(toolResult.Payload), Status: string(status), Error: toolResult.SafeError}
		if err := recordDebug(ctx, debugResult); err != nil {
			return partial, err
		}
		*results = append(*results, toolResult)
	}
	return partial, nil
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

func filterCandidatesToSummaryResults(groups []CandidateGroup, results []ToolCallResult) {
	queried := make(map[string]struct{})
	for _, result := range results {
		if result.Name == ToolRankTrendValues && result.Available {
			var ranked rankedTrendValuesResponse
			if json.Unmarshal(result.Payload, &ranked) == nil {
				for _, value := range ranked.Values {
					queried[explanationCurveKey(Dimension(value.Dimension), value.Value, false)] = struct{}{}
					queried[explanationCurveKey(Dimension(value.Dimension), value.Value, true)] = struct{}{}
				}
			}
			continue
		}
		if !result.Available || (result.Name != ToolBatchTrendSummary && result.Name != ToolReadTrendSummary) {
			continue
		}
		if result.Name == ToolReadTrendSummary {
			var summary summaryToolResponse
			if json.Unmarshal(result.Payload, &summary) == nil && summary.Error == nil {
				queried[explanationCurveKey(Dimension(summary.Curve.Dimension), summary.Curve.Value, summary.Curve.IncludeSecondary)] = struct{}{}
			}
			continue
		}
		var summaries []summaryToolResponse
		if json.Unmarshal(result.Payload, &summaries) != nil {
			continue
		}
		for _, summary := range summaries {
			if summary.Error == nil {
				queried[explanationCurveKey(Dimension(summary.Curve.Dimension), summary.Curve.Value, summary.Curve.IncludeSecondary)] = struct{}{}
			}
		}
	}
	for groupIndex := range groups {
		kept := groups[groupIndex].Candidates[:0]
		for _, candidate := range groups[groupIndex].Candidates {
			if _, ok := queried[explanationCurveKey(candidate.Dimension, candidate.Value, candidate.IncludeSecondary)]; ok {
				kept = append(kept, candidate)
			}
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
