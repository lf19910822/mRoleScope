package evaluation

import (
	"context"
	"fmt"
	"time"

	"mrolescope/internal/aitrend"
)

// Runner executes the production trend AI path against one evaluation
// snapshot. It intentionally returns stream events rather than exposing
// provider internals or user-facing transport types.
type Runner struct {
	Provider       aitrend.IntentProvider
	Snapshot       *Snapshot
	MaxDuration    time.Duration
	FaultInjection string
}

type Result struct {
	Events            []aitrend.StreamEvent
	Draft             *aitrend.Draft
	Stats             aitrend.TrendRunStats
	FirstEventLatency time.Duration
	TotalDuration     time.Duration
}

func NewRunner(provider aitrend.IntentProvider, snapshot *Snapshot) (*Runner, error) {
	if provider == nil || snapshot == nil {
		return nil, fmt.Errorf("evaluation runner requires provider and snapshot")
	}
	return &Runner{Provider: provider, Snapshot: snapshot}, nil
}

func (runner *Runner) Run(ctx context.Context, query string) (Result, error) {
	if runner == nil || runner.Provider == nil || runner.Snapshot == nil {
		return Result{}, fmt.Errorf("evaluation runner is not configured")
	}
	result := Result{Events: make([]aitrend.StreamEvent, 0, aitrend.MaxToolCalls+4)}
	started := time.Now()
	provider := runner.Provider
	if runner.FaultInjection == "invalid_tool_params_once" {
		toolProvider, ok := provider.(aitrend.IntentToolProvider)
		if !ok {
			return result, fmt.Errorf("fault injection requires a tool-capable provider")
		}
		provider = &faultInjectingProvider{base: provider, tools: toolProvider}
	}
	productionRunner, err := aitrend.NewTrendRunner(provider, runner.Snapshot, runner.Snapshot, nil)
	if err != nil {
		return result, err
	}
	productionRunner.MaxDuration = runner.MaxDuration
	productionRunner.StatsSink = func(stats aitrend.TrendRunStats) { result.Stats = stats }
	err = productionRunner.Run(ctx, query, func(event aitrend.StreamEvent) error {
		if len(result.Events) == 0 {
			result.FirstEventLatency = time.Since(started)
		}
		result.Events = append(result.Events, event)
		if event.Draft != nil {
			copy := *event.Draft
			result.Draft = &copy
		}
		return nil
	})
	result.TotalDuration = time.Since(started)
	return result, err
}

type faultInjectingProvider struct {
	base     aitrend.IntentProvider
	tools    aitrend.IntentToolProvider
	injected bool
}

func (provider *faultInjectingProvider) AnalyzeIntent(ctx context.Context, query string, correction *aitrend.ContractError) ([]byte, error) {
	return provider.base.AnalyzeIntent(ctx, query, correction)
}

func (provider *faultInjectingProvider) AnalyzeIntentTurn(ctx context.Context, query string, correction *aitrend.ContractError, history []aitrend.ToolConversation, allowTools bool) (aitrend.IntentToolTurn, error) {
	turn, err := provider.tools.AnalyzeIntentTurn(ctx, query, correction, history, allowTools)
	if err != nil || provider.injected || len(turn.ToolCalls) == 0 {
		return turn, err
	}
	turn.ToolCalls[0].Params = []byte(`{}`)
	provider.injected = true
	return turn, nil
}
