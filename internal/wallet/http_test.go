package wallet_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/raagan-u/controlled/internal/wallet"
)

func TestOnboardingHandlerCreatesAndReplaysWallet(t *testing.T) {
	t.Parallel()

	service, _ := newOnboardingService()
	handler := wallet.NewOnboardingHandler(service)
	body := `{"email":"founder@example.com","displayName":"Raagan","currency":"USD"}`

	first := onboardingRequest(handler, "onboard-1", body)
	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}
	firstPayload := decodeResponse(t, first)
	if firstPayload.Duplicate || firstPayload.Wallet.ID == "" || firstPayload.Wallet.Balances.Available.AmountMinor != 0 {
		t.Fatalf("first response = %#v", firstPayload)
	}

	second := onboardingRequest(handler, "onboard-1", body)
	if second.Code != http.StatusOK {
		t.Fatalf("retry status = %d, body = %s", second.Code, second.Body.String())
	}
	secondPayload := decodeResponse(t, second)
	if !secondPayload.Duplicate || secondPayload.Wallet.ID != firstPayload.Wallet.ID {
		t.Fatalf("retry response = %#v", secondPayload)
	}
}

func TestOnboardingHandlerValidatesBoundary(t *testing.T) {
	t.Parallel()

	service, _ := newOnboardingService()
	handler := wallet.NewOnboardingHandler(service)

	tests := []struct {
		name   string
		key    string
		body   string
		status int
		code   string
	}{
		{name: "missing idempotency key", body: `{"email":"founder@example.com","displayName":"Raagan","currency":"USD"}`, status: http.StatusBadRequest, code: "invalid_idempotency_key"},
		{name: "unknown field", key: "onboard-1", body: `{"email":"founder@example.com","displayName":"Raagan","currency":"USD","admin":true}`, status: http.StatusBadRequest, code: "invalid_request"},
		{name: "trailing object", key: "onboard-1", body: `{}` + `{}`, status: http.StatusBadRequest, code: "invalid_request"},
		{name: "invalid email", key: "onboard-1", body: `{"email":"nope","displayName":"Raagan","currency":"USD"}`, status: http.StatusUnprocessableEntity, code: "invalid_onboarding"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := onboardingRequest(handler, test.key, test.body)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.status, response.Body.String())
			}
			var payload struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if payload.Error.Code != test.code {
				t.Fatalf("error code = %q, want %q", payload.Error.Code, test.code)
			}
		})
	}
}

func onboardingRequest(handler http.Handler, key, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/v1/demo/users", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder) walletOnboardingResponse {
	t.Helper()

	var payload walletOnboardingResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return payload
}

type walletOnboardingResponse struct {
	Wallet struct {
		ID       string `json:"id"`
		Balances struct {
			Available struct {
				AmountMinor int64 `json:"amountMinor"`
			} `json:"available"`
		} `json:"balances"`
	} `json:"wallet"`
	Duplicate bool `json:"duplicate"`
}
