package jdserving

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicJDDTOsExposeOnlyDisplaySafeFields(t *testing.T) {
	response := JobDetailResponse{
		ID:                  "jd_demo_opaque",
		Title:               "服务端工程师",
		Locations:           []string{"上海"},
		ActivityState:       ActivityStateEffective,
		FirstDiscoveredDate: "2026-10-30",
		IsUrgent:            true,
		MainJobFamily:       &FilterOption{Value: "backend", Label: "后端"},
		OtherJobFamilies:    []FilterOption{{Value: "ai_application", Label: "AI 应用"}},
		TechnologyKeywords:  []FilterOption{{Value: "go", Label: "Go"}},
		Education: &EducationRequirement{
			ExplicitRequirement: true,
			LowerBound:          EducationLevelBachelor,
		},
		Experience: &ExperienceRequirement{
			ExplicitRequirement: true,
			MinimumYears:        3,
		},
		Projects: []FilterOption{{Value: "genshin_impact", Label: "原神"}},
		Body: BodySections{
			Responsibilities: "负责服务端功能开发",
			Requirements:     "熟悉 Go",
		},
	}
	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal public JD detail: %v", err)
	}
	text := string(payload)
	for _, forbidden := range []string{
		"job_id", "source_job_id", "presence_period", "published_run", "observation_scope",
		"metadata_version", "body_content", "parser_version", "dictionary_version", "lexicon",
		"resolution", "selection_priority", "audit", "ai_semantic", "provider", "credential",
		"password", "token",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("public JD DTO leaked forbidden field vocabulary %q: %s", forbidden, text)
		}
	}
}

func TestPublicOpenAPIDoesNotDeclareInternalResponseProperties(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "openapi", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read OpenAPI contract: %v", err)
	}
	text := string(contents)
	for _, forbidden := range []string{
		"job_id:", "source_job_id:", "presence_period", "published_run", "observation_scope",
		"metadata_version", "body_content_version", "parser_version:", "dictionary_version:",
		"lexicon_version:", "audit_state", "ai_semantic", "provider:", "credential:", "password:", "token:",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("OpenAPI declares forbidden response field vocabulary %q", forbidden)
		}
	}
}
