package evaluation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"mrolescope/internal/aitrend"
)

func TestTraceRedactsSensitiveToolParametersAndKeepsBoundedEvidence(t *testing.T) {
	version, err := NewEvaluationVersion(completeVersionInput(), time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	intent := aitrend.Intent{ContractVersion: aitrend.ContractVersionV1, SubQueries: []aitrend.SubQuery{{ID: "go", Dimension: aitrend.DimensionTechnology, Expressions: []string{"go"}, Ranking: aitrend.RankingIntent{Metric: aitrend.MetricAverageDailyActiveJobs, Order: aitrend.OrderDescending, Limit: 1}}}}
	draft := &aitrend.Draft{ContractVersion: aitrend.ContractVersionV1, Status: aitrend.DraftComplete, Explanation: "Go 的岗位平均数量最高。"}
	result := Result{
		Draft:             draft,
		Stats:             aitrend.TrendRunStats{InputTokens: 12, OutputTokens: 8, ToolCalls: 1, Intent: &intent, StartedAt: time.Now(), CompletedAt: time.Now()},
		FirstEventLatency: 120 * time.Millisecond,
		TotalDuration:     2 * time.Second,
		Events:            []aitrend.StreamEvent{{Type: aitrend.StreamEventOperation, Status: aitrend.StreamCompleted, Tool: string(aitrend.ToolBatchTrendSummary), Params: json.RawMessage(`{"value":"go","authorization":"Bearer secret","nested":{"password":"hidden"}}`)}},
	}
	attempt := Attempt{CaseID: "E01", Repetition: 1, Status: AttemptComplete, Result: result}
	trace, err := BuildTrace(Case{ID: "E01"}, version, attempt, Score{Pass: true, Draft: passed()}, DefaultUsageRates())
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(trace)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{"Bearer secret", "hidden", "password=", "chain-of-thought"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("trace contains forbidden content %q: %s", forbidden, text)
		}
	}
	if trace.EstimatedCostNanoRMB != 25_600 {
		t.Fatalf("estimated cost = %d, want 25600", trace.EstimatedCostNanoRMB)
	}
	if trace.FirstEventLatencyMS != 120 || trace.TotalDurationMS != 2000 {
		t.Fatalf("timings = %d/%d", trace.FirstEventLatencyMS, trace.TotalDurationMS)
	}
	if _, err := TraceDigest(trace); err != nil {
		t.Fatal(err)
	}
}

func TestTraceClassifiesIncompleteAttempt(t *testing.T) {
	version, err := NewEvaluationVersion(completeVersionInput(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	trace, err := BuildTrace(Case{ID: "U01"}, version, Attempt{CaseID: "U01", Repetition: 1, Status: AttemptInfrastructure, ErrorCode: string(aitrend.ErrorBackendFailure)}, Score{}, DefaultUsageRates())
	if err != nil {
		t.Fatal(err)
	}
	if trace.Status != TraceInfrastructure || trace.ErrorCode != string(aitrend.ErrorBackendFailure) {
		t.Fatalf("trace = %#v", trace)
	}
}

func TestTraceStoresAgentReplySummaryWithoutReplyText(t *testing.T) {
	version, err := NewEvaluationVersion(completeVersionInput(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	result := Result{
		Stats: aitrend.TrendRunStats{
			ResponseModes:        []aitrend.ResponseMode{aitrend.ResponseModeGuidance},
			ReplyQualityWarnings: []string{"too_long"},
		},
		Events: []aitrend.StreamEvent{
			{Type: aitrend.StreamEventAssistant, Status: aitrend.StreamRunning, Delta: "不要把这段回复写入审计"},
			{Type: aitrend.StreamEventAssistant, Status: aitrend.StreamCompleted},
		},
	}
	trace, err := BuildTrace(Case{ID: "G01"}, version, Attempt{CaseID: "G01", Repetition: 1, Status: AttemptComplete, Result: result}, Score{Pass: true}, DefaultUsageRates())
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(trace)
	if err != nil {
		t.Fatal(err)
	}
	if trace.AssistantReplyStatus != "completed" || len(trace.ResponseModes) != 1 || len(trace.ReplyQualityWarnings) != 1 || strings.Contains(string(data), "不要把这段回复写入审计") {
		t.Fatalf("agent trace = %#v, json=%s", trace, data)
	}
}

func TestTraceStoresServiceClassificationSummarySeparately(t *testing.T) {
	version, err := NewEvaluationVersion(completeVersionInput(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	result := Result{
		Stats: aitrend.TrendRunStats{
			ServiceTags: []aitrend.ServiceTag{aitrend.ServiceTagTrend, aitrend.ServiceTagGuidance}, ClassificationStatus: "completed",
			ClassificationInput: 5, ClassificationOutput: 2, ClassificationTime: 150 * time.Millisecond,
			InputTokens: 15, OutputTokens: 10, ToolCalls: 1,
		},
		Events: []aitrend.StreamEvent{
			{Type: aitrend.StreamEventOperation, Status: aitrend.StreamRunning, Tool: string(aitrend.ToolClassifyServiceModes)},
			{Type: aitrend.StreamEventOperation, Status: aitrend.StreamCompleted, Tool: string(aitrend.ToolClassifyServiceModes)},
		},
	}
	trace, err := BuildTrace(Case{ID: "M01"}, version, Attempt{CaseID: "M01", Repetition: 1, Status: AttemptComplete, Result: result}, Score{Pass: true}, DefaultUsageRates())
	if err != nil {
		t.Fatal(err)
	}
	if trace.ClassificationStatus != "completed" || trace.ClassificationInputTokens != 5 || trace.ClassificationOutputTokens != 2 || trace.ClassificationDurationMS != 150 || trace.EstimatedClassificationCostNanoRMB != 8000 {
		t.Fatalf("classification trace = %#v", trace)
	}
	if len(trace.Operations) != 2 || trace.Operations[0].Tool != string(aitrend.ToolClassifyServiceModes) || len(trace.Operations[0].Params) != 0 {
		t.Fatalf("classification operations = %#v", trace.Operations)
	}
}

func TestTraceTypesDoNotRequireProductContext(t *testing.T) {
	_ = context.Background()
}
