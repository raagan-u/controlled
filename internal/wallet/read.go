package wallet

import (
	"context"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/ledger"
	"github.com/raagan-u/controlled/internal/money"
)

type BalanceSet struct {
	Available         money.Money
	Pending           money.Money
	Staked            money.Money
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
		balances.Staked.Currency() != wallet.Currency() ||
		balances.CommitmentReserve.Currency() != wallet.Currency() {
		return Snapshot{}, ErrInvalidWallet
	}
	return Snapshot{wallet: wallet, balances: balances}, nil
}

func (s Snapshot) Wallet() Wallet       { return s.wallet }
func (s Snapshot) Balances() BalanceSet { return s.balances }

type Reader interface {
	WalletByUser(context.Context, identity.UserID) (Snapshot, error)
	TransactionsByUser(context.Context, identity.UserID, int) ([]Transaction, error)
	TransactionByUser(context.Context, identity.UserID, ledger.JournalID) (Transaction, error)
}

type TransactionEntry struct {
	AccountID   ledger.AccountID
	AccountKind ledger.AccountKind
	Side        ledger.Side
	Amount      money.Money
}

type Transaction struct {
	ID          ledger.JournalID
	Reference   ledger.Reference
	Description string
	OccurredAt  time.Time
	PostedAt    time.Time
	Reversed    bool
	Entries     []TransactionEntry
}
