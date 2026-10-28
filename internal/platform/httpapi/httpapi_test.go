package httpapi_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/raagan-u/controlled/internal/platform/httpapi"
)

func TestReadJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		wantErr error
	}{
		{name: "valid", body: `{"name":"Controlled"}`},
		{name: "unknown field", body: `{"name":"Controlled","admin":true}`, wantErr: httpapi.ErrInvalidJSON},
		{name: "trailing object", body: `{}` + `{}`, wantErr: httpapi.ErrInvalidJSON},
		{name: "empty", wantErr: httpapi.ErrInvalidJSON},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(test.body))
			response := httptest.NewRecorder()
			var destination struct {
				Name string `json:"name"`
			}
			err := httpapi.ReadJSON(response, request, &destination, 0)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("ReadJSON() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestWriteError(t *testing.T) {
	t.Parallel()

	response := httptest.NewRecorder()
	httpapi.WriteError(response, http.StatusConflict, "conflict", "The request conflicts.")
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q", response.Header().Get("Content-Type"))
	}
	if body := response.Body.String(); !strings.Contains(body, `"code":"conflict"`) {
		t.Fatalf("body = %s", body)
	}
}
