package httpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"mrolescope/internal/aitrend"
)

func (r Router) aiTrendDebug(c *gin.Context) {
	if !r.aiDebugConfig.Enabled || r.aiDebug == nil || !isLoopbackRequest(c.Request) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8*1024)
	var request struct {
		Query string `json:"query"`
	}
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || strings.TrimSpace(request.Query) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	writer := bufio.NewWriter(c.Writer)
	recorder, err := aitrend.NewDebugTraceRecorder(func(event aitrend.DebugTraceEvent) error {
		data, marshalErr := json.Marshal(event)
		if marshalErr != nil {
			return marshalErr
		}
		if _, writeErr := writer.Write(append(data, '\n')); writeErr != nil {
			return writeErr
		}
		if flushErr := writer.Flush(); flushErr != nil {
			return flushErr
		}
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		}
		return nil
	}, r.aiDebugConfig.MaxEvents, r.aiDebugConfig.MaxBytes)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}
	c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)
	runContext := c.Request.Context()
	if r.aiDebugConfig.Timeout > 0 {
		var cancel context.CancelFunc
		runContext, cancel = context.WithTimeout(runContext, r.aiDebugConfig.Timeout)
		defer cancel()
	}
	_ = r.aiDebug.RunDebug(runContext, strings.TrimSpace(request.Query), func(aitrend.StreamEvent) error { return nil }, recorder)
}

func isLoopbackRequest(request *http.Request) bool {
	if request == nil {
		return false
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(request.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(request.RemoteAddr)
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
