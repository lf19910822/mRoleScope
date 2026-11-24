package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"mrolescope/internal/aitrend"
	"mrolescope/internal/aitrend/evaluation"
)

type fixtureProvider struct {
	item evaluation.Case
	turn int
}

func (provider fixtureProvider) ClassifyServiceModes(context.Context, string) (aitrend.ServiceModeClassification, int, int, error) {
	tags := append([]string(nil), provider.item.ExpectedServiceTags...)
	if len(tags) == 0 {
		if isFixtureGuidance(provider.item) {
			tags = []string{string(aitrend.ServiceTagGuidance)}
		} else {
			tags = []string{string(aitrend.ServiceTagTrend)}
		}
	}
	serviceTags := make([]aitrend.ServiceTag, 0, len(tags))
	for _, tag := range tags {
		serviceTags = append(serviceTags, aitrend.ServiceTag(tag))
	}
	return aitrend.ServiceModeClassification{ContractVersion: aitrend.ServiceModeContractVersionV1, ServiceTags: serviceTags}, 3, 1, nil
}

func (provider fixtureProvider) AnalyzeIntent(_ context.Context, _ string, _ *aitrend.ContractError) ([]byte, error) {
	if isFixtureGuidance(provider.item) {
		return json.Marshal(aitrend.AgentResponse{ContractVersion: aitrend.AgentContractVersionV1, ResponseModes: []aitrend.ResponseMode{aitrend.ResponseModeGuidance}})
	}
	if provider.item.ExpectedStatus == "failure" {
		return nil, &aitrend.ContractError{Code: aitrend.ErrorInvalidCombination, Field: "time_range", Message: "fixture conflict requires clarification"}
	}
	data, err := json.Marshal(fixtureIntent(provider.item))
	return data, err
}

func (provider *fixtureProvider) AnalyzeIntentTurn(_ context.Context, _ string, _ *aitrend.ContractError, _ []aitrend.ToolConversation, _ bool) (aitrend.IntentToolTurn, error) {
	if isFixtureGuidance(provider.item) {
		data, err := json.Marshal(aitrend.AgentResponse{ContractVersion: aitrend.AgentContractVersionV1, ResponseModes: []aitrend.ResponseMode{aitrend.ResponseModeGuidance}})
		return aitrend.IntentToolTurn{Intent: data, InputTokens: 12, OutputTokens: 6}, err
	}
	if provider.item.ExpectedStatus == "failure" {
		return aitrend.IntentToolTurn{}, &aitrend.ContractError{Code: aitrend.ErrorInvalidCombination, Field: "time_range", Message: "fixture conflict requires clarification"}
	}
	categories := make([]string, 0, len(provider.item.ExpectedCategories))
	for _, category := range provider.item.ExpectedCategories {
		categories = append(categories, category)
	}
	if provider.item.ExpectedCategory != "" {
		categories = append(categories, provider.item.ExpectedCategory)
	}
	sort.Strings(categories)
	if provider.turn < len(categories) {
		category := categories[provider.turn]
		provider.turn++
		return aitrend.IntentToolTurn{ToolCalls: []aitrend.ToolCall{{ID: "category", Name: aitrend.ToolListCategoryValues, Params: mustJSON(map[string]string{"category": category})}}, InputTokens: 10, OutputTokens: 4}, nil
	}
	if provider.item.FaultInjection == "invalid_tool_params_once" && provider.turn == len(categories) {
		// The first category request is corrupted by faultInjectingProvider;
		// keep the fixture conversation alive so the correction can be observed
		// before the summary and final Intent turns.
		if len(categories) == 0 {
			categories = append(categories, fixtureInferredCategory(provider.item))
		}
		provider.turn++
		category := categories[0]
		return aitrend.IntentToolTurn{ToolCalls: []aitrend.ToolCall{{ID: "category-corrected", Name: aitrend.ToolListCategoryValues, Params: mustJSON(map[string]string{"category": category})}}, InputTokens: 10, OutputTokens: 4}, nil
	}
	summaryTurn := len(categories)
	if provider.item.FaultInjection == "invalid_tool_params_once" {
		summaryTurn++
	}
	if provider.turn == summaryTurn {
		provider.turn++
		return aitrend.IntentToolTurn{ToolCalls: []aitrend.ToolCall{{ID: "summary", Name: aitrend.ToolBatchTrendSummary, Params: mustJSON(fixtureSummaryRequests(provider.item))}}, InputTokens: 12, OutputTokens: 5}, nil
	}
	data, err := json.Marshal(fixtureAgentResponse(provider.item))
	if err != nil {
		return aitrend.IntentToolTurn{}, err
	}
	provider.turn++
	return aitrend.IntentToolTurn{Intent: data, InputTokens: 14, OutputTokens: 7}, nil
}

func isFixtureGuidance(item evaluation.Case) bool {
	return len(item.ExpectedResponseModes) == 1 && item.ExpectedResponseModes[0] == "guidance" && item.ExpectedDraft != nil && !*item.ExpectedDraft
}

func fixtureAgentResponse(item evaluation.Case) any {
	intent := fixtureIntent(item)
	if len(item.ExpectedResponseModes) == 0 {
		return intent
	}
	modes := make([]aitrend.ResponseMode, 0, len(item.ExpectedResponseModes))
	for _, mode := range item.ExpectedResponseModes {
		modes = append(modes, aitrend.ResponseMode(mode))
	}
	return aitrend.AgentResponse{ContractVersion: aitrend.AgentContractVersionV1, ResponseModes: modes, TrendIntent: &intent}
}

// fixtureSummaryRequests supplies the mandatory discovery evidence that a
// real model would obtain before returning its final Intent. It deliberately
// uses only formal values present in the evaluation snapshot; the subsequent
// production planner still performs the actual complete ranking/query path.
func fixtureSummaryRequests(item evaluation.Case) []map[string]any {
	rangeValue := item.ExpectedTimeRange
	if rangeValue == "" {
		rangeValue = item.TimeRange
	}
	if rangeValue == "" || rangeValue == "conflict" {
		rangeValue = "all"
	}
	requests := make([]map[string]any, 0)
	for index, dimension := range fixtureDimensions(item) {
		values := fixtureSummaryValues(item, dimension, index)
		for _, value := range values {
			requests = append(requests, map[string]any{"dimension": dimension, "value": value, "range": rangeValue})
		}
	}
	if len(requests) == 0 {
		requests = append(requests, map[string]any{"dimension": item.Dimension, "value": "*", "range": rangeValue})
	}
	return requests
}

func fixtureDimensions(item evaluation.Case) []string {
	if len(item.ExpectedDimensions) > 0 {
		return item.ExpectedDimensions
	}
	if item.Dimension != "" {
		return []string{item.Dimension}
	}
	return nil
}

func fixtureSummaryValues(item evaluation.Case, dimension string, index int) []string {
	if index == 0 && dimension == item.Dimension && containsExpectedError(item.ExpectedErrors, "unavailable_value") {
		return append([]string(nil), item.Expressions...)
	}
	if index == 0 && dimension == item.Dimension && len(item.ExpectedValues) > 0 && !containsFixtureWildcard(item.ExpectedValues) {
		return append([]string(nil), item.ExpectedValues...)
	}
	if category, ok := item.ExpectedCategories[dimension]; ok {
		return fixtureCategoryValues(category)
	}
	if dimension == item.Dimension && item.ExpectedCategory != "" {
		return fixtureCategoryValues(item.ExpectedCategory)
	}
	if dimension == item.Dimension {
		for _, expression := range item.Expressions {
			if expression != "" && expression != "*" && !isFixtureCategory(expression) {
				return []string{expression}
			}
		}
	}
	switch dimension {
	case "technology":
		return []string{"go"}
	case "project":
		return []string{"game.genshin-impact"}
	case "job_family":
		return []string{"frontend"}
	case "location":
		return []string{"上海"}
	default:
		return nil
	}
}

func containsExpectedError(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func fixtureCategoryValues(category string) []string {
	switch category {
	case "programming-languages":
		return []string{"go", "python"}
	case "databases-and-storage":
		return []string{"mysql", "redis"}
	case "game-engines":
		return []string{"unreal-engine"}
	case "cloud-native":
		return []string{"docker"}
	default:
		return []string{"go"}
	}
}

func fixtureInferredCategory(item evaluation.Case) string {
	if item.Dimension != string(aitrend.DimensionTechnology) {
		return "programming-languages"
	}
	for _, expression := range item.Expressions {
		switch expression {
		case "redis", "mysql":
			return "databases-and-storage"
		case "unreal-engine":
			return "game-engines"
		case "docker":
			return "cloud-native"
		}
	}
	return "programming-languages"
}

func containsFixtureWildcard(values []string) bool {
	for _, value := range values {
		if value == "*" {
			return true
		}
	}
	return false
}

func isFixtureCategory(value string) bool {
	switch value {
	case "programming-languages", "databases-and-storage", "game-engines", "cloud-native":
		return true
	default:
		return false
	}
}

func fixtureIntent(item evaluation.Case) aitrend.Intent {
	dimensions := append([]string(nil), item.ExpectedDimensions...)
	if len(dimensions) == 0 {
		dimensions = []string{item.Dimension}
	}
	subQueries := make([]aitrend.SubQuery, 0, len(dimensions))
	for index, dimension := range dimensions {
		expressions := []string{"*"}
		if category, ok := item.ExpectedCategories[dimension]; ok {
			expressions = []string{category}
		} else if dimension == item.Dimension {
			expressions = append([]string(nil), item.Expressions...)
		}
		subQueries = append(subQueries, aitrend.SubQuery{
			ID: fmt.Sprintf("fixture-%d", index+1), Dimension: aitrend.Dimension(dimension), Expressions: expressions,
			Exclusions: append([]string(nil), item.Exclusions...),
			Ranking:    aitrend.RankingIntent{Metric: fixtureMetric(item.RankingMetric), Order: fixtureOrder(item.RankingOrder), Limit: minLimit(item.Limit)},
		})
	}
	intent := aitrend.Intent{
		ContractVersion: aitrend.ContractVersionV1,
		SubQueries:      subQueries,
		TimeRange: aitrend.TimeRange{Kind: fixtureRange(func() string {
			if item.ExpectedTimeRange != "" {
				return item.ExpectedTimeRange
			}
			return item.TimeRange
		}())},
		Output: aitrend.OutputIntent{SeriesMetric: aitrend.SeriesMetricDailyActiveJobs, ChartType: aitrend.ChartTypeLine},
	}
	if item.IncludeSecondary != nil && len(intent.SubQueries) > 0 {
		intent.SubQueries[0].IncludeSecondary = item.IncludeSecondary
	}
	return intent
}

func mustJSON(value any) []byte {
	data, _ := json.Marshal(value)
	return data
}

func fixtureMetric(metric string) aitrend.Metric {
	switch metric {
	case string(aitrend.MetricPeriodEndActiveJobs):
		return aitrend.MetricPeriodEndActiveJobs
	case string(aitrend.MetricPeakDailyActiveJobs):
		return aitrend.MetricPeakDailyActiveJobs
	case string(aitrend.MetricPeriodUniqueJobs):
		return aitrend.MetricPeriodUniqueJobs
	default:
		return aitrend.MetricAverageDailyActiveJobs
	}
}

func fixtureOrder(order string) aitrend.Order {
	if order == string(aitrend.OrderAscending) {
		return aitrend.OrderAscending
	}
	return aitrend.OrderDescending
}

func fixtureRange(value string) aitrend.RangeKind {
	switch value {
	case string(aitrend.RangeWeek):
		return aitrend.RangeWeek
	case string(aitrend.RangeQuarter):
		return aitrend.RangeQuarter
	case string(aitrend.RangeYear):
		return aitrend.RangeYear
	case string(aitrend.RangeAll), "":
		return aitrend.RangeAll
	default:
		return aitrend.RangeMonth
	}
}

func minLimit(value int) int {
	if value < 1 {
		return 1
	}
	if value > aitrend.MaxCurves {
		return aitrend.MaxCurves
	}
	return value
}

func main() {
	var casesPath, snapshotPath, outputDir, runID, suite, providerMode, caseIDs string
	flag.StringVar(&casesPath, "cases", "Docs/开发指导/AI评测/benchmarks/AI趋势智能体Benchmark-v1.md", "reviewed benchmark Markdown path")
	flag.StringVar(&suite, "suite", "trend", "benchmark suite: trend or agent")
	flag.StringVar(&providerMode, "provider", evaluationProviderFixture, "model provider: fixture or qwen")
	flag.StringVar(&snapshotPath, "snapshot", "internal/aitrend/evaluation/testdata/serving-snapshot-v3.json", "evaluation-only Serving snapshot path")
	flag.StringVar(&outputDir, "output", evaluation.DefaultArtifactRoot, "independent evaluation artifact directory")
	flag.StringVar(&runID, "run-id", "", "evaluation run ID (default: generated from UTC time)")
	flag.StringVar(&caseIDs, "case-ids", "", "optional comma-separated case IDs for a bounded review sample")
	flag.Parse()
	if suite != "trend" && suite != "agent" {
		fatal(fmt.Errorf("unknown benchmark suite %q", suite))
	}

	if runID == "" {
		runID = "run-" + time.Now().UTC().Format("20060102T150405Z")
	}
	var cases []evaluation.Case
	var err error
	if suite == "agent" {
		if casesPath == "Docs/开发指导/AI评测/benchmarks/AI趋势智能体Benchmark-v1.md" {
			casesPath = "Docs/开发指导/AI评测/benchmarks/AI趋势智能体AgentBenchmark-v1.md"
		}
		cases, err = evaluation.LoadAgentFile(casesPath)
	} else {
		cases, err = evaluation.LoadFile(casesPath)
	}
	if err != nil {
		fatal(err)
	}
	if caseIDs != "" {
		cases, err = selectCasesByID(cases, caseIDs)
		if err != nil {
			fatal(err)
		}
	}
	snapshot, err := evaluation.LoadSnapshot(snapshotPath)
	if err != nil {
		fatal(err)
	}
	if err := snapshot.ValidateCases(cases); err != nil {
		fatal(err)
	}
	if err := validateEvaluationProviderConfiguration(providerMode, http.DefaultClient, os.LookupEnv); err != nil {
		fatal(err)
	}
	version, err := evaluation.NewEvaluationVersion(evaluation.VersionInput{
		RunID: runID, Model: evaluationModelName(providerMode), Provider: evaluationProviderName(providerMode),
		PromptVersion: "intent-prompt-v1", ToolSchemaVersion: "aitrend-tools-v1",
		SnapshotVersion: snapshot.Version(), CodeRevision: "local", CasesVersion: func() string {
			if suite == "agent" {
				return evaluation.AgentBenchmarkVersion
			}
			return evaluation.BenchmarkVersion
		}(), ScorerVersion: "scorer-v1",
	}, time.Now().UTC())
	if err != nil {
		fatal(err)
	}
	executor, err := evaluation.NewExecutor(func(ctx context.Context, item evaluation.Case) (evaluation.Result, error) {
		provider, providerErr := newEvaluationProvider(providerMode, item, http.DefaultClient, os.LookupEnv)
		if providerErr != nil {
			return evaluation.Result{}, providerErr
		}
		runner, runnerErr := evaluation.NewRunner(provider, snapshot)
		if runnerErr != nil {
			return evaluation.Result{}, runnerErr
		}
		runner.FaultInjection = item.FaultInjection
		return runner.Run(ctx, item.Query)
	})
	if err != nil {
		fatal(err)
	}
	traces := make([]evaluation.Trace, 0, len(cases)*evaluation.DefaultRepetitions)
	for _, item := range cases {
		caseEvaluation, runErr := executor.RunCase(context.Background(), item)
		if runErr != nil {
			fatal(runErr)
		}
		for _, attempt := range caseEvaluation.Attempts {
			score := evaluation.ScoreCase(item, attempt.Result, errorForAttempt(attempt))
			trace, traceErr := evaluation.BuildTrace(item, version, attempt, score, evaluation.DefaultUsageRates())
			if traceErr != nil {
				fatal(traceErr)
			}
			traces = append(traces, trace)
			name := fmt.Sprintf("%s-r%d-retry%d.trace.json", strings.ToLower(item.ID), attempt.Repetition, attempt.Retry)
			if _, writeErr := evaluation.WriteJSONArtifact(outputDir, name, trace); writeErr != nil {
				fatal(writeErr)
			}
		}
	}
	var report evaluation.Report
	if suite == "agent" {
		report, err = evaluation.BuildAgentReport(cases, traces, version)
	} else {
		report, err = evaluation.BuildReport(cases, traces, version)
	}
	if err != nil {
		fatal(err)
	}
	if _, err := evaluation.WriteJSONArtifact(outputDir, "report.json", report); err != nil {
		fatal(err)
	}
	fmt.Printf("evaluation=%s cases=%d traces=%d output=%s\n", version.ID(), len(cases), len(traces), filepath.Clean(outputDir))
}

func selectCasesByID(cases []evaluation.Case, raw string) ([]evaluation.Case, error) {
	requested := strings.Split(raw, ",")
	if len(requested) == 0 || len(requested) > 6 {
		return nil, fmt.Errorf("case-ids must contain between 1 and 6 IDs")
	}
	seen := make(map[string]struct{}, len(requested))
	for index := range requested {
		requested[index] = strings.TrimSpace(requested[index])
		if requested[index] == "" {
			return nil, fmt.Errorf("case-ids contains an empty ID")
		}
		if _, ok := seen[requested[index]]; ok {
			return nil, fmt.Errorf("case-ids contains duplicate ID %q", requested[index])
		}
		seen[requested[index]] = struct{}{}
	}
	byID := make(map[string]evaluation.Case, len(cases))
	for _, item := range cases {
		byID[item.ID] = item
	}
	selected := make([]evaluation.Case, 0, len(requested))
	for _, id := range requested {
		item, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("case-ids references unknown case %q", id)
		}
		selected = append(selected, item)
	}
	return selected, nil
}

func evaluationModelName(providerMode string) string {
	if strings.EqualFold(strings.TrimSpace(providerMode), evaluationProviderQwen) {
		return "qwen-plus"
	}
	return "fixture-provider"
}

func evaluationProviderName(providerMode string) string {
	if strings.EqualFold(strings.TrimSpace(providerMode), evaluationProviderQwen) {
		return "qwen"
	}
	return "offline-fixture"
}

func errorForAttempt(attempt evaluation.Attempt) error {
	if attempt.ErrorCode == "" {
		return nil
	}
	return &aitrend.ContractError{Code: aitrend.SafeErrorCode(attempt.ErrorCode), Message: "recorded evaluation outcome"}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
