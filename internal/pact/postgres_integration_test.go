package pact_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/raagan-u/controlled/internal/accountability"
	"github.com/raagan-u/controlled/internal/custody"
	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/ledger"
	"github.com/raagan-u/controlled/internal/pact"
	"github.com/raagan-u/controlled/internal/platform"
	platformpostgres "github.com/raagan-u/controlled/internal/platform/postgres"
	"github.com/raagan-u/controlled/internal/wallet"
	"github.com/raagan-u/controlled/internal/walletops"
)

func TestPostgresPactStakeLifecycle(t *testing.T) {
	databaseURL := os.Getenv("CONTROLLED_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("CONTROLLED_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := platformpostgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ids := platform.NewUUIDGenerator()
	clock := platform.SystemClock{}
	walletStore := wallet.NewPostgresProvisioningStore(pool)
	onboarding := wallet.NewOnboardingService(ids, clock, walletStore)
	runID, _ := ids.NewID()
	owner, err := onboarding.Onboard(ctx, "pact-owner:"+runID, wallet.OnboardingRequest{
		Email: fmt.Sprintf("pact-owner-%s@example.test", runID), DisplayName: "Pact Owner", Currency: "USD",
	})
	if err != nil {
		t.Fatalf("onboard owner: %v", err)
	}
	partner, err := onboarding.Onboard(ctx, "pact-partner:"+runID, wallet.OnboardingRequest{
		Email: fmt.Sprintf("pact-partner-%s@example.test", runID), DisplayName: "Pact Partner", Currency: "USD",
	})
	if err != nil {
		t.Fatalf("onboard partner: %v", err)
	}

	accountabilityService := accountability.NewService(ids, clock, accountability.NewPostgresRepository(pool))
	invitation, _, err := accountabilityService.Invite(
		ctx, "pact-invite:"+runID, owner.Provisioning.User().ID(), partner.Provisioning.User().Email(),
	)
	if err != nil {
		t.Fatalf("invite partner: %v", err)
	}
	if _, err := accountabilityService.Respond(ctx, partner.Provisioning.User().ID(), invitation.ID(), true); err != nil {
		t.Fatalf("accept invitation: %v", err)
	}
	circle, _, err := accountabilityService.CreateCircle(
		ctx, "pact-circle:"+runID, owner.Provisioning.User().ID(), "Pact test circle",
		[]identity.UserID{partner.Provisioning.User().ID()},
	)
	if err != nil {
		t.Fatalf("create circle: %v", err)
	}

	operations := walletops.NewService(
		ids, clock, walletops.NewPostgresRepository(pool, ledger.NewPostgresBook(pool)),
		custody.NewFake(ids), walletops.AllowAllPolicy{},
	)
	funding, err := operations.CreateFunding(ctx, "pact-fund:"+runID, owner.Provisioning.User().ID(), 10_000, "USD")
	if err != nil {
		t.Fatalf("create funding: %v", err)
	}
	if _, err := operations.ConfirmFunding(ctx, owner.Provisioning.User().ID(), funding.ID); err != nil {
		t.Fatalf("confirm funding: %v", err)
	}

	pactService := pact.NewService(ids, clock, pact.NewPostgresRepository(pool))
	now := clock.Now()
	limit, _ := pact.NewMoney(5_000, "USD")
	stake, _ := pact.NewMoney(2_500, "USD")
	terms, err := pact.NewTerms(
		"Integration pact", "Verify stake lifecycle", limit, stake,
		now.Add(time.Hour), now.Add(24*time.Hour), pact.CircleID(circle.ID()),
		pact.BreakConsequenceCommitmentReserve,
	)
	if err != nil {
		t.Fatalf("create terms: %v", err)
	}
	draft, _, err := pactService.CreateDraft(ctx, "pact-create:"+runID, pact.UserID(owner.Provisioning.User().ID()), terms)
	if err != nil {
		t.Fatalf("create pact: %v", err)
	}
	if _, err := pactService.Confirm(ctx, draft.OwnerID(), draft.ID()); err != nil {
		t.Fatalf("confirm pact: %v", err)
	}
	reserved, duplicate, err := pactService.ReserveStake(ctx, "pact-stake:"+runID, draft.OwnerID(), draft.ID())
	if err != nil {
		t.Fatalf("reserve stake: %v", err)
	}
	if duplicate || reserved.Status() != pact.StatusScheduled {
		t.Fatalf("reserved status = %s, duplicate = %v", reserved.Status(), duplicate)
	}
	assertPactWalletBalances(t, ctx, walletStore, owner.Provisioning.User().ID(), 7_500, 2_500)

	retry, duplicate, err := pactService.ReserveStake(ctx, "pact-stake:"+runID, draft.OwnerID(), draft.ID())
	if err != nil || !duplicate || retry.StakeReservationID() != reserved.StakeReservationID() {
		t.Fatalf("unexpected stake retry: pact=%+v duplicate=%v err=%v", retry, duplicate, err)
	}
	cancelled, duplicate, err := pactService.Cancel(ctx, draft.OwnerID(), draft.ID())
	if err != nil {
		t.Fatalf("cancel scheduled pact: %v", err)
	}
	if duplicate || cancelled.Status() != pact.StatusCancelled {
		t.Fatalf("cancelled status = %s, duplicate = %v", cancelled.Status(), duplicate)
	}
	assertPactWalletBalances(t, ctx, walletStore, owner.Provisioning.User().ID(), 10_000, 0)

	activeTerms, _ := pact.NewTerms(
		"Active spending cap", "Verify atomic enforcement", limit, stake,
		now.Add(-time.Hour), now.Add(24*time.Hour), pact.CircleID(circle.ID()),
		pact.BreakConsequenceCommitmentReserve,
	)
	active, _, err := pactService.CreateDraft(ctx, "pact-active:"+runID, draft.OwnerID(), activeTerms)
	if err != nil {
		t.Fatalf("create active pact: %v", err)
	}
	if _, err := pactService.Confirm(ctx, active.OwnerID(), active.ID()); err != nil {
		t.Fatalf("confirm active pact: %v", err)
	}
	active, _, err = pactService.ReserveStake(ctx, "pact-active-stake:"+runID, active.OwnerID(), active.ID())
	if err != nil || active.Status() != pact.StatusActive {
		t.Fatalf("activate pact: status=%v err=%v", active.Status(), err)
	}
	if _, err := operations.Transfer(
		ctx, "pact-counted-transfer:"+runID, owner.Provisioning.User().ID(),
		partner.Provisioning.User().Email(), 3_000, "USD",
	); err != nil {
		t.Fatalf("transfer within pact: %v", err)
	}
	if _, err := operations.Transfer(
		ctx, "pact-blocked-transfer:"+runID, owner.Provisioning.User().ID(),
		partner.Provisioning.User().Email(), 2_001, "USD",
	); !errors.Is(err, walletops.ErrPolicyDenied) {
		t.Fatalf("over-limit transfer error = %v", err)
	} else {
		var blocked *walletops.PolicyBlockedError
		if !errors.As(err, &blocked) || len(blocked.Blocks) != 1 || blocked.Blocks[0].OverageMinor != 1 {
			t.Fatalf("policy block facts = %#v", blocked)
		}
	}
	failedUsage, err := operations.CreateWithdrawal(
		ctx, "pact-failed-withdrawal:"+runID, owner.Provisioning.User().ID(),
		"paypal@example.test", 2_000, "USD",
	)
	if err != nil {
		t.Fatalf("reserve withdrawal within pact: %v", err)
	}
	if _, err := operations.SimulateWithdrawalSettlement(
		ctx, owner.Provisioning.User().ID(), failedUsage.ID, false,
	); err != nil {
		t.Fatalf("fail withdrawal within pact: %v", err)
	}
	if _, err := operations.Transfer(
		ctx, "pact-after-reversal:"+runID, owner.Provisioning.User().ID(),
		partner.Provisioning.User().Email(), 2_000, "USD",
	); err != nil {
		t.Fatalf("transfer after usage reversal: %v", err)
	}

	largeStake, _ := pact.NewMoney(15_000, "USD")
	insufficientTerms, _ := pact.NewTerms(
		"Too large", "Verify atomic failure", limit, largeStake,
		now.Add(time.Hour), now.Add(24*time.Hour), pact.CircleID(circle.ID()),
		pact.BreakConsequenceCommitmentReserve,
	)
	insufficient, _, err := pactService.CreateDraft(ctx, "pact-insufficient:"+runID, draft.OwnerID(), insufficientTerms)
	if err != nil {
		t.Fatalf("create insufficient pact: %v", err)
	}
	if _, err := pactService.Confirm(ctx, insufficient.OwnerID(), insufficient.ID()); err != nil {
		t.Fatalf("confirm insufficient pact: %v", err)
	}
	if _, _, err := pactService.ReserveStake(ctx, "pact-insufficient-stake:"+runID, insufficient.OwnerID(), insufficient.ID()); !errors.Is(err, ledger.ErrInsufficientBalance) {
		t.Fatalf("reserve insufficient stake error = %v", err)
	}
	stored, err := pactService.Pact(ctx, insufficient.OwnerID(), insufficient.ID())
	if err != nil {
		t.Fatalf("read insufficient pact: %v", err)
	}
	if stored.Status() != pact.StatusAwaitingStake || stored.StakeReservationID() != "" {
		t.Fatalf("failed reservation changed pact: status=%s reservation=%s", stored.Status(), stored.StakeReservationID())
	}
}

func assertPactWalletBalances(
	t *testing.T,
	ctx context.Context,
	store *wallet.PostgresProvisioningStore,
	userID identity.UserID,
	available int64,
	staked int64,
) {
	t.Helper()
	snapshot, err := store.WalletByUser(ctx, userID)
	if err != nil {
		t.Fatalf("read wallet: %v", err)
	}
	if snapshot.Balances().Available.AmountMinor() != available || snapshot.Balances().Staked.AmountMinor() != staked {
		t.Fatalf("balances = available %d, staked %d; want %d, %d",
			snapshot.Balances().Available.AmountMinor(), snapshot.Balances().Staked.AmountMinor(), available, staked)
	}
}
