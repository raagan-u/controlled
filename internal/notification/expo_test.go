package notification

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestExpoSenderBuildsActionNotification(t *testing.T) {
	var received []expoMessage
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer expo-access-token" {
			t.Errorf("Authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode request: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(`{"data":[{"status":"ok"},{"status":"error","details":{"error":"DeviceNotRegistered"}}]}`)),
		}, nil
	})}

	sender, err := NewExpoSender("https://exp.host/--/api/v2/push/send", "expo-access-token", client)
	if err != nil {
		t.Fatalf("NewExpoSender() error = %v", err)
	}
	tokens := []string{"ExpoPushToken[valid]", "ExpoPushToken[stale]"}
	message := Message{Title: "Payment request", Body: "Alex requested $12.00.", Kind: KindPaymentRequest, EntityID: "payment-1"}
	invalid, err := sender.Send(t.Context(), tokens, message)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if !reflect.DeepEqual(invalid, []string{tokens[1]}) {
		t.Fatalf("invalid tokens = %v", invalid)
	}
	if len(received) != 2 {
		t.Fatalf("message count = %d, want 2", len(received))
	}
	if received[0].To != tokens[0] || received[0].Title != message.Title || received[0].Body != message.Body ||
		received[0].ChannelID != "actions-v2" || received[0].Data["type"] != message.Kind || received[0].Data["entityId"] != message.EntityID {
		t.Fatalf("message = %#v", received[0])
	}
}
