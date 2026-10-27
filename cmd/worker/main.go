package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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
		case "analyze-ai":
			runAIAnalysis(arguments[1:])
			return
		case "review-ai":
			runAIReview(arguments[1:])
			return
		case "audit-ai":
			runAIOutputAudit(arguments[1:])
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
		case "release-v0.2":
			runV02ReleaseLifecycle(arguments[1:])
			return
		}
	}
	// Preserve the reviewed synchronization helper's existing flag-only
	// invocation while making new analysis execution explicit.
	runSync(arguments)
}

type aiAnalysisOperation string

const aiOutputAuditRoot = "/var/lib/mrolescope/ai-output-audit"

const (
	aiAnalysisOperationExpandPlus                   aiAnalysisOperation = "expand-plus"
	aiAnalysisOperationPrepareOutputAuditDiagnostic aiAnalysisOperation = "prepare-output-audit-diagnostic"
	aiAnalysisOperationPrepareTrial                 aiAnalysisOperation = "prepare-trial"
	aiAnalysisOperationPublishPlus                  aiAnalysisOperation = "publish-plus"
	aiAnalysisOperationRunPlusOutputAuditDiagnostic aiAnalysisOperation = "run-plus-output-audit-diagnostic"
	aiAnalysisOperationRunPlusTrial                 aiAnalysisOperation = "run-plus-trial"
	aiAnalysisOperationStatus                       aiAnalysisOperation = "status"
)

// parseAIAnalysisOperation deliberately provides no escape hatch for a model,
// prompt, SQL statement, path, price, body ID, payload, or credential. The
// root-owned helper/Compose service selects one of these exact verbs.
func parseAIAnalysisOperation(arguments []string) (aiAnalysisOperation, error) {
	if len(arguments) != 1 {
		return "", fmt.Errorf("analyze-ai accepts exactly one fixed operation")
	}
	switch aiAnalysisOperation(arguments[0]) {
	case aiAnalysisOperationPrepareOutputAuditDiagnostic,
		aiAnalysisOperationPrepareTrial,
		aiAnalysisOperationRunPlusOutputAuditDiagnostic,
		aiAnalysisOperationRunPlusTrial,
		aiAnalysisOperationExpandPlus,
		aiAnalysisOperationPublishPlus,
		aiAnalysisOperationStatus:
		return aiAnalysisOperation(arguments[0]), nil
	default:
		return "", fmt.Errorf("analyze-ai operation is not allowlisted")
	}
}

func runAIAnalysis(arguments []string) {
	operation, err := parseAIAnalysisOperation(arguments)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
	}
	// Reject a missing provider credential before configuration or database
	// initialization. A trial cannot make progress without it, and this keeps
	// the failure local without creating a path that could construct a client.
	if aiAnalysisOperationRequiresCredential(operation) && strings.TrimSpace(os.Getenv("MROLESCOPE_QWEN_API_KEY")) == "" {
		slog.Error("AI model credential is not configured")
		os.Exit(1)
	}
	workspaceID := os.Getenv("MROLESCOPE_QWEN_WORKSPACE_ID")
	if _, err := jdanalysis.QwenBeijingWorkspaceCompatibleBaseURL(workspaceID); err != nil {
		slog.Error("AI model workspace is not configured")
		os.Exit(1)
	}
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid AI analysis configuration")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.SyncTimeout)
	defer cancel()
	repository, err := jdanalysis.NewRepository(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("AI analysis database initialization failed")
		os.Exit(1)
	}
	defer repository.Close()

	switch operation {
	case aiAnalysisOperationPrepareOutputAuditDiagnostic:
		prepared, err := repository.PrepareControlledAIOutputAuditDiagnostic(ctx, workspaceID)
		if err != nil {
			slog.Error(jdanalysis.SafeOperationMessage(err))
			os.Exit(1)
		}
		slog.Info("controlled AI output-audit diagnostic prepared",
			"plus_configuration_id", prepared.Plus.ID,
			"plus_queued", prepared.PlusQueued,
			"reused", prepared.Reused,
		)
	case aiAnalysisOperationPrepareTrial:
		prepared, err := repository.PrepareControlledAITrial(ctx, workspaceID)
		if err != nil {
			slog.Error(jdanalysis.SafeOperationMessage(err))
			os.Exit(1)
		}
		slog.Info("controlled AI trial prepared",
			"plus_configuration_id", prepared.Plus.ID,
			"budget_group_id", prepared.BudgetGroupID,
			"plus_queued", prepared.PlusQueued,
			"reused", prepared.Reused,
		)
	case aiAnalysisOperationRunPlusTrial:
		runControlledAIPlusTrial(ctx, repository, workspaceID)
	case aiAnalysisOperationRunPlusOutputAuditDiagnostic:
		runControlledAIOutputAuditDiagnostic(ctx, repository, workspaceID)
	case aiAnalysisOperationExpandPlus:
		configurationID, err := repository.ControlledAIPlusTrialConfigurationID(ctx, workspaceID)
		if err != nil {
			slog.Error(jdanalysis.SafeOperationMessage(err))
			os.Exit(1)
		}
		summary, err := repository.ExpandAIPlusCandidate(ctx, configurationID)
		if err != nil {
			slog.Error(jdanalysis.SafeOperationMessage(err), "configuration_id", configurationID)
			os.Exit(1)
		}
		slog.Info("controlled AI Plus expansion completed",
			"configuration_id", summary.AIConfigurationID,
			"existing_expansion_work", summary.ExistingExpansionWorkCount,
			"queued", summary.Queued,
		)
	case aiAnalysisOperationPublishPlus:
		configurationID, err := repository.ControlledAIPlusTrialConfigurationID(ctx, workspaceID)
		if err != nil {
			slog.Error(jdanalysis.SafeOperationMessage(err))
			os.Exit(1)
		}
		summary, err := repository.PublishAIConfiguration(ctx, configurationID)
		if err != nil {
			slog.Error(jdanalysis.SafeOperationMessage(err), "configuration_id", configurationID)
			os.Exit(1)
		}
		slog.Info("controlled AI Plus configuration published",
			"configuration_id", summary.AIConfigurationID,
			"trial_body_count", summary.TrialBodyCount,
			"total_work_count", summary.TotalWorkCount,
			"terminal_work_count", summary.TerminalWorkCount,
			"structural_success_count", summary.StructuralSuccessCount,
			"safe_failure_count", summary.SafeFailureCount,
			"attempt_count", summary.AttemptCount,
			"input_token_count", summary.InputTokenCount,
			"output_token_count", summary.OutputTokenCount,
			"charge_nano_rmb", summary.ChargeNanoRMB,
			"publisher", summary.Publisher,
		)
	case aiAnalysisOperationStatus:
		runControlledAITrialStatus(ctx, repository, workspaceID)
	}
}

// parseAIReviewModel exposes only the current Plus trial to the direct CVM
// review session. It deliberately does not accept a configuration ID, artifact
// ID, reviewer, SQL statement, body ID, model identifier, or free-form input.
func parseAIReviewModel(arguments []string) (string, error) {
	if len(arguments) != 1 {
		return "", fmt.Errorf("review-ai accepts exactly one fixed trial arm")
	}
	switch arguments[0] {
	case "plus":
		return "qwen-plus", nil
	default:
		return "", fmt.Errorf("review-ai trial arm is not allowlisted")
	}
}

// runAIReview is intentionally separate from runAIAnalysis: it writes JD
// text only to the directly attached CVM terminal and never through slog,
// systemd, GitHub Actions, or a status operation.
func runAIReview(arguments []string) {
	model, err := parseAIReviewModel(arguments)
	if err != nil {
		fmt.Fprintln(os.Stderr, "AI review operation is not allowlisted")
		os.Exit(2)
	}
	workspaceID := os.Getenv("MROLESCOPE_QWEN_WORKSPACE_ID")
	if _, err := jdanalysis.QwenBeijingWorkspaceCompatibleBaseURL(workspaceID); err != nil {
		fmt.Fprintln(os.Stderr, "AI review workspace is not configured")
		os.Exit(1)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "AI review configuration is unavailable")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.SyncTimeout)
	defer cancel()
	repository, err := jdanalysis.NewRepository(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "AI review database is unavailable")
		os.Exit(1)
	}
	defer repository.Close()
	if model != "qwen-plus" {
		fmt.Fprintln(os.Stderr, "AI review operation is not allowlisted")
		os.Exit(2)
	}
	configurationID, err := repository.ControlledAIPlusTrialConfigurationID(ctx, workspaceID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "AI review candidate is unavailable")
		os.Exit(1)
	}
	if _, err := jdanalysis.RunControlledAIReviewSession(ctx, repository, configurationID, model, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "AI review was not recorded")
		os.Exit(1)
	}
}

func aiAnalysisOperationRequiresCredential(operation aiAnalysisOperation) bool {
	return operation == aiAnalysisOperationRunPlusTrial || operation == aiAnalysisOperationRunPlusOutputAuditDiagnostic
}

func runControlledAIPlusTrial(ctx context.Context, repository *jdanalysis.Repository, workspaceID string) {
	configurationID, err := repository.ControlledAIPlusTrialConfigurationID(ctx, workspaceID)
	if err != nil {
		slog.Error(jdanalysis.SafeOperationMessage(err))
		os.Exit(1)
	}
	runControlledAIConfiguration(ctx, repository, configurationID, workspaceID, "controlled AI Plus trial")
}

func runControlledAIOutputAuditDiagnostic(ctx context.Context, repository *jdanalysis.Repository, workspaceID string) {
	configurationID, err := repository.ControlledAIOutputAuditDiagnosticConfigurationID(ctx, workspaceID)
	if err != nil {
		slog.Error(jdanalysis.SafeOperationMessage(err))
		os.Exit(1)
	}
	runControlledAIConfiguration(ctx, repository, configurationID, workspaceID, "controlled AI output-audit diagnostic")
}

func runControlledAIConfiguration(ctx context.Context, repository *jdanalysis.Repository, configurationID, workspaceID, operation string) {
	apiKey := strings.TrimSpace(os.Getenv("MROLESCOPE_QWEN_API_KEY"))
	if apiKey == "" {
		slog.Error("AI model credential is not configured")
		os.Exit(1)
	}
	httpClient := &http.Client{Timeout: 90 * time.Second}
	client, err := jdanalysis.NewQwenPlusClient(apiKey, httpClient, workspaceID)
	if err != nil {
		slog.Error(jdanalysis.SafeOperationMessage(err))
		os.Exit(1)
	}
	summary, err := (jdanalysis.AISemanticProcessor{
		Repository:                   repository,
		RequirePersistentTrialBudget: true,
		OutputAudit:                  jdanalysis.NewAIOutputAuditStore(aiOutputAuditRoot),
		RequireOutputAudit:           true,
		Runner: jdanalysis.AIModelAttemptRunner{
			Client:    client,
			Estimator: jdanalysis.ConservativeAIInputTokenEstimator{},
		},
	}).ProcessPending(ctx, configurationID)
	if err != nil {
		slog.Error(jdanalysis.SafeOperationMessage(err), "operation", operation, "configuration_id", configurationID, "run_id", summary.RunID, "claimed", summary.Claimed, "completed", summary.Completed, "failed", summary.Failed)
		os.Exit(1)
	}
	slog.Info(operation+" completed", "model", "qwen-plus", "configuration_id", configurationID, "run_id", summary.RunID, "claimed", summary.Claimed, "completed", summary.Completed, "failed", summary.Failed)
}

// runAIOutputAudit is intentionally separate from analysis and semantic
// review. It can display raw model output only to a directly attached CVM TTY.
func runAIOutputAudit(arguments []string) {
	if len(arguments) == 1 && arguments[0] == "status" {
		runAIOutputAuditStatus()
		return
	}
	model, err := parseAIOutputAuditModel(arguments)
	if err != nil {
		fmt.Fprintln(os.Stderr, "AI output audit operation is not allowlisted")
		os.Exit(2)
	}
	if !isDirectTerminal(os.Stdin) || !isDirectTerminal(os.Stdout) {
		fmt.Fprintln(os.Stderr, "AI output audit requires a direct terminal")
		os.Exit(2)
	}
	workspaceID := os.Getenv("MROLESCOPE_QWEN_WORKSPACE_ID")
	if _, err := jdanalysis.QwenBeijingWorkspaceCompatibleBaseURL(workspaceID); err != nil {
		fmt.Fprintln(os.Stderr, "AI output audit workspace is not configured")
		os.Exit(1)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "AI output audit configuration is unavailable")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.SyncTimeout)
	defer cancel()
	repository, err := jdanalysis.NewRepository(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "AI output audit database is unavailable")
		os.Exit(1)
	}
	defer repository.Close()
	if model != "qwen-plus" {
		fmt.Fprintln(os.Stderr, "AI output audit operation is not allowlisted")
		os.Exit(2)
	}
	configurationIDs, err := repository.ControlledAIOutputAuditConfigurationIDs(ctx, workspaceID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "AI output audit candidate is unavailable")
		os.Exit(1)
	}
	if _, err := jdanalysis.RunControlledAIOutputAuditSession(ctx, repository, jdanalysis.NewAIOutputAuditStore(aiOutputAuditRoot), configurationIDs, model, true, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "AI output audit was not recorded")
		os.Exit(1)
	}
}

// runAIOutputAuditStatus is the non-interactive, text-free status seam for
// CVM artifact capacity and pending contract-audit counts. Unlike the review
// session, it never opens a response and therefore does not require a TTY.
func runAIOutputAuditStatus() {
	workspaceID := os.Getenv("MROLESCOPE_QWEN_WORKSPACE_ID")
	if _, err := jdanalysis.QwenBeijingWorkspaceCompatibleBaseURL(workspaceID); err != nil {
		slog.Error("AI output audit workspace is not configured")
		os.Exit(1)
	}
	cfg, err := config.Load()
	if err != nil {
		slog.Error("AI output audit configuration is unavailable")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.SyncTimeout)
	defer cancel()
	repository, err := jdanalysis.NewRepository(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("AI output audit database is unavailable")
		os.Exit(1)
	}
	defer repository.Close()
	plusConfigurationIDs, err := repository.ControlledAIOutputAuditConfigurationIDs(ctx, workspaceID)
	if err != nil {
		slog.Error("AI output audit candidate is unavailable")
		os.Exit(1)
	}
	store := jdanalysis.NewAIOutputAuditStore(aiOutputAuditRoot)
	storage, err := store.Status()
	if err != nil {
		slog.Error("AI output audit storage is unavailable")
		os.Exit(1)
	}
	pending, err := repository.ReadAIOutputAuditPendingStatus(ctx, plusConfigurationIDs, store)
	if err != nil {
		slog.Error("AI output audit pending status is unavailable")
		os.Exit(1)
	}
	attributes := []any{
		"captured_artifacts", storage.CapturedArtifactCount,
		"pending_contract_audits", pending.PendingReviewCount,
		"storage_capacity_bytes", storage.CapacityBytes,
		"storage_used_bytes", storage.UsedBytes,
		"storage_available_bytes", storage.AvailableBytes,
	}
	for _, failureCount := range pending.FailureCounts {
		attributes = append(attributes, "pending_"+failureCount.SafeErrorCode, failureCount.Count)
	}
	slog.Info("AI output audit status", attributes...)
}

func parseAIOutputAuditModel(arguments []string) (string, error) {
	return parseAIReviewModel(arguments)
}

func isDirectTerminal(file *os.File) bool {
	if file == nil {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func runControlledAITrialStatus(ctx context.Context, repository *jdanalysis.Repository, workspaceID string) {
	plusID, err := repository.ControlledAIPlusTrialConfigurationID(ctx, workspaceID)
	if err != nil {
		slog.Error(jdanalysis.SafeOperationMessage(err))
		os.Exit(1)
	}
	report, err := repository.ReadAIPlusTrialReport(ctx, plusID)
	if err != nil {
		slog.Error(jdanalysis.SafeOperationMessage(err))
		os.Exit(1)
	}
	slog.Info("controlled AI trial status",
		"plus_configuration_id", report.AIConfigurationID,
		"plus_logical_tasks", report.LogicalTaskCount,
		"plus_pending", report.PendingTaskCount,
		"plus_complete", report.CompleteTaskCount,
		"plus_input_tokens", report.InputTokenCount,
		"plus_output_tokens", report.OutputTokenCount,
	)
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

// runV02ReleaseLifecycle is deliberately a two-action worker command. Its
// Compose services hard-code the action, and the publisher arrives only via
// the root-owned adapter's authenticated-actor environment variable. It
// cannot receive a configuration ID, dictionary, rule, SQL statement, path,
// image, raw JD text, or free-form withdrawal reason.
func runV02ReleaseLifecycle(arguments []string) {
	if len(arguments) != 1 {
		fmt.Fprintln(os.Stderr, "release-v0.2 accepts publish, withdraw-quality, or withdraw-security")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid release lifecycle configuration")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.SyncTimeout)
	defer cancel()
	repository, err := jdanalysis.NewRepository(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("release lifecycle database initialization failed")
		os.Exit(1)
	}
	defer repository.Close()
	publisher := os.Getenv("MROLESCOPE_RELEASE_PUBLISHER")

	switch arguments[0] {
	case "publish":
		summary, err := repository.ReleaseV02Baseline(ctx, publisher)
		if err != nil {
			slog.Error(jdanalysis.SafeOperationMessage(err))
			os.Exit(1)
		}
		slog.Info("v0.2 analysis baseline published",
			"configuration_id", summary.ConfigurationID,
			"dictionary_version_id", summary.DictionaryVersionID,
			"rule_set_sha256", summary.RuleSetSHA256,
			"release_gate_version", summary.ReleaseGateVersion,
			"publisher", summary.Publisher,
			"queued", summary.Queued,
			"pending", summary.Pending,
			"missing_results", summary.MissingResults,
			"evidence_total", summary.EvidenceTotal,
			"invalid_evidence", summary.InvalidEvidence,
			"reused", summary.Reused,
		)
	case "withdraw-quality":
		runV02BaselineWithdrawal(ctx, repository, publisher, jdanalysis.WithdrawalMaterialResultQualityDefect)
	case "withdraw-security":
		runV02BaselineWithdrawal(ctx, repository, publisher, jdanalysis.WithdrawalSecurityPrivacyIncident)
	default:
		fmt.Fprintln(os.Stderr, "release-v0.2 action must be publish, withdraw-quality, or withdraw-security")
		os.Exit(2)
	}
}

func runV02BaselineWithdrawal(ctx context.Context, repository *jdanalysis.Repository, publisher string, category jdanalysis.WithdrawalCategory) {
	summary, err := repository.WithdrawV02Baseline(ctx, publisher, category)
	if err != nil {
		slog.Error(jdanalysis.SafeOperationMessage(err))
		os.Exit(1)
	}
	slog.Warn("v0.2 analysis baseline withdrawn",
		"configuration_id", summary.ConfigurationID,
		"dictionary_version_id", summary.DictionaryVersionID,
		"rule_set_sha256", summary.RuleSetSHA256,
		"release_gate_version", summary.ReleaseGateVersion,
		"publisher", summary.Publisher,
		"withdrawal_category", summary.Category,
		"reused", summary.Reused,
	)
}

func isWithin(candidate, root string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !filepath.IsAbs(relative) && relative != "."
}
