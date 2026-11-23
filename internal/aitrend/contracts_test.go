package aitrend

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func validIntent() Intent {
	return Intent{
		ContractVersion: ContractVersionV1,
		SubQueries: []SubQuery{{
			ID:          "databases",
			Dimension:   DimensionTechnology,
			Expressions: []string{"后台数据库"},
			Ranking: RankingIntent{
				Metric: MetricAverageDailyActiveJobs,
				Order:  OrderDescending,
				Limit:  8,
			},
		}},
		TimeRange: TimeRange{Kind: RangeMonth},
		Output:    OutputIntent{SeriesMetric: SeriesMetricDailyActiveJobs, ChartType: ChartTypeLine},
	}
}

func TestDecodeIntentRejectsUnknownFields(t *testing.T) {
	data, err := json.Marshal(struct {
		Intent
		Unexpected string `json:"unexpected"`
	}{Intent: validIntent(), Unexpected: "no"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeIntent(data); err == nil {
		t.Fatal("DecodeIntent accepted an unknown field")
	}
}

func TestValidateIntentRejectsInvalidCombination(t *testing.T) {
	intent := validIntent()
	operator := ComparisonEqual
	intent.SubQueries[0].Operator = &operator
	if err := ValidateIntent(intent); err == nil {
		t.Fatal("ValidateIntent accepted an operator for technology")
	} else {
		var contractErr *ContractError
		if !errors.As(err, &contractErr) || contractErr.Code != ErrorInvalidCombination {
			t.Fatalf("error = %v, want invalid combination", err)
		}
	}
}

func TestValidateIntentRejectsCurveAndAttemptLimits(t *testing.T) {
	intent := validIntent()
	intent.SubQueries[0].Ranking.Limit = MaxCurves + 1
	if err := ValidateIntent(intent); err == nil {
		t.Fatal("ValidateIntent accepted a ninth curve limit")
	}
}

func TestDecodeAgentResponseSupportsGuidanceTrendAndMixedModes(t *testing.T) {
	guidance, err := DecodeAgentResponse([]byte(`{"contract_version":"ai-trend-agent-v1","response_modes":["guidance"]}`))
	if err != nil || len(guidance.ResponseModes) != 1 || guidance.TrendIntent != nil {
		t.Fatalf("guidance response = %#v, err=%v", guidance, err)
	}
	trendJSON := []byte(`{"contract_version":"ai-trend-draft-v1","sub_queries":[{"id":"q1","dimension":"project","expressions":["*"],"ranking":{"metric":"avg_daily_active_jobs","order":"desc","limit":1}}],"time_range":{"kind":"month"},"output":{"series_metric":"daily_active_jobs","chart_type":"line"}}`)
	trend, err := DecodeAgentResponse(trendJSON)
	if err != nil || len(trend.ResponseModes) != 1 || trend.ResponseModes[0] != ResponseModeTrend || trend.TrendIntent == nil {
		t.Fatalf("legacy trend response = %#v, err=%v", trend, err)
	}
	wrapped := []byte(`{"contract_version":"ai-trend-agent-v1","response_modes":["trend","guidance"],"trend_intent":` + string(trendJSON) + `}`)
	mixed, err := DecodeAgentResponse(wrapped)
	if err != nil || len(mixed.ResponseModes) != 2 || mixed.TrendIntent == nil {
		t.Fatalf("mixed response = %#v, err=%v", mixed, err)
	}
}

func TestDecodeAgentResponseRejectsInvalidModeCombinations(t *testing.T) {
	tests := []string{
		`{"contract_version":"ai-trend-agent-v1","response_modes":[]}`,
		`{"contract_version":"ai-trend-agent-v1","response_modes":["guidance","guidance"]}`,
		`{"contract_version":"ai-trend-agent-v1","response_modes":["trend"]}`,
		`{"contract_version":"ai-trend-agent-v1","response_modes":["guidance"],"trend_intent":{"contract_version":"ai-trend-draft-v1"}}`,
	}
	for _, raw := range tests {
		if _, err := DecodeAgentResponse([]byte(raw)); err == nil {
			t.Fatalf("invalid Agent response accepted: %s", raw)
		}
	}
}

func TestValidateStreamEventAssistantMessage(t *testing.T) {
	if err := ValidateStreamEvent(StreamEvent{Type: StreamEventAssistant, Status: StreamRunning, Delta: "你好"}); err != nil {
		t.Fatalf("assistant delta rejected: %v", err)
	}
	if err := ValidateStreamEvent(StreamEvent{Type: StreamEventAssistant, Status: StreamCompleted}); err != nil {
		t.Fatalf("assistant completion rejected: %v", err)
	}
	if err := ValidateStreamEvent(StreamEvent{Type: StreamEventAssistant, Status: StreamRunning}); err == nil {
		t.Fatal("empty assistant delta accepted")
	}
	longDelta := strings.Repeat("中", MaxReplyRunes+1)
	if err := ValidateStreamEvent(StreamEvent{Type: StreamEventAssistant, Status: StreamRunning, Delta: longDelta}); err == nil {
		t.Fatal("long assistant delta accepted")
	}
}

func TestDecodeStreamEventRejectsUnknownFieldsAndValidatesDraft(t *testing.T) {
	data := []byte(`{"type":"operation","status":"running","tool":"rank_values","unexpected":true}`)
	if _, err := DecodeStreamEvent(data); err == nil {
		t.Fatal("DecodeStreamEvent accepted an unknown field")
	}

	event := StreamEvent{Type: StreamEventOperation, Status: StreamRunning, Tool: "rank_values"}
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeStreamEvent(encoded); err != nil {
		t.Fatalf("DecodeStreamEvent(valid) error = %v", err)
	}
}

func TestValidateDraftRejectsMoreThanEightCurves(t *testing.T) {
	draft := Draft{ContractVersion: ContractVersionV1, Status: DraftComplete}
	for index := 0; index < MaxCurves+1; index++ {
		draft.Curves = append(draft.Curves, DraftCurve{
			Condition:   CurveCondition{Dimension: DimensionTechnology, Value: "go"},
			Label:       "Go",
			Selected:    true,
			MatchStatus: MatchExact,
			Reason:      "ranked",
			SubQueryIDs: []string{"q"},
		})
	}
	if err := ValidateDraft(draft); err == nil {
		t.Fatal("ValidateDraft accepted more than eight curves")
	}
}
