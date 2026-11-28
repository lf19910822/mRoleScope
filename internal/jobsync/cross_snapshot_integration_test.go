package jobsync

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mrolescope/internal/jdserving"
)

type crossPublicationSwitchTrace struct {
	once              sync.Once
	switchPublication func() error
	err               error
}

func (trace *crossPublicationSwitchTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, "membership_facts AS MATERIALIZED") {
		trace.once.Do(func() { trace.err = trace.switchPublication() })
	}
	return ctx
}

func (*crossPublicationSwitchTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {
}

func TestCrossStatisticsBatchUsesOneSnapshotDuringPublicationSwitch(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 7, 2, 0, 0, 0, time.UTC)
	run := publishFixtureRun(t, repository, scope, at, 1, []Observation{
		integrationServingFilterObservation("snapshot-a", "Snapshot A", "a", "上海", false, at),
		integrationServingFilterObservation("snapshot-b", "Snapshot B", "b", "上海", false, at),
	}, nil)
	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatal(err)
	}
	var nextRun string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO collection_runs (id, schedule_key, source_company, filter_config, filter_sha256, status, published_at, observation_scope_id, data_date)
SELECT gen_random_uuid(), schedule_key || '-cross-next', source_company, filter_config, filter_sha256,
       'published', published_at + interval '1 hour', observation_scope_id, data_date + 1
FROM collection_runs WHERE id = $1 RETURNING id::text`, run.ID).Scan(&nextRun); err != nil {
		t.Fatal(err)
	}
	trace := &crossPublicationSwitchTrace{switchPublication: func() error {
		_, err := repository.pool.Exec(ctx, `UPDATE observation_scope_publication_state SET published_run_id = $1 WHERE observation_scope_id = $2`, nextRun, scope.ID)
		return err
	}}
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
	snapshotReader := jdserving.NewRepositoryFromPool(pool)
	query := jdserving.CrossStatisticsQuery{Conditions: []jdserving.CrossCondition{{Dimension: jdserving.TrendDimensionLocation, Operator: "any", Values: []string{"上海"}}}}
	results, err := snapshotReader.ReadCrossStatisticsBatch(ctx, []jdserving.CrossStatisticsQuery{query, query})
	if trace.err != nil {
		t.Fatalf("switch test publication: %v", trace.err)
	}
	if err != nil || len(results) != 2 || results[0].Total != 2 || results[1].Total != 2 {
		t.Fatalf("batch mixed publication snapshots: results=%#v err=%v", results, err)
	}
	current, err := serving.ReadCrossStatistics(ctx, jdserving.CrossStatisticsQuery{GroupBy: jdserving.TrendDimensionLocation})
	if err != nil || current.Total != 0 {
		t.Fatalf("subsequent read did not see switched publication: %#v err=%v", current, err)
	}
}
