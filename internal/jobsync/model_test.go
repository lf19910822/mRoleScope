package jobsync

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNormalizeObservationIgnoresTransportFields(t *testing.T) {
	listItem := map[string]any{
		"id":                "8771",
		"addressDetailList": []any{map[string]any{"addressDetail": " 上海 "}},
	}
	base := map[string]any{
		"id":          "8771",
		"title":       " 服务端研发工程师 ",
		"description": "<p>负责&nbsp;在线服务开发。</p>",
		"jobRequire":  "<p>3 年以上 Go 开发经验。</p>",
		"traceId":     "first-request",
		"updatedAt":   "2026-10-22T00:00:00Z",
	}
	first, err := NormalizeObservation(SourceCompanyMihoyo, listItem, base, time.Date(2026, 10, 22, 2, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	changedTransport := cloneMap(base)
	changedTransport["traceId"] = "second-request"
	changedTransport["updatedAt"] = "2026-10-23T00:00:00Z"
	second, err := NormalizeObservation(SourceCompanyMihoyo, listItem, changedTransport, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if first.ContentSHA256 != second.ContentSHA256 {
		t.Fatalf("transport-only update created a content version: %s != %s", first.ContentSHA256, second.ContentSHA256)
	}

	changedTitle := cloneMap(base)
	changedTitle["title"] = "客户端研发工程师"
	third, err := NormalizeObservation(SourceCompanyMihoyo, listItem, changedTitle, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if first.ContentSHA256 == third.ContentSHA256 {
		t.Fatal("title change did not create a new content hash")
	}

	changedJD := cloneMap(base)
	changedJD["description"] = "负责客户端服务开发。"
	fourth, err := NormalizeObservation(SourceCompanyMihoyo, listItem, changedJD, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if first.ContentSHA256 == fourth.ContentSHA256 {
		t.Fatal("JD change did not create a new content hash")
	}

	changedLocation := cloneMap(listItem)
	changedLocation["addressDetailList"] = []any{map[string]any{"addressDetail": "北京"}}
	fifth, err := NormalizeObservation(SourceCompanyMihoyo, changedLocation, base, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if first.ContentSHA256 == fifth.ContentSHA256 {
		t.Fatal("location change did not create a new content hash")
	}
}

func TestNormalizeObservationNormalizesEquivalentHTMLAndLocations(t *testing.T) {
	listA := map[string]any{"id": "1", "addressDetailList": []any{map[string]any{"addressDetail": "北京"}, map[string]any{"addressDetail": "上海"}}}
	listB := map[string]any{"id": "1", "addressDetailList": []any{map[string]any{"addressDetail": "上海"}, map[string]any{"addressDetail": "北京"}}}
	detailA := map[string]any{"id": "1", "title": "A", "description": "<p>负责\r\n服务</p>", "jobRequire": "<b>Go</b>"}
	detailB := map[string]any{"id": "1", "title": "A", "description": "负责 服务", "jobRequire": "Go"}
	first, err := NormalizeObservation(SourceCompanyMihoyo, listA, detailA, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	second, err := NormalizeObservation(SourceCompanyMihoyo, listB, detailB, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if first.ContentSHA256 != second.ContentSHA256 {
		t.Fatalf("equivalent business content differs: %s != %s", first.ContentSHA256, second.ContentSHA256)
	}
	var display map[string]any
	if err := json.Unmarshal(first.DisplayFields, &display); err != nil {
		t.Fatal(err)
	}
	locations, ok := display["locations"].([]any)
	if !ok || len(locations) != 2 || locations[0] != "上海" || locations[1] != "北京" {
		t.Fatalf("locations were not normalized and sorted: %#v", display["locations"])
	}
}

func cloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
