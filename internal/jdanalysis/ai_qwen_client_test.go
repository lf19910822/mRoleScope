package jdanalysis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQwenClientSendsFrozenStrictProfileAndReadsUsage(t *testing.T) {
	input := syntheticSemanticModelInput(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/chat/completions" {
			t.Fatalf("request = %s %s, want POST /chat/completions", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer synthetic-test-key" {
			t.Fatal("Qwen request did not carry the supplied credential")
		}
		var payload struct {
			EnableThinking bool `json:"enable_thinking"`
			MaxTokens      int  `json:"max_tokens"`
			Messages       []struct {
				Content string `json:"content"`
				Role    string `json:"role"`
			} `json:"messages"`
			Model          string `json:"model"`
			ResponseFormat struct {
				JSONSchema struct {
					Name   string          `json:"name"`
					Schema json.RawMessage `json:"schema"`
					Strict bool            `json:"strict"`
				} `json:"json_schema"`
				Type string `json:"type"`
			} `json:"response_format"`
			Temperature float64 `json:"temperature"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Model != "qwen-plus" || payload.MaxTokens != 2500 || payload.Temperature != 0 || !payload.ResponseFormat.JSONSchema.Strict || payload.ResponseFormat.JSONSchema.Name != AIResponseSchemaVersionV1 || payload.ResponseFormat.Type != "json_schema" {
			t.Fatal("Qwen request did not use the fixed Plus structured-output profile")
		}
		if payload.EnableThinking {
			t.Fatal("Qwen request did not explicitly disable thinking")
		}
		if len(payload.Messages) != 2 || payload.Messages[0].Role != "system" || payload.Messages[1].Role != "user" || payload.Messages[0].Content != AISystemPromptV3() || !json.Valid(payload.ResponseFormat.JSONSchema.Schema) {
			t.Fatal("Qwen request did not carry the canonical prompt/schema pair")
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"{\"response_schema_version\":\"ai-jd-result-v1\"}"}}],"usage":{"prompt_tokens":12,"completion_tokens":8,"total_tokens":20}}`))
	}))
	defer server.Close()

	profile, err := QwenPlusTrialProfileForBeijingWorkspace("llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	client, err := newQwenClientForEndpoint("synthetic-test-key", server.Client(), profile, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Analyze(context.Background(), input)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if string(response.Content) != `{"response_schema_version":"ai-jd-result-v1"}` || response.Usage.InputTokens != 12 || response.Usage.OutputTokens != 8 || response.Usage.TotalTokens != 20 {
		t.Fatalf("Qwen response did not preserve the expected structured-output and usage summary")
	}
	flashProfile, err := QwenFlashComparisonProfileForBeijingWorkspace("llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	if flashProfile.Model() != "qwen-flash" || flashProfile.MaxOutputTokens() != 2500 || flashProfile.ThinkingEnabled() {
		t.Fatalf("Flash comparison profile = %#v, want fixed non-thinking 2,500-token profile", profile)
	}
}

func TestQwenClientClassifiesProviderFailuresWithoutLeakingContent(t *testing.T) {
	const (
		syntheticBody   = "SYNTHETIC_BODY_MUST_NOT_LEAK"
		syntheticSecret = "SYNTHETIC_SECRET_MUST_NOT_LEAK"
	)
	input := syntheticSemanticModelInput(t)
	input.Fields[0].Spans[0].Text = syntheticBody

	for _, test := range []struct {
		name   string
		status int
		want   AISafeErrorCode
	}{
		{name: "authentication", status: http.StatusUnauthorized, want: AISafeErrorProviderAuthentication},
		{name: "rate limited", status: http.StatusTooManyRequests, want: AISafeErrorProviderRateLimited},
		{name: "server", status: http.StatusBadGateway, want: AISafeErrorProviderServer},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(syntheticBody + syntheticSecret))
			}))
			defer server.Close()

			profile, err := QwenPlusTrialProfileForBeijingWorkspace("llm-mrolescope")
			if err != nil {
				t.Fatal(err)
			}
			client, err := newQwenClientForEndpoint(syntheticSecret, server.Client(), profile, server.URL)
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Analyze(context.Background(), input)
			if err == nil || !strings.Contains(err.Error(), string(test.want)) {
				t.Fatalf("provider error = %v, want safe %q classification", err, test.want)
			}
			if strings.Contains(err.Error(), syntheticBody) || strings.Contains(err.Error(), syntheticSecret) {
				t.Fatalf("provider error leaked request/response content: %q", err)
			}
		})
	}
}

func TestQwenClientRejectsMissingCredentialWithoutEchoingIt(t *testing.T) {
	client, err := NewQwenPlusClient("", http.DefaultClient, "llm-mrolescope")
	if client != nil || err == nil || !strings.Contains(err.Error(), string(AISafeErrorConfigurationInvalid)) {
		t.Fatalf("missing credential result = client:%v error:%v", client != nil, err)
	}
}

func TestQwenBeijingWorkspaceProfileUsesDedicatedEndpointAndFreezesIt(t *testing.T) {
	profile, err := QwenPlusTrialProfileForBeijingWorkspace("llm-mrolescope")
	if err != nil {
		t.Fatalf("create Beijing Workspace profile: %v", err)
	}
	if profile.baseURL != "https://llm-mrolescope.cn-beijing.maas.aliyuncs.com/compatible-mode/v1" {
		t.Fatalf("base URL = %q", profile.baseURL)
	}

	otherWorkspace, err := QwenPlusTrialProfileForBeijingWorkspace("llm-other")
	if err != nil {
		t.Fatalf("create second Beijing Workspace profile: %v", err)
	}
	if profile.SHA256() == otherWorkspace.SHA256() {
		t.Fatal("different Workspace endpoints must produce different frozen request profiles")
	}

	for _, workspaceID := range []string{"", "llm.mrolescope", "https://untrusted.example", "llm_mrolescope"} {
		if _, err := QwenPlusTrialProfileForBeijingWorkspace(workspaceID); err == nil {
			t.Fatalf("invalid Workspace ID %q was accepted", workspaceID)
		}
	}
}
