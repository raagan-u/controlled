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
	).EnableAccountability(accountabilityService)
	accountabilityService.EnableExecutor(operations)
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
	progress, err := pactService.PactProgress(ctx, active.OwnerID(), active.ID())
	if err != nil {
		t.Fatalf("read pact progress: %v", err)
	}
	if progress.UsedMinor != 3_000 || progress.RemainingMinor != 2_000 || len(progress.Usage) != 1 {
		t.Fatalf("pact progress = %+v", progress)
	}
	if progress.Usage[0].CounterpartyCtrldID != partner.Provisioning.User().CtrldID() ||
		progress.Usage[0].ReversedAt != (time.Time{}) {
		t.Fatalf("pact usage = %+v", progress.Usage[0])
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
	attempt, err := operations.TransferWithReason(
		ctx, "pact-reviewed-transfer:"+runID, owner.Provisioning.User().ID(),
		partner.Provisioning.User().Email(), 2_001, "USD", "Unexpected essential expense",
	)
	if err != nil || attempt.Request == nil {
		t.Fatalf("create accountability request: attempt=%+v err=%v", attempt, err)
	}
	storedRequest, err := accountabilityService.Request(
		ctx, owner.Provisioning.User().ID(), attempt.Request.ID,
	)
	if err != nil {
		t.Fatalf("read accountability request: %v", err)
	}
	if storedRequest.Status != "ASSIGNED" || storedRequest.AssignedPartnerID != partner.Provisioning.User().ID() ||
		len(storedRequest.Pacts) != 1 || storedRequest.Pacts[0].OverageMinor != 1 {
		t.Fatalf("accountability request = %+v", storedRequest)
	}
	deniedRequest, err := accountabilityService.Decide(
		ctx, partner.Provisioning.User().ID(), storedRequest.ID, false, "", "The expense can wait",
	)
	if err != nil {
		t.Fatalf("deny accountability request: %v", err)
	}
	if deniedRequest.Status != "DENIED" || deniedRequest.AssignedPartnerID != "" ||
		len(deniedRequest.Reviewers) != 1 || deniedRequest.Reviewers[0].DecisionReason != "The expense can wait" {
		t.Fatalf("denied accountability request = %+v", deniedRequest)
	}
	timeoutAttempt, err := operations.TransferWithReason(
		ctx, "pact-timeout-transfer:"+runID, owner.Provisioning.User().ID(),
		partner.Provisioning.User().Email(), 2_001, "USD", "Timeout test",
	)
	if err != nil || timeoutAttempt.Request == nil {
		t.Fatalf("create timeout request: attempt=%+v err=%v", timeoutAttempt, err)
	}
	requestRepository := accountability.NewPostgresRepository(pool)
	maintenanceResult, err := requestRepository.MaintainRequests(ctx, timeoutAttempt.Request.CreatedAt.Add(7*time.Hour))
	if err != nil || maintenanceResult.TimedOut < 1 {
		t.Fatalf("timeout request maintenance = %+v err=%v", maintenanceResult, err)
	}
	maintenanceResult, err = requestRepository.MaintainRequests(ctx, timeoutAttempt.Request.CreatedAt.Add(25*time.Hour))
	if err != nil || maintenanceResult.Expired < 1 {
		t.Fatalf("expire request maintenance = %+v err=%v", maintenanceResult, err)
	}
	expiredRequest, err := accountabilityService.Request(ctx, owner.Provisioning.User().ID(), timeoutAttempt.Request.ID)
	if err != nil || expiredRequest.Status != "EXPIRED" {
		t.Fatalf("expired accountability request = %+v err=%v", expiredRequest, err)
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
	emergencyAttempt, err := operations.TransferWithReason(
		ctx, "pact-emergency:"+runID, owner.Provisioning.User().ID(),
		partner.Provisioning.User().Email(), 1, "USD", "Emergency test",
	)
	if err != nil || emergencyAttempt.Request == nil {
		t.Fatalf("create emergency request: attempt=%+v err=%v", emergencyAttempt, err)
	}
	decisionOnlyService := accountability.NewService(ids, clock, accountability.NewPostgresRepository(pool))
	_, err = decisionOnlyService.Decide(
		ctx, partner.Provisioning.User().ID(), emergencyAttempt.Request.ID, true, "EMERGENCY", "",
	)
	if !errors.Is(err, accountability.ErrUnavailable) {
		t.Fatalf("interrupted emergency approval error = %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE transaction_intents SET status = 'FAILED' WHERE id = $1::text::uuid`, string(emergencyAttempt.Request.IntentID)); err != nil {
		t.Fatalf("simulate legacy failed approved intent: %v", err)
	}
	recoveryResult, err := accountabilityService.RunMaintenance(ctx)
	if err != nil || recoveryResult.Recovered < 1 {
		t.Fatalf("recover emergency approval = %+v err=%v", recoveryResult, err)
	}
	emergencyRequest, err := accountabilityService.Request(
		ctx, owner.Provisioning.User().ID(), emergencyAttempt.Request.ID,
	)
	if err != nil {
		t.Fatalf("read recovered emergency request: %v", err)
	}
	if emergencyRequest.ExecutedOperationID == "" {
		t.Fatalf("emergency request was not executed: %+v", emergencyRequest)
	}
	active, err = pactService.Pact(ctx, active.OwnerID(), active.ID())
	if err != nil || active.Status() != pact.StatusActive || active.EmergencyCount() != 1 {
		t.Fatalf("pact after emergency = %+v err=%v", active, err)
	}
	breakAttempt, err := operations.TransferWithReason(
		ctx, "pact-conscious-break:"+runID, owner.Provisioning.User().ID(),
		partner.Provisioning.User().CtrldID(), 1, "USD", "Conscious break test",
	)
	if err != nil || breakAttempt.Request == nil {
		t.Fatalf("create conscious break request: attempt=%+v err=%v", breakAttempt, err)
	}
	breakRequest, err := accountabilityService.Decide(
		ctx, partner.Provisioning.User().ID(), breakAttempt.Request.ID, true, "CONSCIOUS_BREAK", "",
	)
	if err != nil {
		t.Fatalf("approve conscious break: %v", err)
	}
	if breakRequest.ExecutedOperationID == "" {
		t.Fatalf("conscious break request was not executed: %+v", breakRequest)
	}
	active, err = pactService.Pact(ctx, active.OwnerID(), active.ID())
	if err != nil || active.Status() != pact.StatusResolved || active.Outcome() != pact.OutcomeBroken {
		t.Fatalf("pact after conscious break = %+v err=%v", active, err)
	}
	brokenWallet, err := walletStore.WalletByUser(ctx, owner.Provisioning.User().ID())
	if err != nil {
		t.Fatalf("read wallet after conscious break: %v", err)
	}
	if brokenWallet.Balances().Staked.AmountMinor() != 0 || brokenWallet.Balances().CommitmentReserve.AmountMinor() != 2_500 {
		t.Fatalf("balances after conscious break = %+v", brokenWallet.Balances())
	}
	lifecycleStart := now.Add(2 * time.Hour)
	lifecycleEnd := lifecycleStart.Add(7 * 24 * time.Hour)
	lifecycleStake, _ := pact.NewMoney(1_000, "USD")
	lifecycleTerms, _ := pact.NewTerms(
		"Lifecycle pact", "Verify automatic transitions", limit, lifecycleStake,
		lifecycleStart, lifecycleEnd, pact.CircleID(circle.ID()), pact.BreakConsequenceCommitmentReserve,
	)
	lifecyclePact, _, err := pactService.CreateDraft(ctx, "pact-lifecycle:"+runID, draft.OwnerID(), lifecycleTerms)
	if err != nil {
		t.Fatalf("create lifecycle pact: %v", err)
	}
	if _, err := pactService.Confirm(ctx, lifecyclePact.OwnerID(), lifecyclePact.ID()); err != nil {
		t.Fatalf("confirm lifecycle pact: %v", err)
	}
	lifecyclePact, _, err = pactService.ReserveStake(ctx, "pact-lifecycle-stake:"+runID, lifecyclePact.OwnerID(), lifecyclePact.ID())
	if err != nil || lifecyclePact.Status() != pact.StatusScheduled {
		t.Fatalf("reserve lifecycle pact: pact=%+v err=%v", lifecyclePact, err)
	}
	maintenanceClock := &integrationClock{now: lifecycleStart.Add(time.Minute)}
	lifecycleService := pact.NewService(ids, maintenanceClock, pact.NewPostgresRepository(pool))
	result, err := lifecycleService.RunMaintenance(ctx)
	if err != nil || result.Started != 1 {
		t.Fatalf("start pact maintenance = %+v err=%v", result, err)
	}
	maintenanceClock.now = lifecycleEnd.Add(time.Minute)
	result, err = lifecycleService.RunMaintenance(ctx)
	if err != nil || result.Completed != 1 {
		t.Fatalf("complete pact maintenance = %+v err=%v", result, err)
	}
	lifecyclePact, err = pactService.Pact(ctx, lifecyclePact.OwnerID(), lifecyclePact.ID())
	if err != nil || lifecyclePact.Status() != pact.StatusResolved || lifecyclePact.Outcome() != pact.OutcomeKept {
		t.Fatalf("completed lifecycle pact = %+v err=%v", lifecyclePact, err)
	}
	recoveredWallet, err := walletStore.WalletByUser(ctx, owner.Provisioning.User().ID())
	if err != nil {
		t.Fatalf("read wallet after recovery: %v", err)
	}
	if recoveredWallet.Balances().CommitmentReserve.AmountMinor() != 1_500 {
		t.Fatalf("commitment reserve after recovery = %d, want 1500", recoveredWallet.Balances().CommitmentReserve.AmountMinor())
	}
	var recoveredMinor, applicationMinor int64
	err = pool.QueryRow(ctx, `
		SELECT allocation.recovered_minor, application.amount_minor
		  FROM stake_allocations AS allocation
		  JOIN stake_recovery_applications AS application
		    ON application.source_pact_id = allocation.pact_id
		 WHERE allocation.pact_id = $1::text::uuid
		   AND application.recovery_pact_id = $2::text::uuid
	`, string(active.ID()), string(lifecyclePact.ID())).Scan(&recoveredMinor, &applicationMinor)
	if err != nil || recoveredMinor != 1_000 || applicationMinor != 1_000 {
		t.Fatalf("stake recovery = recovered %d, application %d, err %v", recoveredMinor, applicationMinor, err)
	}

	largeStake, _ := pact.NewMoney(15_000, "USD")
	insufficientTerms, _ := pact.NewTerms(
		"Too large", "Verify atomic failure", limit, largeStake,
		now.Add(time.Hour), now.Add(25*time.Hour), pact.CircleID(circle.ID()),
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

type integrationClock struct{ now time.Time }

func (c *integrationClock) Now() time.Time { return c.now }

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
