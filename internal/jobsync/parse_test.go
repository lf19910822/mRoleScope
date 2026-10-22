package jobsync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mrolescope/internal/collector"
)

func TestParseObservationsAcceptsValidatedTaskDirectory(t *testing.T) {
	root, manifest := writeTaskDirectory(t, `{"success":true,"data":{"total":1,"list":[{"id":"8771","title":"Server","addressDetailList":[{"addressDetail":"Shanghai"}]}]}}`, `{"success":true,"data":{"id":"8771","title":"Server","description":"<p>Build services</p>","jobRequire":"<p>Go</p>"}}`)

	observations, err := ParseObservations(root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(observations) != 1 || observations[0].SourceJobID != "8771" {
		t.Fatalf("observations = %#v, want one job 8771", observations)
	}
}

func TestParseObservationsRejectsInvalidOrIncompleteEvidence(t *testing.T) {
	t.Run("detail_id_mismatch", func(t *testing.T) {
		root, manifest := writeTaskDirectory(t, `{"success":true,"data":{"total":1,"list":[{"id":"8771"}]}}`, `{"success":true,"data":{"id":"other","title":"Server","description":"body","jobRequire":"Go"}}`)
		if _, err := ParseObservations(root, manifest); err == nil {
			t.Fatal("expected mismatched detail ID to be rejected")
		}
	})

	t.Run("tampered_file", func(t *testing.T) {
		root, manifest := writeTaskDirectory(t, `{"success":true,"data":{"total":1,"list":[{"id":"8771"}]}}`, `{"success":true,"data":{"id":"8771","title":"Server","description":"body","jobRequire":"Go"}}`)
		if err := os.WriteFile(filepath.Join(root, "raw/details/8771.json"), []byte(`{"tampered":true}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ParseObservations(root, manifest); err == nil {
			t.Fatal("expected checksum mismatch to be rejected")
		}
	})

	t.Run("missing_detail_file", func(t *testing.T) {
		root, manifest := writeTaskDirectory(t, `{"success":true,"data":{"total":1,"list":[{"id":"8771"}]}}`, `{"success":true,"data":{"id":"8771","title":"Server","description":"body","jobRequire":"Go"}}`)
		manifest.Files = manifest.Files[:1]
		if _, err := ParseObservations(root, manifest); err == nil {
			t.Fatal("expected incomplete details to be rejected")
		}
	})
}

func writeTaskDirectory(t *testing.T, list, detail string) (string, collector.Manifest) {
	return writeTaskDirectoryForID(t, "8771", list, detail)
}

func writeTaskDirectoryForID(t *testing.T, sourceJobID, list, detail string) (string, collector.Manifest) {
	t.Helper()
	root := t.TempDir()
	files := []struct {
		path     string
		contents string
	}{
		{path: "raw/list/page-001.json", contents: list},
		{path: fmt.Sprintf("raw/details/%s.json", sourceJobID), contents: detail},
	}
	entries := make([]collector.File, 0, len(files))
	for _, file := range files {
		fullPath := filepath.Join(root, file.path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatal(err)
		}
		contents := []byte(file.contents)
		if err := os.WriteFile(fullPath, contents, 0o600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(contents)
		entries = append(entries, collector.File{Path: file.path, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(contents))})
	}
	now := time.Date(2026, 10, 22, 2, 0, 0, 0, time.UTC)
	scope := MihoyoTechnicalScope()
	return root, collector.Manifest{
		SchemaVersion: 1,
		Status:        "complete",
		RunID:         "parse-fixture",
		StartedAt:     now,
		FinishedAt:    now,
		Source:        collector.Source{Company: scope.SourceCompany, Filter: scope.Filter, FilterSHA256: scope.FilterSHA256},
		Counts:        collector.Counts{ListReportedTotal: 1, ListItems: 1, UniqueSourceJobID: 1, DetailSucceeded: 1},
		Files:         entries,
	}
}
