package ledger

import "github.com/raagan-u/controlled/internal/money"

type AccountID string
type OwnerID string

type AccountKind string

const (
	AccountKindCustodyAsset      AccountKind = "CUSTODY_ASSET"
	AccountKindAvailable         AccountKind = "AVAILABLE"
	AccountKindPending           AccountKind = "PENDING"
	AccountKindCommitmentReserve AccountKind = "COMMITMENT_RESERVE"
	AccountKindStakeVault        AccountKind = "STAKE_VAULT"
)

type Side string

const (
	SideDebit  Side = "DEBIT"
	SideCredit Side = "CREDIT"
)

type Account struct {
	id         AccountID
	ownerID    OwnerID
	kind       AccountKind
	currency   string
	normalSide Side
}

func NewAccount(id AccountID, ownerID OwnerID, kind AccountKind, currency string) (Account, error) {
	normalSide, valid := normalSideFor(kind)
	if id == "" || !valid || !money.ValidCurrency(currency) {
		return Account{}, ErrInvalidAccount
	}
	if ownerRequired(kind) && ownerID == "" {
		return Account{}, ErrInvalidAccount
	}
	if !ownerRequired(kind) && ownerID != "" {
		return Account{}, ErrInvalidAccount
	}

	return Account{
		id:         id,
		ownerID:    ownerID,
		kind:       kind,
		currency:   currency,
		normalSide: normalSide,
	}, nil
}

func (a Account) ID() AccountID     { return a.id }
func (a Account) OwnerID() OwnerID  { return a.ownerID }
func (a Account) Kind() AccountKind { return a.kind }
func (a Account) Currency() string  { return a.currency }
func (a Account) NormalSide() Side  { return a.normalSide }
func (a Account) Valid() bool {
	_, err := NewAccount(a.id, a.ownerID, a.kind, a.currency)
	return err == nil
}

func ownerRequired(kind AccountKind) bool {
	return kind == AccountKindAvailable ||
		kind == AccountKindPending ||
		kind == AccountKindCommitmentReserve
}

func normalSideFor(kind AccountKind) (Side, bool) {
	switch kind {
	case AccountKindCustodyAsset:
		return SideDebit, true
	case AccountKindAvailable, AccountKindPending, AccountKindCommitmentReserve, AccountKindStakeVault:
		return SideCredit, true
	default:
		return "", false
	}
}
