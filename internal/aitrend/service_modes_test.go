package aitrend

import "testing"

func TestNormalizeServiceTagsDeduplicatesAndOrders(t *testing.T) {
	tags, err := NormalizeServiceTags([]ServiceTag{ServiceTagGuidance, ServiceTagTrend})
	if err != nil || len(tags) != 2 || tags[0] != ServiceTagTrend || tags[1] != ServiceTagGuidance {
		t.Fatalf("tags=%v err=%v", tags, err)
	}
}

func TestNormalizeServiceTagsRejectsInvalidShapes(t *testing.T) {
	for name, tags := range map[string][]ServiceTag{
		"empty":   nil,
		"unknown": {ServiceTag("export")},
		"repeat":  {ServiceTagTrend, ServiceTagTrend},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NormalizeServiceTags(tags); err == nil {
				t.Fatal("invalid tags were accepted")
			}
		})
	}
}

func TestDecodeServiceModeClassificationValidatesContract(t *testing.T) {
	classification, err := decodeServiceModeClassification([]byte(`{"contract_version":"ai-trend-service-mode-v1","service_tags":["guidance","trend"]}`))
	if err != nil || len(classification.ServiceTags) != 2 || classification.ServiceTags[0] != ServiceTagTrend {
		t.Fatalf("classification=%#v err=%v", classification, err)
	}
	if _, err := decodeServiceModeClassification([]byte(`{"contract_version":"ai-trend-service-mode-v1","service_tags":["export"]}`)); err == nil {
		t.Fatal("unknown service tag was accepted")
	}
}

func TestResponseModesFromServiceTags(t *testing.T) {
	modes, err := ResponseModesFromServiceTags([]ServiceTag{ServiceTagGuidance})
	if err != nil || len(modes) != 1 || modes[0] != ResponseModeGuidance {
		t.Fatalf("modes=%v err=%v", modes, err)
	}
}
