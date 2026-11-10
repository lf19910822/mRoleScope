package jobsync

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mrolescope/internal/jdserving"
)

type listQueryTrace struct {
	mutex sync.Mutex
	sql   string
	args  []any
}

func (trace *listQueryTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(data.SQL, "\nWITH selected_history_scope AS (") {
		trace.mutex.Lock()
		trace.sql = data.SQL
		trace.args = append([]any(nil), data.Args...)
		trace.mutex.Unlock()
	}
	return ctx
}

func (trace *listQueryTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

type explainNode struct {
	Alias       string        `json:"Alias"`
	ActualLoops int           `json:"Actual Loops"`
	Plans       []explainNode `json:"Plans"`
}

func historyTransitionLoops(node explainNode) (int, bool) {
	if node.Alias == "history_transitions" {
		return node.ActualLoops, true
	}
	for _, child := range node.Plans {
		if loops, found := historyTransitionLoops(child); found {
			return loops, true
		}
	}
	return 0, false
}

func TestListJobsComputesHistoryTransitionsOnce(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	_, err := repository.pool.Exec(ctx, `
INSERT INTO observation_scopes(id,source_company,filter_config,filter_sha256)
VALUES (md5('perf-scope')::uuid,'fixture','{}',repeat('a',64));
INSERT INTO collection_runs(id,schedule_key,source_company,filter_config,filter_sha256,status,published_at,observation_scope_id,data_date)
SELECT md5('perf-run'||day)::uuid, 'perf'||day, 'fixture', '{}', repeat('a',64), 'published',
       '2026-01-01'::timestamptz + day * interval '1 day', md5('perf-scope')::uuid, '2026-01-01'::date + day
FROM generate_series(1,22) day;
INSERT INTO observation_scope_publication_state(observation_scope_id,published_run_id)
VALUES (md5('perf-scope')::uuid,md5('perf-run22')::uuid);
INSERT INTO jobs(id,source_company,source_job_id)
SELECT md5('perf-job'||number)::uuid,'fixture',number::text FROM generate_series(1,182) number;
INSERT INTO serving_jd_public_identifiers(job_id)
SELECT id FROM jobs WHERE source_company = 'fixture';
INSERT INTO job_source_metadata_versions(id,job_id,metadata_sha256,normalization_version,metadata_payload)
SELECT md5('perf-metadata'||number)::uuid,md5('perf-job'||number)::uuid,repeat('a',64),'fixture','{}'
FROM generate_series(1,182) number;
INSERT INTO job_body_content_versions(id,job_id,body_sha256,normalization_version,description_text,job_require_text,addition_text,description_sha256,job_require_sha256,addition_sha256)
SELECT md5('perf-body'||number||'-'||version)::uuid,md5('perf-job'||number)::uuid,repeat(version::text,64),'fixture','','','',repeat('a',64),repeat('a',64),repeat('a',64)
FROM generate_series(1,182) number CROSS JOIN generate_series(1,2) version;
INSERT INTO job_presence_periods(id,observation_scope_id,job_id,opened_run_id,opened_at,last_seen_run_id,last_seen_at)
SELECT md5('perf-period'||number)::uuid,md5('perf-scope')::uuid,md5('perf-job'||number)::uuid,
       md5('perf-run1')::uuid,'2026-01-02',md5('perf-run22')::uuid,'2026-01-23'
FROM generate_series(1,182) number;
INSERT INTO serving_jd_inventory_memberships(published_run_id,observation_scope_id,job_id,presence_period_id,source_metadata_version_id,body_content_version_id,activity_state,title,first_discovered_at)
SELECT md5('perf-run'||day)::uuid,md5('perf-scope')::uuid,md5('perf-job'||number)::uuid,
       md5('perf-period'||number)::uuid,md5('perf-metadata'||number)::uuid,
       md5('perf-body'||number||'-'||CASE WHEN number <= 24 AND day > 11 THEN 2 ELSE 1 END)::uuid,
       CASE WHEN number > 166 THEN 'confirmed_closed'::serving_jd_activity_state ELSE 'effective'::serving_jd_activity_state END,
       'fixture job '||number,'2026-01-02'
FROM generate_series(1,182) number CROSS JOIN generate_series(1,22) day;
ANALYZE;
`)
	if err != nil {
		t.Fatalf("create synthetic performance fixture: %v", err)
	}

	trace := &listQueryTrace{}
	config, err := pgxpool.ParseConfig(os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	config.ConnConfig.Tracer = trace
	config.AfterConnect = func(ctx context.Context, connection *pgx.Conn) error {
		_, err := connection.Exec(ctx, `SET enable_hashjoin = off; SET enable_mergejoin = off; SET enable_material = off`)
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal("create test pool failed")
	}
	t.Cleanup(pool.Close)
	serving := jdserving.NewRepositoryFromPool(pool)
	rules, _, err := jdserving.ParseListSortRules([]string{"last_modified_desc"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := serving.ListJobs(ctx, jdserving.ListJobsQuery{SortRules: rules, PageNumber: 1, PageSize: 20})
	if err != nil || len(result.Items) != 20 || result.Page.Total != 166 {
		t.Fatalf("list fixture failed: count=%d total=%d err=%v", len(result.Items), result.Page.Total, err)
	}
	trace.mutex.Lock()
	query, arguments := trace.sql, trace.args
	trace.mutex.Unlock()
	if query == "" {
		t.Fatal("list query was not captured")
	}
	var encodedPlan []byte
	if err := pool.QueryRow(ctx, "EXPLAIN (ANALYZE, FORMAT JSON, TIMING OFF) "+query, arguments...).Scan(&encodedPlan); err != nil {
		t.Fatalf("explain list query: %v", err)
	}
	var document []struct {
		Plan explainNode `json:"Plan"`
	}
	if err := json.Unmarshal(encodedPlan, &document); err != nil || len(document) != 1 {
		t.Fatalf("decode list query plan: %v", err)
	}
	loops, found := historyTransitionLoops(document[0].Plan)
	if !found || loops != 1 {
		t.Fatalf("history transitions executed %d times (found=%t); want once per list request", loops, found)
	}
}
