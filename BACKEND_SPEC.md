# Controlled Backend Specification

Status: Foundation aligned for hackathon MVP

Language: Go

Architecture: Modular monolith

API: JSON over HTTP

Database: PostgreSQL

External payment system: PayPal Sandbox

## 1. Responsibility

The backend is authoritative for:

- User and wallet creation
- Double-entry ledger accounting
- Wallet funding, transfers, and withdrawals
- Pact lifecycle and immutable terms
- Account-wide outgoing transaction policy
- Stake Vault and Commitment Reserve movement
- Accountability assignment and decisions
- PayPal orchestration and webhook processing
- Timeouts, retries, reconciliation, and scheduled completion
- Immutable audit history

The frontend contains no balance, authorization, or lifecycle rules.

## 2. System boundary

```text
Web client ─────────────┐
Background worker ──────┼──> Go application services
Integration tests ──────┘             │
                                      ├──> PostgreSQL
                                      ├──> PayPal Sandbox
                                      └──> AI provider later
```

Actual sandbox funds are held in a PayPal account. PostgreSQL records the users to whom those funds belong and whether they are available, pending, staked, or reserved.

Money outside Controlled is outside the enforcement boundary.

## 3. Project structure

```text
cmd/api/                    API process
internal/identity/          Users and demo sessions
internal/ledger/            Accounts, journals, entries, balances
internal/wallet/            Funding, sends, receives, withdrawals
internal/pact/              Terms, lifecycle, outcomes
internal/policy/            Evaluation of outgoing transactions
internal/transaction/       Transaction intents and execution state
internal/accountability/    Circles, assignments, decisions, scores
internal/stake/             Stake Vault and Commitment Reserve
internal/payments/          Custody and PayPal orchestration
internal/jobs/              Outbox, retries, timeouts, schedules
internal/audit/             Product audit timeline
internal/ai/                Post-MVP AI orchestration
internal/platform/          HTTP, PostgreSQL, config, clock, IDs
migrations/                 PostgreSQL migrations
sql/                        Queries if sqlc is adopted
openapi/                    HTTP contract
```

Keep each module pragmatic. Prefer module-owned files such as `domain.go`, `service.go`, `repository.go`, and `http.go` over deep ceremonial layering.

No module updates another module's tables directly. Cross-module state changes use application services inside one database transaction. External side effects use an outbox.

## 4. Technical stack

- Go 1.27.2
- `net/http` with a small router such as `chi`
- PostgreSQL
- `pgx`
- SQL migrations
- Standard `net/http` client for PayPal REST APIs
- `slog` for structured logs
- OpenAPI for the public contract

Redis, Kafka, microservices, and PYUSD are out of scope for the MVP.

## 5. Data conventions

- IDs are UUIDs.
- Timestamps are UTC `timestamptz`.
- Money uses integer minor units and an ISO currency code.
- Floating point must never represent money.
- JSON uses `camelCase`; SQL uses `snake_case`.
- State values are uppercase strings.
- Mutable aggregates use integer versions for optimistic concurrency.
- Ledger and audit records are append-only.

```json
{
  "amountMinor": 2500,
  "currency": "USD"
}
```

## 6. Identity and wallet onboarding

Creating a user atomically creates:

- One Controlled wallet
- Available ledger account
- Pending ledger account
- Commitment Reserve ledger account

The wallet starts with a zero balance. A PayPal identity or payment method may be connected later, but it is not the Controlled wallet itself.

Initial demo endpoints:

```text
POST   /v1/demo/users
POST   /v1/demo/sessions
DELETE /v1/sessions/current
GET    /v1/users/current
GET    /v1/wallet
GET    /v1/wallet/transactions
```

## 7. Double-entry ledger

The ledger is the monetary source of truth.

Core records:

- Ledger account
- Journal transaction
- Debit entry
- Credit entry
- Currency
- Reference type and ID
- Idempotency key
- Posted or reversed status

Minimum tables:

```text
ledger_accounts
ledger_transactions
ledger_entries
idempotency_keys
```

Invariants:

1. Every posted journal balances.
2. A journal contains one currency.
3. Posted entries are immutable.
4. A reversal creates a new journal.
5. An idempotency key posts at most once.
6. Available balances cannot become negative.
7. No application code edits a stored balance directly.

A cached balance projection is permitted but must reconcile to entries.

## 8. Wallet funding and custody

Funding flow:

```text
Create funding intent
-> obtain PayPal approval
-> PayPal capture succeeds
-> verify provider result or webhook
-> credit user's available ledger account
```

The browser redirect is never authoritative.

Internal sends between Controlled users use only ledger movement. External sends and withdrawals reserve funds internally, call the provider asynchronously, and then settle or release the reservation.

All outgoing wallet operations pass through pact policy, including withdrawal.

Custody adapter:

```go
type Custody interface {
    CreateFundingOrder(context.Context, CreateFundingOrder) (FundingOrder, error)
    CaptureFundingOrder(context.Context, CaptureFundingOrder) (FundingCapture, error)
    CreatePayout(context.Context, CreatePayout) (Payout, error)
    GetPayout(context.Context, string) (Payout, error)
    VerifyWebhook(context.Context, WebhookRequest) (VerifiedEvent, error)
}
```

Implementations:

- `FakeCustody` for deterministic application and integration tests
- `PayPalCustody` after the full fake-provider flow works

## 9. Pact domain

A pact contains:

- Owner ID
- Financial goal
- Spending intent
- Spending limit
- Stake amount
- Start and end timestamps
- Accountability circle ID
- Break consequence
- Status, outcome, and version
- Internal stake reservation ID after activation

The only valid MVP break consequence is:

```text
COMMITMENT_RESERVE
```

Lifecycle:

```text
DRAFT
  -> AWAITING_STAKE
  -> SCHEDULED
  -> ACTIVE
  -> RESOLVED

DRAFT | AWAITING_STAKE | SCHEDULED -> CANCELLED
```

Outcomes:

```text
NONE
KEPT
KEPT_WITH_EXCEPTIONS
BROKEN
CANCELLED
```

Terms are immutable after confirmation.

## 10. Stake Vault and Commitment Reserve

The Stake Vault is one global internal ledger account with an allocation record for each pact.

Minimum stake data:

```text
stake_allocations
  id
  owner_id
  pact_id
  amount_minor
  currency
  ledger_transaction_id
  status
  created_at
  resolved_at
```

Ledger movement:

```text
Pact activation:   user available -> Stake Vault
Pact success:      Stake Vault -> user available
Scheduled cancel:  Stake Vault -> user available
Emergency:         no stake movement
Conscious break:   Stake Vault -> user Commitment Reserve
Recovery success:  Commitment Reserve -> user available
```

The ledger journal and pact transition must commit in the same database transaction.

Recovery rules must prevent a trivial follow-up pact from immediately unlocking reserves. Their detailed eligibility policy is deferred until the recovery flow is implemented.

## 11. Accountability

A circle contains `n` accepted partners where `n >= 1`.

Rules:

- Every partner is distinct.
- The owner cannot be a partner.
- Partners have stable positions.
- Each request has one active assignment.
- Assignment advances round robin.
- The owner cannot select the reviewer.
- Only the current assignment can resolve a request.
- A late response to a superseded assignment is rejected.
- Scores measure response timing, not approval or denial.

Core states:

```text
Request:     REQUESTED -> ASSIGNED -> APPROVED | DENIED | EXPIRED
Assignment:  ACTIVE -> ACKNOWLEDGED -> RESPONDED | TIMED_OUT | SUPERSEDED
```

An approved response must classify the request as:

- `EMERGENCY`
- `CONSCIOUS_BREAK`

## 12. Transaction intents and policy

Every outgoing operation begins as a transaction intent:

```text
CREATED
  -> CHECKING_POLICY
  -> AUTHORIZED
  -> APPROVAL_REQUIRED
  -> APPROVED
  -> EXECUTING
  -> COMPLETED

Any non-terminal state -> FAILED | CANCELLED | EXPIRED
```

Policy evaluation:

1. Load every active pact for the owner.
2. Calculate confirmed and pending consumption for each pact.
3. Evaluate the proposed total debit against each pact.
4. Return every blocking pact.
5. Authorize immediately only if every pact permits the transaction.
6. Otherwise create one accountability request referencing all blocking pacts.

Rules:

- The same outgoing transaction consumes capacity in every active pact.
- Pending transactions reserve wallet and pact capacity.
- Failed and expired transactions release capacity.
- Confirmed refunds restore capacity.
- Incoming funds do not consume or increase fixed pact limits.
- An approval applies only to its referenced transaction and amount.

Resolution effects:

| Partner decision | Transaction | Pact effect | Stake effect |
|---|---|---|---|
| Deny | Blocked | Continues | Remains locked |
| Approve emergency | Allowed | Records exception | Remains locked |
| Approve conscious break | Allowed | Resolves as broken | Moves to Commitment Reserve |
| Timeout | Blocked or reassigned | Continues | Remains locked |

## 13. Database model

Initial target tables:

```text
users
sessions
wallets
ledger_accounts
ledger_transactions
ledger_entries
idempotency_keys
pacts
circles
circle_members
stake_allocations
transaction_intents
pact_transaction_evaluations
approval_requests
assignments
partner_responses
funding_intents
payouts
processed_webhooks
outbox_events
scheduled_jobs
audit_events
```

Use foreign keys, positive-amount checks, valid-time checks, unique provider IDs, unique idempotency keys, and a partial unique index enforcing one active assignment per request.

## 14. Transactions and external calls

Every state-changing application command runs in a database transaction.

Never hold a database transaction open while calling PayPal:

1. Validate and reserve internally.
2. Commit state and an outbox event.
3. Call the provider outside the database transaction.
4. Persist the result in a new transaction.
5. Settle or release the reservation.

Use row locks or optimistic versions for wallet debits, pact transitions, assignment resolution, stake movement, and webhook application.

## 15. Idempotency

State-changing public endpoints accept `Idempotency-Key`.

Store:

- Caller identity
- Command name
- Idempotency key
- Request hash
- Final response

Reusing a key with a different request returns a conflict.

Use stable PayPal request IDs, and enforce uniqueness for orders, captures, payouts, ledger references, and webhook event IDs.

## 16. Outbox and background jobs

PostgreSQL is the MVP job queue. Workers claim work using row locking and leases.

Jobs include:

- Start scheduled pact
- Complete active pact
- Expire or reassign partner assignment
- Send notification
- Execute funding capture
- Execute external payout
- Reconcile provider transaction
- Deliver audit or notification event

Jobs must be safe to run more than once. Retry transient failures with bounded backoff and expose permanent failures in the audit timeline.

## 17. PayPal webhooks

```text
POST /v1/webhooks/paypal
```

The handler must:

1. Preserve the original body.
2. Verify authenticity.
3. insert the event ID under a unique constraint.
4. Return success for an already processed event.
5. Apply the state change transactionally.
6. Return failure only when a retry is useful.

## 18. Audit timeline

Record every consequential fact:

- User and wallet creation
- Funding initiated and completed
- Pact created, confirmed, scheduled, started, and resolved
- Stake reserved, released, or moved to Commitment Reserve
- Transaction created, authorized, blocked, approved, denied, and completed
- Partner assigned, acknowledged, timed out, and responded
- External payout initiated and settled
- Provider webhook applied

Audit events are append-only and contain actor, aggregate, timestamp, correlation ID, and safe metadata.

## 19. AI boundary

AI is not required for the first backend milestone.

Later, AI may:

- Propose pact terms using wallet history and user intent
- Produce a factual partner brief from verified records

AI may not:

- Calculate authoritative balances
- Post ledger entries
- Authorize transactions
- Approve or classify a request
- Change active pact terms
- Call PayPal

## 20. Error format

```json
{
  "error": {
    "code": "PACT_INVALID_TRANSITION",
    "message": "The requested pact transition is not allowed.",
    "requestId": "6dd8dc79-34d4-4f97-8926-469578d32545",
    "details": {}
  }
}
```

Expected domain failures return `4xx`. Unexpected failures return `500` without exposing credentials or raw provider responses.

## 21. Test strategy

### Unit tests

- Ledger balance and immutability rules
- Pact transitions and terms
- Stake consequence decisions
- Policy evaluation across multiple pacts
- Round-robin assignment
- Timeout and response scoring

### PostgreSQL integration tests

- Concurrent debits
- Atomic stake and pact transitions
- Idempotency conflicts
- Outbox claiming
- Webhook replay
- One-active-assignment constraint

### Provider contract tests

Run the same wallet application scenarios against `FakeCustody` and PayPal Sandbox adapters where practical.

### End-to-end non-AI flow

```text
create user
-> create wallet
-> fund wallet
-> create circle and pact
-> reserve stake
-> attempt over-limit withdrawal
-> approve conscious break
-> execute withdrawal
-> move stake to Commitment Reserve
-> inspect audit timeline
```

## 22. Security baseline

- Keep provider credentials server-side.
- Store session cookies as secure, HTTP-only, and same-site.
- Hash single-purpose review tokens.
- Verify every webhook.
- Rate-limit authentication, funding, payout, and partner-decision endpoints.
- Never log secrets, full tokens, or unnecessarily sensitive financial payloads.
- Require authenticated ownership checks on every wallet and pact command.
- Validate money, currency, state, and references at every boundary.

## 23. MVP completion criteria

The deterministic backend is complete when APIs can demonstrate:

1. User onboarding creates a wallet and ledger accounts.
2. Fake custody and PayPal Sandbox can fund that wallet.
3. Internal sends preserve ledger balance.
4. Pacts reserve stakes from wallet funds.
5. Every outgoing debit is checked against all active pacts.
6. Accountability can approve emergencies or conscious breaks.
7. Conscious breaks move stakes into Commitment Reserve.
8. Recovery can release an eligible reserve.
9. Retries do not duplicate money or decisions.
10. The audit timeline explains every consequential action.
