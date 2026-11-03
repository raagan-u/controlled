package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/raagan-u/controlled/internal/platform/httpapi"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestLogRequestsRecordsStructuredCompletion(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zap.InfoLevel)
	handler := httpapi.LogRequests(zap.New(core), http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if httpapi.Logger(request.Context()) == nil {
			t.Fatal("request logger is nil")
		}
		httpapi.WriteJSON(response, http.StatusCreated, map[string]string{"status": "created"})
	}))
	request := httptest.NewRequest(http.MethodPost, "/v1/example", strings.NewReader(`{}`))
	request.Header.Set(httpapi.RequestIDHeader, "request-123")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated || response.Header().Get(httpapi.RequestIDHeader) != "request-123" {
		t.Fatalf("response = status %d, request ID %q", response.Code, response.Header().Get(httpapi.RequestIDHeader))
	}
	entries := logs.FilterMessage("HTTP request completed").All()
	if len(entries) != 1 {
		t.Fatalf("completion logs = %d", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["request_id"] != "request-123" || fields["http.method"] != http.MethodPost ||
		fields["http.path"] != "/v1/example" || fields["http.status"] != int64(http.StatusCreated) {
		t.Fatalf("fields = %#v", fields)
	}
}

func TestLogRequestsRecoversPanics(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zap.InfoLevel)
	handler := httpapi.LogRequests(zap.New(core), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("unexpected failure")
	}))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), `"code":"internal_error"`) {
		t.Fatalf("response = status %d, body %s", response.Code, response.Body.String())
	}
	if logs.FilterMessage("HTTP handler panicked").Len() != 1 {
		t.Fatalf("panic logs = %d", logs.FilterMessage("HTTP handler panicked").Len())
	}
	completion := logs.FilterMessage("HTTP request completed").All()
	if len(completion) != 1 || completion[0].Level.String() != "error" {
		t.Fatalf("completion logs = %#v", completion)
	}
	if code := completion[0].ContextMap()["error.code"]; code != "internal_error" {
		t.Fatalf("error code = %#v", code)
	}
}
