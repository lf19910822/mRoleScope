package aitrend

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mrolescope/internal/jdserving"
)

type fakeCrossBatchReader struct {
	queries []jdserving.CrossStatisticsQuery
	results []jdserving.CrossStatisticsResult
	err     error
}

func (reader *fakeCrossBatchReader) ReadCrossStatisticsBatch(_ context.Context, queries []jdserving.CrossStatisticsQuery) ([]jdserving.CrossStatisticsResult, error) {
	reader.queries = queries
	return reader.results, reader.err
}

func TestCrossToolUsesConfirmedScopeAndReturnsOnlyAggregateFacts(t *testing.T) {
	snapshot := crossTestSnapshot()
	reader := &fakeCrossBatchReader{results: []jdserving.CrossStatisticsResult{{Total: 12}}}
	service := ServingToolService{Snapshot: snapshot, Cross: reader, CrossRun: newCrossStatisticsSession("上海 Java 岗位有多少", snapshot)}
	registry, err := service.Registry()
	if err != nil {
		t.Fatal(err)
	}
	call := ToolCall{Name: ToolCrossStatistics, Params: json.RawMessage(`{"sub_queries":[{"id":"q1","time":"current","conditions":[{"dimension":"location","operator":"any","values":["上海"]},{"dimension":"technology","operator":"any","values":["java"]}]}]}`)}
	result, err := registry.Call(context.Background(), call)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Available || len(reader.queries) != 1 || len(reader.queries[0].Conditions) != 2 || len(service.CrossRun.records) != 1 {
		t.Fatalf("cross tool did not retain the confirmed plan: %#v", result)
	}
	text := string(result.Payload)
	if !strings.Contains(text, `"total_jobs":12`) || strings.Contains(text, "job_id") || strings.Contains(text, "description") || strings.Contains(text, "SELECT") {
		t.Fatalf("unsafe or incorrect model payload: %s", text)
	}
}

func TestCrossToolRejectsRawDataAndUnconfirmedScopeBeforeDatabaseRead(t *testing.T) {
	snapshot := crossTestSnapshot()
	for _, params := range []string{
		`{"sub_queries":[{"id":"q","time":"current","sql":"SELECT * FROM jobs","conditions":[{"dimension":"technology","operator":"any","values":["java"]}]}]}`,
		`{"sub_queries":[{"id":"q","time":"current","job_id":"123","conditions":[{"dimension":"technology","operator":"any","values":["java"]}]}]}`,
		`{"sub_queries":[{"id":"q","time":"current","conditions":[{"dimension":"salary","operator":"any","values":["high"]}]}]}`,
		`{"sub_queries":[{"id":"q","time":"current","conditions":[{"dimension":"location","operator":"any","values":["上海"]}]}]}`,
		`{"sub_queries":[{"id":"q","time":"current","conditions":[{"dimension":"location","operator":"any","values":["上海"]},{"dimension":"technology","operator":"any","values":["python"]}]}]}`,
	} {
		reader := &fakeCrossBatchReader{}
		service := ServingToolService{Snapshot: snapshot, Cross: reader, CrossRun: newCrossStatisticsSession("上海 Java 岗位有多少", snapshot)}
		if _, err := service.queryCrossStatistics(context.Background(), json.RawMessage(params)); err == nil {
			t.Fatalf("unsafe tool request was accepted: %s", params)
		}
		if len(reader.queries) != 0 {
			t.Fatalf("unsafe request reached the database: %s", params)
		}
	}
}

func TestCrossToolRejectsUnverifiedGroupLabels(t *testing.T) {
	snapshot := crossTestSnapshot()
	reader := &fakeCrossBatchReader{results: []jdserving.CrossStatisticsResult{{Total: 1, Groups: []jdserving.CrossStatisticsGroup{{Value: "backend", Label: "secret JD content", Count: 1}}}}}
	service := ServingToolService{Snapshot: snapshot, Cross: reader, CrossRun: newCrossStatisticsSession("游戏引擎相关岗位中哪些岗位族最多", snapshot)}
	params := json.RawMessage(`{"sub_queries":[{"id":"q","time":"current","group_by":"job_family","conditions":[{"dimension":"technology","operator":"any","categories":["game-engines"]}]}]}`)
	if _, err := service.queryCrossStatistics(context.Background(), params); err == nil {
		t.Fatal("unverified group label was exposed to the model")
	}
	if len(service.CrossRun.records) != 0 {
		t.Fatal("unverified aggregate was retained for user delivery")
	}
}

func TestCrossToolRejectsChangedPlanAfterConfirmedResults(t *testing.T) {
	snapshot := crossTestSnapshot()
	reader := &fakeCrossBatchReader{results: []jdserving.CrossStatisticsResult{{Total: 12}}}
	service := ServingToolService{Snapshot: snapshot, Cross: reader, CrossRun: newCrossStatisticsSession("上海 Java 岗位有多少", snapshot)}
	params := json.RawMessage(`{"sub_queries":[{"id":"q1","time":"current","conditions":[{"dimension":"location","operator":"any","values":["上海"]},{"dimension":"technology","operator":"any","values":["java"]}]}]}`)
	if _, err := service.queryCrossStatistics(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	changed := json.RawMessage(strings.Replace(string(params), `"q1"`, `"q2"`, 1))
	if _, err := service.queryCrossStatistics(context.Background(), changed); err == nil {
		t.Fatal("same request silently switched its aggregate snapshot")
	}
}
