package jdanalysis

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTrendFormalReleaseRejectsIncompletePublicationAndIsImmutable(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()

	var releaseID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO trend_formal_releases (identity_sha256)
VALUES ($1)
RETURNING id::text`, strings.Repeat("a", 64)).Scan(&releaseID); err != nil {
		t.Fatalf("create candidate trend release: %v", err)
	}
	if _, err := repository.pool.Exec(ctx, `
UPDATE trend_formal_releases
SET status = 'published', published_at = now()
WHERE id = $1`, releaseID); err == nil {
		t.Fatal("incomplete trend release was published")
	}
	if _, err := repository.pool.Exec(ctx, `
UPDATE trend_formal_releases
SET identity_sha256 = $2
WHERE id = $1`, releaseID, strings.Repeat("b", 64)); err == nil {
		t.Fatal("trend release identity was mutable")
	}
}

func TestRepositoryPreparesOneCompleteTrendFormalReleaseCandidate(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	configurationID, _ := prepareCompleteV02ReleaseCandidate(t, repository)
	technologyRelease, err := repository.ReleaseV02Baseline(ctx, "test")
	if err != nil || !technologyRelease.Published || technologyRelease.ConfigurationID != configurationID {
		t.Fatalf("publish technology baseline = %#v, error: %v", technologyRelease, err)
	}

	candidate, err := repository.PrepareTrendFormalRelease(ctx)
	if err != nil {
		t.Fatalf("prepare trend formal release: %v", err)
	}
	if candidate.ID == "" || candidate.IdentitySHA256 == "" || candidate.Status != TrendFormalReleaseCandidate {
		t.Fatalf("prepared trend release = %#v", candidate)
	}
	var components int
	if err := repository.pool.QueryRow(ctx, `
SELECT count(*)
FROM trend_formal_release_components
WHERE trend_formal_release_id = $1`, candidate.ID).Scan(&components); err != nil {
		t.Fatalf("count prepared trend release components: %v", err)
	}
	if components != 5 {
		t.Fatalf("prepared trend release components = %d, want 5", components)
	}
}

func TestTrendFormalReleaseRefusesPublicationUntilEveryObservedVersionIsCovered(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, _, _, _ := insertSyntheticAnalysisPrerequisites(t, repository)
	attachSyntheticTrendMembership(t, repository, bodyID)
	_, _ = prepareCompleteV02ReleaseCandidate(t, repository)
	if _, err := repository.ReleaseV02Baseline(ctx, "test"); err != nil {
		t.Fatalf("publish technology baseline: %v", err)
	}
	candidate, err := repository.PrepareTrendFormalRelease(ctx)
	if err != nil {
		t.Fatalf("prepare trend formal release: %v", err)
	}

	if _, err := repository.PublishTrendFormalRelease(ctx, candidate.ID); !errors.Is(err, ErrTrendFormalReleaseIncomplete) {
		t.Fatalf("publish incomplete trend formal release error = %v, want ErrTrendFormalReleaseIncomplete", err)
	}
}

func TestTrendFormalReleasePublishesOnceAndReadsTheCurrentRelease(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	_, _ = prepareCompleteV02ReleaseCandidate(t, repository)
	if _, err := repository.ReleaseV02Baseline(ctx, "test"); err != nil {
		t.Fatalf("publish technology baseline: %v", err)
	}
	candidate, err := repository.PrepareTrendFormalRelease(ctx)
	if err != nil {
		t.Fatalf("prepare trend formal release: %v", err)
	}
	published, err := repository.PublishTrendFormalRelease(ctx, candidate.ID)
	if err != nil || published.Reused || published.Status != TrendFormalReleasePublished {
		t.Fatalf("publish trend formal release = %#v, error: %v", published, err)
	}
	reused, err := repository.PublishTrendFormalRelease(ctx, candidate.ID)
	if err != nil || !reused.Reused || reused.ID != published.ID {
		t.Fatalf("reuse trend formal release = %#v, error: %v", reused, err)
	}
	current, err := repository.ReadPublishedTrendFormalRelease(ctx)
	if err != nil || current.ID != published.ID || current.Status != TrendFormalReleasePublished {
		t.Fatalf("read published trend formal release = %#v, error: %v", current, err)
	}
}

func attachSyntheticTrendMembership(t *testing.T, repository *Repository, bodyID string) time.Time {
	t.Helper()
	ctx := context.Background()
	const (
		scopeID    = "00000000-0000-4000-8000-000000000701"
		runID      = "00000000-0000-4000-8000-000000000702"
		metadataID = "00000000-0000-4000-8000-000000000703"
	)
	observedAt := time.Now().UTC().Truncate(time.Second)
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO observation_scopes (id, source_company, filter_config, filter_sha256)
VALUES ($1, 'synthetic-source', '{}'::jsonb, $2)`, scopeID, testSHA256("trend-scope")); err != nil {
		t.Fatalf("insert trend scope: %v", err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO collection_runs (
    id, schedule_key, source_company, filter_config, filter_sha256,
    status, started_at, finished_at, published_at,
    observation_scope_id, data_date
) VALUES (
    $1, 'synthetic-trend-run', 'synthetic-source', '{}'::jsonb, $2,
    'published', $3, $3, $3,
    $4, ($3 AT TIME ZONE 'Asia/Shanghai')::date
)`, runID, testSHA256("trend-scope"), observedAt, scopeID); err != nil {
		t.Fatalf("insert trend run: %v", err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO job_source_metadata_versions (
    id, job_id, metadata_sha256, normalization_version, metadata_payload
) VALUES ($1, $2, $3, 'synthetic-trend-v1', '{}'::jsonb)`,
		metadataID, "00000000-0000-4000-8000-000000000501", testSHA256("trend-metadata")); err != nil {
		t.Fatalf("insert trend metadata: %v", err)
	}
	var presenceID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO job_presence_periods (
    observation_scope_id, job_id, opened_run_id, opened_at, last_seen_run_id, last_seen_at
) VALUES ($1, $2, $3, $4, $3, $4)
RETURNING id::text`, scopeID, "00000000-0000-4000-8000-000000000501", runID, observedAt).Scan(&presenceID); err != nil {
		t.Fatalf("insert trend presence: %v", err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO serving_jd_inventory_memberships (
    published_run_id, observation_scope_id, job_id, presence_period_id,
    source_metadata_version_id, body_content_version_id, activity_state,
    title, locations, is_urgent
) VALUES ($1, $2, $3, $4, $5, $6, 'effective', 'Synthetic trend', ARRAY['上海'], false)`,
		runID, scopeID, "00000000-0000-4000-8000-000000000501", presenceID, metadataID, bodyID); err != nil {
		t.Fatalf("insert trend serving membership: %v", err)
	}
	return observedAt
}
