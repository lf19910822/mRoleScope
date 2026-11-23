package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"mrolescope/internal/aitrend"
	"mrolescope/internal/aitrend/evaluation"
)

const (
	evaluationProviderFixture = "fixture"
	evaluationProviderQwen    = "qwen"
)

type environmentLookup func(string) (string, bool)

func validateEvaluationProviderConfiguration(mode string, httpClient *http.Client, lookup environmentLookup) error {
	_, err := newEvaluationProvider(mode, evaluation.Case{}, httpClient, lookup)
	return err
}

func newEvaluationProvider(mode string, item evaluation.Case, httpClient *http.Client, lookup environmentLookup) (aitrend.IntentProvider, error) {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		mode = evaluationProviderFixture
	}
	switch mode {
	case evaluationProviderFixture:
		return &fixtureProvider{item: item}, nil
	case evaluationProviderQwen:
		if lookup == nil {
			lookup = os.LookupEnv
		}
		apiKey, keyOK := lookup("MROLESCOPE_QWEN_API_KEY")
		workspaceID, workspaceOK := lookup("MROLESCOPE_QWEN_WORKSPACE_ID")
		if !keyOK || strings.TrimSpace(apiKey) == "" || !workspaceOK || strings.TrimSpace(workspaceID) == "" {
			return nil, &aitrend.ContractError{Code: aitrend.ErrorConfigurationFailure, Message: "真实评测 provider 未配置"}
		}
		if httpClient == nil {
			httpClient = http.DefaultClient
		}
		return aitrend.NewQwenIntentProvider(apiKey, workspaceID, httpClient)
	default:
		return nil, fmt.Errorf("unknown evaluation provider %q", mode)
	}
}
