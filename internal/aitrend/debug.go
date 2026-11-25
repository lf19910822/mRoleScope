package aitrend

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// DebugTraceSink receives bounded, operator-only diagnostic records. It is
// intentionally separate from AuditStore: debug records are streamed for a
// controlled run and are never persisted as production audit data.
type DebugTraceSink interface {
	Record(DebugTraceEvent) error
}

type DebugTraceEvent struct {
	Sequence        int             `json:"sequence"`
	Type            string          `json:"type"`
	Operation       string          `json:"operation,omitempty"`
	Status          string          `json:"status,omitempty"`
	Tool            string          `json:"tool,omitempty"`
	Params          json.RawMessage `json:"params,omitempty"`
	Result          json.RawMessage `json:"result,omitempty"`
	Body            json.RawMessage `json:"body,omitempty"`
	HTTPStatus      int             `json:"http_status,omitempty"`
	Error           *SafeError      `json:"error,omitempty"`
	Intent          *Intent         `json:"intent,omitempty"`
	Draft           *Draft          `json:"draft,omitempty"`
	Stream          *StreamEvent    `json:"stream,omitempty"`
	TraceID         string          `json:"trace_id,omitempty"`
	TraceURL        string          `json:"trace_url,omitempty"`
	ExportStatus    string          `json:"upload_status,omitempty"`
	ExportErrorCode string          `json:"upload_error_code,omitempty"`
}

const (
	DebugEventStream        = "stream_event"
	DebugEventModelRequest  = "model_request"
	DebugEventModelResponse = "model_response"
	DebugEventToolCall      = "tool_call"
	DebugEventToolResult    = "tool_result"
	DebugEventValidation    = "validation"
	DebugEventFinalIntent   = "final_intent"
	DebugEventDraft         = "draft"
	DebugEventTerminal      = "terminal"
	DebugEventTraceLink     = "trace_link"
)

const debugProviderBodyLimit = 4 << 20

type debugTraceContextKey struct{}

func withDebugTrace(ctx context.Context, sink DebugTraceSink) context.Context {
	if sink == nil {
		return ctx
	}
	return context.WithValue(ctx, debugTraceContextKey{}, sink)
}

func debugTraceFromContext(ctx context.Context) DebugTraceSink {
	if ctx == nil {
		return nil
	}
	sink, _ := ctx.Value(debugTraceContextKey{}).(DebugTraceSink)
	return sink
}

func recordDebug(ctx context.Context, event DebugTraceEvent) error {
	if sink := debugTraceFromContext(ctx); sink != nil {
		return sink.Record(event)
	}
	return nil
}

func debugJSON(data []byte) json.RawMessage {
	if json.Valid(data) {
		return append(json.RawMessage(nil), data...)
	}
	encoded, _ := json.Marshal(string(data))
	return encoded
}

// DebugTraceRecorder adds ordering and hard bounds to an operator stream.
// The callback must not log or persist the event implicitly.
type DebugTraceRecorder struct {
	mu        sync.Mutex
	emit      func(DebugTraceEvent) error
	maxEvents int
	maxBytes  int
	sequence  int
	bytes     int
}

func NewDebugTraceRecorder(emit func(DebugTraceEvent) error, maxEvents, maxBytes int) (*DebugTraceRecorder, error) {
	if emit == nil || maxEvents < 1 || maxBytes < 1 {
		return nil, fmt.Errorf("invalid debug trace limits")
	}
	return &DebugTraceRecorder{emit: emit, maxEvents: maxEvents, maxBytes: maxBytes}, nil
}

func (recorder *DebugTraceRecorder) Record(event DebugTraceEvent) error {
	if recorder == nil {
		return fmt.Errorf("debug trace recorder is unavailable")
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	event.Sequence = recorder.sequence + 1
	event.Params = redactDebugJSON(event.Params)
	event.Result = redactDebugJSON(event.Result)
	event.Body = redactDebugJSON(event.Body)
	if event.Stream != nil {
		stream := *event.Stream
		stream.Params = redactDebugJSON(stream.Params)
		event.Stream = &stream
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if event.Sequence > recorder.maxEvents || recorder.bytes+len(encoded) > recorder.maxBytes {
		return &ContractError{Code: ErrorLimitExceeded, Field: "debug_trace", Message: "debug trace limit exceeded"}
	}
	if err := recorder.emit(event); err != nil {
		return err
	}
	recorder.sequence = event.Sequence
	recorder.bytes += len(encoded)
	return nil
}

var debugSecretKeys = map[string]struct{}{
	"authorization": {}, "api_key": {}, "apikey": {}, "token": {}, "access_token": {},
	"password": {}, "secret": {}, "secret_key": {}, "database_url": {}, "invite_code": {},
}

func redactDebugJSON(data json.RawMessage) json.RawMessage {
	if len(data) == 0 {
		return nil
	}
	var value any
	if json.Unmarshal(data, &value) != nil {
		return nil
	}
	redactDebugValue(value)
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return encoded
}

func redactDebugValue(value any) {
	switch object := value.(type) {
	case map[string]any:
		for key, child := range object {
			if _, secret := debugSecretKeys[strings.ToLower(strings.TrimSpace(key))]; secret {
				object[key] = "<REDACTED>"
				continue
			}
			redactDebugValue(child)
		}
	case []any:
		for _, child := range object {
			redactDebugValue(child)
		}
	}
}
