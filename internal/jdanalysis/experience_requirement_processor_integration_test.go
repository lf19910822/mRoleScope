package jdanalysis

import (
	"context"
	"testing"
)

func TestProcessPendingExperienceRequirementsCatchesUpAndProcessesOnlyNewBodyVersions(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, _, _, _ := insertSyntheticAnalysisPrerequisites(t, repository)
	setSyntheticExperienceJobRequire(t, repository, bodyID, "3年以上工作经验")

	first, err := ProcessPendingExperienceRequirements(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if first.Processed != 1 || first.Configured != 1 {
		t.Fatalf("first deterministic experience processing = %#v, want one configured catch-up result", first)
	}
	firstRequirement, available, err := repository.ReadFormalExperienceRequirement(ctx, bodyID)
	if err != nil || !available || firstRequirement.BaseYear != 3 {
		t.Fatalf("first formal deterministic experience requirement = %#v available:%t err:%v", firstRequirement, available, err)
	}

	second, err := ProcessPendingExperienceRequirements(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if second.Processed != 0 || second.Configured != 0 || second.NotConfigured != 0 || second.Uncertain != 0 {
		t.Fatalf("repeat deterministic experience processing = %#v, want no reprocessing", second)
	}

	newBodyID := insertSyntheticAnalysisBody(t, repository, "00000000-0000-4000-8000-000000000811", "Go")
	setSyntheticExperienceJobRequire(t, repository, newBodyID, "5年以上Go开发经验")
	third, err := ProcessPendingExperienceRequirements(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if third.Processed != 1 || third.Configured != 1 {
		t.Fatalf("new body deterministic experience processing = %#v, want one configured result", third)
	}
	newRequirement, available, err := repository.ReadFormalExperienceRequirement(ctx, newBodyID)
	if err != nil || !available || newRequirement.BaseYear != 5 {
		t.Fatalf("new formal deterministic experience requirement = %#v available:%t err:%v", newRequirement, available, err)
	}
	firstAgain, available, err := repository.ReadFormalExperienceRequirement(ctx, bodyID)
	if err != nil || !available || firstAgain.BaseYear != 3 {
		t.Fatalf("historical deterministic experience result changed = %#v available:%t err:%v", firstAgain, available, err)
	}
}

func setSyntheticExperienceJobRequire(t *testing.T, repository *Repository, bodyContentVersionID, jobRequire string) {
	t.Helper()
	if _, err := repository.pool.Exec(context.Background(), `
UPDATE job_body_content_versions
SET job_require_text = $2,
    job_require_sha256 = $3,
    body_sha256 = $4
WHERE id = $1`, bodyContentVersionID, jobRequire, testSHA256(jobRequire), testSHA256("experience-processor-"+bodyContentVersionID)); err != nil {
		t.Fatal(err)
	}
}
