package jobsync

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMihoyoDetailRecheckerTreatsAmbiguousResponsesAsIndeterminate(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"rate_limited": func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusTooManyRequests) },
		"server_error": func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusBadGateway) },
		"unknown_body": func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(`{"success":true,"data":{}}`))
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

func TestMihoyoDetailRecheckerTreatsTransportTimeoutAsIndeterminate(t *testing.T) {
	timeoutClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})}
	if result := (MihoyoDetailRechecker{Client: timeoutClient}).Recheck(context.Background(), "missing-id"); result.Outcome != RecheckIndeterminate || result.Code != "recheck_transport_error" {
		t.Fatalf("timeout result = %#v, want indeterminate transport error", result)
	}
}

func TestMihoyoDetailRecheckerClosesAfterThreeReviewedNegativeResponses(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = writer.Write([]byte(`{"success":false,"code":"not-found"}`))
	}))
	defer server.Close()
	result := (MihoyoDetailRechecker{Endpoint: server.URL}).Recheck(context.Background(), "missing-id")
	if result.Outcome != RecheckDefinitelyUnavailable || result.Code != "source_explicitly_unavailable" {
		t.Fatalf("negative result = %#v, want definitive unavailable", result)
	}
	if requests != 3 {
		t.Fatalf("requests = %d, want 3", requests)
	}
}

func TestMihoyoDetailRecheckerTreatsMixedRecheckSequencesAsNonClosing(t *testing.T) {
	availableDetail := `{"success":true,"data":{"id":"missing-id","title":"title","description":"body","jobRequire":"requirements"}}`
	for _, testCase := range []struct {
		name         string
		responses    []string
		wantOutcome  RecheckOutcome
		wantCode     string
		wantRequests int
	}{
		{
			name:         "available detail wins after a negative response",
			responses:    []string{`{"success":false}`, availableDetail},
			wantOutcome:  RecheckAvailable,
			wantCode:     "detail_available",
			wantRequests: 2,
		},
		{
			name:         "ambiguous response prevents closure after a negative response",
			responses:    []string{`{"success":false}`, `{"success":true,"data":{}}`},
			wantOutcome:  RecheckIndeterminate,
			wantCode:     "recheck_unknown_response",
			wantRequests: 2,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte(testCase.responses[requests]))
				requests++
			}))
			defer server.Close()

			result := (MihoyoDetailRechecker{Endpoint: server.URL}).Recheck(context.Background(), "missing-id")
			if result.Outcome != testCase.wantOutcome || result.Code != testCase.wantCode {
				t.Fatalf("result = %#v, want outcome=%s code=%s", result, testCase.wantOutcome, testCase.wantCode)
			}
			if requests != testCase.wantRequests {
				t.Fatalf("requests = %d, want %d", requests, testCase.wantRequests)
			}
		})
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
