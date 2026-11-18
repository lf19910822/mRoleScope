package httpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"mrolescope/internal/aitrend"
)

// AITrendStreamRunner is the HTTP-to-orchestrator seam. The runner owns model
// calls and Serving tools; HTTP only accepts a bounded request and forwards
// validated, display-safe events.
type AITrendStreamRunner interface {
	Run(context.Context, string, func(aitrend.StreamEvent) error) error
}

func (r Router) aiTrendDraft(context *gin.Context) {
	if r.aiTrend == nil {
		context.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, 8*1024)
	var request struct {
		Query string `json:"query"`
	}
	decoder := json.NewDecoder(context.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || strings.TrimSpace(request.Query) == "" {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	context.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	context.Header("Cache-Control", "no-store")
	context.Status(http.StatusOK)
	writer := bufio.NewWriter(context.Writer)
	emit := func(event aitrend.StreamEvent) error {
		if event.Type == aitrend.StreamEventOperation {
			if !safeAITrendTool(event.Tool) {
				return errors.New("unregistered trend tool")
			}
			event.Params = filterAITrendParams(event.Tool, event.Params)
		}
		if err := aitrend.ValidateStreamEvent(event); err != nil {
			return err
		}
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := writer.Write(append(data, '\n')); err != nil {
			return err
		}
		if err := writer.Flush(); err != nil {
			return err
		}
		if flusher, ok := context.Writer.(http.Flusher); ok {
			flusher.Flush()
		}
		return nil
	}
	if err := r.aiTrend.Run(context.Request.Context(), strings.TrimSpace(request.Query), emit); err != nil && context.Request.Context().Err() == nil {
		_ = emit(aitrend.StreamEvent{Type: aitrend.StreamEventDone, Status: aitrend.StreamFailure, Error: &aitrend.SafeError{Code: aitrend.ErrorProviderFailure, Message: "分析未完成"}})
	}
}

func safeAITrendTool(tool string) bool {
	switch aitrend.ToolName(tool) {
	case aitrend.ToolListTechnologyCategories, aitrend.ToolListCategoryValues,
		aitrend.ToolListAvailableValues, aitrend.ToolRankValues,
		aitrend.ToolReadTrendSummary, aitrend.ToolReadEventSummary,
		aitrend.ToolValidateCurveDraft, aitrend.ToolBatchTrendSummary,
		aitrend.ToolBatchEventSummary, aitrend.ToolBatchValidateDraft:
		return true
	default:
		return false
	}
}

func filterAITrendParams(tool string, raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var input map[string]json.RawMessage
	if json.Unmarshal(raw, &input) != nil {
		return nil
	}
	allowed := map[string]struct{}{}
	switch aitrend.ToolName(tool) {
	case aitrend.ToolListCategoryValues:
		allowed["category"] = struct{}{}
	case aitrend.ToolListAvailableValues:
		allowed["dimension"] = struct{}{}
	case aitrend.ToolRankValues:
		for _, key := range []string{"dimension", "expression", "sub_query_id"} {
			allowed[key] = struct{}{}
		}
	case aitrend.ToolReadTrendSummary, aitrend.ToolReadEventSummary:
		for _, key := range []string{"dimension", "value", "range", "start_date", "end_date", "include_secondary", "operator"} {
			allowed[key] = struct{}{}
		}
	default:
		return nil
	}
	filtered := make(map[string]json.RawMessage, len(input))
	for key, value := range input {
		if _, ok := allowed[key]; ok {
			filtered[key] = value
		}
	}
	data, _ := json.Marshal(filtered)
	return data
}
