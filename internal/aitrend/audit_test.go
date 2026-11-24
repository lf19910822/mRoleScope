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
	tooManyModes := record
	tooManyModes.ResponseModes = []ResponseMode{ResponseModeTrend, ResponseModeGuidance, ResponseModeTrend}
	if err := tooManyModes.Validate(); err == nil {
		t.Fatal("unbounded response modes accepted")
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

func TestAuditRecordStoresBoundedServiceClassificationSummary(t *testing.T) {
	created := time.Now().UTC()
	record := AuditRecord{
		RequestHash: HashRequest([]byte("分类")), ContractVersion: ContractVersionV1, ValidationStatus: "valid", Status: "success",
		ServiceTags: []ServiceTag{ServiceTagTrend, ServiceTagGuidance}, ClassificationStatus: "completed",
		ClassificationInputTokenCount: 12, ClassificationOutputTokenCount: 4, ClassificationDuration: 250 * time.Millisecond,
		CreatedAt: created, ExpiresAt: created.Add(AuditRetention), ResponseModes: []ResponseMode{ResponseModeTrend, ResponseModeGuidance},
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("bounded classification record rejected: %v", err)
	}
	record.ServiceTags = []ServiceTag{ServiceTagTrend, ServiceTagTrend}
	if err := record.Validate(); err == nil {
		t.Fatal("duplicate service tags accepted")
	}
}
