package jdanalysis

import "testing"

func TestDictionaryLifecycleInputFormats(t *testing.T) {
	for _, operator := range []string{"github-user", "operator_01", "A.1"} {
		if !validOperator(operator) {
			t.Fatalf("validOperator(%q) = false", operator)
		}
	}
	for _, operator := range []string{"", "-operator", "_operator", ".operator", "operator space", "operator/unsafe"} {
		if validOperator(operator) {
			t.Fatalf("validOperator(%q) = true", operator)
		}
	}

	for _, ruleKey := range []string{"kubernetes.01", "tensorrt-llm.01", "go.01"} {
		if !validRuleKey(ruleKey) {
			t.Fatalf("validRuleKey(%q) = false", ruleKey)
		}
	}
	for _, ruleKey := range []string{"", ".go.01", "-go.01", "go.01.", "Go.01", "go_01", "go/01"} {
		if validRuleKey(ruleKey) {
			t.Fatalf("validRuleKey(%q) = true", ruleKey)
		}
	}
}
