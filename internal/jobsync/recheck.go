package jobsync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type RecheckOutcome string

const (
	RecheckAvailable             RecheckOutcome = "available"
	RecheckDefinitelyUnavailable RecheckOutcome = "definitively_unavailable"
	RecheckIndeterminate         RecheckOutcome = "indeterminate"
)

type RecheckResult struct {
	Outcome RecheckOutcome
	Code    string
}

// DetailRechecker is intentionally sequential at the service boundary. A
// source omission must not trigger an unbounded burst of detail requests.
type DetailRechecker interface {
	Recheck(context.Context, string) RecheckResult
}

type IndeterminateRechecker struct{}

func (IndeterminateRechecker) Recheck(context.Context, string) RecheckResult {
	return RecheckResult{Outcome: RecheckIndeterminate, Code: "recheck_not_configured"}
}

// MihoyoDetailRechecker recognizes valid matching detail responses and the
// reviewed explicit-unavailable response. It is invoked sequentially at the
// service boundary so a list omission cannot create parallel source traffic.
type MihoyoDetailRechecker struct {
	Client   *http.Client
	Endpoint string
}

func (checker MihoyoDetailRechecker) Recheck(ctx context.Context, sourceJobID string) RecheckResult {
	for attempt := 0; attempt < 3; attempt++ {
		result := checker.recheckOnce(ctx, sourceJobID)
		if result.Outcome != RecheckDefinitelyUnavailable {
			return result
		}
		if attempt == 2 {
			return result
		}
	}
	return RecheckResult{Outcome: RecheckIndeterminate, Code: "recheck_attempts_exhausted"}
}

func (checker MihoyoDetailRechecker) recheckOnce(ctx context.Context, sourceJobID string) RecheckResult {
	client := checker.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	endpoint := checker.Endpoint
	if endpoint == "" {
		endpoint = "https://ats.openout.mihoyo.com/ats-portal/v1/job/info"
	}
	payload, _ := json.Marshal(map[string]any{
		"id":               sourceJobID,
		"channelDetailIds": []int{1},
		"hireType":         0,
	})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return RecheckResult{Outcome: RecheckIndeterminate, Code: "recheck_request_invalid"}
	}
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	request.Header.Set("Accept", "application/json, text/plain, */*")
	response, err := client.Do(request)
	if err != nil {
		return RecheckResult{Outcome: RecheckIndeterminate, Code: "recheck_transport_error"}
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests {
		return RecheckResult{Outcome: RecheckIndeterminate, Code: "recheck_rate_limited"}
	}
	if response.StatusCode >= 500 {
		return RecheckResult{Outcome: RecheckIndeterminate, Code: "recheck_server_error"}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return RecheckResult{Outcome: RecheckIndeterminate, Code: "recheck_http_unknown"}
	}
	var decoded struct {
		Success *bool          `json:"success"`
		Data    map[string]any `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&decoded); err != nil {
		return RecheckResult{Outcome: RecheckIndeterminate, Code: "recheck_response_invalid"}
	}
	if decoded.Success == nil {
		return RecheckResult{Outcome: RecheckIndeterminate, Code: "recheck_unknown_response"}
	}
	if !*decoded.Success {
		return RecheckResult{Outcome: RecheckDefinitelyUnavailable, Code: "source_explicitly_unavailable"}
	}
	if fmt.Sprint(decoded.Data["id"]) == sourceJobID && hasRequiredDetailFields(decoded.Data) {
		return RecheckResult{Outcome: RecheckAvailable, Code: "detail_available"}
	}
	return RecheckResult{Outcome: RecheckIndeterminate, Code: "recheck_unknown_response"}
}

func hasRequiredDetailFields(data map[string]any) bool {
	for _, field := range []string{"title", "description", "jobRequire"} {
		if normalizedString(data[field]) == "" {
			return false
		}
	}
	return true
}
