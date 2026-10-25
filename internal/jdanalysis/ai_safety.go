package jdanalysis

import "fmt"

// AISafeErrorCode is the complete allowlist for persistent AI diagnostics.
// These identifiers are deliberately semantic-free: no provider body, prompt,
// response, source text, URL, request ID, or credential may be embedded in a
// code or ordinary error string.
type AISafeErrorCode string

const (
	AISafeErrorInputTooLarge          AISafeErrorCode = "input_too_large"
	AISafeErrorBudgetExhausted        AISafeErrorCode = "budget_exhausted"
	AISafeErrorProviderTimeout        AISafeErrorCode = "provider_timeout"
	AISafeErrorProviderRateLimited    AISafeErrorCode = "provider_rate_limited"
	AISafeErrorProviderServer         AISafeErrorCode = "provider_server_error"
	AISafeErrorProviderAuthentication AISafeErrorCode = "provider_authentication_failed"
	AISafeErrorInvalidResponseSchema  AISafeErrorCode = "invalid_response_schema"
	AISafeErrorInvalidEvidence        AISafeErrorCode = "invalid_evidence_reference"
	AISafeErrorPolicyRejected         AISafeErrorCode = "policy_rejected"
	AISafeErrorConfigurationInvalid   AISafeErrorCode = "configuration_invalid"
)

func (code AISafeErrorCode) valid() bool {
	switch code {
	case AISafeErrorInputTooLarge,
		AISafeErrorBudgetExhausted,
		AISafeErrorProviderTimeout,
		AISafeErrorProviderRateLimited,
		AISafeErrorProviderServer,
		AISafeErrorProviderAuthentication,
		AISafeErrorInvalidResponseSchema,
		AISafeErrorInvalidEvidence,
		AISafeErrorPolicyRejected,
		AISafeErrorConfigurationInvalid:
		return true
	default:
		return false
	}
}

type safeAIError struct {
	code AISafeErrorCode
}

func (err safeAIError) Error() string {
	return fmt.Sprintf("AI semantic analysis failed: %s", err.code)
}

// NewSafeAIError intentionally discards the cause. The raw cause can contain
// a JD fragment, a provider response or credentials, none of which is allowed
// across the analysis error/audit boundary.
func NewSafeAIError(code AISafeErrorCode, _ error) error {
	if !code.valid() {
		code = AISafeErrorConfigurationInvalid
	}
	return safeAIError{code: code}
}
