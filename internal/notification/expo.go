package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type ExpoSender struct {
	url         string
	accessToken string
	client      *http.Client
}

func NewExpoSender(url, accessToken string, client *http.Client) (*ExpoSender, error) {
	url = strings.TrimSpace(url)
	if url == "" || client == nil {
		return nil, ErrUnavailable
	}
	return &ExpoSender{url: url, accessToken: strings.TrimSpace(accessToken), client: client}, nil
}

func (s *ExpoSender) Send(ctx context.Context, tokens []string, message Message) ([]string, error) {
	if s == nil || s.client == nil || s.url == "" {
		return nil, ErrUnavailable
	}
	payload := make([]expoMessage, len(tokens))
	for index, token := range tokens {
		payload[index] = expoMessage{
			To: token, Title: message.Title, Body: message.Body, Sound: "default",
			Priority: "high", ChannelID: "actions-v3",
			Data: map[string]string{"type": message.Kind, "entityId": message.EntityID},
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode Expo push: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create Expo push: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	if s.accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+s.accessToken)
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send Expo push: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("send Expo push: status %d", response.StatusCode)
	}
	var result struct {
		Data []struct {
			Status  string `json:"status"`
			Details struct {
				Error string `json:"error"`
			} `json:"details"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Expo push: %w", err)
	}
	var invalid []string
	for index, ticket := range result.Data {
		if index < len(tokens) && ticket.Status == "error" && ticket.Details.Error == "DeviceNotRegistered" {
			invalid = append(invalid, tokens[index])
		}
	}
	return invalid, nil
}

type expoMessage struct {
	To        string            `json:"to"`
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	Sound     string            `json:"sound"`
	Priority  string            `json:"priority"`
	ChannelID string            `json:"channelId"`
	Data      map[string]string `json:"data"`
}
