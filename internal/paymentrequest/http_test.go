package paymentrequest

import (
	"testing"

	"github.com/raagan-u/controlled/internal/identity"
)

func TestResponseHidesPayerApprovalFromRequester(t *testing.T) {
	item := Request{
		RequesterID:             identity.UserID("requester"),
		RecipientID:             identity.UserID("payer"),
		AccountabilityRequestID: "private-approval",
	}

	requesterView := newResponse(item, item.RequesterID)
	if requesterView.AccountabilityRequestID != "" {
		t.Fatalf("requester received payer approval id %q", requesterView.AccountabilityRequestID)
	}

	payerView := newResponse(item, item.RecipientID)
	if payerView.AccountabilityRequestID != "private-approval" {
		t.Fatalf("payer approval id = %q, want private-approval", payerView.AccountabilityRequestID)
	}
}
