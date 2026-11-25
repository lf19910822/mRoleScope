package aitrend

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	LangfuseDefaultFlushTimeout = 20 * time.Second
	LangfuseOTELPath            = "/api/public/otel/v1/traces"
)

type LangfuseDebugConfig struct {
	BaseURL      string
	ProjectID    string
	PublicKey    string
	SecretKey    string
	FlushTimeout time.Duration
	MaxBytes     int
}

func (config LangfuseDebugConfig) Enabled() bool {
	return strings.TrimSpace(config.BaseURL) != "" && strings.TrimSpace(config.ProjectID) != "" &&
		strings.TrimSpace(config.PublicKey) != "" && strings.TrimSpace(config.SecretKey) != ""
}

type LangfuseExportStatus struct {
	Status    string `json:"upload_status"`
	TraceID   string `json:"trace_id,omitempty"`
	TraceURL  string `json:"trace_url,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
}

type LangfuseDebugSink struct {
	mu          sync.Mutex
	config      LangfuseDebugConfig
	client      *http.Client
	traceID     string
	traceURL    string
	root        langfuseSpan
	spans       []langfuseSpan
	active      map[string]int
	flushed     bool
	flushStatus LangfuseExportStatus
}

func NewLangfuseDebugSink(config LangfuseDebugConfig, client *http.Client) (*LangfuseDebugSink, error) {
	if !config.Enabled() {
		return &LangfuseDebugSink{config: config, flushStatus: LangfuseExportStatus{Status: "disabled"}}, nil
	}
	_, err := langfuseTraceEndpoint(config.BaseURL)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{}
	}
	traceID, err := randomHexID(16)
	if err != nil {
		return nil, fmt.Errorf("create Langfuse trace ID: %w", err)
	}
	rootSpanID, err := randomHexID(8)
	if err != nil {
		return nil, fmt.Errorf("create Langfuse span ID: %w", err)
	}
	if config.FlushTimeout <= 0 {
		config.FlushTimeout = LangfuseDefaultFlushTimeout
	}
	if config.MaxBytes <= 0 {
		config.MaxBytes = 8 << 20
	}
	return &LangfuseDebugSink{
		config:   config,
		client:   client,
		traceID:  traceID,
		traceURL: strings.TrimRight(config.BaseURL, "/") + "/project/" + url.PathEscape(config.ProjectID) + "/traces/" + traceID,
		root: langfuseSpan{
			Name:    "mrolescope.ai_debug",
			TraceID: traceID,
			SpanID:  rootSpanID,
			Start:   time.Now().UTC(),
			Attrs: []langfuseAttribute{
				stringAttribute("langfuse.observation.type", "agent"),
				stringAttribute("langfuse.trace.metadata.mrolescope_debug_run_id", traceID),
				stringAttribute("langfuse.trace.metadata.environment", "debug"),
			},
		},
		active: make(map[string]int),
	}, nil
}

func (sink *LangfuseDebugSink) Record(event DebugTraceEvent) error {
	if sink == nil || !sink.config.Enabled() {
		return nil
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if sink.flushed {
		return nil
	}
	switch event.Type {
	case DebugEventModelRequest:
		span := sink.startSpan(event.Operation, "generation", sink.root.SpanID)
		span.Input = event.Body
		if model := modelFromRequest(event.Body); model != "" {
			span.Attrs = append(span.Attrs, stringAttribute("gen_ai.request.model", model))
		}
		sink.active["model:"+event.Operation] = len(sink.spans)
		sink.spans = append(sink.spans, span)
	case DebugEventModelResponse:
		sink.finishActive("model:"+event.Operation, event.Body, event.Error, event.HTTPStatus)
	case DebugEventToolCall:
		span := sink.startSpan(event.Tool, "tool", sink.root.SpanID)
		span.Input = event.Params
		sink.active["tool:"+event.Tool] = len(sink.spans)
		sink.spans = append(sink.spans, span)
	case DebugEventToolResult:
		sink.finishActive("tool:"+event.Tool, event.Result, event.Error, 0)
	case DebugEventValidation:
		span := sink.startSpan("validation", "span", sink.root.SpanID)
		span.Input = event.Params
		span.Output = safeErrorJSON(event.Error)
		span.Status = langfuseStatus{Code: "STATUS_CODE_ERROR", Message: safeErrorMessage(event.Error)}
		sink.spans = append(sink.spans, span)
	case DebugEventFinalIntent:
		span := sink.startSpan("final_intent", "span", sink.root.SpanID)
		span.Output = marshalDebugValue(event.Intent)
		sink.spans = append(sink.spans, span)
	case DebugEventDraft:
		span := sink.startSpan("draft", "span", sink.root.SpanID)
		span.Output = marshalDebugValue(event.Draft)
		sink.spans = append(sink.spans, span)
	case DebugEventTerminal:
		span := sink.startSpan("terminal", "span", sink.root.SpanID)
		span.Output = safeErrorJSON(event.Error)
		if event.Error != nil || event.Status == string(StreamFailure) {
			span.Status = langfuseStatus{Code: "STATUS_CODE_ERROR", Message: safeErrorMessage(event.Error)}
		}
		sink.spans = append(sink.spans, span)
	}
	return nil
}

func (sink *LangfuseDebugSink) Flush(ctx context.Context) LangfuseExportStatus {
	if sink == nil {
		return LangfuseExportStatus{Status: "disabled"}
	}
	sink.mu.Lock()
	if sink.flushed {
		status := sink.flushStatus
		sink.mu.Unlock()
		return status
	}
	sink.flushed = true
	if !sink.config.Enabled() {
		sink.flushStatus = LangfuseExportStatus{Status: "disabled"}
		status := sink.flushStatus
		sink.mu.Unlock()
		return status
	}
	sink.root.End = time.Now().UTC()
	spans := append([]langfuseSpan{sink.root}, sink.spans...)
	for index := range spans {
		if spans[index].End.IsZero() {
			spans[index].End = sink.root.End
		}
	}
	payload, err := json.Marshal(langfuseExportRequest{ResourceSpans: []langfuseResourceSpans{{Resource: langfuseResource{Attributes: []langfuseAttribute{stringAttribute("service.name", "mrolescope"), stringAttribute("service.version", "ai-debug")}}, ScopeSpans: []langfuseScopeSpans{{Scope: langfuseScope{Name: "mrolescope-debug"}, Spans: spansToOTEL(spans)}}}}})
	config := sink.config
	client := sink.client
	traceID := sink.traceID
	traceURL := sink.traceURL
	sink.mu.Unlock()
	if err != nil {
		return sink.setFlushStatus(LangfuseExportStatus{Status: "failed", TraceID: traceID, TraceURL: traceURL, ErrorCode: "encode_failed"})
	}
	if len(payload) > config.MaxBytes {
		return sink.setFlushStatus(LangfuseExportStatus{Status: "failed", TraceID: traceID, TraceURL: traceURL, ErrorCode: "payload_limit_exceeded"})
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, config.FlushTimeout)
		defer cancel()
	}
	endpoint, endpointErr := langfuseTraceEndpoint(config.BaseURL)
	if endpointErr != nil {
		return sink.setFlushStatus(LangfuseExportStatus{Status: "failed", TraceID: traceID, TraceURL: traceURL, ErrorCode: "invalid_endpoint"})
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return sink.setFlushStatus(LangfuseExportStatus{Status: "failed", TraceID: traceID, TraceURL: traceURL, ErrorCode: "request_failed"})
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(config.PublicKey+":"+config.SecretKey)))
	request.Header.Set("x-langfuse-ingestion-version", "4")
	response, err := client.Do(request)
	if err != nil {
		code := "upload_failed"
		if ctx.Err() != nil {
			code = "flush_timeout"
		}
		return sink.setFlushStatus(LangfuseExportStatus{Status: "failed", TraceID: traceID, TraceURL: traceURL, ErrorCode: code})
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return sink.setFlushStatus(LangfuseExportStatus{Status: "failed", TraceID: traceID, TraceURL: traceURL, ErrorCode: fmt.Sprintf("http_%d", response.StatusCode)})
	}
	return sink.setFlushStatus(LangfuseExportStatus{Status: "completed", TraceID: traceID, TraceURL: traceURL})
}

func (sink *LangfuseDebugSink) setFlushStatus(status LangfuseExportStatus) LangfuseExportStatus {
	sink.mu.Lock()
	sink.flushStatus = status
	sink.mu.Unlock()
	return status
}

func (sink *LangfuseDebugSink) TraceStatus() LangfuseExportStatus {
	if sink == nil {
		return LangfuseExportStatus{Status: "disabled"}
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if sink.traceID == "" {
		return sink.flushStatus
	}
	return LangfuseExportStatus{Status: "pending", TraceID: sink.traceID, TraceURL: sink.traceURL}
}

type DebugTraceFanout struct{ sinks []DebugTraceSink }

func NewDebugTraceFanout(sinks ...DebugTraceSink) *DebugTraceFanout {
	filtered := make([]DebugTraceSink, 0, len(sinks))
	for _, sink := range sinks {
		if sink != nil {
			filtered = append(filtered, sink)
		}
	}
	return &DebugTraceFanout{sinks: filtered}
}

func (fanout *DebugTraceFanout) Record(event DebugTraceEvent) error {
	if fanout == nil {
		return nil
	}
	for _, sink := range fanout.sinks {
		if err := sink.Record(event); err != nil {
			return err
		}
	}
	return nil
}

type langfuseSpan struct {
	Name         string
	TraceID      string
	SpanID       string
	ParentSpanID string
	Start        time.Time
	End          time.Time
	Attrs        []langfuseAttribute
	Input        json.RawMessage
	Output       json.RawMessage
	Status       langfuseStatus
}

type langfuseAttribute struct {
	Key   string        `json:"key"`
	Value langfuseValue `json:"value"`
}

type langfuseValue struct {
	StringValue *string `json:"stringValue,omitempty"`
	IntValue    *string `json:"intValue,omitempty"`
	BoolValue   *bool   `json:"boolValue,omitempty"`
}

type langfuseStatus struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type langfuseExportRequest struct {
	ResourceSpans []langfuseResourceSpans `json:"resourceSpans"`
}

type langfuseResourceSpans struct {
	Resource   langfuseResource     `json:"resource"`
	ScopeSpans []langfuseScopeSpans `json:"scopeSpans"`
}

type langfuseResource struct {
	Attributes []langfuseAttribute `json:"attributes,omitempty"`
}

type langfuseScopeSpans struct {
	Scope langfuseScope      `json:"scope"`
	Spans []langfuseOTELSpan `json:"spans"`
}

type langfuseScope struct {
	Name string `json:"name"`
}

type langfuseOTELSpan struct {
	TraceID           string              `json:"traceId"`
	SpanID            string              `json:"spanId"`
	ParentSpanID      string              `json:"parentSpanId,omitempty"`
	Name              string              `json:"name"`
	StartTimeUnixNano string              `json:"startTimeUnixNano"`
	EndTimeUnixNano   string              `json:"endTimeUnixNano"`
	Attributes        []langfuseAttribute `json:"attributes,omitempty"`
	Status            langfuseStatus      `json:"status,omitempty"`
}

func (sink *LangfuseDebugSink) startSpan(name, kind, parent string) langfuseSpan {
	spanID, _ := randomHexID(8)
	return langfuseSpan{Name: name, TraceID: sink.traceID, SpanID: spanID, ParentSpanID: parent, Start: time.Now().UTC(), Attrs: []langfuseAttribute{stringAttribute("langfuse.observation.type", kind)}}
}

func (sink *LangfuseDebugSink) finishActive(key string, output json.RawMessage, safeErr *SafeError, httpStatus int) {
	index, ok := sink.active[key]
	if !ok || index < 0 || index >= len(sink.spans) {
		return
	}
	span := &sink.spans[index]
	span.Output = output
	span.End = time.Now().UTC()
	if usage := usageFromResponse(output); usage[0] >= 0 {
		span.Attrs = append(span.Attrs, intAttribute("gen_ai.usage.input_tokens", int64(usage[0])), intAttribute("gen_ai.usage.output_tokens", int64(usage[1])))
	}
	if safeErr != nil || httpStatus >= http.StatusBadRequest {
		span.Status = langfuseStatus{Code: "STATUS_CODE_ERROR", Message: safeErrorMessage(safeErr)}
	}
	delete(sink.active, key)
}

func spansToOTEL(spans []langfuseSpan) []langfuseOTELSpan {
	result := make([]langfuseOTELSpan, 0, len(spans))
	for _, span := range spans {
		attrs := append([]langfuseAttribute(nil), span.Attrs...)
		if len(span.Input) > 0 {
			attrs = append(attrs, stringAttribute("langfuse.observation.input", string(span.Input)))
		}
		if len(span.Output) > 0 {
			attrs = append(attrs, stringAttribute("langfuse.observation.output", string(span.Output)))
		}
		result = append(result, langfuseOTELSpan{TraceID: span.TraceID, SpanID: span.SpanID, ParentSpanID: span.ParentSpanID, Name: span.Name, StartTimeUnixNano: fmt.Sprintf("%d", span.Start.UnixNano()), EndTimeUnixNano: fmt.Sprintf("%d", span.End.UnixNano()), Attributes: attrs, Status: span.Status})
	}
	return result
}

func langfuseTraceEndpoint(base string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(base))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return "", fmt.Errorf("invalid Langfuse base URL")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + LangfuseOTELPath
	return parsed.String(), nil
}

func randomHexID(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func stringAttribute(key, value string) langfuseAttribute {
	return langfuseAttribute{Key: key, Value: langfuseValue{StringValue: &value}}
}

func intAttribute(key string, value int64) langfuseAttribute {
	encoded := fmt.Sprintf("%d", value)
	return langfuseAttribute{Key: key, Value: langfuseValue{IntValue: &encoded}}
}

func marshalDebugValue(value any) json.RawMessage {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return data
}

func safeErrorJSON(err *SafeError) json.RawMessage { return marshalDebugValue(err) }

func safeErrorMessage(err *SafeError) string {
	if err == nil {
		return ""
	}
	return string(err.Code)
}

func modelFromRequest(data json.RawMessage) string {
	var value struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(data, &value) != nil {
		return ""
	}
	return value.Model
}

func usageFromResponse(data json.RawMessage) [2]int {
	var value struct {
		Usage struct {
			Prompt     int `json:"prompt_tokens"`
			Completion int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(data, &value) != nil || value.Usage.Prompt < 0 || value.Usage.Completion < 0 {
		return [2]int{-1, -1}
	}
	return [2]int{value.Usage.Prompt, value.Usage.Completion}
}
