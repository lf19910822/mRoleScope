package jdanalysis

import (
	"context"
	"testing"
)

func TestProcessPendingEducationRequirementsCatchesUpAndProcessesOnlyNewBodyVersions(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, _, _, _ := insertSyntheticAnalysisPrerequisites(t, repository)
	setSyntheticEducationJobRequire(t, repository, bodyID, "本科及以上")

	first, err := ProcessPendingEducationRequirements(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if first.Processed != 1 {
		t.Fatalf("first deterministic education processing = %#v, want one catch-up result", first)
	}
	firstRequirement, available, err := repository.ReadFormalEducationRequirement(ctx, bodyID)
	if err != nil || !available || firstRequirement.LowerBoundLevel != EducationLevelBachelor {
		t.Fatalf("first formal deterministic education requirement = %#v available:%t err:%v", firstRequirement, available, err)
	}

	second, err := ProcessPendingEducationRequirements(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if second.Processed != 0 {
		t.Fatalf("repeat deterministic education processing = %#v, want no reprocessing", second)
	}

	newBodyID := insertSyntheticAnalysisBody(t, repository, "00000000-0000-4000-8000-000000000801", "Go")
	setSyntheticEducationJobRequire(t, repository, newBodyID, "硕士及以上")
	third, err := ProcessPendingEducationRequirements(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if third.Processed != 1 {
		t.Fatalf("new body deterministic education processing = %#v, want one new result", third)
	}
	newRequirement, available, err := repository.ReadFormalEducationRequirement(ctx, newBodyID)
	if err != nil || !available || newRequirement.LowerBoundLevel != EducationLevelMaster {
		t.Fatalf("new formal deterministic education requirement = %#v available:%t err:%v", newRequirement, available, err)
	}
	firstAgain, available, err := repository.ReadFormalEducationRequirement(ctx, bodyID)
	if err != nil || !available || firstAgain.LowerBoundLevel != EducationLevelBachelor {
		t.Fatalf("historical deterministic education result changed = %#v available:%t err:%v", firstAgain, available, err)
	}
}

func setSyntheticEducationJobRequire(t *testing.T, repository *Repository, bodyContentVersionID, jobRequire string) {
	t.Helper()
	if _, err := repository.pool.Exec(context.Background(), `
UPDATE job_body_content_versions
SET job_require_text = $2,
    job_require_sha256 = $3,
    body_sha256 = $4
WHERE id = $1`, bodyContentVersionID, jobRequire, testSHA256(jobRequire), testSHA256("education-processor-"+bodyContentVersionID)); err != nil {
		t.Fatal(err)
	}
}
