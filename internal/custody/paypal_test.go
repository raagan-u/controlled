package custody

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/raagan-u/controlled/internal/money"
)

func TestPayPalFundingAndPayoutFlow(t *testing.T) {
	t.Parallel()
	var tokenRequests atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/v1/oauth2/token" {
			tokenRequests.Add(1)
			clientID, secret, ok := request.BasicAuth()
			if !ok || clientID != "client" || secret != "secret" {
				t.Fatalf("unexpected OAuth credentials")
			}
			return testJSONResponse(map[string]any{"access_token": "token", "expires_in": 3600}), nil
		}
		if request.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
		}
		switch request.Method + " " + request.URL.Path {
		case "POST /v2/checkout/orders":
			var body struct {
				PurchaseUnits []struct {
					Amount struct{ Value, Currency string } `json:"amount"`
				} `json:"purchase_units"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body.PurchaseUnits[0].Amount.Value != "12.34" {
				t.Fatalf("unexpected order payload: %#v, %v", body, err)
			}
			return testJSONResponse(map[string]any{"id": "ORDER-1", "links": []map[string]string{{"rel": "payer-action", "href": "https://paypal.test/approve"}}}), nil
		case "POST /v2/checkout/orders/ORDER-1/capture":
			return testJSONResponse(map[string]any{
				"id": "ORDER-1", "status": "COMPLETED",
				"purchase_units": []any{map[string]any{
					"payments": map[string]any{"captures": []any{map[string]any{
						"status": "COMPLETED",
						"amount": map[string]string{"value": "12.34", "currency_code": "USD"},
						"seller_receivable_breakdown": map[string]any{
							"net_amount": map[string]string{"value": "11.68", "currency_code": "USD"},
						},
					}}},
				}},
			}), nil
		case "POST /v1/payments/payouts":
			return testJSONResponse(map[string]any{"batch_header": map[string]string{"payout_batch_id": "BATCH-1", "batch_status": "PENDING"}}), nil
		case "GET /v1/payments/payouts/BATCH-1":
			return testJSONResponse(map[string]any{
				"batch_header": map[string]string{"batch_status": "SUCCESS"},
				"items": []any{map[string]any{
					"transaction_status": "SUCCESS",
					"payout_item": map[string]any{
						"receiver": "sandbox@example.com",
						"amount":   map[string]string{"value": "12.34", "currency": "USD"},
					},
				}},
			}), nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
		}
	})}

	provider, err := NewPayPal(PayPalOptions{APIBase: "https://api.paypal.test", Client: client, ClientID: "client", ClientSecret: "secret", ReturnURL: "https://example.test/return", CancelURL: "https://example.test/cancel"})
	if err != nil {
		t.Fatalf("NewPayPal() error = %v", err)
	}
	amount, _ := money.New(1234, "USD")
	order, err := provider.CreateFundingOrder(t.Context(), CreateFundingOrder{IdempotencyKey: "fund-1", Amount: amount})
	if err != nil || order.ID != "ORDER-1" || !strings.Contains(order.ApprovalURL, "paypal.test") {
		t.Fatalf("CreateFundingOrder() = %#v, %v", order, err)
	}
	captured, err := provider.CaptureFundingOrder(t.Context(), order.ID)
	if err != nil || captured.Status != FundingCaptured || !captured.Amount.Equal(amount) || captured.NetAmount.AmountMinor() != 1168 {
		t.Fatalf("CaptureFundingOrder() = %#v, %v", captured, err)
	}
	payout, err := provider.CreatePayout(t.Context(), CreatePayout{IdempotencyKey: "withdraw-1", Amount: amount, Destination: "sandbox@example.com"})
	if err != nil || payout.ID != "BATCH-1" || payout.Status != PayoutPending {
		t.Fatalf("CreatePayout() = %#v, %v", payout, err)
	}
	payout, err = provider.GetPayout(t.Context(), payout.ID)
	if err != nil || payout.Status != PayoutSucceeded || payout.Destination != "sandbox@example.com" {
		t.Fatalf("GetPayout() = %#v, %v", payout, err)
	}
	if tokenRequests.Load() != 1 {
		t.Fatalf("OAuth requests = %d, want 1", tokenRequests.Load())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func testJSONResponse(value any) *http.Response {
	encoded, _ := json.Marshal(value)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(string(encoded))),
		Header:     http.Header{"Content-Type": {"application/json"}},
	}
}
