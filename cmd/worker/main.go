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
	"mrolescope/internal/jobsync"
)

func main() {
	runID := flag.String("run-id", "", "unique collection run ID (generated when empty)")
	outputDir := flag.String("output-dir", "", "private collector task directory below SYNC_TEMP_ROOT")
	fixture := flag.String("fixture", "", "offline collector fixture; development and tests only")
	trigger := flag.String("trigger", string(jobsync.TriggerInitialization), "initialization, scheduled, or retry")
	attemptNumber := flag.Int("attempt-number", 1, "positive whole-run attempt number")
	flag.Parse()
	if *outputDir == "" {
		fmt.Fprintln(os.Stderr, "--output-dir is required")
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration")
		os.Exit(1)
	}
	if err := cfg.ValidateSync(); err != nil {
		slog.Error("invalid synchronization configuration")
		os.Exit(1)
	}
	if !isWithin(filepath.Clean(*outputDir), cfg.SyncTempRoot) {
		slog.Error("unsafe worker output directory")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.SyncTimeout)
	defer cancel()

	repository, err := jobsync.NewRepository(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database initialization failed")
		os.Exit(1)
	}
	defer repository.Close()
	store, err := jobsync.NewCOSStore(ctx, cfg.COS)
	if err != nil {
		slog.Error("COS initialization failed")
		os.Exit(1)
	}
	service := jobsync.Service{
		Repository: repository,
		Collector:  collector.Runner{Python: cfg.CollectorPython, Script: cfg.CollectorScript},
		Archiver:   jobsync.Archiver{Store: store, Prefix: cfg.COS.Prefix},
		Rechecker:  jobsync.MihoyoDetailRechecker{},
	}
	if err := service.Run(ctx, jobsync.RunOptions{
		RunID: *runID, OutputDir: filepath.Clean(*outputDir), FixturePath: *fixture,
		Trigger: jobsync.TriggerKind(*trigger), AttemptNumber: *attemptNumber,
	}); err != nil {
		slog.Error("synchronization failed")
		os.Exit(1)
	}
	slog.Info("synchronization published", "trigger", *trigger, "attempt", *attemptNumber)
}

func isWithin(candidate, root string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !filepath.IsAbs(relative) && relative != "."
}
