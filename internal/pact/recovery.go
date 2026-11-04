package pact

import (
	"context"
	"fmt"
	"time"
)

type RecoveryPact struct {
	ID                     PactID
	Goal                   string
	Status                 Status
	StakeMinor             int64
	ProjectedRecoveryMinor int64
	StartsAt               time.Time
	EndsAt                 time.Time
	RecoveryEligible       bool
}

type StakeRecovery struct {
	Currency               string
	StakedMinor            int64
	StakedPactCount        int
	TotalReservedMinor     int64
	RecoveredMinor         int64
	CommitmentReserveMinor int64
	ProjectedRecoveryMinor int64
	Pacts                  []RecoveryPact
}

type recoveryReader interface {
	StakeRecovery(context.Context, UserID) (StakeRecovery, error)
}

func (s *Service) StakeRecovery(ctx context.Context, ownerID UserID) (StakeRecovery, error) {
	if err := s.ready(); err != nil {
		return StakeRecovery{}, err
	}
	reader, ok := s.repository.(recoveryReader)
	if !ok {
		return StakeRecovery{}, ErrUnavailable
	}
	return reader.StakeRecovery(ctx, ownerID)
}

type recoveryLot struct {
	remainingMinor int64
	resolvedAt     time.Time
}

func (r *PostgresRepository) StakeRecovery(ctx context.Context, ownerID UserID) (StakeRecovery, error) {
	if r == nil || r.pool == nil {
		return StakeRecovery{}, ErrUnavailable
	}
	owner, err := pactUUID(string(ownerID))
	if err != nil {
		return StakeRecovery{}, err
	}
	summary := StakeRecovery{Currency: "USD", Pacts: []RecoveryPact{}}
	var stakedPactCount int64
	err = r.pool.QueryRow(ctx, `
		SELECT COALESCE(sum(allocation.amount_minor), 0), count(*)
		  FROM stake_allocations AS allocation
		  JOIN pacts AS pact ON pact.id = allocation.pact_id
		 WHERE allocation.owner_id = $1
		   AND allocation.status = 'ACTIVE'
		   AND pact.status IN ('SCHEDULED', 'ACTIVE')
	`, owner).Scan(&summary.StakedMinor, &stakedPactCount)
	if err != nil {
		return StakeRecovery{}, fmt.Errorf("read active stake summary: %w", err)
	}
	summary.StakedPactCount = int(stakedPactCount)
	err = r.pool.QueryRow(ctx, `
		SELECT COALESCE(sum(amount_minor), 0),
		       COALESCE(sum(recovered_minor), 0),
		       COALESCE(sum(amount_minor - recovered_minor), 0)
		  FROM stake_allocations
		 WHERE owner_id = $1 AND status = 'COMMITMENT_RESERVED'
	`, owner).Scan(&summary.TotalReservedMinor, &summary.RecoveredMinor, &summary.CommitmentReserveMinor)
	if err != nil {
		return StakeRecovery{}, fmt.Errorf("read commitment recovery summary: %w", err)
	}
	var lots []recoveryLot
	if summary.CommitmentReserveMinor > 0 {
		lotRows, err := r.pool.Query(ctx, `
			SELECT amount_minor - recovered_minor, resolved_at
			  FROM stake_allocations
			 WHERE owner_id = $1
			   AND status = 'COMMITMENT_RESERVED'
			   AND recovered_minor < amount_minor
			 ORDER BY resolved_at, pact_id
		`, owner)
		if err != nil {
			return StakeRecovery{}, fmt.Errorf("read commitment recovery queue: %w", err)
		}
		for lotRows.Next() {
			var lot recoveryLot
			if err := lotRows.Scan(&lot.remainingMinor, &lot.resolvedAt); err != nil {
				lotRows.Close()
				return StakeRecovery{}, err
			}
			lots = append(lots, lot)
		}
		if err := lotRows.Err(); err != nil {
			lotRows.Close()
			return StakeRecovery{}, err
		}
		lotRows.Close()
	}

	pactRows, err := r.pool.Query(ctx, `
		SELECT pact.id::text, pact.goal, pact.status, allocation.amount_minor,
		       pact.starts_at, pact.ends_at, allocation.created_at
		  FROM stake_allocations AS allocation
		  JOIN pacts AS pact ON pact.id = allocation.pact_id
		 WHERE allocation.owner_id = $1
		   AND allocation.status = 'ACTIVE'
		   AND pact.status IN ('SCHEDULED', 'ACTIVE')
		 ORDER BY pact.ends_at, allocation.created_at, pact.id
	`, owner)
	if err != nil {
		return StakeRecovery{}, fmt.Errorf("read projected recovery pacts: %w", err)
	}
	defer pactRows.Close()
	for pactRows.Next() {
		var item RecoveryPact
		var stakeCreatedAt time.Time
		if err := pactRows.Scan(&item.ID, &item.Goal, &item.Status, &item.StakeMinor, &item.StartsAt, &item.EndsAt, &stakeCreatedAt); err != nil {
			return StakeRecovery{}, err
		}
		item.RecoveryEligible = item.EndsAt.Sub(item.StartsAt) >= MinimumRecoveryDuration
		if !item.RecoveryEligible {
			summary.Pacts = append(summary.Pacts, item)
			continue
		}
		remainingStake := item.StakeMinor
		for index := range lots {
			if remainingStake == 0 {
				break
			}
			if lots[index].remainingMinor == 0 || !lots[index].resolvedAt.Before(stakeCreatedAt) {
				continue
			}
			amount := min(remainingStake, lots[index].remainingMinor)
			item.ProjectedRecoveryMinor += amount
			remainingStake -= amount
			lots[index].remainingMinor -= amount
		}
		summary.ProjectedRecoveryMinor += item.ProjectedRecoveryMinor
		summary.Pacts = append(summary.Pacts, item)
	}
	if err := pactRows.Err(); err != nil {
		return StakeRecovery{}, err
	}
	return summary, nil
}

var _ recoveryReader = (*PostgresRepository)(nil)
