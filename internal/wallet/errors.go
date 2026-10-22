package wallet

import "errors"

var (
	ErrInvalidWallet         = errors.New("wallet is invalid")
	ErrInvalidAccountSet     = errors.New("wallet account set is invalid")
	ErrOnboardingUnavailable = errors.New("wallet onboarding dependencies are unavailable")
	ErrInvalidIdempotencyKey = errors.New("idempotency key is required")
	ErrIdempotencyConflict   = errors.New("idempotency key was reused for different onboarding input")
	ErrUserExists            = errors.New("a user with this email already exists")
	ErrWalletExists          = errors.New("a wallet for this user already exists")
	ErrProvisioningExists    = errors.New("provisioning IDs already exist")
	ErrProvisioningNotFound  = errors.New("wallet provisioning was not found")
)
