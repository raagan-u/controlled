package wallet

import (
	"errors"
	"net/http"
	"time"

	"github.com/raagan-u/pay-pact/internal/identity"
	"github.com/raagan-u/pay-pact/internal/platform/httpapi"
)

type ReadHandler struct {
	reader Reader
}

func NewReadHandler(reader Reader) *ReadHandler {
	return &ReadHandler{reader: reader}
}

func (h *ReadHandler) Wallet(response http.ResponseWriter, request *http.Request) {
	user, ok := identity.CurrentUser(request.Context())
	if !ok {
		httpapi.WriteError(response, http.StatusUnauthorized, "authentication_required", "A valid PayPact session is required.")
		return
	}
	snapshot, err := h.reader.WalletByUser(request.Context(), user.ID())
	if err != nil {
		if errors.Is(err, ErrProvisioningNotFound) {
			httpapi.WriteError(response, http.StatusNotFound, "wallet_not_found", "The PayPact wallet was not found.")
			return
		}
		httpapi.WriteError(response, http.StatusInternalServerError, "internal_error", "PayPact could not read the wallet.")
		return
	}
	httpapi.WriteJSON(response, http.StatusOK, walletReadResponse(snapshot))
}

type walletAPIResponse struct {
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
}

func walletReadResponse(snapshot Snapshot) struct {
	Wallet walletAPIResponse `json:"wallet"`
} {
	wallet := snapshot.Wallet()
	accounts := wallet.Accounts()
	balances := snapshot.Balances()
	response := struct {
		Wallet walletAPIResponse `json:"wallet"`
	}{}
	response.Wallet.ID = string(wallet.ID())
	response.Wallet.OwnerID = string(wallet.OwnerID())
	response.Wallet.Currency = wallet.Currency()
	response.Wallet.Accounts.Available = string(accounts.Available)
	response.Wallet.Accounts.Pending = string(accounts.Pending)
	response.Wallet.Accounts.CommitmentReserve = string(accounts.CommitmentReserve)
	response.Wallet.Balances.Available = moneyResponse{
		AmountMinor: balances.Available.AmountMinor(),
		Currency:    balances.Available.Currency(),
	}
	response.Wallet.Balances.Pending = moneyResponse{
		AmountMinor: balances.Pending.AmountMinor(),
		Currency:    balances.Pending.Currency(),
	}
	response.Wallet.Balances.CommitmentReserve = moneyResponse{
		AmountMinor: balances.CommitmentReserve.AmountMinor(),
		Currency:    balances.CommitmentReserve.Currency(),
	}
	response.Wallet.CreatedAt = wallet.CreatedAt()
	return response
}
