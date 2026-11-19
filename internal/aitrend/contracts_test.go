package aitrend

import (
	"encoding/json"
	"errors"
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
