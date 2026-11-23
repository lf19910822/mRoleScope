package evaluation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const DefaultArtifactRoot = "var/ai-evaluation"

// WriteJSONArtifact writes a bounded evaluation artifact outside Serving and
// ordinary user response paths. The caller supplies a run-specific filename;
// path traversal and symlink-like absolute paths are rejected.
func WriteJSONArtifact(root, name string, value any) (string, error) {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(name) == "" || filepath.Base(name) != name || name == "." || name == ".." {
		return "", fmt.Errorf("invalid evaluation artifact path")
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal evaluation artifact: %w", err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", fmt.Errorf("create evaluation artifact directory: %w", err)
	}
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write evaluation artifact: %w", err)
	}
	return path, nil
}
