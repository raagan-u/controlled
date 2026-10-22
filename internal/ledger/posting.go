package ledger

import "math"

type accountState struct {
	account Account
	balance int64
}

func evaluatePosting(journal Journal, states map[AccountID]accountState) (map[AccountID]int64, error) {
	deltas := make(map[AccountID]int64, len(journal.entries))
	for _, entry := range journal.entries {
		state, exists := states[entry.accountID]
		if !exists {
			return nil, ErrAccountNotFound
		}
		if state.account.currency != entry.amount.Currency() {
			return nil, ErrAccountCurrencyMismatch
		}

		delta := entry.amount.AmountMinor()
		if entry.side != state.account.normalSide {
			delta = -delta
		}
		updated, ok := safeSignedAdd(deltas[entry.accountID], delta)
		if !ok {
			return nil, ErrAmountOverflow
		}
		deltas[entry.accountID] = updated
	}

	nextBalances := make(map[AccountID]int64, len(deltas))
	for accountID, delta := range deltas {
		next, ok := safeSignedAdd(states[accountID].balance, delta)
		if !ok {
			return nil, ErrAmountOverflow
		}
		if next < 0 {
			return nil, ErrInsufficientBalance
		}
		nextBalances[accountID] = next
	}
	return nextBalances, nil
}

func safeSignedAdd(left, right int64) (int64, bool) {
	if right > 0 && left > math.MaxInt64-right {
		return 0, false
	}
	if right < 0 && left < math.MinInt64-right {
		return 0, false
	}
	return left + right, true
}
