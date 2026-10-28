# Controlled — MVP Product Specification

Status: Locked foundation for backend implementation

## Product

Controlled is a PayPal-powered wallet that enforces financial commitments before money leaves. A user funds a dedicated Controlled balance, states a financial goal and spending intent, chooses a daily or weekly limit, and backs the commitment with an additional stake.

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
8. Route an approval request by round robin.
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
- Spending limit and currency
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
Recovery success:  Commitment Reserve -> available
```

The global Stake Vault stores separate allocations by user and pact. Commitment Reserve remains owned by the user but cannot be spent, sent, or withdrawn.

A reserve is unlocked only by successfully completing an eligible recovery pact. Recovery requirements will be finalized before that flow is implemented; they must prevent trivial pacts from immediately unlocking a reserve.

PYUSD and rewards are outside the initial MVP. Future versions may explore responsible ways for locked stakes to create user value, but the MVP promises no yield or return.

## Accountability

A circle contains `n` distinct accepted partners, where `n >= 1`. The owner cannot be a member of their own circle.

- Partners have a fixed order.
- One partner receives each request at a time.
- Assignment uses a round-robin pointer.
- The owner cannot select the reviewer.
- A timeout may reassign the request to the next partner.
- Only the active assignment can resolve the request.
- Responsiveness scoring measures response timing, never approval rate.

## Over-limit requests

An over-limit transaction remains blocked while its request is unresolved.

| Decision | Classification | Transaction | Pact | Stake |
|---|---|---|---|---|
| Deny | — | Blocked | Continues | Remains locked |
| Approve | Emergency | Allowed | Continues with exception | Remains locked |
| Approve | Conscious break | Allowed | Resolves as broken | Moves to Commitment Reserve |
| Timeout | — | Blocked or reassigned | Continues | Remains locked |

An approval is scoped to one transaction, amount, and set of blocking pacts. It is not a general increase to the spending limit.

## Transaction policy

- Pending outgoing transactions reserve wallet and pact capacity immediately.
- Failed, cancelled, or expired transactions release reserved capacity.
- Confirmed refunds restore capacity to affected pacts.
- Incoming transfers do not consume pact limits.
- Incoming funds do not increase a fixed pact limit.
- An active pact cannot be cancelled directly; exit requires the conscious-break flow.

## AI after the non-AI MVP

The deterministic wallet must work without AI. Later additions are:

- AI Pact Planner grounded in wallet history and the user's goal
- AI Accountability Brief grounded in verified transaction and pact facts

AI may propose and summarize. It may not authorize a debit, classify an emergency, approve a request, change active terms, post ledger entries, or call PayPal directly.

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

## Out of scope

- Production custody or money-transmission claims
- PYUSD, yield, or investment returns
- Physical or virtual cards
- Monitoring spending outside Controlled
- Lending or credit
- Partial Commitment Reserve allocation
- Production identity verification and compliance
- AI in the initial deterministic backend milestone
