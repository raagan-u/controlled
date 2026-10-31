package main

import (
	"context"
	"errors"

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

func (p googleUserProvisioner) ProvisionPasswordUser(
	ctx context.Context,
	email string,
	displayName string,
	namePending bool,
) (identity.User, error) {
	result, err := p.onboarding.Onboard(ctx, "password:"+email, wallet.OnboardingRequest{
		Email:       email,
		DisplayName: displayName,
		NamePending: namePending,
		Currency:    "USD",
	})
	if err != nil {
		if errors.Is(err, wallet.ErrUserExists) {
			return identity.User{}, identity.ErrEmailAlreadyRegistered
		}
		return identity.User{}, err
	}
	return result.Provisioning.User(), nil
}
