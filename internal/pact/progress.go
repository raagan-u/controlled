package pact

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// UsageItem is one wallet debit that contributes to a pact's spending total.
type UsageItem struct {
	OperationType       string
	OperationID         string
	AmountMinor         int64
	Currency            string
	Status              string
	CountedAt           time.Time
	ReversedAt          time.Time
	CounterpartyName    string
	CounterpartyCtrldID string
	Destination         string
}

// Progress is the authoritative spending read model for one pact.
type Progress struct {
	UsedMinor      int64
	RemainingMinor int64
	OverageMinor   int64
	Usage          []UsageItem
}

type ProgressRepository interface {
	Progress(context.Context, UserID, PactID, bool) (Progress, error)
	ProgressForOwner(context.Context, UserID) (map[PactID]Progress, error)
}

func (s *Service) PactProgress(ctx context.Context, owner UserID, id PactID) (Progress, error) {
	repository, ok := s.repository.(ProgressRepository)
	if !ok {
		return Progress{}, ErrUnavailable
	}
	return repository.Progress(ctx, owner, id, true)
}

func (s *Service) PactsProgress(ctx context.Context, owner UserID) (map[PactID]Progress, error) {
	repository, ok := s.repository.(ProgressRepository)
	if !ok {
		return nil, ErrUnavailable
	}
	return repository.ProgressForOwner(ctx, owner)
}

func (r *PostgresRepository) Progress(ctx context.Context, owner UserID, id PactID, includeUsage bool) (Progress, error) {
	if r == nil || r.pool == nil {
		return Progress{}, ErrUnavailable
	}
	var limit, used int64
	err := r.pool.QueryRow(ctx, `
		SELECT pact.spending_limit_minor,
		       COALESCE(sum(usage.amount_minor) FILTER (WHERE usage.status = 'COUNTED'), 0)::bigint
		  FROM pacts AS pact
		  LEFT JOIN pact_transaction_usage AS usage ON usage.pact_id = pact.id
		 WHERE pact.id = $1::text::uuid AND pact.owner_id = $2::text::uuid
		 GROUP BY pact.id
	`, string(id), string(owner)).Scan(&limit, &used)
	if err != nil {
		return Progress{}, fmt.Errorf("read pact progress: %w", err)
	}
	progress := progressAmounts(limit, used)
	if !includeUsage {
		return progress, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT usage.operation_type, usage.operation_id::text, usage.amount_minor, usage.currency,
		       usage.status, usage.counted_at, usage.reversed_at,
		       COALESCE(recipient.display_name, ''), COALESCE(recipient.ctrld_id, ''),
		       COALESCE(withdrawal.destination, '')
		  FROM pact_transaction_usage AS usage
		  LEFT JOIN internal_transfers AS transfer
		    ON usage.operation_type = 'INTERNAL_TRANSFER' AND transfer.id = usage.operation_id
		  LEFT JOIN wallets AS recipient_wallet ON recipient_wallet.id = transfer.recipient_wallet_id
		  LEFT JOIN users AS recipient ON recipient.id = recipient_wallet.owner_id
		  LEFT JOIN withdrawals AS withdrawal
		    ON usage.operation_type = 'WITHDRAWAL' AND withdrawal.id = usage.operation_id
		 WHERE usage.pact_id = $1::text::uuid AND usage.owner_id = $2::text::uuid
		 ORDER BY usage.counted_at DESC, usage.operation_id
	`, string(id), string(owner))
	if err != nil {
		return Progress{}, fmt.Errorf("read pact usage: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item UsageItem
		var reversedAt pgtype.Timestamptz
		if err := rows.Scan(
			&item.OperationType, &item.OperationID, &item.AmountMinor, &item.Currency,
			&item.Status, &item.CountedAt, &reversedAt,
			&item.CounterpartyName, &item.CounterpartyCtrldID, &item.Destination,
		); err != nil {
			return Progress{}, fmt.Errorf("scan pact usage: %w", err)
		}
		if reversedAt.Valid {
			item.ReversedAt = reversedAt.Time.UTC()
		}
		progress.Usage = append(progress.Usage, item)
	}
	return progress, rows.Err()
}

func (r *PostgresRepository) ProgressForOwner(ctx context.Context, owner UserID) (map[PactID]Progress, error) {
	if r == nil || r.pool == nil {
		return nil, ErrUnavailable
	}
	rows, err := r.pool.Query(ctx, `
		SELECT pact.id::text, pact.spending_limit_minor,
		       COALESCE(sum(usage.amount_minor) FILTER (WHERE usage.status = 'COUNTED'), 0)::bigint
		  FROM pacts AS pact
		  LEFT JOIN pact_transaction_usage AS usage ON usage.pact_id = pact.id
		 WHERE pact.owner_id = $1::text::uuid
		 GROUP BY pact.id
	`, string(owner))
	if err != nil {
		return nil, fmt.Errorf("list pact progress: %w", err)
	}
	defer rows.Close()
	result := make(map[PactID]Progress)
	for rows.Next() {
		var id PactID
		var limit, used int64
		if err := rows.Scan(&id, &limit, &used); err != nil {
			return nil, fmt.Errorf("scan pact progress: %w", err)
		}
		result[id] = progressAmounts(limit, used)
	}
	return result, rows.Err()
}

func progressAmounts(limit, used int64) Progress {
	progress := Progress{UsedMinor: used, RemainingMinor: limit - used}
	if progress.RemainingMinor < 0 {
		progress.OverageMinor = -progress.RemainingMinor
		progress.RemainingMinor = 0
	}
	return progress
}
