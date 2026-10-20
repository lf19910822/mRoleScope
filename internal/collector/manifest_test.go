package collector

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidateFilesAcceptsCompleteManifest(t *testing.T) {
	root := t.TempDir()
	contents := []byte(`{"success":true}`)
	if err := os.MkdirAll(filepath.Join(root, "raw/list"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "raw/list/page-001.json"), contents, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(contents)
	now := time.Now().UTC()
	manifest := Manifest{
		SchemaVersion: 1, Status: "complete", RunID: "test-run", StartedAt: now, FinishedAt: now,
		Source: Source{Company: "mihoyo", Filter: map[string]any{"hireType": 0}, FilterSHA256: hex.EncodeToString(sum[:])},
		Counts: Counts{ListReportedTotal: 1, ListItems: 1, UniqueSourceJobID: 1, DetailSucceeded: 1},
		Files:  []File{{Path: "raw/list/page-001.json", SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(contents))}},
	}
	if err := manifest.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFiles(root, manifest); err != nil {
		t.Fatal(err)
	}
}

func TestManifestRejectsTraversalPath(t *testing.T) {
	now := time.Now().UTC()
	manifest := Manifest{
		SchemaVersion: 1, Status: "complete", RunID: "test-run", StartedAt: now, FinishedAt: now,
		Source: Source{Company: "mihoyo", Filter: map[string]any{"hireType": 0}, FilterSHA256: "0000000000000000000000000000000000000000000000000000000000000000"},
		Counts: Counts{ListReportedTotal: 1, ListItems: 1, UniqueSourceJobID: 1, DetailSucceeded: 1},
		Files:  []File{{Path: "../secret", SHA256: "0000000000000000000000000000000000000000000000000000000000000000", Bytes: 0}},
	}
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected unsafe path rejection")
	}
}
