package jdanalysis

import (
	"context"
	"testing"
)

func TestRepositoryPersistsImmutableDeterministicExperienceRequirementAlongsideAIProjection(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	const jobRequire = "3年以上相关工作经验"
	if _, err := repository.pool.Exec(ctx, `
UPDATE job_body_content_versions
SET job_require_text = $2,
    job_require_sha256 = $3,
    body_sha256 = $4
WHERE id = $1`, bodyID, jobRequire, testSHA256(jobRequire), testSHA256("experience-persistence-body")); err != nil {
		t.Fatal(err)
	}

	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "experience-persistence-reference")
	aiConfiguration := createV3CandidateAIConfiguration(t, repository, technologyConfigurationID, "experience-persistence-ai-history")
	aiResult, err := repository.EnsureAISemanticResult(ctx, bodyID, aiConfiguration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.CompleteAISemanticResult(ctx, aiResult.ID, "configured"); err != nil {
		t.Fatal(err)
	}
	var aiExperienceArtifactID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_semantic_artifacts (ai_semantic_result_id, artifact_kind, semantic_status, review_tier)
VALUES ($1, 'experience_summary', 'configured', 'routine')
RETURNING id::text`, aiResult.ID).Scan(&aiExperienceArtifactID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_experience_summaries (id, status, base_year)
VALUES ($1, 'configured', 5)`, aiExperienceArtifactID); err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseExperienceRequirement(ExperienceParserInput{JobRequire: jobRequire})
	if err != nil {
		t.Fatal(err)
	}
	identity := ExperienceRequirementIdentity{
		BodyContentVersionID: bodyID,
		ParserVersion:        ExperienceRequirementParserVersion(),
		LexiconVersion:       ExperienceRequirementLexiconVersion(),
		LexiconSHA256:        ExperienceRequirementLexiconDigest(),
	}
	first, err := repository.PublishExperienceRequirement(ctx, identity, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if first.Reused || first.ResultID == "" {
		t.Fatalf("first deterministic experience publication = %#v", first)
	}
	second, err := repository.PublishExperienceRequirement(ctx, identity, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Reused || second.ResultID != first.ResultID {
		t.Fatalf("same immutable experience identity was not reused: first=%#v second=%#v", first, second)
	}

	stored, err := repository.ReadDeterministicExperienceRequirement(ctx, identity)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != first.ResultID || stored.Identity != identity || stored.Requirement.Status != ExperienceRequirementConfigured || stored.Requirement.BaseYear != 3 {
		t.Fatalf("stored deterministic experience requirement = %#v", stored)
	}
	wantEvidence := ExperienceRequirementEvidence{
		SourceField: SourceFieldJobRequire, RuleKey: "experience.arabic.years_or_more", MatchedText: jobRequire,
		MinimumYears: 3, CodePointRange: CodePointRange{Start: 0, End: 10}, Modality: ExperienceRequirementModalityRequired,
	}
	if len(stored.Requirement.Evidence) != 1 || stored.Requirement.Evidence[0] != wantEvidence {
		t.Fatalf("stored deterministic experience evidence = %#v, want %#v", stored.Requirement.Evidence, wantEvidence)
	}

	changedParserIdentity := identity
	changedParserIdentity.ParserVersion = "deterministic-experience-parser-test-v2"
	changed, err := repository.PublishExperienceRequirement(ctx, changedParserIdentity, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Reused || changed.ResultID == first.ResultID {
		t.Fatalf("changed parser identity did not create independent history: first=%#v changed=%#v", first, changed)
	}

	var aiBaseYear int
	if err := repository.pool.QueryRow(ctx, `
SELECT base_year::integer
FROM ai_experience_summaries
WHERE id = $1`, aiExperienceArtifactID).Scan(&aiBaseYear); err != nil {
		t.Fatal(err)
	}
	if aiBaseYear != 5 {
		t.Fatalf("deterministic persistence modified historical AI experience projection: %d", aiBaseYear)
	}
}

func TestReadFormalExperienceRequirementNeverFallsBackToAIProjection(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	const jobRequire = "3年以上相关工作经验"
	if _, err := repository.pool.Exec(ctx, `
UPDATE job_body_content_versions
SET job_require_text = $2,
    job_require_sha256 = $3,
    body_sha256 = $4
WHERE id = $1`, bodyID, jobRequire, testSHA256(jobRequire), testSHA256("formal-experience-read-body")); err != nil {
		t.Fatal(err)
	}

	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "formal-experience-read-reference")
	aiConfiguration := createV3CandidateAIConfiguration(t, repository, technologyConfigurationID, "formal-experience-read-ai-history")
	aiResult, err := repository.EnsureAISemanticResult(ctx, bodyID, aiConfiguration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.CompleteAISemanticResult(ctx, aiResult.ID, "configured"); err != nil {
		t.Fatal(err)
	}
	var aiExperienceArtifactID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_semantic_artifacts (ai_semantic_result_id, artifact_kind, semantic_status, review_tier)
VALUES ($1, 'experience_summary', 'configured', 'routine')
RETURNING id::text`, aiResult.ID).Scan(&aiExperienceArtifactID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_experience_summaries (id, status, base_year)
VALUES ($1, 'configured', 5)`, aiExperienceArtifactID); err != nil {
		t.Fatal(err)
	}

	beforeDeterministicResult, available, err := repository.ReadFormalExperienceRequirement(ctx, bodyID)
	if err != nil || available || beforeDeterministicResult.Status != "" || beforeDeterministicResult.BaseYear != 0 || len(beforeDeterministicResult.Evidence) != 0 {
		t.Fatalf("formal experience read fell back to AI projection: result=%#v available:%t err:%v", beforeDeterministicResult, available, err)
	}

	parsed, err := ParseExperienceRequirement(ExperienceParserInput{JobRequire: jobRequire})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.PublishExperienceRequirement(ctx, ExperienceRequirementIdentity{
		BodyContentVersionID: bodyID,
		ParserVersion:        ExperienceRequirementParserVersion(),
		LexiconVersion:       ExperienceRequirementLexiconVersion(),
		LexiconSHA256:        ExperienceRequirementLexiconDigest(),
	}, parsed); err != nil {
		t.Fatal(err)
	}
	formal, available, err := repository.ReadFormalExperienceRequirement(ctx, bodyID)
	if err != nil || !available || formal.Status != ExperienceRequirementConfigured || formal.BaseYear != 3 {
		t.Fatalf("formal deterministic experience read = %#v available:%t err:%v", formal, available, err)
	}
}
