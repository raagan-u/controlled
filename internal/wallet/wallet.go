// Package wallet owns Controlled wallet identity and onboarding.
package wallet

import (
	"time"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/ledger"
	"github.com/raagan-u/controlled/internal/money"
)

type WalletID string

type AccountSet struct {
	Available         ledger.AccountID
	Pending           ledger.AccountID
	CommitmentReserve ledger.AccountID
}

func (a AccountSet) valid() bool {
	if a.Available == "" || a.Pending == "" || a.CommitmentReserve == "" {
		return false
	}
	return a.Available != a.Pending &&
		a.Available != a.CommitmentReserve &&
		a.Pending != a.CommitmentReserve
}

type Wallet struct {
	id        WalletID
	ownerID   identity.UserID
	currency  string
	accounts  AccountSet
	createdAt time.Time
}

func NewWallet(
	id WalletID,
	ownerID identity.UserID,
	currency string,
	accounts AccountSet,
	createdAt time.Time,
) (Wallet, error) {
	if id == "" || ownerID == "" || !money.ValidCurrency(currency) || createdAt.IsZero() {
		return Wallet{}, ErrInvalidWallet
	}
	if !accounts.valid() {
		return Wallet{}, ErrInvalidAccountSet
	}

	return Wallet{
		id:        id,
		ownerID:   ownerID,
		currency:  currency,
		accounts:  accounts,
		createdAt: createdAt.UTC(),
	}, nil
}

func (w Wallet) ID() WalletID             { return w.id }
func (w Wallet) OwnerID() identity.UserID { return w.ownerID }
func (w Wallet) Currency() string         { return w.currency }
func (w Wallet) Accounts() AccountSet     { return w.accounts }
func (w Wallet) CreatedAt() time.Time     { return w.createdAt }
