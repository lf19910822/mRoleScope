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
		case "dictionary-v0.1":
			runV01Dictionary(arguments[1:])
			return
		case "dictionary-v0.2":
			runV02Dictionary(arguments[1:])
			return
		case "analyze-candidate-v0.1":
			runV01CandidateAnalysis(arguments[1:])
			return
		case "analyze-candidate-v0.2":
			runV02CandidateAnalysis(arguments[1:])
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

// runV01Dictionary exposes only the fixed v0.1 lifecycle. It cannot accept a
// dictionary document, SQL, parser version, database address, or source text
// from a caller. The root-owned adapter supplies the authenticated operator.
func runV01Dictionary(arguments []string) {
	if len(arguments) < 2 {
		fmt.Fprintln(os.Stderr, "dictionary-v0.1 accepts seed <operator>, approve <rule-key> <operator>, or prepare-analysis <operator>")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid dictionary lifecycle configuration")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.SyncTimeout)
	defer cancel()
	repository, err := jdanalysis.NewRepository(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("dictionary lifecycle database initialization failed")
		os.Exit(1)
	}
	defer repository.Close()

	seed := jdanalysis.CandidateV01Seed()
	switch arguments[0] {
	case "seed":
		if len(arguments) != 2 {
			fmt.Fprintln(os.Stderr, "dictionary-v0.1 seed accepts exactly one operator")
			os.Exit(2)
		}
		summary, err := repository.ImportCandidateDictionary(ctx, seed, arguments[1])
		if err != nil {
			slog.Error("candidate dictionary import failed")
			os.Exit(1)
		}
		slog.Info("candidate dictionary import completed", "rules_created", summary.RulesCreated, "candidates_created", summary.CandidatesCreated, "reused", summary.Reused)
	case "approve":
		if len(arguments) != 3 {
			fmt.Fprintln(os.Stderr, "dictionary-v0.1 approve accepts exactly one rule key and one operator")
			os.Exit(2)
		}
		summary, err := repository.ApproveCandidateRule(ctx, seed.VersionLabel, arguments[1], arguments[2])
		if err != nil {
			slog.Error("candidate dictionary approval failed")
			os.Exit(1)
		}
		slog.Info("candidate dictionary rule approval completed", "reused", summary.Reused)
	case "prepare-analysis":
		if len(arguments) != 2 {
			fmt.Fprintln(os.Stderr, "dictionary-v0.1 prepare-analysis accepts exactly one operator")
			os.Exit(2)
		}
		summary, err := repository.PrepareCandidateConfigurationForDictionary(ctx, seed.VersionLabel, jdanalysis.V01ParserVersion, arguments[1])
		if err != nil {
			slog.Error("candidate analysis preparation failed")
			os.Exit(1)
		}
		slog.Info("candidate analysis preparation completed", "queued", summary.Queued, "reused", summary.Reused)
	default:
		fmt.Fprintln(os.Stderr, "dictionary-v0.1 operation must be seed, approve, or prepare-analysis")
		os.Exit(2)
	}
}

// runV02Dictionary exposes only the reviewed v0.2 lifecycle. The rule key is
// still passed through the root-owned adapter, but the repository rejects any
// key outside the completed local review allowlist before it can be enabled.
func runV02Dictionary(arguments []string) {
	if len(arguments) < 2 {
		fmt.Fprintln(os.Stderr, "dictionary-v0.2 accepts seed <operator>, approve <rule-key> <operator>, approve-reviewed <operator>, or prepare-analysis <operator>")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid dictionary lifecycle configuration")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.SyncTimeout)
	defer cancel()
	repository, err := jdanalysis.NewRepository(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("dictionary lifecycle database initialization failed")
		os.Exit(1)
	}
	defer repository.Close()

	seed := jdanalysis.CandidateV02Seed()
	switch arguments[0] {
	case "seed":
		if len(arguments) != 2 {
			fmt.Fprintln(os.Stderr, "dictionary-v0.2 seed accepts exactly one operator")
			os.Exit(2)
		}
		summary, err := repository.ImportCandidateDictionary(ctx, seed, arguments[1])
		if err != nil {
			slog.Error("candidate dictionary import failed")
			os.Exit(1)
		}
		slog.Info("candidate dictionary import completed", "rules_created", summary.RulesCreated, "candidates_created", summary.CandidatesCreated, "reused", summary.Reused)
	case "approve":
		if len(arguments) != 3 {
			fmt.Fprintln(os.Stderr, "dictionary-v0.2 approve accepts exactly one rule key and one operator")
			os.Exit(2)
		}
		summary, err := repository.ApproveV02ReviewedRule(ctx, arguments[1], arguments[2])
		if err != nil {
			slog.Error("candidate dictionary approval failed")
			os.Exit(1)
		}
		slog.Info("candidate dictionary rule approval completed", "reused", summary.Reused)
	case "approve-reviewed":
		if len(arguments) != 2 {
			fmt.Fprintln(os.Stderr, "dictionary-v0.2 approve-reviewed accepts exactly one operator")
			os.Exit(2)
		}
		summary, err := repository.ApproveAllV02ReviewedRules(ctx, arguments[1])
		if err != nil {
			slog.Error("reviewed candidate dictionary bulk approval failed")
			os.Exit(1)
		}
		slog.Info("reviewed candidate dictionary bulk approval completed", "approved", summary.Approved, "reused", summary.Reused)
	case "prepare-analysis":
		if len(arguments) != 2 {
			fmt.Fprintln(os.Stderr, "dictionary-v0.2 prepare-analysis accepts exactly one operator")
			os.Exit(2)
		}
		summary, err := repository.PrepareCandidateConfigurationForDictionary(ctx, seed.VersionLabel, jdanalysis.V01ParserVersion, arguments[1])
		if err != nil {
			slog.Error("candidate analysis preparation failed")
			os.Exit(1)
		}
		slog.Info("candidate analysis preparation completed", "queued", summary.Queued, "reused", summary.Reused)
	default:
		fmt.Fprintln(os.Stderr, "dictionary-v0.2 operation must be seed, approve, approve-reviewed, or prepare-analysis")
		os.Exit(2)
	}
}

// runV01CandidateAnalysis processes only the fixed, un-published v0.1
// configuration. Normal `analyze` continues to resolve the published pointer
// and therefore cannot accidentally consume this candidate.
func runV01CandidateAnalysis(arguments []string) {
	if len(arguments) != 0 {
		fmt.Fprintln(os.Stderr, "analyze-candidate-v0.1 accepts no positional arguments")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid candidate analysis configuration")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.SyncTimeout)
	defer cancel()
	repository, err := jdanalysis.NewRepository(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("candidate analysis database initialization failed")
		os.Exit(1)
	}
	defer repository.Close()
	configurationID, err := repository.CandidateConfigurationID(ctx, jdanalysis.CandidateV01Seed().VersionLabel, jdanalysis.V01ParserVersion)
	if err != nil {
		slog.Error("candidate analysis configuration is unavailable")
		os.Exit(1)
	}
	summary, err := (jdanalysis.Processor{Repository: repository}).ProcessPending(ctx, configurationID)
	if err != nil {
		slog.Error(jdanalysis.SafeOperationMessage(err), "claimed", summary.Claimed, "completed", summary.Completed, "failed", summary.Failed)
		os.Exit(1)
	}
	slog.Info("candidate analysis completed", "claimed", summary.Claimed, "completed", summary.Completed, "failed", summary.Failed)
}

// runV02CandidateAnalysis processes only the isolated v0.2 candidate
// configuration. It never resolves or changes the published configuration.
func runV02CandidateAnalysis(arguments []string) {
	if len(arguments) != 0 {
		fmt.Fprintln(os.Stderr, "analyze-candidate-v0.2 accepts no positional arguments")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid candidate analysis configuration")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.SyncTimeout)
	defer cancel()
	repository, err := jdanalysis.NewRepository(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("candidate analysis database initialization failed")
		os.Exit(1)
	}
	defer repository.Close()
	configurationID, err := repository.CandidateConfigurationID(ctx, jdanalysis.CandidateV02Seed().VersionLabel, jdanalysis.V01ParserVersion)
	if err != nil {
		slog.Error("candidate analysis configuration is unavailable")
		os.Exit(1)
	}
	summary, err := (jdanalysis.Processor{Repository: repository}).ProcessPending(ctx, configurationID)
	if err != nil {
		slog.Error(jdanalysis.SafeOperationMessage(err), "claimed", summary.Claimed, "completed", summary.Completed, "failed", summary.Failed)
		os.Exit(1)
	}
	slog.Info("candidate analysis completed", "claimed", summary.Claimed, "completed", summary.Completed, "failed", summary.Failed)
}

func isWithin(candidate, root string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !filepath.IsAbs(relative) && relative != "."
}
