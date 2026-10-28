package jdanalysis

import (
	"context"
	"testing"
)

func TestProcessPendingProjectTagsCatchesUpAndUsesTitleIdentityIncrementally(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	firstMetadataID := insertSyntheticProjectMetadata(t, repository, "00000000-0000-4000-8000-000000000b01", "00000000-0000-4000-8000-000000000b02", "绝区零-客户端", "initial")

	first, err := ProcessPendingProjectTags(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if first.Processed != 1 || first.Unidentified != 0 {
		t.Fatalf("first project-tag processing = %#v, want one matched catch-up result", first)
	}
	formal, available, err := repository.ReadFormalProjectTags(ctx, firstMetadataID)
	if err != nil || !available || len(formal.Assignments) != 1 || formal.Assignments[0].ProjectKey != "game.zenless-zone-zero" {
		t.Fatalf("first formal project tags = %#v available:%t err:%v", formal, available, err)
	}

	second, err := ProcessPendingProjectTags(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if second.Processed != 0 {
		t.Fatalf("repeat project-tag processing = %#v, want no new result", second)
	}

	metadataOnlyID := insertSyntheticProjectMetadataForExistingJob(t, repository, "00000000-0000-4000-8000-000000000b03", "00000000-0000-4000-8000-000000000b01", "绝区零-客户端", "location-change")
	metadataOnly, err := ProcessPendingProjectTags(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if metadataOnly.Processed != 0 {
		t.Fatalf("metadata-only project-tag processing = %#v, want no new result", metadataOnly)
	}
	metadataOnlyFormal, available, err := repository.ReadFormalProjectTags(ctx, metadataOnlyID)
	if err != nil || !available || len(metadataOnlyFormal.Assignments) != 1 || metadataOnlyFormal.Assignments[0].ProjectKey != "game.zenless-zone-zero" {
		t.Fatalf("metadata-only formal project tags = %#v available:%t err:%v", metadataOnlyFormal, available, err)
	}

	titleChangedID := insertSyntheticProjectMetadataForExistingJob(t, repository, "00000000-0000-4000-8000-000000000b04", "00000000-0000-4000-8000-000000000b01", "通用客户端工程师", "title-change")
	titleChanged, err := ProcessPendingProjectTags(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	if titleChanged.Processed != 1 || titleChanged.Unidentified != 1 {
		t.Fatalf("title-changed project-tag processing = %#v, want one unidentified result", titleChanged)
	}
	titleChangedFormal, available, err := repository.ReadFormalProjectTags(ctx, titleChangedID)
	if err != nil || !available || len(titleChangedFormal.Assignments) != 0 {
		t.Fatalf("title-changed formal project tags = %#v available:%t err:%v", titleChangedFormal, available, err)
	}

	reanalysis, err := processPendingProjectTagsForIdentity(ctx, repository, "deterministic-project-title-parser-test-v2", "deterministic-project-dictionary-test-v2", testSHA256("project-dictionary-test-v2"))
	if err != nil {
		t.Fatal(err)
	}
	if reanalysis.Processed != 2 || reanalysis.Unidentified != 1 {
		t.Fatalf("dictionary reanalysis = %#v, want both title identities", reanalysis)
	}
}
