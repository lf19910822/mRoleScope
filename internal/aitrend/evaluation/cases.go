// Package evaluation contains the offline benchmark contract for the trend
// assistant. It deliberately stays separate from the production request path.
package evaluation

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"mrolescope/internal/aitrend"
)

const (
	BenchmarkVersion           = "ai-trend-benchmark-v1"
	AgentBenchmarkVersion      = "ai-trend-agent-benchmark-v1"
	ExpectedCaseCount          = 30
	ExpectedAgentCaseCount     = 10
	ExpectedCategoryCount      = 6
	ExpectedCasesPerCategory   = 5
	ExpectedAgentCategoryCount = 3
	MaxBenchmarkCurves         = 8
)

var benchmarkCategories = []string{
	"explicit-values",
	"category-scope",
	"global-ranking",
	"compound-conditions",
	"unsupported-recovery",
	"draft-validation",
}

var agentBenchmarkCategories = []string{
	"pure-guidance",
	"unclear-or-out-of-scope",
	"mixed-trend-guidance",
}

// Case is the reviewable and executable representation of one benchmark
// question. Formal values are stable IDs; display labels never participate in
// equality checks.
type Case struct {
	ID                     string            `json:"id"`
	Category               string            `json:"category"`
	Query                  string            `json:"query"`
	Dimension              string            `json:"dimension"`
	ExpectedCategory       string            `json:"expected_category,omitempty"`
	Expressions            []string          `json:"expressions"`
	Exclusions             []string          `json:"exclusions,omitempty"`
	TimeRange              string            `json:"time_range,omitempty"`
	ExpectedTimeRange      string            `json:"expected_time_range,omitempty"`
	IncludeSecondary       *bool             `json:"include_secondary,omitempty"`
	RankingMetric          string            `json:"ranking_metric"`
	RequestedMetric        string            `json:"requested_metric,omitempty"`
	RankingOrder           string            `json:"ranking_order"`
	Limit                  int               `json:"limit"`
	RequestedLimit         int               `json:"requested_limit,omitempty"`
	ExpectedValues         []string          `json:"expected_values"`
	RequiredValues         []string          `json:"required_values,omitempty"`
	ExpectedDimensions     []string          `json:"expected_dimensions,omitempty"`
	ExpectedCategories     map[string]string `json:"expected_categories,omitempty"`
	ExpectedSubQueryCount  int               `json:"expected_subquery_count,omitempty"`
	ExpectedScope          string            `json:"expected_scope"`
	MaxCurves              int               `json:"max_curves"`
	ExpectedStatus         string            `json:"expected_status"`
	ExpectedResponseModes  []string          `json:"expected_response_modes,omitempty"`
	ExpectedServiceTags    []string          `json:"expected_service_tags,omitempty"`
	ExpectedTerminalStatus string            `json:"expected_terminal_status,omitempty"`
	ExpectedNoTrendTools   bool              `json:"expected_no_trend_tools,omitempty"`
	ExpectedDraft          *bool             `json:"expected_draft,omitempty"`
	ReplyRequired          bool              `json:"reply_required,omitempty"`
	ReplyLanguage          string            `json:"reply_language,omitempty"`
	ReplyMaxRunes          int               `json:"reply_max_runes,omitempty"`
	ExpectedErrors         []string          `json:"expected_errors,omitempty"`
	FaultInjection         string            `json:"fault_injection,omitempty"`
	ExpectedTotalAtLeast   int               `json:"expected_total_at_least,omitempty"`
	FixedOrderRequired     bool              `json:"fixed_order_required,omitempty"`
	DuplicatesMustMerge    bool              `json:"duplicates_must_merge,omitempty"`
	ExplanationRequired    bool              `json:"explanation_required"`
	ExplanationLanguage    string            `json:"explanation_language,omitempty"`
	ExplanationMaxRunes    int               `json:"explanation_max_runes,omitempty"`
}

// Dataset is the deterministic machine-readable output generated from the
// Markdown review source.
type Dataset struct {
	Version    string `json:"version"`
	Source     string `json:"source"`
	CaseCount  int    `json:"case_count"`
	Categories int    `json:"category_count"`
	Cases      []Case `json:"cases"`
}

// LoadMarkdown extracts JSON blocks fenced with ```json from the reviewable
// benchmark document. Keeping the source in Markdown allows people to review
// the wording and the executable constraints together.
func LoadMarkdown(r io.Reader) ([]Case, error) {
	cases, err := parseMarkdown(r)
	if err != nil {
		return nil, err
	}
	if err := Validate(cases); err != nil {
		return nil, err
	}
	return cases, nil
}

// LoadAgentMarkdown loads the independent Agent interaction benchmark. It is
// intentionally separate from LoadMarkdown so this evaluation source cannot
// be consumed by ordinary trend evaluation or Serving jobs.
func LoadAgentMarkdown(r io.Reader) ([]Case, error) {
	cases, err := parseMarkdown(r)
	if err != nil {
		return nil, err
	}
	if err := ValidateAgent(cases); err != nil {
		return nil, err
	}
	return cases, nil
}

func parseMarkdown(r io.Reader) ([]Case, error) {
	if r == nil {
		return nil, fmt.Errorf("benchmark source is nil")
	}
	scanner := bufio.NewScanner(r)
	lineNumber := 0
	inJSON := false
	var block bytes.Buffer
	cases := make([]Case, 0, ExpectedCaseCount)
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if !inJSON {
			if line == "```json" {
				inJSON = true
				block.Reset()
			}
			continue
		}
		if line == "```" {
			var item Case
			if err := json.Unmarshal(block.Bytes(), &item); err != nil {
				return nil, fmt.Errorf("benchmark JSON near line %d: %w", lineNumber, err)
			}
			cases = append(cases, item)
			inJSON = false
			continue
		}
		block.WriteString(scanner.Text())
		block.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read benchmark source: %w", err)
	}
	if inJSON {
		return nil, fmt.Errorf("benchmark source has an unclosed JSON block")
	}
	return cases, nil
}

// LoadFile loads and validates a reviewable benchmark document.
func LoadFile(path string) ([]Case, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open benchmark source: %w", err)
	}
	defer file.Close()
	return LoadMarkdown(file)
}

func LoadAgentFile(path string) ([]Case, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open Agent benchmark source: %w", err)
	}
	defer file.Close()
	return LoadAgentMarkdown(file)
}

func ValidateAgent(cases []Case) error {
	if len(cases) != ExpectedAgentCaseCount {
		return fmt.Errorf("Agent benchmark has %d cases, want %d", len(cases), ExpectedAgentCaseCount)
	}
	counts := make(map[string]int, ExpectedAgentCategoryCount)
	known := make(map[string]struct{}, len(agentBenchmarkCategories))
	for _, category := range agentBenchmarkCategories {
		known[category] = struct{}{}
	}
	seen := make(map[string]struct{}, len(cases))
	for index, item := range cases {
		if !isAgentCase(item) {
			return fmt.Errorf("Agent case %d does not declare Agent constraints", index+1)
		}
		if item.ID == "" || item.Query == "" || item.Category == "" {
			return fmt.Errorf("Agent case %d has missing identity or query fields", index+1)
		}
		if _, ok := known[item.Category]; !ok {
			return fmt.Errorf("Agent case %s has unknown category %q", item.ID, item.Category)
		}
		if _, ok := seen[item.ID]; ok {
			return fmt.Errorf("duplicate Agent benchmark case ID %q", item.ID)
		}
		seen[item.ID] = struct{}{}
		counts[item.Category]++
		if len(item.ExpectedResponseModes) == 0 || len(item.ExpectedResponseModes) > 2 || !uniqueStrings(item.ExpectedResponseModes) {
			return fmt.Errorf("Agent case %s has invalid response modes", item.ID)
		}
		if err := validateExpectedServiceTags(item); err != nil {
			return err
		}
		for _, mode := range item.ExpectedResponseModes {
			if mode != "trend" && mode != "guidance" {
				return fmt.Errorf("Agent case %s has invalid response mode %q", item.ID, mode)
			}
		}
		if item.ExpectedTerminalStatus == "" || item.ExpectedDraft == nil || !item.ReplyRequired {
			return fmt.Errorf("Agent case %s is missing terminal, Draft, or reply constraints", item.ID)
		}
		if item.ExpectedTerminalStatus != "success" && item.ExpectedTerminalStatus != "partial" && item.ExpectedTerminalStatus != "failure" && item.ExpectedTerminalStatus != "user_stopped" {
			return fmt.Errorf("Agent case %s has invalid terminal status %q", item.ID, item.ExpectedTerminalStatus)
		}
		if containsString(item.ExpectedResponseModes, "trend") && (item.Dimension == "" || item.RankingMetric == "" || item.RankingOrder == "" || item.MaxCurves != MaxBenchmarkCurves) {
			return fmt.Errorf("Agent trend case %s is missing trend constraints", item.ID)
		}
		if item.ReplyLanguage != "zh-CN" || item.ReplyMaxRunes != MaxAgentReplyRunes {
			return fmt.Errorf("Agent case %s must require zh-CN and %d reply runes", item.ID, MaxAgentReplyRunes)
		}
	}
	wants := map[string]int{"pure-guidance": 4, "unclear-or-out-of-scope": 3, "mixed-trend-guidance": 3}
	for _, category := range agentBenchmarkCategories {
		if counts[category] != wants[category] {
			return fmt.Errorf("Agent category %s has %d cases, want %d", category, counts[category], wants[category])
		}
	}
	return nil
}

func validateExpectedServiceTags(item Case) error {
	if len(item.ExpectedServiceTags) == 0 || len(item.ExpectedServiceTags) > 2 || !uniqueStrings(item.ExpectedServiceTags) {
		return fmt.Errorf("case %s has invalid expected service tags", item.ID)
	}
	for _, tag := range item.ExpectedServiceTags {
		if tag != string(aitrend.ServiceTagTrend) && tag != string(aitrend.ServiceTagGuidance) {
			return fmt.Errorf("case %s has invalid expected service tag %q", item.ID, tag)
		}
	}
	return nil
}

const MaxAgentReplyRunes = 1000

func uniqueStrings(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

// Validate enforces the first benchmark's review contract before execution.
func Validate(cases []Case) error {
	if len(cases) != ExpectedCaseCount {
		return fmt.Errorf("benchmark has %d cases, want %d", len(cases), ExpectedCaseCount)
	}
	counts := make(map[string]int, ExpectedCategoryCount)
	knownCategories := make(map[string]struct{}, len(benchmarkCategories))
	for _, category := range benchmarkCategories {
		knownCategories[category] = struct{}{}
	}
	seenIDs := make(map[string]struct{}, len(cases))
	for index, item := range cases {
		agentCase := isAgentCase(item)
		if item.ID == "" || item.Query == "" || item.Category == "" || (!agentCase && item.Dimension == "" && len(item.ExpectedDimensions) == 0) {
			return fmt.Errorf("case %d has missing identity or query fields", index+1)
		}
		if _, exists := seenIDs[item.ID]; exists {
			return fmt.Errorf("duplicate benchmark case ID %q", item.ID)
		}
		seenIDs[item.ID] = struct{}{}
		if _, ok := knownCategories[item.Category]; !ok {
			return fmt.Errorf("case %s has unknown category %q", item.ID, item.Category)
		}
		counts[item.Category]++
		if item.ExpectedSubQueryCount > 1 && len(item.ExpectedDimensions) != item.ExpectedSubQueryCount {
			return fmt.Errorf("case %s expected sub-query count and dimensions disagree", item.ID)
		}
		if !agentCase && (item.RankingMetric == "" || item.RankingOrder == "" || item.ExpectedScope == "" || item.ExpectedStatus == "") {
			return fmt.Errorf("case %s has incomplete ranking or expected-result fields", item.ID)
		}
		if item.ExpectedTerminalStatus != "" {
			switch item.ExpectedTerminalStatus {
			case "success", "partial", "failure", "user_stopped":
			default:
				return fmt.Errorf("case %s has invalid expected terminal status %q", item.ID, item.ExpectedTerminalStatus)
			}
		}
		for _, mode := range item.ExpectedResponseModes {
			if mode != "trend" && mode != "guidance" {
				return fmt.Errorf("case %s has invalid expected response mode %q", item.ID, mode)
			}
		}
		if err := validateExpectedServiceTags(item); err != nil {
			return err
		}
		if item.ReplyMaxRunes < 0 {
			return fmt.Errorf("case %s has invalid reply rune limit", item.ID)
		}
		if item.RankingOrder != "" && item.RankingOrder != "none" && item.RankingOrder != "asc" && item.RankingOrder != "desc" {
			return fmt.Errorf("case %s has invalid ranking order %q", item.ID, item.RankingOrder)
		}
		if !agentCase && (item.Limit < 1 || item.Limit > 20) {
			return fmt.Errorf("case %s has invalid limit %d", item.ID, item.Limit)
		}
		if !agentCase && item.RequestedLimit > 0 && item.RequestedLimit < item.Limit {
			return fmt.Errorf("case %s requested_limit %d is below contract limit %d", item.ID, item.RequestedLimit, item.Limit)
		}
		if !agentCase && item.MaxCurves != MaxBenchmarkCurves {
			return fmt.Errorf("case %s has max_curves=%d, want %d", item.ID, item.MaxCurves, MaxBenchmarkCurves)
		}
		if item.ExpectedStatus != "" {
			switch item.ExpectedStatus {
			case "complete", "partial", "failure":
			default:
				return fmt.Errorf("case %s has invalid expected status %q", item.ID, item.ExpectedStatus)
			}
		}
		if item.ExplanationRequired && item.ExplanationMaxRunes < 0 {
			return fmt.Errorf("case %s has invalid explanation rune limit", item.ID)
		}
		if item.FaultInjection != "" && item.FaultInjection != "invalid_tool_params_once" {
			return fmt.Errorf("case %s has unknown fault injection %q", item.ID, item.FaultInjection)
		}
	}
	for _, category := range benchmarkCategories {
		if counts[category] != ExpectedCasesPerCategory {
			return fmt.Errorf("category %s has %d cases, want %d", category, counts[category], ExpectedCasesPerCategory)
		}
	}
	return nil
}

func isAgentCase(item Case) bool {
	return len(item.ExpectedResponseModes) > 0 || item.ExpectedDraft != nil || item.ReplyRequired
}

// BuildDataset produces stable JSON for the execution runner. Struct field
// order and case order are intentionally preserved from the reviewed source.
func BuildDataset(cases []Case, source string) (Dataset, error) {
	if err := Validate(cases); err != nil {
		return Dataset{}, err
	}
	if strings.TrimSpace(source) == "" {
		return Dataset{}, fmt.Errorf("benchmark source identifier is empty")
	}
	return Dataset{Version: BenchmarkVersion, Source: source, CaseCount: len(cases), Categories: ExpectedCategoryCount, Cases: append([]Case(nil), cases...)}, nil
}

func BuildAgentDataset(cases []Case, source string) (Dataset, error) {
	if err := ValidateAgent(cases); err != nil {
		return Dataset{}, err
	}
	if strings.TrimSpace(source) == "" {
		return Dataset{}, fmt.Errorf("Agent benchmark source identifier is empty")
	}
	return Dataset{Version: AgentBenchmarkVersion, Source: source, CaseCount: len(cases), Categories: len(agentBenchmarkCategories), Cases: append([]Case(nil), cases...)}, nil
}

// MarshalDataset returns indented, deterministic machine-readable benchmark
// data suitable for a versioned evaluation artifact.
func MarshalDataset(dataset Dataset) ([]byte, error) {
	if dataset.Version == "" || dataset.Source == "" || dataset.CaseCount != len(dataset.Cases) {
		return nil, fmt.Errorf("invalid benchmark dataset metadata")
	}
	if err := Validate(dataset.Cases); err != nil {
		return nil, err
	}
	return json.MarshalIndent(dataset, "", "  ")
}

func MarshalAgentDataset(dataset Dataset) ([]byte, error) {
	if dataset.Version != AgentBenchmarkVersion || dataset.Source == "" || dataset.CaseCount != len(dataset.Cases) {
		return nil, fmt.Errorf("invalid Agent benchmark dataset metadata")
	}
	if err := ValidateAgent(dataset.Cases); err != nil {
		return nil, err
	}
	return json.MarshalIndent(dataset, "", "  ")
}

// Categories returns the stable category order used by the first benchmark.
func Categories() []string { return append([]string(nil), benchmarkCategories...) }

func AgentCategories() []string { return append([]string(nil), agentBenchmarkCategories...) }

// SortedCaseIDs is useful for reports without changing the reviewed case
// order used by the runner.
func SortedCaseIDs(cases []Case) []string {
	ids := make([]string, 0, len(cases))
	for _, item := range cases {
		ids = append(ids, item.ID)
	}
	sort.Strings(ids)
	return ids
}
