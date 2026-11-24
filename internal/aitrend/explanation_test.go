package aitrend

import (
	"strings"
	"testing"
)

func TestAssistantReplyContextSafeForModelRemovesOperationNames(t *testing.T) {
	context := AssistantReplyContext{
		Query:         "哪些项目的岗位最多？",
		ResponseModes: []ResponseMode{ResponseModeTrend},
		Draft: &DraftExplanationContext{
			Candidates:     []DraftExplanationCandidate{{Label: "原神", MatchStatus: MatchExact}},
			ToolOperations: []string{"batch_trend_summary", "raw_sql"},
		},
	}
	safe := context.SafeForModel()
	if safe.Draft == nil || len(safe.Draft.ToolOperations) != 0 {
		t.Fatalf("unsafe tool operations remained: %#v", safe)
	}
	if len(context.Draft.ToolOperations) != 2 {
		t.Fatalf("original context was mutated: %#v", context)
	}
}

func TestAssistantReplyContextSafeForModelSanitizesServiceModeFeedback(t *testing.T) {
	context := AssistantReplyContext{ServiceModeFeedback: &ServiceModeFeedback{
		Code:    ServiceModeFeedbackUnsupportedCombination,
		Message: "internal backend password and tool failure",
	}}
	safe := context.SafeForModel()
	if safe.ServiceModeFeedback == nil || safe.ServiceModeFeedback.Code != ServiceModeFeedbackUnsupportedCombination || safe.ServiceModeFeedback.Message != "部分请求暂不支持，已保留当前可以处理的结果。" {
		t.Fatalf("safe feedback = %#v", safe.ServiceModeFeedback)
	}
	if strings.Contains(safe.ServiceModeFeedback.Message, "password") || strings.Contains(safe.ServiceModeFeedback.Message, "backend") {
		t.Fatalf("internal feedback leaked: %#v", safe.ServiceModeFeedback)
	}
	invalid := (AssistantReplyContext{ServiceModeFeedback: &ServiceModeFeedback{Code: "backend_failure", Message: "internal"}}).SafeForModel()
	if invalid.ServiceModeFeedback != nil {
		t.Fatalf("invalid feedback was exposed: %#v", invalid.ServiceModeFeedback)
	}
}

func TestAssistantReplyQualityWarningsArePostStreamOnlySignals(t *testing.T) {
	if warnings := AssistantReplyQualityWarnings("按岗位数量整理候选。"); len(warnings) != 0 {
		t.Fatalf("valid warnings = %#v", warnings)
	}
	if warnings := AssistantReplyQualityWarnings("plain English"); len(warnings) != 1 || warnings[0] != "not_chinese" {
		t.Fatalf("language warnings = %#v", warnings)
	}
	if warnings := AssistantReplyQualityWarnings(strings.Repeat("中", MaxReplyRunes+1)); len(warnings) != 1 || warnings[0] != "too_long" {
		t.Fatalf("length warnings = %#v", warnings)
	}
}
