package wallet

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/raagan-u/pay-pact/internal/identity"
	"github.com/raagan-u/pay-pact/internal/platform/httpapi"
)

type Onboarder interface {
	Onboard(context.Context, string, OnboardingRequest) (OnboardingResult, error)
}

type OnboardingHandler struct {
	onboarder Onboarder
}

func NewOnboardingHandler(onboarder Onboarder) *OnboardingHandler {
	return &OnboardingHandler{onboarder: onboarder}
}

func (h *OnboardingHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", http.MethodPost)
		httpapi.WriteError(response, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is supported.")
		return
	}

	var input struct {
		Email       string `json:"email"`
		DisplayName string `json:"displayName"`
		Currency    string `json:"currency"`
	}
	if err := httpapi.ReadJSON(response, request, &input, httpapi.DefaultMaxBodyBytes); err != nil {
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_request", "Request body must be a valid onboarding object.")
		return
	}

	result, err := h.onboarder.Onboard(
		request.Context(),
		request.Header.Get("Idempotency-Key"),
		OnboardingRequest{
			Email:       input.Email,
			DisplayName: input.DisplayName,
			Currency:    input.Currency,
		},
	)
	if err != nil {
		writeOnboardingError(response, err)
		return
	}

	status := http.StatusCreated
	if result.Duplicate {
		status = http.StatusOK
	}
	httpapi.WriteJSON(response, status, onboardingResponse(result))
}

type moneyResponse struct {
	AmountMinor int64  `json:"amountMinor"`
	Currency    string `json:"currency"`
}

type onboardingAPIResponse struct {
	User struct {
		ID          string    `json:"id"`
		Email       string    `json:"email"`
		DisplayName string    `json:"displayName"`
		CreatedAt   time.Time `json:"createdAt"`
	} `json:"user"`
	Wallet struct {
		ID       string `json:"id"`
		OwnerID  string `json:"ownerId"`
		Currency string `json:"currency"`
		Accounts struct {
			Available         string `json:"available"`
			Pending           string `json:"pending"`
			CommitmentReserve string `json:"commitmentReserve"`
		} `json:"accounts"`
		Balances struct {
			Available         moneyResponse `json:"available"`
			Pending           moneyResponse `json:"pending"`
			CommitmentReserve moneyResponse `json:"commitmentReserve"`
		} `json:"balances"`
		CreatedAt time.Time `json:"createdAt"`
	} `json:"wallet"`
	Duplicate bool `json:"duplicate"`
}

func onboardingResponse(result OnboardingResult) onboardingAPIResponse {
	provisioning := result.Provisioning
	user := provisioning.User()
	wallet := provisioning.Wallet()
	accounts := wallet.Accounts()

	response := onboardingAPIResponse{Duplicate: result.Duplicate}
	response.User.ID = string(user.ID())
	response.User.Email = user.Email()
	response.User.DisplayName = user.DisplayName()
	response.User.CreatedAt = user.CreatedAt()
	response.Wallet.ID = string(wallet.ID())
	response.Wallet.OwnerID = string(wallet.OwnerID())
	response.Wallet.Currency = wallet.Currency()
	response.Wallet.Accounts.Available = string(accounts.Available)
	response.Wallet.Accounts.Pending = string(accounts.Pending)
	response.Wallet.Accounts.CommitmentReserve = string(accounts.CommitmentReserve)
	response.Wallet.Balances.Available = moneyResponse{Currency: wallet.Currency()}
	response.Wallet.Balances.Pending = moneyResponse{Currency: wallet.Currency()}
	response.Wallet.Balances.CommitmentReserve = moneyResponse{Currency: wallet.Currency()}
	response.Wallet.CreatedAt = wallet.CreatedAt()
	return response
}

func writeOnboardingError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidIdempotencyKey):
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_idempotency_key", "A non-empty Idempotency-Key header is required.")
	case errors.Is(err, identity.ErrInvalidUserID),
		errors.Is(err, identity.ErrInvalidEmail),
		errors.Is(err, identity.ErrInvalidDisplayName),
		errors.Is(err, identity.ErrInvalidCreatedAt),
		errors.Is(err, ErrInvalidWallet),
		errors.Is(err, ErrInvalidAccountSet):
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "invalid_onboarding", "Email, display name, and a supported currency are required.")
	case errors.Is(err, ErrIdempotencyConflict):
		httpapi.WriteError(response, http.StatusConflict, "idempotency_conflict", "This Idempotency-Key was already used with different input.")
	case errors.Is(err, ErrUserExists), errors.Is(err, ErrWalletExists), errors.Is(err, ErrProvisioningExists):
		httpapi.WriteError(response, http.StatusConflict, "user_exists", "A PayPact user already exists for this email.")
	default:
		httpapi.WriteError(response, http.StatusInternalServerError, "internal_error", "PayPact could not create the wallet.")
	}
}
