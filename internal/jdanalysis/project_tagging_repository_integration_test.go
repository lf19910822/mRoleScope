package jdanalysis

import (
	"context"
	"testing"
)

func TestRepositoryPublishesImmutableProjectTagsAndReusesSameTitleAcrossMetadataVersions(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	firstMetadataID := insertSyntheticProjectMetadata(t, repository, "00000000-0000-4000-8000-000000000a01", "00000000-0000-4000-8000-000000000a02", "绝区零-客户端", "first")
	secondMetadataID := insertSyntheticProjectMetadataForExistingJob(t, repository, "00000000-0000-4000-8000-000000000a03", "00000000-0000-4000-8000-000000000a01", "绝区零-客户端", "location-only-change")
	identity := ProjectTagIdentity{
		SourceMetadataVersionID: firstMetadataID,
		ParserVersion:           ProjectTagParserVersion(),
		DictionaryVersion:       ProjectTagDictionaryVersion(),
		DictionarySHA256:        ProjectTagDictionaryDigest(),
	}
	parsed, err := ParseProjectTitle("绝区零-客户端")
	if err != nil {
		t.Fatal(err)
	}
	first, err := repository.PublishProjectTagResult(ctx, identity, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if first.Reused || first.ResultID == "" {
		t.Fatalf("first project-tag publication = %#v", first)
	}

	secondIdentity := identity
	secondIdentity.SourceMetadataVersionID = secondMetadataID
	second, err := repository.PublishProjectTagResult(ctx, secondIdentity, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Reused || second.ResultID != first.ResultID {
		t.Fatalf("same title result was not reused: first=%#v second=%#v", first, second)
	}

	stored, err := repository.ReadDeterministicProjectTagResult(ctx, secondIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != first.ResultID || stored.Identity != secondIdentity || len(stored.Result.Assignments) != 1 {
		t.Fatalf("stored project-tag result = %#v", stored)
	}
	assignment := stored.Result.Assignments[0]
	if assignment.ProjectKey != "game.zenless-zone-zero" || assignment.DisplayName != "绝区零" || assignment.Classification != ProjectTagNamedGame || len(assignment.Evidence) != 1 || assignment.Evidence[0] != (ProjectTitleEvidence{MatchedKeyword: "绝区零", CodePointRange: CodePointRange{Start: 0, End: 3}}) {
		t.Fatalf("stored project assignment = %#v", assignment)
	}

	noMatchMetadataID := insertSyntheticProjectMetadataForExistingJob(t, repository, "00000000-0000-4000-8000-000000000a04", "00000000-0000-4000-8000-000000000a01", "通用客户端工程师", "title-change")
	empty, err := ParseProjectTitle("通用客户端工程师")
	if err != nil {
		t.Fatal(err)
	}
	noMatchIdentity := identity
	noMatchIdentity.SourceMetadataVersionID = noMatchMetadataID
	noMatch, err := repository.PublishProjectTagResult(ctx, noMatchIdentity, empty)
	if err != nil {
		t.Fatal(err)
	}
	if noMatch.Reused || noMatch.ResultID == "" {
		t.Fatalf("no-match project-tag publication = %#v", noMatch)
	}
	storedNoMatch, err := repository.ReadDeterministicProjectTagResult(ctx, noMatchIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if len(storedNoMatch.Result.Assignments) != 0 {
		t.Fatalf("stored no-match result = %#v, want empty assignments", storedNoMatch)
	}

	changedParser := identity
	changedParser.ParserVersion = "deterministic-project-title-parser-test-v2"
	changed, err := repository.PublishProjectTagResult(ctx, changedParser, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Reused || changed.ResultID == first.ResultID {
		t.Fatalf("changed parser identity did not retain separate project-tag history: first=%#v changed=%#v", first, changed)
	}

	var resultCount, assignmentCount, evidenceCount, observationCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM project_tag_results),
  (SELECT count(*) FROM project_tag_assignments),
  (SELECT count(*) FROM project_tag_evidence),
  (SELECT count(*) FROM project_title_observations)`).Scan(&resultCount, &assignmentCount, &evidenceCount, &observationCount); err != nil {
		t.Fatal(err)
	}
	if resultCount != 3 || assignmentCount != 2 || evidenceCount != 2 || observationCount != 3 {
		t.Fatalf("project-tag persistence counts = results:%d assignments:%d evidence:%d observations:%d", resultCount, assignmentCount, evidenceCount, observationCount)
	}
}

func TestReadFormalProjectTagsNeverFallsBackToSourceProjectMetadata(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	metadataID := insertSyntheticProjectMetadata(t, repository, "00000000-0000-4000-8000-000000000a11", "00000000-0000-4000-8000-000000000a12", "通用客户端工程师", "formal-no-fallback")

	before, available, err := repository.ReadFormalProjectTags(ctx, metadataID)
	if err != nil || available || len(before.Assignments) != 0 {
		t.Fatalf("formal project read fell back before deterministic processing: result=%#v available:%t err:%v", before, available, err)
	}

	parsed, err := ParseProjectTitle("通用客户端工程师")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.PublishProjectTagResult(ctx, ProjectTagIdentity{
		SourceMetadataVersionID: metadataID,
		ParserVersion:           ProjectTagParserVersion(),
		DictionaryVersion:       ProjectTagDictionaryVersion(),
		DictionarySHA256:        ProjectTagDictionaryDigest(),
	}, parsed); err != nil {
		t.Fatal(err)
	}
	after, available, err := repository.ReadFormalProjectTags(ctx, metadataID)
	if err != nil || !available || len(after.Assignments) != 0 {
		t.Fatalf("formal unidentified project read = %#v available:%t err:%v", after, available, err)
	}
}

func insertSyntheticProjectMetadata(t *testing.T, repository *Repository, jobID, metadataID, title, salt string) string {
	t.Helper()
	if _, err := repository.pool.Exec(context.Background(), `
INSERT INTO jobs (id, source_company, source_job_id)
VALUES ($1, 'project-tag-test', $2)`, jobID, "source-"+salt); err != nil {
		t.Fatal(err)
	}
	return insertSyntheticProjectMetadataForExistingJob(t, repository, metadataID, jobID, title, salt)
}

func insertSyntheticProjectMetadataForExistingJob(t *testing.T, repository *Repository, metadataID, jobID, title, salt string) string {
	t.Helper()
	payload := `{"title":{"state":"string","value":"` + title + `"},"locations":{"state":"array","items":[]},"source_project":{"id":{"state":"absent"},"name":{"state":"string","value":"社会招聘"}}}`
	if _, err := repository.pool.Exec(context.Background(), `
INSERT INTO job_source_metadata_versions (
    id, job_id, metadata_sha256, normalization_version, metadata_payload
) VALUES ($1, $2, $3, 'source-metadata-v1', $4::jsonb)`, metadataID, jobID, testSHA256("project-metadata-"+salt), payload); err != nil {
		t.Fatal(err)
	}
	return metadataID
}
