package aitrend

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestApplyTrendRankResultsCannotAddOutOfScopeValues(t *testing.T) {
	groups := []CandidateGroup{{SubQueryID: "engines", Candidates: []Candidate{
		{Dimension: DimensionTechnology, Value: "unity", Label: "Unity", SubQueryID: "engines"},
		{Dimension: DimensionTechnology, Value: "unreal", Label: "Unreal", SubQueryID: "engines"},
	}}}
	response := rankedTrendValuesResponse{Values: []rankedTrendValue{
		{SubQueryID: "engines", Dimension: DimensionJobFamily, Value: "backend"},
		{SubQueryID: "engines", Dimension: DimensionTechnology, Value: "java"},
		{SubQueryID: "engines", Dimension: DimensionJobFamily, Value: "unity"},
		{SubQueryID: "engines", Dimension: DimensionTechnology, Value: "unreal"},
		{SubQueryID: "engines", Dimension: DimensionTechnology, Value: "unreal"},
	}}
	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if applyTrendRankResults(groups, []ToolCallResult{{Name: ToolRankTrendValues, Available: true, Payload: payload}}) {
		t.Fatal("global ranking was applied to a scoped category")
	}
	if len(groups[0].Candidates) != 2 || groups[0].Candidates[0].Value != "unity" || groups[0].Candidates[1].Value != "unreal" {
		t.Fatalf("global ranking changed the scoped category: %#v", groups[0].Candidates)
	}
}

func TestApplyTrendRankResultsForWholeDimensionStillRequiresFormalCandidates(t *testing.T) {
	groups := []CandidateGroup{{SubQueryID: "all", AllowGlobalRanking: true, Candidates: []Candidate{
		{Dimension: DimensionTechnology, Value: "java", Label: "Java", SubQueryID: "all"},
	}}}
	response := rankedTrendValuesResponse{Values: []rankedTrendValue{
		{SubQueryID: "all", Dimension: DimensionJobFamily, Value: "java"},
		{SubQueryID: "all", Dimension: DimensionTechnology, Value: "unknown"},
		{SubQueryID: "all", Dimension: DimensionTechnology, Value: "java"},
	}}
	payload, _ := json.Marshal(response)
	if !applyTrendRankResults(groups, []ToolCallResult{{Name: ToolRankTrendValues, Available: true, Payload: payload}}) {
		t.Fatal("whole-dimension ranking was not applied")
	}
	if len(groups[0].Candidates) != 1 || groups[0].Candidates[0].Value != "java" || groups[0].Candidates[0].Dimension != DimensionTechnology {
		t.Fatalf("whole-dimension ranking accepted an unknown or wrong-dimension value: %#v", groups[0].Candidates)
	}
}

func TestEmitValidatedDraftReturnsValidationFailureAfterSendingEvent(t *testing.T) {
	validationErr := errors.New("draft did not pass validation")
	registry, err := NewToolRegistry([]ToolDefinition{{Name: ToolBatchValidateDraft, Handler: func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, validationErr
	}}})
	if err != nil {
		t.Fatal(err)
	}
	var events []StreamEvent
	err = emitValidatedDraft(context.Background(), registry, Draft{}, func(event StreamEvent) error {
		events = append(events, event)
		return nil
	})
	if !errors.Is(err, validationErr) {
		t.Fatalf("got error %v, want the validation failure", err)
	}
	if len(events) != 2 || events[0].Status != StreamRunning || events[1].Status != StreamFailed {
		t.Fatalf("unexpected validation events: %#v", events)
	}
}

func TestEmitValidatedDraftPreservesValidationAndEmissionFailures(t *testing.T) {
	validationErr := errors.New("invalid draft")
	emissionErr := errors.New("stream closed")
	registry, err := NewToolRegistry([]ToolDefinition{{Name: ToolBatchValidateDraft, Handler: func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, validationErr
	}}})
	if err != nil {
		t.Fatal(err)
	}
	err = emitValidatedDraft(context.Background(), registry, Draft{}, func(event StreamEvent) error {
		if event.Status == StreamFailed {
			return emissionErr
		}
		return nil
	})
	if !errors.Is(err, validationErr) || !errors.Is(err, emissionErr) {
		t.Fatalf("validation and stream errors must both be retained: %v", err)
	}
}
