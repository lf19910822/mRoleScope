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
	"mrolescope/internal/jdanalysis"
	"mrolescope/internal/jobsync"
)

func main() {
	arguments := os.Args[1:]
	if len(arguments) > 0 {
		switch arguments[0] {
		case "sync":
			runSync(arguments[1:])
			return
		case "analyze":
			runPendingAnalysis(arguments[1:])
			return
		case "backfill-versioned-content":
			runVersionedContentBackfill(arguments[1:])
			return
		}
	}
	// Preserve the reviewed synchronization helper's existing flag-only
	// invocation while making new analysis execution explicit.
	runSync(arguments)
}

func runSync(arguments []string) {
	flags := flag.NewFlagSet("sync", flag.ExitOnError)
	runID := flags.String("run-id", "", "unique collection run ID (generated when empty)")
	outputDir := flags.String("output-dir", "", "private collector task directory below SYNC_TEMP_ROOT")
	fixture := flags.String("fixture", "", "offline collector fixture; development and tests only")
	trigger := flags.String("trigger", string(jobsync.TriggerInitialization), "initialization, scheduled, or retry")
	attemptNumber := flags.Int("attempt-number", 1, "positive whole-run attempt number")
	_ = flags.Parse(arguments)
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

func runPendingAnalysis(arguments []string) {
	flags := flag.NewFlagSet("analyze", flag.ExitOnError)
	_ = flags.Parse(arguments)
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "analyze accepts no positional arguments")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid analysis configuration")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.SyncTimeout)
	defer cancel()
	repository, err := jdanalysis.NewRepository(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("analysis database initialization failed")
		os.Exit(1)
	}
	defer repository.Close()
	configurationID, err := repository.PublishedConfigurationID(ctx)
	if err != nil {
		slog.Error(jdanalysis.SafeOperationMessage(err))
		os.Exit(1)
	}
	summary, err := (jdanalysis.Processor{Repository: repository}).ProcessPending(ctx, configurationID)
	if err != nil {
		slog.Error(jdanalysis.SafeOperationMessage(err), "claimed", summary.Claimed, "completed", summary.Completed, "failed", summary.Failed)
		os.Exit(1)
	}
	slog.Info("analysis completed", "claimed", summary.Claimed, "completed", summary.Completed, "failed", summary.Failed)
}

// runVersionedContentBackfill has a deliberately narrow interface because it
// is invoked only through the root-owned remote maintenance helper. It reads
// the already-published baseline from PostgreSQL and emits counts only.
func runVersionedContentBackfill(arguments []string) {
	flags := flag.NewFlagSet("backfill-versioned-content", flag.ExitOnError)
	_ = flags.Parse(arguments)
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "backfill-versioned-content accepts no positional arguments")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid versioned-content backfill configuration")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.SyncTimeout)
	defer cancel()
	repository, err := jobsync.NewRepository(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("versioned-content backfill database initialization failed")
		os.Exit(1)
	}
	defer repository.Close()
	summary, err := repository.BackfillVersionedContent(ctx)
	if err != nil {
		// Repository errors may wrap database values. Never expose them from a
		// command that operates over real remote JD data.
		slog.Error("versioned-content baseline backfill failed")
		os.Exit(1)
	}
	slog.Info("versioned-content baseline backfill completed",
		"published_observations", summary.PublishedObservations,
		"version_references_created", summary.VersionReferencesCreated,
		"body_periods_created", summary.BodyPeriodsCreated,
		"skipped_observations", summary.SkippedObservations,
		"skipped_body_periods", summary.SkippedBodyPeriods,
	)
}

func isWithin(candidate, root string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !filepath.IsAbs(relative) && relative != "."
}
