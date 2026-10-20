package collector

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Manifest struct {
	SchemaVersion int       `json:"schema_version"`
	Status        string    `json:"status"`
	RunID         string    `json:"run_id"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at"`
	Source        Source    `json:"source"`
	Counts        Counts    `json:"counts"`
	Files         []File    `json:"files"`
}

type Source struct {
	Company      string         `json:"company"`
	Filter       map[string]any `json:"filter"`
	FilterSHA256 string         `json:"filter_sha256"`
}

type Counts struct {
	ListReportedTotal int `json:"list_reported_total"`
	ListItems         int `json:"list_items"`
	UniqueSourceJobID int `json:"unique_source_job_ids"`
	DetailSucceeded   int `json:"detail_succeeded"`
	DetailFailed      int `json:"detail_failed"`
}

type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

func ReadManifest(path string) (Manifest, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(bytes, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func (m Manifest) Validate() error {
	if m.SchemaVersion != 1 {
		return fmt.Errorf("unsupported manifest schema_version %d", m.SchemaVersion)
	}
	if m.Status != "complete" {
		return fmt.Errorf("manifest status must be complete, got %q", m.Status)
	}
	if m.RunID == "" || m.Source.Company == "" || len(m.Source.Filter) == 0 || !sha256Pattern.MatchString(m.Source.FilterSHA256) {
		return fmt.Errorf("manifest identity or source is invalid")
	}
	if m.StartedAt.IsZero() || m.FinishedAt.IsZero() || m.FinishedAt.Before(m.StartedAt) {
		return fmt.Errorf("manifest timestamps are invalid")
	}
	c := m.Counts
	if c.ListReportedTotal <= 0 || c.ListItems != c.ListReportedTotal || c.UniqueSourceJobID != c.ListReportedTotal || c.DetailSucceeded != c.ListReportedTotal || c.DetailFailed != 0 {
		return fmt.Errorf("manifest counts are incomplete or inconsistent")
	}
	if len(m.Files) == 0 {
		return fmt.Errorf("manifest has no archived files")
	}
	seenPaths := make(map[string]struct{}, len(m.Files))
	for _, file := range m.Files {
		if !isSafeRelativePath(file.Path) || file.Bytes < 0 || !sha256Pattern.MatchString(file.SHA256) {
			return fmt.Errorf("manifest file entry is invalid: %q", file.Path)
		}
		if _, exists := seenPaths[file.Path]; exists {
			return fmt.Errorf("manifest file path is duplicated: %q", file.Path)
		}
		seenPaths[file.Path] = struct{}{}
	}
	return nil
}

func ValidateFiles(root string, manifest Manifest) error {
	for _, file := range manifest.Files {
		path := filepath.Join(root, file.Path)
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("stat %s: %w", file.Path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || info.IsDir() || info.Size() != file.Bytes {
			return fmt.Errorf("file size mismatch for %s", file.Path)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", file.Path, err)
		}
		sum := sha256.Sum256(contents)
		if hex.EncodeToString(sum[:]) != file.SHA256 {
			return fmt.Errorf("file checksum mismatch for %s", file.Path)
		}
	}
	return nil
}

func isSafeRelativePath(path string) bool {
	clean := filepath.Clean(path)
	return path != "" && !filepath.IsAbs(path) && clean != "." && !strings.HasPrefix(clean, ".."+string(filepath.Separator)) && clean != ".."
}
