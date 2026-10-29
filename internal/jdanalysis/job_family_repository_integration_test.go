package jdanalysis

import (
	"context"
	"testing"
)

func TestRepositoryPublishesAndReadsReusableFormalJobFamily(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	firstMetadataID := insertSyntheticJobFamilyMetadata(t, repository, "00000000-0000-4000-8000-000000000d01", "00000000-0000-4000-8000-000000000d02", "服务端运维", "1", "first")
	secondMetadataID := insertSyntheticJobFamilyMetadataForExistingJob(t, repository, "00000000-0000-4000-8000-000000000d03", "00000000-0000-4000-8000-000000000d01", "服务端运维", "1", "location-only-change")
	parsed, err := ParseJobFamilyTitle("服务端运维")
	if err != nil {
		t.Fatal(err)
	}

	first, err := repository.PublishJobFamilyResult(ctx, firstMetadataID, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if first.Reused || first.ResultID == "" {
		t.Fatalf("first job-family publication = %#v", first)
	}

	second, err := repository.PublishJobFamilyResult(ctx, secondMetadataID, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Reused || second.ResultID != first.ResultID {
		t.Fatalf("same-title job-family result was not reused: first=%#v second=%#v", first, second)
	}

	formal, available, err := repository.ReadFormalJobFamily(ctx, secondMetadataID)
	if err != nil {
		t.Fatal(err)
	}
	if !available || formal.Status != JobFamilyClassified || formal.PrimaryFamilyKey != JobFamilyOperationsSRE {
		t.Fatalf("formal job family = %#v available:%t, want classified operations_sre", formal, available)
	}
	if formal.FamilyMatchCounts[JobFamilyBackend] != 1 || formal.FamilyMatchCounts[JobFamilyOperationsSRE] != 1 {
		t.Fatalf("formal job-family counts = %#v, want backend:1 operations_sre:1", formal.FamilyMatchCounts)
	}
	wantEvidence := []JobFamilyTitleEvidence{
		{FamilyKey: JobFamilyBackend, MatchedAlias: "服务端", CodePointRange: CodePointRange{Start: 0, End: 3}},
		{FamilyKey: JobFamilyOperationsSRE, MatchedAlias: "运维", CodePointRange: CodePointRange{Start: 3, End: 5}},
	}
	if len(formal.Evidence) != len(wantEvidence) {
		t.Fatalf("formal job-family evidence = %#v, want %#v", formal.Evidence, wantEvidence)
	}
	for index, want := range wantEvidence {
		if formal.Evidence[index] != want {
			t.Fatalf("formal job-family evidence[%d] = %#v, want %#v", index, formal.Evidence[index], want)
		}
	}
}

func TestProcessPendingJobFamiliesDoesNotUseSourceProjectMetadata(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	metadataID := insertSyntheticJobFamilyMetadataWithPayload(t, repository, "00000000-0000-4000-8000-000000000d31", "00000000-0000-4000-8000-000000000d32", `{"title":{"state":"string","value":"通用工程师"},"official_category":{"id":{"state":"string","value":"1"}},"source_project":{"name":{"state":"string","value":"后端平台"},"object_name":{"state":"string","value":"服务端"}}}`, "forbidden-source-project-fallback")

	if _, err := ProcessPendingJobFamilies(ctx, repository); err != nil {
		t.Fatal(err)
	}
	formal, available, err := repository.ReadFormalJobFamily(ctx, metadataID)
	if err != nil || !available || formal.Status != JobFamilyUnclassified || formal.PrimaryFamilyKey != "" || len(formal.FamilyMatchCounts) != 0 {
		t.Fatalf("source-project fallback formal result = %#v available:%t err:%v, want unclassified title-only result", formal, available, err)
	}
}

func TestProcessPendingJobFamiliesPersistsExplicitScopeAndTitleQualityStatuses(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	inScopeID := insertSyntheticJobFamilyMetadata(t, repository, "00000000-0000-4000-8000-000000000d11", "00000000-0000-4000-8000-000000000d12", "通用工程师", "1", "in-scope-no-match")
	outOfScopeID := insertSyntheticJobFamilyMetadata(t, repository, "00000000-0000-4000-8000-000000000d13", "00000000-0000-4000-8000-000000000d14", "后端工程师", "2", "out-of-scope")
	invalidTitleID := insertSyntheticJobFamilyMetadata(t, repository, "00000000-0000-4000-8000-000000000d15", "00000000-0000-4000-8000-000000000d16", "", "1", "invalid-title")
	scopeUnknownID := insertSyntheticJobFamilyMetadataWithPayload(t, repository, "00000000-0000-4000-8000-000000000d17", "00000000-0000-4000-8000-000000000d18", `{"title":{"state":"string","value":"后端工程师"}}`, "scope-unknown")

	before, available, err := repository.ReadFormalJobFamily(ctx, inScopeID)
	if err != nil || available || before.Status != "" {
		t.Fatalf("formal result before deterministic observation = %#v available:%t err:%v, want unavailable", before, available, err)
	}

	summary, err := ProcessPendingJobFamilies(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Processed != 1 || summary.Unclassified != 1 || summary.Classified != 0 || summary.OutOfScope != 1 || summary.ScopeUnknown != 1 || summary.InvalidTitle != 1 || summary.RetryableFailures != 0 {
		t.Fatalf("scope and title-quality catch-up summary = %#v, want one in-scope unclassified result", summary)
	}

	tests := []struct {
		name       string
		metadataID string
		status     JobFamilyTitleStatus
	}{
		{name: "in scope no match", metadataID: inScopeID, status: JobFamilyUnclassified},
		{name: "out of scope", metadataID: outOfScopeID, status: JobFamilyOutOfScope},
		{name: "invalid title", metadataID: invalidTitleID, status: JobFamilyInvalidTitle},
		{name: "scope unknown", metadataID: scopeUnknownID, status: JobFamilyScopeUnknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			formal, available, err := repository.ReadFormalJobFamily(ctx, test.metadataID)
			if err != nil || !available || formal.Status != test.status || formal.PrimaryFamilyKey != "" || len(formal.FamilyMatchCounts) != 0 {
				t.Fatalf("formal job-family status = %#v available:%t err:%v, want %q with no primary family", formal, available, err, test.status)
			}
		})
	}

	var completedResultCount int
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM job_family_results`).Scan(&completedResultCount); err != nil {
		t.Fatal(err)
	}
	if completedResultCount != 1 {
		t.Fatalf("completed title results = %d, want exactly one in-scope title result", completedResultCount)
	}
}

func TestJobFamilyHistoryRejectsMutation(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	metadataID := insertSyntheticJobFamilyMetadata(t, repository, "00000000-0000-4000-8000-000000000d21", "00000000-0000-4000-8000-000000000d22", "服务端运维", "1", "immutable-history")
	if _, err := ProcessPendingJobFamilies(ctx, repository); err != nil {
		t.Fatal(err)
	}
	mutations := []string{
		`UPDATE job_family_results SET completed_at = now()`,
		`DELETE FROM job_family_match_evidence`,
		`DELETE FROM job_family_observations WHERE source_metadata_version_id = '` + metadataID + `'`,
	}
	for _, mutation := range mutations {
		if _, err := repository.pool.Exec(ctx, mutation); err == nil {
			t.Fatalf("immutable job-family history accepted mutation: %s", mutation)
		}
	}
}

func insertSyntheticJobFamilyMetadata(t *testing.T, repository *Repository, jobID, metadataID, title, categoryID, salt string) string {
	t.Helper()
	if _, err := repository.pool.Exec(context.Background(), `
INSERT INTO jobs (id, source_company, source_job_id)
VALUES ($1, 'job-family-test', $2)`, jobID, "source-"+salt); err != nil {
		t.Fatal(err)
	}
	return insertSyntheticJobFamilyMetadataForExistingJob(t, repository, metadataID, jobID, title, categoryID, salt)
}

func insertSyntheticJobFamilyMetadataForExistingJob(t *testing.T, repository *Repository, metadataID, jobID, title, categoryID, salt string) string {
	t.Helper()
	payload := `{"title":{"state":"string","value":"` + title + `"},"official_category":{"id":{"state":"string","value":"` + categoryID + `"},"name":{"state":"string","value":"程序&技术类"}},"locations":{"state":"array","items":[]},"source_project":{"id":{"state":"absent"},"name":{"state":"string","value":"社会招聘"}}}`
	return insertSyntheticJobFamilyMetadataPayloadForExistingJob(t, repository, metadataID, jobID, payload, salt)
}

func insertSyntheticJobFamilyMetadataWithPayload(t *testing.T, repository *Repository, jobID, metadataID, payload, salt string) string {
	t.Helper()
	if _, err := repository.pool.Exec(context.Background(), `
INSERT INTO jobs (id, source_company, source_job_id)
VALUES ($1, 'job-family-test', $2)`, jobID, "source-"+salt); err != nil {
		t.Fatal(err)
	}
	return insertSyntheticJobFamilyMetadataPayloadForExistingJob(t, repository, metadataID, jobID, payload, salt)
}

func insertSyntheticJobFamilyMetadataPayloadForExistingJob(t *testing.T, repository *Repository, metadataID, jobID, payload, salt string) string {
	t.Helper()
	if _, err := repository.pool.Exec(context.Background(), `
INSERT INTO job_source_metadata_versions (
    id, job_id, metadata_sha256, normalization_version, metadata_payload
) VALUES ($1, $2, $3, 'source-metadata-v1', $4::jsonb)`, metadataID, jobID, testSHA256("job-family-metadata-"+salt), payload); err != nil {
		t.Fatal(err)
	}
	return metadataID
}
