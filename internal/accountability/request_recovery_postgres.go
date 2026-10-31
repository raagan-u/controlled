package accountability

import (
	"context"
	"fmt"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/money"
	"github.com/raagan-u/controlled/internal/walletops"
)

func (r *PostgresRepository) ApprovedDebits(ctx context.Context) ([]walletops.ApprovedDebit, error) {
	if r == nil || r.pool == nil {
		return nil, ErrUnavailable
	}
	rows, err := r.pool.Query(ctx, `
		SELECT request.id::text, intent.id::text, intent.idempotency_key,
		       intent.owner_id::text, intent.operation_type, intent.destination,
		       intent.amount_minor, intent.currency, request.classification
		  FROM accountability_requests AS request
		  JOIN transaction_intents AS intent ON intent.id = request.intent_id
		 WHERE request.status = 'APPROVED' AND intent.status = 'APPROVED'
		 ORDER BY request.resolved_at, request.id
	`)
	if err != nil {
		return nil, fmt.Errorf("list approved transaction intents: %w", err)
	}
	var debits []walletops.ApprovedDebit
	for rows.Next() {
		var debit walletops.ApprovedDebit
		var ownerID, kind, currency string
		var amountMinor int64
		if err := rows.Scan(
			&debit.RequestID, &debit.IntentID, &debit.IdempotencyKey,
			&ownerID, &kind, &debit.Destination, &amountMinor, &currency, &debit.Classification,
		); err != nil {
			rows.Close()
			return nil, err
		}
		debit.OwnerID = identity.UserID(ownerID)
		debit.Kind = walletops.DebitKind(kind)
		debit.Amount, err = money.New(amountMinor, currency)
		if err != nil {
			rows.Close()
			return nil, err
		}
		debits = append(debits, debit)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for index := range debits {
		pactRows, err := r.pool.Query(ctx, `
			SELECT pact_id::text FROM accountability_request_pacts
			 WHERE request_id = $1::text::uuid ORDER BY pact_id
		`, debits[index].RequestID)
		if err != nil {
			return nil, err
		}
		for pactRows.Next() {
			var pactID string
			if err := pactRows.Scan(&pactID); err != nil {
				pactRows.Close()
				return nil, err
			}
			debits[index].PactIDs = append(debits[index].PactIDs, pactID)
		}
		if err := pactRows.Err(); err != nil {
			pactRows.Close()
			return nil, err
		}
		pactRows.Close()
	}
	return debits, nil
}
