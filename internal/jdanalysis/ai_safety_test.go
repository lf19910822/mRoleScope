package jdanalysis

import (
	"errors"
	"strings"
	"testing"
)

func TestNewSafeAIErrorDropsUntrustedDiagnostics(t *testing.T) {
	const (
		syntheticBody   = "synthetic JD body must never be returned"
		syntheticSecret = "synthetic-api-secret-must-never-be-returned"
	)

	err := NewSafeAIError(AISafeErrorInvalidResponseSchema, errors.New(syntheticBody+" "+syntheticSecret))
	if !strings.Contains(err.Error(), string(AISafeErrorInvalidResponseSchema)) {
		t.Fatalf("safe error = %q, want its approved code", err)
	}
	if strings.Contains(err.Error(), syntheticBody) || strings.Contains(err.Error(), syntheticSecret) {
		t.Fatalf("safe error leaked an untrusted diagnostic: %q", err)
	}

	invalidCode := AISafeErrorCode("untrusted_provider_message")
	err = NewSafeAIError(invalidCode, errors.New(syntheticBody+" "+syntheticSecret))
	if !strings.Contains(err.Error(), string(AISafeErrorConfigurationInvalid)) {
		t.Fatalf("invalid safe code error = %q, want %q", err, AISafeErrorConfigurationInvalid)
	}
	if strings.Contains(err.Error(), syntheticBody) || strings.Contains(err.Error(), syntheticSecret) || strings.Contains(err.Error(), string(invalidCode)) {
		t.Fatalf("invalid safe code error leaked untrusted content: %q", err)
	}
}
