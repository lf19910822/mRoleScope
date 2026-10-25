package jdanalysis

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const aiModelMaximumAttempts = 3

// AIModelAttempt is the durable-safe shape of one physical model call. Its
// association ID is deterministic per logical task and attempt number so a
// later ledger can remain idempotent without retaining a request body.
type AIModelAttempt struct {
	AssociationID string
	Number        int
	SafeErrorCode AISafeErrorCode
}

// AIModelAttemptRunner applies the fixed v0.1 retry policy around the
// replaceable client seam. It is deliberately time-injectable for deterministic
// tests and does not make retry policy a provider concern.
type AIModelAttemptRunner struct {
	Client                  ControlledAIModelClient
	Sleep                   func(context.Context, time.Duration) error
	BaseDelay               time.Duration
	Budget                  *AITrialBudgetGate
	Estimator               AIInputTokenEstimator
	InitialAttemptCount     int
	PersistentTrialBudget   *PersistentAITrialBudget
	PersistentTrialWorkItem string
}

// Execute performs at most three physical requests for one logical task.
// Authentication, schema, evidence, policy, and configuration failures are
// terminal. Every completed physical call appears in attempts.
func (runner AIModelAttemptRunner) Execute(ctx context.Context, logicalTaskID string, input AIModelInput) (AIModelResponse, []AIModelAttempt, error) {
	if runner.Client == nil || logicalTaskID == "" || runner.InitialAttemptCount < 0 || runner.InitialAttemptCount >= aiModelMaximumAttempts || (runner.Budget != nil && runner.PersistentTrialBudget != nil) || (runner.PersistentTrialBudget != nil && runner.PersistentTrialWorkItem != logicalTaskID) {
		return AIModelResponse{}, nil, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	estimatedInputTokens := 0
	if runner.Budget != nil || runner.PersistentTrialBudget != nil {
		if runner.Estimator == nil {
			return AIModelResponse{}, nil, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
		}
		var err error
		estimatedInputTokens, err = runner.Estimator.Estimate(input)
		if err != nil {
			return AIModelResponse{}, nil, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
		}
	}
	sleep := runner.Sleep
	if sleep == nil {
		sleep = sleepWithContext
	}
	baseDelay := runner.BaseDelay
	if baseDelay <= 0 {
		baseDelay = time.Second
	}
	attempts := make([]AIModelAttempt, 0, aiModelMaximumAttempts)
	for number := runner.InitialAttemptCount + 1; number <= aiModelMaximumAttempts; number++ {
		attempt := AIModelAttempt{AssociationID: fmt.Sprintf("%s:attempt-%02d", logicalTaskID, number), Number: number}
		var reservation AITrialBudgetReservation
		var persistentReservation PersistentAITrialBudgetReservation
		if runner.Budget != nil {
			var err error
			reservation, err = runner.Budget.Reserve(logicalTaskID, attempt.AssociationID, estimatedInputTokens)
			if err != nil {
				return AIModelResponse{}, attempts, err
			}
		}
		if runner.PersistentTrialBudget != nil {
			var err error
			persistentReservation, err = runner.PersistentTrialBudget.Reserve(ctx, logicalTaskID, number, estimatedInputTokens)
			if err != nil {
				return AIModelResponse{}, attempts, err
			}
		}
		response, err := runner.Client.Analyze(ctx, input)
		if err == nil {
			if runner.Budget != nil {
				if err := runner.Budget.RecordUsage(reservation, response.Usage); err != nil {
					attempt.SafeErrorCode = AISafeErrorBudgetExhausted
					attempts = append(attempts, attempt)
					return AIModelResponse{}, attempts, err
				}
			}
			if runner.PersistentTrialBudget != nil {
				if err := runner.PersistentTrialBudget.RecordUsage(ctx, persistentReservation, response.Usage); err != nil {
					attempt.SafeErrorCode = AISafeErrorBudgetExhausted
					attempts = append(attempts, attempt)
					return AIModelResponse{}, attempts, err
				}
			}
			attempts = append(attempts, attempt)
			return response, attempts, nil
		}
		code, retryAfter, retryable := retryableAIProviderFailure(err)
		attempt.SafeErrorCode = code
		attempts = append(attempts, attempt)
		if !retryable || number == aiModelMaximumAttempts {
			return AIModelResponse{}, attempts, err
		}
		delay := retryAfter
		if delay <= 0 {
			delay = baseDelay << (number - 1)
		}
		if err := sleep(ctx, delay); err != nil {
			return AIModelResponse{}, attempts, NewSafeAIError(AISafeErrorProviderTimeout, err)
		}
	}
	return AIModelResponse{}, attempts, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
}

func retryableAIProviderFailure(err error) (AISafeErrorCode, time.Duration, bool) {
	var providerFailure AIProviderFailure
	if errors.As(err, &providerFailure) {
		switch providerFailure.Code {
		case AISafeErrorProviderTimeout, AISafeErrorProviderRateLimited, AISafeErrorProviderServer:
			return providerFailure.Code, providerFailure.RetryAfter, true
		default:
			return providerFailure.Code, 0, false
		}
	}
	var safeError safeAIError
	if errors.As(err, &safeError) {
		return safeError.code, 0, false
	}
	return AISafeErrorConfigurationInvalid, 0, false
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
