package jobsync

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMihoyoDetailRecheckerTreatsUnknownAndFailureResponsesAsIndeterminate(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"rate_limited": func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusTooManyRequests) },
		"server_error": func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusBadGateway) },
		"unknown_body": func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(`{"success":false,"code":"unknown"}`))
		},
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(handler)
			defer server.Close()
			result := (MihoyoDetailRechecker{Endpoint: server.URL}).Recheck(context.Background(), "missing-id")
			if result.Outcome != RecheckIndeterminate {
				t.Fatalf("outcome = %s, want indeterminate", result.Outcome)
			}
		})
	}
}

func TestMihoyoDetailRecheckerTreatsTransportTimeoutAndUnverifiedCloseAsIndeterminate(t *testing.T) {
	timeoutClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})}
	if result := (MihoyoDetailRechecker{Client: timeoutClient}).Recheck(context.Background(), "missing-id"); result.Outcome != RecheckIndeterminate || result.Code != "recheck_transport_error" {
		t.Fatalf("timeout result = %#v, want indeterminate transport error", result)
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"success":false,"code":"not-found"}`))
	}))
	defer server.Close()
	result := (MihoyoDetailRechecker{Endpoint: server.URL, EnableClose: true}).Recheck(context.Background(), "missing-id")
	if result.Outcome != RecheckIndeterminate || result.Code != "unavailable_classifier_unverified" {
		t.Fatalf("unverified close result = %#v, want indeterminate", result)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	if function == nil {
		return nil, errors.New("round trip function is nil")
	}
	return function(request)
}

func TestMihoyoDetailRecheckerRecognizesMatchingDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("method = %s", request.Method)
		}
		_, _ = writer.Write([]byte(`{"success":true,"data":{"id":"8771","title":"title","description":"body","jobRequire":"requirements"}}`))
	}))
	defer server.Close()
	result := (MihoyoDetailRechecker{Endpoint: server.URL}).Recheck(context.Background(), "8771")
	if result.Outcome != RecheckAvailable {
		t.Fatalf("outcome = %s, want available", result.Outcome)
	}
}
