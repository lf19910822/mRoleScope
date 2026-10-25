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

func TestAIModelAttemptRunnerSendsNormalChineseJDWithinFixedInputBudget(t *testing.T) {
	gate, err := NewAITrialBudgetGate(QwenPlusTrialPricingProfile())
	if err != nil {
		t.Fatal(err)
	}
	client := &sequenceAIModelClient{outcomes: []aiModelOutcome{{
		response: AIModelResponse{Usage: AIModelUsage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}},
	}}}

	input := normalChineseAIModelInput()
	estimator := ConservativeAIInputTokenEstimator{}
	estimatedTokens, err := estimator.Estimate(input)
	if err != nil {
		t.Fatalf("normal Chinese JD estimate error = %v", err)
	}
	if estimatedTokens >= QwenPlusTrialPricingProfile().MaxInputTokens {
		t.Fatalf("normal Chinese JD estimated input tokens = %d, want less than fixed limit", estimatedTokens)
	}
	_, attempts, err := (AIModelAttemptRunner{
		Budget:    gate,
		Client:    client,
		Estimator: estimator,
	}).Execute(context.Background(), "normal-chinese-jd", input)
	if err != nil {
		t.Fatalf("normal Chinese JD preflight error = %v, want provider call", err)
	}
	if client.calls != 1 || len(attempts) != 1 {
		t.Fatalf("normal Chinese JD calls/attempts = %d/%d, want 1/1", client.calls, len(attempts))
	}
}

func TestAIModelAttemptRunnerBlocksActuallyOversizedChineseInputBeforeProviderCall(t *testing.T) {
	gate, err := NewAITrialBudgetGate(QwenPlusTrialPricingProfile())
	if err != nil {
		t.Fatal(err)
	}
	client := &sequenceAIModelClient{outcomes: []aiModelOutcome{{
		response: AIModelResponse{Usage: AIModelUsage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}},
	}}}
	input := normalChineseAIModelInput()
	for index := range input.Fields[0].Spans {
		input.Fields[0].Spans[index].Text = strings.Repeat("超限中文输入。", 1_000)
	}

	_, attempts, err := (AIModelAttemptRunner{
		Budget:    gate,
		Client:    client,
		Estimator: ConservativeAIInputTokenEstimator{},
	}).Execute(context.Background(), "oversized-chinese-jd", input)
	if err == nil || !strings.Contains(err.Error(), string(AISafeErrorInputTooLarge)) {
		t.Fatalf("oversized Chinese JD error = %v, want safe input-too-large failure", err)
	}
	if client.calls != 0 || len(attempts) != 0 {
		t.Fatalf("oversized Chinese JD calls/attempts = %d/%d, want 0/0", client.calls, len(attempts))
	}
}

func TestAIInputTokenEstimatorContractIncludesFixedChatEnvelope(t *testing.T) {
	want := sha256Hex(strings.Join([]string{
		AIInputTokenEstimatorVersionV1,
		"vocabulary_sha256=" + qwenBPEVocabularySHA256,
		"segmentation=ascii_letter_runs_and_single_non_ascii_or_non_letter_units-v1",
		"bpe=qwen_tiktoken_lowest_rank_merge-v1",
		"chat_envelope_tokens=512",
	}, "\n"))
	if got := AIInputTokenEstimatorSHA256(); got != want {
		t.Fatalf("input token estimator contract = %q, want fixed 512-token envelope contract", got)
	}
}

func TestConservativeQwenTokenCountCountsEveryUTF8ByteBeforeMerging(t *testing.T) {
	count, err := conservativeQwenTokenCount("\ue000")
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("private-use Unicode code point token count = %d, want 3", count)
	}
}

func normalChineseAIModelInput() AIModelInput {
	spans := make([]AIModelInputSpan, 0, 32)
	mentions := make([]VerifiedTechnologyMention, 0, 32)
	for index := 1; index <= 32; index++ {
		spanID := fmt.Sprintf("jobRequire:s%02d", index)
		spans = append(spans, AIModelInputSpan{
			SpanID: spanID,
			Text:   "负责分布式服务设计、开发和稳定性保障，具备良好的协作与问题定位能力，并持续优化系统质量和工程效能。",
		})
		mentions = append(mentions, VerifiedTechnologyMention{
			CanonicalName:     fmt.Sprintf("技术%d", index),
			MentionEvidenceID: fmt.Sprintf("evidence-%02d", index),
			SourceField:       SourceFieldJobRequire,
			SpanID:            spanID,
			TechnologyKey:     fmt.Sprintf("technology-%02d", index),
		})
	}
	return AIModelInput{
		BodyContentVersionID:       "synthetic-normal-chinese-jd",
		Fields:                     []AIModelInputField{{SourceField: SourceFieldJobRequire, Spans: spans}},
		InputContractVersion:       AIInputContractVersionV1,
		RequestID:                  "synthetic-normal-chinese-request",
		VerifiedTechnologyMentions: mentions,
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
