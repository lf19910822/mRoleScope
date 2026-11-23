package evaluation

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"mrolescope/internal/aitrend"
)

func completeResult() Result {
	return Result{Draft: &aitrend.Draft{ContractVersion: aitrend.ContractVersionV1, Status: aitrend.DraftComplete}}
}

func TestExecutorRunsThreeRepetitionsSeriallyAndRetriesInfrastructure(t *testing.T) {
	var calls, active, maxActive int32
	executor, err := NewExecutor(func(context.Context, Case) (Result, error) {
		atomic.AddInt32(&calls, 1)
		current := atomic.AddInt32(&active, 1)
		for {
			old := atomic.LoadInt32(&maxActive)
			if current <= old || atomic.CompareAndSwapInt32(&maxActive, old, current) {
				break
			}
		}
		atomic.AddInt32(&active, -1)
		if calls%2 == 1 {
			return Result{}, &aitrend.ContractError{Code: aitrend.ErrorBackendFailure, Message: "fixture backend failure"}
		}
		return completeResult(), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := executor.RunCase(context.Background(), Case{ID: "E01"})
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.ValidRuns != 3 || evaluation.Incomplete || calls != 6 || maxActive != 1 {
		t.Fatalf("evaluation=%#v calls=%d maxActive=%d", evaluation, calls, maxActive)
	}
}

func TestExecutorDoesNotRetrySemanticFailure(t *testing.T) {
	calls := 0
	executor, err := NewExecutor(func(context.Context, Case) (Result, error) {
		calls++
		return Result{}, &aitrend.ContractError{Code: aitrend.ErrorInvalidEnum, Message: "invalid fixture"}
	})
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := executor.RunCase(context.Background(), Case{ID: "U05"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 || evaluation.ValidRuns != 3 || evaluation.Attempts[0].Status != AttemptSemanticFailure {
		t.Fatalf("evaluation=%#v calls=%d", evaluation, calls)
	}
}

func TestExecutorMarksPersistentInfrastructureFailureIncomplete(t *testing.T) {
	executor, err := NewExecutor(func(context.Context, Case) (Result, error) {
		return Result{}, errors.New("network unavailable")
	})
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := executor.RunCase(context.Background(), Case{ID: "U01"})
	if err != nil {
		t.Fatal(err)
	}
	if !evaluation.Incomplete || evaluation.ValidRuns != 0 || len(evaluation.Attempts) != 9 {
		t.Fatalf("evaluation=%#v", evaluation)
	}
}
