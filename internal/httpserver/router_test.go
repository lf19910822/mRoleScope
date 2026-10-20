package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type checker struct{ err error }

func (c checker) Ping(context.Context) error { return c.err }

func TestHealthDoesNotNeedDatabase(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	New(nil).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body["status"] != "ok" {
		t.Fatalf("unexpected response: %q, error: %v", response.Body.String(), err)
	}
}

func TestReadyReportsDatabaseFailure(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	New(checker{err: errors.New("unavailable")}).ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}
