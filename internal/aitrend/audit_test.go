package aitrend

import (
	"strings"
	"testing"
	"time"
)

func TestAuditRecordIsBoundedAndIrreversible(t *testing.T) {
	created := time.Now()
	record := AuditRecord{RequestHash: HashRequest([]byte("哪些数据库")), ContractVersion: ContractVersionV1, ValidationStatus: "valid", Status: "success", CreatedAt: created, ExpiresAt: created.Add(AuditRetention)}
	if err := record.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(record.RequestHash) != 64 || strings.Contains(record.RequestHash, "数据库") {
		t.Fatalf("request hash = %q", record.RequestHash)
	}
	tooMany := record
	tooMany.ToolCallCount = MaxToolCalls + 1
	if err := tooMany.Validate(); err == nil {
		t.Fatal("unbounded tool count accepted")
	}
}

func TestAuditRecordUsesMigrationValidationStatuses(t *testing.T) {
	created := time.Now()
	for _, status := range []string{"not_started", "valid", "invalid", "corrected"} {
		record := AuditRecord{
			RequestHash:      HashRequest([]byte(status)),
			ContractVersion:  ContractVersionV1,
			ValidationStatus: status,
			Status:           "success",
			CreatedAt:        created,
			ExpiresAt:        created.Add(AuditRetention),
		}
		if err := record.Validate(); err != nil {
			t.Fatalf("validation status %q rejected: %v", status, err)
		}
	}
}
