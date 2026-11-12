package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"mrolescope/internal/access"
	"mrolescope/internal/jdserving"
)

func newTestAccessService(t *testing.T) *access.Service {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("known-invite"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate invite hash: %v", err)
	}
	service, err := access.New(access.Config{
		InviteBcryptHash:     string(hash),
		SessionSigningSecret: strings.Repeat("a", 64),
		Now:                  func() time.Time { return time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("create access service: %v", err)
	}
	return service
}

func TestAccessVerificationSetsSecureOpaqueCookie(t *testing.T) {
	router := NewWithAccess(nil, newTestAccessService(t))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/access/verify", strings.NewReader(`{"invite_code":"known-invite"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("verification status = %d body=%s", response.Code, response.Body.String())
	}
	cookie := response.Result().Cookies()
	if len(cookie) != 1 || cookie[0].Name != accessCookieName || cookie[0].Value == "" ||
		!cookie[0].Secure || !cookie[0].HttpOnly || cookie[0].SameSite != http.SameSiteLaxMode ||
		cookie[0].MaxAge != int(access.SessionLifetime/time.Second) {
		t.Fatalf("verification cookie = %#v", cookie)
	}
	if strings.Contains(response.Body.String(), "known-invite") {
		t.Fatal("verification response leaked invite code")
	}
}

func TestAccessVerificationFailureIsGenericAndRateLimited(t *testing.T) {
	router := NewWithAccess(nil, newTestAccessService(t))
	for attempt := 0; attempt < accessVerificationFailureLimit; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/access/verify", strings.NewReader(`{"invite_code":"wrong"}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized || response.Body.String() != `{"error":"invalid_invite"}` {
			t.Fatalf("failure %d = status:%d body:%s", attempt, response.Code, response.Body.String())
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/access/verify", strings.NewReader(`{"invite_code":"wrong"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusTooManyRequests || response.Body.String() != `{"error":"rate_limited"}` {
		t.Fatalf("rate-limited response = status:%d body:%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "wrong") {
		t.Fatal("rate-limit response leaked invite code")
	}
}

func TestBusinessRoutesRequireSessionAndPreserveExistingResponse(t *testing.T) {
	reader := jdListReaderFunc(func(context.Context, jdserving.ListJobsQuery) (jdserving.JobListResponse, error) {
		return jdserving.JobListResponse{}, nil
	})
	service := newTestAccessService(t)
	router := NewWithAccess(nil, service, reader)

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil))
	if unauthorized.Code != http.StatusUnauthorized || unauthorized.Body.String() != `{"error":"unauthorized"}` {
		t.Fatalf("unauthorized business response = status:%d body:%s", unauthorized.Code, unauthorized.Body.String())
	}

	token, err := service.Issue("known-invite")
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	authorizedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	authorizedRequest.AddCookie(&http.Cookie{Name: accessCookieName, Value: token})
	authorized := httptest.NewRecorder()
	router.ServeHTTP(authorized, authorizedRequest)
	if authorized.Code != http.StatusOK {
		t.Fatalf("authorized business response = status:%d body:%s", authorized.Code, authorized.Body.String())
	}
}

func TestAccessCheckHasNoBusinessPayload(t *testing.T) {
	service := newTestAccessService(t)
	router := NewWithAccess(nil, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/access/check", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || strings.Contains(response.Body.String(), "jobs") {
		t.Fatalf("access check response = status:%d body:%s", response.Code, response.Body.String())
	}

	token, err := service.Issue("known-invite")
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/access/check", nil)
	request.AddCookie(&http.Cookie{Name: accessCookieName, Value: token})
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("authorized access check status = %d", response.Code)
	}
}

func TestAccessVerificationRejectsMalformedBodyWithoutLoggingSecrets(t *testing.T) {
	router := NewWithAccess(nil, newTestAccessService(t))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/access/verify", strings.NewReader(`not-json-known-invite`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || response.Body.String() != `{"error":"invalid_request"}` {
		t.Fatalf("malformed verification response = status:%d body:%s", response.Code, response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if strings.Contains(response.Body.String(), "known-invite") {
		t.Fatal("malformed response leaked request body")
	}
}

func TestRequestSourceDoesNotTrustForwardedFor(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/access/verify", nil)
	request.RemoteAddr = "10.0.0.4:4321"
	request.Header.Set("X-Forwarded-For", "198.51.100.10")

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request
	if source := requestSource(context); source != "10.0.0.4" {
		t.Fatalf("requestSource() = %q, want remote address", source)
	}
	request.Header.Set("X-Real-IP", "203.0.113.7")
	if source := requestSource(context); source != "203.0.113.7" {
		t.Fatalf("requestSource() = %q, want trusted real IP", source)
	}
}
