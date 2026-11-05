package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestOpenAICompatibleModelSendsStrictSchema(t *testing.T) {
	t.Parallel()
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://models.example.test/v1/chat/completions" || request.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("request = %s auth=%q", request.URL, request.Header.Get("Authorization"))
		}
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		format := payload["response_format"].(map[string]any)
		if format["type"] != "json_schema" {
			t.Errorf("response format = %#v", format)
		}
		provider := payload["provider"].(map[string]any)
		if provider["require_parameters"] != true {
			t.Errorf("provider = %#v", provider)
		}
		return jsonResponse(http.StatusOK, `{"choices":[{"message":{"content":"{\"headline\":\"A steady start\",\"message\":\"Keep the boundary realistic and connected to the goal.\",\"tone\":\"NEUTRAL\",\"suggestedLimitMinor\":5000,\"suggestedStakeMinor\":1000,\"suggestedDurationDays\":30}"}}]}`), nil
	})}
	model, err := NewOpenAICompatibleModel(OpenAICompatibleOptions{BaseURL: "https://models.example.test/v1", Model: "model", APIKey: "secret", Timeout: time.Second, RequireParameters: true, HTTPClient: client})
	if err != nil {
		t.Fatal(err)
	}
	draft, err := model.Draft(context.Background(), ModelInput{Kind: GuidancePactCreation})
	if err != nil || draft.SuggestedLimitMinor != 5_000 {
		t.Fatalf("draft = %#v, error = %v", draft, err)
	}
}

func TestOpenAICompatibleModelMapsProviderFailures(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusPaymentRequired, http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		status := status
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return jsonResponse(status, `{}`), nil })}
			model, _ := NewOpenAICompatibleModel(OpenAICompatibleOptions{BaseURL: "https://models.example.test", Model: "model", APIKey: "secret", Timeout: time.Second, HTTPClient: client})
			_, err := model.Draft(context.Background(), ModelInput{})
			want := ErrUnavailable
			if status == http.StatusPaymentRequired || status == http.StatusTooManyRequests {
				want = ErrRateLimited
			}
			if err != want {
				t.Fatalf("error = %v, want %v", err, want)
			}
		})
	}
}

func TestOpenAICompatibleModelClassifiesTransactionWithStrictSchema(t *testing.T) {
	t.Parallel()
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		format := payload["response_format"].(map[string]any)
		schema := format["json_schema"].(map[string]any)
		if schema["name"] != "controlled_transaction_relationship" {
			t.Errorf("schema = %#v", schema)
		}
		return jsonResponse(http.StatusOK, `{"choices":[{"message":{"content":"{\"relationship\":\"CONFLICTING\",\"confidence\":\"HIGH\",\"reason\":\"The purchase directly matches the behavior the pact aims to avoid.\"}"}}]}`), nil
	})}
	model, err := NewOpenAICompatibleModel(OpenAICompatibleOptions{BaseURL: "https://models.example.test/v1", Model: "model", APIKey: "secret", Timeout: time.Second, HTTPClient: client})
	if err != nil {
		t.Fatal(err)
	}
	classification, err := model.ClassifyTransaction(context.Background(), ClassificationInput{Goal: "Avoid expensive dinners", Intent: "Protect savings", Reason: "expensive dinner", IsPurchase: true})
	if err != nil || classification.Relationship != RelationshipConflicting || classification.Confidence != ConfidenceHigh {
		t.Fatalf("classification = %#v, error = %v", classification, err)
	}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
