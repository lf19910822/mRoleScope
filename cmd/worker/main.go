package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"mrolescope/internal/collector"
	"mrolescope/internal/config"
)

func main() {
	runID := flag.String("run-id", "", "unique collection run ID")
	outputDir := flag.String("output-dir", "", "collector task directory")
	fixture := flag.String("fixture", "", "offline collector fixture; development and tests only")
	flag.Parse()
	if *runID == "" || *outputDir == "" {
		fmt.Fprintln(os.Stderr, "--run-id and --output-dir are required")
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	context, cancel := context.WithTimeout(context.Background(), cfg.CollectorTimeout)
	defer cancel()

	manifest, err := (collector.Runner{Python: cfg.CollectorPython, Script: cfg.CollectorScript}).Run(
		context, *runID, filepath.Clean(*outputDir), *fixture,
	)
	if err != nil {
		slog.Error("collection validation failed", "error", err)
		os.Exit(1)
	}
	slog.Info("collector task validated", "run_id", manifest.RunID, "jobs", manifest.Counts.DetailSucceeded)
	// M0 intentionally stops here. The persistent state machine, archive uploader,
	// parser and atomic publication transition land before the first chart cycle.
}
