package httpapi_test

import (
	"context"
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
		if response.Code != http.StatusNoContent {
			t.Fatalf("status = %d", response.Code)
		}
	})

	t.Run("ready", func(t *testing.T) {
		handler := httpapi.NewHealthHandler(pinger{}, time.Second)
		response := httptest.NewRecorder()
		handler.Readiness(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if response.Code != http.StatusNoContent {
			t.Fatalf("status = %d", response.Code)
		}
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

type pinger struct {
	err error
}

func (p pinger) Ping(context.Context) error {
	return p.err
}
