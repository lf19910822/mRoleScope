package jdanalysis

import (
	"context"
	"testing"
)

func TestRepositoryPersistsImmutableDeterministicEducationRequirementAlongsideAIProjection(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	const jobRequire = "本科及以上"
	if _, err := repository.pool.Exec(ctx, `
UPDATE job_body_content_versions
SET job_require_text = $2,
    job_require_sha256 = $3,
    body_sha256 = $4
WHERE id = $1`, bodyID, jobRequire, testSHA256(jobRequire), testSHA256("education-persistence-body")); err != nil {
		t.Fatal(err)
	}

	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "education-persistence-reference")
	aiConfiguration := createV3CandidateAIConfiguration(t, repository, technologyConfigurationID, "education-persistence-ai-history")
	aiResult, err := repository.EnsureAISemanticResult(ctx, bodyID, aiConfiguration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.CompleteAISemanticResult(ctx, aiResult.ID, "configured"); err != nil {
		t.Fatal(err)
	}
	var aiEducationArtifactID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_semantic_artifacts (ai_semantic_result_id, artifact_kind, semantic_status, review_tier)
VALUES ($1, 'education_summary', 'configured', 'routine')
RETURNING id::text`, aiResult.ID).Scan(&aiEducationArtifactID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_minimum_education_requirements (id, status, minimum_level)
VALUES ($1, 'configured', 'master')`, aiEducationArtifactID); err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseEducationRequirement(EducationParserInput{JobRequire: jobRequire})
	if err != nil {
		t.Fatal(err)
	}
	identity := EducationRequirementIdentity{
		BodyContentVersionID: bodyID,
		ParserVersion:        EducationRequirementParserVersion(),
		LexiconVersion:       EducationRequirementLexiconVersion(),
		LexiconSHA256:        EducationRequirementLexiconDigest(),
	}
	first, err := repository.PublishEducationRequirement(ctx, identity, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if first.Reused || first.ResultID == "" {
		t.Fatalf("first deterministic education publication = %#v", first)
	}
	second, err := repository.PublishEducationRequirement(ctx, identity, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Reused || second.ResultID != first.ResultID {
		t.Fatalf("same immutable identity was not reused: first=%#v second=%#v", first, second)
	}

	stored, err := repository.ReadDeterministicEducationRequirement(ctx, identity)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != first.ResultID || stored.Identity != identity || stored.Requirement.Status != EducationRequirementConfigured || stored.Requirement.LowerBoundLevel != EducationLevelBachelor || stored.Requirement.UpperBoundLevel != "" {
		t.Fatalf("stored deterministic education requirement = %#v", stored)
	}
	wantEvidence := EducationRequirementEvidence{
		SourceField: SourceFieldJobRequire, RuleKey: "education.bachelor.benke", MatchedAlias: "本科",
		CodePointRange: CodePointRange{Start: 0, End: 2}, Modality: EducationRequirementModalityRequired,
	}
	if len(stored.Requirement.Evidence) != 1 || stored.Requirement.Evidence[0] != wantEvidence {
		t.Fatalf("stored deterministic evidence = %#v, want %#v", stored.Requirement.Evidence, wantEvidence)
	}

	changedParserIdentity := identity
	changedParserIdentity.ParserVersion = "deterministic-education-parser-test-v2"
	changed, err := repository.PublishEducationRequirement(ctx, changedParserIdentity, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Reused || changed.ResultID == first.ResultID {
		t.Fatalf("changed parser identity did not create independent history: first=%#v changed=%#v", first, changed)
	}

	var aiLevel string
	if err := repository.pool.QueryRow(ctx, `
SELECT minimum_level::text
FROM ai_minimum_education_requirements
WHERE id = $1`, aiEducationArtifactID).Scan(&aiLevel); err != nil {
		t.Fatal(err)
	}
	if aiLevel != "master" {
		t.Fatalf("deterministic persistence modified historical AI education projection: %q", aiLevel)
	}
}

func TestReadFormalEducationRequirementNeverFallsBackToAIProjection(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	const jobRequire = "本科及以上"
	setSyntheticEducationJobRequire(t, repository, bodyID, jobRequire)

	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "formal-education-read-reference")
	aiConfiguration := createV3CandidateAIConfiguration(t, repository, technologyConfigurationID, "formal-education-read-ai-history")
	aiResult, err := repository.EnsureAISemanticResult(ctx, bodyID, aiConfiguration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.CompleteAISemanticResult(ctx, aiResult.ID, "configured"); err != nil {
		t.Fatal(err)
	}
	var aiEducationArtifactID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_semantic_artifacts (ai_semantic_result_id, artifact_kind, semantic_status, review_tier)
VALUES ($1, 'education_summary', 'configured', 'routine')
RETURNING id::text`, aiResult.ID).Scan(&aiEducationArtifactID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_minimum_education_requirements (id, status, minimum_level)
VALUES ($1, 'configured', 'master')`, aiEducationArtifactID); err != nil {
		t.Fatal(err)
	}

	beforeDeterministicResult, available, err := repository.ReadFormalEducationRequirement(ctx, bodyID)
	if err != nil || available || beforeDeterministicResult.Status != "" || beforeDeterministicResult.LowerBoundLevel != "" || beforeDeterministicResult.UpperBoundLevel != "" || len(beforeDeterministicResult.Evidence) != 0 {
		t.Fatalf("formal read fell back to AI projection: result=%#v available:%t err:%v", beforeDeterministicResult, available, err)
	}

	parsed, err := ParseEducationRequirement(EducationParserInput{JobRequire: jobRequire})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.PublishEducationRequirement(ctx, EducationRequirementIdentity{
		BodyContentVersionID: bodyID,
		ParserVersion:        EducationRequirementParserVersion(),
		LexiconVersion:       EducationRequirementLexiconVersion(),
		LexiconSHA256:        EducationRequirementLexiconDigest(),
	}, parsed); err != nil {
		t.Fatal(err)
	}
	formal, available, err := repository.ReadFormalEducationRequirement(ctx, bodyID)
	if err != nil || !available || formal.LowerBoundLevel != EducationLevelBachelor || formal.UpperBoundLevel != "" {
		t.Fatalf("formal deterministic education read = %#v available:%t err:%v", formal, available, err)
	}
}
