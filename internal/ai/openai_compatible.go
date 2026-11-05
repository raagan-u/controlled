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

func (m *OpenAICompatibleModel) ClassifyTransaction(ctx context.Context, input ClassificationInput) (Classification, error) {
	if m == nil || m.client == nil {
		return Classification{}, ErrUnavailable
	}
	contextJSON, err := json.Marshal(input)
	if err != nil {
		return Classification{}, ErrInvalidResponse
	}
	payload := map[string]any{
		"model": m.model, "temperature": 0,
		"messages": []map[string]string{
			{"role": "system", "content": classificationPrompt},
			{"role": "user", "content": "Classify only this server-verified context:\n" + string(contextJSON)},
		},
		"response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "controlled_transaction_relationship", "strict": true, "schema": classificationSchema()}},
	}
	if m.requireParameters {
		payload["provider"] = map[string]bool{"require_parameters": true}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Classification{}, ErrUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint, bytes.NewReader(body))
	if err != nil {
		return Classification{}, ErrUnavailable
	}
	request.Header.Set("Authorization", "Bearer "+m.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := m.client.Do(request)
	if err != nil {
		return Classification{}, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusPaymentRequired {
		return Classification{}, ErrRateLimited
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Classification{}, ErrUnavailable
	}
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxProviderResponseBytes)).Decode(&envelope); err != nil || len(envelope.Choices) == 0 {
		return Classification{}, ErrInvalidResponse
	}
	var result Classification
	decoder := json.NewDecoder(strings.NewReader(envelope.Choices[0].Message.Content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return Classification{}, fmt.Errorf("%w: decode classification", ErrInvalidResponse)
	}
	return result, nil
}

const systemPrompt = `You are the read-only guidance layer for Controlled, a wallet with user-created spending pacts.
The provided JSON is server-verified. Never infer missing transactions, balances, people, merchants, or categories.
Return supportive, concise language without shame, diagnosis, certainty, or financial advice.
The headline and message must contain no digits, currency symbols, amounts, percentages, or dates; the application renders verified numeric facts separately.
For PACT_CREATION, suggest conservative terms in integer USD minor units and a duration in whole days.
For every other kind, all three suggested term fields must be zero.
For TRANSACTION_PREVIEW, prioritize the spending pattern, then the proposed reason's relationship to the pact, then the forecast. Treat ALIGNED as potentially aligned, not proof. Keep UNCLEAR neutral. Become firmer only when forecastWarning is true, while sounding like someone who cares about the user's stated goal.
For ACCOUNTABILITY_BRIEF, requesterName is the person proposing the payment and the current reader is their reviewer. The message must begin exactly with: "<requesterName> raised this request." Continue in the third person and never imply that requesterName is reviewing or approving the request.
Do not authorize, approve, execute, or imply that any money movement has occurred.`

const classificationPrompt = `You classify how a proposed wallet transaction reason relates to one user-created spending pact.
Use only the supplied goal, intent, transaction reason, and purchase flag.
Return ALIGNED only when the reason clearly supports the pact goal or intent. This means potentially aligned, never exempt from the spending limit.
Return CONFLICTING only when the reason clearly works against the stated goal or intent.
Return EXCEPTIONAL when the reason clearly describes a one-time exception or when the pact specifically targets this type of exceptional purchase.
Return UNCLEAR when the reason is generic, ambiguous, or lacks enough context.
Use HIGH confidence only for explicit textual evidence. Otherwise use MEDIUM or LOW.
Keep the explanation concise and do not make a financial decision.`

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

func classificationSchema() map[string]any {
	return map[string]any{
		"type": "object", "additionalProperties": false,
		"required": []string{"relationship", "confidence", "reason"},
		"properties": map[string]any{
			"relationship": map[string]any{"type": "string", "enum": []string{"ALIGNED", "CONFLICTING", "EXCEPTIONAL", "UNCLEAR"}},
			"confidence":   map[string]any{"type": "string", "enum": []string{"HIGH", "MEDIUM", "LOW"}},
			"reason":       map[string]any{"type": "string", "maxLength": 180},
		},
	}
}
