package wallet

import (
	"errors"
	"net/http"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/ledger"
	"github.com/raagan-u/controlled/internal/platform/httpapi"
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
		httpapi.WriteError(response, http.StatusUnauthorized, "authentication_required", "A valid Controlled session is required.")
		return
	}
	snapshot, err := h.reader.WalletByUser(request.Context(), user.ID())
	if err != nil {
		if errors.Is(err, ErrProvisioningNotFound) {
			httpapi.WriteError(response, http.StatusNotFound, "wallet_not_found", "The Controlled wallet was not found.")
			return
		}
		httpapi.WriteError(response, http.StatusInternalServerError, "internal_error", "Controlled could not read the wallet.")
		return
	}
	httpapi.WriteJSON(response, http.StatusOK, walletReadResponse(snapshot))
}

func (h *ReadHandler) Transactions(response http.ResponseWriter, request *http.Request) {
	user, ok := identity.CurrentUser(request.Context())
	if !ok {
		httpapi.WriteError(response, http.StatusUnauthorized, "authentication_required", "A valid Controlled session is required.")
		return
	}
	transactions, err := h.reader.TransactionsByUser(request.Context(), user.ID(), 50)
	if err != nil {
		httpapi.WriteError(response, http.StatusInternalServerError, "internal_error", "Controlled could not read wallet transactions.")
		return
	}

	items := make([]transactionAPIResponse, 0, len(transactions))
	for _, transaction := range transactions {
		items = append(items, transactionResponse(transaction))
	}
	httpapi.WriteJSON(response, http.StatusOK, struct {
		Transactions []transactionAPIResponse `json:"transactions"`
	}{Transactions: items})
}

func (h *ReadHandler) Transaction(response http.ResponseWriter, request *http.Request) {
	user, ok := identity.CurrentUser(request.Context())
	if !ok {
		httpapi.WriteError(response, http.StatusUnauthorized, "authentication_required", "A valid Controlled session is required.")
		return
	}
	transaction, err := h.reader.TransactionByUser(
		request.Context(), user.ID(), ledger.JournalID(request.PathValue("id")),
	)
	if errors.Is(err, ErrTransactionNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "transaction_not_found", "The wallet transaction was not found.")
		return
	}
	if err != nil {
		httpapi.WriteError(response, http.StatusInternalServerError, "internal_error", "Controlled could not read the wallet transaction.")
		return
	}
	httpapi.WriteJSON(response, http.StatusOK, struct {
		Transaction transactionAPIResponse `json:"transaction"`
	}{Transaction: transactionResponse(transaction)})
}

func transactionResponse(transaction Transaction) transactionAPIResponse {
	item := transactionAPIResponse{
		ID:          string(transaction.ID),
		Reference:   transactionReferenceAPIResponse{Type: transaction.Reference.Type, ID: transaction.Reference.ID},
		Description: transaction.Description, OccurredAt: transaction.OccurredAt,
		PostedAt: transaction.PostedAt, Reversed: transaction.Reversed,
		Entries: make([]transactionEntryAPIResponse, 0, len(transaction.Entries)),
	}
	if transaction.Transfer != nil {
		item.Transfer = &transferDetailsAPIResponse{
			Sender:    partyAPIResponse{ID: string(transaction.Transfer.Sender.ID), CtrldID: transaction.Transfer.Sender.CtrldID, Email: transaction.Transfer.Sender.Email, DisplayName: transaction.Transfer.Sender.DisplayName},
			Recipient: partyAPIResponse{ID: string(transaction.Transfer.Recipient.ID), CtrldID: transaction.Transfer.Recipient.CtrldID, Email: transaction.Transfer.Recipient.Email, DisplayName: transaction.Transfer.Recipient.DisplayName},
			Remark:    transaction.Transfer.Remark, PaymentRequestID: transaction.Transfer.PaymentRequestID,
		}
	}
	if transaction.Approval != nil {
		item.Approval = &approvalDetailsAPIResponse{
			RequestID: transaction.Approval.RequestID, Reason: transaction.Approval.Reason,
			Classification: transaction.Approval.Classification,
			ApprovedBy:     partyAPIResponse{ID: string(transaction.Approval.ApprovedBy.ID), CtrldID: transaction.Approval.ApprovedBy.CtrldID, Email: transaction.Approval.ApprovedBy.Email, DisplayName: transaction.Approval.ApprovedBy.DisplayName},
		}
	}
	for _, entry := range transaction.Entries {
		item.Entries = append(item.Entries, transactionEntryAPIResponse{
			AccountID: string(entry.AccountID), AccountKind: string(entry.AccountKind), Side: string(entry.Side),
			Amount: moneyResponse{AmountMinor: entry.Amount.AmountMinor(), Currency: entry.Amount.Currency()},
		})
	}
	return item
}

type transactionAPIResponse struct {
	ID          string                          `json:"id"`
	Reference   transactionReferenceAPIResponse `json:"reference"`
	Description string                          `json:"description"`
	OccurredAt  time.Time                       `json:"occurredAt"`
	PostedAt    time.Time                       `json:"postedAt"`
	Reversed    bool                            `json:"reversed"`
	Entries     []transactionEntryAPIResponse   `json:"entries"`
	Transfer    *transferDetailsAPIResponse     `json:"transferDetails,omitempty"`
	Approval    *approvalDetailsAPIResponse     `json:"approvalDetails,omitempty"`
}

type approvalDetailsAPIResponse struct {
	RequestID      string           `json:"requestId"`
	Reason         string           `json:"reason"`
	Classification string           `json:"classification"`
	ApprovedBy     partyAPIResponse `json:"approvedBy"`
}

type partyAPIResponse struct {
	ID          string `json:"id"`
	CtrldID     string `json:"ctrldId"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}
type transferDetailsAPIResponse struct {
	Sender           partyAPIResponse `json:"sender"`
	Recipient        partyAPIResponse `json:"recipient"`
	Remark           string           `json:"remark,omitempty"`
	PaymentRequestID string           `json:"paymentRequestId,omitempty"`
}

type transactionReferenceAPIResponse struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type transactionEntryAPIResponse struct {
	AccountID   string        `json:"accountId"`
	AccountKind string        `json:"accountKind"`
	Side        string        `json:"side"`
	Amount      moneyResponse `json:"amount"`
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
		Staked            moneyResponse `json:"staked"`
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
	response.Wallet.Balances.Staked = moneyResponse{
		AmountMinor: balances.Staked.AmountMinor(),
		Currency:    balances.Staked.Currency(),
	}
	response.Wallet.Balances.CommitmentReserve = moneyResponse{
		AmountMinor: balances.CommitmentReserve.AmountMinor(),
		Currency:    balances.CommitmentReserve.Currency(),
	}
	response.Wallet.CreatedAt = wallet.CreatedAt()
	return response
}
