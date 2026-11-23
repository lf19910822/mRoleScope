package evaluation

import (
	"os"
	"path/filepath"
	"testing"
)

func benchmarkSourcePath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "Docs", "开发指导", "AI评测", "AI趋势智能体Benchmark-v1.md")
}

func agentBenchmarkSourcePath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "Docs", "开发指导", "AI评测", "AI趋势智能体AgentBenchmark-v1.md")
}

func TestReviewableBenchmarkHasThirtyCasesAcrossSixCategories(t *testing.T) {
	cases, err := LoadFile(benchmarkSourcePath(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != ExpectedCaseCount {
		t.Fatalf("case count = %d, want %d", len(cases), ExpectedCaseCount)
	}
	counts := map[string]int{}
	for _, item := range cases {
		counts[item.Category]++
	}
	for _, category := range Categories() {
		if counts[category] != ExpectedCasesPerCategory {
			t.Fatalf("category %s count = %d, want %d", category, counts[category], ExpectedCasesPerCategory)
		}
	}
}

func TestBenchmarkGeneratorPreservesCanonicalConstraints(t *testing.T) {
	cases, err := LoadFile(benchmarkSourcePath(t))
	if err != nil {
		t.Fatal(err)
	}
	dataset, err := BuildDataset(cases, "AI趋势智能体Benchmark-v1.md")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := MarshalDataset(dataset)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) == 0 || string(encoded[:1]) != "{" {
		t.Fatalf("generated dataset is not JSON: %q", encoded[:min(len(encoded), 20)])
	}
	if dataset.Version != BenchmarkVersion || dataset.CaseCount != ExpectedCaseCount || dataset.Categories != ExpectedCategoryCount {
		t.Fatalf("dataset metadata = %#v", dataset)
	}
	var foundWildcard, foundRankingMetric, foundCapacity bool
	for _, item := range dataset.Cases {
		for _, expression := range item.Expressions {
			if expression == "*" {
				foundWildcard = true
			}
		}
		if item.RankingMetric != "" {
			foundRankingMetric = true
		}
		if item.MaxCurves == MaxBenchmarkCurves {
			foundCapacity = true
		}
	}
	if !foundWildcard || !foundRankingMetric || !foundCapacity {
		t.Fatalf("generated cases lost required constraints: wildcard=%v metric=%v capacity=%v", foundWildcard, foundRankingMetric, foundCapacity)
	}
}

func TestTrendUnsupportedCasesDeclareAgentBoundaries(t *testing.T) {
	cases, err := LoadFile(benchmarkSourcePath(t))
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]Case, len(cases))
	for _, item := range cases {
		byID[item.ID] = item
	}
	for _, id := range []string{"U01", "U02", "U03", "U05"} {
		item := byID[id]
		if len(item.ExpectedResponseModes) != 2 || item.ExpectedDraft == nil || !*item.ExpectedDraft || !item.ReplyRequired {
			t.Fatalf("case %s is missing trend+guidance constraints: %#v", id, item)
		}
	}
	u04 := byID["U04"]
	if len(u04.ExpectedResponseModes) != 1 || u04.ExpectedResponseModes[0] != "guidance" || u04.ExpectedDraft == nil || *u04.ExpectedDraft || !u04.ExpectedNoTrendTools || u04.ExpectedTerminalStatus != "success" {
		t.Fatalf("U04 Agent boundary = %#v", u04)
	}
}

func TestBenchmarkSourceDoesNotContainSensitivePayloads(t *testing.T) {
	data, err := os.ReadFile(benchmarkSourcePath(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"password=", "token=", "authorization", "<p>", "JD正文"} {
		if containsFold(string(data), forbidden) {
			t.Fatalf("benchmark source contains forbidden payload marker %q", forbidden)
		}
	}
}

func TestAgentBenchmarkIsIndependentAndGroupedFourThreeThree(t *testing.T) {
	cases, err := LoadAgentFile(agentBenchmarkSourcePath(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != ExpectedAgentCaseCount {
		t.Fatalf("Agent case count = %d, want %d", len(cases), ExpectedAgentCaseCount)
	}
	counts := map[string]int{}
	for _, item := range cases {
		counts[item.Category]++
		if item.ReplyLanguage != "zh-CN" || item.ReplyMaxRunes != MaxAgentReplyRunes || !item.ReplyRequired {
			t.Fatalf("Agent reply contract missing for %s: %#v", item.ID, item)
		}
	}
	if counts["pure-guidance"] != 4 || counts["unclear-or-out-of-scope"] != 3 || counts["mixed-trend-guidance"] != 3 {
		t.Fatalf("Agent category counts = %#v", counts)
	}
	dataset, err := BuildAgentDataset(cases, "AI趋势智能体AgentBenchmark-v1.md")
	if err != nil {
		t.Fatal(err)
	}
	if dataset.Version != AgentBenchmarkVersion || dataset.Categories != ExpectedAgentCategoryCount {
		t.Fatalf("Agent dataset metadata = %#v", dataset)
	}
	if _, err := MarshalAgentDataset(dataset); err != nil {
		t.Fatal(err)
	}
}

func TestAgentBenchmarkSourceDoesNotContainSensitivePayloads(t *testing.T) {
	data, err := os.ReadFile(agentBenchmarkSourcePath(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"password=", "token=", "authorization", "<p>", "JD正文"} {
		if containsFold(string(data), forbidden) {
			t.Fatalf("Agent benchmark source contains forbidden payload marker %q", forbidden)
		}
	}
}

func containsFold(value, needle string) bool {
	return len(needle) > 0 && len(value) >= len(needle) &&
		stringContainsFold(value, needle)
}

func stringContainsFold(value, needle string) bool {
	// Avoid importing a second parser just for the source safety assertion.
	for index := 0; index+len(needle) <= len(value); index++ {
		match := true
		for offset := 0; offset < len(needle); offset++ {
			left, right := value[index+offset], needle[offset]
			if left >= 'A' && left <= 'Z' {
				left += 'a' - 'A'
			}
			if right >= 'A' && right <= 'Z' {
				right += 'a' - 'A'
			}
			if left != right {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
