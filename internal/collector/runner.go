package collector

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

type Runner struct {
	Python string
	Script string
}

// Run invokes the collector with a fixed executable and argument vector. It
// intentionally does not invoke a shell, so paths and run IDs are not parsed as commands.
func (r Runner) Run(ctx context.Context, runID, outputDir, fixturePath string) (Manifest, error) {
	arguments := []string{r.Script, "--run-id", runID, "--output-dir", outputDir}
	if fixturePath != "" {
		arguments = append(arguments, "--fixture", fixturePath)
	}
	command := exec.CommandContext(ctx, r.Python, arguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		return Manifest{}, fmt.Errorf("collector failed: %w: %s", err, string(output))
	}
	manifest, err := ReadManifest(filepath.Join(outputDir, "manifest.json"))
	if err != nil {
		return Manifest{}, err
	}
	if err := ValidateFiles(outputDir, manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}
