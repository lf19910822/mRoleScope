package jdanalysis

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAIModelAttemptRunnerRetriesOnlyTransientFailuresAndCountsEveryAttempt(t *testing.T) {
	client := &sequenceAIModelClient{outcomes: []aiModelOutcome{
		{err: AIProviderFailure{Code: AISafeErrorProviderTimeout}},
		{err: AIProviderFailure{Code: AISafeErrorProviderRateLimited, RetryAfter: 7 * time.Second}},
		{response: AIModelResponse{Content: []byte(`{}`), Usage: AIModelUsage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}}},
	}}
	sleeper := &recordingAISleeper{}
	runner := AIModelAttemptRunner{Client: client, Sleep: sleeper.Sleep}

	response, attempts, err := runner.Execute(context.Background(), "logical-task-01", syntheticSemanticModelInput(t))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if response.Usage.TotalTokens != 2 || client.calls != 3 {
		t.Fatalf("response/calls = %#v/%d, want success on third call", response.Usage, client.calls)
	}
	if got, want := len(attempts), 3; got != want {
		t.Fatalf("attempt count = %d, want %d", got, want)
	}
	for index, attempt := range attempts {
		wantID := []string{"logical-task-01:attempt-01", "logical-task-01:attempt-02", "logical-task-01:attempt-03"}[index]
		if attempt.AssociationID != wantID || attempt.Number != index+1 {
			t.Fatalf("attempt %d = %#v, want association %q", index, attempt, wantID)
		}
	}
	if attempts[0].SafeErrorCode != AISafeErrorProviderTimeout || attempts[1].SafeErrorCode != AISafeErrorProviderRateLimited || attempts[2].SafeErrorCode != "" {
		t.Fatalf("attempt safety codes = %#v", attempts)
	}
	if got, want := sleeper.Delays, []time.Duration{time.Second, 7 * time.Second}; !equalDurations(got, want) {
		t.Fatalf("retry delays = %#v, want %#v", got, want)
	}
}

func TestAIModelAttemptRunnerDoesNotRetryTerminalOrFourthAttempt(t *testing.T) {
	for _, test := range []struct {
		name      string
		outcomes  []aiModelOutcome
		wantCalls int
		wantCode  AISafeErrorCode
	}{
		{
			name:      "authentication",
			outcomes:  []aiModelOutcome{{err: AIProviderFailure{Code: AISafeErrorProviderAuthentication}}},
			wantCalls: 1,
			wantCode:  AISafeErrorProviderAuthentication,
		},
		{
			name:      "schema",
			outcomes:  []aiModelOutcome{{err: NewSafeAIError(AISafeErrorInvalidResponseSchema, errors.New("synthetic"))}},
			wantCalls: 1,
			wantCode:  AISafeErrorInvalidResponseSchema,
		},
		{
			name:      "evidence",
			outcomes:  []aiModelOutcome{{err: NewSafeAIError(AISafeErrorInvalidEvidence, errors.New("synthetic"))}},
			wantCalls: 1,
			wantCode:  AISafeErrorInvalidEvidence,
		},
		{
			name:      "policy",
			outcomes:  []aiModelOutcome{{err: NewSafeAIError(AISafeErrorPolicyRejected, errors.New("synthetic"))}},
			wantCalls: 1,
			wantCode:  AISafeErrorPolicyRejected,
		},
		{
			name: "three server failures",
			outcomes: []aiModelOutcome{
				{err: AIProviderFailure{Code: AISafeErrorProviderServer}},
				{err: AIProviderFailure{Code: AISafeErrorProviderServer}},
				{err: AIProviderFailure{Code: AISafeErrorProviderServer}},
			},
			wantCalls: 3,
			wantCode:  AISafeErrorProviderServer,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &sequenceAIModelClient{outcomes: test.outcomes}
			runner := AIModelAttemptRunner{Client: client, Sleep: (&recordingAISleeper{}).Sleep}

			_, attempts, err := runner.Execute(context.Background(), "logical-task-01", syntheticSemanticModelInput(t))
			if err == nil || client.calls != test.wantCalls || len(attempts) != test.wantCalls {
				t.Fatalf("error/calls/attempts = %v/%d/%d, want terminal after %d", err, client.calls, len(attempts), test.wantCalls)
			}
			if attempts[len(attempts)-1].SafeErrorCode != test.wantCode {
				t.Fatalf("final attempt = %#v, want safe code %q", attempts[len(attempts)-1], test.wantCode)
			}
		})
	}
}

type aiModelOutcome struct {
	err      error
	response AIModelResponse
}

type recordingAISleeper struct {
	Delays []time.Duration
}

func (sleeper *recordingAISleeper) Sleep(_ context.Context, delay time.Duration) error {
	sleeper.Delays = append(sleeper.Delays, delay)
	return nil
}

type sequenceAIModelClient struct {
	calls    int
	outcomes []aiModelOutcome
}

func (client *sequenceAIModelClient) Analyze(_ context.Context, _ AIModelInput) (AIModelResponse, error) {
	outcome := client.outcomes[client.calls]
	client.calls++
	return outcome.response, outcome.err
}

func equalDurations(got, want []time.Duration) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range want {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
