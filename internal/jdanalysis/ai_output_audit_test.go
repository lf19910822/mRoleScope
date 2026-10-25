package jdanalysis

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestAIOutputAuditStoreCapturesRawOutputWithRootOnlyPermissions(t *testing.T) {
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	store := newAIOutputAuditStore(t.TempDir(), aiOutputAuditPolicy{
		CapacityBytes: 64 << 20,
		Retention:     7 * 24 * time.Hour,
		Now:           func() time.Time { return now },
	})
	identity := AIOutputAuditIdentity{
		ConfigurationID:      "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		WorkItemID:           "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		AttemptNumber:        1,
		BodyContentVersionID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
	}
	wantResponse := []byte(`{"request_id":"opaque","experience":{"status":"uncertain"}}`)

	reservation, err := store.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	artifact, err := reservation.Capture(identity, wantResponse)
	if err != nil {
		t.Fatalf("Capture() error = %v", err)
	}

	gotResponse, err := os.ReadFile(artifact.ResponsePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotResponse, wantResponse) {
		t.Fatalf("captured response = %q, want byte-identical %q", gotResponse, wantResponse)
	}
	if artifact.ResponseSHA256 != sha256Hex(string(wantResponse)) {
		t.Fatalf("response SHA-256 = %q", artifact.ResponseSHA256)
	}
	for _, path := range []string{store.root, filepath.Dir(artifact.ResponsePath), artifact.ResponsePath, artifact.MetadataPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		wantMode := os.FileMode(0o600)
		if info.IsDir() {
			wantMode = 0o700
		}
		if info.Mode().Perm() != wantMode {
			t.Fatalf("mode for %s = %04o, want %04o", path, info.Mode().Perm(), wantMode)
		}
	}
	metadata, err := os.ReadFile(artifact.MetadataPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(metadata, wantResponse) {
		t.Fatal("metadata contains raw model output")
	}
}

func TestAIOutputAuditStoreCleansExpiredArtifactsAndRefusesCapacityOverflow(t *testing.T) {
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	policy := aiOutputAuditPolicy{
		CapacityBytes: aiOutputAuditReservationBytes,
		Retention:     7 * 24 * time.Hour,
		Now:           func() time.Time { return now },
	}
	store := newAIOutputAuditStore(t.TempDir(), policy)
	identity := AIOutputAuditIdentity{
		ConfigurationID:      "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		WorkItemID:           "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		AttemptNumber:        1,
		BodyContentVersionID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
	}
	first, err := store.Reserve()
	if err != nil {
		t.Fatalf("first Reserve() error = %v", err)
	}
	firstArtifact, err := first.Capture(identity, bytes.Repeat([]byte{'x'}, aiOutputAuditMaxResponseBytes))
	if err != nil {
		t.Fatalf("first Capture() error = %v", err)
	}
	if _, err := store.Reserve(); !errors.Is(err, ErrAIOutputAuditCapacity) {
		t.Fatalf("second Reserve() error = %v, want capacity refusal", err)
	}

	now = now.Add(7*24*time.Hour + time.Second)
	second, err := store.Reserve()
	if err != nil {
		t.Fatalf("Reserve() after expiry error = %v", err)
	}
	if _, err := os.Stat(firstArtifact.ResponsePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired response stat error = %v, want not exist", err)
	}
	if err := second.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
}

func TestAIOutputAuditStoreReportsOnlySafeCapacityCounts(t *testing.T) {
	store := newAIOutputAuditStore(t.TempDir(), aiOutputAuditPolicy{
		CapacityBytes: 2 * aiOutputAuditReservationBytes,
		Retention:     7 * 24 * time.Hour,
	})
	identity := AIOutputAuditIdentity{
		ConfigurationID:      "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		WorkItemID:           "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		AttemptNumber:        1,
		BodyContentVersionID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
	}
	reservation, err := store.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reservation.Capture(identity, []byte(`{"synthetic":"response-must-not-appear-in-status"}`)); err != nil {
		t.Fatal(err)
	}

	status, err := store.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.CapturedArtifactCount != 1 || status.CapacityBytes != 2*aiOutputAuditReservationBytes || status.UsedBytes <= 0 || status.AvailableBytes != status.CapacityBytes-status.UsedBytes {
		t.Fatalf("safe output-audit status = %#v", status)
	}
}

func TestAIOutputAuditPolicyIdentityUsesTheEnforcedDefaultPolicy(t *testing.T) {
	policy := defaultAIOutputAuditPolicy()
	components := aiOutputAuditInputContractComponents()
	want := []string{
		AIOutputAuditPolicyVersionV1,
		"retention=" + aiOutputAuditRetentionContractValue(policy.Retention),
		"capacity_bytes=" + strconv.FormatInt(policy.CapacityBytes, 10),
		"response_reservation_bytes=" + strconv.FormatInt(aiOutputAuditReservationBytes, 10),
	}
	if !equalStrings(components, want) {
		t.Fatalf("audit policy identity components = %q, want enforced defaults %q", components, want)
	}
}

func TestAIOutputAuditStoreRemovesIncompleteArtifactPairsBeforeReservation(t *testing.T) {
	store := NewAIOutputAuditStore(t.TempDir())
	identity := AIOutputAuditIdentity{
		ConfigurationID:      "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		WorkItemID:           "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		AttemptNumber:        1,
		BodyContentVersionID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
	}
	directory := filepath.Join(store.root, identity.ConfigurationID, identity.WorkItemID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	orphanResponse := filepath.Join(directory, "attempt-01.response.json")
	orphanMetadata := filepath.Join(directory, "attempt-02"+aiOutputAuditMetadataFileExtension)
	if err := os.WriteFile(orphanResponse, []byte(`{"orphan":"response"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orphanMetadata, []byte(`{"orphan":"metadata"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	reservation, err := store.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if err := reservation.Release(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{orphanResponse, orphanMetadata} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("incomplete audit artifact %q stat error = %v, want removal", path, err)
		}
	}
}
