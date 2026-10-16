# Pay Pact Backend Specification

Status: Draft for hackathon MVP  
Backend language: Go  
API style: JSON over HTTP  
Database: PostgreSQL  
External systems: PayPal Sandbox and an AI model provider

## 1. Purpose

The backend is the authoritative implementation of every Pay Pact workflow. The web application, AI agent, background worker, and integration tests must use the same application services and domain rules.

The backend owns:

- Pact lifecycle and invariants
- Spending decisions
- Accountability-partner assignment and responses
- Stake payment orchestration
- Settlement decisions
- AI tool authorization
- Timeouts, retries, and scheduled completion
- Audit history

The frontend must not contain business rules.

## 2. System boundary

```text
Web client ─────────────┐
AI tool calls ──────────┼──> Go API/Application Services
Background worker ──────┤             │
Integration tests ──────┘             ├──> PostgreSQL
                                      ├──> PayPal Sandbox
                                      └──> AI Provider
```

PayPal owns payment authorization and money movement. Pay Pact owns the commitment decision that permits a payment operation.

AI output is untrusted input. AI cannot bypass application validation or call PayPal directly.

## 3. Architecture

Use a modular monolith deployed as one Go service for the MVP.

```text
cmd/api/                    Process entry point
internal/identity/          Users, sessions, partner access tokens
internal/pact/              Pact aggregate and lifecycle
internal/budgeting/         Spending ledger and limit checks
internal/accountability/    Overrides, assignments, responses, scores
internal/payments/          Stake and purchase payment orchestration
internal/ai/                AI prompts, schemas, and tool loop
internal/jobs/              Outbox, retries, timeouts, scheduled work
internal/audit/             Immutable product event history
internal/platform/          Database, HTTP, clock, IDs, configuration
migrations/                 PostgreSQL migrations
sql/                        Queries used by sqlc
openapi/                    Public HTTP API definition
```

Each module should separate:

1. HTTP transport
2. Application services
3. Domain rules
4. Repository and external-service adapters

No module may update another module's tables directly. Cross-module work occurs through application services inside a database transaction or through outbox events.

## 4. Technical stack

- Go with `net/http` and `chi`
- PostgreSQL
- `pgx` for database connectivity
- `sqlc` for typed queries
- `goose` for SQL migrations
- Standard `net/http` client for PayPal REST APIs
- Official Go SDK for the AI provider where available
- `slog` for structured logs
- OpenAPI for the HTTP contract

Redis, Kafka, and separate microservices are out of scope.

## 5. Data conventions

- IDs are UUIDs.
- All timestamps are stored as UTC `timestamptz` values.
- Money is stored as integer minor units plus an ISO currency code.
- Floating-point values must never represent money.
- API field names use `camelCase`.
- Database column names use `snake_case`.
- State values are uppercase strings.
- Every mutable aggregate has an integer `version` for optimistic concurrency.

Example money value:

```json
{
  "amountMinor": 2500,
  "currency": "USD"
}
```

## 6. Domain model

### 6.1 Pact

```text
DRAFT
  -> AWAITING_STAKE
  -> ACTIVE
  -> ACTIVE_WITH_EXCEPTION
  -> SUCCEEDED
  -> BREACHED
```

Terminal states are `SUCCEEDED` and `BREACHED`.

A pact contains:

- Owner
- Spending limit
- Stake amount
- Currency
- Start and end timestamps
- Neutral-fund destination
- Four ordered accountability partners
- Current round-robin pointer
- Current status and version

Active pact financial terms are immutable.

### 6.2 Stake payment

Stake payment state is separate from pact state.

```text
NOT_FUNDED
  -> ORDER_CREATED
  -> CAPTURE_PENDING
  -> FUNDED
  -> REFUND_PENDING -> REFUNDED
  -> FORFEIT_PENDING -> FORFEITED
  -> FAILED
```

The pact becomes `ACTIVE` only after the backend verifies that the stake capture is completed.

### 6.3 Purchase intent

A purchase intent represents an attempted purchase inside the controlled MVP environment.

```text
PENDING_CHECK
  -> ALLOWED
  -> OVERRIDE_REQUIRED
  -> APPROVED_BY_OVERRIDE
  -> DENIED
  -> COMPLETED
  -> CANCELLED
```

Completing a purchase adds its captured amount to the pact spending ledger. A purchase must not be counted from a browser callback alone.

An approved override applies only to the purchase intent and amount referenced by the override. It is not a general limit increase.

### 6.4 Override request

Types:

- `EMERGENCY_EXCEPTION`
- `CONSCIOUS_BREAK`

States:

```text
REQUESTED
  -> ASSIGNED
  -> AWAITING_USER
  -> REASSIGNED
  -> APPROVED
  -> DENIED
  -> EXPIRED
```

Resolution effects:

| Type | Approved access | Pact effect | Stake effect |
|---|---|---|---|
| Emergency exception | Grant requested purchase | `ACTIVE_WITH_EXCEPTION` | Preserve until pact completion |
| Conscious break | Grant requested purchase | `BREACHED` | Forfeit |
| Denied or expired | No grant | No change | Preserve |

AI cannot choose the request type or decide whether an emergency is legitimate.

### 6.5 Partner assignment

Each override has one active assignment at a time.

```text
ACTIVE
  -> ACKNOWLEDGED
  -> RESPONDED
  -> TIMED_OUT
  -> SUPERSEDED
```

Rules:

- Select the next partner using the pact's round-robin pointer.
- The owner cannot select the reviewer.
- Failure to acknowledge before the deadline creates an assignment for the next partner.
- Only the current active assignment can resolve the override.
- A timed-out partner's late response is rejected.
- Advance the round-robin pointer once per override, regardless of outcome.

### 6.6 Responsiveness projection

Responsiveness is derived from assignment events and is not authoritative domain state.

Score the last ten assignments:

| Acknowledgement time | Points |
|---|---:|
| Within 5 minutes | 5 |
| Within 30 minutes | 4 |
| Within 2 hours | 3 |
| Before deadline | 2 |
| After deadline | 1 |
| No response | 0 |

Approval and denial decisions do not affect the score.

## 7. Core invariants

1. A pact cannot activate before its stake is funded.
2. An owner cannot be an accountability partner on their own pact.
3. A pact must have four distinct accountability partners before confirmation.
4. Active pact financial terms cannot be edited.
5. Only one assignment may be active for an override.
6. Only an active assignment may resolve an override.
7. One approved override grants only its referenced purchase and amount.
8. A pact may be settled only once.
9. A PayPal order, capture, refund, payout, and webhook event may be processed only once.
10. AI output never overrides domain validation.
11. Emergency exceptions are recorded without being treated as absolute failures.
12. The operating account is not a forfeiture destination.
13. Every consequential state change creates an audit event.

## 8. HTTP API

All endpoints use the `/v1` prefix.

### 8.1 Identity

```text
POST /v1/demo/sessions
DELETE /v1/sessions/current
GET /v1/users/current
```

The MVP may use seeded demo identities and secure HTTP-only session cookies. Production authentication is out of scope.

Partner review links use random, single-purpose, expiring tokens. Store only a hash of each token.

### 8.2 AI pact proposal

```text
POST /v1/ai/pact-proposals
```

Request:

```json
{
  "message": "Keep my discretionary spending below $150 next week. I will stake $25."
}
```

Response:

```json
{
  "proposal": {
    "limit": { "amountMinor": 15000, "currency": "USD" },
    "stake": { "amountMinor": 2500, "currency": "USD" },
    "startsAt": "2026-10-19T00:00:00Z",
    "endsAt": "2026-10-26T00:00:00Z"
  },
  "assumptions": [],
  "missingInformation": ["Four accountability partners are required."],
  "warnings": []
}
```

This endpoint proposes terms only. It does not create or activate a pact.

### 8.3 Pacts

```text
POST /v1/pacts
GET /v1/pacts/{pactId}
POST /v1/pacts/{pactId}/partners
POST /v1/pacts/{pactId}/confirm
GET /v1/pacts/{pactId}/events
```

`confirm` validates the full pact and moves it from `DRAFT` to `AWAITING_STAKE`.

### 8.4 Stake funding

```text
POST /v1/pacts/{pactId}/stake/orders
POST /v1/pacts/{pactId}/stake/orders/{orderId}/capture
GET /v1/pacts/{pactId}/stake
```

The backend calculates the stake amount from the pact. A client-supplied amount is never trusted.

Order capture is accepted only when the order belongs to the pact and the authenticated owner approved it.

### 8.5 Purchase intents

```text
POST /v1/pacts/{pactId}/purchase-intents
GET /v1/purchase-intents/{purchaseIntentId}
POST /v1/purchase-intents/{purchaseIntentId}/orders
POST /v1/purchase-intents/{purchaseIntentId}/orders/{orderId}/capture
```

Creating a purchase intent runs the deterministic budget check. If the purchase exceeds the remaining limit, the response is `OVERRIDE_REQUIRED` and no purchase order is created.

### 8.6 Overrides

```text
POST /v1/purchase-intents/{purchaseIntentId}/overrides
GET /v1/overrides/{overrideId}
POST /v1/overrides/{overrideId}/additional-information
```

The request type is explicitly selected by the owner. AI may help draft the reason but cannot submit a different type without owner confirmation.

### 8.7 Partner review

```text
GET /v1/partner/assignments/{reviewToken}
POST /v1/partner/assignments/{reviewToken}/acknowledge
POST /v1/partner/assignments/{reviewToken}/respond
```

Response body:

```json
{
  "decision": "APPROVE",
  "message": "Approved as presented."
}
```

Allowed decisions are `APPROVE`, `DENY`, and `NEED_MORE_INFO`.

### 8.8 PayPal webhooks

```text
POST /v1/webhooks/paypal
```

The handler must:

1. Read and preserve the original request body.
2. Verify webhook authenticity using PayPal's verification mechanism.
3. Insert the PayPal event ID into a table with a unique constraint.
4. Return success for already processed events.
5. Apply state changes transactionally.
6. Return a non-success response only when retrying is useful.

Browser callbacks are not authoritative payment confirmation.

### 8.9 Health

```text
GET /health/live
GET /health/ready
```

Readiness checks database connectivity but must not depend on PayPal or the AI provider being reachable.

## 9. Error format

```json
{
  "error": {
    "code": "PACT_INVALID_TRANSITION",
    "message": "An active pact cannot be confirmed again.",
    "requestId": "6dd8dc79-34d4-4f97-8926-469578d32545",
    "details": {}
  }
}
```

Expected domain failures return `4xx`. Unexpected failures return `500` with no secret or provider response body exposed.

## 10. Database model

Minimum tables:

```text
users
sessions
pacts
pact_partners
stake_payments
purchase_intents
spend_events
override_requests
assignments
partner_responses
outbox_jobs
paypal_webhook_events
audit_events
ai_runs
```

Required database protections include:

- Unique partner position per pact
- Unique partner membership per pact
- Partial unique index for one active assignment per override
- Unique PayPal order, capture, refund, payout, and webhook event IDs
- Unique successful settlement per pact
- Foreign keys for every aggregate relationship
- Check constraints for positive money amounts and valid time ranges

## 11. Transactions and concurrency

Use a database transaction for each state-changing application command.

Lock or compare the aggregate version before transitions that may race:

- Pact confirmation
- PayPal capture processing
- Partner reassignment
- Partner response
- Pact completion
- Refund or forfeiture scheduling

External network calls must not occur while holding long-lived database locks.

Use this pattern:

1. Validate and reserve the operation in a transaction.
2. Commit an outbox job.
3. Perform the provider call outside the transaction.
4. Persist the provider result in a new transaction.

## 12. Idempotency

State-changing public endpoints accept an `Idempotency-Key` header.

The backend stores:

- Caller identity
- Endpoint or command name
- Idempotency key
- Request hash
- Final response

Reusing a key with a different request body returns a conflict.

Use a stable `PayPal-Request-Id` for each retryable PayPal operation. Provider IDs and webhook event IDs also have unique database constraints.

## 13. Background jobs

Use PostgreSQL as the MVP job queue. Workers claim jobs with row locking and leases.

Job types:

- `ASSIGN_PARTNER`
- `EXPIRE_ASSIGNMENT`
- `SEND_PARTNER_NOTIFICATION`
- `COMPLETE_PACT`
- `REFUND_STAKE`
- `FORFEIT_STAKE`
- `RECONCILE_PAYPAL_PAYMENT`

Jobs must be safe to run more than once. Retry transient failures with bounded exponential backoff. Permanent failures move to a failed state visible in the audit timeline.

## 14. PayPal adapter

Define a narrow internal interface rather than exposing PayPal types to the domain:

```go
type Payments interface {
    CreateStakeOrder(context.Context, CreateStakeOrder) (Order, error)
    CaptureOrder(context.Context, CaptureOrder) (Capture, error)
    RefundStake(context.Context, RefundStake) (Refund, error)
    ForfeitStake(context.Context, ForfeitStake) (Payout, error)
    VerifyWebhook(context.Context, WebhookRequest) (VerifiedEvent, error)
}
```

Provide two implementations:

- `PayPalPayments`
- `FakePayments`

Rules:

- PayPal credentials remain server-side.
- Use sandbox endpoints for the hackathon.
- Stake orders use the amount stored on the pact.
- Refunds reference the original stake capture.
- Forfeitures target only the configured neutral-fund sandbox account.
- Log PayPal resource IDs, never credentials or full sensitive payloads.
- A completed webhook or reconciliation result is required before final payment state is trusted.

## 15. AI adapter

Use structured outputs for pact proposals and partner summaries. Use function calling only for application capabilities exposed as narrow tools.

Initial AI operations:

- `ProposePact`
- `ExplainPact`
- `SuggestPurchaseAlternatives`
- `PrepareOverrideBrief`

Allowed tools:

- `get_pact`
- `check_purchase`
- `create_override_draft`

The AI is not permitted to call payment settlement functions.

Every AI run records:

- Purpose
- Model identifier
- Prompt or prompt version
- Tool calls
- Structured result
- Latency
- Success or failure
- Related pact or override ID

Do not place PayPal credentials, review tokens, or unnecessary personal information in model input. Disable provider-side response storage where supported.

## 16. Audit events

Audit events are append-only and include:

- Event type
- Aggregate type and ID
- Actor type and ID
- Timestamp
- Request ID
- Previous and resulting state where applicable
- Non-sensitive metadata

Minimum events:

```text
PACT_CREATED
PACT_CONFIRMED
STAKE_ORDER_CREATED
STAKE_FUNDED
PACT_ACTIVATED
PURCHASE_CHECKED
OVERRIDE_REQUESTED
PARTNER_ASSIGNED
PARTNER_ACKNOWLEDGED
ASSIGNMENT_TIMED_OUT
OVERRIDE_APPROVED
OVERRIDE_DENIED
EMERGENCY_EXCEPTION_APPLIED
PACT_SUCCEEDED
PACT_BREACHED
STAKE_REFUNDED
STAKE_FORFEITED
```

## 17. Security requirements

- Secrets are loaded from environment or the deployment secret store.
- Session cookies are `HttpOnly`, `Secure`, and `SameSite=Lax` or stricter.
- Partner review tokens are random, hashed at rest, scoped to one assignment, and expire.
- Apply authorization checks inside application services, not only HTTP middleware.
- Verify PayPal webhooks before processing them.
- Rate-limit AI, authentication, partner response, and payment endpoints.
- Validate currency and amount consistency at every payment boundary.
- Never accept a PayPal resource ID without checking its database ownership.
- Redact secrets and personal data from logs.
- Include request IDs in logs and error responses.

## 18. Testing strategy

### Unit tests

Test domain state transitions and invariants without external services:

- Pact confirmation and activation
- Spending limit calculations
- Emergency exception behavior
- Conscious-break behavior
- Round-robin selection
- Timeout and late-response rejection
- Responsiveness scoring
- Settlement eligibility

### Repository tests

Run against PostgreSQL and verify constraints, transactions, concurrent responses, and job claiming.

### Provider contract tests

Test PayPal and AI adapters against recorded or local fake HTTP servers. Never make normal unit tests depend on external services.

### Sandbox integration tests

Run explicit tests against PayPal Sandbox for:

- Stake order creation and capture
- Duplicate capture handling
- Webhook verification and replay
- Refund
- Payout or documented forfeiture fallback

### End-to-end test

The required automated happy path is:

```text
create pact
-> add four partners
-> confirm
-> fund stake
-> activate
-> create over-limit purchase intent
-> request emergency exception
-> time out first partner
-> assign next partner
-> approve
-> complete purchase
-> complete pact
-> refund stake
```

A second path covers an approved conscious break and forfeiture.

## 19. Observability

Every request log includes:

- Request ID
- Method and route
- Authenticated actor ID
- Status code
- Duration

Metrics should include:

- HTTP error rate and latency
- Active and failed jobs
- Partner acknowledgement latency
- PayPal operation failures
- Webhook processing lag
- AI latency and structured-output failures

Do not log model prompts or payment payloads indiscriminately.

## 20. Delivery phases

### Phase 1: Fake-provider vertical slice

- Database and migrations
- Pact state machine
- Purchase checks
- Round-robin partner workflow
- Fake payments
- Audit timeline
- End-to-end API test

### Phase 2: PayPal Sandbox

- Checkout order creation
- Capture
- Webhook verification
- Refund
- Payout or documented forfeiture fallback
- Idempotency and reconciliation

### Phase 3: AI

- Structured pact proposal
- Purchase alternatives
- Override brief
- Tool-call authorization and audit

### Phase 4: Submission hardening

- Demo identities and seeded data
- Public API documentation
- Deployment configuration
- Error and loading behavior
- Reproducible judge setup
- Demo script

## 21. MVP exclusions

- Production wallet, escrow, or custody claims
- Real prepaid cards
- Bank aggregation
- Monitoring transactions outside Pay Pact
- Real charitable or tax-deductible donation claims
- Quorum or weighted voting
- Multiple simultaneous active pacts per owner
- Microservices or a distributed message broker
- AI deciding emergency legitimacy or moving money autonomously
