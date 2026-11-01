package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxProviderResponseBytes = 1 << 20

type OpenAICompatibleOptions struct {
	BaseURL           string
	Model             string
	APIKey            string
	Timeout           time.Duration
	RequireParameters bool
	HTTPClient        *http.Client
}

type OpenAICompatibleModel struct {
	endpoint          string
	model             string
	apiKey            string
	requireParameters bool
	client            *http.Client
}

func NewOpenAICompatibleModel(options OpenAICompatibleOptions) (*OpenAICompatibleModel, error) {
	base, err := url.Parse(strings.TrimRight(strings.TrimSpace(options.BaseURL), "/"))
	if err != nil || base.Scheme == "" || base.Host == "" || strings.TrimSpace(options.Model) == "" || strings.TrimSpace(options.APIKey) == "" || options.Timeout <= 0 {
		return nil, ErrUnavailable
	}
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: options.Timeout}
	}
	return &OpenAICompatibleModel{endpoint: base.String() + "/chat/completions", model: strings.TrimSpace(options.Model), apiKey: strings.TrimSpace(options.APIKey), requireParameters: options.RequireParameters, client: client}, nil
}

func (m *OpenAICompatibleModel) Draft(ctx context.Context, input ModelInput) (ModelDraft, error) {
	if m == nil || m.client == nil {
		return ModelDraft{}, ErrUnavailable
	}
	contextJSON, err := json.Marshal(input)
	if err != nil {
		return ModelDraft{}, ErrInvalidResponse
	}
	payload := map[string]any{
		"model":       m.model,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": "Use only this server-verified context:\n" + string(contextJSON)},
		},
		"response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "controlled_pact_guidance", "strict": true, "schema": guidanceSchema()}},
	}
	if m.requireParameters {
		payload["provider"] = map[string]bool{"require_parameters": true}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ModelDraft{}, ErrUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint, bytes.NewReader(body))
	if err != nil {
		return ModelDraft{}, ErrUnavailable
	}
	request.Header.Set("Authorization", "Bearer "+m.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := m.client.Do(request)
	if err != nil {
		return ModelDraft{}, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusPaymentRequired {
		return ModelDraft{}, ErrRateLimited
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ModelDraft{}, ErrUnavailable
	}
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxProviderResponseBytes))
	if err := decoder.Decode(&envelope); err != nil || len(envelope.Choices) == 0 {
		return ModelDraft{}, ErrInvalidResponse
	}
	var draft ModelDraft
	decoder = json.NewDecoder(strings.NewReader(envelope.Choices[0].Message.Content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&draft); err != nil {
		return ModelDraft{}, fmt.Errorf("%w: decode structured output", ErrInvalidResponse)
	}
	return draft, nil
}

const systemPrompt = `You are the read-only guidance layer for Controlled, a wallet with user-created spending pacts.
The provided JSON is server-verified. Never infer missing transactions, balances, people, merchants, or categories.
Return supportive, concise language without shame, diagnosis, certainty, or financial advice.
The headline and message must contain no digits, currency symbols, amounts, percentages, or dates; the application renders verified numeric facts separately.
For PACT_CREATION, suggest conservative terms in integer USD minor units and a duration in whole days.
For every other kind, all three suggested term fields must be zero.
Do not authorize, approve, execute, or imply that any money movement has occurred.`

func guidanceSchema() map[string]any {
	return map[string]any{
		"type": "object", "additionalProperties": false,
		"required": []string{"headline", "message", "tone", "suggestedLimitMinor", "suggestedStakeMinor", "suggestedDurationDays"},
		"properties": map[string]any{
			"headline":              map[string]any{"type": "string", "maxLength": 80},
			"message":               map[string]any{"type": "string", "maxLength": 280},
			"tone":                  map[string]any{"type": "string", "enum": []string{"NEUTRAL", "POSITIVE", "CAUTION", "CRITICAL"}},
			"suggestedLimitMinor":   map[string]any{"type": "integer"},
			"suggestedStakeMinor":   map[string]any{"type": "integer"},
			"suggestedDurationDays": map[string]any{"type": "integer"},
		},
	}
}
