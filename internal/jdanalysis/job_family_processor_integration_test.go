package jdanalysis

import (
	"context"
	"testing"
)

func TestProcessPendingJobFamiliesCatchesUpAndReusesTitleIdentity(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	firstMetadataID := insertSyntheticJobFamilyMetadata(t, repository, "00000000-0000-4000-8000-000000000e01", "00000000-0000-4000-8000-000000000e02", "服务端运维", "1", "initial")

	first, err := ProcessPendingJobFamilies(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if first.Processed != 1 || first.Classified != 1 || first.Unclassified != 0 || first.RetryableFailures != 0 {
		t.Fatalf("first job-family processing = %#v, want one classified catch-up", first)
	}
	formal, available, err := repository.ReadFormalJobFamily(ctx, firstMetadataID)
	if err != nil || !available || formal.PrimaryFamilyKey != JobFamilyOperationsSRE {
		t.Fatalf("first formal job family = %#v available:%t err:%v", formal, available, err)
	}

	second, err := ProcessPendingJobFamilies(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if second.Processed != 0 || second.RetryableFailures != 0 {
		t.Fatalf("repeat job-family processing = %#v, want no processing", second)
	}

	metadataOnlyID := insertSyntheticJobFamilyMetadataForExistingJob(t, repository, "00000000-0000-4000-8000-000000000e03", "00000000-0000-4000-8000-000000000e01", "服务端运维", "1", "location-change")
	metadataOnly, err := ProcessPendingJobFamilies(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if metadataOnly.Processed != 0 {
		t.Fatalf("metadata-only job-family processing = %#v, want reused title result", metadataOnly)
	}
	reused, available, err := repository.ReadFormalJobFamily(ctx, metadataOnlyID)
	if err != nil || !available || reused.PrimaryFamilyKey != JobFamilyOperationsSRE {
		t.Fatalf("metadata-only formal job family = %#v available:%t err:%v", reused, available, err)
	}

	titleChangedID := insertSyntheticJobFamilyMetadataForExistingJob(t, repository, "00000000-0000-4000-8000-000000000e04", "00000000-0000-4000-8000-000000000e01", "通用岗位", "1", "title-change")
	titleChanged, err := ProcessPendingJobFamilies(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if titleChanged.Processed != 1 || titleChanged.Unclassified != 1 {
		t.Fatalf("title-changed job-family processing = %#v, want one unclassified result", titleChanged)
	}
	changed, available, err := repository.ReadFormalJobFamily(ctx, titleChangedID)
	if err != nil || !available || changed.Status != JobFamilyUnclassified || changed.PrimaryFamilyKey != "" {
		t.Fatalf("title-changed formal job family = %#v available:%t err:%v", changed, available, err)
	}

	scopeChangedID := insertSyntheticJobFamilyMetadataForExistingJob(t, repository, "00000000-0000-4000-8000-000000000e05", "00000000-0000-4000-8000-000000000e01", "通用岗位", "2", "scope-change")
	scopeChanged, err := ProcessPendingJobFamilies(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if scopeChanged.Processed != 0 || scopeChanged.RetryableFailures != 0 {
		t.Fatalf("out-of-scope metadata change = %#v, want observation-only processing", scopeChanged)
	}
	outOfScope, available, err := repository.ReadFormalJobFamily(ctx, scopeChangedID)
	if err != nil || !available || outOfScope.Status != JobFamilyOutOfScope || outOfScope.PrimaryFamilyKey != "" {
		t.Fatalf("out-of-scope changed formal job family = %#v available:%t err:%v", outOfScope, available, err)
	}
}

func TestProcessPendingJobFamiliesAuditsOneStorageFailureAndRetriesIndependently(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	previousMetadataID := insertSyntheticJobFamilyMetadata(t, repository, "00000000-0000-4000-8000-000000000e11", "00000000-0000-4000-8000-000000000e12", "前端工程师", "1", "retry-previous-title")
	failedMetadataID := insertSyntheticJobFamilyMetadataForExistingJob(t, repository, "00000000-0000-4000-8000-000000000e13", "00000000-0000-4000-8000-000000000e11", "后端工程师", "1", "retry-current-title")

	if _, err := repository.pool.Exec(ctx, `
CREATE FUNCTION fail_one_job_family_test_result()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM job_family_title_identities
        WHERE id = NEW.job_family_title_identity_id
          AND normalized_title = '后端工程师'
    ) THEN
        RAISE EXCEPTION 'synthetic job-family storage failure';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER fail_one_job_family_test_result_trigger
BEFORE INSERT ON job_family_results
FOR EACH ROW EXECUTE FUNCTION fail_one_job_family_test_result();`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := repository.pool.Exec(context.Background(), `
DROP TRIGGER IF EXISTS fail_one_job_family_test_result_trigger ON job_family_results;
DROP FUNCTION IF EXISTS fail_one_job_family_test_result();`); err != nil {
			t.Error(err)
		}
	})

	first, err := ProcessPendingJobFamilies(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if first.Processed != 1 || first.Classified != 1 || first.RetryableFailures != 1 {
		t.Fatalf("first independent job-family run = %#v, want one success and one retryable failure", first)
	}
	failed, available, err := repository.ReadFormalJobFamily(ctx, failedMetadataID)
	if err != nil || available || failed.Status != "" {
		t.Fatalf("failed title formal result = %#v available:%t err:%v, want no fallback result", failed, available, err)
	}
	previous, available, err := repository.ReadFormalJobFamily(ctx, previousMetadataID)
	if err != nil || !available || previous.Status != JobFamilyClassified || previous.PrimaryFamilyKey != JobFamilyFrontend {
		t.Fatalf("previous title formal result = %#v available:%t err:%v", previous, available, err)
	}
	var auditCount int
	var safeErrorCode string
	if err := repository.pool.QueryRow(ctx, `
SELECT count(*), coalesce(min(safe_error_code), '')
FROM job_family_retry_audits`).Scan(&auditCount, &safeErrorCode); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 || safeErrorCode != "job_family_processing_failed" {
		t.Fatalf("safe retry audit = count:%d error:%q, want one stable safe error", auditCount, safeErrorCode)
	}

	if _, err := repository.pool.Exec(ctx, `
DROP TRIGGER fail_one_job_family_test_result_trigger ON job_family_results;
DROP FUNCTION fail_one_job_family_test_result();`); err != nil {
		t.Fatal(err)
	}
	second, err := ProcessPendingJobFamilies(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if second.Processed != 1 || second.Classified != 1 || second.RetryableFailures != 0 {
		t.Fatalf("retry after storage recovery = %#v, want previously failed title to publish", second)
	}
	retried, available, err := repository.ReadFormalJobFamily(ctx, failedMetadataID)
	if err != nil || !available || retried.PrimaryFamilyKey != JobFamilyBackend {
		t.Fatalf("retried title formal result = %#v available:%t err:%v", retried, available, err)
	}
}

func TestProcessPendingJobFamiliesRetainsObservationFailureAndContinues(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	failedMetadataID := insertSyntheticJobFamilyMetadata(t, repository, "00000000-0000-4000-8000-000000000e51", "00000000-0000-4000-8000-000000000e52", "后端工程师", "1", "observation-retry-failure")
	successfulMetadataID := insertSyntheticJobFamilyMetadata(t, repository, "00000000-0000-4000-8000-000000000e53", "00000000-0000-4000-8000-000000000e54", "前端工程师", "1", "observation-retry-success")

	if _, err := repository.pool.Exec(ctx, `
CREATE FUNCTION fail_one_job_family_observation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.source_metadata_version_id = '00000000-0000-4000-8000-000000000e52'::uuid THEN
        RAISE EXCEPTION 'synthetic job-family observation storage failure';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER fail_one_job_family_observation_trigger
BEFORE INSERT ON job_family_observations
FOR EACH ROW EXECUTE FUNCTION fail_one_job_family_observation();`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := repository.pool.Exec(context.Background(), `
DROP TRIGGER IF EXISTS fail_one_job_family_observation_trigger ON job_family_observations;
DROP FUNCTION IF EXISTS fail_one_job_family_observation();`); err != nil {
			t.Error(err)
		}
	})

	first, err := ProcessPendingJobFamilies(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if first.Processed != 1 || first.Classified != 1 || first.RetryableFailures != 1 {
		t.Fatalf("job-family processing with one observation failure = %#v, want one success and one retryable failure", first)
	}
	failed, available, err := repository.ReadFormalJobFamily(ctx, failedMetadataID)
	if err != nil || available || failed.Status != "" {
		t.Fatalf("failed observation formal result = %#v available:%t err:%v, want no fallback result", failed, available, err)
	}
	successful, available, err := repository.ReadFormalJobFamily(ctx, successfulMetadataID)
	if err != nil || !available || successful.PrimaryFamilyKey != JobFamilyFrontend {
		t.Fatalf("successful observation formal result = %#v available:%t err:%v", successful, available, err)
	}
	var auditCount int
	var safeErrorCode string
	if err := repository.pool.QueryRow(ctx, `
SELECT count(*), coalesce(min(safe_error_code), '')
FROM job_family_observation_retry_audits`).Scan(&auditCount, &safeErrorCode); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 || safeErrorCode != "job_family_observation_failed" {
		t.Fatalf("safe observation retry audit = count:%d error:%q, want one stable safe error", auditCount, safeErrorCode)
	}
	if _, err := repository.pool.Exec(ctx, `DELETE FROM job_family_observation_retry_audits`); err == nil {
		t.Fatal("immutable observation retry audit accepted deletion")
	}

	if _, err := repository.pool.Exec(ctx, `
DROP TRIGGER fail_one_job_family_observation_trigger ON job_family_observations;
DROP FUNCTION fail_one_job_family_observation();`); err != nil {
		t.Fatal(err)
	}
	second, err := ProcessPendingJobFamilies(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if second.Processed != 1 || second.Classified != 1 || second.RetryableFailures != 0 {
		t.Fatalf("retry after observation storage recovery = %#v, want previously unobserved title to publish", second)
	}
	retried, available, err := repository.ReadFormalJobFamily(ctx, failedMetadataID)
	if err != nil || !available || retried.PrimaryFamilyKey != JobFamilyBackend {
		t.Fatalf("retried observation formal result = %#v available:%t err:%v", retried, available, err)
	}
}

func TestProcessPendingJobFamiliesCatchesUpWhenEitherDictionaryIdentityChanges(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	metadataID := insertSyntheticJobFamilyMetadata(t, repository, "00000000-0000-4000-8000-000000000e21", "00000000-0000-4000-8000-000000000e22", "后端工程师", "1", "identity-catch-up")
	parsed, err := ParseJobFamilyTitle("后端工程师")
	if err != nil {
		t.Fatal(err)
	}
	previous := CurrentJobFamilyIdentity(metadataID)
	previous.DictionaryVersion = "deterministic-job-family-dictionary-test-v0"
	previous.DictionarySHA256 = testSHA256("job-family-dictionary-test-v0")
	previous.ProjectMaskDictionaryVersion = "deterministic-project-dictionary-test-v0"
	previous.ProjectMaskDictionarySHA256 = testSHA256("project-mask-dictionary-test-v0")
	first, err := repository.publishJobFamilyResult(ctx, previous, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if first.Reused {
		t.Fatalf("prior identity result unexpectedly reused: %#v", first)
	}

	catchUp, err := ProcessPendingJobFamilies(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if catchUp.Processed != 1 || catchUp.Classified != 1 || catchUp.RetryableFailures != 0 {
		t.Fatalf("current identity catch-up = %#v, want one new result", catchUp)
	}
	var resultCount int
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM job_family_results`).Scan(&resultCount); err != nil {
		t.Fatal(err)
	}
	if resultCount != 2 {
		t.Fatalf("immutable results after dictionary-identity catch-up = %d, want 2", resultCount)
	}
	formal, available, err := repository.ReadFormalJobFamily(ctx, metadataID)
	if err != nil || !available || formal.PrimaryFamilyKey != JobFamilyBackend {
		t.Fatalf("current formal result after identity catch-up = %#v available:%t err:%v", formal, available, err)
	}
}

func TestProcessPendingJobFamiliesContinuesWhenRetryAuditStorageFails(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	failedMetadataID := insertSyntheticJobFamilyMetadata(t, repository, "00000000-0000-4000-8000-000000000e31", "00000000-0000-4000-8000-000000000e32", "后端工程师", "1", "audit-storage-failure")
	successfulMetadataID := insertSyntheticJobFamilyMetadata(t, repository, "00000000-0000-4000-8000-000000000e33", "00000000-0000-4000-8000-000000000e34", "前端工程师", "1", "audit-storage-success")
	const (
		failedTitleIdentityID     = "00000000-0000-4000-8000-000000000e41"
		successfulTitleIdentityID = "00000000-0000-4000-8000-000000000e42"
	)
	insertSyntheticJobFamilyObservation(t, repository, failedMetadataID, failedTitleIdentityID, "后端工程师")
	insertSyntheticJobFamilyObservation(t, repository, successfulMetadataID, successfulTitleIdentityID, "前端工程师")

	if _, err := repository.pool.Exec(ctx, `
CREATE FUNCTION fail_one_job_family_result_with_unavailable_audit()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.job_family_title_identity_id = '00000000-0000-4000-8000-000000000e41'::uuid THEN
        RAISE EXCEPTION 'synthetic job-family result failure';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER fail_one_job_family_result_with_unavailable_audit_trigger
BEFORE INSERT ON job_family_results
FOR EACH ROW EXECUTE FUNCTION fail_one_job_family_result_with_unavailable_audit();

CREATE FUNCTION fail_job_family_retry_audit()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'synthetic job-family retry-audit storage failure';
END;
$$;
CREATE TRIGGER fail_job_family_retry_audit_trigger
BEFORE INSERT ON job_family_retry_audits
FOR EACH ROW EXECUTE FUNCTION fail_job_family_retry_audit();`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := repository.pool.Exec(context.Background(), `
DROP TRIGGER IF EXISTS fail_job_family_retry_audit_trigger ON job_family_retry_audits;
DROP FUNCTION IF EXISTS fail_job_family_retry_audit();
DROP TRIGGER IF EXISTS fail_one_job_family_result_with_unavailable_audit_trigger ON job_family_results;
DROP FUNCTION IF EXISTS fail_one_job_family_result_with_unavailable_audit();`); err != nil {
			t.Error(err)
		}
	})

	summary, err := ProcessPendingJobFamilies(ctx, repository)
	if err == nil {
		t.Fatal("retry-audit storage outage was not surfaced")
	}
	if summary.Processed != 1 || summary.Classified != 1 || summary.RetryableFailures != 1 {
		t.Fatalf("job-family processing during retry-audit outage = %#v, want one success and one independently retained failure", summary)
	}
	successful, available, readErr := repository.ReadFormalJobFamily(ctx, successfulMetadataID)
	if readErr != nil || !available || successful.PrimaryFamilyKey != JobFamilyFrontend {
		t.Fatalf("successful title after retry-audit outage = %#v available:%t err:%v", successful, available, readErr)
	}
	failed, available, readErr := repository.ReadFormalJobFamily(ctx, failedMetadataID)
	if readErr != nil || available || failed.Status != "" {
		t.Fatalf("failed title after retry-audit outage = %#v available:%t err:%v, want pending without fallback", failed, available, readErr)
	}
}

func insertSyntheticJobFamilyObservation(t *testing.T, repository *Repository, metadataID, titleIdentityID, title string) {
	t.Helper()
	ctx := context.Background()
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO job_family_title_identities (id, title_sha256, normalization_version, normalized_title)
VALUES ($1, $2, 'source-metadata-v1', $3)`, titleIdentityID, testSHA256(title), title); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO job_family_observations (source_metadata_version_id, job_family_title_identity_id, status)
VALUES ($1, $2, 'in_scope')`, metadataID, titleIdentityID); err != nil {
		t.Fatal(err)
	}
}
