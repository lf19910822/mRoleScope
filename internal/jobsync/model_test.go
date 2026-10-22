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

func TestNormalizeObservationSeparatesMetadataAndBodyVersions(t *testing.T) {
	list := map[string]any{
		"id": "1",
		"addressDetailList": []any{
			map[string]any{"addressId": float64(20), "addressDetail": "Shanghai"},
			map[string]any{"addressId": float64(10), "addressDetail": "Beijing"},
		},
	}
	base := map[string]any{
		"id":               "1",
		"title":            "Platform Engineer",
		"competencyTypeId": "1",
		"competencyType":   "Technical",
		"jobNatureId":      "2",
		"jobNature":        "Full time",
		"hireType":         float64(0),
		"hireTypeName":     "Social",
		"hurry":            false,
		"projectId":        float64(5),
		"projectName":      "Source group",
		"objectId":         "9",
		"objectName":       "General",
		"tagList":          []any{"Go", "Docker", "Go"},
		"description":      "<p>Build Go services</p>",
		"jobRequire":       "<p>Use Docker</p>",
		"addition":         "<p>Remote</p>",
		"status":           float64(1),
		"hadDelivery":      float64(0),
		"channelDetailIds": []any{float64(1)},
	}

	first := mustNormalizeObservation(t, list, base)
	if first.MetadataSHA256 == "" || first.BodySHA256 == "" || first.DescriptionSHA256 == "" || first.JobRequireSHA256 == "" || first.AdditionSHA256 == "" {
		t.Fatal("split versions must include all business and field hashes")
	}
	if first.BodyNormalizationVersion != BodyNormalizationVersion || first.MetadataNormalizationVersion != MetadataNormalizationVersion {
		t.Fatalf("normalization versions = body:%q metadata:%q", first.BodyNormalizationVersion, first.MetadataNormalizationVersion)
	}
	assertSourceTagsAreDeduplicated(t, first.MetadataPayload, 2)

	titleChanged := cloneMap(base)
	titleChanged["title"] = "Runtime Engineer"
	assertMetadataOnlyChange(t, first, mustNormalizeObservation(t, list, titleChanged), "title")

	reorderedLocations := cloneMap(list)
	reorderedLocations["addressDetailList"] = []any{
		map[string]any{"addressId": float64(10), "addressDetail": "Beijing"},
		map[string]any{"addressId": float64(20), "addressDetail": "Shanghai"},
	}
	if reordered := mustNormalizeObservation(t, reorderedLocations, base); reordered.MetadataSHA256 != first.MetadataSHA256 {
		t.Fatalf("location order changed metadata hash: %s != %s", reordered.MetadataSHA256, first.MetadataSHA256)
	}

	tagsChanged := cloneMap(base)
	tagsChanged["tagList"] = []any{"Docker", "Go", "Kubernetes"}
	assertMetadataOnlyChange(t, first, mustNormalizeObservation(t, list, tagsChanged), "source tags")

	bodyChanged := cloneMap(base)
	bodyChanged["addition"] = "<p>Hybrid</p>"
	changedBody := mustNormalizeObservation(t, list, bodyChanged)
	if changedBody.MetadataSHA256 != first.MetadataSHA256 || changedBody.BodySHA256 == first.BodySHA256 {
		t.Fatalf("body update changed wrong versions: metadata=%t body=%t", changedBody.MetadataSHA256 != first.MetadataSHA256, changedBody.BodySHA256 != first.BodySHA256)
	}

	volatileChanged := cloneMap(base)
	volatileChanged["status"] = float64(99)
	volatileChanged["hadDelivery"] = float64(1)
	volatileChanged["channelDetailIds"] = []any{float64(1), float64(7)}
	volatileChanged["archiveDigest"] = "not-a-business-field"
	volatile := mustNormalizeObservation(t, list, volatileChanged)
	if volatile.MetadataSHA256 != first.MetadataSHA256 || volatile.BodySHA256 != first.BodySHA256 {
		t.Fatal("volatile source fields must not create metadata or body versions")
	}
}

func TestNormalizeObservationPreservesTypedMetadataEmptyStates(t *testing.T) {
	list := map[string]any{"id": "1"}
	base := map[string]any{
		"id": "1", "title": "Synthetic", "description": "Description", "jobRequire": "Requirement",
	}
	absent := mustNormalizeObservation(t, list, base)

	nullValue := cloneMap(base)
	nullValue["objectName"] = nil
	null := mustNormalizeObservation(t, list, nullValue)

	emptyStringValue := cloneMap(base)
	emptyStringValue["objectName"] = ""
	emptyString := mustNormalizeObservation(t, list, emptyStringValue)

	emptyArrayValue := cloneMap(base)
	emptyArrayValue["tagList"] = []any{}
	emptyArray := mustNormalizeObservation(t, list, emptyArrayValue)

	nullArrayValue := cloneMap(base)
	nullArrayValue["tagList"] = nil
	nullArray := mustNormalizeObservation(t, list, nullArrayValue)

	invalidValue := cloneMap(base)
	invalidValue["objectName"] = []any{"unexpected"}
	invalid := mustNormalizeObservation(t, list, invalidValue)

	seen := map[string]struct{}{}
	for name, observation := range map[string]Observation{
		"absent": absent, "null": null, "empty_string": emptyString,
		"empty_array": emptyArray, "null_array": nullArray, "invalid": invalid,
	} {
		if _, duplicate := seen[observation.MetadataSHA256]; duplicate {
			t.Fatalf("typed source state %s reused another metadata version", name)
		}
		seen[observation.MetadataSHA256] = struct{}{}
		if observation.BodySHA256 != absent.BodySHA256 {
			t.Fatalf("metadata state %s unexpectedly changed body version", name)
		}
	}
}

func mustNormalizeObservation(t *testing.T, list, detail map[string]any) Observation {
	t.Helper()
	observation, err := NormalizeObservation(SourceCompanyMihoyo, list, detail, time.Date(2026, 10, 22, 2, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return observation
}

func assertMetadataOnlyChange(t *testing.T, before, after Observation, label string) {
	t.Helper()
	if before.MetadataSHA256 == after.MetadataSHA256 {
		t.Fatalf("%s did not change metadata version", label)
	}
	if before.BodySHA256 != after.BodySHA256 {
		t.Fatalf("%s unexpectedly changed body version", label)
	}
}

func assertSourceTagsAreDeduplicated(t *testing.T, payload json.RawMessage, want int) {
	t.Helper()
	var metadata map[string]any
	if err := json.Unmarshal(payload, &metadata); err != nil {
		t.Fatal(err)
	}
	tags, ok := metadata["source_tags"].(map[string]any)
	if !ok {
		t.Fatalf("source tags payload = %#v", metadata["source_tags"])
	}
	items, ok := tags["items"].([]any)
	if !ok || len(items) != want {
		t.Fatalf("source tags = %#v, want %d deduplicated entries", tags, want)
	}
}

func cloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
