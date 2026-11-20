package aitrend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	EnableThinking bool                     `json:"enable_thinking"`
	MaxTokens      int                      `json:"max_tokens"`
	Messages       []qwenTrendMessage       `json:"messages"`
	Model          string                   `json:"model"`
	ResponseFormat *qwenTrendResponseFormat `json:"response_format,omitempty"`
	Tools          []qwenTool               `json:"tools,omitempty"`
	Temperature    float64                  `json:"temperature"`
}

type qwenTrendMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCalls  []qwenToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Name       string         `json:"name,omitempty"`
}

type qwenTool struct {
	Type     string         `json:"type"`
	Function qwenToolSchema `json:"function"`
}

type qwenToolSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type qwenToolCall struct {
	ID       string              `json:"id"`
	Type     string              `json:"type"`
	Function qwenToolCallPayload `json:"function"`
}

type qwenToolCallPayload struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
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
			Content   string         `json:"content"`
			ToolCalls []qwenToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

const intentExplanationPrompt = `请用简体中文解释本次趋势草稿选择，说明控制在 80 个 Unicode 字符以内，必须完整结束，不能在句子中间截断。
只能使用用户问题、确定性的候选列表、排序指标和聚合数值。简短说明为什么这些曲线回答了问题，必要时说明扩展推荐或部分结果。
不得编造数据，不得输出 SQL、原始文本、内部键或思维链。只返回包含 explanation 字段的 JSON。`

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
		ResponseFormat: &qwenTrendResponseFormat{
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

// AnalyzeIntentTurn supports the bounded model-tool loop used by production
// trend analysis. The provider returns either function calls or the final
// structured Intent; the orchestrator executes and validates the calls.
func (provider *QwenIntentProvider) AnalyzeIntentTurn(ctx context.Context, query string, correction *ContractError, history []ToolConversation, allowTools bool) (IntentToolTurn, error) {
	if provider == nil || provider.apiKey == "" || provider.httpClient == nil || strings.TrimSpace(query) == "" {
		return IntentToolTurn{}, &ContractError{Code: ErrorConfigurationFailure, Message: "AI provider is not configured"}
	}
	messages := []qwenTrendMessage{
		{Role: "system", Content: IntentPromptV1()},
		{Role: "user", Content: toolLoopUserPrompt(query, correction, history)},
	}
	for _, exchange := range history {
		assistant := qwenTrendMessage{Role: "assistant"}
		for _, call := range exchange.Calls {
			assistant.ToolCalls = append(assistant.ToolCalls, qwenToolCall{
				ID: call.ID, Type: "function",
				Function: qwenToolCallPayload{Name: string(call.Name), Arguments: string(call.Params)},
			})
		}
		messages = append(messages, assistant)
		for index, call := range exchange.Calls {
			if index >= len(exchange.Results) {
				continue
			}
			result := exchange.Results[index]
			content := marshalToolResultForModel(result)
			messages = append(messages, qwenTrendMessage{Role: "tool", ToolCallID: call.ID, Content: string(content)})
		}
	}
	hasTrendSummary := historyContainsTool(history, ToolBatchTrendSummary) || historyContainsTool(history, ToolReadTrendSummary)
	requestBody := qwenTrendRequest{
		EnableThinking: false,
		MaxTokens:      1200,
		Messages:       messages,
		Model:          "qwen-plus",
		Temperature:    0,
	}
	if allowTools && !hasTrendSummary {
		requestBody.Tools = qwenModelTools()
	}
	// Keep discovery turns free to emit tool calls. Once the model has
	// received a trend summary, constrain the next response to the Intent
	// schema so provider-specific envelopes cannot leak into validation.
	if !allowTools || hasTrendSummary {
		requestBody.ResponseFormat = &qwenTrendResponseFormat{
			Type:       "json_schema",
			JSONSchema: qwenTrendJSONSchema{Name: ContractVersionV1, Strict: true, Schema: json.RawMessage(IntentSchemaV1())},
		}
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return IntentToolTurn{}, &ContractError{Code: ErrorProviderFailure, Message: "AI 请求未能生成"}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.endpoint, bytes.NewReader(body))
	if err != nil {
		return IntentToolTurn{}, &ContractError{Code: ErrorProviderFailure, Message: "AI 请求未能生成"}
	}
	request.Header.Set("Authorization", "Bearer "+provider.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := provider.httpClient.Do(request)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return IntentToolTurn{}, err
		}
		return IntentToolTurn{}, &ContractError{Code: ErrorProviderFailure, Message: "AI 服务暂时不可用"}
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return IntentToolTurn{}, &ContractError{Code: ErrorProviderFailure, Message: "AI 服务暂时不可用"}
	}
	var payload qwenTrendResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&payload); err != nil || len(payload.Choices) != 1 {
		return IntentToolTurn{}, &ContractError{Code: ErrorProviderFailure, Message: "AI 返回内容无效"}
	}
	if payload.Usage.PromptTokens < 0 || payload.Usage.CompletionTokens < 0 || payload.Usage.TotalTokens < 0 {
		return IntentToolTurn{}, &ContractError{Code: ErrorProviderFailure, Message: "AI 返回用量无效"}
	}
	message := payload.Choices[0].Message
	if len(message.ToolCalls) > 0 {
		if strings.TrimSpace(message.Content) != "" {
			return IntentToolTurn{}, &ContractError{Code: ErrorInvalidCombination, Field: "model_response", Message: "AI 同时返回了工具调用和最终内容"}
		}
		calls := make([]ToolCall, 0, len(message.ToolCalls))
		for index, toolCall := range message.ToolCalls {
			callID := strings.TrimSpace(toolCall.ID)
			if callID == "" {
				callID = fmt.Sprintf("tool-call-%d", index+1)
			}
			calls = append(calls, ToolCall{ID: callID, Name: ToolName(strings.TrimSpace(toolCall.Function.Name)), Params: json.RawMessage(toolCall.Function.Arguments)})
		}
		return IntentToolTurn{ToolCalls: calls, InputTokens: payload.Usage.PromptTokens, OutputTokens: payload.Usage.CompletionTokens}, nil
	}
	content := strings.TrimSpace(message.Content)
	if content == "" || len(content) > MaxToolOutputBytes {
		return IntentToolTurn{}, &ContractError{Code: ErrorProviderFailure, Message: "AI 返回内容无效"}
	}
	content = string(unwrapIntentEnvelope([]byte(content)))
	return IntentToolTurn{Intent: []byte(content), InputTokens: payload.Usage.PromptTokens, OutputTokens: payload.Usage.CompletionTokens}, nil
}

func qwenModelTools() []qwenTool {
	descriptors := trendToolDescriptors()
	tools := make([]qwenTool, 0, len(descriptors))
	for _, descriptor := range descriptors {
		tools = append(tools, qwenTool{Type: "function", Function: qwenToolSchema{Name: string(descriptor.Name), Description: descriptor.Description, Parameters: descriptor.Parameters}})
	}
	return tools
}

func toolLoopUserPrompt(query string, correction *ContractError, history []ToolConversation) string {
	value := strings.TrimSpace(query)
	if correction != nil {
		value += "\n\n修正上一版输出：字段 " + correction.Field + " 无效（" + correction.Message + "）。请只返回修正后的 JSON。"
	}
	if historyContainsTool(history, ToolBatchTrendSummary) || historyContainsTool(history, ToolReadTrendSummary) {
		return value + "\n\n已收到趋势摘要结果。现在只返回符合 Schema 的最终 Intent 对象，不要再调用工具。"
	}
	if len(history) > 0 {
		return value + "\n\n候选范围已经通过工具确认。返回最终 Intent 前，必须先调用 batch_trend_summary 获取这些候选的每日活跃岗位数量摘要，用于按用户要求排序；工具调用阶段不要输出最终 Intent。"
	}
	return value
}

func historyContainsTool(history []ToolConversation, name ToolName) bool {
	for _, exchange := range history {
		for _, call := range exchange.Calls {
			if call.Name == name {
				return true
			}
		}
	}
	return false
}

func unwrapIntentEnvelope(content []byte) []byte {
	var envelope struct {
		Intent json.RawMessage `json:"intent"`
		Type   string          `json:"type"`
	}
	if err := json.Unmarshal(content, &envelope); err != nil {
		return content
	}
	if len(envelope.Intent) > 0 && string(envelope.Intent) != "null" {
		var encoded string
		if json.Unmarshal(envelope.Intent, &encoded) == nil {
			return []byte(encoded)
		}
		return bytes.TrimSpace(envelope.Intent)
	}
	if envelope.Type == "final" {
		var fields map[string]json.RawMessage
		if json.Unmarshal(content, &fields) == nil {
			delete(fields, "type")
			if normalized, err := json.Marshal(fields); err == nil {
				return normalized
			}
		}
	}
	return content
}

func marshalToolResultForModel(result ToolCallResult) []byte {
	if result.SafeError != nil && len(result.Payload) == 0 {
		data, _ := json.Marshal(struct {
			Error *SafeError `json:"error"`
		}{Error: result.SafeError})
		return data
	}
	if result.SafeError != nil {
		data, _ := json.Marshal(struct {
			Result json.RawMessage `json:"result"`
			Error  *SafeError      `json:"error"`
		}{Result: result.Payload, Error: result.SafeError})
		return data
	}
	if len(result.Payload) == 0 {
		return []byte(`{"error":{"code":"backend_failure","message":"tool returned no result"}}`)
	}
	return result.Payload
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
		MaxTokens:      200,
		Messages: []qwenTrendMessage{
			{Role: "system", Content: intentExplanationPrompt},
			{Role: "user", Content: string(input)},
		},
		Model: "qwen-plus",
		ResponseFormat: &qwenTrendResponseFormat{
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
	if !containsChinese(explanation) {
		return "", 0, 0, &ContractError{Code: ErrorInvalidCombination, Field: "explanation", Message: "AI 说明必须使用中文"}
	}
	if utf8.RuneCountInString(explanation) > MaxExplanationRunes {
		return "", 0, 0, &ContractError{Code: ErrorLimitExceeded, Field: "explanation", Message: "AI 说明超过长度限制"}
	}
	if payload.Usage.PromptTokens < 0 || payload.Usage.CompletionTokens < 0 || payload.Usage.TotalTokens < 0 {
		return "", 0, 0, &ContractError{Code: ErrorProviderFailure, Message: "AI 返回用量无效"}
	}
	return explanation, payload.Usage.PromptTokens, payload.Usage.CompletionTokens, nil
}

func containsChinese(value string) bool {
	for _, r := range value {
		if (r >= '\u3400' && r <= '\u4dbf') || (r >= '\u4e00' && r <= '\u9fff') {
			return true
		}
	}
	return false
}
