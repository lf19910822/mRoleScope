package evaluation

import (
	"context"
	"fmt"
	"time"

	"mrolescope/internal/aitrend"
)

const (
	DefaultRepetitions       = 3
	MaxInfrastructureRetries = 2
	MaxEvaluationToolCalls   = 20
)

type AttemptStatus string

const (
	AttemptComplete        AttemptStatus = "complete"
	AttemptSemanticFailure AttemptStatus = "semantic_failure"
	AttemptInfrastructure  AttemptStatus = "infrastructure_failure"
	AttemptLimitExceeded   AttemptStatus = "limit_exceeded"
	AttemptIncomplete      AttemptStatus = "incomplete"
)

type Attempt struct {
	CaseID     string        `json:"case_id"`
	Repetition int           `json:"repetition"`
	Retry      int           `json:"retry"`
	Status     AttemptStatus `json:"status"`
	ErrorCode  string        `json:"error_code,omitempty"`
	Duration   time.Duration `json:"duration_ns"`
	Result     Result        `json:"-"`
}

type CaseEvaluation struct {
	CaseID     string    `json:"case_id"`
	Attempts   []Attempt `json:"attempts"`
	ValidRuns  int       `json:"valid_runs"`
	Incomplete bool      `json:"incomplete"`
}

// ExecuteFunc is deliberately injectable so the retry policy can be tested
// without network access while production calls still use Runner.Run.
type ExecuteFunc func(context.Context, Case) (Result, error)
type ExecuteAttemptFunc func(context.Context, Case, int, int) (Result, error)

type Executor struct {
	Execute                  ExecuteFunc
	ExecuteAttempt           ExecuteAttemptFunc
	Repetitions              int
	MaxInfrastructureRetries int
}

func NewExecutor(execute ExecuteFunc) (*Executor, error) {
	if execute == nil {
		return nil, fmt.Errorf("evaluation executor requires an execute function")
	}
	return &Executor{Execute: execute, Repetitions: DefaultRepetitions, MaxInfrastructureRetries: MaxInfrastructureRetries}, nil
}

func (executor *Executor) RunCase(ctx context.Context, item Case) (CaseEvaluation, error) {
	if executor == nil || executor.Execute == nil {
		return CaseEvaluation{}, fmt.Errorf("evaluation executor is not configured")
	}
	repetitions := executor.Repetitions
	if repetitions <= 0 {
		repetitions = DefaultRepetitions
	}
	if repetitions > DefaultRepetitions {
		return CaseEvaluation{}, fmt.Errorf("repetitions %d exceeds benchmark limit %d", repetitions, DefaultRepetitions)
	}
	retries := executor.MaxInfrastructureRetries
	if retries < 0 || retries > MaxInfrastructureRetries {
		return CaseEvaluation{}, fmt.Errorf("infrastructure retry limit %d is outside [0,%d]", retries, MaxInfrastructureRetries)
	}
	evaluation := CaseEvaluation{CaseID: item.ID, Attempts: make([]Attempt, 0, repetitions)}
	for repetition := 1; repetition <= repetitions; repetition++ {
		completed := false
		for retry := 0; retry <= retries; retry++ {
			started := time.Now()
			var result Result
			var err error
			if executor.ExecuteAttempt != nil {
				result, err = executor.ExecuteAttempt(ctx, item, repetition, retry)
			} else {
				result, err = executor.Execute(ctx, item)
			}
			attempt := Attempt{CaseID: item.ID, Repetition: repetition, Retry: retry, Result: result, Duration: time.Since(started)}
			attempt.Status, attempt.ErrorCode = classifyAttempt(result, err)
			evaluation.Attempts = append(evaluation.Attempts, attempt)
			if attempt.Status == AttemptInfrastructure {
				if retry < retries {
					continue
				}
				evaluation.Incomplete = true
				completed = true
				break
			}
			if attempt.Status == AttemptComplete || attempt.Status == AttemptSemanticFailure || attempt.Status == AttemptLimitExceeded {
				evaluation.ValidRuns++
			}
			completed = true
			break
		}
		if !completed {
			return CaseEvaluation{}, fmt.Errorf("case %s repetition %d did not produce a terminal attempt", item.ID, repetition)
		}
	}
	return evaluation, nil
}

func classifyAttempt(result Result, err error) (AttemptStatus, string) {
	if result.Stats.ToolCalls > MaxEvaluationToolCalls {
		return AttemptLimitExceeded, string(aitrend.ErrorLimitExceeded)
	}
	if err == nil {
		if result.Draft == nil && isGuidanceResult(result) {
			return AttemptComplete, ""
		}
		if result.Draft == nil {
			return AttemptSemanticFailure, string(aitrend.ErrorMissingField)
		}
		return AttemptComplete, ""
	}
	if contractErr, ok := err.(*aitrend.ContractError); ok {
		switch contractErr.Code {
		case aitrend.ErrorBackendFailure, aitrend.ErrorProviderFailure, aitrend.ErrorConfigurationFailure:
			return AttemptInfrastructure, string(contractErr.Code)
		case aitrend.ErrorLimitExceeded:
			return AttemptLimitExceeded, string(contractErr.Code)
		default:
			return AttemptSemanticFailure, string(contractErr.Code)
		}
	}
	return AttemptInfrastructure, string(aitrend.ErrorBackendFailure)
}

func isGuidanceResult(result Result) bool {
	for _, mode := range result.Stats.ResponseModes {
		if mode == aitrend.ResponseModeGuidance {
			return true
		}
	}
	return false
}
