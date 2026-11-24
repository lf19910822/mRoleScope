package evaluation

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"mrolescope/internal/aitrend"
)

type SubScore struct {
	Pass    bool     `json:"pass"`
	Reasons []string `json:"reasons,omitempty"`
}

type Score struct {
	Pass         bool     `json:"pass"`
	Agent        SubScore `json:"agent"`
	ToolCalls    SubScore `json:"tool_calls"`
	Scope        SubScore `json:"scope"`
	Ranking      SubScore `json:"ranking"`
	Draft        SubScore `json:"draft"`
	Explanation  SubScore `json:"explanation"`
	ReplyQuality SubScore `json:"reply_quality"`
	Errors       SubScore `json:"errors"`
}

// ScoreCase applies deterministic constraints from a reviewed case. It does
// not use a model judge and keeps explanation quality advisory.
func ScoreCase(item Case, result Result, runErr error) Score {
	score := Score{}
	score.Agent = scoreAgentResponse(item, result)
	score.ToolCalls = scoreToolCalls(item, result)
	score.Scope = scoreScope(item, result)
	score.Ranking = scoreRanking(item, result)
	score.Draft = scoreDraft(item, result, runErr)
	score.Explanation = scoreExplanation(item, result)
	score.ReplyQuality = scoreReplyQuality(item, result)
	score.Errors = scoreExpectedErrors(item, result, runErr)
	score.Pass = scoreFunctionalPass(item, result, runErr, score)
	return score
}

func scoreFunctionalPass(item Case, result Result, runErr error, score Score) bool {
	if len(item.ExpectedResponseModes) > 0 || item.ExpectedDraft != nil || item.ReplyRequired {
		if runErr != nil {
			return false
		}
		return score.Agent.Pass && score.Errors.Pass && score.Draft.Pass
	}
	if item.ExpectedStatus == "failure" {
		// A clarification/error response may not contain a Draft. Explanation
		// remains advisory, so the expected failure itself is the hard result.
		return runErr != nil && score.Errors.Pass
	}
	if runErr != nil || result.Draft == nil {
		return false
	}
	return score.ToolCalls.Pass && score.Scope.Pass && score.Ranking.Pass && score.Draft.Pass && score.Errors.Pass
}

func scoreAgentResponse(item Case, result Result) SubScore {
	if len(item.ExpectedResponseModes) == 0 && item.ExpectedDraft == nil && !item.ReplyRequired {
		return passed()
	}
	if result.Stats.Intent == nil && len(result.Stats.ResponseModes) == 0 {
		return failed("Agent response mode is missing")
	}
	actualModes := make(map[string]struct{}, len(result.Stats.ResponseModes))
	for _, mode := range result.Stats.ResponseModes {
		actualModes[string(mode)] = struct{}{}
	}
	if len(item.ExpectedResponseModes) > 0 && !sameSet(actualModes, normalizedSet(item.ExpectedResponseModes)) {
		return failed(fmt.Sprintf("response modes %v do not equal expected %v", actualModes, item.ExpectedResponseModes))
	}
	if item.ExpectedNoTrendTools {
		for _, call := range operationCalls(result) {
			if call.Tool != string(aitrend.ToolAnalyzeIntent) && call.Tool != string(aitrend.ToolExplainDraft) {
				return failed(fmt.Sprintf("guidance-only response called trend tool %q", call.Tool))
			}
		}
	}
	if item.ReplyRequired && strings.TrimSpace(assistantMessage(result)) == "" {
		return failed("assistant reply is missing")
	}
	if item.ExpectedTerminalStatus != "" && resultTerminalStatus(result) != item.ExpectedTerminalStatus {
		return failed(fmt.Sprintf("terminal status %q does not equal expected %q", resultTerminalStatus(result), item.ExpectedTerminalStatus))
	}
	return passed()
}

func assistantMessage(result Result) string {
	var builder strings.Builder
	for _, event := range result.Events {
		if event.Type == aitrend.StreamEventAssistant {
			builder.WriteString(event.Delta)
		}
	}
	return builder.String()
}

func resultTerminalStatus(result Result) string {
	for index := len(result.Events) - 1; index >= 0; index-- {
		if result.Events[index].Type == aitrend.StreamEventDone {
			return string(result.Events[index].Status)
		}
	}
	return ""
}

func scoreToolCalls(item Case, result Result) SubScore {
	if result.Stats.ToolCalls > MaxEvaluationToolCalls {
		return failed("tool call limit exceeded")
	}
	calls := operationCalls(result)
	if len(calls) == 0 {
		return failed("no tool operation was recorded")
	}
	if item.ExpectedScope == "explicit" || strings.Contains(item.ExpectedScope, "explicit-") {
		if containsWildcard(calls) {
			return failed("explicit-value case used wildcard scope")
		}
	}
	return passed()
}

func scoreScope(item Case, result Result) SubScore {
	if item.ExpectedStatus == "failure" {
		return passed()
	}
	if item.ExpectedDraft != nil && !*item.ExpectedDraft {
		return passed()
	}
	if result.Stats.Intent == nil {
		return failed("final Intent is missing")
	}
	if len(result.Stats.Intent.SubQueries) == 0 {
		return failed("final Intent has no sub-query")
	}
	if item.ExpectedCategory != "" && !hasCategoryLookup(operationCalls(result), item.ExpectedCategory) {
		return failed(fmt.Sprintf("technology category %q was not confirmed", item.ExpectedCategory))
	}
	for dimension, category := range item.ExpectedCategories {
		if dimension != string(aitrend.DimensionTechnology) || !hasCategoryLookup(operationCalls(result), category) {
			return failed(fmt.Sprintf("expected category %q for %s was not confirmed", category, dimension))
		}
	}
	if item.ExpectedSubQueryCount > 0 && len(result.Stats.Intent.SubQueries) != item.ExpectedSubQueryCount {
		return failed(fmt.Sprintf("sub-query count %d does not equal %d", len(result.Stats.Intent.SubQueries), item.ExpectedSubQueryCount))
	}
	if len(item.ExpectedDimensions) > 0 {
		actualDimensions := make(map[string]struct{}, len(result.Stats.Intent.SubQueries))
		for _, query := range result.Stats.Intent.SubQueries {
			actualDimensions[string(query.Dimension)] = struct{}{}
		}
		for _, dimension := range item.ExpectedDimensions {
			if _, ok := actualDimensions[dimension]; !ok {
				return failed(fmt.Sprintf("expected dimension %q is missing", dimension))
			}
		}
	}
	for _, query := range result.Stats.Intent.SubQueries {
		if item.Dimension != "" && item.ExpectedSubQueryCount == 0 && string(query.Dimension) != item.Dimension {
			return failed(fmt.Sprintf("dimension %q does not equal %q", query.Dimension, item.Dimension))
		}
		if item.IncludeSecondary != nil && query.IncludeSecondary == nil {
			return failed("secondary-family preference is missing")
		}
		if item.IncludeSecondary != nil && query.IncludeSecondary != nil && *query.IncludeSecondary != *item.IncludeSecondary {
			return failed("secondary-family preference does not match")
		}
		if len(item.Exclusions) > 0 && !sameSet(normalizedSet(query.Exclusions), normalizedSet(item.Exclusions)) {
			return failed("exclusion conditions do not match")
		}
	}
	actual := draftValues(result.Draft)
	for _, required := range item.RequiredValues {
		if !draftHasValue(result.Draft, aitrend.Dimension(item.Dimension), required) {
			return failed(fmt.Sprintf("required draft value %q is missing", required))
		}
	}
	expected := normalizedSet(item.ExpectedValues)
	switch {
	case item.ExpectedScope == "explicit" || strings.Contains(item.ExpectedScope, "explicit-") || item.ExpectedScope == "alias-normalized":
		if !sameSet(actual, expected) {
			return failed(fmt.Sprintf("draft values %v do not equal expected %v", actual, expected))
		}
	case item.ExpectedScope == "partial-unknown":
		if len(actual) == 0 || !containsAny(actual, expected) {
			return failed("partial result did not retain a valid candidate")
		}
	case item.ExpectedScope == "category" || strings.Contains(item.ExpectedScope, "category-"):
		categoryConfirmed := item.ExpectedCategory != "" && hasCategoryLookup(operationCalls(result), item.ExpectedCategory)
		if (!categoryConfirmed && !intentContainsExpression(result.Stats.Intent, item.Expressions)) || len(actual) == 0 {
			return failed("category scope was not preserved")
		}
	case item.ExpectedScope == "global" || strings.Contains(item.ExpectedScope, "global-") || item.ExpectedScope == "multiple-subqueries":
		if !intentContainsWildcard(result.Stats.Intent) && !containsWildcard(operationCalls(result)) {
			return failed("global scope did not use wildcard")
		}
	case item.ExpectedScope == "clarification-required":
		return passed()
	}
	return passed()
}

func hasCategoryLookup(calls []operationCall, expected string) bool {
	for _, call := range calls {
		if call.Tool != string(aitrend.ToolListCategoryValues) {
			continue
		}
		var params struct {
			Category string `json:"category"`
		}
		if json.Unmarshal(call.Params, &params) == nil && params.Category == expected {
			return true
		}
	}
	return false
}

func scoreRanking(item Case, result Result) SubScore {
	if item.ExpectedStatus == "failure" {
		return passed()
	}
	if item.ExpectedDraft != nil && !*item.ExpectedDraft {
		return passed()
	}
	if result.Stats.Intent == nil || len(result.Stats.Intent.SubQueries) == 0 {
		return failed("ranking Intent is missing")
	}
	for _, query := range result.Stats.Intent.SubQueries {
		if item.RankingMetric != "" && string(query.Ranking.Metric) != item.RankingMetric {
			return failed(fmt.Sprintf("ranking metric %q does not equal %q", query.Ranking.Metric, item.RankingMetric))
		}
		if item.RankingOrder != "none" && string(query.Ranking.Order) != item.RankingOrder {
			return failed(fmt.Sprintf("ranking order %q does not equal %q", query.Ranking.Order, item.RankingOrder))
		}
		if item.Limit > 0 && !rankingLimitMatches(item, result, query.Ranking.Limit) {
			return failed(fmt.Sprintf("ranking limit %d does not equal %d", query.Ranking.Limit, minExpectedLimit(item.Limit)))
		}
	}
	expectedTimeRange := item.ExpectedTimeRange
	if expectedTimeRange == "" {
		expectedTimeRange = item.TimeRange
	}
	if expectedTimeRange != "" && expectedTimeRange != "conflict" && string(result.Stats.Intent.TimeRange.Kind) != expectedTimeRange {
		return failed(fmt.Sprintf("time range %q does not equal %q", result.Stats.Intent.TimeRange.Kind, expectedTimeRange))
	}
	return passed()
}

func minExpectedLimit(limit int) int {
	if limit > aitrend.MaxCurves {
		return aitrend.MaxCurves
	}
	return limit
}

func rankingLimitMatches(item Case, result Result, actual int) bool {
	expected := minExpectedLimit(item.Limit)
	if actual == expected {
		return true
	}
	// A bounded category/global query may have fewer formal candidates than the
	// requested limit. In that case the model may return the actual candidate
	// count, provided the Draft contains exactly that many candidates. Explicit
	// comparisons remain exact because their requested values are known.
	if actual < expected && item.ExpectedScope != "explicit" && !strings.Contains(item.ExpectedScope, "explicit-") && result.Draft != nil && actual > 0 && len(result.Draft.Curves) == actual {
		return true
	}
	return false
}

func scoreExpectedErrors(item Case, result Result, runErr error) SubScore {
	if len(item.ExpectedErrors) == 0 {
		return passed()
	}
	seen := make(map[string]struct{})
	if runErr != nil {
		if contractErr, ok := runErr.(*aitrend.ContractError); ok {
			seen[string(contractErr.Code)] = struct{}{}
			if contractErr.Field == "time_range" {
				seen["conflicting_time_range"] = struct{}{}
			}
		}
	}
	completedTools := make(map[string]bool)
	failedTools := make(map[string]bool)
	contractFailedTools := make(map[string]bool)
	for _, event := range result.Events {
		if event.Error != nil {
			seen[string(event.Error.Code)] = struct{}{}
		}
		if event.Type == aitrend.StreamEventOperation {
			if event.Status == aitrend.StreamFailed {
				failedTools[event.Tool] = true
				if event.Error != nil && isModelContractErrorCode(event.Error.Code) {
					contractFailedTools[event.Tool] = true
				}
			}
			if event.Status == aitrend.StreamCompleted {
				completedTools[event.Tool] = true
			}
		}
		if event.Type == aitrend.StreamEventValidation {
			seen["invalid_tool_params_then_corrected"] = struct{}{}
		}
	}
	for tool := range failedTools {
		if completedTools[tool] && contractFailedTools[tool] {
			seen["invalid_tool_params_then_corrected"] = struct{}{}
		}
	}
	if result.Draft != nil {
		for _, warning := range result.Draft.Warnings {
			if strings.Contains(warning, "暂未按项目过滤") {
				seen["unsupported_cross_dimension_filter"] = struct{}{}
			}
			if strings.Contains(warning, "占比") || strings.Contains(warning, "变化率") {
				seen["unsupported_metric"] = struct{}{}
			}
			if strings.Contains(warning, "折线图") {
				seen["unsupported_chart_type"] = struct{}{}
			}
		}
	}
	for _, expected := range item.ExpectedErrors {
		if _, ok := seen[expected]; !ok {
			return failed(fmt.Sprintf("expected error %q was not observed", expected))
		}
	}
	return passed()
}

func isModelContractErrorCode(code aitrend.SafeErrorCode) bool {
	switch code {
	case aitrend.ErrorInvalidJSON, aitrend.ErrorMissingField, aitrend.ErrorInvalidEnum, aitrend.ErrorInvalidCombination, aitrend.ErrorUnavailableValue:
		return true
	default:
		return false
	}
}

func scoreDraft(item Case, result Result, runErr error) SubScore {
	if item.ExpectedDraft != nil && !*item.ExpectedDraft {
		if result.Draft != nil {
			return failed("Draft was generated for a guidance-only case")
		}
		return passed()
	}
	if item.ExpectedStatus == "failure" {
		return passed()
	}
	if runErr != nil || result.Draft == nil {
		return failed("draft is unavailable")
	}
	if len(result.Draft.Curves) > MaxBenchmarkCurves {
		return failed("draft exceeds eight curves")
	}
	if item.ExpectedTotalAtLeast > 0 && len(result.Draft.Curves) < item.ExpectedTotalAtLeast {
		return failed("draft contains fewer candidates than the expected capacity sample")
	}
	if item.ExpectedStatus == "complete" && result.Draft.Status != aitrend.DraftComplete {
		return failed(fmt.Sprintf("draft status %q is not complete", result.Draft.Status))
	}
	if item.ExpectedStatus == "partial" && result.Draft.Status != aitrend.DraftPartial {
		return failed(fmt.Sprintf("draft status %q is not partial", result.Draft.Status))
	}
	if item.DuplicatesMustMerge && hasDuplicateCurveValues(result.Draft) {
		return failed("duplicate curve identities were not merged")
	}
	return passed()
}

func scoreExplanation(item Case, result Result) SubScore {
	if !item.ExplanationRequired {
		return passed()
	}
	if result.Draft == nil || strings.TrimSpace(result.Draft.Explanation) == "" {
		return failed("explanation is empty")
	}
	maxRunes := item.ExplanationMaxRunes
	if maxRunes <= 0 {
		maxRunes = aitrend.MaxExplanationRunes
	}
	runes := []rune(result.Draft.Explanation)
	if len(runes) > maxRunes {
		return failed("explanation exceeds the product length limit")
	}
	if item.ExplanationLanguage == "zh-CN" && !containsChinese(result.Draft.Explanation) {
		return failed("explanation is not Chinese")
	}
	return passed()
}

// scoreReplyQuality is advisory: Score.Pass deliberately excludes it so a
// quality warning cannot turn a valid Draft or guidance result into failure.
func scoreReplyQuality(item Case, result Result) SubScore {
	if !item.ReplyRequired {
		return passed()
	}
	reply := strings.TrimSpace(assistantMessage(result))
	if reply == "" {
		return failed("assistant reply is empty")
	}
	if item.ReplyMaxRunes > 0 && len([]rune(reply)) > item.ReplyMaxRunes {
		return failed("assistant reply exceeds the advisory rune limit")
	}
	if item.ReplyLanguage == "zh-CN" && !containsChinese(reply) {
		return failed("assistant reply is not Chinese")
	}
	return passed()
}

type operationCall struct {
	Tool   string
	Params json.RawMessage
}

func operationCalls(result Result) []operationCall {
	calls := make([]operationCall, 0)
	for _, event := range result.Events {
		if event.Type != aitrend.StreamEventOperation || event.Status != aitrend.StreamRunning || event.Tool == string(aitrend.ToolAnalyzeIntent) || event.Tool == string(aitrend.ToolExplainDraft) {
			continue
		}
		calls = append(calls, operationCall{Tool: event.Tool, Params: event.Params})
	}
	return calls
}

func containsWildcard(calls []operationCall) bool {
	for _, call := range calls {
		if strings.Contains(string(call.Params), `"*"`) {
			return true
		}
	}
	return false
}

func intentContainsWildcard(intent *aitrend.Intent) bool {
	for _, query := range intent.SubQueries {
		for _, expression := range query.Expressions {
			if expression == "*" {
				return true
			}
		}
	}
	return false
}

func intentContainsExpression(intent *aitrend.Intent, expressions []string) bool {
	wanted := normalizedSet(expressions)
	for _, query := range intent.SubQueries {
		for _, expression := range query.Expressions {
			if _, ok := wanted[expression]; ok {
				return true
			}
		}
	}
	return false
}

func draftValues(draft *aitrend.Draft) map[string]struct{} {
	values := make(map[string]struct{})
	if draft == nil {
		return values
	}
	for _, curve := range draft.Curves {
		values[curve.Condition.Value] = struct{}{}
	}
	return values
}

func draftHasValue(draft *aitrend.Draft, dimension aitrend.Dimension, value string) bool {
	if draft == nil {
		return false
	}
	for _, curve := range draft.Curves {
		if curve.Condition.Dimension == dimension && curve.Condition.Value == value {
			return true
		}
	}
	return false
}

func normalizedSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func sameSet(left, right map[string]struct{}) bool {
	if len(left) != len(right) {
		return false
	}
	for value := range left {
		if _, ok := right[value]; !ok {
			return false
		}
	}
	return true
}

func containsAny(actual, expected map[string]struct{}) bool {
	for value := range actual {
		if _, ok := expected[value]; ok {
			return true
		}
	}
	return false
}

func hasDuplicateCurveValues(draft *aitrend.Draft) bool {
	seen := make(map[string]struct{}, len(draft.Curves))
	for _, curve := range draft.Curves {
		key := string(curve.Condition.Dimension) + "\x00" + curve.Condition.Value
		if _, ok := seen[key]; ok {
			return true
		}
		seen[key] = struct{}{}
	}
	return false
}

func containsChinese(value string) bool {
	for _, character := range value {
		if unicode.Is(unicode.Han, character) {
			return true
		}
	}
	return false
}

func passed() SubScore { return SubScore{Pass: true} }

func failed(reason string) SubScore { return SubScore{Reasons: []string{reason}} }
