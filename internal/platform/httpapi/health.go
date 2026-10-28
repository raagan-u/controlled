package httpapi

import (
	"context"
	"net/http"
	"time"
)

type Pinger interface {
	Ping(context.Context) error
}

type HealthHandler struct {
	database Pinger
	timeout  time.Duration
}

func NewHealthHandler(database Pinger, timeout time.Duration) *HealthHandler {
	return &HealthHandler{database: database, timeout: timeout}
}

func (h *HealthHandler) Liveness(response http.ResponseWriter, _ *http.Request) {
	response.WriteHeader(http.StatusNoContent)
}

func (h *HealthHandler) Readiness(response http.ResponseWriter, request *http.Request) {
	if h == nil || h.database == nil || h.timeout <= 0 {
		WriteError(response, http.StatusServiceUnavailable, "not_ready", "Controlled is not ready.")
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), h.timeout)
	defer cancel()
	if err := h.database.Ping(ctx); err != nil {
		WriteError(response, http.StatusServiceUnavailable, "not_ready", "Controlled cannot reach its database.")
		return
	}
	response.WriteHeader(http.StatusNoContent)
}
