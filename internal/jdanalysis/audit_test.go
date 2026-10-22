package jdanalysis

import (
	"errors"
	"strings"
	"testing"
)

func TestSafeAnalysisAuditAllowsOnlyFixedMessagesAndCountFields(t *testing.T) {
	severity, message, diagnostics, err := safeAnalysisAudit("candidate_not_ready", map[string]int{
		"queued": 1, "pending": 2, "missing_results": 3,
	})
	if err != nil || severity != "warning" || message != "candidate analysis configuration is not ready" || string(diagnostics) != `{"missing_results":3,"pending":2,"queued":1}` {
		t.Fatalf("safe audit = severity:%q message:%q diagnostics:%s err:%v", severity, message, diagnostics, err)
	}
	if _, _, _, err := safeAnalysisAudit("source_body_text", map[string]int{}); err == nil {
		t.Fatal("unknown audit code was accepted")
	}
	if _, _, _, err := safeAnalysisAudit("candidate_prepared", map[string]int{"raw_body": 1}); err == nil {
		t.Fatal("unapproved diagnostic field was accepted")
	}
}

func TestSafeOperationMessageNeverEchoesSourceOrCredentials(t *testing.T) {
	upstream := errors.New("password=synthetic-secret <p>synthetic JD body</p> token=synthetic-token")
	message := SafeOperationMessage(upstream)
	for _, forbidden := range []string{"synthetic-secret", "synthetic JD body", "synthetic-token", "password=", "token="} {
		if strings.Contains(message, forbidden) {
			t.Fatalf("safe operation message leaked %q: %q", forbidden, message)
		}
	}
}
