package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/platform/httpapi"
)

func TestHealthHandler(t *testing.T) {
	t.Parallel()

	t.Run("live", func(t *testing.T) {
		handler := httpapi.NewHealthHandler(pinger{}, time.Second)
		response := httptest.NewRecorder()
		handler.Liveness(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d", response.Code)
		}
		assertHealthResponse(t, response, "ok")
	})

	t.Run("ready", func(t *testing.T) {
		handler := httpapi.NewHealthHandler(pinger{}, time.Second)
		response := httptest.NewRecorder()
		handler.Readiness(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d", response.Code)
		}
		assertHealthResponse(t, response, "ready")
	})

	t.Run("database unavailable", func(t *testing.T) {
		handler := httpapi.NewHealthHandler(pinger{err: errors.New("database unavailable")}, time.Second)
		response := httptest.NewRecorder()
		handler.Readiness(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d", response.Code)
		}
	})
}

func assertHealthResponse(t *testing.T, response *httptest.ResponseRecorder, status string) {
	t.Helper()
	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q", response.Header().Get("Content-Type"))
	}
	var body httpapi.HealthResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != status || body.Service != "controlled-api" {
		t.Fatalf("body = %#v", body)
	}
}

type pinger struct {
	err error
}

func (p pinger) Ping(context.Context) error {
	return p.err
}
