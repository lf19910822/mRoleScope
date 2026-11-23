package evaluation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteJSONArtifactUsesIndependentRestrictedDirectory(t *testing.T) {
	root := t.TempDir()
	path, err := WriteJSONArtifact(root, "trace.json", map[string]string{"status": "success"})
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("artifact mode = %o, want 600", info.Mode().Perm())
	}
	if _, err := WriteJSONArtifact(root, "../outside.json", map[string]string{}); err == nil {
		t.Fatal("path traversal was accepted")
	}
	if filepath.Dir(path) != root {
		t.Fatalf("artifact escaped independent root: %s", path)
	}
}
