# Controlled — MVP Product Specification

Status: Locked foundation for backend implementation

## Product

Controlled is a PayPal-powered wallet that enforces financial commitments before money leaves. A user funds a dedicated Controlled balance, states a financial goal and spending intent, chooses one total limit for a fixed pact period, and backs the commitment with an additional stake. Users layer daily, weekly, monthly, or custom-period limits by creating multiple pacts.

Without an active pact, the wallet can send, receive, spend, and withdraw freely. While one or more pacts are active, every outgoing transaction is evaluated against all of them.

The MVP uses an internal wallet ledger backed by PayPal Sandbox. It is a proof of concept, not a production bank or custodial-wallet service.

## MVP goal

Demonstrate one complete non-AI flow:

1. Onboard a user and create a Controlled wallet.
2. Fund the wallet through PayPal Sandbox.
3. Create a goal-based spending pact.
4. Select Commitment Reserve as the break consequence.
5. Add at least one accountability partner.
6. Lock the stake from the wallet's available balance.
7. Attempt an outgoing transaction that exceeds an active limit.
8. Route an approval request through the pact's ordered accountability circle.
9. Approve or deny the transaction.
10. Preserve the stake after an emergency or move it to Commitment Reserve after a conscious break.
11. Record an auditable timeline of the entire flow.

## Wallet

Every onboarded user receives one internal Controlled wallet with:

- Available balance
- Pending balance
- Active stake allocations
- Commitment Reserve balance

Actual sandbox backing funds sit in a PayPal account. The internal double-entry ledger records ownership and availability.

Every outgoing path passes through Controlled policy:

- Internal sends between Controlled users
- External sends
- Merchant payments in the controlled demo
- Withdrawals
- Provider and platform fees included in the debit

Incoming money is unrestricted. Money outside Controlled is outside its enforcement boundary.

## Pact

A pact contains:

- Owner
- Financial goal
- Spending intent
- Total spending limit for the complete pact period
- Stake amount
- Start and end timestamps
- Accountability circle
- Break consequence
- Status and version

The only enabled MVP break consequence is:

```text
COMMITMENT_RESERVE
```

Neutral fund, goal vault, and split consequences may appear in the interface as disabled future options.

Pact lifecycle:

```text
DRAFT
  -> AWAITING_STAKE
  -> SCHEDULED
  -> ACTIVE
  -> RESOLVED

DRAFT | AWAITING_STAKE | SCHEDULED -> CANCELLED
```

Resolved outcomes:

- `KEPT`
- `KEPT_WITH_EXCEPTIONS`
- `BROKEN`

Active pact terms are immutable.

Times are entered and displayed in the user's local timezone and stored as UTC. A pact period includes its start timestamp and excludes its end timestamp. A confirmed pact becomes active immediately when its start has arrived, or remains scheduled until automatic activation.

The MVP is USD-only. Limits and stakes use positive integer cents. A stake may be any amount greater than zero up to the owner's available balance; configurable product caps are deferred.

## Multiple active pacts

- A user may have more than one active pact.
- Every outgoing transaction is evaluated against every active pact.
- The same transaction consumes capacity in every applicable pact.
- Every active pact must permit the transaction.
- An approval request references all pacts blocking that transaction.
- Each affected pact records its own exception or break outcome.

## Stake Vault and Commitment Reserve

Stake movement is internal ledger movement, not a separate PayPal purchase:

```text
Pact activation:   available -> Stake Vault
Pact success:      Stake Vault -> available
Emergency:         stake remains in Stake Vault
Conscious break:   Stake Vault -> Commitment Reserve
```

The global Stake Vault stores separate allocations by user and pact. Commitment Reserve remains owned by the user but cannot be spent, sent, or withdrawn.

Commitment Reserve remains user-owned but unavailable indefinitely in the MVP. The product may show a `Start recovery pact` entry point, but no release or recovery-qualification rules exist yet.

PYUSD and rewards are outside the initial MVP. Future versions may explore responsible ways for locked stakes to create user value, but the MVP promises no yield or return.

## Accountability

A circle contains `n` distinct partners who accepted the owner's invitation, where `n >= 1`. The owner cannot be a member of their own circle. Partners may participate in multiple circles and pacts.

- The owner explicitly sets the partner order.
- The ordered circle is immutable after pact confirmation.
- One partner receives each request at a time.
- The owner cannot select the reviewer.
- Each assignment lasts six hours and the complete request expires after 24 hours.
- Denial or non-response advances to the next eligible partner.
- If every partner denies, the request and transaction are cancelled.
- If nobody eligible remains without every partner denying, the request waits until expiry.
- Only the active assignment can resolve the request.
- The owner may cancel a pending request, which also cancels that transaction attempt.
- A request requires the owner's reason and exposes verified transaction and pact facts.

## Over-limit requests

An otherwise valid over-limit transaction remains blocked while its request is unresolved. Insufficient funds cannot be overridden and do not create an accountability request.

| Decision | Classification | Transaction | Pact | Stake |
|---|---|---|---|---|
| Deny | — | Blocked | Continues | Remains locked |
| Approve | Emergency | Allowed | Continues with exception | Remains locked |
| Approve | Conscious break | Allowed | Resolves as broken | Moves to Commitment Reserve |
| Timeout | — | Cancelled or reassigned | Continues | Remains locked |

An approval is scoped to the exact transaction type, amount, destination, and set of blocking pacts. It is single-use and not a general increase to the spending limit. Approval immediately attempts execution after atomically rechecking the balance and every currently active pact. A failed execution requires a new transaction attempt.

## Transaction policy

- A blocked transaction does not reserve wallet funds or pact capacity while awaiting approval.
- Successful internal transfers count immediately; withdrawals count when their funds are reserved for settlement.
- Failed or reversed debits remove their amount from pact usage.
- A completed debit continues counting unless that debit itself is reversed; a separate repayment is simply incoming money.
- Multiple pending requests may exist, but approval always performs a fresh atomic policy and balance check.
- Incoming transfers do not consume pact limits.
- Incoming funds do not increase a fixed pact limit.
- Funding, stake movements, and Commitment Reserve movements do not consume pact limits.
- An active pact cannot be cancelled directly; exit requires the conscious-break flow.
- At pact expiry, pending requests are cancelled and the pact resolves as `KEPT` or `KEPT_WITH_EXCEPTIONS`.

## AI after the non-AI MVP

The deterministic wallet must work without AI. Later additions are:

- AI Pact Planner grounded in wallet history and the user's goal
- AI Accountability Brief grounded in verified transaction and pact facts

AI may propose terms, explain activity, draft an accountability reason, and summarize verified facts. It may not authorize a debit, classify an emergency, approve a request, change active terms, override policy, post ledger entries, or call PayPal directly.

## Business model

Controlled may charge a small fee when a pact activates. The fee must be:

- Disclosed before activation
- Separate from the stake
- Independent of success or failure
- Refunded if activation fails
- Unrelated to Commitment Reserve balances

## Core invariants

1. Internal journal transactions must balance.
2. Available wallet balances cannot become negative.
3. A pact cannot activate until its stake is reserved from the owner's wallet.
4. The owner cannot belong to their own accountability circle.
5. A circle must have at least one distinct accepted partner.
6. Active pact terms cannot be edited.
7. Every active pact must permit an outgoing transaction.
8. Only an active assignment can resolve an approval request.
9. One approval grants only its referenced transaction and amount.
10. A pact can resolve only once.
11. Payment, webhook, ledger, and public command retries must be idempotent.
12. Emergency exceptions are recorded without being treated as absolute failure.
13. Controlled never receives a broken pact's stake.
14. Every consequential state change creates an audit event.
15. All monetary values are USD integer cents; floating-point money is forbidden.
16. Public commands, partner decisions, and transaction execution are idempotent.

## Out of scope

- Production custody or money-transmission claims
- PYUSD, yield, or investment returns
- Physical or virtual cards
- Monitoring spending outside Controlled
- Lending or credit
- Partial Commitment Reserve allocation
- Production identity verification and compliance
- AI in the initial deterministic backend milestone
