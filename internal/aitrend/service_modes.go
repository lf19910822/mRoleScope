package aitrend

import (
	"context"
	"encoding/json"
	"fmt"
)

type serviceTagsContextKey struct{}

const ServiceModeContractVersionV1 = "ai-trend-service-mode-v1"

const ServiceModeFeedbackUnsupportedCombination = "unsupported_combination"

type ServiceTag string

const (
	ServiceTagTrend    ServiceTag = "trend"
	ServiceTagGuidance ServiceTag = "guidance"
)

var serviceTagOrder = []ServiceTag{ServiceTagTrend, ServiceTagGuidance}

// ServiceModeClassification is the bounded result of the first, lightweight
// model call. It is deliberately separate from AgentResponse so the service
// classifier can grow without changing the user-facing Agent contract.
type ServiceModeClassification struct {
	ContractVersion string       `json:"contract_version"`
	ServiceTags     []ServiceTag `json:"service_tags"`
}

// ServiceModeFeedback is a small, server-authored constraint passed to the
// reply model when a recognized service combination cannot be fully handled.
// The model never receives the internal error or tool details that led to it.
type ServiceModeFeedback struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func safeServiceModeFeedback(feedback *ServiceModeFeedback) *ServiceModeFeedback {
	if feedback == nil || feedback.Code != ServiceModeFeedbackUnsupportedCombination {
		return nil
	}
	return &ServiceModeFeedback{
		Code:    ServiceModeFeedbackUnsupportedCombination,
		Message: "部分请求暂不支持，已保留当前可以处理的结果。",
	}
}

// IntentServiceModeClassifier is the model boundary for the first request
// classification call. It must not call Serving tools or produce a Draft.
type IntentServiceModeClassifier interface {
	ClassifyServiceModes(context.Context, string) (ServiceModeClassification, int, int, error)
}

func withServiceTags(ctx context.Context, tags []ServiceTag) context.Context {
	return context.WithValue(ctx, serviceTagsContextKey{}, append([]ServiceTag(nil), tags...))
}

func serviceTagsFromContext(ctx context.Context) []ServiceTag {
	if ctx == nil {
		return nil
	}
	tags, _ := ctx.Value(serviceTagsContextKey{}).([]ServiceTag)
	return append([]ServiceTag(nil), tags...)
}

func serviceTagsContain(tags []ServiceTag, wanted ServiceTag) bool {
	for _, tag := range tags {
		if tag == wanted {
			return true
		}
	}
	return false
}

func NormalizeServiceTags(tags []ServiceTag) ([]ServiceTag, error) {
	seen := make(map[ServiceTag]struct{}, len(tags))
	for _, tag := range tags {
		if tag != ServiceTagTrend && tag != ServiceTagGuidance {
			return nil, contractError(ErrorInvalidEnum, "service_tags", fmt.Sprintf("unsupported service tag %q", tag))
		}
		if _, exists := seen[tag]; exists {
			return nil, contractError(ErrorInvalidCombination, "service_tags", "duplicate service tag")
		}
		seen[tag] = struct{}{}
	}
	if len(seen) == 0 {
		return nil, contractError(ErrorMissingField, "service_tags", "at least one service tag is required")
	}
	result := make([]ServiceTag, 0, len(seen))
	for _, tag := range serviceTagOrder {
		if _, exists := seen[tag]; exists {
			result = append(result, tag)
		}
	}
	return result, nil
}

func ValidateServiceModeClassification(classification ServiceModeClassification) error {
	if classification.ContractVersion != ServiceModeContractVersionV1 {
		return contractError(ErrorInvalidEnum, "contract_version", "unsupported service mode contract version")
	}
	_, err := NormalizeServiceTags(classification.ServiceTags)
	return err
}

func ResponseModesFromServiceTags(tags []ServiceTag) ([]ResponseMode, error) {
	normalized, err := NormalizeServiceTags(tags)
	if err != nil {
		return nil, err
	}
	modes := make([]ResponseMode, 0, len(normalized))
	for _, tag := range normalized {
		modes = append(modes, ResponseMode(tag))
	}
	return modes, nil
}

func decodeServiceModeClassification(data []byte) (ServiceModeClassification, error) {
	var classification ServiceModeClassification
	if err := decodeStrict(data, &classification); err != nil {
		return ServiceModeClassification{}, &ContractError{Code: ErrorInvalidJSON, Message: err.Error()}
	}
	if err := ValidateServiceModeClassification(classification); err != nil {
		return ServiceModeClassification{}, err
	}
	normalized, err := NormalizeServiceTags(classification.ServiceTags)
	if err != nil {
		return ServiceModeClassification{}, err
	}
	classification.ServiceTags = normalized
	return classification, nil
}

var serviceModeSchemaV1 = json.RawMessage(`{"type":"object","additionalProperties":false,"required":["contract_version","service_tags"],"properties":{"contract_version":{"const":"ai-trend-service-mode-v1"},"service_tags":{"type":"array","minItems":1,"maxItems":2,"items":{"enum":["trend","guidance"]}}}}`)
