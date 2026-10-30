package pact_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/pact"
)

func TestPactHandlers(t *testing.T) {
	now := time.Date(2026, time.October, 30, 12, 0, 0, 0, time.UTC)
	user, err := identity.NewUser("owner", "owner@example.com", "Owner", now)
	if err != nil {
		t.Fatalf("new user: %v", err)
	}
	handler := pact.NewHandler(stubPactCommands{now: now})
	sessions := pactSessionManager{user: user}
	mux := http.NewServeMux()
	mux.Handle("POST /v1/pacts", identity.RequireSession(sessions, http.HandlerFunc(handler.CreateDraft)))
	mux.Handle("GET /v1/pacts", identity.RequireSession(sessions, http.HandlerFunc(handler.Pacts)))
	mux.Handle("GET /v1/pacts/{id}", identity.RequireSession(sessions, http.HandlerFunc(handler.Pact)))
	mux.Handle("PUT /v1/pacts/{id}", identity.RequireSession(sessions, http.HandlerFunc(handler.UpdateDraft)))
	mux.Handle("POST /v1/pacts/{id}/confirm", identity.RequireSession(sessions, http.HandlerFunc(handler.Confirm)))

	body := `{"goal":"Save for rent","intent":"Spend less","spendingLimitMinor":20000,"stakeMinor":5000,"currency":"USD","startsAt":"2026-10-31T00:00:00Z","endsAt":"2026-11-30T00:00:00Z","accountabilityCircleId":"circle","breakConsequence":"COMMITMENT_RESERVE"}`
	tests := []struct {
		name, method, path, body string
		want                     int
	}{
		{name: "create", method: http.MethodPost, path: "/v1/pacts", body: body, want: http.StatusCreated},
		{name: "update", method: http.MethodPut, path: "/v1/pacts/pact", body: body, want: http.StatusOK},
		{name: "confirm", method: http.MethodPost, path: "/v1/pacts/pact/confirm", body: `{}`, want: http.StatusOK},
		{name: "detail", method: http.MethodGet, path: "/v1/pacts/pact", want: http.StatusOK},
		{name: "list", method: http.MethodGet, path: "/v1/pacts", want: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Authorization", "Bearer token")
			request.Header.Set("Idempotency-Key", test.name)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.want, response.Body.String())
			}
		})
	}
}

type stubPactCommands struct{ now time.Time }

func (s stubPactCommands) CreateDraft(_ context.Context, _ string, owner pact.UserID, terms pact.Terms) (*pact.Pact, bool, error) {
	value, err := pact.CreateDraft("pact", owner, terms, s.now)
	return value, false, err
}

func (s stubPactCommands) UpdateDraft(_ context.Context, owner pact.UserID, _ pact.PactID, terms pact.Terms) (*pact.Pact, error) {
	return pact.CreateDraft("pact", owner, terms, s.now)
}

func (s stubPactCommands) Confirm(_ context.Context, owner pact.UserID, _ pact.PactID) (*pact.Pact, error) {
	value, err := pact.CreateDraft("pact", owner, handlerTerms(s.now), s.now)
	if err != nil {
		return nil, err
	}
	err = value.Confirm(pact.ConfirmationFacts{PartnerCount: 1, DistinctPartnerCount: 1}, s.now)
	return value, err
}

func (s stubPactCommands) Pact(_ context.Context, owner pact.UserID, _ pact.PactID) (*pact.Pact, error) {
	return pact.CreateDraft("pact", owner, handlerTerms(s.now), s.now)
}

func (s stubPactCommands) Pacts(ctx context.Context, owner pact.UserID) ([]*pact.Pact, error) {
	value, err := s.Pact(ctx, owner, "pact")
	return []*pact.Pact{value}, err
}

func handlerTerms(now time.Time) pact.Terms {
	limit, _ := pact.NewMoney(20_000, "USD")
	stake, _ := pact.NewMoney(5_000, "USD")
	terms, _ := pact.NewTerms(
		"Save for rent", "Spend less", limit, stake,
		now.Add(time.Hour), now.Add(30*24*time.Hour), "circle", pact.BreakConsequenceCommitmentReserve,
	)
	return terms
}

type pactSessionManager struct{ user identity.User }

func (s pactSessionManager) Start(context.Context, string) (identity.StartedSession, error) {
	return identity.StartedSession{}, nil
}

func (s pactSessionManager) Authenticate(context.Context, string) (identity.User, error) {
	return s.user, nil
}

func (s pactSessionManager) End(context.Context, string) error { return nil }
