package main

import (
	"context"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/wallet"
)

type googleUserProvisioner struct {
	onboarding *wallet.OnboardingService
}

func (p googleUserProvisioner) ProvisionGoogleUser(
	ctx context.Context,
	profile identity.GoogleProfile,
) (identity.User, error) {
	result, err := p.onboarding.Onboard(ctx, "google:"+profile.Subject, wallet.OnboardingRequest{
		Email:       profile.Email,
		DisplayName: profile.DisplayName,
		Currency:    "USD",
	})
	if err != nil {
		return identity.User{}, err
	}
	return result.Provisioning.User(), nil
}
