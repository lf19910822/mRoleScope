package jobsync

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mrolescope/internal/jdserving"
)

type crossPerformanceTrace struct {
	mu   sync.Mutex
	sql  string
	args []any
}

func (trace *crossPerformanceTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, "membership_facts AS MATERIALIZED") {
		trace.mu.Lock()
		trace.sql = data.SQL
		trace.args = append([]any(nil), data.Args...)
		trace.mu.Unlock()
	}
	return ctx
}

func (*crossPerformanceTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestCrossStatisticsSyntheticPerformance(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	insertPublishedAnalysisConfiguration(t, repository)
	fixturePrefix := "cross-perf-" + strings.ReplaceAll(newUUID(), "-", "") + "-"
	fixtureSQL := strings.ReplaceAll(`
INSERT INTO observation_scopes (id, source_company, filter_config, filter_sha256)
VALUES (md5('cross-perf-scope')::uuid, 'cross-fixture', '{}', repeat('a',64));
INSERT INTO collection_runs (id, schedule_key, source_company, filter_config, filter_sha256, status, published_at, observation_scope_id, data_date)
VALUES (md5('cross-perf-run')::uuid, 'cross-perf', 'cross-fixture', '{}', repeat('a',64), 'published', now(), md5('cross-perf-scope')::uuid, current_date);
INSERT INTO observation_scope_publication_state (observation_scope_id, published_run_id)
VALUES (md5('cross-perf-scope')::uuid, md5('cross-perf-run')::uuid);
INSERT INTO jobs (id, source_company, source_job_id)
SELECT md5('cross-perf-job'||n)::uuid, 'cross-fixture', n::text FROM generate_series(1,1000) AS n;
INSERT INTO job_source_metadata_versions (id, job_id, metadata_sha256, normalization_version, metadata_payload)
SELECT md5('cross-perf-meta'||n)::uuid, md5('cross-perf-job'||n)::uuid, repeat('a',64), 'fixture', '{"locations":{"state":"array"}}'::jsonb
FROM generate_series(1,1000) AS n;
INSERT INTO job_body_content_versions (id, job_id, body_sha256, normalization_version, description_text, job_require_text, addition_text, description_sha256, job_require_sha256, addition_sha256)
SELECT md5('cross-perf-body'||n)::uuid, md5('cross-perf-job'||n)::uuid, repeat('a',64), 'fixture', '', '', '', repeat('a',64), repeat('a',64), repeat('a',64)
FROM generate_series(1,1000) AS n;
INSERT INTO job_presence_periods (id, observation_scope_id, job_id, opened_run_id, opened_at, last_seen_run_id, last_seen_at)
SELECT md5('cross-perf-presence'||n)::uuid, md5('cross-perf-scope')::uuid, md5('cross-perf-job'||n)::uuid, md5('cross-perf-run')::uuid, now(), md5('cross-perf-run')::uuid, now()
FROM generate_series(1,1000) AS n;
INSERT INTO serving_jd_inventory_memberships (id, published_run_id, observation_scope_id, job_id, presence_period_id, source_metadata_version_id, body_content_version_id, activity_state, title, locations, first_discovered_at)
SELECT md5('cross-perf-member'||n)::uuid, md5('cross-perf-run')::uuid, md5('cross-perf-scope')::uuid, md5('cross-perf-job'||n)::uuid,
       md5('cross-perf-presence'||n)::uuid, md5('cross-perf-meta'||n)::uuid, md5('cross-perf-body'||n)::uuid, 'effective', 'fixture',
       ARRAY[CASE WHEN n % 2 = 0 THEN '上海' ELSE '北京' END], now()
FROM generate_series(1,1000) AS n;
INSERT INTO technology_identities (technology_key, canonical_name, technology_kind)
VALUES ('cross-perf-java', 'Cross Perf Java', 'programming-language') ON CONFLICT (technology_key) DO NOTHING;
INSERT INTO technology_analysis_results (id, body_content_version_id, parser_version, dictionary_version_id, configuration_id, status, completed_at)
SELECT md5('cross-perf-tech-result'||n)::uuid, md5('cross-perf-body'||n)::uuid, configuration.parser_version, configuration.dictionary_version_id, configuration.id, 'complete', now()
FROM generate_series(1,1000) AS n CROSS JOIN technology_analysis_publication_state AS publication
JOIN technology_analysis_configurations AS configuration ON configuration.id = publication.configuration_id
WHERE publication.singleton = true;
INSERT INTO technology_mentions (analysis_result_id, technology_identity_id)
SELECT md5('cross-perf-tech-result'||n)::uuid, identity.id FROM generate_series(1,1000) AS n
CROSS JOIN technology_identities AS identity WHERE identity.technology_key = 'cross-perf-java';
INSERT INTO job_family_title_identities (id, title_sha256, normalization_version, normalized_title)
SELECT md5('cross-perf-title'||n)::uuid, encode(digest('cross-perf-title'||n, 'sha256'), 'hex'), 'fixture', 'fixture'
FROM generate_series(1,1000) AS n;
INSERT INTO job_family_results (id, job_family_title_identity_id, parser_version, dictionary_version, dictionary_sha256, project_mask_parser_version, project_mask_dictionary_version, project_mask_dictionary_sha256, status, primary_family_key)
SELECT md5('cross-perf-family-result'||n)::uuid, md5('cross-perf-title'||n)::uuid, 'fixture', 'fixture', repeat('a',64), 'fixture', 'fixture', repeat('b',64), 'classified', 'backend'
FROM generate_series(1,1000) AS n;
INSERT INTO job_family_matches (job_family_result_id, family_key, match_count)
SELECT md5('cross-perf-family-result'||n)::uuid, 'backend', 1 FROM generate_series(1,1000) AS n;
INSERT INTO serving_jd_dimension_resolutions (serving_jd_inventory_membership_id, dimension, selection_priority, resolution_key, technology_analysis_result_id)
SELECT md5('cross-perf-member'||n)::uuid, 'technology', 0, 'fixture-tech', md5('cross-perf-tech-result'||n)::uuid
FROM generate_series(1,1000) AS n;
INSERT INTO serving_jd_dimension_resolutions (serving_jd_inventory_membership_id, dimension, selection_priority, resolution_key, job_family_result_id)
SELECT md5('cross-perf-member'||n)::uuid, 'job_family', 0, 'fixture-family', md5('cross-perf-family-result'||n)::uuid
FROM generate_series(1,1000) AS n;
ANALYZE;`, "cross-perf-", fixturePrefix)
	_, err := repository.pool.Exec(ctx, fixtureSQL)
	if err != nil {
		t.Fatalf("create synthetic cross-statistics fixture: %v", err)
	}
	trace := &crossPerformanceTrace{}
	config, err := pgxpool.ParseConfig(os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	config.ConnConfig.Tracer = trace
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal("create test pool failed")
	}
	t.Cleanup(pool.Close)
	serving := jdserving.NewRepositoryFromPool(pool)
	query := jdserving.CrossStatisticsQuery{Conditions: []jdserving.CrossCondition{
		{Dimension: jdserving.TrendDimensionLocation, Operator: "any", Values: []string{"上海"}},
		{Dimension: jdserving.TrendDimensionTechnology, Operator: "any", Values: []string{fixturePrefix + "java"}},
	}, GroupBy: jdserving.TrendDimensionJobFamily}
	started := time.Now()
	result, err := serving.ReadCrossStatistics(ctx, query)
	elapsed := time.Since(started)
	if err != nil || result.Total != 500 || len(result.Groups) != 1 || result.Groups[0].Count != 500 {
		t.Fatalf("synthetic 1000-job query returned %#v, err=%v", result, err)
	}
	trace.mu.Lock()
	sql, args := trace.sql, trace.args
	trace.mu.Unlock()
	if sql == "" {
		t.Fatal("cross-statistics query was not captured")
	}
	var encoded []byte
	if err := pool.QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON, TIMING OFF) "+sql, args...).Scan(&encoded); err != nil {
		t.Fatalf("explain synthetic cross query: %v", err)
	}
	var plan []struct {
		ExecutionTime float64 `json:"Execution Time"`
		PlanningTime  float64 `json:"Planning Time"`
	}
	if err := json.Unmarshal(encoded, &plan); err != nil || len(plan) != 1 {
		t.Fatalf("decode aggregate query plan: %v", err)
	}
	t.Logf("synthetic fixture: 1000 current jobs, 500 matches; end-to-end=%s; query-plan execution=%.2fms, planning=%.2fms", elapsed.Round(time.Millisecond), plan[0].ExecutionTime, plan[0].PlanningTime)
}
