package walletops

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/raagan-u/controlled/internal/custody"
	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/ledger"
	"github.com/raagan-u/controlled/internal/notification"
	"github.com/raagan-u/controlled/internal/platform/httpapi"
)

type Commander interface {
	CreateFunding(context.Context, string, identity.UserID, int64, string) (FundingIntent, error)
	ConfirmFunding(context.Context, identity.UserID, OperationID) (FundingIntent, error)
	Transfer(context.Context, string, identity.UserID, string, int64, string) (Transfer, error)
	CreateWithdrawal(context.Context, string, identity.UserID, string, int64, string) (Withdrawal, error)
	ReconcileWithdrawal(context.Context, identity.UserID, OperationID) (Withdrawal, error)
	SimulateWithdrawalSettlement(context.Context, identity.UserID, OperationID, bool) (Withdrawal, error)
}

type accountableCommander interface {
	TransferWithReason(context.Context, string, identity.UserID, string, int64, string, string) (DebitAttempt, error)
	CreateWithdrawalWithReason(context.Context, string, identity.UserID, string, int64, string, string) (DebitAttempt, error)
}

type guidedCommander interface {
	TransferWithGuidance(context.Context, string, identity.UserID, string, int64, string, string, string, bool) (DebitAttempt, error)
}

type Handler struct {
	commands      Commander
	notifications notification.Notifier
}

func NewHandler(commands Commander) *Handler {
	return &Handler{commands: commands}
}

func (h *Handler) EnableNotifications(notifier notification.Notifier) *Handler {
	if h != nil {
		h.notifications = notifier
	}
	return h
}

func (h *Handler) CreateFunding(response http.ResponseWriter, request *http.Request) {
	user, ok := identity.CurrentUser(request.Context())
	if !ok {
		writeAuthenticationRequired(response)
		return
	}
	var input moneyInput
	if err := httpapi.ReadJSON(response, request, &input, httpapi.DefaultMaxBodyBytes); err != nil {
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_request", "Request body must contain a funding amount.")
		return
	}
	intent, err := h.commands.CreateFunding(
		request.Context(), request.Header.Get("Idempotency-Key"), user.ID(), input.AmountMinor, input.Currency,
	)
	if err != nil {
		writeOperationError(response, err)
		return
	}
	status := http.StatusCreated
	if intent.Duplicate {
		status = http.StatusOK
	}
	httpapi.WriteJSON(response, status, fundingResponse(intent))
}

func (h *Handler) ConfirmFunding(response http.ResponseWriter, request *http.Request) {
	user, ok := identity.CurrentUser(request.Context())
	if !ok {
		writeAuthenticationRequired(response)
		return
	}
	intent, err := h.commands.ConfirmFunding(
		request.Context(), user.ID(), OperationID(request.PathValue("id")),
	)
	if err != nil {
		writeOperationError(response, err)
		return
	}
	httpapi.WriteJSON(response, http.StatusOK, fundingResponse(intent))
}

func (h *Handler) CreateTransfer(response http.ResponseWriter, request *http.Request) {
	user, ok := identity.CurrentUser(request.Context())
	if !ok {
		writeAuthenticationRequired(response)
		return
	}
	var input struct {
		RecipientEmail string `json:"recipientEmail"`
		AmountMinor    int64  `json:"amountMinor"`
		Currency       string `json:"currency"`
		Reason         string `json:"reason"`
		AssessmentID   string `json:"assessmentId"`
		IsPurchase     bool   `json:"isPurchase"`
	}
	if err := httpapi.ReadJSON(response, request, &input, httpapi.DefaultMaxBodyBytes); err != nil {
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_request", "Request body must contain a recipient and amount.")
		return
	}
	var transfer Transfer
	var review *ReviewRequest
	var err error
	if commands, ok := h.commands.(guidedCommander); ok {
		attempt, attemptErr := commands.TransferWithGuidance(
			request.Context(), request.Header.Get("Idempotency-Key"), user.ID(),
			input.RecipientEmail, input.AmountMinor, input.Currency, input.Reason, input.AssessmentID, input.IsPurchase,
		)
		err = attemptErr
		if attempt.Transfer != nil {
			transfer = *attempt.Transfer
		}
		review = attempt.Request
	} else if commands, ok := h.commands.(accountableCommander); ok {
		attempt, attemptErr := commands.TransferWithReason(
			request.Context(), request.Header.Get("Idempotency-Key"), user.ID(),
			input.RecipientEmail, input.AmountMinor, input.Currency, input.Reason,
		)
		err = attemptErr
		if attempt.Transfer != nil {
			transfer = *attempt.Transfer
		}
		review = attempt.Request
	} else {
		transfer, err = h.commands.Transfer(
			request.Context(), request.Header.Get("Idempotency-Key"), user.ID(),
			input.RecipientEmail, input.AmountMinor, input.Currency,
		)
	}
	if err != nil {
		writeOperationError(response, err)
		return
	}
	if review != nil {
		if !review.Duplicate {
			h.notify(review.AssignedPartnerID, notification.Message{Title: "Approval needed", Body: fmt.Sprintf("%s needs approval to send $%.2f", user.DisplayName(), float64(input.AmountMinor)/100), Kind: notification.KindAccountabilityRequest, EntityID: review.ID})
		}
		httpapi.WriteJSON(response, http.StatusAccepted, reviewRequestResponse(*review))
		return
	}
	status := http.StatusCreated
	if transfer.Duplicate {
		status = http.StatusOK
	}
	httpapi.WriteJSON(response, status, transferResponse(transfer))
}

func (h *Handler) CreateWithdrawal(response http.ResponseWriter, request *http.Request) {
	user, ok := identity.CurrentUser(request.Context())
	if !ok {
		writeAuthenticationRequired(response)
		return
	}
	var input struct {
		Destination string `json:"destination"`
		AmountMinor int64  `json:"amountMinor"`
		Currency    string `json:"currency"`
		Reason      string `json:"reason"`
	}
	if err := httpapi.ReadJSON(response, request, &input, httpapi.DefaultMaxBodyBytes); err != nil {
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_request", "Request body must contain a destination and amount.")
		return
	}
	var withdrawal Withdrawal
	var review *ReviewRequest
	var err error
	if commands, ok := h.commands.(accountableCommander); ok {
		attempt, attemptErr := commands.CreateWithdrawalWithReason(
			request.Context(), request.Header.Get("Idempotency-Key"), user.ID(),
			input.Destination, input.AmountMinor, input.Currency, input.Reason,
		)
		err = attemptErr
		if attempt.Withdrawal != nil {
			withdrawal = *attempt.Withdrawal
		}
		review = attempt.Request
	} else {
		withdrawal, err = h.commands.CreateWithdrawal(
			request.Context(), request.Header.Get("Idempotency-Key"), user.ID(),
			input.Destination, input.AmountMinor, input.Currency,
		)
	}
	if err != nil {
		writeOperationError(response, err)
		return
	}
	if review != nil {
		if !review.Duplicate {
			h.notify(review.AssignedPartnerID, notification.Message{Title: "Approval needed", Body: fmt.Sprintf("%s needs approval to withdraw $%.2f", user.DisplayName(), float64(input.AmountMinor)/100), Kind: notification.KindAccountabilityRequest, EntityID: review.ID})
		}
		httpapi.WriteJSON(response, http.StatusAccepted, reviewRequestResponse(*review))
		return
	}
	httpapi.WriteJSON(response, http.StatusAccepted, withdrawalResponse(withdrawal))
}

func (h *Handler) notify(recipient identity.UserID, message notification.Message) {
	if h != nil && h.notifications != nil {
		h.notifications.Notify(recipient, message)
	}
}

func reviewRequestResponse(review ReviewRequest) struct {
	AccountabilityRequest struct {
		ID                string    `json:"id"`
		IntentID          string    `json:"intentId"`
		Status            string    `json:"status"`
		AssignedPartnerID string    `json:"assignedPartnerId"`
		CreatedAt         time.Time `json:"createdAt"`
		ExpiresAt         time.Time `json:"expiresAt"`
		Duplicate         bool      `json:"duplicate"`
	} `json:"accountabilityRequest"`
} {
	var response struct {
		AccountabilityRequest struct {
			ID                string    `json:"id"`
			IntentID          string    `json:"intentId"`
			Status            string    `json:"status"`
			AssignedPartnerID string    `json:"assignedPartnerId"`
			CreatedAt         time.Time `json:"createdAt"`
			ExpiresAt         time.Time `json:"expiresAt"`
			Duplicate         bool      `json:"duplicate"`
		} `json:"accountabilityRequest"`
	}
	response.AccountabilityRequest.ID = review.ID
	response.AccountabilityRequest.IntentID = string(review.IntentID)
	response.AccountabilityRequest.Status = review.Status
	response.AccountabilityRequest.AssignedPartnerID = string(review.AssignedPartnerID)
	response.AccountabilityRequest.CreatedAt = review.CreatedAt
	response.AccountabilityRequest.ExpiresAt = review.ExpiresAt
	response.AccountabilityRequest.Duplicate = review.Duplicate
	return response
}

func (h *Handler) SettleWithdrawal(response http.ResponseWriter, request *http.Request) {
	user, ok := identity.CurrentUser(request.Context())
	if !ok {
		writeAuthenticationRequired(response)
		return
	}
	var input struct {
		Succeeded *bool `json:"succeeded"`
	}
	if err := httpapi.ReadJSON(response, request, &input, httpapi.DefaultMaxBodyBytes); err != nil || input.Succeeded == nil {
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_request", "Request body must specify whether the fake payout succeeded.")
		return
	}
	withdrawal, err := h.commands.SimulateWithdrawalSettlement(
		request.Context(), user.ID(), OperationID(request.PathValue("id")), *input.Succeeded,
	)
	if err != nil {
		writeOperationError(response, err)
		return
	}
	httpapi.WriteJSON(response, http.StatusOK, withdrawalResponse(withdrawal))
}

func (h *Handler) ReconcileWithdrawal(response http.ResponseWriter, request *http.Request) {
	user, ok := identity.CurrentUser(request.Context())
	if !ok {
		writeAuthenticationRequired(response)
		return
	}
	withdrawal, err := h.commands.ReconcileWithdrawal(
		request.Context(), user.ID(), OperationID(request.PathValue("id")),
	)
	if err != nil {
		writeOperationError(response, err)
		return
	}
	httpapi.WriteJSON(response, http.StatusOK, withdrawalResponse(withdrawal))
}

type moneyInput struct {
	AmountMinor int64  `json:"amountMinor"`
	Currency    string `json:"currency"`
}

type moneyOutput struct {
	AmountMinor int64  `json:"amountMinor"`
	Currency    string `json:"currency"`
}

type fundingAPIResponse struct {
	ID                  string        `json:"id"`
	Status              FundingStatus `json:"status"`
	Amount              moneyOutput   `json:"amount"`
	GrossAmount         *moneyOutput  `json:"grossAmount,omitempty"`
	ProviderOrderID     string        `json:"providerOrderId,omitempty"`
	ApprovalURL         string        `json:"approvalUrl,omitempty"`
	LedgerTransactionID string        `json:"ledgerTransactionId,omitempty"`
	CreatedAt           time.Time     `json:"createdAt"`
	UpdatedAt           time.Time     `json:"updatedAt"`
	CompletedAt         *time.Time    `json:"completedAt,omitempty"`
	Duplicate           bool          `json:"duplicate"`
}

func fundingResponse(intent FundingIntent) struct {
	FundingIntent fundingAPIResponse `json:"fundingIntent"`
} {
	item := fundingAPIResponse{
		ID: string(intent.ID), Status: intent.Status,
		Amount:          moneyOutput{AmountMinor: intent.Amount.AmountMinor(), Currency: intent.Amount.Currency()},
		ProviderOrderID: intent.ProviderOrderID, ApprovalURL: intent.ApprovalURL,
		LedgerTransactionID: string(intent.LedgerTransactionID), CreatedAt: intent.CreatedAt,
		UpdatedAt: intent.UpdatedAt, Duplicate: intent.Duplicate,
	}
	if intent.NetAmount.Valid() {
		gross := item.Amount
		item.GrossAmount = &gross
		item.Amount = moneyOutput{AmountMinor: intent.NetAmount.AmountMinor(), Currency: intent.NetAmount.Currency()}
	}
	if !intent.CompletedAt.IsZero() {
		item.CompletedAt = &intent.CompletedAt
	}
	return struct {
		FundingIntent fundingAPIResponse `json:"fundingIntent"`
	}{FundingIntent: item}
}

type transferAPIResponse struct {
	ID                  string      `json:"id"`
	RecipientID         string      `json:"recipientId"`
	RecipientEmail      string      `json:"recipientEmail"`
	Amount              moneyOutput `json:"amount"`
	LedgerTransactionID string      `json:"ledgerTransactionId"`
	CreatedAt           time.Time   `json:"createdAt"`
	Duplicate           bool        `json:"duplicate"`
}

func transferResponse(transfer Transfer) struct {
	Transfer transferAPIResponse `json:"transfer"`
} {
	return struct {
		Transfer transferAPIResponse `json:"transfer"`
	}{Transfer: transferAPIResponse{
		ID: string(transfer.ID), RecipientID: string(transfer.RecipientID), RecipientEmail: transfer.RecipientEmail,
		Amount:              moneyOutput{AmountMinor: transfer.Amount.AmountMinor(), Currency: transfer.Amount.Currency()},
		LedgerTransactionID: string(transfer.LedgerTransactionID), CreatedAt: transfer.CreatedAt,
		Duplicate: transfer.Duplicate,
	}}
}

type withdrawalAPIResponse struct {
	ID                       string           `json:"id"`
	Status                   WithdrawalStatus `json:"status"`
	Destination              string           `json:"destination"`
	Amount                   moneyOutput      `json:"amount"`
	ProviderPayoutID         string           `json:"providerPayoutId,omitempty"`
	ReservationTransactionID string           `json:"reservationTransactionId"`
	SettlementTransactionID  string           `json:"settlementTransactionId,omitempty"`
	CreatedAt                time.Time        `json:"createdAt"`
	UpdatedAt                time.Time        `json:"updatedAt"`
	CompletedAt              *time.Time       `json:"completedAt,omitempty"`
	Duplicate                bool             `json:"duplicate"`
}

func withdrawalResponse(withdrawal Withdrawal) struct {
	Withdrawal withdrawalAPIResponse `json:"withdrawal"`
} {
	item := withdrawalAPIResponse{
		ID: string(withdrawal.ID), Status: withdrawal.Status, Destination: withdrawal.Destination,
		Amount:                   moneyOutput{AmountMinor: withdrawal.Amount.AmountMinor(), Currency: withdrawal.Amount.Currency()},
		ProviderPayoutID:         withdrawal.ProviderPayoutID,
		ReservationTransactionID: string(withdrawal.ReservationTransactionID),
		SettlementTransactionID:  string(withdrawal.SettlementTransactionID),
		CreatedAt:                withdrawal.CreatedAt, UpdatedAt: withdrawal.UpdatedAt, Duplicate: withdrawal.Duplicate,
	}
	if !withdrawal.CompletedAt.IsZero() {
		item.CompletedAt = &withdrawal.CompletedAt
	}
	return struct {
		Withdrawal withdrawalAPIResponse `json:"withdrawal"`
	}{Withdrawal: item}
}

func writeAuthenticationRequired(response http.ResponseWriter) {
	httpapi.WriteError(response, http.StatusUnauthorized, "authentication_required", "A valid Controlled session is required.")
}

func writeOperationError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidIdempotencyKey):
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_idempotency_key", "A non-empty Idempotency-Key header is required.")
	case errors.Is(err, ErrInvalidOperation), errors.Is(err, ErrCurrencyMismatch), errors.Is(err, ErrSelfTransfer):
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "invalid_wallet_operation", "The wallet operation details are invalid.")
	case errors.Is(err, ErrRecipientNotFound), errors.Is(err, ErrOperationNotFound):
		httpapi.WriteError(response, http.StatusNotFound, "wallet_operation_not_found", "The recipient or wallet operation was not found.")
	case errors.Is(err, ErrIdempotencyConflict):
		httpapi.WriteError(response, http.StatusConflict, "idempotency_conflict", "This Idempotency-Key was already used with different input.")
	case errors.Is(err, ledger.ErrInsufficientBalance):
		httpapi.WriteError(response, http.StatusConflict, "insufficient_balance", "The wallet does not have enough available balance.")
	case errors.Is(err, ErrPolicyDenied):
		httpapi.WriteError(response, http.StatusConflict, "policy_denied", "An active Controlled policy blocked this operation.")
	case errors.Is(err, ErrInvalidState):
		httpapi.WriteError(response, http.StatusConflict, "invalid_operation_state", "The wallet operation cannot transition from its current state.")
	case errors.Is(err, custody.ErrUnavailable):
		httpapi.WriteError(response, http.StatusBadGateway, "custody_unavailable", "The custody provider is unavailable.")
	default:
		httpapi.WriteError(response, http.StatusInternalServerError, "internal_error", "Controlled could not complete the wallet operation.")
	}
}
