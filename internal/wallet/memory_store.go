package wallet

import (
	"context"
	"sync"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/ledger"
)

type onboardingRecord struct {
	provisioning Provisioning
}

// MemoryProvisioningStore is a deterministic adapter for the demo and tests.
// A PostgreSQL implementation will replace it without changing onboarding rules.
type MemoryProvisioningStore struct {
	mu           sync.RWMutex
	byKey        map[string]onboardingRecord
	usersByID    map[identity.UserID]identity.User
	usersByEmail map[string]identity.UserID
	walletsByID  map[WalletID]Wallet
	walletByUser map[identity.UserID]WalletID
	accounts     map[ledger.AccountID]ledger.Account
}

func NewMemoryProvisioningStore() *MemoryProvisioningStore {
	return &MemoryProvisioningStore{
		byKey:        make(map[string]onboardingRecord),
		usersByID:    make(map[identity.UserID]identity.User),
		usersByEmail: make(map[string]identity.UserID),
		walletsByID:  make(map[WalletID]Wallet),
		walletByUser: make(map[identity.UserID]WalletID),
		accounts:     make(map[ledger.AccountID]ledger.Account),
	}
}

func (s *MemoryProvisioningStore) Create(
	ctx context.Context,
	idempotencyKey string,
	provisioning Provisioning,
) (Provisioning, bool, error) {
	if err := ctx.Err(); err != nil {
		return Provisioning{}, false, err
	}
	if idempotencyKey == "" {
		return Provisioning{}, false, ErrInvalidIdempotencyKey
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if record, exists := s.byKey[idempotencyKey]; exists {
		if !sameOnboarding(record.provisioning, provisioning) {
			return Provisioning{}, false, ErrIdempotencyConflict
		}
		return record.provisioning, true, nil
	}
	if _, exists := s.usersByEmail[provisioning.user.Email()]; exists {
		return Provisioning{}, false, ErrUserExists
	}
	if _, exists := s.usersByID[provisioning.user.ID()]; exists {
		return Provisioning{}, false, ErrProvisioningExists
	}
	if _, exists := s.walletByUser[provisioning.user.ID()]; exists {
		return Provisioning{}, false, ErrWalletExists
	}
	if _, exists := s.walletsByID[provisioning.wallet.ID()]; exists {
		return Provisioning{}, false, ErrProvisioningExists
	}
	for _, account := range provisioning.accounts {
		if _, exists := s.accounts[account.ID()]; exists {
			return Provisioning{}, false, ErrProvisioningExists
		}
	}

	s.byKey[idempotencyKey] = onboardingRecord{provisioning: provisioning}
	s.usersByID[provisioning.user.ID()] = provisioning.user
	s.usersByEmail[provisioning.user.Email()] = provisioning.user.ID()
	s.walletsByID[provisioning.wallet.ID()] = provisioning.wallet
	s.walletByUser[provisioning.user.ID()] = provisioning.wallet.ID()
	for _, account := range provisioning.accounts {
		s.accounts[account.ID()] = account
	}
	return provisioning, false, nil
}

func (s *MemoryProvisioningStore) ByUser(ctx context.Context, userID identity.UserID) (Provisioning, error) {
	if err := ctx.Err(); err != nil {
		return Provisioning{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	walletID, exists := s.walletByUser[userID]
	if !exists {
		return Provisioning{}, ErrProvisioningNotFound
	}
	user := s.usersByID[userID]
	wallet := s.walletsByID[walletID]
	accountSet := wallet.Accounts()
	accounts := []ledger.Account{
		s.accounts[accountSet.Available],
		s.accounts[accountSet.Pending],
		s.accounts[accountSet.CommitmentReserve],
	}
	return newProvisioning(user, wallet, accounts)
}

func sameOnboarding(left, right Provisioning) bool {
	return left.user.Email() == right.user.Email() &&
		left.user.DisplayName() == right.user.DisplayName() &&
		left.wallet.Currency() == right.wallet.Currency()
}
