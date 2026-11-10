package jdserving

import (
	"testing"
	"time"
)

func historyAt(day string, hour int) time.Time {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	parsed, err := time.ParseInLocation("2006-01-02", day, location)
	if err != nil {
		panic(err)
	}
	return parsed.Add(time.Duration(hour) * time.Hour)
}

func TestBuildHistoryNodesUsesDailyFinalObservationAndMergesLatest(t *testing.T) {
	rows := []historyObservation{
		{Date: "2026-11-01", PublishedAt: historyAt("2026-11-01", 1), RunID: "early", PresencePeriodID: "period-a", SourceMetadataVersionID: "meta-1", BodyContentVersionID: "body-1", ActivityState: ActivityStateEffective},
		{Date: "2026-11-01", PublishedAt: historyAt("2026-11-01", 2), RunID: "final", PresencePeriodID: "period-a", SourceMetadataVersionID: "meta-1", BodyContentVersionID: "body-1", ActivityState: ActivityStateEffective},
		{Date: "2026-11-03", PublishedAt: historyAt("2026-11-03", 1), RunID: "changed", PresencePeriodID: "period-a", SourceMetadataVersionID: "meta-2", BodyContentVersionID: "body-2", ActivityState: ActivityStateEffective},
	}

	nodes := buildHistoryNodes(selectFinalHistoryObservations(rows))
	if len(nodes) != 2 {
		t.Fatalf("history nodes = %#v, want discovered and latest-modified", nodes)
	}
	if nodes[0].Date != "2026-11-01" || nodes[0].Type != HistoryNodeDiscovered {
		t.Fatalf("first node = %#v", nodes[0])
	}
	if nodes[1].Date != "2026-11-03" || nodes[1].Type != HistoryNodeModified {
		t.Fatalf("modified node = %#v", nodes[1])
	}
	if !historyNodeHasTag(nodes[1], HistoryNodeTagLatest) {
		t.Fatalf("latest marker was not merged into modified node: %#v", nodes[1])
	}
}

func TestBuildHistoryNodesIgnoresParserOnlyChangesAndMarksCloseReopenGap(t *testing.T) {
	rows := []historyObservation{
		{Date: "2026-11-01", PublishedAt: historyAt("2026-11-01", 1), PresencePeriodID: "period-a", SourceMetadataVersionID: "meta-1", BodyContentVersionID: "body-1", ActivityState: ActivityStateEffective},
		// The parser output is intentionally different, but source version IDs are unchanged.
		{Date: "2026-11-02", PublishedAt: historyAt("2026-11-02", 1), PresencePeriodID: "period-a", SourceMetadataVersionID: "meta-1", BodyContentVersionID: "body-1", ActivityState: ActivityStateEffective, ParserFingerprint: "new-parser"},
		{Date: "2026-11-03", PublishedAt: historyAt("2026-11-03", 1), PresencePeriodID: "period-a", SourceMetadataVersionID: "meta-2", BodyContentVersionID: "body-2", ActivityState: ActivityStateConfirmedClosed},
		{Date: "2026-11-06", PublishedAt: historyAt("2026-11-06", 1), PresencePeriodID: "period-b", SourceMetadataVersionID: "meta-2", BodyContentVersionID: "body-2", ActivityState: ActivityStateEffective},
	}

	nodes := buildHistoryNodes(selectFinalHistoryObservations(rows))
	if len(nodes) != 3 {
		t.Fatalf("history nodes = %#v, want discovered, close, latest-reopen", nodes)
	}
	for _, node := range nodes {
		if node.Date == "2026-11-02" {
			t.Fatalf("parser-only change created a node: %#v", node)
		}
	}
	if nodes[1].Type != HistoryNodeConfirmedClosed {
		t.Fatalf("close node = %#v", nodes[1])
	}
	if nodes[2].Type != HistoryNodeReopened || nodes[2].ConnectPrevious {
		t.Fatalf("reopen node = %#v, want disconnected reopen", nodes[2])
	}
}

func TestBuildHistoryNodesTreatsEffectiveObservationAfterCloseAsReopened(t *testing.T) {
	rows := []historyObservation{
		{Date: "2026-11-01", PublishedAt: historyAt("2026-11-01", 1), PresencePeriodID: "period-a", SourceMetadataVersionID: "meta-1", BodyContentVersionID: "body-1", ActivityState: ActivityStateEffective},
		{Date: "2026-11-03", PublishedAt: historyAt("2026-11-03", 1), PresencePeriodID: "period-a", SourceMetadataVersionID: "meta-1", BodyContentVersionID: "body-1", ActivityState: ActivityStateConfirmedClosed},
		// A controlled historical repair can retain the original presence period.
		{Date: "2026-11-06", PublishedAt: historyAt("2026-11-06", 1), PresencePeriodID: "period-a", SourceMetadataVersionID: "meta-1", BodyContentVersionID: "body-1", ActivityState: ActivityStateEffective},
	}

	nodes := buildHistoryNodes(rows)
	if len(nodes) != 3 {
		t.Fatalf("history nodes = %#v, want discovered, closed, reopened", nodes)
	}
	if nodes[2].Type != HistoryNodeReopened {
		t.Fatalf("effective observation after close = %#v, want reopened", nodes[2])
	}
}

func TestBuildHistoryNodesDoesNotCreateLatestNodeAfterConfirmedClose(t *testing.T) {
	rows := []historyObservation{
		{Date: "2026-11-01", PublishedAt: historyAt("2026-11-01", 1), PresencePeriodID: "period-a", ActivityState: ActivityStateEffective},
		{Date: "2026-11-02", PublishedAt: historyAt("2026-11-02", 1), PresencePeriodID: "period-a", ActivityState: ActivityStateConfirmedClosed},
		{Date: "2026-11-03", PublishedAt: historyAt("2026-11-03", 1), PresencePeriodID: "period-a", ActivityState: ActivityStateConfirmedClosed},
	}

	nodes := buildHistoryNodes(rows)
	if len(nodes) != 2 {
		t.Fatalf("history nodes = %#v, want discovered and confirmed closed", nodes)
	}
	if nodes[1].Type != HistoryNodeConfirmedClosed {
		t.Fatalf("latest closed node = %#v, want confirmed_closed", nodes[1])
	}
	if !historyNodeHasTag(nodes[1], HistoryNodeTagLatest) {
		t.Fatalf("latest marker was not merged into the close node: %#v", nodes[1])
	}
}

func TestHistoryComparisonUsesNearestSuccessfulObservationAndClassifiesBodySections(t *testing.T) {
	previous := historyObservation{SourceMetadataVersionID: "meta-1", BodyContentVersionID: "body-1", Detail: JobDetailResponse{Title: "旧岗位", Body: BodySections{Responsibilities: "负责 A", Requirements: "要求 B", Supplemental: stringPointer("补充 C")}}}
	current := historyObservation{SourceMetadataVersionID: "meta-2", BodyContentVersionID: "body-2", Detail: JobDetailResponse{Title: "新岗位", Body: BodySections{Responsibilities: "负责 A\n负责 D", Requirements: "要求 B", Supplemental: stringPointer("补充 E")}}}

	comparison := compareHistoryObservations(previous, current)
	if comparison.Status != ComparisonStatusAvailable || len(comparison.DimensionChanges) == 0 {
		t.Fatalf("comparison dimensions = %#v", comparison)
	}
	if len(comparison.BodySections) != 2 {
		t.Fatalf("comparison body sections = %#v", comparison.BodySections)
	}
	if comparison.BodySections[0].Section != BodySectionResponsibilities || comparison.BodySections[1].Section != BodySectionSupplemental {
		t.Fatalf("comparison body order = %#v", comparison.BodySections)
	}
}

func stringPointer(value string) *string { return &value }
