package identity

import (
	"context"
	"strings"

	"google.golang.org/api/idtoken"
)

type GoogleVerifier struct {
	clientID string
}

func NewGoogleVerifier(clientID string) *GoogleVerifier {
	return &GoogleVerifier{clientID: strings.TrimSpace(clientID)}
}

func (v *GoogleVerifier) Verify(ctx context.Context, rawToken string) (GoogleProfile, error) {
	if v == nil || v.clientID == "" || strings.TrimSpace(rawToken) == "" {
		return GoogleProfile{}, ErrInvalidGoogleToken
	}
	payload, err := idtoken.Validate(ctx, rawToken, v.clientID)
	if err != nil || (payload.Issuer != "accounts.google.com" && payload.Issuer != "https://accounts.google.com") {
		return GoogleProfile{}, ErrInvalidGoogleToken
	}
	email, emailOK := payload.Claims["email"].(string)
	verified, verifiedOK := payload.Claims["email_verified"].(bool)
	subject := strings.TrimSpace(payload.Subject)
	email = strings.ToLower(strings.TrimSpace(email))
	if subject == "" || !emailOK || !verifiedOK || !verified || !validEmail(email) {
		return GoogleProfile{}, ErrInvalidGoogleToken
	}

	displayName, _ := payload.Claims["name"].(string)
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = strings.SplitN(email, "@", 2)[0]
	}
	return GoogleProfile{Subject: subject, Email: email, DisplayName: displayName}, nil
}
