package wallet

import (
	"context"
	"strings"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/ledger"
)

type IDGenerator interface {
	NewID() (string, error)
}

type Clock interface {
	Now() time.Time
}

type OnboardingRequest struct {
	Email       string
	DisplayName string
	// NamePending marks DisplayName as a placeholder the user will replace after sign-up.
	NamePending bool
	Currency    string
}

type Provisioning struct {
	user     identity.User
	wallet   Wallet
	accounts []ledger.Account
}

func newProvisioning(user identity.User, wallet Wallet, accounts []ledger.Account) (Provisioning, error) {
	if user.ID() == "" || wallet.OwnerID() != user.ID() || len(accounts) != 3 {
		return Provisioning{}, ErrInvalidWallet
	}

	expected := map[ledger.AccountKind]ledger.AccountID{
		ledger.AccountKindAvailable:         wallet.Accounts().Available,
		ledger.AccountKindPending:           wallet.Accounts().Pending,
		ledger.AccountKindCommitmentReserve: wallet.Accounts().CommitmentReserve,
	}
	seen := make(map[ledger.AccountKind]struct{}, len(accounts))
	for _, account := range accounts {
		accountID, wanted := expected[account.Kind()]
		if !wanted || account.ID() != accountID ||
			account.OwnerID() != ledger.OwnerID(user.ID()) ||
			account.Currency() != wallet.Currency() {
			return Provisioning{}, ErrInvalidAccountSet
		}
		if _, duplicate := seen[account.Kind()]; duplicate {
			return Provisioning{}, ErrInvalidAccountSet
		}
		seen[account.Kind()] = struct{}{}
	}

	return Provisioning{user: user, wallet: wallet, accounts: append([]ledger.Account(nil), accounts...)}, nil
}

func (p Provisioning) User() identity.User { return p.user }
func (p Provisioning) Wallet() Wallet      { return p.wallet }
func (p Provisioning) Accounts() []ledger.Account {
	return append([]ledger.Account(nil), p.accounts...)
}

type ProvisioningStore interface {
	Create(context.Context, string, Provisioning) (Provisioning, bool, error)
}

type OnboardingService struct {
	ids   IDGenerator
	clock Clock
	store ProvisioningStore
}

func NewOnboardingService(ids IDGenerator, clock Clock, store ProvisioningStore) *OnboardingService {
	return &OnboardingService{ids: ids, clock: clock, store: store}
}

type OnboardingResult struct {
	Provisioning Provisioning
	Duplicate    bool
}

func (s *OnboardingService) Onboard(
	ctx context.Context,
	idempotencyKey string,
	request OnboardingRequest,
) (OnboardingResult, error) {
	if s == nil || s.ids == nil || s.clock == nil || s.store == nil {
		return OnboardingResult{}, ErrOnboardingUnavailable
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		return OnboardingResult{}, ErrInvalidIdempotencyKey
	}

	now := s.clock.Now()
	userID, err := s.ids.NewID()
	if err != nil {
		return OnboardingResult{}, err
	}
	user, err := identity.NewUser(
		identity.UserID(userID),
		request.Email,
		request.DisplayName,
		now,
	)
	if err != nil {
		return OnboardingResult{}, err
	}
	user = user.WithNamePending(request.NamePending)

	accountIDs, err := generateIDs(s.ids, 3)
	if err != nil {
		return OnboardingResult{}, err
	}
	accountSet := AccountSet{
		Available:         ledger.AccountID(accountIDs[0]),
		Pending:           ledger.AccountID(accountIDs[1]),
		CommitmentReserve: ledger.AccountID(accountIDs[2]),
	}
	walletID, err := s.ids.NewID()
	if err != nil {
		return OnboardingResult{}, err
	}
	wallet, err := NewWallet(
		WalletID(walletID),
		user.ID(),
		strings.ToUpper(strings.TrimSpace(request.Currency)),
		accountSet,
		now,
	)
	if err != nil {
		return OnboardingResult{}, err
	}

	accounts, err := walletAccounts(wallet)
	if err != nil {
		return OnboardingResult{}, err
	}
	provisioning, err := newProvisioning(user, wallet, accounts)
	if err != nil {
		return OnboardingResult{}, err
	}

	stored, duplicate, err := s.store.Create(ctx, strings.TrimSpace(idempotencyKey), provisioning)
	if err != nil {
		return OnboardingResult{}, err
	}
	return OnboardingResult{Provisioning: stored, Duplicate: duplicate}, nil
}

func generateIDs(generator IDGenerator, count int) ([]string, error) {
	ids := make([]string, 0, count)
	for range count {
		id, err := generator.NewID()
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func walletAccounts(wallet Wallet) ([]ledger.Account, error) {
	definitions := []struct {
		id   ledger.AccountID
		kind ledger.AccountKind
	}{
		{id: wallet.accounts.Available, kind: ledger.AccountKindAvailable},
		{id: wallet.accounts.Pending, kind: ledger.AccountKindPending},
		{id: wallet.accounts.CommitmentReserve, kind: ledger.AccountKindCommitmentReserve},
	}

	accounts := make([]ledger.Account, 0, len(definitions))
	for _, definition := range definitions {
		account, err := ledger.NewAccount(
			definition.id,
			ledger.OwnerID(wallet.ownerID),
			definition.kind,
			wallet.currency,
		)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}
