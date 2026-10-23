package wallet

import (
	"context"

	"github.com/raagan-u/pay-pact/internal/identity"
	"github.com/raagan-u/pay-pact/internal/money"
)

type BalanceSet struct {
	Available         money.Money
	Pending           money.Money
	CommitmentReserve money.Money
}

type Snapshot struct {
	wallet   Wallet
	balances BalanceSet
}

func NewSnapshot(wallet Wallet, balances BalanceSet) (Snapshot, error) {
	if wallet.ID() == "" ||
		balances.Available.Currency() != wallet.Currency() ||
		balances.Pending.Currency() != wallet.Currency() ||
		balances.CommitmentReserve.Currency() != wallet.Currency() {
		return Snapshot{}, ErrInvalidWallet
	}
	return Snapshot{wallet: wallet, balances: balances}, nil
}

func (s Snapshot) Wallet() Wallet       { return s.wallet }
func (s Snapshot) Balances() BalanceSet { return s.balances }

type Reader interface {
	WalletByUser(context.Context, identity.UserID) (Snapshot, error)
}
