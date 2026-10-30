# Controlled HTTP API integration specification

This document is for an external integrator testing the Controlled backend without knowledge of its internal code. It describes the HTTP API that exists now and includes a complete copy-paste workflow.

## Environment and scope

Local base URL:

```text
http://127.0.0.1:8080
```

The current API is a hackathon prototype:

- `/v1/demo/*` endpoints replace real identity verification and PayPal callbacks;
- funding and payout operations use an in-memory fake custody provider;
- internal Controlled wallet-to-wallet transfers use the real PostgreSQL ledger;
- accountability invitations, ordered circles, and pact drafts use PostgreSQL;
- funding and withdrawal ledger postings are real within the local database, but no real money moves; and
- pact policy enforcement is not connected yet, so valid debits are currently allowed when funds are sufficient.

See [SERVICE_SETUP.md](./SERVICE_SETUP.md) before running the flow.

## Protocol conventions

### JSON

Requests with bodies use:

```http
Content-Type: application/json
```

Bodies must contain exactly one JSON object, may not exceed 1 MiB, and may not contain unknown fields.

### Authentication

Protected routes require a Controlled session. The demo session endpoint sets an HTTP-only cookie named `controlled_session`. The examples use curl cookie jars:

```bash
-c /tmp/alice.cookies  # save the Set-Cookie response
-b /tmp/alice.cookies  # send that cookie on later requests
```

The authentication middleware also accepts `Authorization: Bearer <session-token>`, but the demo login response intentionally does not expose the raw token in JSON. Cookie authentication is therefore the supported external test flow.

Sessions last 24 hours unless revoked. Logging in does not invalidate older sessions.

### Money

All amounts are integers in the currency's minor unit. For USD:

```text
100   = USD 1.00
2500  = USD 25.00
10000 = USD 100.00
```

Operation amounts must be greater than zero. Currency codes are normalized to uppercase, must contain exactly three ASCII letters, and must match the wallet's currency.

### IDs and timestamps

Resource IDs are UUID strings. Timestamps are RFC 3339 UTC values, for example `2026-10-23T05:15:30.123456Z`.

### Idempotency

The following create endpoints require a non-empty `Idempotency-Key` header:

- onboarding
- funding intent creation
- internal transfer creation
- withdrawal creation
- accountability invitation creation
- accountability circle creation
- pact draft creation

Use a new opaque key for each intended operation. Keys for wallet operations are scoped to the acting user and operation type. Repeating a request with the same key and the same semantic input returns the original resource with `duplicate: true`. Reusing the key with different input returns `409 idempotency_conflict`.

Confirmation and settlement endpoints are also safe to retry. A completed resource is returned with `duplicate: true`.

### Error envelope

All JSON errors use this shape:

```json
{
  "error": {
    "code": "insufficient_balance",
    "message": "The wallet does not have enough available balance."
  }
}
```

Clients should branch on `error.code`, not the human-readable message.

## Endpoint summary

| Method | Path | Auth | Idempotency key | Success |
| --- | --- | --- | --- | --- |
| `GET` | `/healthz` | no | no | `204` |
| `GET` | `/readyz` | no | no | `204` |
| `POST` | `/v1/demo/users` | no | required | `201`, or `200` replay |
| `POST` | `/v1/demo/sessions` | no | no | `201` |
| `DELETE` | `/v1/sessions/current` | yes | no | `204` |
| `GET` | `/v1/users/current` | yes | no | `200` |
| `GET` | `/v1/wallet` | yes | no | `200` |
| `GET` | `/v1/wallet/transactions` | yes | no | `200` |
| `POST` | `/v1/wallet/funding-intents` | yes | required | `201`, or `200` replay |
| `POST` | `/v1/demo/funding-intents/{id}/confirm` | yes | no | `200` |
| `POST` | `/v1/wallet/internal-transfers` | yes | required | `201`, or `200` replay |
| `POST` | `/v1/wallet/withdrawals` | yes | required | `202` |
| `POST` | `/v1/demo/withdrawals/{id}/settle` | yes | no | `200` |
| `POST` | `/v1/accountability/invitations` | yes | required | `201`, or `200` replay |
| `GET` | `/v1/accountability/invitations` | yes | no | `200` |
| `POST` | `/v1/accountability/invitations/{id}/accept` | yes | no | `200` |
| `POST` | `/v1/accountability/invitations/{id}/decline` | yes | no | `200` |
| `POST` | `/v1/accountability/circles` | yes | required | `201`, or `200` replay |
| `GET` | `/v1/accountability/circles` | yes | no | `200` |
| `POST` | `/v1/pacts` | yes | required | `201`, or `200` replay |
| `GET` | `/v1/pacts` | yes | no | `200` |
| `GET` | `/v1/pacts/{id}` | yes | no | `200` |
| `PUT` | `/v1/pacts/{id}` | yes | no | `200` |
| `POST` | `/v1/pacts/{id}/confirm` | yes | no | `200` |

## Complete external test flow

The following walkthrough creates two users, funds one wallet, sends money, withdraws money, and verifies both ledgers. It assumes the API is already running.

### 1. Define test variables

Use a fresh run ID so the flow can be repeated against the same database:

```bash
export CONTROLLED_BASE_URL='http://127.0.0.1:8080'
export CONTROLLED_RUN_ID="$(date +%s)"
export ALICE_EMAIL="alice-${CONTROLLED_RUN_ID}@example.test"
export BOB_EMAIL="bob-${CONTROLLED_RUN_ID}@example.test"
export ALICE_COOKIES="/tmp/controlled-alice-${CONTROLLED_RUN_ID}.cookies"
export BOB_COOKIES="/tmp/controlled-bob-${CONTROLLED_RUN_ID}.cookies"
```

### 2. Check service health

```bash
curl --fail --silent --show-error --output /dev/null \
  "${CONTROLLED_BASE_URL}/healthz"

curl --fail --silent --show-error --output /dev/null \
  "${CONTROLLED_BASE_URL}/readyz"
```

Both commands should exit with status zero.

### 3. Onboard two users

Create Alice:

```bash
curl --fail-with-body --silent --show-error \
  -X POST "${CONTROLLED_BASE_URL}/v1/demo/users" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: onboard-alice-${CONTROLLED_RUN_ID}" \
  -d "{\"email\":\"${ALICE_EMAIL}\",\"displayName\":\"Alice\",\"currency\":\"USD\"}" \
  | jq
```

Create Bob:

```bash
curl --fail-with-body --silent --show-error \
  -X POST "${CONTROLLED_BASE_URL}/v1/demo/users" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: onboard-bob-${CONTROLLED_RUN_ID}" \
  -d "{\"email\":\"${BOB_EMAIL}\",\"displayName\":\"Bob\",\"currency\":\"USD\"}" \
  | jq
```

Each response contains a user, a wallet with three ledger account IDs, zero balances, and `duplicate: false`:

```json
{
  "user": {
    "id": "<user-uuid>",
    "email": "alice-<run-id>@example.test",
    "displayName": "Alice",
    "createdAt": "<timestamp>"
  },
  "wallet": {
    "id": "<wallet-uuid>",
    "ownerId": "<user-uuid>",
    "currency": "USD",
    "accounts": {
      "available": "<account-uuid>",
      "pending": "<account-uuid>",
      "commitmentReserve": "<account-uuid>"
    },
    "balances": {
      "available": {"amountMinor": 0, "currency": "USD"},
      "pending": {"amountMinor": 0, "currency": "USD"},
      "commitmentReserve": {"amountMinor": 0, "currency": "USD"}
    },
    "createdAt": "<timestamp>"
  },
  "duplicate": false
}
```

### 4. Start both sessions

```bash
curl --fail-with-body --silent --show-error \
  -X POST "${CONTROLLED_BASE_URL}/v1/demo/sessions" \
  -H 'Content-Type: application/json' \
  -c "${ALICE_COOKIES}" \
  -d "{\"email\":\"${ALICE_EMAIL}\"}" \
  | jq

curl --fail-with-body --silent --show-error \
  -X POST "${CONTROLLED_BASE_URL}/v1/demo/sessions" \
  -H 'Content-Type: application/json' \
  -c "${BOB_COOKIES}" \
  -d "{\"email\":\"${BOB_EMAIL}\"}" \
  | jq
```

The response contains `user` and `expiresAt`. Verify Alice's authenticated identity:

```bash
curl --fail-with-body --silent --show-error \
  -b "${ALICE_COOKIES}" \
  "${CONTROLLED_BASE_URL}/v1/users/current" \
  | jq
```

### 5. Create and confirm Alice's funding intent

Create a simulated USD 100.00 funding order and retain its ID:

```bash
FUNDING_RESPONSE="$(curl --fail-with-body --silent --show-error \
  -X POST "${CONTROLLED_BASE_URL}/v1/wallet/funding-intents" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: fund-alice-${CONTROLLED_RUN_ID}" \
  -b "${ALICE_COOKIES}" \
  -d '{"amountMinor":10000,"currency":"USD"}')"

echo "${FUNDING_RESPONSE}" | jq
export FUNDING_ID="$(echo "${FUNDING_RESPONSE}" | jq -r '.fundingIntent.id')"
```

The initial status is `AWAITING_CONFIRMATION`. The `approvalUrl` uses the fake provider and must not be opened.

Simulate provider approval and capture:

```bash
curl --fail-with-body --silent --show-error \
  -X POST "${CONTROLLED_BASE_URL}/v1/demo/funding-intents/${FUNDING_ID}/confirm" \
  -H 'Content-Type: application/json' \
  -b "${ALICE_COOKIES}" \
  -d '{}' \
  | jq
```

The status is now `COMPLETED`, and `ledgerTransactionId` and `completedAt` are populated.

Verify Alice's available balance is `10000`:

```bash
curl --fail-with-body --silent --show-error \
  -b "${ALICE_COOKIES}" \
  "${CONTROLLED_BASE_URL}/v1/wallet" \
  | jq '.wallet.balances'
```

### 6. Transfer USD 30.00 from Alice to Bob

```bash
curl --fail-with-body --silent --show-error \
  -X POST "${CONTROLLED_BASE_URL}/v1/wallet/internal-transfers" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: alice-to-bob-${CONTROLLED_RUN_ID}" \
  -b "${ALICE_COOKIES}" \
  -d "{\"recipientEmail\":\"${BOB_EMAIL}\",\"amountMinor\":3000,\"currency\":\"USD\"}" \
  | jq
```

The response status is `201`, and the body contains:

```json
{
  "transfer": {
    "id": "<transfer-uuid>",
    "recipientId": "<bob-user-uuid>",
    "recipientEmail": "bob-<run-id>@example.test",
    "amount": {"amountMinor": 3000, "currency": "USD"},
    "ledgerTransactionId": "<ledger-transaction-uuid>",
    "createdAt": "<timestamp>",
    "duplicate": false
  }
}
```

Verify balances:

```bash
curl --fail-with-body --silent --show-error \
  -b "${ALICE_COOKIES}" \
  "${CONTROLLED_BASE_URL}/v1/wallet" \
  | jq '.wallet.balances'

curl --fail-with-body --silent --show-error \
  -b "${BOB_COOKIES}" \
  "${CONTROLLED_BASE_URL}/v1/wallet" \
  | jq '.wallet.balances'
```

Alice should have `7000` available and Bob should have `3000` available.

### 7. Withdraw USD 20.00 from Alice

Create and retain the withdrawal:

```bash
WITHDRAWAL_RESPONSE="$(curl --fail-with-body --silent --show-error \
  -X POST "${CONTROLLED_BASE_URL}/v1/wallet/withdrawals" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: withdraw-alice-${CONTROLLED_RUN_ID}" \
  -b "${ALICE_COOKIES}" \
  -d '{"destination":"alice-paypal@example.test","amountMinor":2000,"currency":"USD"}')"

echo "${WITHDRAWAL_RESPONSE}" | jq
export WITHDRAWAL_ID="$(echo "${WITHDRAWAL_RESPONSE}" | jq -r '.withdrawal.id')"
```

The endpoint returns `202 Accepted` with status `PENDING`. While pending, Alice should have `5000` available and `2000` pending.

Simulate a successful provider payout:

```bash
curl --fail-with-body --silent --show-error \
  -X POST "${CONTROLLED_BASE_URL}/v1/demo/withdrawals/${WITHDRAWAL_ID}/settle" \
  -H 'Content-Type: application/json' \
  -b "${ALICE_COOKIES}" \
  -d '{"succeeded":true}' \
  | jq
```

The withdrawal status becomes `COMPLETED`. Alice remains at `5000` available and returns to `0` pending.

To test payout failure, create a different withdrawal with a new idempotency key, then settle it with `{"succeeded":false}`. Its status becomes `FAILED`, and the reserved amount moves from pending back to available.

### 8. Inspect transaction history

```bash
curl --fail-with-body --silent --show-error \
  -b "${ALICE_COOKIES}" \
  "${CONTROLLED_BASE_URL}/v1/wallet/transactions" \
  | jq
```

The endpoint returns up to the 50 most recent ledger transactions involving the user's wallet, newest first:

```json
{
  "transactions": [
    {
      "id": "<ledger-transaction-uuid>",
      "reference": {
        "type": "WITHDRAWAL_SETTLEMENT",
        "id": "<withdrawal-uuid>"
      },
      "description": "Settle withdrawal",
      "occurredAt": "<timestamp>",
      "postedAt": "<timestamp>",
      "reversed": false,
      "entries": [
        {
          "accountId": "<account-uuid>",
          "accountKind": "PENDING",
          "side": "DEBIT",
          "amount": {"amountMinor": 2000, "currency": "USD"}
        }
      ]
    }
  ]
}
```

Each user sees only entries belonging to that user's wallet, even when the underlying balanced ledger transaction includes system or counterparty accounts.

### 9. Verify idempotent replay

Repeat Alice's transfer with the exact same key and body:

```bash
curl --fail-with-body --silent --show-error \
  -X POST "${CONTROLLED_BASE_URL}/v1/wallet/internal-transfers" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: alice-to-bob-${CONTROLLED_RUN_ID}" \
  -b "${ALICE_COOKIES}" \
  -d "{\"recipientEmail\":\"${BOB_EMAIL}\",\"amountMinor\":3000,\"currency\":\"USD\"}" \
  | jq
```

This returns the original transfer with `duplicate: true`; it does not move another USD 30.00.

### 10. Revoke sessions

```bash
curl --fail-with-body --silent --show-error \
  -X DELETE \
  -b "${ALICE_COOKIES}" \
  "${CONTROLLED_BASE_URL}/v1/sessions/current"

curl --fail-with-body --silent --show-error \
  -X DELETE \
  -b "${BOB_COOKIES}" \
  "${CONTROLLED_BASE_URL}/v1/sessions/current"
```

Both return `204 No Content`. A later protected request with either cookie returns `401 invalid_session`.

## Endpoint details

### `GET /healthz`

Returns `204 No Content` while the HTTP process is alive. It does not check dependencies.

### `GET /readyz`

Returns `204 No Content` when PostgreSQL responds within the configured timeout. Otherwise returns `503 not_ready`.

### `POST /v1/demo/users`

Creates one demo user, one wallet, and its `AVAILABLE`, `PENDING`, and `COMMITMENT_RESERVE` ledger accounts atomically.

Headers:

```http
Content-Type: application/json
Idempotency-Key: <required opaque key>
```

Request:

```json
{
  "email": "person@example.test",
  "displayName": "Person",
  "currency": "USD"
}
```

Email is trimmed and lowercased. Display name must not be blank. A user may own only one wallet, and emails are unique.

### `POST /v1/demo/sessions`

Starts a 24-hour session for an existing demo user.

```json
{
  "email": "person@example.test"
}
```

Success sets the `controlled_session` cookie and returns the user plus `expiresAt`. There is currently no password or external identity-provider check; this route is demo-only.

### `DELETE /v1/sessions/current`

Revokes the presented session and expires its cookie. Requires authentication. Returns `204 No Content`.

### `GET /v1/users/current`

Returns the authenticated user:

```json
{
  "user": {
    "id": "<user-uuid>",
    "email": "person@example.test",
    "displayName": "Person",
    "createdAt": "<timestamp>"
  }
}
```

### `GET /v1/wallet`

Returns wallet metadata, account IDs, and current balances. Requires authentication. Its wallet shape matches the onboarding response.

Balance meanings:

- `available`: spendable now
- `pending`: reserved for an in-flight withdrawal
- `commitmentReserve`: retained after a future conscious pact break; currently always zero through public API flows

### `GET /v1/wallet/transactions`

Returns a fixed maximum of 50 recent ledger transactions. Pagination and caller-selected limits are not implemented.

Reference types currently include:

- `FUNDING`
- `INTERNAL_TRANSFER`
- `WITHDRAWAL_RESERVATION`
- `WITHDRAWAL_SETTLEMENT`
- `WITHDRAWAL_RELEASE`

A failed withdrawal produces a release transaction that returns the pending amount to available balance.

### `POST /v1/wallet/funding-intents`

Creates a fake custody funding order. Requires authentication and an idempotency key.

```json
{
  "amountMinor": 10000,
  "currency": "USD"
}
```

The response status is `AWAITING_CONFIRMATION` until the demo confirmation endpoint is called. Creating the intent alone does not change wallet balances.

### `POST /v1/demo/funding-intents/{id}/confirm`

Simulates successful provider approval and capture for a funding intent owned by the authenticated user.

```json
{}
```

On success, the status becomes `COMPLETED` and the amount is credited to available balance. An ID belonging to another user is treated as not found.

### `POST /v1/wallet/internal-transfers`

Atomically moves available balance from the authenticated user's wallet to another Controlled wallet.

```json
{
  "recipientEmail": "recipient@example.test",
  "amountMinor": 3000,
  "currency": "USD"
}
```

The recipient must already exist, must not be the sender, and must have a wallet in the same currency. Insufficient available balance returns `409 insufficient_balance` without creating a transfer.

### `POST /v1/wallet/withdrawals`

Reserves available balance and creates a fake pending payout.

```json
{
  "destination": "paypal-recipient@example.test",
  "amountMinor": 2000,
  "currency": "USD"
}
```

`destination` is currently an opaque, non-empty string. The route returns `202 Accepted` with status `PENDING`. A production implementation must validate a real PayPal destination.

### `POST /v1/demo/withdrawals/{id}/settle`

Simulates the custody provider's final payout result for a withdrawal owned by the authenticated user.

Successful payout:

```json
{"succeeded": true}
```

Failed payout:

```json
{"succeeded": false}
```

Success removes the pending liability and sets `COMPLETED`. Failure releases the amount back to available balance and sets `FAILED`.

### Accountability relationships

Invite an existing Controlled user:

```http
POST /v1/accountability/invitations
Idempotency-Key: invite-bob-1
```

```json
{"partnerEmail": "bob@example.test"}
```

The invited user discovers the invitation through `GET /v1/accountability/invitations` and responds with either:

```text
POST /v1/accountability/invitations/{id}/accept
POST /v1/accountability/invitations/{id}/decline
```

After acceptance, the owner creates an explicitly ordered circle:

```http
POST /v1/accountability/circles
Idempotency-Key: create-circle-1
```

```json
{
  "name": "Close friends",
  "partnerIds": ["first-partner-uuid", "second-partner-uuid"]
}
```

Every partner must have accepted an invitation from that owner. Circles are immutable; create a new circle to use a different order or membership. `GET /v1/accountability/circles` lists the authenticated owner's circles.

### Pact drafts

Create a USD pact draft:

```http
POST /v1/pacts
Idempotency-Key: create-pact-1
```

```json
{
  "goal": "Keep enough money for rent",
  "intent": "Reduce discretionary spending this month",
  "spendingLimitMinor": 20000,
  "stakeMinor": 5000,
  "currency": "USD",
  "startsAt": "2026-11-01T00:00:00+05:30",
  "endsAt": "2026-12-01T00:00:00+05:30",
  "accountabilityCircleId": "circle-uuid",
  "breakConsequence": "COMMITMENT_RESERVE"
}
```

`PUT /v1/pacts/{id}` accepts the same body while the pact remains `DRAFT`. `GET /v1/pacts` lists owned pacts and `GET /v1/pacts/{id}` returns one owned pact.

`POST /v1/pacts/{id}/confirm` makes the terms immutable and moves the pact to `AWAITING_STAKE`. Phase 3 will reserve the stake and move it to `SCHEDULED` or `ACTIVE`; confirmation alone does not yet enforce the spending limit.

## Error reference

| HTTP status | Code | Meaning |
| --- | --- | --- |
| `400` | `invalid_request` | malformed JSON, an unknown field, more than one JSON value, or an invalid settlement body |
| `400` | `invalid_idempotency_key` | required `Idempotency-Key` is blank or missing |
| `401` | `authentication_required` | no session cookie or bearer token was supplied |
| `401` | `invalid_credentials` | demo login email does not identify an onboarded user |
| `401` | `invalid_session` | session is invalid, expired, or revoked |
| `404` | `wallet_not_found` | authenticated user has no provisioned wallet |
| `404` | `wallet_operation_not_found` | recipient or user-owned operation does not exist |
| `404` | `accountability_not_found` | partner or invitation does not exist |
| `404` | `pact_not_found` | user-owned pact does not exist |
| `409` | `user_exists` | an account already exists for the onboarding email |
| `409` | `idempotency_conflict` | the key was previously used with different input |
| `409` | `insufficient_balance` | available wallet balance cannot cover the debit |
| `409` | `policy_denied` | pact policy rejected the debit; reserved for the policy integration |
| `409` | `invalid_operation_state` | operation cannot make the requested state transition |
| `409` | `accountability_not_ready` | selected circle is missing, not owned, or has no eligible partners |
| `409` | `partner_not_accepted` | at least one requested circle partner has not accepted |
| `409` | `invitation_exists` | an active invitation or relationship already exists |
| `409` | `invitation_not_resolvable` | invitation is final or caller is not its invited partner |
| `409` | `pact_conflict` | pact idempotency or optimistic version conflict |
| `422` | `invalid_email` | login email syntax is invalid |
| `422` | `invalid_onboarding` | onboarding email, name, or currency is invalid |
| `422` | `invalid_wallet_operation` | amount, currency, destination, recipient, or self-transfer is invalid |
| `422` | `invalid_accountability_relationship` | invalid owner, partner, circle name, or duplicate membership |
| `422` | `invalid_pact_terms` | invalid money, period, goal, intent, circle, or break consequence |
| `502` | `custody_unavailable` | custody provider is unavailable |
| `503` | `not_ready` | readiness check cannot reach PostgreSQL |
| `500` | `internal_error` | unexpected server or persistence failure |

## Integration expectations

- Treat every money-moving create request as retryable only when using the same idempotency key and identical input.
- Store operation IDs and ledger transaction IDs as opaque values.
- Do not calculate authoritative balances client-side; refresh `GET /v1/wallet` after a completed operation.
- Do not infer payout completion from the initial `202`; wait for a terminal `COMPLETED` or `FAILED` state. The MVP exposes demo settlement instead of webhooks or polling endpoints.
- Do not follow `fake.controlled.local` approval URLs. They are placeholders.
- Complete fake provider flows before restarting the API process.
- Never use `/v1/demo/*` routes as the design for production authentication or PayPal callbacks.
