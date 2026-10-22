package jobsync

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"mrolescope/internal/collector"
)

var ErrScopeBusy = errors.New("another synchronization is already active for this observation scope")

// RunFailure deliberately exposes only stable operational information. Its
// underlying cause can originate in an external response or SDK and must not
// flow into command logs or the audit database.
type RunFailure struct {
	Stage string
	Code  string
}

func (failure RunFailure) Error() string {
	return fmt.Sprintf("synchronization %s failed (%s)", failure.Stage, failure.Code)
}

type CollectorRunner interface {
	Run(context.Context, string, string, string) (collector.Manifest, error)
}

type BatchArchiver interface {
	Archive(context.Context, string, Scope, string, collector.Manifest) (ArchiveResult, error)
}

// Service coordinates the work that is intentionally outside the database
// transaction (collection and COS archive) and then asks Repository to make
// the history publication atomic.
type Service struct {
	Repository *Repository
	Collector  CollectorRunner
	Archiver   BatchArchiver
	Rechecker  DetailRechecker
}

type RunOptions struct {
	RunID         string
	OutputDir     string
	FixturePath   string
	Trigger       TriggerKind
	AttemptNumber int
}

func (service Service) Run(ctx context.Context, options RunOptions) error {
	if service.Repository == nil || service.Collector == nil || service.Archiver == nil {
		return fmt.Errorf("synchronization service is incomplete")
	}
	if options.RunID == "" {
		options.RunID = newUUID()
	}
	if options.OutputDir == "" || filepath.Clean(options.OutputDir) == "." {
		return fmt.Errorf("a dedicated collector output directory is required")
	}
	if !options.Trigger.Valid() {
		return fmt.Errorf("a valid synchronization trigger is required")
	}
	if options.AttemptNumber <= 0 {
		return fmt.Errorf("attempt number must be positive")
	}

	scope := MihoyoTechnicalScope()
	storedScope, err := service.Repository.EnsureScope(ctx, scope)
	if err != nil {
		return err
	}
	release, acquired, err := service.Repository.TryAcquireScopeLock(ctx, storedScope.ID)
	if err != nil {
		return err
	}
	if !acquired {
		return ErrScopeBusy
	}
	defer release()

	startedAt := time.Now().UTC()
	run, err := service.Repository.CreateRun(ctx, storedScope, options.RunID, options.Trigger, options.AttemptNumber, startedAt)
	if err != nil {
		return err
	}
	stage := "collecting"
	fail := func(cause error) error {
		code, _ := safeFailure(stage)
		if markErr := service.Repository.FailRun(context.Background(), run, stage, cause); markErr != nil {
			return RunFailure{Stage: stage, Code: "failure_recording_failed"}
		}
		return RunFailure{Stage: stage, Code: code}
	}
	if err := service.Repository.AppendAudit(ctx, run, AuditEvent{
		Stage: stage, Severity: "info", Code: "collection_started", Message: "collection attempt started",
	}); err != nil {
		return fail(err)
	}

	manifest, err := service.Collector.Run(ctx, options.RunID, options.OutputDir, options.FixturePath)
	if err != nil {
		return fail(fmt.Errorf("collect batch: %w", err))
	}
	stage = "validating"
	if err := service.Repository.SetRunStage(ctx, run.ID, stage); err != nil {
		return fail(err)
	}
	if manifest.RunID != options.RunID || manifest.Source.Company != scope.SourceCompany || manifest.Source.FilterSHA256 != scope.FilterSHA256 {
		return fail(fmt.Errorf("collector manifest does not match the fixed observation scope"))
	}
	observations, err := ParseObservations(options.OutputDir, manifest)
	if err != nil {
		return fail(fmt.Errorf("parse validated batch: %w", err))
	}
	if err := service.Repository.AppendAudit(ctx, run, AuditEvent{
		Stage: stage, Severity: "info", Code: "batch_validated", Message: "collector manifest and observations validated",
		Diagnostics: map[string]any{"observations": len(observations)},
	}); err != nil {
		return fail(err)
	}

	rechecks, err := service.recheckMissing(ctx, storedScope, observations, options.FixturePath != "", run)
	if err != nil {
		return fail(err)
	}

	stage = "archiving"
	if err := service.Repository.SetRunStage(ctx, run.ID, stage); err != nil {
		return fail(err)
	}
	if err := service.Repository.AppendAudit(ctx, run, AuditEvent{
		Stage: stage, Severity: "info", Code: "archive_started", Message: "uploading validated source evidence",
	}); err != nil {
		return fail(err)
	}
	archive, err := service.Archiver.Archive(ctx, options.OutputDir, scope, asiaShanghaiDate(startedAt).Format("2006-01-02"), manifest)
	if err != nil {
		return fail(fmt.Errorf("archive batch: %w", err))
	}

	stage = "persisting"
	if err := service.Repository.SetRunStage(ctx, run.ID, stage); err != nil {
		return fail(err)
	}
	if err := service.Repository.AppendAudit(ctx, run, AuditEvent{
		Stage: stage, Severity: "info", Code: "publication_started", Message: "publishing archived batch transaction",
	}); err != nil {
		return fail(err)
	}
	if err := service.Repository.Publish(ctx, PublishInput{
		Run: run, Scope: storedScope, Manifest: manifest, Observations: observations, Rechecks: rechecks, Archive: archive,
	}); err != nil {
		return fail(fmt.Errorf("publish archived batch: %w", err))
	}
	return nil
}

func (service Service) recheckMissing(ctx context.Context, scope StoredScope, observations []Observation, fixtureMode bool, run Run) (map[string]RecheckResult, error) {
	openPeriods, err := service.Repository.OpenPresence(ctx, scope.ID)
	if err != nil {
		return nil, err
	}
	observed := make(map[string]struct{}, len(observations))
	for _, observation := range observations {
		observed[observation.SourceJobID] = struct{}{}
	}
	checker := service.Rechecker
	if checker == nil || fixtureMode {
		checker = IndeterminateRechecker{}
	}
	results := make(map[string]RecheckResult)
	for _, period := range openPeriods {
		if _, currentlyListed := observed[period.SourceJobID]; currentlyListed {
			continue
		}
		// This loop is intentionally sequential: a list omission cannot turn
		// into an uncontrolled parallel burst against the upstream source.
		result := checker.Recheck(ctx, period.SourceJobID)
		if result.Outcome != RecheckAvailable && result.Outcome != RecheckDefinitelyUnavailable && result.Outcome != RecheckIndeterminate {
			result = RecheckResult{Outcome: RecheckIndeterminate, Code: "recheck_invalid_outcome"}
		}
		results[period.SourceJobID] = result
		if err := service.Repository.AppendAudit(ctx, run, AuditEvent{
			Stage: "missing-recheck", Severity: "info", Code: result.Code, Message: "missing job detail rechecked",
			Diagnostics: map[string]any{"source_job_id": period.SourceJobID, "outcome": result.Outcome},
		}); err != nil {
			return nil, err
		}
	}
	return results, nil
}
