package jdanalysis

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestAITrialBudgetGateBlocksUnverifiableOversizedAndOverBudgetAttemptsBeforeSending(t *testing.T) {
	input := syntheticSemanticModelInput(t)
	client := &sequenceAIModelClient{outcomes: []aiModelOutcome{{response: AIModelResponse{Content: []byte(`{}`), Usage: AIModelUsage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}}}}}

	unverified := QwenPlusTrialPricingProfile()
	unverified.verified = false
	if _, err := NewAITrialBudgetGate(unverified); err == nil || !strings.Contains(err.Error(), string(AISafeErrorConfigurationInvalid)) {
		t.Fatalf("unverified pricing error = %v, want safe configuration failure", err)
	}

	gate, err := NewAITrialBudgetGate(QwenPlusTrialPricingProfile())
	if err != nil {
		t.Fatal(err)
	}
	runner := AIModelAttemptRunner{
		Budget:    gate,
		Client:    client,
		Estimator: fixedAIInputTokenEstimator{tokens: 10001},
	}
	_, attempts, err := runner.Execute(context.Background(), "logical-task-01", input)
	if err == nil || !strings.Contains(err.Error(), string(AISafeErrorInputTooLarge)) || client.calls != 0 || len(attempts) != 0 {
		t.Fatalf("oversized preflight = error:%v calls:%d attempts:%d; want no send", err, client.calls, len(attempts))
	}

	gate, err = NewAITrialBudgetGate(QwenPlusTrialPricingProfile())
	if err != nil {
		t.Fatal(err)
	}
	gate.reservedNanoRMB = gate.maxNanoRMB - 1
	runner.Budget = gate
	runner.Estimator = fixedAIInputTokenEstimator{tokens: 1}
	_, attempts, err = runner.Execute(context.Background(), "logical-task-02", input)
	if err == nil || !strings.Contains(err.Error(), string(AISafeErrorBudgetExhausted)) || client.calls != 0 || len(attempts) != 0 {
		t.Fatalf("budget preflight = error:%v calls:%d attempts:%d; want no send", err, client.calls, len(attempts))
	}
}

func TestAITrialBudgetGateEnforcesFixedTrialCapsAndRecordsUsage(t *testing.T) {
	profile := QwenPlusTrialPricingProfile()
	profile.inputNanoRMBPerToken = 1
	profile.outputNanoRMBPerToken = 1
	gate, err := NewAITrialBudgetGate(profile)
	if err != nil {
		t.Fatal(err)
	}
	for logicalTask := 1; logicalTask <= 60; logicalTask++ {
		for attempt := 1; attempt <= 3; attempt++ {
			reservation, err := gate.Reserve("task-"+twoDigit(logicalTask), "task-"+twoDigit(logicalTask)+":attempt-"+twoDigit(attempt), 100)
			if err != nil {
				t.Fatalf("Reserve(task=%d attempt=%d) error = %v", logicalTask, attempt, err)
			}
			if err := gate.RecordUsage(reservation, AIModelUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15}); err != nil {
				t.Fatalf("RecordUsage(task=%d attempt=%d) error = %v", logicalTask, attempt, err)
			}
		}
	}
	snapshot := gate.Snapshot()
	if snapshot.LogicalTaskCount != 60 || snapshot.AttemptCount != 180 || snapshot.ChargedNanoRMB != 60*3*15 {
		t.Fatalf("budget snapshot = %#v", snapshot)
	}
	if _, err := gate.Reserve("task-61", "task-61:attempt-01", 100); err == nil || !strings.Contains(err.Error(), string(AISafeErrorBudgetExhausted)) {
		t.Fatalf("61st logical task error = %v, want safe budget failure", err)
	}
	if profile := QwenPlusTrialPricingProfile(); profile.MaxInputTokens != 10000 || profile.MaxOutputTokens != 2500 || profile.Model != "qwen-plus" {
		t.Fatalf("Plus pricing profile is not fixed to the trial request profile: %#v", profile)
	}
}

func TestAITrialBudgetGateDoesNotReuseAnAttemptAssociationID(t *testing.T) {
	gate, err := NewAITrialBudgetGate(QwenPlusTrialPricingProfile())
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := gate.Reserve("task-01", "task-01:attempt-01", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := gate.RecordUsage(reservation, AIModelUsage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := gate.Reserve("task-01", "task-01:attempt-01", 1); err == nil || !strings.Contains(err.Error(), string(AISafeErrorConfigurationInvalid)) {
		t.Fatalf("reused association error = %v, want safe configuration failure", err)
	}
}

type fixedAIInputTokenEstimator struct {
	tokens int
}

func (estimator fixedAIInputTokenEstimator) Estimate(AIModelInput) (int, error) {
	return estimator.tokens, nil
}

func twoDigit(value int) string {
	return fmt.Sprintf("%02d", value)
}
