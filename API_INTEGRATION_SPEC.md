# Controlled HTTP API reference

This document describes the API implemented by the current repository. Local base URL:

```text
http://127.0.0.1:8080
```

## 1. Protocol

### JSON

Requests with bodies use `Content-Type: application/json`. Bodies must contain exactly one JSON object, must not contain unknown fields, and are limited to 1 MiB.

Errors have a stable envelope:

```json
{
  "error": {
    "code": "insufficient_balance",
    "message": "The wallet does not have enough available balance."
  }
}
```

Branch on `error.code`, not the human-readable message.

### Authentication

Protected endpoints accept either:

```http
Authorization: Bearer <controlled-session-token>
```

or the HTTP-only `controlled_session` cookie. Sessions last 24 hours unless revoked.

Password and Google authentication return `sessionToken`, `user`, and `expiresAt`. The demo session endpoint sets only the cookie and is intended for curl/local testing.

### Money and time

Money is an integer minor-unit amount. For USD, `1250` means `$12.50`. Product operations are USD-only. IDs are opaque UUID strings. Timestamps are RFC 3339.

### Idempotency

Creation and money commands marked **yes** below require `Idempotency-Key`. Replaying the same semantic request with the same key returns the original resource. Reusing a key with different input returns `409 idempotency_conflict`.

## 2. Endpoint inventory

| Method | Path | Auth | Idempotency |
| --- | --- | --- | --- |
| `GET` | `/healthz` | no | no |
| `GET` | `/readyz` | no | no |
| `POST` | `/v1/demo/users` | no | yes |
| `POST` | `/v1/demo/sessions` | no | no |
| `POST` | `/v1/auth/google` | no | no |
| `POST` | `/v1/auth/password/register` | no | no |
| `POST` | `/v1/auth/password/login` | no | no |
| `DELETE` | `/v1/sessions/current` | yes | no |
| `GET` | `/v1/users/current` | yes | no |
| `PATCH` | `/v1/users/current` | yes | no |
| `GET` | `/v1/users/recipients` | yes | no |
| `POST` | `/v1/push-devices` | yes | no |
| `DELETE` | `/v1/push-devices` | yes | no |
| `GET` | `/v1/wallet` | yes | no |
| `GET` | `/v1/wallet/transactions` | yes | no |
| `GET` | `/v1/wallet/transactions/{id}` | yes | no |
| `POST` | `/v1/wallet/funding-intents` | yes | yes |
| `POST` | `/v1/wallet/funding-intents/{id}/capture` | yes | no |
| `POST` | `/v1/demo/funding-intents/{id}/confirm` | yes | no |
| `POST` | `/v1/wallet/internal-transfers` | yes | yes |
| `POST` | `/v1/wallet/withdrawals` | yes | yes |
| `POST` | `/v1/wallet/withdrawals/{id}/reconcile` | yes | no |
| `POST` | `/v1/demo/withdrawals/{id}/settle` | yes | no |
| `POST` | `/v1/wallet/payment-requests` | yes | yes |
| `GET` | `/v1/wallet/payment-requests` | yes | no |
| `GET` | `/v1/wallet/payment-requests/{id}` | yes | no |
| `POST` | `/v1/wallet/payment-requests/{id}/pay` | yes | yes |
| `POST` | `/v1/wallet/payment-requests/{id}/decline` | yes | no |
| `POST` | `/v1/wallet/payment-requests/{id}/cancel` | yes | no |
| `POST` | `/v1/accountability/invitations` | yes | yes |
| `GET` | `/v1/accountability/invitations` | yes | no |
| `POST` | `/v1/accountability/invitations/{id}/accept` | yes | no |
| `POST` | `/v1/accountability/invitations/{id}/decline` | yes | no |
| `POST` | `/v1/accountability/circles` | yes | yes |
| `GET` | `/v1/accountability/circles` | yes | no |
| `GET` | `/v1/accountability/requests` | yes | no |
| `GET` | `/v1/accountability/requests/{id}` | yes | no |
| `POST` | `/v1/accountability/requests/{id}/approve` | yes | no |
| `POST` | `/v1/accountability/requests/{id}/deny` | yes | no |
| `POST` | `/v1/accountability/requests/{id}/cancel` | yes | no |
| `POST` | `/v1/pacts` | yes | yes |
| `GET` | `/v1/pacts` | yes | no |
| `GET` | `/v1/pacts/stake-recovery` | yes | no |
| `GET` | `/v1/pacts/{id}` | yes | no |
| `PUT` | `/v1/pacts/{id}` | yes | no |
| `POST` | `/v1/pacts/{id}/confirm` | yes | no |
| `POST` | `/v1/pacts/{id}/stake` | yes | yes |
| `POST` | `/v1/pacts/{id}/cancel` | yes | no |
| `POST` | `/v1/ai/pact-suggestions` | yes | no |
| `GET` | `/v1/ai/pacts/{id}/insight` | yes | no |
| `GET` | `/v1/ai/accountability/{id}/brief` | yes | no |
| `POST` | `/v1/shopping/price-checks` | yes | no |

## 3. Authentication and profile

Register with password:

```http
POST /v1/auth/password/register
Content-Type: application/json
```

```json
{
  "email": "alice@example.com",
  "password": "a-long-test-password",
  "displayName": "Alice"
}
```

Login uses the same route suffix with email/password:

```http
POST /v1/auth/password/login
```

```json
{"email":"alice@example.com","password":"a-long-test-password"}
```

Google authentication accepts an ID token issued for the configured Web client ID:

```http
POST /v1/auth/google
```

```json
{"idToken":"<google-id-token>"}
```

The successful response shape is:

```json
{
  "sessionToken": "<opaque-token>",
  "user": {
    "id": "<uuid>",
    "ctrldId": "ctrld...",
    "email": "alice@example.com",
    "displayName": "Alice",
    "namePending": false,
    "createdAt": "<timestamp>"
  },
  "expiresAt": "<timestamp>"
}
```

Read/update the profile with `GET`/`PATCH /v1/users/current`. Patch body:

```json
{"displayName":"Alice"}
```

Search recipients with `GET /v1/users/recipients?query=alice`. The query is optional and results exclude the current user.

Revoke the current token/cookie with `DELETE /v1/sessions/current`.

## 4. Demo bootstrap

For API-only local tests, create a user and wallet:

```bash
curl --fail-with-body -X POST http://127.0.0.1:8080/v1/demo/users \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-alice-1' \
  -d '{"email":"alice@example.test","displayName":"Alice","currency":"USD"}'
```

Start a cookie session:

```bash
curl --fail-with-body -X POST http://127.0.0.1:8080/v1/demo/sessions \
  -H 'Content-Type: application/json' \
  -c /tmp/controlled-alice.cookies \
  -d '{"email":"alice@example.test"}'
```

Use `-b /tmp/controlled-alice.cookies` on protected requests. Demo endpoints are not production identity flows.

## 5. Wallet reads

`GET /v1/wallet` returns wallet account IDs and balances:

```json
{
  "wallet": {
    "id": "<uuid>",
    "ownerId": "<uuid>",
    "currency": "USD",
    "accounts": {
      "available": "<uuid>",
      "pending": "<uuid>",
      "commitmentReserve": "<uuid>"
    },
    "balances": {
      "available": {"amountMinor": 10000, "currency": "USD"},
      "pending": {"amountMinor": 0, "currency": "USD"},
      "staked": {"amountMinor": 2500, "currency": "USD"},
      "commitmentReserve": {"amountMinor": 0, "currency": "USD"}
    }
  }
}
```

`GET /v1/wallet/transactions` returns recent transactions. `GET /v1/wallet/transactions/{id}` returns one transaction only when it belongs to the authenticated wallet. Transfer transactions can include `transferDetails.remark`; accountability-approved transactions can include `approvalDetails` with classification, owner reason, request ID, and approver.

## 6. Funding

Create a funding intent:

```http
POST /v1/wallet/funding-intents
Idempotency-Key: fund-1
```

```json
{"amountMinor":10000,"currency":"USD"}
```

With PayPal Sandbox, open the returned `approvalUrl`, approve as a Sandbox buyer, then call:

```http
POST /v1/wallet/funding-intents/{id}/capture
Content-Type: application/json

{}
```

With the fake provider, `/v1/demo/funding-intents/{id}/confirm` performs the corresponding local confirmation. A completed intent credits the provider-confirmed net amount exactly once.

## 7. Transfers, purchase checks, and withdrawals

Send to an existing Controlled email/ID selected from the recipient directory:

```http
POST /v1/wallet/internal-transfers
Idempotency-Key: send-1
```

```json
{
  "recipientEmail": "bob@example.test",
  "amountMinor": 3000,
  "currency": "USD",
  "reason": "Dinner and train ticket"
}
```

`reason` is persisted as the transfer remark. When active pacts block the debit, it is also the required owner reason for the accountability request. A permitted transfer returns the transfer. A blocked transfer with a reason returns `202` and `accountabilityRequest`. Insufficient funds return `409` and never create a review.

Optional product price check:

```http
POST /v1/shopping/price-checks
```

```json
{
  "query": "Sony WH-1000XM5 headphones",
  "amountMinor": 39999,
  "currency": "USD"
}
```

The response describes the best confident in-stock new USD offer, savings, and verdict. `503 price_check_unavailable` means Channel3 is not configured/unavailable; it does not affect transfer eligibility.

Create a withdrawal:

```http
POST /v1/wallet/withdrawals
Idempotency-Key: withdraw-1
```

```json
{
  "destination": "sandbox-paypal-recipient@example.test",
  "amountMinor": 2000,
  "currency": "USD",
  "reason": "Move to PayPal"
}
```

A permitted withdrawal reserves funds and returns `202`. Reconcile a PayPal payout with `POST /v1/wallet/withdrawals/{id}/reconcile`. The fake provider supports:

```http
POST /v1/demo/withdrawals/{id}/settle

{"succeeded":true}
```

Failure releases pending funds and reverses pact usage.

## 8. Payment requests

Create:

```http
POST /v1/wallet/payment-requests
Idempotency-Key: request-1
```

```json
{
  "recipientId": "<recipient-user-id>",
  "amountMinor": 1800,
  "currency": "USD",
  "note": "Shared cab"
}
```

List/detail:

```text
GET /v1/wallet/payment-requests
GET /v1/wallet/payment-requests/{id}
```

Actions:

```text
POST /v1/wallet/payment-requests/{id}/pay       (recipient, Idempotency-Key required)
POST /v1/wallet/payment-requests/{id}/decline   (recipient)
POST /v1/wallet/payment-requests/{id}/cancel    (requester)
```

Payment statuses are `PENDING`, `PROCESSING`, `AWAITING_APPROVAL`, `PAID`, `DECLINED`, `CANCELLED`, and `EXPIRED`. Paying goes through the payer's current balance and pact policy.

## 9. Accountability relationships

Invite an existing user:

```http
POST /v1/accountability/invitations
Idempotency-Key: invite-1

{"partnerEmail":"bob@example.test"}
```

The partner lists invitations and responds:

```text
GET  /v1/accountability/invitations
POST /v1/accountability/invitations/{id}/accept
POST /v1/accountability/invitations/{id}/decline
```

Create an immutable ordered circle using accepted partner user IDs:

```http
POST /v1/accountability/circles
Idempotency-Key: circle-1
```

```json
{
  "name": "Close friends",
  "partnerIds": ["<first-user-id>", "<second-user-id>"]
}
```

List with `GET /v1/accountability/circles`.

## 10. Pacts

Create/update terms:

```http
POST /v1/pacts
Idempotency-Key: pact-1
```

```json
{
  "goal": "Protect rent money",
  "intent": "Limit discretionary spending this month",
  "spendingLimitMinor": 20000,
  "stakeMinor": 5000,
  "currency": "USD",
  "startsAt": "2026-11-03T00:00:00Z",
  "endsAt": "2026-12-03T00:00:00Z",
  "accountabilityCircleId": "<circle-id>",
  "breakConsequence": "COMMITMENT_RESERVE"
}
```

`PUT /v1/pacts/{id}` accepts the same body while `DRAFT`. Confirm and reserve the stake:

The interval from `startsAt` to `endsAt` must be at least 24 hours. Pacts lasting from 24 hours to under seven days are valid spending commitments but do not qualify for Commitment Reserve recovery.

```text
POST /v1/pacts/{id}/confirm
POST /v1/pacts/{id}/stake     (Idempotency-Key required)
```

Stake reservation returns `SCHEDULED` for a future start or `ACTIVE` once the start has arrived. `GET /v1/pacts` and `GET /v1/pacts/{id}` include `progress` with used, remaining, overage, and usage items.

`POST /v1/pacts/{id}/cancel` supports draft, awaiting-stake, and scheduled pacts. Active pacts cannot be directly cancelled.

Read the authenticated user's stake and Commitment Reserve recovery plan:

```http
GET /v1/pacts/stake-recovery
```

```json
{
  "currency": "USD",
  "stakedMinor": 2000,
  "stakedPactCount": 1,
  "totalReservedMinor": 3000,
  "recoveredMinor": 1000,
  "commitmentReserveMinor": 2000,
  "projectedRecoveryMinor": 2000,
  "pacts": [
    {
      "id": "<pact-id>",
      "goal": "Stay within the weekly food budget",
      "status": "ACTIVE",
      "stakeMinor": 2000,
      "projectedRecoveryMinor": 2000,
      "startsAt": "2026-11-26T00:00:00Z",
      "recoveryEligible": true,
      "endsAt": "2026-12-03T00:00:00Z"
    }
  ]
}
```

The response includes all scheduled/active staked pacts and marks whether each meets the seven-day recovery minimum. Ineligible pacts have zero projected recovery. When an eligible pact resolves `KEPT` or `KEPT_WITH_EXCEPTIONS`, its own stake returns normally and up to the same amount moves from Commitment Reserve to Available. Recovery consumes the oldest eligible reserve allocations first and can partially recover one allocation. A pact can recover only reserve entries created before that pact's stake was reserved. Broken, cancelled, or shorter pacts recover nothing.

## 11. Accountability decisions

List/detail requests visible to the owner or reviewers:

```text
GET /v1/accountability/requests
GET /v1/accountability/requests/{id}
```

The detail includes transaction facts, owner reason, blocking-pact snapshots, current assignee, reviewer history, denial reasons, classification, and execution result.

The current reviewer approves:

```http
POST /v1/accountability/requests/{id}/approve

{"classification":"EMERGENCY"}
```

The other classification is `CONSCIOUS_BREAK`.

Denial requires a reason:

```http
POST /v1/accountability/requests/{id}/deny

{"reason":"This can wait until after the pact ends."}
```

The owner can cancel a pending request with `POST /v1/accountability/requests/{id}/cancel`.

Approval rechecks balance and all active pacts before execution. Emergency preserves the pact/stake. Conscious break resolves blocking pacts as broken and moves their stakes to Commitment Reserve.

## 12. Guidance

Suggest pact terms:

```http
POST /v1/ai/pact-suggestions

{"goal":"Keep weekends intentional","intent":"Protect travel savings"}
```

Read grounded insight/brief:

```text
GET /v1/ai/pacts/{id}/insight
GET /v1/ai/accountability/{id}/brief
```

Responses identify `source` as `AI` or `RULES`. Provider failure falls back to deterministic rules after verified source data loads. Guidance endpoints never mutate wallet, pact, or review state.

## 13. Push devices

Register:

```http
POST /v1/push-devices

{"token":"ExpoPushToken[...]","platform":"android"}
```

`platform` may be `android` or `ios`. Unregister with:

```http
DELETE /v1/push-devices

{"token":"ExpoPushToken[...]"}
```

Push delivery is best-effort. Always refresh the corresponding API resource.

## 14. Common error codes

| Status | Code | Meaning |
| --- | --- | --- |
| `400` | `invalid_request` | malformed/unknown JSON or missing body fields |
| `401` | `authentication_required` | no token/cookie |
| `401` | `invalid_session` | expired, revoked, or unknown token |
| `401` | `invalid_credentials` | password/demo credentials failed |
| `401` | `invalid_google_token` | Google token verification failed |
| `403` | `not_current_reviewer` | caller is not active reviewer |
| `404` | `*_not_found` | resource absent or not visible to caller |
| `409` | `idempotency_conflict` | key reused with different input |
| `409` | `insufficient_balance` | available balance cannot cover debit/stake |
| `409` | `policy_denied` | pact blocked debit and no review was created |
| `409` | `approval_stale` | approved operation no longer matches current state |
| `409` | `invalid_request_state` | resource is final or action is invalid now |
| `422` | `invalid_pact_terms` | pact money, period, text, circle, or consequence invalid |
| `422` | `pact_duration_too_short` | pact duration is less than 24 hours |
| `422` | `accountability_reason_required` | blocked debit lacks owner reason |
| `422` | `accountability_denial_reason_required` | reviewer denial lacks reason |
| `422` | `invalid_accountability_decision` | approval classification invalid |
| `422` | `invalid_price_check` | product query/amount invalid |
| `502` | `custody_unavailable` | custody provider failed |
| `503` | `price_check_unavailable` | Channel3 disabled/unavailable |
| `503` | `not_ready` | PostgreSQL readiness failed |

## 15. Client expectations

- Generate and retain one idempotency key per intended command retry sequence.
- Treat operation/resource IDs as opaque.
- Refresh wallet, transactions, pacts, and request state after mutations.
- Never calculate authoritative balances or pact outcomes client-side.
- Do not treat an initial withdrawal `202` as settlement completion.
- Do not retry a failed command with a new key unless the user intends a new operation.
- Never expose server provider credentials to a mobile client.
- Do not build production integrations on `/v1/demo/*` routes.
