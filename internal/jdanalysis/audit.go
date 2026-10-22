package jdanalysis

import (
	"encoding/json"
	"fmt"
)

type analysisAuditDefinition struct {
	Severity string
	Message  string
	Fields   map[string]struct{}
}

// Audit messages are selected by code, never passed through from a caller.
// Diagnostics are counts only; no text-bearing field can cross this boundary.
var analysisAuditDefinitions = map[string]analysisAuditDefinition{
	"analysis_run_started": {
		Severity: "info", Message: "analysis run started",
		Fields: map[string]struct{}{"claimed": {}},
	},
	"analysis_work_failed": {
		Severity: "error", Message: "analysis work failed safely",
		Fields: map[string]struct{}{"attempt": {}},
	},
	"analysis_work_completion_failed": {
		Severity: "error", Message: "analysis work completion failed safely",
		Fields: map[string]struct{}{"attempt": {}},
	},
	"candidate_prepared": {
		Severity: "info", Message: "candidate analysis configuration prepared",
		Fields: map[string]struct{}{"queued": {}},
	},
	"candidate_not_ready": {
		Severity: "warning", Message: "candidate analysis configuration is not ready",
		Fields: map[string]struct{}{"queued": {}, "pending": {}, "missing_results": {}},
	},
	"candidate_published": {
		Severity: "info", Message: "candidate analysis configuration published",
		Fields: map[string]struct{}{"queued": {}},
	},
	"candidate_failed": {
		Severity: "error", Message: "candidate analysis configuration failed",
		Fields: map[string]struct{}{},
	},
}

func safeAnalysisAudit(code string, diagnostics map[string]int) (severity, message string, encoded []byte, err error) {
	definition, exists := analysisAuditDefinitions[code]
	if !exists {
		return "", "", nil, fmt.Errorf("analysis audit code is not permitted")
	}
	safe := make(map[string]int, len(diagnostics))
	for field, value := range diagnostics {
		if _, allowed := definition.Fields[field]; !allowed || value < 0 {
			return "", "", nil, fmt.Errorf("analysis audit diagnostic is not permitted")
		}
		safe[field] = value
	}
	encoded, err = json.Marshal(safe)
	if err != nil {
		return "", "", nil, fmt.Errorf("encode safe analysis audit diagnostics: %w", err)
	}
	if len(encoded) > 8192 {
		return "", "", nil, fmt.Errorf("analysis audit diagnostics exceed size limit")
	}
	return definition.Severity, definition.Message, encoded, nil
}

// SafeOperationMessage intentionally discards untrusted errors. It is the
// only text that an analysis command or helper may expose for a failed run.
func SafeOperationMessage(err error) string {
	if err == nil {
		return "analysis operation completed"
	}
	if err == ErrConfigurationBusy {
		return "analysis configuration is busy"
	}
	if err == ErrCandidateNotReady {
		return "candidate analysis configuration is not ready"
	}
	if err == ErrCandidateFailed {
		return "candidate analysis configuration is failed"
	}
	if err == ErrNoPublishedConfiguration {
		return "no published analysis configuration is available"
	}
	return "analysis operation failed safely"
}
