package aitrend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"mrolescope/internal/jdanalysis"
)

// IntentProvider is the only model boundary used by the trend runner. It
// receives the user's trend question and returns a transient, structured
// intent. The caller is responsible for validating the returned contract.
type IntentProvider interface {
	AnalyzeIntent(context.Context, string, *ContractError) ([]byte, error)
}

// IntentUsageProvider is an optional extension used by the runner to retain
// provider-reported token usage without changing the existing test seam.
type IntentUsageProvider interface {
	AnalyzeIntentUsage(context.Context, string, *ContractError) ([]byte, int, int, error)
}

// QwenIntentProvider is a fixed Qwen Plus adapter for the trend intent
// contract. It does not expose a caller-selected URL, model, or arbitrary
// prompt and never logs request text, provider responses, or credentials.
type QwenIntentProvider struct {
	apiKey     string
	endpoint   string
	httpClient *http.Client
}

func NewQwenIntentProvider(apiKey, workspaceID string, httpClient *http.Client) (*QwenIntentProvider, error) {
	if strings.TrimSpace(apiKey) == "" || httpClient == nil {
		return nil, &ContractError{Code: ErrorConfigurationFailure, Message: "AI provider is not configured"}
	}
	baseURL, err := jdanalysis.QwenBeijingWorkspaceCompatibleBaseURL(strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, &ContractError{Code: ErrorConfigurationFailure, Message: "AI provider workspace is invalid"}
	}
	return &QwenIntentProvider{
		apiKey:     apiKey,
		endpoint:   strings.TrimRight(baseURL, "/") + "/chat/completions",
		httpClient: httpClient,
	}, nil
}

type qwenTrendRequest struct {
	EnableThinking bool                    `json:"enable_thinking"`
	MaxTokens      int                     `json:"max_tokens"`
	Messages       []qwenTrendMessage      `json:"messages"`
	Model          string                  `json:"model"`
	ResponseFormat qwenTrendResponseFormat `json:"response_format"`
	Temperature    float64                 `json:"temperature"`
}

type qwenTrendMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type qwenTrendResponseFormat struct {
	Type       string              `json:"type"`
	JSONSchema qwenTrendJSONSchema `json:"json_schema"`
}

type qwenTrendJSONSchema struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

type qwenTrendResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

const intentExplanationPrompt = `Explain the completed trend-draft selection for the user in no more than 100 Unicode characters.
Use only the supplied question, deterministic candidate list, ranking metric, and aggregate values. Mention why the selected curves answer the question and any broad/partial matching when useful.
Do not invent data, do not output SQL or source text, do not reveal internal keys, and do not show chain-of-thought. Return JSON with exactly one explanation field.`

var intentExplanationSchema = json.RawMessage(`{"type":"object","properties":{"explanation":{"type":"string","maxLength":100}},"required":["explanation"],"additionalProperties":false}`)

func (provider *QwenIntentProvider) AnalyzeIntent(ctx context.Context, query string, correction *ContractError) ([]byte, error) {
	content, _, _, err := provider.AnalyzeIntentUsage(ctx, query, correction)
	return content, err
}

func (provider *QwenIntentProvider) AnalyzeIntentUsage(ctx context.Context, query string, correction *ContractError) ([]byte, int, int, error) {
	if provider == nil || provider.apiKey == "" || provider.httpClient == nil || strings.TrimSpace(query) == "" {
		return nil, 0, 0, &ContractError{Code: ErrorConfigurationFailure, Message: "AI provider is not configured"}
	}
	userPrompt := strings.TrimSpace(query)
	if correction != nil {
		// ContractError messages are server-authored and bounded. They are the
		// only feedback sent back to the model after a validation failure.
		userPrompt += "\n\n修正上一版输出：字段 " + correction.Field + " 无效（" + correction.Message + "）。请只返回修正后的 JSON。"
	}
	body, err := json.Marshal(qwenTrendRequest{
		EnableThinking: false,
		MaxTokens:      1200,
		Messages: []qwenTrendMessage{
			{Role: "system", Content: IntentPromptV1()},
			{Role: "user", Content: userPrompt},
		},
		Model: "qwen-plus",
		ResponseFormat: qwenTrendResponseFormat{
			Type:       "json_schema",
			JSONSchema: qwenTrendJSONSchema{Name: ContractVersionV1, Strict: true, Schema: json.RawMessage(IntentSchemaV1())},
		},
		Temperature: 0,
	})
	if err != nil {
		return nil, 0, 0, &ContractError{Code: ErrorProviderFailure, Message: "AI 请求未能生成"}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, 0, &ContractError{Code: ErrorProviderFailure, Message: "AI 请求未能生成"}
	}
	request.Header.Set("Authorization", "Bearer "+provider.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := provider.httpClient.Do(request)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, 0, 0, err
		}
		return nil, 0, 0, &ContractError{Code: ErrorProviderFailure, Message: "AI 服务暂时不可用"}
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, 0, 0, &ContractError{Code: ErrorProviderFailure, Message: "AI 服务暂时不可用"}
	}
	var payload qwenTrendResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&payload); err != nil || len(payload.Choices) != 1 || strings.TrimSpace(payload.Choices[0].Message.Content) == "" {
		return nil, 0, 0, &ContractError{Code: ErrorProviderFailure, Message: "AI 返回内容无效"}
	}
	content := strings.TrimSpace(payload.Choices[0].Message.Content)
	if len(content) > MaxToolOutputBytes {
		return nil, 0, 0, &ContractError{Code: ErrorLimitExceeded, Field: "ai_output", Message: "AI 输出超出限制"}
	}
	if payload.Usage.PromptTokens < 0 || payload.Usage.CompletionTokens < 0 || payload.Usage.TotalTokens < 0 {
		return nil, 0, 0, &ContractError{Code: ErrorProviderFailure, Message: "AI 返回用量无效"}
	}
	return []byte(content), payload.Usage.PromptTokens, payload.Usage.CompletionTokens, nil
}

func (provider *QwenIntentProvider) ExplainDraftUsage(ctx context.Context, query string, explanationContext DraftExplanationContext) (string, int, int, error) {
	if provider == nil || provider.apiKey == "" || provider.httpClient == nil || strings.TrimSpace(query) == "" {
		return "", 0, 0, &ContractError{Code: ErrorConfigurationFailure, Message: "AI provider is not configured"}
	}
	input, err := json.Marshal(struct {
		Query   string                  `json:"query"`
		Context DraftExplanationContext `json:"context"`
	}{Query: strings.TrimSpace(query), Context: explanationContext})
	if err != nil {
		return "", 0, 0, &ContractError{Code: ErrorProviderFailure, Message: "AI 请求未能生成"}
	}
	body, err := json.Marshal(qwenTrendRequest{
		EnableThinking: false,
		MaxTokens:      160,
		Messages: []qwenTrendMessage{
			{Role: "system", Content: intentExplanationPrompt},
			{Role: "user", Content: string(input)},
		},
		Model: "qwen-plus",
		ResponseFormat: qwenTrendResponseFormat{
			Type:       "json_schema",
			JSONSchema: qwenTrendJSONSchema{Name: "ai-trend-explanation-v1", Strict: true, Schema: intentExplanationSchema},
		},
		Temperature: 0,
	})
	if err != nil {
		return "", 0, 0, &ContractError{Code: ErrorProviderFailure, Message: "AI 请求未能生成"}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", 0, 0, &ContractError{Code: ErrorProviderFailure, Message: "AI 请求未能生成"}
	}
	request.Header.Set("Authorization", "Bearer "+provider.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := provider.httpClient.Do(request)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", 0, 0, err
		}
		return "", 0, 0, &ContractError{Code: ErrorProviderFailure, Message: "AI 服务暂时不可用"}
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", 0, 0, &ContractError{Code: ErrorProviderFailure, Message: "AI 服务暂时不可用"}
	}
	var payload qwenTrendResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&payload); err != nil || len(payload.Choices) != 1 || strings.TrimSpace(payload.Choices[0].Message.Content) == "" {
		return "", 0, 0, &ContractError{Code: ErrorProviderFailure, Message: "AI 返回内容无效"}
	}
	var result struct {
		Explanation string `json:"explanation"`
	}
	if err := decodeStrict([]byte(payload.Choices[0].Message.Content), &result); err != nil {
		return "", 0, 0, &ContractError{Code: ErrorInvalidJSON, Field: "explanation", Message: "AI 说明格式无效"}
	}
	explanation := strings.TrimSpace(result.Explanation)
	if explanation == "" {
		return "", 0, 0, &ContractError{Code: ErrorMissingField, Field: "explanation", Message: "AI 说明为空"}
	}
	if utf8.RuneCountInString(explanation) > MaxExplanationRunes {
		return "", 0, 0, &ContractError{Code: ErrorLimitExceeded, Field: "explanation", Message: "AI 说明超过长度限制"}
	}
	if payload.Usage.PromptTokens < 0 || payload.Usage.CompletionTokens < 0 || payload.Usage.TotalTokens < 0 {
		return "", 0, 0, &ContractError{Code: ErrorProviderFailure, Message: "AI 返回用量无效"}
	}
	return explanation, payload.Usage.PromptTokens, payload.Usage.CompletionTokens, nil
}
