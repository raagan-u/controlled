# Controlled — Backend specification

Status: implemented hackathon backend

## 1. Responsibilities

The backend is the authority for identity, sessions, wallet balances, ledger history, custody operations, pacts, policy enforcement, accountability, payment requests, provider integrations, notifications, and audit state.

The mobile client renders state and submits commands. It does not calculate authoritative balances, pact outcomes, reviewer eligibility, or transaction authorization.

## 2. Architecture

Controlled is a Go modular monolith served by `cmd/api`.

```text
HTTP transport and session middleware
  |
  +-- identity            users, Google/password auth, sessions, recipients
  +-- wallet              provisioning and read models
  +-- ledger              double-entry accounts and journals
  +-- walletops           funding, transfer, withdrawal, policy integration
  +-- paymentrequest      request/pay/decline/cancel lifecycle
  +-- pact                terms, stake, progress, lifecycle
  +-- accountability      invitations, circles, review, execution recovery
  +-- ai                  grounded guidance with deterministic fallback
  +-- shopping            Channel3-backed price checks
  +-- notification        device registry and Expo delivery worker
  +-- platform            config, PostgreSQL, migrations, HTTP, logging
```

Domain services own validation and lifecycle rules. PostgreSQL repositories own atomic persistence. HTTP handlers decode strict JSON, map domain errors to stable error codes, and never implement financial policy themselves.

## 3. Runtime stack

- Go `1.27.2`
- standard-library `net/http` method/path routing
- PostgreSQL 17 through `pgx/v5`
- embedded SQL migrations with advisory locking and checksum validation
- Zap structured logging
- Expo Push Service
- selectable fake or PayPal Sandbox custody
- optional OpenAI-compatible AI provider
- optional Channel3 catalog provider
- Expo React Native client using bearer sessions

## 4. Startup and shutdown

Startup performs the following in order:

1. load and validate configuration;
2. create the configured Zap logger;
3. open and ping the PostgreSQL pool;
4. verify/apply embedded migrations;
5. initialize identity, wallet, providers, pacts, accountability, payment requests, AI, shopping, and push services;
6. log an explicit enabled/disabled provider status for each feature;
7. start the notification worker and lifecycle maintenance loop; and
8. listen for HTTP traffic.

The server exposes `/healthz` for process liveness and `/readyz` for database readiness. `SIGINT`/`SIGTERM` trigger graceful HTTP shutdown within `CONTROLLED_SHUTDOWN_TIMEOUT`.

## 5. HTTP conventions

- JSON request bodies are limited to 1 MiB.
- Unknown fields, multiple JSON values, and malformed input are rejected.
- Protected routes accept `Authorization: Bearer <token>` or the `controlled_session` cookie.
- Session lifetime is 24 hours.
- IDs are opaque UUID strings.
- Timestamps are RFC 3339 UTC values.
- Authoritative money uses integer minor units and a three-letter currency code; public product flows require USD.
- Errors use `{ "error": { "code": "...", "message": "..." } }`.
- Create/money commands use `Idempotency-Key`; replay with identical semantic input returns the original resource, while conflicting reuse returns `409`.

## 6. Identity and sessions

Supported authentication:

- **Google:** verifies signature, issuer, expiry, verified email, and configured Web client-ID audience. A valid identity provisions a user/wallet when needed and issues a Controlled session.
- **Password:** registration provisions the user/wallet and stores a bcrypt hash; login verifies it and issues a session.
- **Demo:** creates users and starts cookie sessions for local integration testing only.

Users have a UUID, unique email, unique human-facing `ctrldId`, display name, `namePending` flag, and creation time. The current user can update their display name and search up to 50 other recipients by name, email, or Controlled ID.

Password reset and email verification do not exist yet.

## 7. Ledger and wallet

Each wallet owns ledger accounts for:

- `AVAILABLE`
- `PENDING`
- `COMMITMENT_RESERVE`

Stakes are posted to the system USD `STAKE_VAULT` account and attributed back to users/pacts through `stake_allocations`. Wallet reads expose the user's allocated total as the computed `staked` balance. Commitment Reserve allocations retain their source pact, recovered amount, and completion time; `stake_recovery_applications` records every partial oldest-first recovery and its ledger transaction.

Journal transactions contain balanced debit/credit entries. Database constraints and repository transactions prevent unbalanced postings, cross-currency movement, duplicate references, and negative available balances.

Wallet read models return account IDs and computed balances. Transaction read models include reference metadata, entries, reversal state, transfer parties/remarks, payment-request linkage, and pact-approval details when applicable.

## 8. Funding and custody

`CONTROLLED_CUSTODY_PROVIDER` selects:

- `fake` — deterministic, process-local orders and payouts for local development;
- `paypal_sandbox` — PayPal Orders v2 for funding and Payouts v1 for withdrawals.

Funding flow:

1. create a provider order and persist a funding intent;
2. return its approval URL when applicable;
3. capture/confirm the order;
4. credit the provider-reported net amount to available balance once; and
5. retain gross/net/provider identifiers for audit and idempotency.

Withdrawal flow:

1. validate balance and pact policy;
2. reserve available funds into pending and count pact usage;
3. create the provider payout;
4. reconcile provider status; and
5. on success consume pending, or on failure release pending and reverse pact usage.

Provider calls use bounded HTTP timeouts and idempotency/request identifiers. Provider failure cannot create ledger movement without the corresponding durable operation state.

## 9. Internal transfers and payment requests

An internal transfer atomically debits the sender's available account and credits the recipient's available account. Self-transfer, unknown recipient, non-USD input, non-positive amount, and insufficient balance are rejected. Optional remarks are persisted and returned with the transaction.

Payment requests are seven-day resources between two distinct users. States are:

```text
PENDING -> PROCESSING -> PAID
                    \-> AWAITING_APPROVAL -> PAID
PENDING -> DECLINED | CANCELLED | EXPIRED
```

The recipient pays through the same transfer/pact pipeline as a direct send. The note becomes the accountability reason when review is required. Repository transitions prevent duplicate payment and release a failed processing attempt back to a retryable state.

## 10. Pact domain

Terms contain goal, intent, spending limit, stake, period, circle ID, and `COMMITMENT_RESERVE` consequence. Public amounts are positive USD cents. Confirmed terms cannot be edited.

Statuses:

- `DRAFT`
- `AWAITING_STAKE`
- `SCHEDULED`
- `ACTIVE`
- `RESOLVED`
- `CANCELLED`

Outcomes:

- `NONE`
- `KEPT`
- `KEPT_WITH_EXCEPTIONS`
- `BROKEN`
- `CANCELLED`

Stake reservation, cancellation release, successful completion release, conscious-break movement, and recovery are ledger postings performed in the same database transaction as the pact transition. A kept pact may recover Commitment Reserve up to its own stake, but only from reserve allocations that predate its stake reservation. Recovery locks allocations oldest-first, supports partial lots, and is idempotent through the pact's terminal transition and ledger key.

Progress is derived from `pact_transaction_usage`. Successful applicable debits add usage once per pact. Reversed/failed withdrawal reservations mark usage reversed. Progress returns used, remaining, overage, and contributing operations.

## 11. Policy enforcement

Before an outgoing transfer or withdrawal is persisted, the PostgreSQL policy repository locks/rechecks the wallet and all active pacts.

- Insufficient available balance fails immediately.
- A debit within every active limit executes normally.
- A debit exceeding any active limit returns the blocking-pact snapshot.
- With a valid owner reason, the pending intent and one accountability request are persisted.
- Pending review does not reserve funds or capacity.

Approved execution re-runs the full check atomically. The approval authorizes only the stored operation type, destination, amount, currency, and blocking context. New balance failure or an unrelated pact block makes the approval stale/failed rather than bypassing policy.

## 12. Accountability

Invitations establish owner/partner relationships. A circle is an immutable ordered set of accepted partners owned by the pact owner.

An accountability request persists:

- transaction intent;
- blocking-pact snapshots;
- reviewer order;
- current assignment;
- owner reason;
- per-reviewer decision and denial reason;
- classification and terminal execution result; and
- controlled audit events.

Only the active reviewer can decide. Approval requires `EMERGENCY` or `CONSCIOUS_BREAK`; denial requires a non-empty reason. Each assignment expires after six hours and the whole request after 24 hours. Denial/timeout advances in circle order. Approval executes immediately when possible; maintenance recovers approved-but-not-executed requests after interruption.

Emergency increments each affected pact's exception count and retains stake. Conscious break resolves each affected pact as broken and moves each full stake allocation to Commitment Reserve before executing the debit.

## 13. Guidance and shopping

AI endpoints read verified server state and return a stable `guidance` schema. With `CONTROLLED_AI_PROVIDER=disabled`, rules generate the response. With `openai_compatible`, the model receives only a curated snapshot and must return strict structured output. The backend validates all output and falls back to rules on provider error.

AI has no command interface to wallet, ledger, pact, or accountability services.

The shopping endpoint validates a specific 3–160 character USD product query and positive intended amount. When `CHANNEL3_API_KEY` is configured, it searches up to five products, performs a lexical-confidence check, filters to in-stock new USD offers, and returns the lowest eligible price. Shopping failure returns a service error and never affects payment policy.

## 14. Notifications

Authenticated users register Expo tokens by platform. Notifications are queued in a bounded in-memory channel and delivered asynchronously; wallet and review commands never wait for push delivery. Invalid Expo tokens are removed.

Kinds are:

- `PAYMENT_REQUEST`
- `ACCOUNTABILITY_REQUEST`
- `ACCOUNTABILITY_INVITATION`
- `MONEY_RECEIVED`

Delivery covers new requests, money received, invitation responses, reviewer assignment, denial/reassignment, approval, cancellation, and payment-request decline/cancellation. Persistent domain state and action counts remain authoritative.

## 15. Maintenance

Maintenance runs once at startup and once per minute:

- activates scheduled pacts;
- completes ended pacts, releases their stake, and applies eligible Commitment Reserve recovery;
- cancels affected pending requests when pact periods end;
- advances timed-out accountability assignments;
- expires accountability requests;
- recovers approved debits not yet marked executed; and
- expires payment requests.

The operations are retry-safe and use database locking/terminal-state checks.

## 16. Persistence model

The migration set currently creates or evolves:

- `users`, `oauth_identities`, `password_credentials`, `sessions`
- `wallets`, `ledger_accounts`, `ledger_transactions`, `ledger_entries`
- `funding_intents`, `internal_transfers`, `withdrawals`, `idempotency_keys`
- `accountability_invitations`, `accountability_circles`, `accountability_circle_members`
- `pacts`, `stake_allocations`, `stake_recovery_applications`, `pact_transaction_usage`
- `transaction_intents`, `accountability_requests`, request pact/reviewer/assignment tables
- `payment_requests`
- `push_devices`
- `controlled_audit_events`

Later migrations add Controlled IDs, payment-request lifecycle fields, provider net funding amounts, pending-name state, transfer remarks, accountability denial reasons, and partial stake recovery tracking.

Applied migrations are immutable. The runner stores checksums and refuses startup if a previously applied migration changes.

## 17. Transactions and external calls

Database transactions never remain open across PayPal, AI, Channel3, or Expo network calls. Provider workflows use durable intermediate states and reconciliation. Ledger and policy transitions that must agree are committed together.

Critical lock order is stable: wallet/available account, relevant operation, active pacts/stake allocations, then derived usage/request state. The implementation relies on unique constraints and row locks in addition to domain validation.

## 18. Logging and observability

Zap emits structured JSON at configurable `DEBUG`, `INFO`, `WARN`, or `ERROR` levels. Logs cover:

- startup configuration without secrets;
- database connection/migration state;
- loaded/disabled providers and feature availability;
- HTTP method/path/status/duration;
- identity, wallet, pact, accountability, notification, AI, and shopping operations;
- provider failures and fallback decisions; and
- graceful maintenance/shutdown events.

Secrets, raw passwords, session tokens, Google ID tokens, and provider credentials must never be logged.

## 19. Security baseline

- bcrypt password hashing
- Google token verification on the server
- bearer/cookie session validation and revocation
- strict JSON decoding and body limits
- parameterized SQL
- secrets supplied through server environment only
- non-root, read-only API container with `no-new-privileges`
- PostgreSQL not exposed by the Compose stack
- bounded provider timeouts
- owner/reviewer authorization at repository/service boundaries

The MVP does not claim production KYC, compliance, custody, rate limiting, fraud detection, password recovery, or webhook-grade PayPal reconciliation.

## 20. Testing

The repository includes:

- unit tests for money, ledger, domain transitions, handlers, providers, logging, and config;
- PostgreSQL integration tests for wallet, identity, ledger, pact, accountability, and wallet-operation atomicity;
- fake provider tests;
- PayPal, OpenAI-compatible, Channel3, and Expo HTTP contract tests; and
- race-detector-compatible Go tests.

Run `go test ./...` for the standard suite. Set `CONTROLLED_TEST_DATABASE_URL` to include PostgreSQL integration tests.

## 21. Non-negotiable invariants

1. Every journal transaction balances.
2. Available balance cannot be negative.
3. Provider retries and public commands cannot duplicate money movement.
4. Pact stake and pact state change atomically.
5. All active pacts are evaluated for every outgoing debit.
6. Pending approval reserves neither funds nor pact capacity.
7. Only the current reviewer can decide, and denial reasons persist.
8. Approved execution rechecks balance and policy.
9. A conscious break never transfers stake ownership to Controlled.
10. Optional providers cannot weaken deterministic authorization.
