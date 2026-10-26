package jdanalysis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	qwenBeijingWorkspaceCompatiblePath   = "/compatible-mode/v1"
	qwenBeijingWorkspaceCompatibleSuffix = ".cn-beijing.maas.aliyuncs.com"
)

var qwenWorkspaceIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

// ControlledAIModelClient is the replaceable provider seam. Implementations
// receive only a prebuilt transient AIModelInput and must return no logs,
// persistence commands, or raw provider diagnostics.
type ControlledAIModelClient interface {
	Analyze(context.Context, AIModelInput) (AIModelResponse, error)
}

// AIProviderCompletionSignal is the bounded, provider-neutral completion
// indicator retained with an attempt. It deliberately cannot hold a provider
// supplied value or provider text.
type AIProviderCompletionSignal string

const (
	AIProviderCompletionSignalUnknown AIProviderCompletionSignal = "unknown"
	AIProviderCompletionSignalStop    AIProviderCompletionSignal = "stop"
	AIProviderCompletionSignalLength  AIProviderCompletionSignal = "length"
	AIProviderCompletionSignalOther   AIProviderCompletionSignal = "other"
)

func (signal AIProviderCompletionSignal) valid() bool {
	switch signal {
	case AIProviderCompletionSignalUnknown,
		AIProviderCompletionSignalStop,
		AIProviderCompletionSignalLength,
		AIProviderCompletionSignalOther:
		return true
	default:
		return false
	}
}

func normalizeQwenCompletionSignal(value string) AIProviderCompletionSignal {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return AIProviderCompletionSignalUnknown
	case "stop":
		return AIProviderCompletionSignalStop
	case "length":
		return AIProviderCompletionSignalLength
	default:
		return AIProviderCompletionSignalOther
	}
}

// AIModelResponse is transient provider output. Content remains in memory only
// until the strict response validator consumes it. The completion signal and
// byte length are bounded redacted metadata suitable for the durable ledger.
type AIModelResponse struct {
	CompletionSignal    AIProviderCompletionSignal
	Content             []byte
	ResponseLengthBytes int
	Usage               AIModelUsage
}

// AIModelUsage is the bounded, redacted token accounting returned by a model.
type AIModelUsage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

// AIProviderFailure is a safe provider classification for retry and ledger
// policy. It deliberately omits raw response content, request text, and keys.
type AIProviderFailure struct {
	Code       AISafeErrorCode
	RetryAfter time.Duration
}

func (failure AIProviderFailure) Error() string {
	code := failure.Code
	if !code.valid() {
		code = AISafeErrorConfigurationInvalid
	}
	return fmt.Sprintf("AI semantic analysis failed: %s", code)
}

// QwenClient is a narrow OpenAI-compatible Bailian adapter. Its constructor
// only accepts a fixed profile; it has no caller-provided prompt, schema,
// model, price, body ID, or logging hook.
type QwenClient struct {
	apiKey     string
	httpClient *http.Client
	profile    QwenRequestProfile
	endpoint   string
}

var _ ControlledAIModelClient = (*QwenClient)(nil)

// QwenRequestProfile is a frozen model request profile. Its fields are private
// so an operation cannot turn a fixed Plus/Flash profile into an arbitrary
// model call.
type QwenRequestProfile struct {
	baseURL         string
	enableThinking  bool
	maxInputTokens  int
	maxOutputTokens int
	model           string
}

// Model returns the fixed model identifier for safe configuration summaries.
func (profile QwenRequestProfile) Model() string {
	return profile.model
}

// MaxInputTokens returns the frozen per-request input limit.
func (profile QwenRequestProfile) MaxInputTokens() int {
	return profile.maxInputTokens
}

// MaxOutputTokens returns the frozen per-request output limit.
func (profile QwenRequestProfile) MaxOutputTokens() int {
	return profile.maxOutputTokens
}

// ThinkingEnabled reports the fixed reasoning mode; v0.1 always disables it.
func (profile QwenRequestProfile) ThinkingEnabled() bool {
	return profile.enableThinking
}

// SHA256 identifies every fixed, non-secret request setting. It is persisted
// with a candidate configuration so a later worker cannot silently exchange a
// reviewed Plus/Flash request profile for a different provider invocation.
func (profile QwenRequestProfile) SHA256() string {
	encoded, err := json.Marshal(struct {
		BaseURL         string `json:"base_url"`
		EnableThinking  bool   `json:"enable_thinking"`
		MaxInputTokens  int    `json:"max_input_tokens"`
		MaxOutputTokens int    `json:"max_output_tokens"`
		Model           string `json:"model"`
	}{
		BaseURL:         profile.baseURL,
		EnableThinking:  profile.enableThinking,
		MaxInputTokens:  profile.maxInputTokens,
		MaxOutputTokens: profile.maxOutputTokens,
		Model:           profile.model,
	})
	if err != nil {
		return ""
	}
	return sha256Hex(string(encoded))
}

// QwenBeijingWorkspaceCompatibleBaseURL constructs the only production
// provider origin permitted for the controlled trial. It accepts one DNS-label
// Workspace ID and never permits a caller to select a region, host, scheme, or
// path.
func QwenBeijingWorkspaceCompatibleBaseURL(workspaceID string) (string, error) {
	if !qwenWorkspaceIDPattern.MatchString(workspaceID) {
		return "", NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	return "https://" + workspaceID + qwenBeijingWorkspaceCompatibleSuffix + qwenBeijingWorkspaceCompatiblePath, nil
}

// NewQwenFlashComparisonClient constructs the fixed Flash comparison adapter
// for one reviewed Beijing Workspace.
func NewQwenFlashComparisonClient(apiKey string, httpClient *http.Client, workspaceID string) (*QwenClient, error) {
	profile, err := QwenFlashComparisonProfileForBeijingWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	return newQwenClientForEndpoint(apiKey, httpClient, profile, profile.baseURL)
}

// NewQwenPlusClient constructs the fixed non-thinking Plus baseline adapter
// for one reviewed Beijing Workspace.
func NewQwenPlusClient(apiKey string, httpClient *http.Client, workspaceID string) (*QwenClient, error) {
	profile, err := QwenPlusTrialProfileForBeijingWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	return newQwenClientForEndpoint(apiKey, httpClient, profile, profile.baseURL)
}

// QwenFlashComparisonProfileForBeijingWorkspace is the immutable low-cost
// comparison profile; it is not a default production-routing choice.
func QwenFlashComparisonProfileForBeijingWorkspace(workspaceID string) (QwenRequestProfile, error) {
	baseURL, err := QwenBeijingWorkspaceCompatibleBaseURL(workspaceID)
	if err != nil {
		return QwenRequestProfile{}, err
	}
	return QwenRequestProfile{
		baseURL:         baseURL,
		enableThinking:  false,
		maxInputTokens:  10000,
		maxOutputTokens: 5000,
		model:           "qwen-flash",
	}, nil
}

// QwenPlusTrialProfileForBeijingWorkspace is the immutable v0.1 Plus baseline
// profile. Any token, model, or provider Workspace change belongs to a new
// immutable AI configuration.
func QwenPlusTrialProfileForBeijingWorkspace(workspaceID string) (QwenRequestProfile, error) {
	baseURL, err := QwenBeijingWorkspaceCompatibleBaseURL(workspaceID)
	if err != nil {
		return QwenRequestProfile{}, err
	}
	return QwenRequestProfile{
		baseURL:         baseURL,
		enableThinking:  false,
		maxInputTokens:  10000,
		maxOutputTokens: 5000,
		model:           "qwen-plus",
	}, nil
}

// Analyze sends one prebuilt, controlled input and returns transient structured
// output plus usage. It never logs request/response text or credentials.
func (client *QwenClient) Analyze(ctx context.Context, input AIModelInput) (AIModelResponse, error) {
	if client == nil || client.apiKey == "" || client.httpClient == nil || input.RequestID == "" || input.InputContractVersion != AIInputContractVersionV1 {
		return AIModelResponse{}, AIProviderFailure{Code: AISafeErrorConfigurationInvalid}
	}
	assets, err := responseContractAssets(input.responseContract.responseSchemaVersion)
	if err != nil {
		return AIModelResponse{}, AIProviderFailure{Code: AISafeErrorConfigurationInvalid}
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return AIModelResponse{}, AIProviderFailure{Code: AISafeErrorConfigurationInvalid}
	}
	requestBody := qwenChatRequest{
		EnableThinking: client.profile.enableThinking,
		MaxTokens:      client.profile.maxOutputTokens,
		Messages: []qwenChatMessage{
			{Content: assets.prompt, Role: "system"},
			{Content: string(inputJSON), Role: "user"},
		},
		Model: client.profile.model,
		ResponseFormat: qwenResponseFormat{
			JSONSchema: qwenJSONSchema{
				Name:   input.responseContract.responseSchemaVersion,
				Schema: json.RawMessage(assets.schema),
				Strict: true,
			},
			Type: "json_schema",
		},
		Temperature: 0,
	}
	encodedRequest, err := json.Marshal(requestBody)
	if err != nil {
		return AIModelResponse{}, AIProviderFailure{Code: AISafeErrorConfigurationInvalid}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, bytes.NewReader(encodedRequest))
	if err != nil {
		return AIModelResponse{}, AIProviderFailure{Code: AISafeErrorConfigurationInvalid}
	}
	request.Header.Set("Authorization", "Bearer "+client.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return AIModelResponse{}, AIProviderFailure{Code: classifyTransportError(err)}
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return AIModelResponse{}, AIProviderFailure{
			Code:       classifyQwenStatus(response.StatusCode),
			RetryAfter: parseRetryAfter(response.Header.Get("Retry-After"), time.Now()),
		}
	}
	var payload qwenChatResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&payload); err != nil || len(payload.Choices) != 1 || payload.Choices[0].Message.Content == "" || !payload.Usage.valid() {
		return AIModelResponse{}, AIProviderFailure{Code: AISafeErrorInvalidResponseSchema}
	}
	return AIModelResponse{
		CompletionSignal:    normalizeQwenCompletionSignal(payload.Choices[0].FinishReason),
		Content:             []byte(payload.Choices[0].Message.Content),
		ResponseLengthBytes: len([]byte(payload.Choices[0].Message.Content)),
		Usage: AIModelUsage{
			InputTokens:  payload.Usage.PromptTokens,
			OutputTokens: payload.Usage.CompletionTokens,
			TotalTokens:  payload.Usage.TotalTokens,
		},
	}, nil
}

func classifyQwenStatus(statusCode int) AISafeErrorCode {
	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return AISafeErrorProviderAuthentication
	case statusCode == http.StatusTooManyRequests:
		return AISafeErrorProviderRateLimited
	case statusCode >= http.StatusInternalServerError:
		return AISafeErrorProviderServer
	default:
		return AISafeErrorConfigurationInvalid
	}
}

func classifyTransportError(err error) AISafeErrorCode {
	if errors.Is(err, context.DeadlineExceeded) {
		return AISafeErrorProviderTimeout
	}
	var networkError interface{ Timeout() bool }
	if errors.As(err, &networkError) && networkError.Timeout() {
		return AISafeErrorProviderTimeout
	}
	return AISafeErrorProviderServer
}

func newQwenClientForEndpoint(apiKey string, httpClient *http.Client, profile QwenRequestProfile, baseURL string) (*QwenClient, error) {
	if apiKey == "" || httpClient == nil || !validQwenProfile(profile) {
		return nil, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	parsedURL, err := url.Parse(baseURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	parsedURL.Path = strings.TrimRight(parsedURL.Path, "/") + "/chat/completions"
	return &QwenClient{apiKey: apiKey, endpoint: parsedURL.String(), httpClient: httpClient, profile: profile}, nil
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if retryAt, err := http.ParseTime(value); err == nil && retryAt.After(now) {
		return retryAt.Sub(now)
	}
	return 0
}

func validQwenProfile(profile QwenRequestProfile) bool {
	baseURL, err := QwenBeijingWorkspaceCompatibleBaseURL(strings.TrimSuffix(strings.TrimPrefix(profile.baseURL, "https://"), qwenBeijingWorkspaceCompatibleSuffix+qwenBeijingWorkspaceCompatiblePath))
	return err == nil && profile.baseURL == baseURL && !profile.enableThinking && profile.maxInputTokens == 10000 && profile.maxOutputTokens == 5000 && (profile.model == "qwen-plus" || profile.model == "qwen-flash")
}

type qwenChatMessage struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

type qwenChatRequest struct {
	EnableThinking bool               `json:"enable_thinking"`
	MaxTokens      int                `json:"max_tokens"`
	Messages       []qwenChatMessage  `json:"messages"`
	Model          string             `json:"model"`
	ResponseFormat qwenResponseFormat `json:"response_format"`
	Temperature    float64            `json:"temperature"`
}

type qwenChatResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage qwenUsage `json:"usage"`
}

type qwenJSONSchema struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
	Strict bool            `json:"strict"`
}

type qwenResponseFormat struct {
	JSONSchema qwenJSONSchema `json:"json_schema"`
	Type       string         `json:"type"`
}

type qwenUsage struct {
	CompletionTokens int `json:"completion_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (usage qwenUsage) valid() bool {
	return usage.PromptTokens >= 0 && usage.CompletionTokens >= 0 && usage.TotalTokens >= 0
}
