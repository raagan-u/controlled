package notification

import "github.com/raagan-u/controlled/internal/identity"

const (
	KindPaymentRequest        = "PAYMENT_REQUEST"
	KindAccountabilityRequest = "ACCOUNTABILITY_REQUEST"
	KindInvitation            = "ACCOUNTABILITY_INVITATION"
	KindMoneyReceived         = "MONEY_RECEIVED"
)

type Message struct {
	Title    string
	Body     string
	Kind     string
	EntityID string
}

type Notifier interface {
	Notify(identity.UserID, Message)
}
