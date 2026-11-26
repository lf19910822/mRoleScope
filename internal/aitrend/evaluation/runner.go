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
	Langfuse       aitrend.LangfuseDebugConfig
}

type discardDebugSink struct{}

func (discardDebugSink) Record(aitrend.DebugTraceEvent) error { return nil }

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
		faultProvider := &faultInjectingProvider{base: provider, tools: toolProvider}
		if _, classified := provider.(aitrend.IntentServiceModeClassifier); classified {
			provider = &faultInjectingClassifiedProvider{faultInjectingProvider: faultProvider}
		} else {
			provider = faultProvider
		}
	}
	productionRunner, err := aitrend.NewTrendRunner(provider, runner.Snapshot, runner.Snapshot, nil)
	if err != nil {
		return result, err
	}
	productionRunner.MaxDuration = runner.MaxDuration
	productionRunner.StatsSink = func(stats aitrend.TrendRunStats) { result.Stats = stats }
	emit := func(event aitrend.StreamEvent) error {
		if result.FirstEventLatency == 0 && isFirstModelEvent(event) {
			result.FirstEventLatency = time.Since(started)
		}
		result.Events = append(result.Events, event)
		if event.Draft != nil {
			copy := *event.Draft
			result.Draft = &copy
		}
		return nil
	}
	if runner.Langfuse.Enabled() {
		ctx = aitrend.WithLangfuseTraceContext(ctx, runner.Langfuse.TraceContext)
		productionRunner.Langfuse = runner.Langfuse
		err = productionRunner.RunDebug(ctx, query, emit, discardDebugSink{})
	} else {
		err = productionRunner.Run(ctx, query, emit)
	}
	result.TotalDuration = time.Since(started)
	return result, err
}

func isFirstModelEvent(event aitrend.StreamEvent) bool {
	return !(event.Type == aitrend.StreamEventOperation && event.Tool == string(aitrend.ToolAnalyzeIntent) && event.Status == aitrend.StreamRunning)
}

type faultInjectingProvider struct {
	base     aitrend.IntentProvider
	tools    aitrend.IntentToolProvider
	injected bool
}

func (provider *faultInjectingProvider) AnalyzeIntent(ctx context.Context, query string, correction *aitrend.ContractError) ([]byte, error) {
	return provider.base.AnalyzeIntent(ctx, query, correction)
}

type faultInjectingClassifiedProvider struct {
	*faultInjectingProvider
}

func (provider *faultInjectingClassifiedProvider) ClassifyServiceModes(ctx context.Context, query string) (aitrend.ServiceModeClassification, int, int, error) {
	classifier, ok := provider.base.(aitrend.IntentServiceModeClassifier)
	if !ok {
		return aitrend.ServiceModeClassification{}, 0, 0, fmt.Errorf("classification provider is unavailable")
	}
	return classifier.ClassifyServiceModes(ctx, query)
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
