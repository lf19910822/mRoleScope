package main

import (
	"net/http"
	"strings"
	"testing"

	"mrolescope/internal/aitrend"
	"mrolescope/internal/aitrend/evaluation"
)

func TestNewEvaluationProviderDefaultsToFixture(t *testing.T) {
	provider, err := newEvaluationProvider("", evaluation.Case{ID: "G01"}, http.DefaultClient, func(string) (string, bool) {
		t.Fatal("fixture mode read credentials")
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := provider.(*fixtureProvider); !ok {
		t.Fatalf("provider = %T, want fixtureProvider", provider)
	}
}

func TestNewEvaluationProviderRejectsUnknownMode(t *testing.T) {
	_, err := newEvaluationProvider("unknown", evaluation.Case{}, http.DefaultClient, func(string) (string, bool) { return "", false })
	if err == nil || !strings.Contains(err.Error(), "unknown evaluation provider") {
		t.Fatalf("unknown provider error = %v", err)
	}
}

func TestNewEvaluationProviderQwenRequiresCredentialsBeforeNetwork(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, nil
	})}
	secret := "must-not-leak"
	_, err := newEvaluationProvider("qwen", evaluation.Case{}, client, func(name string) (string, bool) {
		if name == "MROLESCOPE_QWEN_API_KEY" {
			return secret, true
		}
		return "", false
	})
	if err == nil {
		t.Fatal("missing workspace was accepted")
	}
	if requests != 0 {
		t.Fatalf("requests = %d, want 0", requests)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("credential leaked in error: %v", err)
	}
	contractErr, ok := err.(*aitrend.ContractError)
	if !ok || contractErr.Code != aitrend.ErrorConfigurationFailure {
		t.Fatalf("error = %#v, want configuration failure", err)
	}
}

func TestNewEvaluationProviderQwenBuildsProductionProvider(t *testing.T) {
	provider, err := newEvaluationProvider("qwen", evaluation.Case{}, http.DefaultClient, func(name string) (string, bool) {
		switch name {
		case "MROLESCOPE_QWEN_API_KEY":
			return "synthetic-key", true
		case "MROLESCOPE_QWEN_WORKSPACE_ID":
			return "llm-mrolescope", true
		default:
			return "", false
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := provider.(*aitrend.QwenIntentProvider); !ok {
		t.Fatalf("provider = %T, want QwenIntentProvider", provider)
	}
}

func TestEvaluationVersionMetadataDistinguishesProviders(t *testing.T) {
	if got := evaluationModelName("fixture"); got != "fixture-provider" {
		t.Fatalf("fixture model = %q", got)
	}
	if got := evaluationProviderName("fixture"); got != "offline-fixture" {
		t.Fatalf("fixture provider = %q", got)
	}
	if got := evaluationModelName("qwen"); got != "qwen-plus" {
		t.Fatalf("qwen model = %q", got)
	}
	if got := evaluationProviderName("qwen"); got != "qwen" {
		t.Fatalf("qwen provider = %q", got)
	}
}

func TestValidateEvaluationProviderConfigurationFailsBeforeRun(t *testing.T) {
	err := validateEvaluationProviderConfiguration("qwen", http.DefaultClient, func(string) (string, bool) { return "", false })
	if err == nil {
		t.Fatal("missing qwen configuration was accepted")
	}
	if contractErr, ok := err.(*aitrend.ContractError); !ok || contractErr.Code != aitrend.ErrorConfigurationFailure {
		t.Fatalf("configuration error = %#v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
